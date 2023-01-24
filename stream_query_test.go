// Copyright 2022 The NATS Authors
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

package jsm_test

import (
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	natsd "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func checkStreamQueryMatched(t *testing.T, mgr *jsm.Manager, expect int, opts ...jsm.StreamQueryOpt) {
	t.Helper()

	matched, err := mgr.QueryStreams(opts...)
	checkErr(t, err, "query failed")
	if len(matched) != expect {
		t.Fatalf("expected %d matched, got %d", expect, len(matched))
	}
}

func TestStreamQueryCreatePeriod(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, nc *nats.Conn, mgr *jsm.Manager) {
		_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.Replicas(2))
		checkErr(t, err, "create failed")

		checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryOlderThan(time.Hour))
		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryOlderThan(time.Hour), jsm.StreamQueryInvert())

		time.Sleep(50 * time.Millisecond)

		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryOlderThan(10*time.Millisecond))
		checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryOlderThan(10*time.Millisecond), jsm.StreamQueryInvert())
	})
}

func TestStreamQueryReplicas(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, nc *nats.Conn, mgr *jsm.Manager) {
		_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.Replicas(2))
		checkErr(t, err, "create failed")
		_, err = mgr.NewStream("q2", jsm.Subjects("in.q2"), jsm.MemoryStorage(), jsm.Replicas(1))
		checkErr(t, err, "create failed")

		checkStreamQueryMatched(t, mgr, 2, jsm.StreamQueryReplicas(1))
		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryReplicas(2))
		checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryReplicas(3))

		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryReplicas(1), jsm.StreamQueryInvert())
		checkStreamQueryMatched(t, mgr, 2, jsm.StreamQueryReplicas(2), jsm.StreamQueryInvert())
		checkStreamQueryMatched(t, mgr, 2, jsm.StreamQueryReplicas(3), jsm.StreamQueryInvert())
	})
}

func TestStreamQueryIdlePeriod(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, nc *nats.Conn, mgr *jsm.Manager) {
		_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.Replicas(2))
		checkErr(t, err, "create failed")

		t.Run("Without consumers", func(t *testing.T) {
			_, err = nc.Request("in.q1", []byte("hello"), time.Second)
			checkErr(t, err, "req failed")

			time.Sleep(100 * time.Millisecond)

			// nothing is idle for less than a milli
			checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryIdleLongerThan(time.Millisecond))
			checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryIdleLongerThan(time.Millisecond), jsm.StreamQueryInvert())

			// but they are idle for less than 10 seconds
			checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryIdleLongerThan(10*time.Second))
			checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryIdleLongerThan(10*time.Second), jsm.StreamQueryInvert())
		})

		t.Run("With consumers", func(t *testing.T) {
			_, err = nc.Request("in.q1", []byte("hello"), time.Second)
			checkErr(t, err, "req failed")

			// its not idle as its had a message now
			checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryIdleLongerThan(100*time.Millisecond))
			checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryIdleLongerThan(100*time.Millisecond), jsm.StreamQueryInvert())

			time.Sleep(500 * time.Millisecond)

			// now its all idle
			checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryIdleLongerThan(100*time.Millisecond))
			checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryIdleLongerThan(100*time.Millisecond), jsm.StreamQueryInvert())

			_, err = nc.Request("in.q1", []byte("hello"), time.Second)
			checkErr(t, err, "req failed")
			checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryIdleLongerThan(100*time.Millisecond))
			checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryIdleLongerThan(100*time.Millisecond), jsm.StreamQueryInvert())
		})
	})
}

func TestStreamQueryEmpty(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, nc *nats.Conn, mgr *jsm.Manager) {
		_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.Replicas(2))
		checkErr(t, err, "create failed")

		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryWithoutMessages())
		checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryWithoutMessages(), jsm.StreamQueryInvert())

		_, err = nc.Request("in.q1", []byte("hello"), time.Second)
		checkErr(t, err, "req failed")

		checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryWithoutMessages())
		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryWithoutMessages(), jsm.StreamQueryInvert())
	})
}

func TestStreamQueryConsumersLimit(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, _ *nats.Conn, mgr *jsm.Manager) {
		_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.Replicas(2))
		checkErr(t, err, "create failed")

		t.Run("Less than n consumers", func(t *testing.T) {
			checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryFewerConsumersThan(10))
			checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryFewerConsumersThan(10), jsm.StreamQueryInvert())
		})
	})
}

func TestStreamQueryCluster(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, _ *nats.Conn, mgr *jsm.Manager) {
		_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.Replicas(3))
		checkErr(t, err, "create failed")

		t.Run("does not match the cluster", func(t *testing.T) {
			checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryClusterName("foo"))
			checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryClusterName("foo"), jsm.StreamQueryInvert())
		})

		t.Run("Regex match the clusteR", func(t *testing.T) {
			checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryClusterName("T.+T"))
			checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryClusterName("T.+T"), jsm.StreamQueryInvert())
		})

		t.Run("Combining matchers, 1 excluding all results", func(t *testing.T) {
			checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryServerName("s1"), jsm.StreamQueryClusterName("OTHER"))
			checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryInvert(), jsm.StreamQueryServerName("o1"), jsm.StreamQueryClusterName("OTHER"))
		})
	})
}

func TestStreamQueryServer(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, _ *nats.Conn, mgr *jsm.Manager) {
		stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1", "in.q1.other"), jsm.MemoryStorage(), jsm.Replicas(2))
		checkErr(t, err, "create failed")

		nfo, err := stream.LatestInformation()
		checkErr(t, err, "info failed")

		checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryServerName("foo"))
		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryServerName("foo"), jsm.StreamQueryInvert())

		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQueryServerName(nfo.Cluster.Leader))
		checkStreamQueryMatched(t, mgr, 0, jsm.StreamQueryServerName(nfo.Cluster.Leader), jsm.StreamQueryInvert())
	})
}

func TestStreamSubjectWildcardMatch(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, _ *nats.Conn, mgr *jsm.Manager) {
		_, err := mgr.NewStream("q1", jsm.Subjects("in.q1", "in.q1.other"), jsm.MemoryStorage(), jsm.Replicas(2))
		checkErr(t, err, "create failed")

		checkStreamQueryMatched(t, mgr, 0, jsm.StreamQuerySubjectWildcard("foo"))
		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQuerySubjectWildcard("foo"), jsm.StreamQueryInvert())

		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQuerySubjectWildcard("in.>"))
		checkStreamQueryMatched(t, mgr, 0, jsm.StreamQuerySubjectWildcard("in.>"), jsm.StreamQueryInvert())

		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQuerySubjectWildcard("in.*"))
		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQuerySubjectWildcard("in.*"), jsm.StreamQueryInvert())

		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQuerySubjectWildcard("in.*.*"))
		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQuerySubjectWildcard("in.*.*"), jsm.StreamQueryInvert())

		checkStreamQueryMatched(t, mgr, 0, jsm.StreamQuerySubjectWildcard("in.*.*.>"))
		checkStreamQueryMatched(t, mgr, 1, jsm.StreamQuerySubjectWildcard("in.*.*.>"), jsm.StreamQueryInvert())
	})
}
