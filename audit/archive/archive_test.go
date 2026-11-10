// Copyright 2024-2026 The NATS Authors
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

package archive

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func expectedPagedFile(t *testing.T, extension string, tags ...*Tag) string {
	t.Helper()
	dir, err := dirNameFromTags(tags)
	if err != nil {
		t.Fatalf("failed to generate path from tags %+v: %v", tags, err)
	}
	return filepath.Join(dir, "0001."+extension)
}

func expectedSpecialFile(name, extension string) string {
	return filepath.Join("capture", "misc", name+"."+extension)
}

func Test_CreateThenReadArchive(t *testing.T) {
	rng := rand.New(rand.NewSource(123456))

	archivePath := filepath.Join(t.TempDir(), "archive.zip")
	aw, err := NewWriter(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive: %s", err)
	}

	files := map[string][]byte{
		"empty_file.txt": make([]byte, 0),
		"2KB_file.bin":   make([]byte, 2048),
		"2MB_file.bin":   make([]byte, 2048*1024),
	}

	for fileName, fileContent := range files {
		_, err = rng.Read(fileContent)
		if err != nil {
			t.Fatalf("Failed to generate random file contents: %s", err)
		}
		err = aw.addArtifact(fileName, bytes.NewReader(fileContent))
		if err != nil {
			t.Fatalf("Failed to add file '%s': %s", fileName, err)
		}
	}

	err = aw.Close()
	if err != nil {
		t.Fatalf("Error closing writer: %s", err)
	}

	fileInfo, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("Failed to get archive stats: %s", err)
	}
	t.Logf("Archive file size: %d KiB", fileInfo.Size()/1024)

	ar, err := NewReader(archivePath)
	defer func(ar *Reader) {
		err := ar.Close()
		if err != nil {
			t.Logf("Archive close error: %s", err)
		}
	}(ar)
	if err != nil {
		t.Fatalf("Failed to create archive: %s", err)
	}

	expectedArtifactsCount := len(files) + 1 // (+1 for manifest)
	if expectedArtifactsCount != ar.rawFilesCount() {
		t.Fatalf("Wrong number of artifacts. Expected: %d actual: %d", expectedArtifactsCount, ar.rawFilesCount())
	}

	for fileName, fileContent := range files {
		fileReader, size, err := ar.getFileReader(fileName)
		if err != nil {
			t.Fatalf("Failed to get file: %s: %s", fileName, err)
		}
		defer fileReader.Close()

		if uint64(len(fileContent)) != size {
			t.Fatalf("File %s size mismatch: %d vs. %d", fileName, len(fileContent), size)
		}

		buf, err := io.ReadAll(fileReader)
		if err != nil {
			t.Fatalf("Failed to read content of %s: %s", fileName, err)
		}

		if !bytes.Equal(fileContent, buf) {
			t.Fatalf("File %s content mismatch", fileName)
		}

		t.Logf("Verified file %s, uncompressed size: %dB", fileName, size)
	}
}

