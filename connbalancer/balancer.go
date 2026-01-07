// Copyright 2024 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connbalancer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type ConnectionSelector struct {
	ServerName      string
	Cluster         string
	Idle            time.Duration
	Account         string
	SubjectInterest string
	Kind            []string
}

type Balancer interface {
	Balance(ctx context.Context) (balanced int, err error)
}

type balancer struct {
	nc       *nats.Conn
	duration time.Duration
	limits   *ConnectionSelector
	log      api.Logger
}

type conn struct {
	serverId   string
	serverName string
	conn       *server.ConnInfo
}

type serverConnections struct {
	serverId    string
	serverName  string
	connections []*conn
	offset      int
}

func New(nc *nats.Conn, runTime time.Duration, log api.Logger, connections ConnectionSelector) (Balancer, error) {
	if connections.SubjectInterest != "" && connections.Account == "" {
		return nil, fmt.Errorf("can only filter by subject if account is given")
	}

	return &balancer{
		nc:       nc,
		duration: runTime,
		limits:   &connections,
		log:      log,
	}, nil
}

func (c *balancer) Balance(ctx context.Context) (int, error) {
	connz, err := c.getConnz(ctx)
	if err != nil {
		return 0, err
	}
	serverCount := len(connz)

	c.log.Debugf("Had %d connz responses", serverCount)

	matched, err := c.pickConnections(connz)
	if err != nil {
		return 0, err
	}

	c.log.Debugf("Matched %d connections", len(matched))

	if len(matched) == 0 {
		return 0, nil
	}

	grouped := c.groupConnectionsByServer(matched)

	totalConnections := len(matched)
	evenDist := c.calculateEvenDistribution(totalConnections, serverCount)

	c.calculateOffsets(grouped, evenDist)

	c.log.Infof("Connection distribution across %d servers (target: %d per server):", serverCount, evenDist)
	for _, server := range grouped {
		c.log.Infof("  Server '%s': %d connections (offset: %+d)",
			server.serverName, len(server.connections), server.offset)
	}

	toKick := c.selectConnectionsToKick(grouped)

	c.log.Debugf("Selected %d connections to kick (from %d matched)", len(toKick), len(matched))

	if len(toKick) == 0 {
		return 0, nil
	}

	var sleep = c.duration / time.Duration(len(toKick))
	var success int

	c.log.Infof("Balancing %d connections with %v sleep between each balance request", len(toKick), sleep)

	for i, m := range toKick {
		cid, err := c.nc.GetClientID()
		if err != nil {
			c.log.Errorf("Could not exclude self from kicks: %v", err)
			continue
		}

		if m.serverId == c.nc.ConnectedServerId() && m.conn.Cid == cid {
			c.log.Debugf("Not kicking own connection")
			continue
		}

		res, err := c.reqMany(ctx, fmt.Sprintf("$SYS.REQ.SERVER.%s.KICK", m.serverId), &server.KickClientReq{CID: m.conn.Cid}, 1)
		if err != nil {
			c.log.Errorf("Could not kick %d on %s: %v", m.conn.Cid, m.serverId, err)
			continue
		}

		if len(res) != 1 {
			c.log.Errorf("Could not kick %d on %s: expected 1 response but had %d", m.conn.Cid, m.serverId, len(res))
			continue
		}

		var resp server.ServerAPIResponse
		err = json.Unmarshal(res[0].Data, &resp)
		if err != nil {
			c.log.Errorf("Could not kick %d on %s: invalid server response: %v", m.conn.Cid, m.serverId, err)
			continue
		}

		if resp.Error != nil {
			c.log.Errorf("Could not kick %d on %s: invalid server response: %v", m.conn.Cid, m.serverId, resp.Error.Description)
			continue
		}

		name := m.conn.Name
		if name != "" {
			name = fmt.Sprintf(" (%s)", name)
		}

		if m.conn.Account != "" {
			c.log.Infof("Balanced client %d%s in account %s on %s", m.conn.Cid, name, m.conn.Account, m.serverName)
		} else {
			c.log.Infof("Balanced client %d%s on %s", m.conn.Cid, name, m.serverName)
		}

		success++

		if i != len(toKick)-1 {
			timer := time.NewTimer(sleep)
			c.log.Debugf("Sleeping for %v", sleep)
			select {
			case <-timer.C:
			case <-ctx.Done():
				break
			}
		}
	}

	return success, nil
}

func (c *balancer) pickConnections(connz []*server.ServerAPIConnzResponse) ([]*conn, error) {
	var result []*conn

	for _, resp := range connz {
		if resp.Data == nil {
			continue
		}

		for _, client := range resp.Data.Conns {
			isIdleMatch := c.limits.Idle == 0 || time.Since(client.LastActivity) > c.limits.Idle
			kindMatch := len(c.limits.Kind) == 0 || slices.Contains(c.limits.Kind, client.Kind)

			// others were handled in the connz request already
			if isIdleMatch && kindMatch {
				result = append(result, &conn{
					serverId:   resp.Server.ID,
					serverName: resp.Server.Name,
					conn:       client,
				})
			}
		}
	}

	return result, nil
}

func (c *balancer) groupConnectionsByServer(matched []*conn) map[string]*serverConnections {
	grouped := make(map[string]*serverConnections)

	for _, m := range matched {
		if _, exists := grouped[m.serverId]; !exists {
			grouped[m.serverId] = &serverConnections{
				serverId:    m.serverId,
				serverName:  m.serverName,
				connections: []*conn{},
			}
		}
		grouped[m.serverId].connections = append(grouped[m.serverId].connections, m)
	}

	return grouped
}

