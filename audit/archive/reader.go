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
	"archive/zip"
	"cmp"
	"errors"
	"fmt"
	"path/filepath"

	"encoding/json"

	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

// Reader encapsulates a reader for the actual underlying archive, and also provides indices for faster and
// more convenient iteration and querying of the archive content
type Reader struct {
	archiveReader       *zip.ReadCloser
	path                string
	filesMap            map[string]*zip.File
	accountTags         []Tag
	clusterTags         []Tag
	serverTags          []Tag
	streamTags          []Tag
	accountNames        []string
	clusters            []ClusterRef
	clustersServerNames map[ClusterRef][]string
	accountStreams      map[string][]StreamRef
	streamServerNames   map[accountStream][]string
	domainFiles         map[string]struct{}
	clusterIndexFiles   map[string]struct{}
	ts                  *time.Time
	invertedIndex       map[Tag][]string
	// Warnings holds non-fatal issues detected while opening the archive,
	// such as files present in the ZIP but absent from the manifest.
	Warnings []string
}

// StreamRef identifies a stream within an account. Domain is empty for streams captured from servers without a
// JetStream domain. Cluster is set only when the stream's cluster is numbered, see ClusterRef.
type StreamRef struct {
	Domain  string
	Name    string
	Cluster ClusterRef
}

func (s StreamRef) String() string {
	label := domainLabel(s.Name, s.Domain)
	if s.Cluster.Index != "" {
		label += " in cluster " + s.Cluster.NumberedName()
	}
	return label
}

// ClusterRef identifies a cluster. Domain is empty for clusters of servers without a JetStream domain. Index is set
// only when clusters that are not routed to each other share the name and domain, and numbers them from 0 in the
// order gather assigned. The number is the archive's own and is not a cluster name.
type ClusterRef struct {
	Domain string
	Name   string
	Index  string
}

func (c ClusterRef) String() string {
	return domainLabel(c.NumberedName(), c.Domain)
}

// NumberedName is the cluster name followed by its number when the cluster is numbered
func (c ClusterRef) NumberedName() string {
	if c.Index == "" {
		return c.Name
	}
	return c.Name + " #" + c.Index
}

// ServerLabel is the server name followed by its ID shortened to the first and last 8 characters
func ServerLabel(name, id string) string {
	switch {
	case id == "":
		return name
	case len(id) > 19:
		id = id[:8] + "..." + id[len(id)-8:]
	}
	return fmt.Sprintf("%s (%s)", name, id)
}

func domainLabel(name, domain string) string {
	if domain == "" {
		return name
	}
	return fmt.Sprintf("%s (domain %s)", name, domain)
}

type accountStream struct {
	account string
	stream  StreamRef
}

type AuditMetadata struct {
	Timestamp              time.Time `json:"capture_timestamp"`
	ConnectedServerName    string    `json:"connected_server_name"`
	ConnectedServerVersion string    `json:"connected_server_version"`
	ConnectURL             string    `json:"connect_url"`
	UserName               string    `json:"user_name"`
	Version                string    `json:"version"`
}

func (r *Reader) rawFilesCount() int {
	return len(r.archiveReader.File)
}

// Close closes the reader
func (r *Reader) Close() error {
	if r.archiveReader != nil {
		err := r.archiveReader.Close()
		r.archiveReader = nil
		return err
	}
	return nil
}

// getFileReader create a reader for the given filename, if it exists in the archive.
func (r *Reader) getFileReader(name string) (io.ReadCloser, uint64, error) {
	f, exists := r.filesMap[name]
	if !exists {
		return nil, 0, os.ErrNotExist
	}
	reader, err := f.Open()
	if err != nil {
		return nil, 0, err
	}
	return reader, f.UncompressedSize64, nil
}

// loadFile decodes the provided filename into the given value
func (r *Reader) loadFile(name string, v any) error {
	f, _, err := r.getFileReader(name)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	err = decoder.Decode(v)
	if err != nil {
		return fmt.Errorf("failed to decode file %s: %w", name, err)
	}
	return nil
}

// ErrNoMatches is returned if no artifact matched the input combination of tags
var ErrNoMatches = fmt.Errorf("no file matched the given query")

// ErrMultipleMatches is returned if multiple artifact matched the input combination of tags
var ErrMultipleMatches = fmt.Errorf("multiple files matched the given query")

