package archive

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/jsm.go/api"
)

type testArchive struct {
	ZipReader     *zip.Reader
	FilePath      string
	Manifest      map[string][]*Tag
	StreamPages   map[string]bool
	TotalObjects  int
	ExpectedNames map[int]string
}

func newWriterForTest(t *testing.T, ts time.Time) (*Writer, string) {
	tmpfile := filepath.Join(t.TempDir(), "archive.zip")

	f, err := os.Create(tmpfile)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	w := &Writer{
		zipWriter:    zip.NewWriter(f),
		fileWriter:   f,
		ts:           &ts,
		manifestMap:  make(map[string][]*Tag),
		pagedWriters: make(map[string]*pagedWriter),
	}

	return w, tmpfile
}
func testStreamInfo(index int) *api.StreamInfo {
	return &api.StreamInfo{
		Config: api.StreamConfig{
			Name:     fmt.Sprintf("stream-%02d", index),
			Subjects: []string{fmt.Sprintf("test.%02d", index)},
			Storage:  api.MemoryStorage,
		},
		Created: time.Now().UTC(),
		State: api.StreamState{
			Msgs:     100 + uint64(index),
			Bytes:    2048 + uint64(index),
			FirstSeq: 1,
			LastSeq:  100 + uint64(index),
		},
	}
}

func inspectArchive(t *testing.T, zr *zip.Reader, expectedDir string) (map[string]bool, int, map[string][]*Tag) {
	t.Helper()

	streamPages := map[string]bool{}
	total := 0
	var manifest map[string][]*Tag

	for _, f := range zr.File {
		switch {
		case strings.HasPrefix(f.Name, expectedDir+"/") && strings.HasSuffix(f.Name, ".json"):
			streamPages[f.Name] = true
			rc, _ := f.Open()
			dec := json.NewDecoder(rc)
			for dec.More() {
				var obj map[string]any
				if err := dec.Decode(&obj); err != nil {
					t.Errorf("decode failed in %s: %v", f.Name, err)
				} else {
					total++
				}
			}
			rc.Close()
		case f.Name == "capture/misc/manifest.json":
			rc, _ := f.Open()
			defer rc.Close()
			if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
				t.Fatalf("failed to decode manifest: %v", err)
			}
		}
	}

	return streamPages, total, manifest
}