func Test_CreateThenReadArchiveUsingTags(t *testing.T) {
	rng := rand.New(rand.NewSource(123456))

	archivePath := filepath.Join(t.TempDir(), "archive.zip")
	aw, err := NewWriter(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive: %s", err)
	}

	clusters := map[string][]string{
		"C1": {
			"X",
			"Y",
			"Z",
		},
		"C2": {
			"A",
			"B",
			"C",
			"D",
			"E",
		},
	}

	type DummyRecord struct {
		FooString string
		BarInt    int
		BazBytes  []byte
	}

	type DummyHealthStats DummyRecord
	type DummyClusterInfo DummyRecord
	type DummyServerInfo DummyRecord
	type DummyStreamInfo DummyRecord
	type DummyAccountInfo DummyRecord

	expectedClusters := make([]string, 0, 2)
	expectedServers := make([]string, 0, 8)

	for clusterName, clusterServers := range clusters {
		expectedClusters = append(expectedClusters, clusterName)

		var err error
		// Add one (dummy) cluster info for each cluster
		ci := &DummyClusterInfo{
			FooString: clusterName,
			BarInt:    rng.Int(),
			BazBytes:  make([]byte, 100),
		}
		rng.Read(ci.BazBytes)
		err = aw.Add(ci, TagCluster(clusterName), TagServer(clusterServers[0]), TagArtifactType("cluster_info"))
		if err != nil {
			t.Fatalf("Failed to add cluster info: %s", err)
		}

		for _, serverName := range clusterServers {
			expectedServers = append(expectedServers, serverName)

			// Add one (dummy) health stats for each server
			hs := &DummyHealthStats{
				FooString: serverName,
				BarInt:    rng.Int(),
				BazBytes:  make([]byte, 50),
			}
			rng.Read(hs.BazBytes)

			err = aw.Add(hs, TagCluster(clusterName), TagServer(serverName), TagServerHealth())
			if err != nil {
				t.Fatalf("Failed to add server health: %s", err)
			}

			// Add one (dummy) server info for each server
			si := &DummyServerInfo{
				FooString: serverName,
				BarInt:    rng.Int(),
				BazBytes:  make([]byte, 50),
			}
			rng.Read(si.BazBytes)

			err = aw.Add(si, TagCluster(clusterName), TagServer(serverName), TagArtifactType("server_info"))
			if err != nil {
				t.Fatalf("Failed to add server health: %s", err)
			}
		}
	}
	slices.Sort(expectedClusters)
	slices.Sort(expectedServers)

	// Add account info
	globalAccountName := "$G"
	for _, serverName := range clusters["C1"] {

		si := &DummyAccountInfo{
			FooString: globalAccountName,
			BarInt:    rng.Int(),
			BazBytes:  make([]byte, 50),
		}
		rng.Read(si.BazBytes)
		err = aw.Add(si, TagAccount(globalAccountName), TagCluster("C1"), TagServer(serverName), TagArtifactType("account_info"))
		if err != nil {
			t.Fatalf("Failed to add account info: %s", err)
		}
	}

	// Add some stream artifacts
	streamName := "ORDERS"
	streamAccount := globalAccountName
	streamReplicas := []string{"A", "B", "E"}
	for _, streamReplicaServerName := range streamReplicas {
		// Add one (dummy) health stats for each server
		si := &DummyStreamInfo{
			FooString: streamAccount + "_" + streamName + "_" + streamReplicaServerName,
			BarInt:    rng.Int(),
			BazBytes:  make([]byte, 50),
		}
		rng.Read(si.BazBytes)

		tags := []*Tag{
			TagAccount(streamAccount),
			TagServer(streamReplicaServerName),
			TagStream(streamName),
			TagArtifactType("stream_info"),
			TagCluster("C2"),
		}

		err = aw.Add(si, tags...)
		if err != nil {
			t.Fatalf("Failed to add stream info: %s", err)
		}
	}

	expectedMessageBytes := []byte("Hello World!")
	err = aw.AddRaw(bytes.NewReader(expectedMessageBytes), "txt", TagSpecial("message"))
	if err != nil {
		t.Fatalf("Failed to raw artifact: %s", err)
	}

	err = aw.Close()
	if err != nil {
		t.Fatalf("Error closing writer: %s", err)
	}

	fileInfo, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("Failed to get archive stats: %s", err)
	}
	t.Logf("Archive file size: %d KiB", fileInfo.Size()/1024)

	ar, err := NewReader(archivePath)
	defer func(ar *Reader) {
		err := ar.Close()
		if err != nil {
			t.Logf("Archive close error: %s", err)
		}
	}(ar)
	if err != nil {
		t.Fatalf("Failed to open archive: %s", err)
	}

	expectedFilesList := []string{
		// Server health
		expectedPagedFile(t, "json", TagCluster("C1"), TagServer("X"), TagServerHealth()),
		expectedPagedFile(t, "json", TagCluster("C1"), TagServer("Y"), TagServerHealth()),
		expectedPagedFile(t, "json", TagCluster("C1"), TagServer("Z"), TagServerHealth()),
		expectedPagedFile(t, "json", TagCluster("C2"), TagServer("A"), TagServerHealth()),
		expectedPagedFile(t, "json", TagCluster("C2"), TagServer("B"), TagServerHealth()),
		expectedPagedFile(t, "json", TagCluster("C2"), TagServer("C"), TagServerHealth()),
		expectedPagedFile(t, "json", TagCluster("C2"), TagServer("D"), TagServerHealth()),
		expectedPagedFile(t, "json", TagCluster("C2"), TagServer("E"), TagServerHealth()),

		// Server info
		expectedPagedFile(t, "json", TagCluster("C1"), TagServer("X"), TagArtifactType("server_info")),
		expectedPagedFile(t, "json", TagCluster("C1"), TagServer("Y"), TagArtifactType("server_info")),
		expectedPagedFile(t, "json", TagCluster("C1"), TagServer("Z"), TagArtifactType("server_info")),
		expectedPagedFile(t, "json", TagCluster("C2"), TagServer("A"), TagArtifactType("server_info")),
		expectedPagedFile(t, "json", TagCluster("C2"), TagServer("B"), TagArtifactType("server_info")),
		expectedPagedFile(t, "json", TagCluster("C2"), TagServer("C"), TagArtifactType("server_info")),
		expectedPagedFile(t, "json", TagCluster("C2"), TagServer("D"), TagArtifactType("server_info")),
		expectedPagedFile(t, "json", TagCluster("C2"), TagServer("E"), TagArtifactType("server_info")),

		// Cluster info
		expectedPagedFile(t, "json", TagCluster("C1"), TagServer("X"), TagArtifactType("cluster_info")),
		expectedPagedFile(t, "json", TagCluster("C2"), TagServer("A"), TagArtifactType("cluster_info")),

		// Stream info
		expectedPagedFile(t, "json", TagAccount("$G"), TagCluster("C2"), TagServer("A"), TagStream("ORDERS"), TagArtifactType("stream_info")),
		expectedPagedFile(t, "json", TagAccount("$G"), TagCluster("C2"), TagServer("B"), TagStream("ORDERS"), TagArtifactType("stream_info")),
		expectedPagedFile(t, "json", TagAccount("$G"), TagCluster("C2"), TagServer("E"), TagStream("ORDERS"), TagArtifactType("stream_info")),

		// Account info
		expectedPagedFile(t, "json", TagAccount("$G"), TagCluster("C1"), TagServer("X"), TagArtifactType("account_info")),
		expectedPagedFile(t, "json", TagAccount("$G"), TagCluster("C1"), TagServer("Y"), TagArtifactType("account_info")),
		expectedPagedFile(t, "json", TagAccount("$G"), TagCluster("C1"), TagServer("Z"), TagArtifactType("account_info")),

		// Misc
		expectedSpecialFile("message", "txt"),
	}
	expectedArtifactsCount := len(expectedFilesList) + 1 // +1 for manifest
	if expectedArtifactsCount != ar.rawFilesCount() {
		t.Fatalf("Wrong number of artifacts. Expected: %d actual: %d", expectedArtifactsCount, ar.rawFilesCount())
	}

	t.Logf("Listing archive contents:")
	for fileName := range ar.filesMap {
		t.Logf(" - %s", fileName)
	}

	for _, fileName := range expectedFilesList {
		if fileName == "capture/misc/message.txt" {
			// Don't try to deserialize text file
			continue
		}
		var r DummyRecord
		err := ar.loadFile(fileName, &r)
		if err != nil {
			t.Fatalf("Failed to load artifact: %s: %s", fileName, err)
		}
		//t.Logf("%s: %+v", fileName, r)
		if r.FooString == "" {
			t.Fatalf("Unexpected empty structure field for file %s", fileName)
		}
	}

	fileReader, _, err := ar.getFileReader("capture/misc/message.txt")
	if err != nil {
		t.Fatalf("Failed to open message file reader: %s", err)
	}
	messageBytes, err := io.ReadAll(fileReader)
	if err != nil {
		t.Fatalf("Failed to read message: %s", err)
	}
	if !bytes.Equal(messageBytes, expectedMessageBytes) {
		t.Fatalf("Expected message: %s, actual: %s", expectedMessageBytes, messageBytes)
	}

	for _, serverName := range expectedServers {
		var si DummyServerInfo
		err := ar.Load(&si, TagServer(serverName), TagArtifactType("server_info"))
		if err != nil {
			t.Fatalf("Failed to load server info artifact for server %s: %s", serverName, err)
		}
		if serverName != si.FooString {
			t.Fatalf("Unexpected value '%s' (should be: '%s')", si.FooString, serverName)
		}
	}

	var clusterNames []string
	for _, cluster := range ar.Clusters() {
		if cluster.Domain != "" {
			t.Fatalf("Unexpected domain for cluster %s", cluster)
		}
		clusterNames = append(clusterNames, cluster.Name)
	}
	if slices.Compare(clusterNames, expectedClusters) != 0 {
		t.Fatalf("Expected clusters: %v, got: %v", expectedClusters, clusterNames)
	}

	for _, clusterName := range clusterNames {
		serverNames := ar.ClusterServerNames(ClusterRef{Name: clusterName})
		if slices.Compare(clusters[clusterName], serverNames) != 0 {
			t.Fatalf("Expected cluster %s servers: %v, got: %v", clusterName, clusters[clusterName], serverNames)
		}
	}

	expectedAccountNames := []string{globalAccountName}
	accountNames := ar.AccountNames()
	if slices.Compare(expectedAccountNames, accountNames) != 0 {
		t.Fatalf("Expected accounts: %v, got: %v", expectedAccountNames, accountNames)
	}

	expectedStreams := []StreamRef{{Name: "ORDERS"}}
	streams := ar.AccountStreams(globalAccountName)
	if !slices.Equal(expectedStreams, streams) {
		t.Fatalf("Expected account %s streams: %v, got: %v", globalAccountName, expectedStreams, streams)
	}

	expectedReplicaNames := []string{"A", "B", "E"}
	replicaNames := ar.StreamServerNames(globalAccountName, StreamRef{Name: "ORDERS"})
	if slices.Compare(expectedReplicaNames, replicaNames) != 0 {
		t.Fatalf("Expected stream %s/%s replicas: %v, got: %v", globalAccountName, "ORDERS", expectedReplicaNames, replicaNames)
	}

	var foo struct{}
	if err = ar.Load(&foo, TagCluster("C1"), TagServer("A")); !errors.Is(err, ErrNoMatches) {
		t.Fatalf("Expected error '%s', but got: '%s'", ErrNoMatches, err)
	}
	if err = ar.Load(&foo, TagServerHealth()); !errors.Is(err, ErrMultipleMatches) {
		t.Fatalf("Expected error '%s', but got: '%s'", ErrMultipleMatches, err)
	}
}

