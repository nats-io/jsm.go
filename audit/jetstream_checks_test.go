package audit

import (
	"path/filepath"
	"testing"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
)

func setupJetstreamCheck(t *testing.T, checkid string, streams map[string]any) Outcome {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "audit.zip")

	writer, err := archive.NewWriter(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive writer: %v", err)
	}

	for serverName, stream := range streams {
		err := writer.Add(
			stream,
			archive.TagAccount("A"),
			archive.TagStream("S1"),
			archive.TagServer(serverName),
			archive.TagCluster("C1"),
			archive.TagStreamInfo(),
		)
		if err != nil {
			t.Fatalf("failed to add stream for %s: %v", serverName, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close archive: %v", err)
	}

	reader, err := archive.NewReader(archivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	defer reader.Close()

	cc := &CheckCollection{}
	if err := RegisterJetStreamChecks(cc); err != nil {
		t.Fatalf("failed to register jetstream checks: %v", err)
	}

	var check *Check
	cc.EachCheck(func(c *Check) {
		if c.Code == checkid {
			check = c
		}
	})
	if check == nil {
		t.Fatalf("check %s not found", checkid)
	}

	examples := newExamplesCollection(0)
	result, err := check.Handler(check, reader, examples, api.NewDefaultLogger(api.WarnLevel))
	if err != nil {
		t.Fatalf("check handler failed: %v", err)
	}

	return result
}

func TestJETSTREAM_001(t *testing.T) {
	t.Run("Should fail when one replica is too far behind", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_001", map[string]any{
			"N1": &api.StreamInfo{Config: api.StreamConfig{Name: "S1"}, State: api.StreamState{LastSeq: 100}, Cluster: &api.ClusterInfo{Leader: "N1"}},
			"N2": &api.StreamInfo{Config: api.StreamConfig{Name: "S1"}, State: api.StreamState{LastSeq: 89}, Cluster: &api.ClusterInfo{Leader: "N1"}},
		})
		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should pass when replicas are close", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_001", map[string]any{
			"N1": &api.StreamInfo{Config: api.StreamConfig{Name: "S1"}, State: api.StreamState{LastSeq: 100}, Cluster: &api.ClusterInfo{Leader: "N1"}},
			"N2": &api.StreamInfo{Config: api.StreamConfig{Name: "S1"}, State: api.StreamState{LastSeq: 90}, Cluster: &api.ClusterInfo{Leader: "N1"}},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	t.Run("Should pass for an empty stream where all replicas have no messages", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_001", map[string]any{
			"N1": &api.StreamInfo{Config: api.StreamConfig{Name: "S1"}, State: api.StreamState{LastSeq: 0}, Cluster: &api.ClusterInfo{Leader: "N1"}},
			"N2": &api.StreamInfo{Config: api.StreamConfig{Name: "S1"}, State: api.StreamState{LastSeq: 0}, Cluster: &api.ClusterInfo{Leader: "N1"}},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	t.Run("Should pass for a single-replica stream", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_001", map[string]any{
			"N1": &api.StreamInfo{Config: api.StreamConfig{Name: "S1"}, State: api.StreamState{LastSeq: 500}, Cluster: &api.ClusterInfo{Leader: "N1"}},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestJETSTREAM_002(t *testing.T) {
	t.Run("Should warn when subject count is too high", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_002", map[string]any{
			"N1": &api.StreamInfo{Config: api.StreamConfig{Name: "S1"}, State: api.StreamState{NumSubjects: 1_500_000}, Cluster: &api.ClusterInfo{Leader: "N1"}},
		})
		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should pass when subject count is normal", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_002", map[string]any{
			"N1": &api.StreamInfo{Config: api.StreamConfig{Name: "S1"}, State: api.StreamState{NumSubjects: 50_000}, Cluster: &api.ClusterInfo{Leader: "N1"}},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	// check uses >, so exactly at the threshold must pass
	t.Run("Should pass when subject count is exactly at the threshold", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_002", map[string]any{
			"N1": &api.StreamInfo{Config: api.StreamConfig{Name: "S1"}, State: api.StreamState{NumSubjects: 1_000_000}, Cluster: &api.ClusterInfo{Leader: "N1"}},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestJETSTREAM_003(t *testing.T) {
	t.Run("Should warn when message usage is near limit", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_003", map[string]any{
			"N1": &api.StreamInfo{
				Config:  api.StreamConfig{Name: "S1", MaxMsgs: 1000},
				State:   api.StreamState{Msgs: 950},
				Cluster: &api.ClusterInfo{Leader: "N1"},
			},
		})
		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should warn when memory usage is near limit", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_003", map[string]any{
			"N1": &api.StreamInfo{
				Config:  api.StreamConfig{Name: "S1", MaxBytes: 1000},
				State:   api.StreamState{Bytes: 950},
				Cluster: &api.ClusterInfo{Leader: "N1"},
			},
		})
		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should warn when consumer count is near limit", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_003", map[string]any{
			"N1": &api.StreamInfo{
				Config:  api.StreamConfig{Name: "S1", MaxConsumers: 10},
				State:   api.StreamState{Consumers: 10},
				Cluster: &api.ClusterInfo{Leader: "N1"},
			},
		})
		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should pass when usage is below limits", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_003", map[string]any{
			"N1": &api.StreamInfo{
				Config:  api.StreamConfig{Name: "S1", MaxMsgs: 1000, MaxBytes: 1000, MaxConsumers: 10},
				State:   api.StreamState{Msgs: 100, Bytes: 100, Consumers: 1},
				Cluster: &api.ClusterInfo{Leader: "N1"},
			},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	t.Run("Should pass when no limits are configured", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_003", map[string]any{
			"N1": &api.StreamInfo{
				Config:  api.StreamConfig{Name: "S1"}, // MaxMsgs=0, MaxBytes=0, MaxConsumers=0
				State:   api.StreamState{Msgs: 99999, Bytes: 99999, Consumers: 999},
				Cluster: &api.ClusterInfo{Leader: "N1"},
			},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	// checkLimit uses >, so exactly at 90% of the limit must pass:
	// MaxMsgs=1000, threshold=int64(1000*0.9)=900; Msgs=900 → 900>900 is false → Pass
	t.Run("Should pass when message usage is exactly at the threshold boundary", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_003", map[string]any{
			"N1": &api.StreamInfo{
				Config:  api.StreamConfig{Name: "S1", MaxMsgs: 1000},
				State:   api.StreamState{Msgs: 900},
				Cluster: &api.ClusterInfo{Leader: "N1"},
			},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestJETSTREAM_004(t *testing.T) {
	t.Run("Should warn when metadata check is enabled and unhealthy", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_004", map[string]any{
			"N1": &api.StreamInfo{
				Config: api.StreamConfig{
					Name: "S1",
					Metadata: map[string]string{
						"io.nats.monitor.enabled":   "true",
						"io.nats.monitor.msgs-warn": "500",
					},
				},
				State:   api.StreamState{Msgs: 400},
				Cluster: &api.ClusterInfo{Leader: "N1"},
			},
		})
		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	// msgs-warn fires when Msgs <= threshold; Msgs=600 is above 500 so no warnings are raised
	t.Run("Should pass when monitoring is enabled and stream is healthy", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_004", map[string]any{
			"N1": &api.StreamInfo{
				Config: api.StreamConfig{
					Name: "S1",
					Metadata: map[string]string{
						"io.nats.monitor.enabled":   "true",
						"io.nats.monitor.msgs-warn": "500",
					},
				},
				State:   api.StreamState{Msgs: 600},
				Cluster: &api.ClusterInfo{Leader: "N1"},
			},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	// msgs-critical fires when Msgs <= threshold
	t.Run("Should fail when metadata check reports a critical", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_004", map[string]any{
			"N1": &api.StreamInfo{
				Config: api.StreamConfig{
					Name: "S1",
					Metadata: map[string]string{
						"io.nats.monitor.enabled":       "true",
						"io.nats.monitor.msgs-critical": "500",
					},
				},
				State:   api.StreamState{Msgs: 100},
				Cluster: &api.ClusterInfo{Leader: "N1"},
			},
		})
		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should pass when monitoring metadata is not enabled", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_004", map[string]any{
			"N1": &api.StreamInfo{
				Config:  api.StreamConfig{Name: "S1"},
				State:   api.StreamState{Msgs: 10},
				Cluster: &api.ClusterInfo{Leader: "N1"},
			},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	t.Run("Should skip non-leader server artifacts", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_004", map[string]any{
			"N1": &api.StreamInfo{ // leader, healthy
				Config: api.StreamConfig{
					Name: "S1",
					Metadata: map[string]string{
						"io.nats.monitor.enabled":       "true",
						"io.nats.monitor.msgs-critical": "500",
					},
				},
				State:   api.StreamState{Msgs: 600},
				Cluster: &api.ClusterInfo{Leader: "N1"},
			},
			"N2": &api.StreamInfo{ // not the leader; unhealthy data must be ignored
				Config: api.StreamConfig{
					Name: "S1",
					Metadata: map[string]string{
						"io.nats.monitor.enabled":       "true",
						"io.nats.monitor.msgs-critical": "500",
					},
				},
				State:   api.StreamState{Msgs: 10},
				Cluster: &api.ClusterInfo{Leader: "N1"}, // N2 is not the leader
			},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestJETSTREAM_005(t *testing.T) {
	t.Run("Should fail when a consumer metadata is enabled and unhealthy", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_005", map[string]any{
			"N1": &streamWithConsumers{
				StreamInfo: api.StreamInfo{
					Config:  api.StreamConfig{Name: "ORDERS"},
					Cluster: &api.ClusterInfo{Leader: "N1"},
				},
				ConsumerDetail: []api.ConsumerInfo{
					{
						Name:    "CRITICAL",
						Stream:  "ORDERS",
						Cluster: &api.ClusterInfo{Leader: "N1"},
						Config: api.ConsumerConfig{
							Metadata: map[string]string{
								"io.nats.monitor.enabled":                  "true",
								"io.nats.monitor.outstanding-ack-critical": "5",
							},
						},
						NumAckPending: 10,
					},
				},
			},
		})
		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should pass when consumer metadata is enabled and is healthy", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_005", map[string]any{
			"N1": &streamWithConsumers{
				StreamInfo: api.StreamInfo{
					Config:  api.StreamConfig{Name: "ORDERS"},
					Cluster: &api.ClusterInfo{Leader: "N1"},
				},
				ConsumerDetail: []api.ConsumerInfo{
					{
						Name:    "HEALTHY",
						Stream:  "ORDERS",
						Cluster: &api.ClusterInfo{Leader: "N1"},
						Config: api.ConsumerConfig{
							Metadata: map[string]string{
								"io.nats.monitor.enabled":                  "true",
								"io.nats.monitor.outstanding-ack-critical": "20",
							},
						},
						NumAckPending: 5,
					},
				},
			},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	t.Run("Should pass when consumer monitoring is not enabled", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_005", map[string]any{
			"N1": &streamWithConsumers{
				StreamInfo: api.StreamInfo{
					Config:  api.StreamConfig{Name: "ORDERS"},
					Cluster: &api.ClusterInfo{Leader: "N1"},
				},
				ConsumerDetail: []api.ConsumerInfo{
					{
						Name:          "NO_MONITORING",
						Stream:        "ORDERS",
						Cluster:       &api.ClusterInfo{Leader: "N1"},
						Config:        api.ConsumerConfig{}, // no monitoring metadata
						NumAckPending: 100,
					},
				},
			},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	t.Run("Should skip consumer checks for non-leader server artifacts", func(t *testing.T) {
		result := setupJetstreamCheck(t, "JETSTREAM_005", map[string]any{
			"N1": &streamWithConsumers{ // leader, healthy consumer
				StreamInfo: api.StreamInfo{
					Config:  api.StreamConfig{Name: "ORDERS"},
					Cluster: &api.ClusterInfo{Leader: "N1"},
				},
				ConsumerDetail: []api.ConsumerInfo{
					{
						Name:    "LEADER_CONSUMER",
						Stream:  "ORDERS",
						Cluster: &api.ClusterInfo{Leader: "N1"},
						Config: api.ConsumerConfig{
							Metadata: map[string]string{
								"io.nats.monitor.enabled":                  "true",
								"io.nats.monitor.outstanding-ack-critical": "20",
							},
						},
						NumAckPending: 5,
					},
				},
			},
			"N2": &streamWithConsumers{ // not the leader; unhealthy consumer data must be ignored
				StreamInfo: api.StreamInfo{
					Config:  api.StreamConfig{Name: "ORDERS"},
					Cluster: &api.ClusterInfo{Leader: "N1"},
				},
				ConsumerDetail: []api.ConsumerInfo{
					{
						Name:    "NON_LEADER_CONSUMER",
						Stream:  "ORDERS",
						Cluster: &api.ClusterInfo{Leader: "N1"}, // N2 is not the leader
						Config: api.ConsumerConfig{
							Metadata: map[string]string{
								"io.nats.monitor.enabled":                  "true",
								"io.nats.monitor.outstanding-ack-critical": "20",
							},
						},
						NumAckPending: 100, // unhealthy, but skipped because not leader
					},
				},
			},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	// Consumer monitoring only defines critical thresholds (no warning thresholds),
	// so the PassWithIssues branch in checkConsumerMetadataMonitoring is unreachable via metadata.
}