func buildPagedTestArchive(t *testing.T, totalObjects, objectsPerPage int) *testArchive {
	t.Helper()

	writer, tmpFile := newWriterForTest(t, time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC))
	expected := make(map[int]string)

	tags := []*Tag{
		TagAccount("A"),
		TagCluster("C"),
		TagServer("S"),
		TagStream("TEST_STREAM"),
		TagStreamInfo(),
	}

	pageCount := (totalObjects + objectsPerPage - 1) / objectsPerPage
	for page := 0; page < pageCount; page++ {
		for i := 0; i < objectsPerPage; i++ {
			index := page*objectsPerPage + i
			if index >= totalObjects {
				break
			}
			expected[index] = fmt.Sprintf("stream-%02d", index)
			if err := writer.Add(testStreamInfo(index), tags...); err != nil {
				t.Fatalf("Add failed at index %d: %v", index, err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	zipBytes, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read archive file: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("failed to open zip reader: %v", err)
	}

	expectedDir, err := dirNameFromTags(tags)
	if err != nil {
		t.Fatalf("failed to resolve expected stream_info dir: %v", err)
	}

	streamPages, total, manifest := inspectArchive(t, zr, expectedDir)

	return &testArchive{
		ZipReader:     zr,
		FilePath:      tmpFile,
		Manifest:      manifest,
		StreamPages:   streamPages,
		TotalObjects:  total,
		ExpectedNames: expected,
	}
}

func TestPagedWriter(t *testing.T) {
	ta := buildPagedTestArchive(t, 50, 5)

	t.Run("writes multiple paged files to correct nested paths", func(t *testing.T) {
		if len(ta.StreamPages) < 2 {
			t.Errorf("expected at least 2 stream_info pages, got %d", len(ta.StreamPages))
		}
	})

	t.Run("encodes all entries into paged files", func(t *testing.T) {
		if ta.TotalObjects != 50 {
			t.Errorf("expected 50 entries, got %d", ta.TotalObjects)
		}
	})

	t.Run("includes manifest.json in archive", func(t *testing.T) {
		if ta.Manifest == nil {
			t.Fatal("manifest.json not found in archive")
		}
	})

	t.Run("manifest includes correct paths and tags", func(t *testing.T) {
		tags := []*Tag{
			TagAccount("A"),
			TagCluster("C"),
			TagServer("S"),
			TagStream("TEST_STREAM"),
			TagStreamInfo(),
		}
		expectedDir, err := dirNameFromTags(tags)
		if err != nil {
			t.Fatalf("failed to resolve expected manifest path prefix: %v", err)
		}
		expectedPrefix := expectedDir + "/"

		for path, tags := range ta.Manifest {
			if !strings.HasPrefix(path, expectedPrefix) {
				t.Errorf("unexpected manifest path: %s, expected prefix: %s", path, expectedPrefix)
			}
			found := false
			for _, tag := range tags {
				if tag.Name == typeTagLabel && tag.Value == streamDetailsArtifactType {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("manifest entry %s missing stream_info type tag", path)
			}
		}
	})

	t.Run("each stream_info page contains only complete JSON objects", func(t *testing.T) {
		for page := range ta.StreamPages {
			rc, err := ta.ZipReader.Open(page)
			if err != nil {
				t.Errorf("failed to open page %s: %v", page, err)
				continue
			}
			dec := json.NewDecoder(rc)
			for dec.More() {
				var obj map[string]any
				if err := dec.Decode(&obj); err != nil {
					t.Errorf("decode failed in %s: %v", page, err)
				}
			}
			rc.Close()
		}
	})

	t.Run("addPathToManifest fails on missing type tag", func(t *testing.T) {
		writer := &Writer{
			manifestMap: make(map[string][]*Tag),
		}
		tags := []*Tag{
			TagAccount("A"),
			TagCluster("C"),
			TagServer("S"),
			TagStream("TEST_STREAM"),
		}
		err := writer.addPathToManifest(1, "json", tags)
		if err == nil {
			t.Fatal("expected error due to missing type tag, got nil")
		}
		if !strings.Contains(err.Error(), "missing required tag: artifact type") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("addPathToManifest creates expected directory structures", func(t *testing.T) {
		tests := []struct {
			name string
			tags []*Tag
		}{
			{"server VARZ", []*Tag{TagCluster("CL1"), TagServer("srvA"), TagServerVars()}},
			{"account CONNZ", []*Tag{TagAccount("ACC1"), TagCluster("CL2"), TagServer("srvB"), TagAccountConnections()}},
			{"stream info", []*Tag{TagAccount("ACC2"), TagCluster("CL3"), TagServer("srvC"), TagStream("mystream"), TagStreamInfo()}},
			{"profile", []*Tag{TagCluster("CL4"), TagServer("srvD"), TagServerProfile(), TagProfileName("cpu")}},
			{"special metadata", []*Tag{TagSpecial("audit_metadata")}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				w := &Writer{manifestMap: make(map[string][]*Tag)}
				err := w.addPathToManifest(0, "json", tt.tags)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				var actualPath string
				for key := range w.manifestMap {
					if isNonPagedArtifact(tt.tags) {
						actualPath = strings.TrimSuffix(key, filepath.Ext(key))
					} else {
						actualPath = filepath.Dir(key)
					}
					break
				}
				expectedDir, err := dirNameFromTags(tt.tags)
				if err != nil {
					t.Fatalf("failed to resolve expected path: %v", err)
				}
				if actualPath != expectedDir {
					t.Errorf("expected path %q, got %q", expectedDir, actualPath)
				}
			})
		}
	})

	t.Run("verifies decompressed size and counts all non-manifest files", func(t *testing.T) {
		var totalSize int64
		var counted int
		for _, f := range ta.ZipReader.File {
			if strings.HasSuffix(f.Name, "manifest.json") {
				continue
			}
			counted++
			totalSize += int64(f.UncompressedSize64)
		}
		if counted != 50 {
			t.Errorf("expected 50 files, got %d", counted)
		}
		const minExpectedSize = 10_000
		if totalSize < minExpectedSize {
			t.Errorf("uncompressed size %d bytes is smaller than expected size %d bytes", totalSize, minExpectedSize)
		}
	})
}

func TestPagedReader(t *testing.T) {
	ta := buildPagedTestArchive(t, 50, 5)

	reader, err := NewReader(ta.FilePath)
	if err != nil {
		t.Fatalf("failed to create archive reader: %v", err)
	}

	t.Run("iterates all *api.StreamInfo entries", func(t *testing.T) {
		count := 0
		err := ForEachTaggedArtifact(reader, []*Tag{TagAccount("A"), TagCluster("C"), TagServer("S"), TagStream("TEST_STREAM"), TagStreamInfo()}, func(entry *api.StreamInfo) error {
			count++
			return nil
		})
		if err != nil {
			t.Fatalf("EachPagedByTags failed: %v", err)
		}
		if count != len(ta.ExpectedNames) {
			t.Errorf("expected %d entries, got %d", len(ta.ExpectedNames), count)
		}
	})

	t.Run("stops early when callback returns error", func(t *testing.T) {
		count := 0
		err := ForEachTaggedArtifact(reader, []*Tag{TagAccount("A"), TagCluster("C"), TagServer("S"), TagStream("TEST_STREAM"), TagStreamInfo()}, func(entry *api.StreamInfo) error {
			count++
			if count == 5 {
				return fmt.Errorf("stop here")
			}
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "stop here") {
			t.Errorf("expected error 'stop here', got %v", err)
		}
		if count != 5 {
			t.Errorf("expected early exit at 5, got %d", count)
		}
	})

	t.Run("verifies *api.StreamInfo object content", func(t *testing.T) {
		seen := map[int]string{}

		err := ForEachTaggedArtifact(reader, []*Tag{TagAccount("A"), TagCluster("C"), TagServer("S"), TagStream("TEST_STREAM"), TagStreamInfo()}, func(info *api.StreamInfo) error {
			var i int
			if _, err := fmt.Sscanf(info.Config.Name, "stream-%02d", &i); err != nil {
				t.Errorf("invalid stream name: %q", info.Config.Name)
				return nil
			}
			seen[i] = info.Config.Name

			if info.State.Msgs < 100 {
				t.Errorf("stream %q has too few messages", info.Config.Name)
			}
			return nil
		})

		if err != nil {
			t.Fatalf("EachPagedByTags failed: %v", err)
		}

		if len(seen) != len(ta.ExpectedNames) {
			t.Errorf("expected %d entries, got %d", len(ta.ExpectedNames), len(seen))
		}

		for i, want := range ta.ExpectedNames {
			if got := seen[i]; got != want {
				t.Errorf("entry %d: got %q, want %q", i, got, want)
			}
		}
	})

	t.Run("EachClusterServerArtifact iterates all expected entries", func(t *testing.T) {
		expectedCluster := "C"
		expectedServer := "S"
		expectedCount := len(ta.ExpectedNames)

		count := 0
		count, err := EachClusterServerArtifact(reader, TagStreamInfo(), func(clusterTag *Tag, serverTag *Tag, err error, artifact *api.StreamInfo) error {
			if err != nil {
				t.Errorf("unexpected error for %s/%s: %v", clusterTag.Value, serverTag.Value, err)
				return nil
			}
			if clusterTag.Value != expectedCluster || serverTag.Value != expectedServer {
				t.Errorf("unexpected cluster/server: %s/%s", clusterTag.Value, serverTag.Value)
			}
			count++
			return nil
		})
		if err != nil {
			t.Fatalf("EachClusterServerArtifact failed: %v", err)
		}
		if count != expectedCount {
			t.Errorf("expected %d entries, got %d", expectedCount, count)
		}
	})

}