func Test_IterateResourcesUsingTags(t *testing.T) {
	rng := rand.New(rand.NewSource(123456))

	dummyArtifact := struct {
		x int
		y []byte
	}{
		x: rng.Int(),
	}
	rng.Read(dummyArtifact.y)

	archivePath := filepath.Join(t.TempDir(), "archive.zip")
	aw, err := NewWriter(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive: %s", err)
	}

	clusterServerMap := map[string][]string{
		"C1": {"A", "B", "C"},
		"C2": {"X", "Y", "Z"},
	}

	expectedClusterNames := []string{
		"C1",
		"C2",
	}
	slices.SortFunc(expectedClusterNames, strings.Compare)

	for clusterName, serverNames := range clusterServerMap {
		for _, serverName := range serverNames {
			err = aw.Add(
				dummyArtifact,
				TagCluster(clusterName),
				TagServer(serverName),
				TagServerHealth(),
			)
			if err != nil {
				t.Fatalf("Failed to add artifact: %s", err)
			}
		}
	}

	err = aw.Close()
	if err != nil {
		t.Fatalf("Error closing writer: %s", err)
	}

	// Done writing, now verify

	ar, err := NewReader(archivePath)
	defer func(ar *Reader) {
		err := ar.Close()
		if err != nil {
			t.Logf("Failed to close reader: %s", err)
		}
	}(ar)
	if err != nil {
		t.Fatalf("Failed to open archive: %s", err)
	}

	var clusterNames []string
	for _, cluster := range ar.Clusters() {
		clusterNames = append(clusterNames, cluster.Name)
	}

	if !slices.Equal(clusterNames, expectedClusterNames) {
		t.Fatalf("Expected clusters: %v, actual: %v", expectedClusterNames, clusterNames)
	}

	if len(ar.ClusterServerNames(ClusterRef{Name: "NO_SUCH_CLUSTER"})) != 0 {
		t.Fatalf("Looking up non-existent cluster produced some results")
	}

	for clusterName, expectedServerNames := range clusterServerMap {
		serverNames := ar.ClusterServerNames(ClusterRef{Name: clusterName})
		slices.SortFunc(expectedServerNames, strings.Compare)
		slices.SortFunc(serverNames, strings.Compare)
		if !slices.Equal(serverNames, expectedServerNames) {
			t.Fatalf("Expected cluster %s servers: %v, actual: %v", clusterName, expectedServerNames, serverNames)
		}
	}
}

