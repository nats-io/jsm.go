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
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type CheckMetaOptions struct {
	ExpectServers int           `json:"expect_servers" yaml:"expect_servers"`
	LagCritical   uint64        `json:"lag_critical" yaml:"lag_critical"`
	SeenCritical  time.Duration `json:"seen_critical" yaml:"seen_critical"`

	Resolver func(conn *nats.Conn) (*JSZResponse, error) `json:"-" yaml:"-"`
}

type JSZResponse struct {
	Data   server.JSInfo     `json:"data"`
	Server server.ServerInfo `json:"server"`
}

func CheckJetstreamMeta(nc *nats.Conn, check *Result, opts CheckMetaOptions) error {
	if opts.Resolver == nil {
		opts.Resolver = func(conn *nats.Conn) (*JSZResponse, error) {
			jszresp := &JSZResponse{}
			jreq, err := json.Marshal(&server.JSzOptions{LeaderOnly: true})
			if check.CriticalIfErr(err, "request failed: %v", err) {
				return nil, nil
			}

			res, err := nc.Request("$SYS.REQ.SERVER.PING.JSZ", jreq, time.Second)
			if check.CriticalIfErr(err, "JSZ API request failed: %s", err) {
				return nil, nil
			}

			err = json.Unmarshal(res.Data, jszresp)
			check.CriticalIfErr(err, "invalid result received: %s", err)

			return jszresp, nil
		}
	}

	jszresp, err := opts.Resolver(nc)
	if err != nil {
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
		if peer.Active > opts.SeenCritical {
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
		check.Critical("%d inactive more than %s", inactive, opts.SeenCritical)
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
