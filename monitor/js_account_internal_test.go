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
	"testing"
	"time"

	"github.com/nats-io/jsm.go/api"
)

func TestCheckStreamClusterHealth(t *testing.T) {
	mkStream := func(replicas int, leader string, peers []*api.PeerInfo) *api.StreamInfo {
		nfo := &api.StreamInfo{
			Config: api.StreamConfig{Replicas: replicas},
		}
		if leader != "" || peers != nil {
			nfo.Cluster = &api.ClusterInfo{
				Leader:   leader,
				Replicas: peers,
			}
		}
		return nfo
	}

	defaultOpts := &CheckJetStreamAccountOptions{
		ReplicaSeenCritical: 60,
		ReplicaLagCritical:  100,
	}

	pdVal := func(t *testing.T, check *Result, name string) float64 {
		t.Helper()
		for _, pd := range check.PerfData {
			if pd.Name == name {
				return pd.Value
			}
		}
		t.Fatalf("perf data item %q not found", name)
		return 0
	}

	t.Run("empty stream list produces no criticals", func(t *testing.T) {
		check := &Result{}
		checkStreamClusterHealth(check, defaultOpts, nil)
		requireEmpty(t, check.Criticals)
	})

	t.Run("single-replica streams are ok", func(t *testing.T) {
		check := &Result{}
		checkStreamClusterHealth(check, defaultOpts, []*api.StreamInfo{
			mkStream(1, "", nil),
			mkStream(1, "", nil),
		})
		requireEmpty(t, check.Criticals)
		if pdVal(t, check, "replicas_ok") != 2 {
			t.Fatalf("expected replicas_ok=2")
		}
	})

	t.Run("nil info counts as crit", func(t *testing.T) {
		check := &Result{}
		checkStreamClusterHealth(check, defaultOpts, []*api.StreamInfo{nil})
		requireElement(t, check.Criticals, "1 unhealthy streams")
		if pdVal(t, check, "replicas_ok") != 0 {
			t.Fatalf("expected replicas_ok=0 for stream with fetch error")
		}
	})

	t.Run("nil cluster info counts as crit", func(t *testing.T) {
		check := &Result{}
		nfo := &api.StreamInfo{Config: api.StreamConfig{Replicas: 3}}
		checkStreamClusterHealth(check, defaultOpts, []*api.StreamInfo{nfo})
		requireElement(t, check.Criticals, "1 unhealthy streams")
	})

	t.Run("no leader counts as unhealthy", func(t *testing.T) {
		check := &Result{}
		nfo := mkStream(3, "", []*api.PeerInfo{{}, {}})
		checkStreamClusterHealth(check, defaultOpts, []*api.StreamInfo{nfo})
		requireElement(t, check.Criticals, "1 unhealthy streams")
		if pdVal(t, check, "replicas_no_leader") != 1 {
			t.Fatalf("expected replicas_no_leader=1")
		}
	})

	t.Run("fewer replicas than configured counts as unhealthy", func(t *testing.T) {
		check := &Result{}
		// 3 replicas configured but only 1 peer in cluster (should be 2)
		nfo := mkStream(3, "leader", []*api.PeerInfo{{}})
		checkStreamClusterHealth(check, defaultOpts, []*api.StreamInfo{nfo})
		requireElement(t, check.Criticals, "1 unhealthy streams")
		if pdVal(t, check, "replicas_missing_replicas") != 1 {
			t.Fatalf("expected replicas_missing_replicas=1")
		}
	})

	t.Run("offline replica marks stream unhealthy and not ok", func(t *testing.T) {
		check := &Result{}
		nfo := mkStream(3, "leader", []*api.PeerInfo{
			{Offline: true},
			{Active: time.Second},
		})
		checkStreamClusterHealth(check, defaultOpts, []*api.StreamInfo{nfo})
		requireElement(t, check.Criticals, "1 unhealthy streams")
		if pdVal(t, check, "replicas_ok") != 0 {
			t.Fatalf("expected stream with offline replica to not be counted ok")
		}
		if pdVal(t, check, "replicas_fail") != 1 {
			t.Fatalf("expected replicas_fail=1")
		}
	})

	t.Run("lagged replica marks stream unhealthy", func(t *testing.T) {
		check := &Result{}
		nfo := mkStream(3, "leader", []*api.PeerInfo{
			{Active: time.Second, Lag: 200},
			{Active: time.Second, Lag: 10},
		})
		checkStreamClusterHealth(check, defaultOpts, []*api.StreamInfo{nfo})
		requireElement(t, check.Criticals, "1 unhealthy streams")
		if pdVal(t, check, "replicas_lagged") != 1 {
			t.Fatalf("expected replicas_lagged=1")
		}
		if pdVal(t, check, "replicas_ok") != 0 {
			t.Fatalf("expected stream with lagged replica to not be counted ok")
		}
	})

	t.Run("zero lag threshold skips lag check", func(t *testing.T) {
		check := &Result{}
		opts := &CheckJetStreamAccountOptions{ReplicaSeenCritical: 60, ReplicaLagCritical: 0}
		nfo := mkStream(3, "leader", []*api.PeerInfo{
			{Active: time.Second, Lag: 99999},
			{Active: time.Second, Lag: 99999},
		})
		checkStreamClusterHealth(check, opts, []*api.StreamInfo{nfo})
		requireEmpty(t, check.Criticals)
	})

	t.Run("not-seen replica marks stream unhealthy", func(t *testing.T) {
		check := &Result{}
		nfo := mkStream(3, "leader", []*api.PeerInfo{
			{Active: 120 * time.Second}, // > 60s threshold
			{Active: time.Second},
		})
		checkStreamClusterHealth(check, defaultOpts, []*api.StreamInfo{nfo})
		requireElement(t, check.Criticals, "1 unhealthy streams")
		if pdVal(t, check, "replicas_not_seen") != 1 {
			t.Fatalf("expected replicas_not_seen=1")
		}
		if pdVal(t, check, "replicas_ok") != 0 {
			t.Fatalf("expected stream with not-seen replica to not be counted ok")
		}
	})

	t.Run("zero seen threshold skips seen check", func(t *testing.T) {
		check := &Result{}
		opts := &CheckJetStreamAccountOptions{ReplicaSeenCritical: 0, ReplicaLagCritical: 100}
		nfo := mkStream(3, "leader", []*api.PeerInfo{
			{Active: 999 * time.Second},
			{Active: 999 * time.Second},
		})
		checkStreamClusterHealth(check, opts, []*api.StreamInfo{nfo})
		requireEmpty(t, check.Criticals)
	})

	t.Run("healthy replicated stream is counted ok", func(t *testing.T) {
		check := &Result{}
		nfo := mkStream(3, "leader", []*api.PeerInfo{
			{Active: time.Second, Lag: 5},
			{Active: time.Second, Lag: 5},
		})
		checkStreamClusterHealth(check, defaultOpts, []*api.StreamInfo{nfo})
		requireEmpty(t, check.Criticals)
		if pdVal(t, check, "replicas_ok") != 1 {
			t.Fatalf("expected replicas_ok=1")
		}
	})

	t.Run("unhealthy count includes all problem types", func(t *testing.T) {
		check := &Result{}
		// one stream with offline replica, one with no leader, one lagged
		offlineStream := mkStream(3, "leader", []*api.PeerInfo{
			{Offline: true},
			{Active: time.Second},
		})
		noLeaderStream := mkStream(3, "", []*api.PeerInfo{{}, {}})
		laggedStream := mkStream(3, "leader", []*api.PeerInfo{
			{Active: time.Second, Lag: 200},
			{Active: time.Second, Lag: 10},
		})
		checkStreamClusterHealth(check, defaultOpts, []*api.StreamInfo{offlineStream, noLeaderStream, laggedStream})
		requireElement(t, check.Criticals, "3 unhealthy streams")
	})
}
