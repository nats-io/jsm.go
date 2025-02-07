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

package monitor

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type CheckJetstreamMetaOptions struct {
	// ExpectServers the expected number of known servers in the meta cluster
	ExpectServers int `json:"expect_servers" yaml:"expect_servers"`
	// LagCritical the critical threshold for how many operations behind a peer may be
	LagCritical uint64 `json:"lag_critical" yaml:"lag_critical"`
	// SeenCritical the critical threshold for how long ago a peer was seen (seconds)
	SeenCritical float64 `json:"seen_critical" yaml:"seen_critical"`

	Resolver func(*nats.Conn) (*server.ServerAPIJszResponse, error) `json:"-" yaml:"-"`
}

func CheckJetstreamMeta(servers string, nopts []nats.Option, check *Result, opts CheckJetstreamMetaOptions) error {
	var nc *nats.Conn
	var err error

	if opts.Resolver == nil {
		nc, err = nats.Connect(servers, nopts...)
		if check.CriticalIfErr(err, "connection failed: %v", err) {
			return nil
		}

		opts.Resolver = func(conn *nats.Conn) (*server.ServerAPIJszResponse, error) {
			jszresp := &server.ServerAPIJszResponse{}
			jreq, err := json.Marshal(&server.JSzOptions{LeaderOnly: true})
			if check.CriticalIfErr(err, "request failed: %v", err) {
				return nil, err
			}

			res, err := nc.Request("$SYS.REQ.SERVER.PING.JSZ", jreq, time.Second)
			if check.CriticalIfErr(err, "JSZ API request failed: %s", err) {
				return nil, err
			}

			err = json.Unmarshal(res.Data, jszresp)
			if check.CriticalIfErr(err, "invalid result received: %s", err) {
				return nil, err
			}

			if jszresp.Error == nil {
				check.Critical("invalid result received: %s", jszresp.Error.Error())
				return nil, fmt.Errorf("invalid result received: %s", jszresp.Error.Error())
			}

			return jszresp, nil
		}
	}

	jszresp, err := opts.Resolver(nc)
	if err != nil {
		return nil
	}

	if jszresp.Data == nil {
		check.Critical("no JSZ response received")
		return nil
	}

	ci := jszresp.Data.Meta
	if ci == nil {
		check.Critical("no cluster information")
		return nil
	}

	if ci.Leader == "" {
		check.Critical("No leader")
		return nil
	}

	check.Pd(&PerfDataItem{
		Name:  "peers",
		Value: float64(len(ci.Replicas) + 1),
		Warn:  float64(opts.ExpectServers),
		Crit:  float64(opts.ExpectServers),
		Help:  "Configured RAFT peers",
	})

	if len(ci.Replicas)+1 != opts.ExpectServers {
		check.Critical("%d peers of expected %d", len(ci.Replicas)+1, opts.ExpectServers)
	}

	notCurrent := 0
	inactive := 0
	offline := 0
	lagged := 0
	for _, peer := range ci.Replicas {
		if !peer.Current {
			notCurrent++
		}
		if peer.Offline {
			offline++
		}
		if peer.Active > secondsToDuration(opts.SeenCritical) {
			inactive++
		}
		if peer.Lag > opts.LagCritical {
			lagged++
		}
	}

	check.Pd(
		&PerfDataItem{Name: "peer_offline", Value: float64(offline), Help: "Offline RAFT peers"},
		&PerfDataItem{Name: "peer_not_current", Value: float64(notCurrent), Help: "RAFT peers that are not current"},
		&PerfDataItem{Name: "peer_inactive", Value: float64(inactive), Help: "Inactive RAFT peers"},
		&PerfDataItem{Name: "peer_lagged", Value: float64(lagged), Help: "RAFT peers that are lagged more than configured threshold"},
	)

	if notCurrent > 0 {
		check.Critical("%d not current", notCurrent)
	}
	if inactive > 0 {
		check.Critical("%d inactive more than %s", inactive, secondsToDuration(opts.SeenCritical))
	}
	if offline > 0 {
		check.Critical("%d offline", offline)
	}
	if lagged > 0 {
		check.Critical("%d lagged more than %d ops", lagged, opts.LagCritical)
	}

	if len(check.Criticals) == 0 && len(check.Warnings) == 0 {
		check.Ok("%d peers led by %s", len(jszresp.Data.Meta.Replicas)+1, jszresp.Data.Meta.Leader)
	}

	return nil
}