func Test_WriterOverwritesExistingFile(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "archive.zip")

	// Write first archive with one artifact
	aw1, err := NewWriter(archivePath)
	if err != nil {
		t.Fatalf("Failed to create first archive: %s", err)
	}
	type Record struct{ Value string }
	err = aw1.Add(Record{"first"}, TagCluster("C1"), TagServer("S1"), TagServerHealth())
	if err != nil {
		t.Fatalf("Failed to add to first archive: %s", err)
	}
	if err = aw1.Close(); err != nil {
		t.Fatalf("Failed to close first archive: %s", err)
	}

	// Write second archive at same path with a different artifact
	aw2, err := NewWriter(archivePath)
	if err != nil {
		t.Fatalf("Failed to create second archive at same path: %s", err)
	}
	err = aw2.Add(Record{"second"}, TagCluster("C2"), TagServer("S2"), TagServerVars())
	if err != nil {
		t.Fatalf("Failed to add to second archive: %s", err)
	}
	if err = aw2.Close(); err != nil {
		t.Fatalf("Failed to close second archive: %s", err)
	}

	ar, err := NewReader(archivePath)
	if err != nil {
		t.Fatalf("Failed to open overwritten archive: %s", err)
	}
	defer ar.Close()

	// The second archive should have C2/S2 but not C1/S1
	if clusters := ar.Clusters(); len(clusters) != 1 || clusters[0] != (ClusterRef{Name: "C2"}) {
		t.Fatalf("Expected only cluster C2 after overwrite, got: %v", clusters)
	}

	var r Record
	if err = ar.Load(&r, TagCluster("C2"), TagServer("S2"), TagServerVars()); err != nil {
		t.Fatalf("Failed to load artifact from overwritten archive: %s", err)
	}
	if r.Value != "second" {
		t.Fatalf("Expected value 'second', got: %s", r.Value)
	}
}

