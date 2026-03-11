package audit

import (
	"path/filepath"
	"testing"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/nats-server/v2/server"
)

func setupClusterCheck(t *testing.T, checkid string, artifacts map[string]any, tag *archive.Tag, clusterName string) Outcome {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "audit.zip")

	writer, err := archive.NewWriter(archivePath)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	for serverName, data := range artifacts {
		err := writer.Add(data, archive.TagCluster(clusterName), archive.TagServer(serverName), tag)
		if err != nil {
			t.Fatalf("failed to add artifact for %s: %v", serverName, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	reader, err := archive.NewReader(archivePath)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	cc := &CheckCollection{}
	if err := RegisterClusterChecks(cc); err != nil {
		t.Fatalf("failed to register cluster checks: %v", err)
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
	outcome, err := check.Handler(check, reader, examples, api.NewDefaultLogger(api.ErrorLevel))
	if err != nil {
		t.Fatalf("check handler failed: %v", err)
	}

	return outcome
}

// setupMultiClusterCheck creates an archive with multiple clusters (cluster → server → artifact)
// and runs the named check against it.
func setupMultiClusterCheck(t *testing.T, checkid string, clusters map[string]map[string]any, tag *archive.Tag) Outcome {
	t.Helper()

	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "audit.zip")

	writer, err := archive.NewWriter(archivePath)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	for clusterName, servers := range clusters {
		for serverName, data := range servers {
			err := writer.Add(data, archive.TagCluster(clusterName), archive.TagServer(serverName), tag)
			if err != nil {
				t.Fatalf("failed to add artifact for %s/%s: %v", clusterName, serverName, err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	reader, err := archive.NewReader(archivePath)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	cc := &CheckCollection{}
	if err := RegisterClusterChecks(cc); err != nil {
		t.Fatalf("failed to register cluster checks: %v", err)
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
	outcome, err := check.Handler(check, reader, examples, api.NewDefaultLogger(api.ErrorLevel))
	if err != nil {
		t.Fatalf("check handler failed: %v", err)
	}

	return outcome
}

func TestCLUSTER_001(t *testing.T) {
	t.Run("Should warn if memory usage has an outlier", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_001", map[string]any{
			"s1": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 100_000_000}},
			"s2": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 100_000_000}},
			"s3": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 400_000_000}},
		}, archive.TagServerVars(), "T1")

		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should pass if memory usage is uniform", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_001", map[string]any{
			"s1": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 100_000_000}},
			"s2": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 105_000_000}},
			"s3": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 110_000_000}},
		}, archive.TagServerVars(), "T1")

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	// single-server cluster has no peers to compare against; must not falsely warn
	t.Run("Should pass for a single-server cluster", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_001", map[string]any{
			"s1": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 999_000_000}},
		}, archive.TagServerVars(), "T1")

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	// server at exactly mean*1.5 must not trigger — the check uses >, not >=
	// With [100M, 100M, 200M]: mean = 133.33M, threshold = 200M; s3 at 200M is not > 200M
	t.Run("Should pass when server memory is exactly at the outlier threshold", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_001", map[string]any{
			"s1": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 100_000_000}},
			"s2": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 100_000_000}},
			"s3": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 200_000_000}},
		}, archive.TagServerVars(), "T1")

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	// when VARZ data is nil the server is skipped (numServers stays 0); check must return Pass
	t.Run("Should pass gracefully when VARZ data field is nil", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_001", map[string]any{
			"s1": &server.ServerAPIVarzResponse{}, // Data is nil
		}, archive.TagServerVars(), "T1")

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestCLUSTER_002(t *testing.T) {
	t.Run("Should fail if outbound gateway config mismatches", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_002", map[string]any{
			"s1": &server.ServerAPIGatewayzResponse{Data: &server.Gatewayz{
				OutboundGateways: map[string]*server.RemoteGatewayz{
					"C2": {IsConfigured: true},
				},
			}},
			"s2": &server.ServerAPIGatewayzResponse{Data: &server.Gatewayz{
				OutboundGateways: map[string]*server.RemoteGatewayz{
					"C3": {IsConfigured: true},
				},
			}},
		}, archive.TagServerGateways(), "T1")

		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	// fail case must also cover inbound-only mismatch (outbound identical, inbound differs)
	t.Run("Should fail if inbound gateway config mismatches", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_002", map[string]any{
			"s1": &server.ServerAPIGatewayzResponse{Data: &server.Gatewayz{
				OutboundGateways: map[string]*server.RemoteGatewayz{
					"C2": {IsConfigured: true},
				},
				InboundGateways: map[string][]*server.RemoteGatewayz{
					"C3": {{IsConfigured: true}},
				},
			}},
			"s2": &server.ServerAPIGatewayzResponse{Data: &server.Gatewayz{
				OutboundGateways: map[string]*server.RemoteGatewayz{
					"C2": {IsConfigured: true},
				},
				InboundGateways: map[string][]*server.RemoteGatewayz{
					"C4": {{IsConfigured: true}}, // different inbound source
				},
			}},
		}, archive.TagServerGateways(), "T1")

		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should pass when gateway configs are the same", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_002", map[string]any{
			"s1": &server.ServerAPIGatewayzResponse{Data: &server.Gatewayz{
				OutboundGateways: map[string]*server.RemoteGatewayz{
					"C2": {IsConfigured: true},
				},
				InboundGateways: map[string][]*server.RemoteGatewayz{
					"C3": {{IsConfigured: true}},
				},
			}},
			"s2": &server.ServerAPIGatewayzResponse{Data: &server.Gatewayz{
				OutboundGateways: map[string]*server.RemoteGatewayz{
					"C2": {IsConfigured: true},
				},
				InboundGateways: map[string][]*server.RemoteGatewayz{
					"C3": {{IsConfigured: true}},
				},
			}},
		}, archive.TagServerGateways(), "T1")

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	// servers with no configured gateways are uniform (empty == empty); must pass
	t.Run("Should pass when all servers have no configured gateways", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_002", map[string]any{
			"s1": &server.ServerAPIGatewayzResponse{Data: &server.Gatewayz{}},
			"s2": &server.ServerAPIGatewayzResponse{Data: &server.Gatewayz{}},
		}, archive.TagServerGateways(), "T1")

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestCLUSTER_003(t *testing.T) {
	t.Run("Should warn when HA asset count is too high", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_003", map[string]any{
			"s1": &server.ServerAPIJszResponse{Data: &server.JSInfo{
				JetStreamStats: server.JetStreamStats{
					HAAssets: 1234,
				},
			}},
		}, archive.TagServerJetStream(), "T1")

		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	// multi-server scenario where only one server exceeds the threshold
	t.Run("Should warn when at least one server in a cluster exceeds HA asset threshold", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_003", map[string]any{
			"s1": &server.ServerAPIJszResponse{Data: &server.JSInfo{
				JetStreamStats: server.JetStreamStats{HAAssets: 500},
			}},
			"s2": &server.ServerAPIJszResponse{Data: &server.JSInfo{
				JetStreamStats: server.JetStreamStats{HAAssets: 1234},
			}},
		}, archive.TagServerJetStream(), "T1")

		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should pass when HA asset count is under threshold", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_003", map[string]any{
			"s1": &server.ServerAPIJszResponse{Data: &server.JSInfo{
				JetStreamStats: server.JetStreamStats{
					HAAssets: 1000,
				},
			}},
		}, archive.TagServerJetStream(), "T1")

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	// nil Data is logged as a warning but must not panic or fail the check
	t.Run("Should pass gracefully when JSZ Data field is nil", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_003", map[string]any{
			"s1": &server.ServerAPIJszResponse{}, // Data is nil
		}, archive.TagServerJetStream(), "T1")

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestCLUSTER_004(t *testing.T) {
	// was the only case missing a pass subtest
	t.Run("Should pass when cluster name has no whitespace", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_004", map[string]any{
			"s1": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 100}},
		}, archive.TagServerVars(), "good-cluster-name")

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	t.Run("Should fail when cluster name contains whitespace", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_004", map[string]any{
			"s1": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 100}},
		}, archive.TagServerVars(), "bad cluster name")

		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	// implementation checks for \n but it was never exercised
	t.Run("Should fail when cluster name contains a newline", func(t *testing.T) {
		result := setupClusterCheck(t, "CLUSTER_004", map[string]any{
			"s1": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 100}},
		}, archive.TagServerVars(), "bad\ncluster")

		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	// only one cluster out of several has a bad name; overall result must still be Fail
	t.Run("Should fail when only some clusters have whitespace in their names", func(t *testing.T) {
		result := setupMultiClusterCheck(t, "CLUSTER_004", map[string]map[string]any{
			"good-cluster": {"s1": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 100}}},
			"bad cluster":  {"s2": &server.ServerAPIVarzResponse{Data: &server.Varz{Mem: 100}}},
		}, archive.TagServerVars())

		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})
}
