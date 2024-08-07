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

package monitor_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/monitor"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func TestCheckMessage(t *testing.T) {
	t.Run("Body timestamp", func(t *testing.T) {
		withJetStream(t, func(srv *server.Server, nc *nats.Conn) {
			check := &monitor.Result{}

			mgr, err := jsm.New(nc)
			assertNoError(t, err)

			_, err = mgr.NewStream("TEST")
			checkErr(t, err, "stream create failed: %v", err)

			opts := monitor.CheckStreamMessageOptions{
				StreamName:      "TEST",
				Subject:         "TEST",
				AgeCritical:     5,
				AgeWarning:      1,
				BodyAsTimestamp: true,
			}
			assertNoError(t, monitor.CheckStreamMessage(srv.ClientURL(), nil, check, opts))
			assertListIsEmpty(t, check.Warnings)
			assertListIsEmpty(t, check.OKs)
			assertListEquals(t, check.Criticals, "no message found")

			now := time.Now().Unix()
			_, err = nc.Request("TEST", []byte(strconv.Itoa(int(now))), time.Second)
			checkErr(t, err, "publish failed: %v", err)

			check = &monitor.Result{}
			assertNoError(t, monitor.CheckStreamMessage(srv.ClientURL(), nil, check, opts))
			assertListIsEmpty(t, check.Warnings)
			assertListIsEmpty(t, check.Criticals)
			assertListEquals(t, check.OKs, "Valid message on TEST > TEST")

			now = time.Now().Add(-2 * time.Second).Unix()
			_, err = nc.Request("TEST", []byte(strconv.Itoa(int(now))), time.Second)
			checkErr(t, err, "publish failed: %v", err)

			check = &monitor.Result{}
			assertNoError(t, monitor.CheckStreamMessage(srv.ClientURL(), nil, check, opts))
			assertListIsEmpty(t, check.Criticals)
			if len(check.Warnings) != 1 {
				t.Fatalf("expected 1 warning got: %v", check.Warnings)
			}

			now = time.Now().Add(-6 * time.Second).Unix()
			_, err = nc.Request("TEST", []byte(strconv.Itoa(int(now))), time.Second)
			checkErr(t, err, "publish failed: %v", err)

			check = &monitor.Result{}
			assertNoError(t, monitor.CheckStreamMessage(srv.ClientURL(), nil, check, opts))
			assertListIsEmpty(t, check.Warnings)
			if len(check.Criticals) != 1 {
				t.Fatalf("expected 1 critical got: %v", check.Criticals)
			}

			opts.BodyAsTimestamp = false
			check = &monitor.Result{}
			assertNoError(t, monitor.CheckStreamMessage(srv.ClientURL(), nil, check, opts))
			assertListIsEmpty(t, check.Warnings)
			assertListIsEmpty(t, check.Criticals)
			assertListEquals(t, check.OKs, "Valid message on TEST > TEST")
		})
	})
}
