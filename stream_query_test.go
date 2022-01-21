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
	"regexp"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	natsd "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func checkStreamQueryMatched(t *testing.T, expect int, mgr *jsm.Manager, q jsm.StreamQuery) {
	t.Helper()

	matched, err := mgr.QueryStreams(q)
	checkErr(t, err, "query failed")
	if len(matched) != expect {
		t.Fatalf("expected %d matched, got %d", expect, len(matched))
	}
}

func TestStreamQueryCreatePeriod(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, nc *nats.Conn, mgr *jsm.Manager) {
		_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.Replicas(2))
		checkErr(t, err, "create failed")

		p := time.Hour
		checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{CreatedPeriod: &p})
		checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{Invert: true, CreatedPeriod: &p})

		time.Sleep(50 * time.Millisecond)
		p = 10 * time.Millisecond
		checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{CreatedPeriod: &p})
		checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{Invert: true, CreatedPeriod: &p})
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

			p := time.Millisecond
			// nothing is idle for less than a milli
			checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{IdlePeriod: &p})
			checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{Invert: true, IdlePeriod: &p})

			p = 10 * time.Second
			// but they are idle for less than 10 seconds
			checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{IdlePeriod: &p})
			checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{Invert: true, IdlePeriod: &p})
		})

		t.Run("With consumers", func(t *testing.T) {
			cons, err := mgr.NewConsumer("q1", jsm.DurableName("PULL"))
			checkErr(t, err, "create failed")

			_, err = nc.Request("in.q1", []byte("hello"), time.Second)
			checkErr(t, err, "req failed")

			p := 100 * time.Millisecond

			// its not idle but consumers are idle, so should still not match
			checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{IdlePeriod: &p})
			checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{Invert: true, IdlePeriod: &p})

			time.Sleep(500 * time.Millisecond)

			// now its all idle
			checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{IdlePeriod: &p})
			checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{Invert: true, IdlePeriod: &p})

			_, err = cons.NextMsg()
			checkErr(t, err, "next failed")

			// now its not idle again
			checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{IdlePeriod: &p})
			checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{Invert: true, IdlePeriod: &p})
		})
	})
}

func TestStreamQueryEmpty(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, nc *nats.Conn, mgr *jsm.Manager) {
		_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.Replicas(2))
		checkErr(t, err, "create failed")

		tf := true

		checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{Empty: &tf})
		checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{Invert: true, Empty: &tf})

		_, err = nc.Request("in.q1", []byte("hello"), time.Second)
		checkErr(t, err, "req failed")

		checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{Empty: &tf})
		checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{Invert: true, Empty: &tf})
	})
}

func TestStreamQueryConsumersLimit(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, _ *nats.Conn, mgr *jsm.Manager) {
		_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.Replicas(2))
		checkErr(t, err, "create failed")

		limit := 10

		t.Run("Less than n consumers", func(t *testing.T) {
			checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{ConsumersLimit: &limit})
			checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{Invert: true, ConsumersLimit: &limit})
		})
	})
}

func TestStreamQueryCluster(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, _ *nats.Conn, mgr *jsm.Manager) {
		_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.Replicas(3))
		checkErr(t, err, "create failed")

		t.Run("does not match the cluster", func(t *testing.T) {
			checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{Cluster: regexp.MustCompile("foo")})
			checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{Invert: true, Cluster: regexp.MustCompile("foo")})
		})

		t.Run("Regex match the clusteR", func(t *testing.T) {
			checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{Cluster: regexp.MustCompile("T.+T")})
			checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{Invert: true, Cluster: regexp.MustCompile("T.+T")})
		})

		t.Run("Combining matchers, 1 excluding all results", func(t *testing.T) {
			checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{Server: regexp.MustCompile("s1"), Cluster: regexp.MustCompile("OTHER")})
			checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{Invert: true, Server: regexp.MustCompile("o1"), Cluster: regexp.MustCompile("OTHER")})
		})
	})
}

func TestStreamQueryServer(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, _ *nats.Conn, mgr *jsm.Manager) {
		stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.Replicas(2))
		checkErr(t, err, "create failed")

		nfo, err := stream.LatestInformation()
		checkErr(t, err, "info failed")

		checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{Server: regexp.MustCompile("foo")})
		checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{Invert: true, Server: regexp.MustCompile("foo")})

		checkStreamQueryMatched(t, 1, mgr, jsm.StreamQuery{Server: regexp.MustCompile(nfo.Cluster.Leader)})
		checkStreamQueryMatched(t, 0, mgr, jsm.StreamQuery{Invert: true, Server: regexp.MustCompile(nfo.Cluster.Leader)})
	})
}
