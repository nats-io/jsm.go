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
	"bytes"
	"time"

	"github.com/nats-io/nats.go"
)

// CheckConnectionOptions configures the NATS Connection check
type CheckConnectionOptions struct {
	// ConnectTimeWarning warning threshold for time to establish the connection (seconds)
	ConnectTimeWarning float64 `json:"connect_time_warning" yaml:"connect_time_warning"`
	// ConnectTimeCritical critical threshold for time to establish the connection (seconds)
	ConnectTimeCritical float64 `json:"connect_time_critical" yaml:"connect_time_critical"`
	// ServerRttWarning warning threshold for the connection rtt check (seconds)
	ServerRttWarning float64 `json:"server_rtt_warning" yaml:"server_rtt_warning"`
	// ServerRttCritical critical threshold for the connection rtt check (seconds)
	ServerRttCritical float64 `json:"server_rtt_critical" yaml:"server_rtt_critical"`
	// RequestRttWarning warning threshold for the request-respond rtt check (seconds)
	RequestRttWarning float64 `json:"request_rtt_warning" yaml:"request_rtt_warning"`
	// RequestRttCritical critical threshold for the request-respond rtt check (seconds)
	RequestRttCritical float64 `json:"request_rtt_critical" yaml:"request_rtt_critical"`
}

func CheckConnection(server string, nopts []nats.Option, timeout time.Duration, check *Result, opts CheckConnectionOptions) error {
	connStart := time.Now()
	nc, err := nats.Connect(server, nopts...)
	if check.CriticalIfErrf(err, "connection failed: %v", err) {
		return nil
	}
	defer nc.Close()

	ct := time.Since(connStart)
	check.Pd(&PerfDataItem{Name: "connect_time", Value: ct.Seconds(), Warn: opts.ConnectTimeWarning, Crit: opts.ConnectTimeCritical, Unit: "s", Help: "Time taken to connect to NATS"})

	if ct >= time.Duration(opts.ConnectTimeCritical*float64(time.Second)) {
		check.Criticalf("connected to %s, connect time exceeded %v", nc.ConnectedUrl(), opts.ConnectTimeCritical)
	} else if ct >= time.Duration(opts.ConnectTimeWarning*float64(time.Second)) {
		check.Warnf("connected to %s, connect time exceeded %v", nc.ConnectedUrl(), opts.ConnectTimeWarning)
	} else {
		check.Okf("connected to %s in %s", nc.ConnectedUrl(), ct)
	}

	rtt, err := nc.RTT()
	check.CriticalIfErrf(err, "rtt failed: %s", err)

	check.Pd(&PerfDataItem{Name: "rtt", Value: rtt.Seconds(), Warn: opts.ServerRttWarning, Crit: opts.ServerRttCritical, Unit: "s", Help: "The round-trip-time of the connection"})
	if rtt >= time.Duration(opts.ServerRttCritical*float64(time.Second)) {
		check.Criticalf("rtt time exceeded %v", opts.ServerRttCritical)
	} else if rtt >= time.Duration(opts.ServerRttWarning*float64(time.Second)) {
		check.Criticalf("rtt time exceeded %v", opts.ServerRttWarning)
	} else {
		check.Okf("rtt time %v", rtt)
	}

	msg := []byte(randomPassword(100))
	ib := nc.NewRespInbox()
	sub, err := nc.SubscribeSync(ib)
	check.CriticalIfErrf(err, "could not subscribe to %s: %s", ib, err)
	sub.AutoUnsubscribe(1)

	start := time.Now()
	err = nc.Publish(ib, msg)
	check.CriticalIfErrf(err, "could not publish to %s: %s", ib, err)

	received, err := sub.NextMsg(timeout)
	check.CriticalIfErrf(err, "did not receive from %s: %s", ib, err)

	reqt := time.Since(start)
	check.Pd(&PerfDataItem{Name: "request_time", Value: reqt.Seconds(), Warn: opts.RequestRttWarning, Crit: opts.RequestRttCritical, Unit: "s", Help: "Time taken for a full Request-Reply operation"})

	if !bytes.Equal(received.Data, msg) {
		check.Critical("did not receive expected message")
	}

	if reqt >= time.Duration(opts.RequestRttCritical*float64(time.Second)) {
		check.Criticalf("round trip request took %f", reqt.Seconds())
	} else if reqt >= time.Duration(opts.RequestRttWarning*float64(time.Second)) {
		check.Warnf("round trip request took %f", reqt.Seconds())
	} else {
		check.Okf("round trip took %fs", reqt.Seconds())
	}

	return nil
}
