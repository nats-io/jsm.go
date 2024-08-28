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

	"github.com/google/go-cmp/cmp"
	"github.com/nats-io/jsm.go/api"
)

func TestStream_checkSources(t *testing.T) {
	setup := func() (*Result, *api.StreamInfo) {
		return &Result{}, &api.StreamInfo{
			Config: api.StreamConfig{
				Name: "test_stream",
			},
		}
	}

	t.Run("Should handle fewer than desired", func(t *testing.T) {
		check, si := setup()
		streamCheckSources(si, check, StreamHealthCheckOptions{
			MinSources: 1,
			MaxSources: 2,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "0 sources")

		check, si = setup()
		si.Sources = append(si.Sources, &api.StreamSourceInfo{})
		streamCheckSources(si, check, StreamHealthCheckOptions{
			MinSources: 2,
			MaxSources: 3,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "1 sources")
	})

	t.Run("Should handle more than desired", func(t *testing.T) {
		check, si := setup()
		si.Sources = append(si.Sources, &api.StreamSourceInfo{})
		si.Sources = append(si.Sources, &api.StreamSourceInfo{})
		streamCheckSources(si, check, StreamHealthCheckOptions{
			MaxSources: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "2 sources")
	})

	t.Run("Should handle valid number of sources", func(t *testing.T) {
		check, si := setup()
		si.Sources = append(si.Sources, &api.StreamSourceInfo{})
		si.Sources = append(si.Sources, &api.StreamSourceInfo{})
		streamCheckSources(si, check, StreamHealthCheckOptions{
			MinSources: 2,
			MaxSources: 3,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireElement(t, check.OKs, "2 sources")
	})

	t.Run("Should detect lagged replicas", func(t *testing.T) {
		check, si := setup()
		si.Sources = append(si.Sources, &api.StreamSourceInfo{
			Lag: 100,
		})
		si.Sources = append(si.Sources, &api.StreamSourceInfo{
			Lag: 200,
		})
		streamCheckSources(si, check, StreamHealthCheckOptions{
			SourcesLagCritical: 100,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "2 sources are lagged")
	})

	t.Run("Should detect not seen replicas", func(t *testing.T) {
		check, si := setup()
		si.Sources = append(si.Sources, &api.StreamSourceInfo{
			Active: time.Second,
		})
		si.Sources = append(si.Sources, &api.StreamSourceInfo{
			Active: 2 * time.Second,
		})
		streamCheckSources(si, check, StreamHealthCheckOptions{
			SourcesSeenCritical: float64(1) / 1000,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "2 sources are inactive")
	})

	t.Run("Should handle valid replicas", func(t *testing.T) {
		check, si := setup()
		si.Sources = append(si.Sources, &api.StreamSourceInfo{
			Lag:    100,
			Active: 100 * time.Millisecond,
		})
		si.Sources = append(si.Sources, &api.StreamSourceInfo{
			Lag:    200,
			Active: time.Millisecond,
		})
		streamCheckSources(si, check, StreamHealthCheckOptions{
			SourcesLagCritical:  500,
			SourcesSeenCritical: 1,
			MinSources:          2,
			MaxSources:          10,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		if !cmp.Equal(check.OKs, []string{"2 sources", "2 sources current", "2 sources active"}) {
			t.Fatalf("invalid OK status: %v", check.OKs)
		}
	})
}

func TestStream_checkMessages(t *testing.T) {
	setup := func() (*Result, *api.StreamInfo) {
		return &Result{}, &api.StreamInfo{
			Config: api.StreamConfig{
				Name: "test_stream",
			},
		}
	}

	t.Run("Should handle no thresholds", func(t *testing.T) {
		check, si := setup()

		si.State.Msgs = 1000
		streamCheckMessages(si, check, StreamHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle critical situations", func(t *testing.T) {
		check, si := setup()
		si.State.Msgs = 1000
		streamCheckMessages(si, check, StreamHealthCheckOptions{
			MessagesCrit: 1000,
		}, api.NewDiscardLogger())

		requireElement(t, check.Criticals, "1000 messages")
		requireEmpty(t, check.OKs)

		check, si = setup()
		si.State.Msgs = 999
		streamCheckMessages(si, check, StreamHealthCheckOptions{
			MessagesCrit: 1000,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "999 messages")
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle warning situations", func(t *testing.T) {
		check, si := setup()
		si.State.Msgs = 1000
		streamCheckMessages(si, check, StreamHealthCheckOptions{
			MessagesWarn: 1000,
		}, api.NewDiscardLogger())
		requireElement(t, check.Warnings, "1000 messages")
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.OKs)

		check, si = setup()
		si.State.Msgs = 999

		streamCheckMessages(si, check, StreamHealthCheckOptions{
			MessagesWarn: 1000,
		}, api.NewDiscardLogger())
		requireElement(t, check.Warnings, "999 messages")
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle ok situations", func(t *testing.T) {
		check, si := setup()
		si.State.Msgs = 1000
		streamCheckMessages(si, check, StreamHealthCheckOptions{
			MessagesWarn: 500,
			MessagesCrit: 200,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireElement(t, check.OKs, "1000 messages")
	})
}

func TestStream_checkSubjects(t *testing.T) {
	setup := func() (*Result, *api.StreamInfo) {
		return &Result{}, &api.StreamInfo{
			Config: api.StreamConfig{
				Name: "test_stream",
			},
		}
	}

	t.Run("Should handle no thresholds", func(t *testing.T) {
		check, si := setup()
		streamCheckSubjects(si, check, StreamHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("warn less than crit", func(t *testing.T) {
		t.Run("Should handle fewer subjects", func(t *testing.T) {
			check, si := setup()
			si.State.NumSubjects = 100

			streamCheckSubjects(si, check, StreamHealthCheckOptions{
				SubjectsWarn: 200,
				SubjectsCrit: 300,
			}, api.NewDiscardLogger())
			requireEmpty(t, check.Criticals)
			requireEmpty(t, check.Warnings)
			requireElement(t, check.OKs, "100 subjects")
		})

		t.Run("Should handle more than subjects", func(t *testing.T) {
			check, si := setup()
			si.State.NumSubjects = 400
			streamCheckSubjects(si, check, StreamHealthCheckOptions{
				SubjectsWarn: 200,
				SubjectsCrit: 300,
			}, api.NewDiscardLogger())
			requireElement(t, check.Criticals, "400 subjects")
			requireEmpty(t, check.Warnings)
			requireEmpty(t, check.OKs)

			check, si = setup()
			si.State.NumSubjects = 250
			streamCheckSubjects(si, check, StreamHealthCheckOptions{
				SubjectsWarn: 200,
				SubjectsCrit: 300,
			}, api.NewDiscardLogger())
			requireEmpty(t, check.Criticals)
			requireElement(t, check.Warnings, "250 subjects")
			requireEmpty(t, check.OKs)
		})

		t.Run("Should handle valid subject counts", func(t *testing.T) {
			check, si := setup()
			si.State.NumSubjects = 100
			streamCheckSubjects(si, check, StreamHealthCheckOptions{
				SubjectsWarn: 200,
				SubjectsCrit: 300,
			}, api.NewDiscardLogger())
			requireEmpty(t, check.Criticals)
			requireEmpty(t, check.Warnings)
			requireElement(t, check.OKs, "100 subjects")
		})
	})

	t.Run("warn more than crit", func(t *testing.T) {
		t.Run("Should handle fewer subjects", func(t *testing.T) {
			check, si := setup()
			si.State.NumSubjects = 100
			streamCheckSubjects(si, check, StreamHealthCheckOptions{
				SubjectsWarn: 300,
				SubjectsCrit: 200,
			}, api.NewDiscardLogger())
			requireElement(t, check.Criticals, "100 subjects")
			requireEmpty(t, check.Warnings)
			requireEmpty(t, check.OKs)

			check, si = setup()
			si.State.NumSubjects = 250
			streamCheckSubjects(si, check, StreamHealthCheckOptions{
				SubjectsWarn: 300,
				SubjectsCrit: 200,
			}, api.NewDiscardLogger())
			requireEmpty(t, check.Criticals)
			requireElement(t, check.Warnings, "250 subjects")
			requireEmpty(t, check.OKs)
		})

		t.Run("Should handle valid subject counts", func(t *testing.T) {
			check, si := setup()
			si.State.NumSubjects = 400
			streamCheckSubjects(si, check, StreamHealthCheckOptions{
				SubjectsWarn: 300,
				SubjectsCrit: 200,
			}, api.NewDiscardLogger())

			requireEmpty(t, check.Criticals)
			requireEmpty(t, check.Warnings)
			requireElement(t, check.OKs, "400 subjects")
		})
	})
}

func TestStream_checkMirror(t *testing.T) {
	t.Run("Should handle no thresholds", func(t *testing.T) {
		check := &Result{}
		streamCheckMirror(&api.StreamInfo{}, check, StreamHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle absent state", func(t *testing.T) {
		check := &Result{}
		si := &api.StreamInfo{
			Config: api.StreamConfig{
				Name:   "test",
				Mirror: &api.StreamSource{},
			},
		}

		streamCheckMirror(si, check, StreamHealthCheckOptions{
			SourcesLagCritical:  1,
			SourcesSeenCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "invalid state")
	})

	t.Run("Should handle lag greater than critical", func(t *testing.T) {
		check := &Result{}
		si := &api.StreamInfo{
			Config: api.StreamConfig{
				Name:   "test",
				Mirror: &api.StreamSource{},
			},
			Mirror: &api.StreamSourceInfo{
				Lag: 100,
			},
		}

		streamCheckMirror(si, check, StreamHealthCheckOptions{
			SourcesLagCritical: 100,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Mirror Lag 100")

		check = &Result{}
		si.Mirror.Lag = 200
		streamCheckMirror(si, check, StreamHealthCheckOptions{
			SourcesLagCritical: 100,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Mirror Lag 200")
	})

	t.Run("Should handle seen greater than critical", func(t *testing.T) {
		check := &Result{}
		si := &api.StreamInfo{
			Config: api.StreamConfig{
				Name:   "test",
				Mirror: &api.StreamSource{},
			},
			Mirror: &api.StreamSourceInfo{
				Active: time.Millisecond,
			},
		}

		streamCheckMirror(si, check, StreamHealthCheckOptions{
			SourcesSeenCritical: float64(1) / 1000,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Mirror Seen 1ms")

		check = &Result{}
		si.Mirror.Active = time.Second
		streamCheckMirror(si, check, StreamHealthCheckOptions{
			SourcesSeenCritical: float64(1) / 1000,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Mirror Seen 1s")
	})

	t.Run("Should handle healthy mirrors", func(t *testing.T) {
		check := &Result{}
		si := &api.StreamInfo{
			Config: api.StreamConfig{
				Name: "test",
				Mirror: &api.StreamSource{
					Name: "X",
				},
			},
			Mirror: &api.StreamSourceInfo{
				Active: time.Millisecond,
				Lag:    100,
			},
		}

		streamCheckMirror(si, check, StreamHealthCheckOptions{
			SourcesLagCritical:  200,
			SourcesSeenCritical: 1,
		}, api.NewDiscardLogger())

		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireElement(t, check.OKs, "Mirror X")
	})
}

func TestStream_checkCluster(t *testing.T) {
	t.Run("Skip without threshold", func(t *testing.T) {
		check := &Result{}
		streamCheckCluster(&api.StreamInfo{}, check, StreamHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should be critical when the stream is not clustered and a threshold is given", func(t *testing.T) {
		check := &Result{}
		streamCheckCluster(&api.StreamInfo{}, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Stream is not clustered")
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should be critical when replica counts do not match expectation", func(t *testing.T) {
		check := &Result{}
		si := &api.StreamInfo{
			Cluster: &api.ClusterInfo{
				Leader: "p2",
				Replicas: []*api.PeerInfo{
					{Name: "p1"},
				},
			},
			Config: api.StreamConfig{
				Replicas: 2,
			},
		}

		streamCheckCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Expected 3 replicas got 2")
	})

	t.Run("Should handle no leaders", func(t *testing.T) {
		check := &Result{}
		si := &api.StreamInfo{
			Cluster: &api.ClusterInfo{
				Replicas: []*api.PeerInfo{
					{Name: "p1"},
					{Name: "p2"},
					{Name: "p3"},
				},
			},
			Config: api.StreamConfig{
				Replicas: 3,
			},
		}

		streamCheckCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "No leader")

		check = &Result{}
		si.Cluster = &api.ClusterInfo{
			Leader: "p1",
			Replicas: []*api.PeerInfo{
				{Name: "p2"},
				{Name: "p3"},
			},
		}

		streamCheckCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireElement(t, check.OKs, "3 peers")
	})

	t.Run("Should detect lagged peers", func(t *testing.T) {
		check := &Result{}
		si := &api.StreamInfo{
			Cluster: &api.ClusterInfo{
				Leader: "p1",
				Replicas: []*api.PeerInfo{
					{Name: "p2", Lag: 1000},
					{Name: "p3", Lag: 10},
				},
			},
			Config: api.StreamConfig{
				Replicas: 3,
			},
		}

		streamCheckCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
			ClusterLagCritical:   100,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "1 replicas lagged")
	})

	t.Run("Should detect inactive peers", func(t *testing.T) {
		check := &Result{}
		si := &api.StreamInfo{
			Cluster: &api.ClusterInfo{
				Leader: "p1",
				Replicas: []*api.PeerInfo{
					{Name: "p2", Lag: 10, Active: time.Second},
					{Name: "p3", Lag: 10, Active: time.Hour},
				},
			},
			Config: api.StreamConfig{
				Replicas: 3,
			},
		}

		streamCheckCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
			ClusterSeenCritical:  60,
		}, api.NewDiscardLogger())

		requireElement(t, check.Criticals, "1 replicas inactive")
	})

	t.Run("Should detect offline peers", func(t *testing.T) {
		check := &Result{}
		si := &api.StreamInfo{
			Cluster: &api.ClusterInfo{
				Leader: "p1",
				Replicas: []*api.PeerInfo{
					{Name: "p2", Lag: 10, Active: time.Second, Offline: true},
					{Name: "p3", Lag: 10, Active: time.Hour},
				},
			},
			Config: api.StreamConfig{
				Replicas: 3,
			},
		}

		streamCheckCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
		}, api.NewDiscardLogger())

		requireElement(t, check.Criticals, "1 replicas offline")
	})

	t.Run("Should handle ok streams", func(t *testing.T) {
		check := &Result{}
		si := &api.StreamInfo{
			Cluster: &api.ClusterInfo{
				Leader: "p1",
				Replicas: []*api.PeerInfo{
					{Name: "p2", Lag: 10, Active: time.Second},
					{Name: "p3", Lag: 10, Active: time.Second},
				},
			},
			Config: api.StreamConfig{
				Replicas: 3,
			},
		}
		streamCheckCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
			ClusterLagCritical:   20,
			ClusterSeenCritical:  60,
		}, api.NewDiscardLogger())

		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireElement(t, check.OKs, "3 peers")
	})
}
