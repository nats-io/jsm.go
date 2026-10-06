// Copyright 2026 The NATS Authors
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

package monitor_test

import (
	"testing"
	"time"

	"github.com/nats-io/jsm.go/monitor"
	"github.com/nats-io/jsm.go/test"
	"github.com/nats-io/nats.go"
	ntfclient "github.com/synadia-io/orbit.go/ntf-client"
)

func withServer(t *testing.T, cb func(url string)) {
	t.Helper()

	ntfc := ntfclient.New(t, test.NTFURL())
	defer ntfc.Close(t)
	ntfc.WithServer(t, func(_ testing.TB, _ *nats.Conn, instance *ntfclient.Instance) {
		cb(instance.Servers[0].URL)
	})
}

func TestCheckConnection(t *testing.T) {
	t.Run("bad server", func(t *testing.T) {
		check := &monitor.Result{}
		err := monitor.CheckConnection("nats://127.0.0.1:1", nil, time.Second, check, monitor.CheckConnectionOptions{})
		assertNoError(t, err)
		assertListIsEmpty(t, check.Warnings)
		assertListIsEmpty(t, check.OKs)
		if len(check.Criticals) == 0 {
			t.Fatal("expected critical for bad server, got none")
		}
	})

	t.Run("zero thresholds do not trigger", func(t *testing.T) {
		withServer(t, func(url string) {
			check := &monitor.Result{}
			err := monitor.CheckConnection(url, nil, 5*time.Second, check, monitor.CheckConnectionOptions{})
			assertNoError(t, err)
			assertListIsEmpty(t, check.Criticals)
			assertListIsEmpty(t, check.Warnings)
			if len(check.OKs) == 0 {
				t.Fatal("expected OK items with zero thresholds, got none")
			}
		})
	})

	t.Run("connect time critical", func(t *testing.T) {
		withServer(t, func(url string) {
			check := &monitor.Result{}
			err := monitor.CheckConnection(url, nil, 5*time.Second, check, monitor.CheckConnectionOptions{
				ConnectTimeCritical: 0.000001, // 1 µs — always exceeded
				ConnectTimeWarning:  0.0000001,
			})
			assertNoError(t, err)
			if len(check.Criticals) == 0 {
				t.Fatal("expected critical for connect time, got none")
			}
		})
	})

	t.Run("connect time warning not critical", func(t *testing.T) {
		withServer(t, func(url string) {
			check := &monitor.Result{}
			err := monitor.CheckConnection(url, nil, 5*time.Second, check, monitor.CheckConnectionOptions{
				ConnectTimeCritical: 3600,     // 1 hour — never exceeded
				ConnectTimeWarning:  0.000001, // 1 µs — always exceeded
			})
			assertNoError(t, err)
			assertListIsEmpty(t, check.Criticals)
			if len(check.Warnings) == 0 {
				t.Fatal("expected warning for connect time, got none")
			}
		})
	})

	t.Run("rtt critical", func(t *testing.T) {
		withServer(t, func(url string) {
			check := &monitor.Result{}
			err := monitor.CheckConnection(url, nil, 5*time.Second, check, monitor.CheckConnectionOptions{
				ServerRttCritical: 0.000001, // 1 µs — always exceeded
				ServerRttWarning:  0.0000001,
			})
			assertNoError(t, err)
			if len(check.Criticals) == 0 {
				t.Fatal("expected critical for rtt, got none")
			}
		})
	})

	t.Run("rtt warning not critical", func(t *testing.T) {
		withServer(t, func(url string) {
			check := &monitor.Result{}
			err := monitor.CheckConnection(url, nil, 5*time.Second, check, monitor.CheckConnectionOptions{
				ServerRttCritical: 3600,     // 1 hour — never exceeded
				ServerRttWarning:  0.000001, // 1 µs — always exceeded
			})
			assertNoError(t, err)
			assertListIsEmpty(t, check.Criticals)
			if len(check.Warnings) == 0 {
				t.Fatal("expected warning for rtt, got none")
			}
		})
	})

	t.Run("request rtt critical", func(t *testing.T) {
		withServer(t, func(url string) {
			check := &monitor.Result{}
			err := monitor.CheckConnection(url, nil, 5*time.Second, check, monitor.CheckConnectionOptions{
				RequestRttCritical: 0.000001, // 1 µs — always exceeded
				RequestRttWarning:  0.0000001,
			})
			assertNoError(t, err)
			if len(check.Criticals) == 0 {
				t.Fatal("expected critical for request rtt, got none")
			}
		})
	})

	t.Run("request rtt warning not critical", func(t *testing.T) {
		withServer(t, func(url string) {
			check := &monitor.Result{}
			err := monitor.CheckConnection(url, nil, 5*time.Second, check, monitor.CheckConnectionOptions{
				RequestRttCritical: 3600,     // 1 hour — never exceeded
				RequestRttWarning:  0.000001, // 1 µs — always exceeded
			})
			assertNoError(t, err)
			assertListIsEmpty(t, check.Criticals)
			if len(check.Warnings) == 0 {
				t.Fatal("expected warning for request rtt, got none")
			}
		})
	})

	t.Run("all ok with large thresholds", func(t *testing.T) {
		withServer(t, func(url string) {
			check := &monitor.Result{}
			err := monitor.CheckConnection(url, nil, 5*time.Second, check, monitor.CheckConnectionOptions{
				ConnectTimeCritical: 3600,
				ConnectTimeWarning:  3600,
				ServerRttCritical:   3600,
				ServerRttWarning:    3600,
				RequestRttCritical:  3600,
				RequestRttWarning:   3600,
			})
			assertNoError(t, err)
			assertListIsEmpty(t, check.Criticals)
			assertListIsEmpty(t, check.Warnings)
			if len(check.OKs) == 0 {
				t.Fatal("expected OK items, got none")
			}
		})
	})
}