// Load queries the indices for a single artifact matching the given input tags.
// If a single artifact is found, then it is deserialized into v
// If multiple artifact or no artifacts match the input tag, then ErrMultipleMatches and ErrNoMatches are returned
// respectively
func (r *Reader) Load(v any, queryTags ...*Tag) error {
	matching, err := intersectFileSets(r.invertedIndex, queryTags)
	if err != nil {
		return err
	}

	if len(matching) == 1 {
		for file := range matching {
			return r.loadFile(file, v)
		}
	}

	if len(matching) == 0 {
		return ErrNoMatches
	}
	return ErrMultipleMatches
}

// NewReader creates a new reader for the file at the given archivePath.
// Reader expect the file to comply to format and content created by a Writer in this same package.
// During creation, Reader creates in-memory indices to speed up subsequent queries.
func NewReader(archivePath string) (*Reader, error) {
	// Create a zip reader
	archiveReader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open archive: %w", err)
	}

	// Create map of filename -> file
	filesMap := make(map[string]*zip.File, len(archiveReader.File))
	for _, f := range archiveReader.File {
		filesMap[f.Name] = f
	}

	// Find and open the manifest file
	manifestFileName, err := createFilenameFromTags("json", []*Tag{internalTagManifest()})
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	manifestFile, exists := filesMap[manifestFileName]
	if !exists {
		return nil, fmt.Errorf("manifest file not found in archive")
	}

	manifestFileReader, err := manifestFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest: %w", err)
	}
	defer manifestFileReader.Close()

	// Load manifest, which is a normalized index:
	// For each file, a list of tags is present
	manifestMap := make(map[string][]Tag, len(filesMap))
	err = json.NewDecoder(manifestFileReader).Decode(&manifestMap)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Create inverted index for tag lookups
	invertedIndex := make(map[Tag][]string)
	domainFiles := make(map[string]struct{})
	clusterIndexFiles := make(map[string]struct{})
	for fileName, tags := range manifestMap {
		for _, tag := range tags {
			invertedIndex[tag] = append(invertedIndex[tag], fileName)
			switch tag.Name {
			case domainTagLabel:
				domainFiles[fileName] = struct{}{}
			case clusterIndexTagLabel:
				clusterIndexFiles[fileName] = struct{}{}
			}
		}
	}

	// Check that each file in the manifest exists in the archive
	for fileName := range manifestMap {
		_, present := filesMap[fileName]
		if !present {
			return nil, fmt.Errorf("file %s is in manifest, but not present in archive", fileName)
		}
	}

	// Check that each file in the archive is present in the manifest
	manifestFilePath, err := createFilenameFromTags("json", []*Tag{internalTagManifest()})
	if err != nil {
		return nil, fmt.Errorf("failed to compose expected manifest path: %w", err)
	}
	var warnings []string
	for filePath := range filesMap {
		_, present := manifestMap[filePath]
		if filePath != manifestFilePath && !present {
			warnings = append(warnings, fmt.Sprintf("archive file %s is not present in manifest", filePath))
		}
	}

	// Map of cluster to set of server names
	clustersServersMap := make(map[ClusterRef]map[string]any)
	accountsStreamsMap := make(map[string]map[StreamRef]map[string]any)

	for _, tags := range manifestMap {
		// Take note of certain tags, if present
		var cluster, server, account, stream, domain, clusterIndex string
		for _, tag := range tags {
			switch tag.Name {
			case clusterTagLabel:
				cluster = tag.Value
			case serverTagLabel:
				server = tag.Value
			case accountTagLabel:
				account = tag.Value
			case streamTagLabel:
				stream = tag.Value
			case domainTagLabel:
				domain = tag.Value
			case clusterIndexTagLabel:
				clusterIndex = tag.Value
			}
		}

		clusterRef := ClusterRef{Domain: domain, Name: cluster, Index: clusterIndex}

		// If a cluster tag is set, create a record for it
		if cluster != "" {
			ref := clusterRef
			if _, knownCluster := clustersServersMap[ref]; !knownCluster {
				clustersServersMap[ref] = make(map[string]any)
			}
			// File has cluster and server tags, save server in set for this cluster
			if server != "" {
				clustersServersMap[ref][server] = nil // Map used as set, value doesn't matter
			}
		}

		// If an account tag is set, create a record for it
		if account != "" {
			if _, knownAccount := accountsStreamsMap[account]; !knownAccount {
				accountsStreamsMap[account] = make(map[StreamRef]map[string]any)
			}
			// If account and stream tags present, save stream in set for this account
			if stream != "" {
				ref := StreamRef{Domain: domain, Name: stream}
				if clusterIndex != "" {
					ref.Cluster = clusterRef
				}
				if _, knownStream := accountsStreamsMap[account][ref]; !knownStream {
					accountsStreamsMap[account][ref] = make(map[string]any)
				}
				// If account and stream and server tags present, save server in set for this stream
				if server != "" {
					accountsStreamsMap[account][ref][server] = nil // Map used as set, value doesn't matter
				}
			}
		}
	}

	clusters := slices.SortedFunc(maps.Keys(clustersServersMap), compareClusterRefs)
	clusterServers := make(map[ClusterRef][]string, len(clustersServersMap))
	for ref, serversSet := range clustersServersMap {
		clusterServers[ref] = slices.Sorted(maps.Keys(serversSet))
	}
	accounts := slices.Sorted(maps.Keys(accountsStreamsMap))
	accountsStreams := make(map[string][]StreamRef, len(accountsStreamsMap))
	streamsServers := make(map[accountStream][]string)
	for account, streamsServersSet := range accountsStreamsMap {
		accountsStreams[account] = slices.SortedFunc(maps.Keys(streamsServersSet), compareStreamRefs)
		for ref, serversSet := range streamsServersSet {
			streamsServers[accountStream{account, ref}] = slices.Sorted(maps.Keys(serversSet))
		}
	}

	// Returns a deduplicated list of tags for the specific label present in the archive
	// e.g. getUniqueTags(serverTagLabel) -> [Tag(server, s1), Tag(server, s2, Tag(server, s3)]
	getUniqueTags := func(label TagLabel) ([]Tag, error) {
		var tags []Tag
		for tag := range invertedIndex {
			if tag.Name == label {
				tags = append(tags, tag)
			}
		}
		var sortErr error
		slices.SortFunc(tags, func(a, b Tag) int {
			if a.Name != b.Name {
				// Fallback to consistent ordering just in case
				sortErr = fmt.Errorf("unexpected comparison between different tags")
				return strings.Compare(string(a.Name), string(b.Name))
			}
			return strings.Compare(a.Value, b.Value)
		})
		return tags, sortErr
	}

	accountTags, err := getUniqueTags(accountTagLabel)
	if err != nil {
		return nil, err
	}

	clusterTags, err := getUniqueTags(clusterTagLabel)
	if err != nil {
		return nil, err
	}

	serverTags, err := getUniqueTags(serverTagLabel)
	if err != nil {
		return nil, err
	}

	streamTags, err := getUniqueTags(streamTagLabel)
	if err != nil {
		return nil, err
	}

	reader := &Reader{
		path:                archivePath,
		archiveReader:       archiveReader,
		filesMap:            filesMap,
		accountTags:         accountTags,
		clusterTags:         clusterTags,
		serverTags:          serverTags,
		streamTags:          streamTags,
		accountNames:        accounts,
		clusters:            clusters,
		clustersServerNames: clusterServers,
		accountStreams:      accountsStreams,
		streamServerNames:   streamsServers,
		domainFiles:         domainFiles,
		clusterIndexFiles:   clusterIndexFiles,
		ts:                  &manifestFile.Modified,
		invertedIndex:       invertedIndex,
		Warnings:            warnings,
	}

	return reader, nil
}

