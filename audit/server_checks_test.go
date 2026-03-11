package audit

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/nats-server/v2/server"
)

func setupServerCheck(t *testing.T, checkid string, artifacts map[string]any, typeTag *archive.Tag) Outcome {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "audit.zip")

	writer, err := archive.NewWriter(archivePath)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	for serverName, artifact := range artifacts {
		err := writer.Add(artifact,
			archive.TagCluster("C1"),
			archive.TagServer(serverName),
			typeTag)
		if err != nil {
			t.Fatalf("failed to add artifact: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	reader, err := archive.NewReader(archivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	defer reader.Close()

	cc := &CheckCollection{}
	if err := RegisterServerChecks(cc); err != nil {
		t.Fatalf("failed to register checks: %v", err)
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
	result, err := check.Handler(check, reader, examples, api.NewDefaultLogger(api.ErrorLevel))
	if err != nil {
		t.Fatalf("check handler failed: %v", err)
	}

	return result
}

func TestSERVER_001(t *testing.T) {
	t.Run("Should warn when a server reports non-ok status", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_001", map[string]any{
			"n1": &server.ServerAPIHealthzResponse{Data: &server.HealthStatus{Status: "ok"}},
			"n2": &server.ServerAPIHealthzResponse{Data: &server.HealthStatus{Status: "fail", StatusCode: 500}},
		}, archive.TagServerHealth())

		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should pass when all servers are healthy", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_001", map[string]any{
			"n1": &server.ServerAPIHealthzResponse{Data: &server.HealthStatus{Status: "ok"}},
			"n2": &server.ServerAPIHealthzResponse{Data: &server.HealthStatus{Status: "ok"}},
		}, archive.TagServerHealth())

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestSERVER_002(t *testing.T) {
	t.Run("Should fail when servers have different versions", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_002", map[string]any{
			"n1": &server.ServerAPIVarzResponse{
				Data: &server.Varz{Version: "2.11.0"},
			},
			"n2": &server.ServerAPIVarzResponse{
				Data: &server.Varz{Version: "2.10.8"},
			},
		}, archive.TagServerVars())

		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should pass when all servers have the same version", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_002", map[string]any{
			"n1": &server.ServerAPIVarzResponse{
				Data: &server.Varz{Version: "2.11.0"},
			},
			"n2": &server.ServerAPIVarzResponse{
				Data: &server.Varz{Version: "2.11.0"},
			},
		}, archive.TagServerVars())

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestSERVER_003(t *testing.T) {
	t.Run("Should fail when average CPU usage exceeds threshold", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_003", map[string]any{
			"n1": &server.ServerAPIVarzResponse{Data: &server.Varz{CPU: 361.0, Cores: 4}},
		}, archive.TagServerVars())

		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should pass when average CPU usage is within threshold", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_003", map[string]any{
			"n1": &server.ServerAPIVarzResponse{Data: &server.Varz{CPU: 360.0, Cores: 4}},
		}, archive.TagServerVars())

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	t.Run("Should skip server when Cores is zero", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_003", map[string]any{
			"n1": &server.ServerAPIVarzResponse{Data: &server.Varz{CPU: 999.0, Cores: 0}},
		}, archive.TagServerVars())

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestSERVER_004(t *testing.T) {
	t.Run("Should warn when server reports slow consumers", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_004", map[string]any{
			"n1": &server.ServerAPIVarzResponse{Data: &server.Varz{SlowConsumers: 3}},
		}, archive.TagServerVars())

		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should pass when no slow consumers", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_004", map[string]any{
			"n1": &server.ServerAPIVarzResponse{Data: &server.Varz{SlowConsumers: 0}},
		}, archive.TagServerVars())

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestSERVER_005(t *testing.T) {
	t.Run("Should warn when memory usage exceeds threshold", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_005", map[string]any{
			"n1": &server.ServerAPIJszResponse{
				Data: &server.JSInfo{
					Config: server.JetStreamConfig{
						MaxMemory: 1000,
					},
					JetStreamStats: server.JetStreamStats{
						Memory: 901,
					},
				},
			},
		}, archive.TagServerJetStream())

		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should warn when store usage exceeds threshold", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_005", map[string]any{
			"n1": &server.ServerAPIJszResponse{
				Data: &server.JSInfo{
					Config: server.JetStreamConfig{
						MaxStore: 1000,
					},
					JetStreamStats: server.JetStreamStats{
						Store: 901,
					},
				},
			},
		}, archive.TagServerJetStream())

		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should pass when memory and store usage are below threshold", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_005", map[string]any{
			"n1": &server.ServerAPIJszResponse{
				Data: &server.JSInfo{
					Config: server.JetStreamConfig{
						MaxMemory: 1000,
						MaxStore:  1000,
					},
					JetStreamStats: server.JetStreamStats{
						Memory: 500,
						Store:  500,
					},
				},
			},
		}, archive.TagServerJetStream())

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestSERVER_006(t *testing.T) {
	t.Run("Should warn when JetStream domain contains whitespace", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_006", map[string]any{
			"n1": &server.ServerAPIJszResponse{
				Data: &server.JSInfo{
					Config: server.JetStreamConfig{Domain: "foo bar"},
				},
			},
		}, archive.TagServerJetStream())

		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should pass when domain has no whitespace", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_006", map[string]any{
			"n1": &server.ServerAPIJszResponse{
				Data: &server.JSInfo{
					Config: server.JetStreamConfig{Domain: "foobar"},
				},
			},
		}, archive.TagServerJetStream())

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestSERVER_007(t *testing.T) {
	t.Run("Should warn when authentication is not required", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_007", map[string]any{
			"n1": &server.ServerAPIVarzResponse{
				Data: &server.Varz{AuthRequired: false},
			},
		}, archive.TagServerVars())

		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should pass when authentication is required", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_007", map[string]any{
			"n1": &server.ServerAPIVarzResponse{
				Data: &server.Varz{AuthRequired: true},
			},
		}, archive.TagServerVars())

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestSERVER_008(t *testing.T) {
	now := time.Now().UTC()

	t.Run("Should fail when a certificate expires within the critical threshold", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_008", map[string]any{
			"n1": &server.ServerAPIVarzResponse{
				Data: &server.Varz{
					Now:             now,
					TLSCertNotAfter: now.Add(24 * time.Hour),
				},
			},
		}, archive.TagServerVars())

		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should fail when a certificate is already expired", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_008", map[string]any{
			"n1": &server.ServerAPIVarzResponse{
				Data: &server.Varz{
					Now:             now,
					TLSCertNotAfter: now.Add(-24 * time.Hour),
				},
			},
		}, archive.TagServerVars())

		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should warn when a certificate expires within the warning threshold", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_008", map[string]any{
			"n1": &server.ServerAPIVarzResponse{
				Data: &server.Varz{
					Now:             now,
					TLSCertNotAfter: now.Add(10 * 24 * time.Hour),
				},
			},
		}, archive.TagServerVars())

		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should pass when certificates are not expiring soon", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_008", map[string]any{
			"n1": &server.ServerAPIVarzResponse{
				Data: &server.Varz{
					Now:             now,
					TLSCertNotAfter: now.Add(90 * 24 * time.Hour),
				},
			},
		}, archive.TagServerVars())

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	t.Run("Should skip when no TLS certificates are present", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_008", map[string]any{
			"n1": &server.ServerAPIVarzResponse{
				Data: &server.Varz{Now: now},
			},
		}, archive.TagServerVars())

		if result != Skipped {
			t.Errorf("expected result %v, got %v", Skipped, result)
		}
	})

	t.Run("Should fail when a cluster TLS certificate is expired", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_008", map[string]any{
			"n1": &server.ServerAPIVarzResponse{
				Data: &server.Varz{
					Now: now,
					Cluster: server.ClusterOptsVarz{
						TLSCertNotAfter: now.Add(-1 * time.Hour),
					},
				},
			},
		}, archive.TagServerVars())

		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should fail when a gateway TLS certificate is expiring critically", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_008", map[string]any{
			"n1": &server.ServerAPIVarzResponse{
				Data: &server.Varz{
					Now: now,
					Gateway: server.GatewayOptsVarz{
						TLSCertNotAfter: now.Add(24 * time.Hour),
					},
				},
			},
		}, archive.TagServerVars())

		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should warn when a leafnode TLS certificate is expiring soon", func(t *testing.T) {
		result := setupServerCheck(t, "SERVER_008", map[string]any{
			"n1": &server.ServerAPIVarzResponse{
				Data: &server.Varz{
					Now: now,
					LeafNode: server.LeafNodeOptsVarz{
						TLSCertNotAfter: now.Add(10 * 24 * time.Hour),
					},
				},
			},
		}, archive.TagServerVars())

		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})
}

func TestSERVER_Skipped(t *testing.T) {
	checks := []string{"SERVER_001", "SERVER_002", "SERVER_003", "SERVER_004", "SERVER_005", "SERVER_006", "SERVER_007", "SERVER_008"}

	for _, checkid := range checks {
		t.Run("Empty archive skips "+checkid, func(t *testing.T) {
			result := setupServerCheck(t, checkid, map[string]any{}, archive.TagServerVars())
			if result != Pass && result != Skipped {
				t.Errorf("expected Pass or Skipped for empty archive, got %v", result)
			}
		})
	}
}
