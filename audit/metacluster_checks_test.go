package audit

import (
	"path/filepath"
	"testing"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/nats-server/v2/server"
)

func setupMetaCheck(t *testing.T, checkid string, artifacts map[string]any) Outcome {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "audit.zip")

	writer, err := archive.NewWriter(archivePath)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	for serverName, data := range artifacts {
		err := writer.Add(data, archive.TagCluster("C1"), archive.TagServer(serverName), archive.TagServerJetStream())
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
	if err := RegisterMetaChecks(cc); err != nil {
		t.Fatalf("failed to register meta checks: %v", err)
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

func TestMETA_001(t *testing.T) {
	t.Run("Should fail when a meta replica is offline", func(t *testing.T) {
		result := setupMetaCheck(t, "META_001", map[string]any{
			"s1": &server.ServerAPIJszResponse{
				Data: &server.JSInfo{
					Meta: &server.MetaClusterInfo{
						Replicas: []*server.PeerInfo{
							{Name: "s2", Offline: true},
						},
					},
				},
			},
		})
		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should pass when all replicas are online", func(t *testing.T) {
		result := setupMetaCheck(t, "META_001", map[string]any{
			"s1": &server.ServerAPIJszResponse{
				Data: &server.JSInfo{
					Meta: &server.MetaClusterInfo{
						Replicas: []*server.PeerInfo{
							{Name: "s2", Offline: false},
						},
					},
				},
			},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}

func TestMETA_002(t *testing.T) {
	t.Run("Should fail when servers disagree on meta leader", func(t *testing.T) {
		result := setupMetaCheck(t, "META_002", map[string]any{
			"s1": &server.ServerAPIJszResponse{
				Data: &server.JSInfo{
					Meta: &server.MetaClusterInfo{Leader: "s1"},
				},
			},
			"s2": &server.ServerAPIJszResponse{
				Data: &server.JSInfo{
					Meta: &server.MetaClusterInfo{Leader: "s2"},
				},
			},
		})
		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should pass when all servers agree on the meta leader", func(t *testing.T) {
		result := setupMetaCheck(t, "META_002", map[string]any{
			"s1": &server.ServerAPIJszResponse{
				Data: &server.JSInfo{
					Meta: &server.MetaClusterInfo{Leader: "s1"},
				},
			},
			"s2": &server.ServerAPIJszResponse{
				Data: &server.JSInfo{
					Meta: &server.MetaClusterInfo{Leader: "s1"},
				},
			},
		})
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}