// AccountNames list the unique names of accounts found in the archive
// The list of names is sorted alphabetically
func (r *Reader) AccountNames() []string {
	return slices.Clone(r.accountNames)
}

// AccountStreams list the unique streams found in the archive for the given account
// Streams with the same name in different JetStream domains are listed separately
// The list is sorted by stream name, then domain
func (r *Reader) AccountStreams(accountName string) []StreamRef {
	streams, present := r.accountStreams[accountName]
	if present {
		return slices.Clone(streams)
	}
	return make([]StreamRef, 0)
}

// Clusters list the unique clusters found in the archive
// Clusters with the same name in different JetStream domains, and numbered clusters, are listed separately
// The list is sorted by cluster name, then domain, then number
func (r *Reader) Clusters() []ClusterRef {
	return slices.Clone(r.clusters)
}

// ClusterServerNames list the unique server names found in the archive for the given cluster
// The list of names is sorted alphabetically
func (r *Reader) ClusterServerNames(cluster ClusterRef) []string {
	servers, present := r.clustersServerNames[cluster]
	if present {
		return slices.Clone(servers)
	}
	return make([]string, 0)
}

// StreamServerNames list the unique server names found in the archive for the given stream in the given account
// The list of names is sorted alphabetically
func (r *Reader) StreamServerNames(accountName string, stream StreamRef) []string {
	servers, present := r.streamServerNames[accountStream{accountName, stream}]
	if present {
		return slices.Clone(servers)
	}
	return make([]string, 0)
}

func compareClusterRefs(a, b ClusterRef) int {
	return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.Domain, b.Domain), CompareClusterIndexes(a.Index, b.Index))
}

