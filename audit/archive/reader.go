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

package archive

import (
	"archive/zip"
	"fmt"

	"encoding/json"

	"io"
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
	clusterNames        []string
	clustersServerNames map[string][]string
	accountStreamNames  map[string][]string
	streamServerNames   map[string][]string
	ts                  *time.Time
	invertedIndex       map[Tag][]string
}

type AuditMetadata struct {
	Timestamp              time.Time `json:"capture_timestamp"`
	ConnectedServerName    string    `json:"connected_server_name"`
	ConnectedServerVersion string    `json:"connected_server_version"`
	ConnectURL             string    `json:"connect_url"`
	UserName               string    `json:"user_name"`
	CLIVersion             string    `json:"cli_version"`
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
	if len(queryTags) == 0 {
		return ErrNoMatches
	}

	// Collect the list of file names for each tag
	var fileSets [][]string
	for _, tag := range queryTags {
		files, found := r.invertedIndex[*tag]
		if !found {
			return ErrNoMatches
		}
		fileSets = append(fileSets, files)
	}

	// Start with the first set
	baseSet := make(map[string]struct{}, len(fileSets[0]))
	for _, file := range fileSets[0] {
		baseSet[file] = struct{}{}
	}

	// Intersect with the remaining sets
	for _, files := range fileSets[1:] {
		seen := make(map[string]struct{}, len(files))
		for _, f := range files {
			if _, ok := baseSet[f]; ok {
				seen[f] = struct{}{}
			}
		}
		baseSet = seen
		if len(baseSet) == 0 {
			return ErrNoMatches
		}
	}

	// Determine match
	if len(baseSet) == 1 {
		for file := range baseSet {
			return r.loadFile(file, v)
		}
	}

	if len(baseSet) == 0 {
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
	for fileName, tags := range manifestMap {
		for _, tag := range tags {
			invertedIndex[tag] = append(invertedIndex[tag], fileName)
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
	for filePath := range filesMap {
		_, present := manifestMap[filePath]
		if filePath != manifestFilePath && !present {
			fmt.Printf("Warning: archive file %s is not present in manifest\n", filePath)
		}
	}

	// Map of cluster to set of server names
	clustersServersMap := make(map[string]map[string]any)
	accountsStreamsMap := make(map[string]map[string]map[string]any)

	for _, tags := range manifestMap {
		// Take note of certain tags, if present
		var cluster, server, account, stream string
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
			}
		}

		// If a cluster tag is set, create a record for it
		if cluster != "" {
			if _, knownCluster := clustersServersMap[cluster]; !knownCluster {
				clustersServersMap[cluster] = make(map[string]any)
			}
			// File has cluster and server tags, save server in set for this cluster
			if server != "" {
				clustersServersMap[cluster][server] = nil // Map used as set, value doesn't matter
			}
		}

		// If an account tag is set, create a record for it
		if account != "" {
			if _, knownAccount := accountsStreamsMap[account]; !knownAccount {
				accountsStreamsMap[account] = make(map[string]map[string]any)
			}
			// If account and stream tags present, save stream in set for this account
			if stream != "" {
				if _, knownStream := accountsStreamsMap[account][stream]; !knownStream {
					accountsStreamsMap[account][stream] = make(map[string]any)
				}
				// If account and stream and server tags present, save server in set for this stream
				if server != "" {
					accountsStreamsMap[account][stream][server] = nil // Map used as set, value doesn't matter
				}
			}
		}
	}

	clusters, clusterServers := shrinkMapOfSets(clustersServersMap)
	accounts, accountsStreams := shrinkMapOfSets(accountsStreamsMap)
	streamsServers := make(map[string][]string, len(accounts))
	for account, streamsMapServersSet := range accountsStreamsMap {
		_, streamServers := shrinkMapOfSets(streamsMapServersSet)
		for stream, serversList := range streamServers {
			key := account + "/" + stream
			streamsServers[key] = serversList
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
		slices.SortFunc(tags, func(a, b Tag) int {
			if a.Name != b.Name {
				// Fallback to consistent ordering just in case
				err = fmt.Errorf("unexpected comparison between different tags")
				return strings.Compare(string(a.Name), string(b.Name))
			}
			return strings.Compare(a.Value, b.Value)
		})
		return tags, err
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
		clusterNames:        clusters,
		clustersServerNames: clusterServers,
		accountStreamNames:  accountsStreams,
		streamServerNames:   streamsServers,
		ts:                  &manifestFile.Modified,
		invertedIndex:       invertedIndex,
	}

	return reader, nil
}

// AccountNames list the unique names of accounts found in the archive
// The list of names is sorted alphabetically
func (r *Reader) AccountNames() []string {
	return slices.Clone(r.accountNames)
}

// AccountStreamNames list the unique stream names found in the archive for the given account
// The list of names is sorted alphabetically
func (r *Reader) AccountStreamNames(accountName string) []string {
	streams, present := r.accountStreamNames[accountName]
	if present {
		return slices.Clone(streams)
	}
	return make([]string, 0)
}

// ClusterNames list the unique names of clusters found in the archive
// The list of names is sorted alphabetically
func (r *Reader) ClusterNames() []string {
	return slices.Clone(r.clusterNames)
}

// ClusterServerNames list the unique server names found in the archive for the given cluster
// The list of names is sorted alphabetically
func (r *Reader) ClusterServerNames(clusterName string) []string {
	servers, present := r.clustersServerNames[clusterName]
	if present {
		return slices.Clone(servers)
	}
	return make([]string, 0)
}

// StreamServerNames list the unique server names found in the archive for the given stream in the given account
// The list of names is sorted alphabetically
func (r *Reader) StreamServerNames(accountName, streamName string) []string {
	servers, present := r.streamServerNames[accountName+"/"+streamName]
	if present {
		return slices.Clone(servers)
	}
	return make([]string, 0)
}

// shrinkMapOfSets utility method, given a map[string] of sets (map[string]any), return:
// The list of (unique) keys as string slice plus a shrunk map where sets are replaced with lists
// The list of unique keys and each list in the map are sorted alphabetically.
func shrinkMapOfSets[T any](m map[string]map[string]T) ([]string, map[string][]string) {
	keysList := make([]string, 0, len(m))
	newMap := make(map[string][]string, len(m))
	for k, valuesMap := range m {
		keysList = append(keysList, k)
		newMap[k] = make([]string, 0, len(valuesMap))
		for value := range valuesMap {
			newMap[k] = append(newMap[k], value)
		}
		slices.Sort(newMap[k])
	}
	slices.Sort(keysList)
	return keysList, newMap
}

func eachClusterServer[T any](r *Reader, tag *Tag, cb func(clusterTag *Tag, serverTag *Tag, err error, resp *T) error) (int, error) {
	found := 0

	for _, clusterName := range r.ClusterNames() {
		clusterTag := TagCluster(clusterName)
		servers := r.ClusterServerNames(clusterName)
		found += len(servers)

		for _, serverName := range servers {
			serverTag := TagServer(serverName)

			var resp T
			err := r.Load(&resp, clusterTag, serverTag, tag)
			err = cb(clusterTag, serverTag, err, &resp)

			if err != nil {
				return found, err
			}
		}
	}

	return found, nil
}

// EachClusterServerVarz iterates over all servers ordered by cluster and calls the callback function with the loaded Varz response
//
// The callback function will receive any error encountered during loading the server varz file and should check that and handle it
// If the callback returns an error iteration is stopped and that error is returned
//
// Errors returned match those documented in Load() otherwise any other error that are encountered
func (r *Reader) EachClusterServerVarz(cb func(clusterTag *Tag, serverTag *Tag, err error, vz *server.ServerAPIVarzResponse) error) (int, error) {
	return eachClusterServer(r, TagServerVars(), cb)
}

// EachClusterServerHealthz iterates over all servers ordered by cluster and calls the callback function with the loaded Healthz response
//
// The callback function will receive any error encountered during loading the server varz file and should check that and handle it
// If the callback returns an error iteration is stopped and that error is returned
//
// Errors returned match those documented in Load() otherwise any other error that are encountered
func (r *Reader) EachClusterServerHealthz(cb func(clusterTag *Tag, serverTag *Tag, err error, hz *server.ServerAPIHealthzResponse) error) (int, error) {
	return eachClusterServer(r, TagServerJetStream(), cb)
}

// EachClusterServerJsz iterates over all servers ordered by cluster and calls the callback function with the loaded Jsz response
//
// The callback function will receive any error encountered during loading the server varz file and should check that and handle it
// If the callback returns an error iteration is stopped and that error is returned
//
// Errors returned match those documented in Load() otherwise any other error that are encountered
func (r *Reader) EachClusterServerJsz(cb func(clusterTag *Tag, serverTag *Tag, err error, jsz *server.ServerAPIJszResponse) error) (int, error) {
	return eachClusterServer(r, TagServerJetStream(), cb)
}

// EachClusterServerAccountz iterates over all servers ordered by cluster and calls the callback function with the loaded Accountz response
//
// The callback function will receive any error encountered during loading the server varz file and should check that and handle it
// If the callback returns an error iteration is stopped and that error is returned
//
// Errors returned match those documented in Load() otherwise any other error that are encountered
func (r *Reader) EachClusterServerAccountz(cb func(clusterTag *Tag, serverTag *Tag, err error, az *server.ServerAPIAccountzResponse) error) (int, error) {
	return eachClusterServer(r, TagServerAccounts(), cb)
}

// EachClusterServerLeafz iterates over all servers ordered by cluster and calls the callback function with the loaded Leafz response
//
// The callback function will receive any error encountered during loading the server varz file and should check that and handle it
// If the callback returns an error iteration is stopped and that error is returned
//
// Errors returned match those documented in Load() otherwise any other error that are encountered
func (r *Reader) EachClusterServerLeafz(cb func(clusterTag *Tag, serverTag *Tag, err error, lz *server.ServerAPILeafzResponse) error) (int, error) {
	return eachClusterServer(r, TagServerLeafs(), cb)
}
