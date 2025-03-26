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

package test

import (
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	testapi "github.com/nats-io/jsm.go/test/testing_client/api"
	"github.com/nats-io/jsm.go/test/testing_client/srvtest"
)

func checkConsumerQueryMatched(t *testing.T, s *jsm.Stream, expect int, opts ...jsm.ConsumerQueryOpt) {
	t.Helper()

	matched, err := s.QueryConsumers(opts...)
	checkErr(t, err, "query failed")
	if len(matched) != expect {
		t.Fatalf("expected %d matched, got %d", expect, len(matched))
	}
}

func TestConsumerApiLevel(t *testing.T) {
	withTesterJetStreamCluster(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ []*testapi.ManagedServer) {
		s, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.Replicas(2))
		checkErr(t, err, "create failed")

		_, err = s.NewConsumer(jsm.PauseUntil(time.Now().Add(time.Hour)), jsm.DurableName("PAUSED"))
		checkErr(t, err, "create failed")

		_, err = s.NewConsumer(jsm.DurableName("OLD"))
		checkErr(t, err, "create failed")

		checkConsumerQueryMatched(t, s, 1, jsm.ConsumerQueryApiLevelMin(1))
		checkConsumerQueryMatched(t, s, 2, jsm.ConsumerQueryApiLevelMin(0))

		res, err := s.QueryConsumers(jsm.ConsumerQueryApiLevelMin(1))
		checkErr(t, err, "query failed")
		if res[0].Name() != "PAUSED" {
			t.Fatalf("did not match paused consumer")
		}

		res, err = s.QueryConsumers(jsm.ConsumerQueryApiLevelMin(1), jsm.ConsumerQueryInvert())
		checkErr(t, err, "query failed")
		if res[0].Name() != "OLD" {
			t.Fatalf("did not match unpaused consumer")
		}
	})
}