// CompareClusterIndexes orders cluster numbers numerically, with an unnumbered cluster first
func CompareClusterIndexes(a, b string) int {
	return cmp.Or(cmp.Compare(len(a), len(b)), strings.Compare(a, b))
}

func compareStreamRefs(a, b StreamRef) int {
	return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.Domain, b.Domain), compareClusterRefs(a.Cluster, b.Cluster))
}

// EachClusterServerVarz iterates over all servers ordered by cluster and calls the callback function with the loaded Varz response
//
// The callback function will receive any error encountered during loading the server varz file and should check that and handle it
// If the callback returns an error iteration is stopped and that error is returned
//
// Errors returned match those documented in Load() otherwise any other error that are encountered
func (r *Reader) EachClusterServerVarz(cb func(cluster ClusterRef, serverName string, err error, vz *server.ServerAPIVarzResponse) error) (int, error) {
	return EachClusterServerArtifact(r, TagServerVars(), cb)
}

// EachClusterServerHealthz iterates over all servers ordered by cluster and calls the callback function with the loaded Healthz response
//
// The callback function will receive any error encountered during loading the server varz file and should check that and handle it
// If the callback returns an error iteration is stopped and that error is returned
//
// Errors returned match those documented in Load() otherwise any other error that are encountered
func (r *Reader) EachClusterServerHealthz(cb func(cluster ClusterRef, serverName string, err error, hz *server.ServerAPIHealthzResponse) error) (int, error) {
	return EachClusterServerArtifact(r, TagServerHealth(), cb)
}

// EachClusterServerJsz iterates over all servers ordered by cluster and calls the callback function with the loaded Jsz response
//
// The callback function will receive any error encountered during loading the server varz file and should check that and handle it
// If the callback returns an error iteration is stopped and that error is returned
//
// Errors returned match those documented in Load() otherwise any other error that are encountered
func (r *Reader) EachClusterServerJsz(cb func(cluster ClusterRef, serverName string, err error, jsz *server.ServerAPIJszResponse) error) (int, error) {
	return EachClusterServerArtifact(r, TagServerJetStream(), cb)
}

// EachClusterServerAccountz iterates over all servers ordered by cluster and calls the callback function with the loaded Accountz response
//
// The callback function will receive any error encountered during loading the server varz file and should check that and handle it
// If the callback returns an error iteration is stopped and that error is returned
//
// Errors returned match those documented in Load() otherwise any other error that are encountered
func (r *Reader) EachClusterServerAccountz(cb func(cluster ClusterRef, serverName string, err error, az *server.ServerAPIAccountzResponse) error) (int, error) {
	return EachClusterServerArtifact(r, TagServerAccounts(), cb)
}

// EachClusterServerLeafz iterates over all servers ordered by cluster and calls the callback function with the loaded Leafz response
//
// The callback function will receive any error encountered during loading the server varz file and should check that and handle it
// If the callback returns an error iteration is stopped and that error is returned
//
// Errors returned match those documented in Load() otherwise any other error that are encountered
func (r *Reader) EachClusterServerLeafz(cb func(cluster ClusterRef, serverName string, err error, lz *server.ServerAPILeafzResponse) error) (int, error) {
	return EachClusterServerArtifact(r, TagServerLeafs(), cb)
}

// EachClusterServerArtifact iterates over all paged JSON artifact files in the archive by looping
// through every cluster and its servers. For each cluster, server pair, it calls ForEachServerArtifact
// to load all artifacts of type T associated with that combination.
//
// For each decoded artifact, the provided callback function is called with the cluster, the server name,
// and the loaded artifact (or an error if no matching artifact was found).
//
// The function returns the total count of processed artifacts and any error encountered during iteration.
func EachClusterServerArtifact[T any](r *Reader, artifactTag *Tag, cb func(cluster ClusterRef, serverName string, err error, artifact *T) error) (int, error) {
	count := 0
	for _, cluster := range r.Clusters() {
		for _, serverName := range r.ClusterServerNames(cluster) {
			err := ForEachServerArtifact(r, cluster, serverName, []*Tag{artifactTag}, func(artifact *T) error {
				count++
				return cb(cluster, serverName, nil, artifact)
			})
			if errors.Is(err, ErrNoMatches) {
				// if we found nothing then we call the cb once so that it can deal with the error
				if cbErr := cb(cluster, serverName, err, nil); cbErr != nil {
					return count, cbErr
				}
			} else if err != nil {
				return count, err
			}
		}
	}
	return count, nil
}

