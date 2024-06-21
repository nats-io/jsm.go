package jsm

import (
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/monitor"
)

func requireLen[A any](t *testing.T, s []A, expect int) {
	t.Helper()

	if len(s) != expect {
		t.Fatalf("Expected %d elements in collection: %v", expect, s)
	}
}
func requireEmpty[A any](t *testing.T, s []A) {
	t.Helper()

	if len(s) != 0 {
		t.Fatalf("Expected empty collection: %v", s)
	}
}

func requireElement[S ~[]E, E comparable](t *testing.T, s S, v E) {
	t.Helper()

	if !slices.Contains(s, v) {
		t.Fatalf("Expected %v to contain %v", s, v)
	}
}

func requireRegexElement(t *testing.T, s []string, m string) {
	t.Helper()

	r, err := regexp.Compile(m)
	if err != nil {
		t.Fatalf("invalid regex: %v", err)
	}

	if !slices.ContainsFunc(s, func(s string) bool {
		return r.MatchString(s)
	}) {
		t.Fatalf("Expected %v to contain element matching %q", s, m)
	}
}

func TestStream_checkSources(t *testing.T) {
	setup := func() (*Stream, *monitor.Result, *api.StreamInfo) {
		return &Stream{}, &monitor.Result{}, &api.StreamInfo{
			Config: api.StreamConfig{
				Name: "test_stream",
			},
		}
	}

	t.Run("Should handle fewer than desired", func(t *testing.T) {
		s, check, si := setup()
		s.checkSources(si, check, StreamHealthCheckOptions{
			MinSources: 1,
			MaxSources: 2,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "0 sources")

		s, check, si = setup()
		si.Sources = append(si.Sources, &api.StreamSourceInfo{})
		s.checkSources(si, check, StreamHealthCheckOptions{
			MinSources: 2,
			MaxSources: 3,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "1 sources")
	})

	t.Run("Should handle more than desired", func(t *testing.T) {
		s, check, si := setup()
		si.Sources = append(si.Sources, &api.StreamSourceInfo{})
		si.Sources = append(si.Sources, &api.StreamSourceInfo{})
		s.checkSources(si, check, StreamHealthCheckOptions{
			MaxSources: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "2 sources")
	})

	t.Run("Should handle valid number of sources", func(t *testing.T) {
		s, check, si := setup()
		si.Sources = append(si.Sources, &api.StreamSourceInfo{})
		si.Sources = append(si.Sources, &api.StreamSourceInfo{})
		s.checkSources(si, check, StreamHealthCheckOptions{
			MinSources: 2,
			MaxSources: 3,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireElement(t, check.OKs, "2 sources")
	})

	t.Run("Should detect lagged replicas", func(t *testing.T) {
		s, check, si := setup()
		si.Sources = append(si.Sources, &api.StreamSourceInfo{
			Lag: 100,
		})
		si.Sources = append(si.Sources, &api.StreamSourceInfo{
			Lag: 200,
		})
		s.checkSources(si, check, StreamHealthCheckOptions{
			SourcesLagCritical: 100,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "2 sources are lagged")
	})

	t.Run("Should detect not seen replicas", func(t *testing.T) {
		s, check, si := setup()
		si.Sources = append(si.Sources, &api.StreamSourceInfo{
			Active: time.Second,
		})
		si.Sources = append(si.Sources, &api.StreamSourceInfo{
			Active: 2 * time.Second,
		})
		s.checkSources(si, check, StreamHealthCheckOptions{
			SourcesSeenCritical: time.Millisecond,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "2 sources are inactive")
	})

	t.Run("Should handle valid replicas", func(t *testing.T) {
		s, check, si := setup()
		si.Sources = append(si.Sources, &api.StreamSourceInfo{
			Lag:    100,
			Active: 100 * time.Millisecond,
		})
		si.Sources = append(si.Sources, &api.StreamSourceInfo{
			Lag:    200,
			Active: time.Millisecond,
		})
		s.checkSources(si, check, StreamHealthCheckOptions{
			SourcesLagCritical:  500,
			SourcesSeenCritical: time.Second,
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
	setup := func() (*Stream, *monitor.Result, *api.StreamInfo) {
		return &Stream{}, &monitor.Result{}, &api.StreamInfo{
			Config: api.StreamConfig{
				Name: "test_stream",
			},
		}
	}

	t.Run("Should handle no thresholds", func(t *testing.T) {
		s, check, si := setup()

		si.State.Msgs = 1000
		s.checkMessages(si, check, StreamHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle critical situations", func(t *testing.T) {
		s, check, si := setup()
		si.State.Msgs = 1000
		s.checkMessages(si, check, StreamHealthCheckOptions{
			MessagesCrit: 1000,
		}, api.NewDiscardLogger())

		requireElement(t, check.Criticals, "1000 messages")
		requireEmpty(t, check.OKs)

		s, check, si = setup()
		si.State.Msgs = 999
		s.checkMessages(si, check, StreamHealthCheckOptions{
			MessagesCrit: 1000,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "999 messages")
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle warning situations", func(t *testing.T) {
		s, check, si := setup()
		si.State.Msgs = 1000
		s.checkMessages(si, check, StreamHealthCheckOptions{
			MessagesWarn: 1000,
		}, api.NewDiscardLogger())
		requireElement(t, check.Warnings, "1000 messages")
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.OKs)

		s, check, si = setup()
		si.State.Msgs = 999

		s.checkMessages(si, check, StreamHealthCheckOptions{
			MessagesWarn: 1000,
		}, api.NewDiscardLogger())
		requireElement(t, check.Warnings, "999 messages")
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle ok situations", func(t *testing.T) {
		s, check, si := setup()
		si.State.Msgs = 1000
		s.checkMessages(si, check, StreamHealthCheckOptions{
			MessagesWarn: 500,
			MessagesCrit: 200,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireElement(t, check.OKs, "1000 messages")
	})
}

func TestStream_checkSubjects(t *testing.T) {
	setup := func() (*Stream, *monitor.Result, *api.StreamInfo) {
		return &Stream{}, &monitor.Result{}, &api.StreamInfo{
			Config: api.StreamConfig{
				Name: "test_stream",
			},
		}
	}

	t.Run("Should handle no thresholds", func(t *testing.T) {
		s, check, si := setup()
		s.checkSubjects(si, check, StreamHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("warn less than crit", func(t *testing.T) {
		t.Run("Should handle fewer subjects", func(t *testing.T) {
			s, check, si := setup()
			si.State.NumSubjects = 100

			s.checkSubjects(si, check, StreamHealthCheckOptions{
				SubjectsWarn: 200,
				SubjectsCrit: 300,
			}, api.NewDiscardLogger())
			requireEmpty(t, check.Criticals)
			requireEmpty(t, check.Warnings)
			requireElement(t, check.OKs, "100 subjects")
		})

		t.Run("Should handle more than subjects", func(t *testing.T) {
			s, check, si := setup()
			si.State.NumSubjects = 400
			s.checkSubjects(si, check, StreamHealthCheckOptions{
				SubjectsWarn: 200,
				SubjectsCrit: 300,
			}, api.NewDiscardLogger())
			requireElement(t, check.Criticals, "400 subjects")
			requireEmpty(t, check.Warnings)
			requireEmpty(t, check.OKs)

			s, check, si = setup()
			si.State.NumSubjects = 250
			s.checkSubjects(si, check, StreamHealthCheckOptions{
				SubjectsWarn: 200,
				SubjectsCrit: 300,
			}, api.NewDiscardLogger())
			requireEmpty(t, check.Criticals)
			requireElement(t, check.Warnings, "250 subjects")
			requireEmpty(t, check.OKs)
		})

		t.Run("Should handle valid subject counts", func(t *testing.T) {
			s, check, si := setup()
			si.State.NumSubjects = 100
			s.checkSubjects(si, check, StreamHealthCheckOptions{
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
			s, check, si := setup()
			si.State.NumSubjects = 100
			s.checkSubjects(si, check, StreamHealthCheckOptions{
				SubjectsWarn: 300,
				SubjectsCrit: 200,
			}, api.NewDiscardLogger())
			requireElement(t, check.Criticals, "100 subjects")
			requireEmpty(t, check.Warnings)
			requireEmpty(t, check.OKs)

			s, check, si = setup()
			si.State.NumSubjects = 250
			s.checkSubjects(si, check, StreamHealthCheckOptions{
				SubjectsWarn: 300,
				SubjectsCrit: 200,
			}, api.NewDiscardLogger())
			requireEmpty(t, check.Criticals)
			requireElement(t, check.Warnings, "250 subjects")
			requireEmpty(t, check.OKs)
		})

		t.Run("Should handle valid subject counts", func(t *testing.T) {
			s, check, si := setup()
			si.State.NumSubjects = 400
			s.checkSubjects(si, check, StreamHealthCheckOptions{
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
	s := Stream{}

	t.Run("Should handle no thresholds", func(t *testing.T) {
		check := &monitor.Result{}
		s.checkMirror(&api.StreamInfo{}, check, StreamHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle absent state", func(t *testing.T) {
		check := &monitor.Result{}
		si := &api.StreamInfo{
			Config: api.StreamConfig{
				Name:   "test",
				Mirror: &api.StreamSource{},
			},
		}

		s.checkMirror(si, check, StreamHealthCheckOptions{
			SourcesLagCritical:  1,
			SourcesSeenCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "invalid state")
	})

	t.Run("Should handle lag greater than critical", func(t *testing.T) {
		check := &monitor.Result{}
		si := &api.StreamInfo{
			Config: api.StreamConfig{
				Name:   "test",
				Mirror: &api.StreamSource{},
			},
			Mirror: &api.StreamSourceInfo{
				Lag: 100,
			},
		}

		s.checkMirror(si, check, StreamHealthCheckOptions{
			SourcesLagCritical: 100,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Mirror Lag 100")

		check = &monitor.Result{}
		si.Mirror.Lag = 200
		s.checkMirror(si, check, StreamHealthCheckOptions{
			SourcesLagCritical: 100,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Mirror Lag 200")
	})

	t.Run("Should handle seen greater than critical", func(t *testing.T) {
		check := &monitor.Result{}
		si := &api.StreamInfo{
			Config: api.StreamConfig{
				Name:   "test",
				Mirror: &api.StreamSource{},
			},
			Mirror: &api.StreamSourceInfo{
				Active: time.Millisecond,
			},
		}

		s.checkMirror(si, check, StreamHealthCheckOptions{
			SourcesSeenCritical: time.Millisecond,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Mirror Seen 1ms")

		check = &monitor.Result{}
		si.Mirror.Active = time.Second
		s.checkMirror(si, check, StreamHealthCheckOptions{
			SourcesSeenCritical: time.Millisecond,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Mirror Seen 1s")
	})

	t.Run("Should handle healthy mirrors", func(t *testing.T) {
		check := &monitor.Result{}
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

		s.checkMirror(si, check, StreamHealthCheckOptions{
			SourcesLagCritical:  200,
			SourcesSeenCritical: time.Second,
		}, api.NewDiscardLogger())

		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireElement(t, check.OKs, "Mirror X")
	})
}

func TestStream_checkCluster(t *testing.T) {
	s := Stream{}

	t.Run("Skip without threshold", func(t *testing.T) {
		check := &monitor.Result{}
		s.checkCluster(&api.StreamInfo{}, check, StreamHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should be critical when the stream is not clustered and a threshold is given", func(t *testing.T) {
		check := &monitor.Result{}
		s.checkCluster(&api.StreamInfo{}, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Stream is not clustered")
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should be critical when replica counts do not match expectation", func(t *testing.T) {
		check := &monitor.Result{}
		si := &api.StreamInfo{
			Cluster: &api.ClusterInfo{
				Leader: "p2",
				Replicas: []*api.PeerInfo{
					{Name: "p1"},
				},
			}}

		s.checkCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Expected 3 replicas got 2")
	})

	t.Run("Should handle no leaders", func(t *testing.T) {
		check := &monitor.Result{}
		si := &api.StreamInfo{
			Cluster: &api.ClusterInfo{
				Replicas: []*api.PeerInfo{
					{Name: "p1"},
					{Name: "p2"},
					{Name: "p3"},
				},
			},
		}

		s.checkCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "No leader")

		check = &monitor.Result{}
		si.Cluster = &api.ClusterInfo{
			Leader: "p1",
			Replicas: []*api.PeerInfo{
				{Name: "p2"},
				{Name: "p3"},
			},
		}

		s.checkCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireElement(t, check.OKs, "3 peers")
	})

	t.Run("Should detect lagged peers", func(t *testing.T) {
		check := &monitor.Result{}
		si := &api.StreamInfo{
			Cluster: &api.ClusterInfo{
				Leader: "p1",
				Replicas: []*api.PeerInfo{
					{Name: "p2", Lag: 1000},
					{Name: "p3", Lag: 10},
				},
			},
		}

		s.checkCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
			ClusterLagCritical:   100,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "1 replicas lagged")
	})

	t.Run("Should detect inactive peers", func(t *testing.T) {
		check := &monitor.Result{}
		si := &api.StreamInfo{
			Cluster: &api.ClusterInfo{
				Leader: "p1",
				Replicas: []*api.PeerInfo{
					{Name: "p2", Lag: 10, Active: time.Second},
					{Name: "p3", Lag: 10, Active: time.Hour},
				},
			},
		}

		s.checkCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
			ClusterSeenCritical:  time.Minute,
		}, api.NewDiscardLogger())

		requireElement(t, check.Criticals, "1 replicas inactive")
	})

	t.Run("Should detect offline peers", func(t *testing.T) {
		check := &monitor.Result{}
		si := &api.StreamInfo{
			Cluster: &api.ClusterInfo{
				Leader: "p1",
				Replicas: []*api.PeerInfo{
					{Name: "p2", Lag: 10, Active: time.Second, Offline: true},
					{Name: "p3", Lag: 10, Active: time.Hour},
				},
			},
		}

		s.checkCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
		}, api.NewDiscardLogger())

		requireElement(t, check.Criticals, "1 replicas offline")
	})

	t.Run("Should handle ok streams", func(t *testing.T) {
		check := &monitor.Result{}
		si := &api.StreamInfo{
			Cluster: &api.ClusterInfo{
				Leader: "p1",
				Replicas: []*api.PeerInfo{
					{Name: "p2", Lag: 10, Active: time.Second},
					{Name: "p3", Lag: 10, Active: time.Second},
				},
			},
		}
		s.checkCluster(si, check, StreamHealthCheckOptions{
			ClusterExpectedPeers: 3,
			ClusterLagCritical:   20,
			ClusterSeenCritical:  time.Minute,
		}, api.NewDiscardLogger())

		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireElement(t, check.OKs, "3 peers")
	})
}
