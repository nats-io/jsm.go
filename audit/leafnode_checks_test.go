package audit

import (
	"path/filepath"
	"testing"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/nats-server/v2/server"
)

func setupLeafCheck(t *testing.T, checkid string, artifacts map[string]any, tag *archive.Tag) Outcome {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "audit.zip")

	writer, err := archive.NewWriter(archivePath)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	for serverName, data := range artifacts {
		err := writer.Add(data, archive.TagCluster("C1"), archive.TagServer(serverName), tag)
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
	if err := RegisterLeafnodeChecks(cc); err != nil {
		t.Fatalf("failed to register leaf checks: %v", err)
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

func TestLEAF_001(t *testing.T) {
	t.Run("Should fail when a leafnode has whitespace in its name", func(t *testing.T) {
		result := setupLeafCheck(t, "LEAF_001", map[string]any{
			"s1": &server.ServerAPILeafzResponse{Data: &server.Leafz{
				Leafs: []*server.LeafInfo{
					{Name: "foo bar"},
					{Name: "ok"},
				},
			}},
		}, archive.TagServerLeafs())

		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should pass when all leafnode names are correct", func(t *testing.T) {
		result := setupLeafCheck(t, "LEAF_001", map[string]any{
			"s1": &server.ServerAPILeafzResponse{Data: &server.Leafz{
				Leafs: []*server.LeafInfo{
					{Name: "edge-A"},
					{Name: "edge-B"},
				},
			}},
		}, archive.TagServerLeafs())

		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}