func Test_WriterCreationInNonExistingDirectoryFails(t *testing.T) {
	nonExistentDir := filepath.Join(t.TempDir(), "does", "not", "exist")
	_, err := NewWriter(filepath.Join(nonExistentDir, "archive.zip"))
	if err == nil {
		t.Fatal("Expected error when creating archive in non-existent directory, got nil")
	}
}

func Test_AddSameTagsTwiceProducesTwoPages(t *testing.T) {
	type Record struct{ Value string }

	archivePath := filepath.Join(t.TempDir(), "archive.zip")
	aw, err := NewWriter(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive: %s", err)
	}

	tags := []*Tag{TagCluster("C1"), TagServer("S1"), TagServerHealth()}
	if err = aw.Add(Record{"first"}, tags...); err != nil {
		t.Fatalf("Failed to add first artifact: %s", err)
	}
	if err = aw.Add(Record{"second"}, tags...); err != nil {
		t.Fatalf("Failed to add second artifact: %s", err)
	}
	if err = aw.Close(); err != nil {
		t.Fatalf("Failed to close archive: %s", err)
	}

	ar, err := NewReader(archivePath)
	if err != nil {
		t.Fatalf("Failed to open archive: %s", err)
	}
	defer ar.Close()

	var seen []string
	err = ForEachTaggedArtifact(ar, tags, func(r *Record) error {
		seen = append(seen, r.Value)
		return nil
	})
	if err != nil {
		t.Fatalf("ForEachTaggedArtifact failed: %s", err)
	}
	if len(seen) != 2 {
		t.Fatalf("Expected 2 entries from duplicate tags, got %d: %v", len(seen), seen)
	}
	if seen[0] != "first" || seen[1] != "second" {
		t.Fatalf("Unexpected order or values: %v", seen)
	}
}