func (c *balancer) calculateEvenDistribution(totalConnections, serverCount int) int {
	if serverCount == 0 {
		return 0
	}

	return int(math.Ceil(float64(totalConnections) / float64(serverCount)))
}

func (c *balancer) calculateOffsets(grouped map[string]*serverConnections, evenDistribution int) {
	for _, server := range grouped {
		server.offset = len(server.connections) - evenDistribution
	}
}

func (c *balancer) selectConnectionsToKick(grouped map[string]*serverConnections) []*conn {
	var toKick []*conn

	for _, server := range grouped {
		if server.offset <= 0 {
			continue
		}

		kickCount := min(server.offset, len(server.connections))
		toKick = append(toKick, server.connections[:kickCount]...)
	}

	return toKick
}

func (c *balancer) getConnz(ctx context.Context) ([]*server.ServerAPIConnzResponse, error) {
	var (
		results    []*server.ServerAPIConnzResponse
		nextOffset int
		err        error
	)

	for {
		var z []*server.ServerAPIConnzResponse

		nextOffset, z, err = c.getConnzWithOffset(ctx, nextOffset)
		if err != nil {
			return nil, err
		}

		results = append(results, z...)

		if nextOffset == 0 {
			break
		}

		c.log.Infof("Gathering paged connection information")
	}

	return results, nil
}

func (c *balancer) getConnzWithOffset(ctx context.Context, offset int) (nextOffset int, results []*server.ServerAPIConnzResponse, err error) {
	req := &server.ConnzEventOptions{
		ConnzOptions: server.ConnzOptions{
			Account:       c.limits.Account,
			FilterSubject: c.limits.SubjectInterest,
			Offset:        offset,
		},
		EventFilterOptions: server.EventFilterOptions{
			Name:       c.limits.ServerName,
			Cluster:    c.limits.Cluster,
			ExactMatch: true,
		},
	}

	connz, err := c.reqMany(ctx, "$SYS.REQ.SERVER.PING.CONNZ", req, 0)
	if err != nil {
		return 0, nil, err
	}

	for _, msg := range connz {
		z, err := c.parseConnzMsg(msg)
		if err != nil {
			return 0, nil, err
		}

		if z.Data.Offset+z.Data.Limit < z.Data.Limit {
			if z.Data.Offset+z.Data.Limit+1 > nextOffset {
				nextOffset = z.Data.Offset + z.Data.Limit + 1
			}
		}

		results = append(results, z)
	}

	return nextOffset, results, nil
}

func (c *balancer) parseConnzMsg(msg *nats.Msg) (*server.ServerAPIConnzResponse, error) {
	reqresp := server.ServerAPIConnzResponse{}

	err := json.Unmarshal(msg.Data, &reqresp)
	if err != nil {
		return nil, err
	}

	if reqresp.Error != nil {
		return nil, fmt.Errorf("invalid response received: %v", reqresp.Error)
	}

	if reqresp.Data == nil {
		return nil, fmt.Errorf("no data received in response: %s", string(msg.Data))
	}

	return &reqresp, nil
}

func (c *balancer) reqMany(ctx context.Context, subj string, req any, expect int) ([]*nats.Msg, error) {
	jreq := []byte("{}")
	var err error

	if req != nil {
		switch val := req.(type) {
		case string:
			jreq = []byte(val)
		default:
			jreq, err = json.Marshal(req)
			if err != nil {
				return nil, err
			}
		}
	}

	c.log.Tracef(">>> %s: %s", subj, string(jreq))

	var (
		mu       sync.Mutex
		ctr      int
		finisher *time.Timer
		errs     = make(chan error)
		sub      *nats.Subscription
		res      []*nats.Msg
	)

	to, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if expect == 0 {
		finisher = time.NewTimer(2 * time.Second)
		go func(ctx context.Context, cancel context.CancelFunc, finisher *time.Timer) {
			select {
			case <-finisher.C:
				cancel()
			case <-ctx.Done():
				return
			}
		}(to, cancel, finisher)
	}

	sub, err = c.nc.Subscribe(c.nc.NewRespInbox(), func(m *nats.Msg) {
		mu.Lock()
		defer mu.Unlock()

		c.log.Tracef("<<< (%dB) %s", len(m.Data), string(m.Data))
		if finisher != nil {
			finisher.Reset(300 * time.Millisecond)
		}

		if m.Header.Get("Status") == "503" {
			errs <- nats.ErrNoResponders
			return
		}

		res = append(res, m)
		ctr++

		if expect > 0 && ctr == expect {
			cancel()
		}
	})
	if err != nil {
		return nil, err
	}

	if expect > 0 {
		sub.AutoUnsubscribe(expect)
	}

	msg := nats.NewMsg(subj)
	msg.Reply = sub.Subject
	msg.Data = jreq

	err = c.nc.PublishMsg(msg)
	if err != nil {
		return nil, err
	}

	select {
	case err = <-errs:
		if errors.Is(err, nats.ErrNoResponders) {
			return nil, fmt.Errorf("server request failed, ensure the account used has system privileges and appropriate permissions")
		}

		return nil, err
	case <-to.Done():
		sub.Unsubscribe()
	}

	mu.Lock()
	c.log.Debugf("=== Received %d responses", ctr)
	mu.Unlock()

	return res, nil
}
