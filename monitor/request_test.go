// Copyright 2025 The NATS Authors
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
	"regexp"
	"testing"
	"time"

	"github.com/nats-io/jsm.go/monitor"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func TestCheckRequest(t *testing.T) {
	t.Run("Body match", func(t *testing.T) {
		withJetStream(t, func(srv *server.Server, nc *nats.Conn) {
			check := &monitor.Result{}

			_, err := nc.Subscribe("test", func(msg *nats.Msg) {
				msg.Respond([]byte("test payload"))
			})
			assertNoError(t, err)

			assertNoError(t, monitor.CheckRequest(srv.ClientURL(), nil, check, time.Second, monitor.CheckRequestOptions{
				Subject:       "test",
				ResponseMatch: "no match",
			}))
			assertListIsEmpty(t, check.OKs)
			assertListIsEmpty(t, check.Warnings)
			assertListEquals(t, check.Criticals, "response does not match regexp")

			check = &monitor.Result{}
			assertNoError(t, monitor.CheckRequest(srv.ClientURL(), nil, check, time.Second, monitor.CheckRequestOptions{
				Subject:       "test",
				ResponseMatch: ".+payload",
			}))
			assertListIsEmpty(t, check.Criticals)
			assertListIsEmpty(t, check.Warnings)
			assertListEquals(t, check.OKs, "Valid response")
		})
	})

	t.Run("Headers", func(t *testing.T) {
		withJetStream(t, func(srv *server.Server, nc *nats.Conn) {
			check := &monitor.Result{}

			_, err := nc.Subscribe("test", func(msg *nats.Msg) {
				rmsg := nats.NewMsg(msg.Reply)
				rmsg.Header.Add("test", "test header")
				msg.RespondMsg(rmsg)
			})
			assertNoError(t, err)

			assertNoError(t, monitor.CheckRequest(srv.ClientURL(), nil, check, time.Second, monitor.CheckRequestOptions{
				Subject:     "test",
				HeaderMatch: map[string]string{"test": "no match", "other": "header"},
			}))
			assertListIsEmpty(t, check.OKs)
			assertListIsEmpty(t, check.Warnings)
			assertListEquals(t, check.Criticals, `invalid header "other" = ""`, `invalid header "test" = "test header"`)

			check = &monitor.Result{}
			assertNoError(t, monitor.CheckRequest(srv.ClientURL(), nil, check, time.Second, monitor.CheckRequestOptions{
				Subject:     "test",
				HeaderMatch: map[string]string{"test": "test header"},
			}))
			assertListIsEmpty(t, check.Criticals)
			assertListIsEmpty(t, check.Warnings)
			assertListEquals(t, check.OKs, "Valid response")
		})
	})

	t.Run("Response Time", func(t *testing.T) {
		withJetStream(t, func(srv *server.Server, nc *nats.Conn) {
			check := &monitor.Result{}
			_, err := nc.Subscribe("test", func(msg *nats.Msg) {
				time.Sleep(500 * time.Millisecond)
				msg.Respond([]byte("test payload"))
			})
			assertNoError(t, err)

			assertNoError(t, monitor.CheckRequest(srv.ClientURL(), nil, check, time.Second, monitor.CheckRequestOptions{
				Subject:              "test",
				ResponseTimeWarn:     0.2,
				ResponseTimeCritical: 1,
			}))
			assertListIsEmpty(t, check.Criticals)
			assertListIsEmpty(t, check.OKs)
			if len(check.Warnings) != 1 {
				t.Fatalf("expected 1 warning, got %d", len(check.Warnings))
			}
			m, err := regexp.MatchString("^response took \\d+ms", check.Warnings[0])
			assertNoError(t, err)
			if !m {
				t.Fatalf("warning not match %s", check.Warnings[0])
			}

			check = &monitor.Result{}
			assertNoError(t, monitor.CheckRequest(srv.ClientURL(), nil, check, time.Second, monitor.CheckRequestOptions{
				Subject:              "test",
				ResponseTimeWarn:     0.2,
				ResponseTimeCritical: 0.4,
			}))
			assertListIsEmpty(t, check.Warnings)
			assertListIsEmpty(t, check.OKs)
			if len(check.Criticals) != 1 {
				t.Fatalf("expected 1 warning, got %d", len(check.Criticals))
			}
			m, err = regexp.MatchString("^response took \\d+ms", check.Criticals[0])
			assertNoError(t, err)
			if !m {
				t.Fatalf("warning not match %s", check.Criticals[0])
			}

			check = &monitor.Result{}
			assertNoError(t, monitor.CheckRequest(srv.ClientURL(), nil, check, time.Second, monitor.CheckRequestOptions{
				Subject:              "test",
				ResponseTimeWarn:     0.8,
				ResponseTimeCritical: 1,
			}))
			assertListIsEmpty(t, check.Warnings)
			assertListIsEmpty(t, check.Criticals)
			assertListEquals(t, check.OKs, "Valid response")
		})
	})
}