func Test_NonUniqueServerNameAcrossClusters(t *testing.T) {
	type Record struct{ Cluster, Server string }

	archivePath := filepath.Join(t.TempDir(), "archive.zip")
	aw, err := NewWriter(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive: %s", err)
	}

	// Same server name "s1" exists in both C1 and C2
	for _, pair := range []struct{ cluster, server string }{
		{"C1", "s1"},
		{"C2", "s1"},
	} {
		if err = aw.Add(
			Record{pair.cluster, pair.server},
			TagCluster(pair.cluster),
			TagServer(pair.server),
			TagServerHealth(),
		); err != nil {
			t.Fatalf("Failed to add artifact for %s/%s: %s", pair.cluster, pair.server, err)
		}
	}
	if err = aw.Close(); err != nil {
		t.Fatalf("Failed to close archive: %s", err)
	}

	ar, err := NewReader(archivePath)
	if err != nil {
		t.Fatalf("Failed to open archive: %s", err)
	}
	defer ar.Close()

	if clusters := ar.Clusters(); len(clusters) != 2 {
		t.Fatalf("Expected 2 clusters, got: %v", clusters)
	}
	for _, cluster := range []string{"C1", "C2"} {
		servers := ar.ClusterServerNames(ClusterRef{Name: cluster})
		if len(servers) != 1 || servers[0] != "s1" {
			t.Fatalf("Cluster %s: expected server [s1], got: %v", cluster, servers)
		}

		var r Record
		if err = ar.Load(&r, TagCluster(cluster), TagServer("s1"), TagServerHealth()); err != nil {
			t.Fatalf("Failed to load artifact for %s/s1: %s", cluster, err)
		}
		if r.Cluster != cluster || r.Server != "s1" {
			t.Fatalf("Cluster %s: unexpected record %+v", cluster, r)
		}
	}
}

func Test_SameStreamNameAcrossDomains(t *testing.T) {
	type Record struct{ Domain, Server string }

	archivePath := filepath.Join(t.TempDir(), "archive.zip")
	aw, err := NewWriter(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive: %s", err)
	}

	for _, r := range []struct {
		domain, server string
		cluster        *Tag
	}{
		{"hub", "h1", TagCluster("HUB")},
		{"hub", "h2", TagCluster("HUB")},
		{"leaf", "l1", TagNoCluster()},
		{"leaf", "h1", TagCluster("LEAF")},
		{"", "n1", TagCluster("C1")},
		{"", "h1", TagCluster("C1")},
	} {
		tags := []*Tag{TagAccount("A"), TagStream("ORDERS"), r.cluster, TagServer(r.server), TagStreamInfo()}
		if r.domain != "" {
			tags = append(tags, TagDomain(r.domain))
		}
		if err = aw.Add(Record{r.domain, r.server}, tags...); err != nil {
			t.Fatalf("Failed to add artifact for %s/%s: %s", r.domain, r.server, err)
		}
	}
	if err = aw.Close(); err != nil {
		t.Fatalf("Failed to close archive: %s", err)
	}

	ar, err := NewReader(archivePath)
	if err != nil {
		t.Fatalf("Failed to open archive: %s", err)
	}
	defer ar.Close()

	expectedStreams := []StreamRef{{Name: "ORDERS"}, {Domain: "hub", Name: "ORDERS"}, {Domain: "leaf", Name: "ORDERS"}}
	if streams := ar.AccountStreams("A"); !slices.Equal(expectedStreams, streams) {
		t.Fatalf("Expected streams: %v, got: %v", expectedStreams, streams)
	}

	for ref, expectedServers := range map[StreamRef][]string{
		{Name: "ORDERS"}:                 {"h1", "n1"},
		{Domain: "hub", Name: "ORDERS"}:  {"h1", "h2"},
		{Domain: "leaf", Name: "ORDERS"}: {"h1", "l1"},
		{Domain: "edge", Name: "ORDERS"}: {},
	} {
		if servers := ar.StreamServerNames("A", ref); !slices.Equal(expectedServers, servers) {
			t.Fatalf("Stream %+v: expected servers %v, got: %v", ref, expectedServers, servers)
		}
	}

	for _, ref := range []StreamRef{{Name: "ORDERS"}, {Domain: "hub", Name: "ORDERS"}, {Domain: "leaf", Name: "ORDERS"}} {
		var got []Record
		err = ForEachStreamArtifact(ar, "A", ref, "h1", TagStreamInfo(), func(r *Record) error {
			got = append(got, *r)
			return nil
		})
		if err != nil {
			t.Fatalf("Stream %+v: failed to read h1 artifacts: %s", ref, err)
		}
		if expected := []Record{{ref.Domain, "h1"}}; !slices.Equal(expected, got) {
			t.Fatalf("Stream %+v: expected records %v, got: %v", ref, expected, got)
		}
	}

	err = ForEachStreamArtifact(ar, "A", StreamRef{Name: "ORDERS"}, "l1", TagStreamInfo(), func(*Record) error { return nil })
	if !errors.Is(err, ErrNoMatches) {
		t.Fatalf("Expected error '%s', but got: '%v'", ErrNoMatches, err)
	}

	var r Record
	if err = ar.Load(&r, TagAccount("A"), TagStream("ORDERS"), TagServer("l1"), TagStreamInfo()); err != nil {
		t.Fatalf("Failed to load leaf artifact: %s", err)
	}
	if r.Domain != "leaf" {
		t.Fatalf("Unexpected record %+v", r)
	}

	if err = ar.Load(&r, TagAccount("A"), TagStream("ORDERS"), TagDomain("hub"), TagStreamInfo()); !errors.Is(err, ErrMultipleMatches) {
		t.Fatalf("Expected error '%s', but got: '%s'", ErrMultipleMatches, err)
	}
}

