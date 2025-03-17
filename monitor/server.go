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

// CheckServerOptions configures the server check
type CheckServerOptions struct {
	// Name is the server to get details for
	Name string `json:"name" yaml:"name"`
	// CPUWarning is the warning threshold for CPU usage
	CPUWarning int `json:"cpu_warning" yaml:"cpu_warning"`
	// CPUCritical is the critical threshold for CPU usage
	CPUCritical int `json:"cpu_critical" yaml:"cpu_critical"`
	// MemoryWarning is the warning threshold for Memory usage
	MemoryWarning int `json:"memory_warning" yaml:"memory_warning"`
	// MemoryCritical is the critical threshold for Memory usage
	MemoryCritical int `json:"memory_critical" yaml:"memory_critical"`
	// ConnectionsWarning is the warning threshold for how many connections the server should have
	ConnectionsWarning int `json:"connections_warning" yaml:"connections_warning"`
	// ConnectionsCritical is the critical threshold for how many connections the server should have
	ConnectionsCritical int `json:"connections_critical" yaml:"connections_critical"`
	// SubscriptionsWarning is the warning threshold for how many subscriptions the server should have
	SubscriptionsWarning int `json:"subscriptions_warning" yaml:"subscriptions_warning"`
	// SubscriptionsCritical is the critical threshold for how many subscriptions the server should have
	SubscriptionsCritical int `json:"subscriptions_critical" yaml:"subscriptions_critical"`
	// UptimeWarning is warning threshold for the uptime of the server (seconds)
	UptimeWarning float64 `json:"uptime_warning" yaml:"uptime_warning"`
	// UptimeCritical is warning threshold for the uptime of the server (seconds)
	UptimeCritical float64 `json:"uptime_critical" yaml:"uptime_critical"`
	// AuthenticationRequired checks if authentication is required on the server
	AuthenticationRequired bool `json:"authentication_required" yaml:"authentication_required"`
	// TLSRequired checks if TLS is required on the server
	TLSRequired bool `json:"tls_required" yaml:"tls_required"`
	// JetStreamRequired checks if JetStream is enabled on the server
	JetStreamRequired bool `json:"jet_stream_required" yaml:"jet_stream_required"`

	Resolver func(nc *nats.Conn, name string, timeout time.Duration) (*server.Varz, error) `json:"-" yaml:"-"`
}

func CheckServer(server string, nopts []nats.Option, check *Result, timeout time.Duration, opts CheckServerOptions) error {
	var nc *nats.Conn
	var err error

	if opts.Resolver == nil {
		nc, err = nats.Connect(server, nopts...)
		if check.CriticalIfErr(err, "connection failed: %v", err) {
			return nil
		}
		defer nc.Close()
	}

	return CheckServerWithConnection(nc, check, timeout, opts)
}

func CheckServerWithConnection(nc *nats.Conn, check *Result, timeout time.Duration, opts CheckServerOptions) error {
	var err error

	if opts.Resolver == nil {
		opts.Resolver = fetchVarz
	}

	vz, err := opts.Resolver(nc, opts.Name, timeout)
	if check.CriticalIfErr(err, "varz failed: %v", err) {
		return nil
	}

	if vz == nil {
		check.Critical("no data received")
		return nil
	}

	if vz.Name != opts.Name {
		check.Critical("result from wrong server %q", vz.Name)
	}

	if opts.JetStreamRequired {
		if vz.JetStream.Config == nil {
			check.Critical("JetStream not enabled")
		} else {
			check.Ok("JetStream enabled")
		}
	}

	if opts.TLSRequired {
		if vz.TLSRequired {
			check.Ok("TLS required")
		} else {
			check.Critical("TLS not required")
		}
	}

	if opts.AuthenticationRequired {
		if vz.AuthRequired {
			check.Ok("Authentication required")
		} else {
			check.Critical("Authentication not required")
		}
	}

	up := vz.Now.Sub(vz.Start)
	if opts.UptimeWarning > 0 || opts.UptimeCritical > 0 {
		if opts.UptimeCritical > opts.UptimeWarning {
			check.Critical("Up invalid thresholds")
			return nil
		}

		if up <= secondsToDuration(opts.UptimeCritical) {
			check.Critical("Up %s", f(up))
		} else if up <= secondsToDuration(opts.UptimeWarning) {
			check.Warn("Up %s", f(up))
		} else {
			check.Ok("Up %s", f(up))
		}
	}

	check.Pd(
		&PerfDataItem{Name: "uptime", Value: up.Seconds(), Warn: opts.UptimeWarning, Crit: opts.UptimeCritical, Unit: "s", Help: "NATS Server uptime in seconds"},
		&PerfDataItem{Name: "cpu", Value: vz.CPU, Warn: float64(opts.CPUWarning), Crit: float64(opts.CPUCritical), Unit: "%", Help: "NATS Server CPU usage in percentage"},
		&PerfDataItem{Name: "mem", Value: float64(vz.Mem), Warn: float64(opts.MemoryWarning), Crit: float64(opts.MemoryCritical), Help: "NATS Server memory usage in bytes"},
		&PerfDataItem{Name: "connections", Value: float64(vz.Connections), Warn: float64(opts.ConnectionsWarning), Crit: float64(opts.ConnectionsCritical), Help: "Active connections"},
		&PerfDataItem{Name: "subscriptions", Value: float64(vz.Subscriptions), Warn: float64(opts.SubscriptionsWarning), Crit: float64(opts.SubscriptionsCritical), Help: "Active subscriptions"},
	)

	checkVal := func(name string, crit float64, warn float64, value float64, r bool) {
		if crit == 0 && warn == 0 {
			return
		}

		if !r && crit < warn {
			check.Critical("%s invalid thresholds", name)
			return
		}

		if r && crit < warn {
			if value <= crit {
				check.Critical("%s %.2f", name, value)
			} else if value <= warn {
				check.Warn("%s %.2f", name, value)
			} else {
				check.Ok("%s %.2f", name, value)
			}
		} else {
			if value >= crit {
				check.Critical("%s %.2f", name, value)
			} else if value >= warn {
				check.Warn("%s %.2f", name, value)
			} else {
				check.Ok("%s %.2f", name, value)
			}
		}
	}

	checkVal("CPU", float64(opts.CPUCritical), float64(opts.CPUWarning), vz.CPU, false)
	checkVal("Memory", float64(opts.MemoryCritical), float64(opts.MemoryWarning), float64(vz.Mem), false)
	checkVal("Connections", float64(opts.ConnectionsCritical), float64(opts.ConnectionsWarning), float64(vz.Connections), true)
	checkVal("Subscriptions", float64(opts.SubscriptionsCritical), float64(opts.SubscriptionsWarning), float64(vz.Subscriptions), true)

	return nil
}

func fetchVarz(nc *nats.Conn, name string, timeout time.Duration) (*server.Varz, error) {
	if name == "" {
		return nil, fmt.Errorf("server name is required")
	}

	req, err := json.Marshal(server.VarzEventOptions{EventFilterOptions: server.EventFilterOptions{Name: name}})
	if err != nil {
		return nil, err
	}

	res, err := nc.Request("$SYS.REQ.SERVER.PING.VARZ", req, timeout)
	if err != nil {
		return nil, err
	}

	resp := &server.ServerAPIVarzResponse{}
	err = json.Unmarshal(res.Data, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("invalid response received: %#v", resp.Error.Error())
	}

	if resp.Data == nil {
		return nil, fmt.Errorf("no data received for %s", name)
	}

	return resp.Data, nil
}