// ForEachTaggedArtifact iterates over all paged JSON artifact files in the archive that match
// the given set of tags and calls the provided callback function for each decoded artifact.
//
// The function uses the Reader’s inverted index to collect the file names associated with each tag,
// performs an intersection of these sets to determine the files that match all the given tags,
// and then filters these to include only those files that match the paged artifact naming
// convention.
//
// The matching files are sorted by name, opened, and decoded from JSON into an object of type T.
// For each decoded artifact, the callback function cb is called. If the callback returns an error
// we iterating and the error is returned.
//
// If no files match the provided tags it returns ErrNoMatches.
// It also returns any errors encountered during file opening or JSON decoding.
func ForEachTaggedArtifact[T any](r *Reader, tags []*Tag, cb func(*T) error) error {
	matching, err := intersectFileSets(r.invertedIndex, tags)
	if err != nil {
		return err
	}

	return forEachPagedArtifact(r, matching, cb)
}

// ForEachStreamArtifact iterates over the paged JSON artifacts of type artifactTag captured for the stream from the
// named server. Only artifacts in the stream's domain and numbered cluster match, and an empty domain or number
// matches no artifact that has one.
func ForEachStreamArtifact[T any](r *Reader, accountName string, stream StreamRef, serverName string, artifactTag *Tag, cb func(*T) error) error {
	return forEachScopedArtifact(r, stream.Domain, stream.Cluster.Index, []*Tag{TagAccount(accountName), TagStream(stream.Name), TagServer(serverName), artifactTag}, cb)
}

// ForEachServerArtifact iterates over the paged JSON artifacts captured from the named server in the cluster that
// match all the given tags. Only artifacts in the cluster's domain and number match, and an empty domain or number
// matches no artifact that has one.
func ForEachServerArtifact[T any](r *Reader, cluster ClusterRef, serverName string, tags []*Tag, cb func(*T) error) error {
	return forEachScopedArtifact(r, cluster.Domain, cluster.Index, append([]*Tag{TagCluster(cluster.Name), TagServer(serverName)}, tags...), cb)
}

func forEachScopedArtifact[T any](r *Reader, domain, clusterIndex string, tags []*Tag, cb func(*T) error) error {
	if domain != "" {
		tags = append(tags, TagDomain(domain))
	}
	if clusterIndex != "" {
		tags = append(tags, &Tag{Name: clusterIndexTagLabel, Value: clusterIndex})
	}

	matching, err := intersectFileSets(r.invertedIndex, tags)
	if err != nil {
		return err
	}

	for name := range matching {
		_, inDomain := r.domainFiles[name]
		_, numbered := r.clusterIndexFiles[name]
		if (domain == "" && inDomain) || (clusterIndex == "" && numbered) {
			delete(matching, name)
		}
	}

	return forEachPagedArtifact(r, matching, cb)
}

func forEachPagedArtifact[T any](r *Reader, matching map[string]struct{}, cb func(*T) error) error {
	var files []*zip.File
	for name := range matching {
		base := filepath.Base(name)
		if strings.HasSuffix(base, ".json") && len(base) == len("0001.json") {
			if f, ok := r.filesMap[name]; ok {
				files = append(files, f)
			}
		}
	}
	if len(files) == 0 {
		return ErrNoMatches
	}

	slices.SortFunc(files, func(a, b *zip.File) int {
		return strings.Compare(a.Name, b.Name)
	})

	for _, f := range files {
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open %s: %w", f.Name, err)
		}

		var obj T
		if err := json.NewDecoder(rc).Decode(&obj); err != nil {
			rc.Close()
			return fmt.Errorf("decode %s: %w", f.Name, err)
		}
		rc.Close()

		if err := cb(&obj); err != nil {
			return err
		}
	}

	return nil
}

// Intersect the inverted index and find the files for the given tags
func intersectFileSets(index map[Tag][]string, tags []*Tag) (map[string]struct{}, error) {
	if len(tags) == 0 {
		return nil, ErrNoMatches
	}
	fileSets := make([][]string, 0, len(tags))
	for _, tag := range tags {
		files, ok := index[*tag]
		if !ok {
			return nil, ErrNoMatches
		}
		fileSets = append(fileSets, files)
	}
	result := make(map[string]struct{}, len(fileSets[0]))
	for _, f := range fileSets[0] {
		result[f] = struct{}{}
	}
	for _, set := range fileSets[1:] {
		next := make(map[string]struct{})
		for _, f := range set {
			if _, ok := result[f]; ok {
				next[f] = struct{}{}
			}
		}
		result = next
		if len(result) == 0 {
			return nil, ErrNoMatches
		}
	}
	return result, nil
}