func Test_SameClusterAndServerNamesAcrossDomains(t *testing.T) {
	type Record struct{ Domain, Server string }

	archivePath := filepath.Join(t.TempDir(), "archive.zip")
	aw, err := NewWriter(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive: %s", err)
	}

	for _, r := range []struct{ domain, cluster, server string }{
		{"s1", "SITE", "n1"},
		{"s1", "SITE", "n2"},
		{"s2", "SITE", "n1"},
		{"", "SITE", "n1"},
		{"d1", "C1", "a1"},
		{"", "C1", "x"},
	} {
		tags := []*Tag{TagCluster(r.cluster), TagServer(r.server), TagServerVars()}
		if r.domain != "" {
			tags = append(tags, TagDomain(r.domain))
		}
		if err = aw.Add(Record{r.domain, r.server}, tags...); err != nil {
			t.Fatalf("Failed to add artifact for %s/%s/%s: %s", r.domain, r.cluster, r.server, err)
		}
	}
	if err = aw.Close(); err != nil {
		t.Fatalf("Failed to close archive: %s", err)
	}

	ar, err := NewReader(archivePath)
	if err != nil {
		t.Fatalf("Failed to open archive: %s", err)
	}
	defer ar.Close()

	expectedClusters := []ClusterRef{{"", "C1"}, {"d1", "C1"}, {"", "SITE"}, {"s1", "SITE"}, {"s2", "SITE"}}
	if clusters := ar.Clusters(); !slices.Equal(expectedClusters, clusters) {
		t.Fatalf("Expected clusters: %v, got: %v", expectedClusters, clusters)
	}

	for _, tc := range []struct {
		cluster ClusterRef
		records []Record
	}{
		{ClusterRef{"", "C1"}, []Record{{"", "x"}}},
		{ClusterRef{"d1", "C1"}, []Record{{"d1", "a1"}}},
		{ClusterRef{"", "SITE"}, []Record{{"", "n1"}}},
		{ClusterRef{"s1", "SITE"}, []Record{{"s1", "n1"}, {"s1", "n2"}}},
		{ClusterRef{"s2", "SITE"}, []Record{{"s2", "n1"}}},
	} {
		var got []Record
		for _, serverName := range ar.ClusterServerNames(tc.cluster) {
			err = ForEachServerArtifact(ar, tc.cluster, serverName, []*Tag{TagServerVars()}, func(r *Record) error {
				got = append(got, *r)
				return nil
			})
			if err != nil {
				t.Fatalf("Cluster %s server %s: %s", tc.cluster, serverName, err)
			}
		}
		if !slices.Equal(tc.records, got) {
			t.Fatalf("Cluster %s: expected records %v, got: %v", tc.cluster, tc.records, got)
		}
	}

	var visited []Record
	_, err = EachClusterServerArtifact(ar, TagServerVars(), func(cluster ClusterRef, serverName string, err error, r *Record) error {
		if err != nil {
			t.Fatalf("Cluster %s server %s: %s", cluster, serverName, err)
		}
		visited = append(visited, *r)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to iterate servers: %s", err)
	}
	if len(visited) != 6 {
		t.Fatalf("Expected 6 server artifacts, got: %v", visited)
	}
}
