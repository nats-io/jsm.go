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
	"testing"
	"time"

	"github.com/nats-io/jsm.go/monitor"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func TestCheckJSZ(t *testing.T) {
	t.Run("nil meta", func(t *testing.T) {
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckJetstreamMeta("", nil, check, monitor.CheckJetstreamMetaOptions{
			Resolver: func(_ *nats.Conn) (*server.ServerAPIJszResponse, error) {
				return &server.ServerAPIJszResponse{
					Data: &server.JSInfo{},
				}, nil
			},
		}))
		assertListEquals(t, check.Criticals, "no cluster information")
	})

	t.Run("no meta leader", func(t *testing.T) {
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckJetstreamMeta("", nil, check, monitor.CheckJetstreamMetaOptions{
			Resolver: func(_ *nats.Conn) (*server.ServerAPIJszResponse, error) {
				r := &server.ServerAPIJszResponse{
					Data: &server.JSInfo{
						Meta: &server.MetaClusterInfo{},
					},
				}

				return r, nil
			},
		}))

		assertListEquals(t, check.Criticals, "No leader")
		assertListIsEmpty(t, check.OKs)
	})

	t.Run("invalid peer count", func(t *testing.T) {
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckJetstreamMeta("", nil, check, monitor.CheckJetstreamMetaOptions{
			ExpectServers: 2,
			Resolver: func(_ *nats.Conn) (*server.ServerAPIJszResponse, error) {
				r := &server.ServerAPIJszResponse{
					Data: &server.JSInfo{
						Meta: &server.MetaClusterInfo{
							Leader: "L1",
						},
					},
				}

				return r, nil
			},
		}))

		assertListEquals(t, check.Criticals, "1 peers of expected 2")
	})

	t.Run("good peer", func(t *testing.T) {
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckJetstreamMeta("", nil, check, monitor.CheckJetstreamMetaOptions{
			ExpectServers: 3,
			SeenCritical:  1,
			LagCritical:   10,
			Resolver: func(_ *nats.Conn) (*server.ServerAPIJszResponse, error) {
				r := &server.ServerAPIJszResponse{
					Data: &server.JSInfo{
						Meta: &server.MetaClusterInfo{
							Leader: "l1",
							Replicas: []*server.PeerInfo{
								{Name: "replica1", Current: true, Active: 10 * time.Millisecond, Lag: 1},
								{Name: "replica2", Current: true, Active: 10 * time.Millisecond, Lag: 1},
							},
						},
					},
				}

				return r, nil
			},
		}))
		assertListIsEmpty(t, check.Criticals)
		assertHasPDItem(t, check, "peers=3;3;3", "peer_offline=0", "peer_not_current=0", "peer_inactive=0", "peer_lagged=0")
	})

	t.Run("not current peer", func(t *testing.T) {
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckJetstreamMeta("", nil, check, monitor.CheckJetstreamMetaOptions{
			ExpectServers: 3,
			SeenCritical:  1,
			LagCritical:   10,
			Resolver: func(_ *nats.Conn) (*server.ServerAPIJszResponse, error) {
				r := &server.ServerAPIJszResponse{
					Data: &server.JSInfo{
						Meta: &server.MetaClusterInfo{
							Leader: "l1",
							Replicas: []*server.PeerInfo{
								{Name: "replica1", Active: 10 * time.Millisecond, Lag: 1},
								{Name: "replica2", Current: true, Active: 10 * time.Millisecond, Lag: 1},
							},
						},
					},
				}

				return r, nil
			},
		}))

		assertListEquals(t, check.Criticals, "1 not current")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "peers=3;3;3", "peer_offline=0", "peer_not_current=1", "peer_inactive=0", "peer_lagged=0")
	})

	t.Run("offline peer", func(t *testing.T) {
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckJetstreamMeta("", nil, check, monitor.CheckJetstreamMetaOptions{
			ExpectServers: 3,
			SeenCritical:  1,
			LagCritical:   10,
			Resolver: func(_ *nats.Conn) (*server.ServerAPIJszResponse, error) {
				r := &server.ServerAPIJszResponse{
					Data: &server.JSInfo{
						Meta: &server.MetaClusterInfo{
							Leader: "l1",
							Replicas: []*server.PeerInfo{
								{Name: "replica1", Current: true, Offline: true, Active: 10 * time.Millisecond, Lag: 1},
								{Name: "replica2", Current: true, Active: 10 * time.Millisecond, Lag: 1},
							},
						},
					},
				}

				return r, nil
			},
		}))
		assertListEquals(t, check.Criticals, "1 offline")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "peers=3;3;3", "peer_offline=1", "peer_not_current=0", "peer_inactive=0", "peer_lagged=0")
	})

	t.Run("inactive peer", func(t *testing.T) {
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckJetstreamMeta("", nil, check, monitor.CheckJetstreamMetaOptions{
			ExpectServers: 3,
			SeenCritical:  1,
			LagCritical:   10,
			Resolver: func(_ *nats.Conn) (*server.ServerAPIJszResponse, error) {
				r := &server.ServerAPIJszResponse{
					Data: &server.JSInfo{
						Meta: &server.MetaClusterInfo{
							Leader: "l1",
							Replicas: []*server.PeerInfo{
								{Name: "replica1", Current: true, Active: 10 * time.Hour, Lag: 1},
								{Name: "replica2", Current: true, Active: 10 * time.Millisecond, Lag: 1},
							},
						},
					},
				}

				return r, nil
			},
		}))
		assertListEquals(t, check.Criticals, "1 inactive more than 1s")
		assertListIsEmpty(t, check.OKs)
		assertHasPDItem(t, check, "peers=3;3;3", "peer_offline=0", "peer_not_current=0", "peer_inactive=1", "peer_lagged=0")
	})

	t.Run("lagged peer", func(t *testing.T) {
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckJetstreamMeta("", nil, check, monitor.CheckJetstreamMetaOptions{
			ExpectServers: 3,
			SeenCritical:  1,
			LagCritical:   10,
			Resolver: func(_ *nats.Conn) (*server.ServerAPIJszResponse, error) {
				r := &server.ServerAPIJszResponse{
					Data: &server.JSInfo{
						Meta: &server.MetaClusterInfo{
							Leader: "l1",
							Replicas: []*server.PeerInfo{
								{Name: "replica1", Current: true, Active: 10 * time.Millisecond, Lag: 10000},
								{Name: "replica2", Current: true, Active: 10 * time.Millisecond, Lag: 1},
							},
						},
					},
				}

				return r, nil
			},
		}))
		assertListEquals(t, check.Criticals, "1 lagged more than 10 ops")
		assertHasPDItem(t, check, "peers=3;3;3", "peer_offline=0", "peer_not_current=0", "peer_inactive=0", "peer_lagged=1")
	})

	t.Run("multiple errors", func(t *testing.T) {
		check := &monitor.Result{}
		assertNoError(t, monitor.CheckJetstreamMeta("", nil, check, monitor.CheckJetstreamMetaOptions{
			ExpectServers: 3,
			SeenCritical:  1,
			LagCritical:   10,
			Resolver: func(_ *nats.Conn) (*server.ServerAPIJszResponse, error) {
				r := &server.ServerAPIJszResponse{
					Data: &server.JSInfo{
						Meta: &server.MetaClusterInfo{
							Leader: "l1",
							Replicas: []*server.PeerInfo{
								{Name: "replica1", Current: true, Active: 10 * time.Millisecond, Lag: 10000},
								{Name: "replica2", Current: true, Active: 10 * time.Hour, Lag: 1},
								{Name: "replica3", Current: true, Offline: true, Active: 10 * time.Millisecond, Lag: 1},
								{Name: "replica4", Active: 10 * time.Millisecond, Lag: 1},
							},
						},
					},
				}

				return r, nil
			},
		}))
		assertHasPDItem(t, check, "peers=5;3;3", "peer_offline=1", "peer_not_current=1", "peer_inactive=1", "peer_lagged=1")
		assertListEquals(t, check.Criticals, "5 peers of expected 3",
			"1 not current",
			"1 inactive more than 1s",
			"1 offline",
			"1 lagged more than 10 ops")
	})
}
