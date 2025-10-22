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

package gather

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/s2"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type EndpointSelection struct {
	ServerEndpoints  bool
	ServerProfiles   bool
	AccountEndpoints bool
	Streams          bool
	Consumers        bool
}

// EndpointCaptureConfig configuration for capturing and tagging server and account endpoints
type EndpointCaptureConfig struct {
	ApiSuffix string
	TypeTag   *archive.Tag
}

type Configuration struct {
	LogLevel               api.Level
	Timeout                time.Duration
	TargetPath             string
	Include                EndpointSelection
	ServerEndpointConfigs  []EndpointCaptureConfig
	AccountEndpointConfigs []EndpointCaptureConfig
	ServerProfileNames     []profileConfiguration
	Detailed               bool
}

// endpointPagingInfo maps a given endpoint's API suffix to the JSON field path that contains
// the array of data elements for that endpoint. This mapping is used to determine if the
// response has reached the paging limit. Additional endpoints that support pagination can be
// added to this map as needed.
var endpointPagingInfo = map[string]string{
	"CONNZ": "data.connections",
	"SUBSZ": "data.subscriptions_list",
	"JSZ":   "data.account_details",
}

type profileConfiguration struct {
	name  string
	debug int
}

func (p *profileConfiguration) Name() string { return p.name }

func NewCaptureConfiguration() *Configuration {
	return &Configuration{
		LogLevel: api.InfoLevel,
		Timeout:  5 * time.Second,
		ServerEndpointConfigs: []EndpointCaptureConfig{
			{
				"VARZ",
				archive.TagServerVars(),
			},
			{
				"CONNZ",
				archive.TagServerConnections(),
			},
			{
				"ROUTEZ",
				archive.TagServerRoutes(),
			},
			{
				"GATEWAYZ",
				archive.TagServerGateways(),
			},
			{
				"LEAFZ",
				archive.TagServerLeafs(),
			},
			{
				"SUBSZ",
				archive.TagServerSubs(),
			},
			{
				"JSZ",
				archive.TagServerJetStream(),
			},
			{
				"ACCOUNTZ",
				archive.TagServerAccounts(),
			},
			{
				"HEALTHZ",
				archive.TagServerHealth(),
			},
		},
		AccountEndpointConfigs: []EndpointCaptureConfig{
			{
				"CONNZ",
				archive.TagAccountConnections(),
			},
			{
				"LEAFZ",
				archive.TagAccountLeafs(),
			},
			{
				"SUBSZ",
				archive.TagAccountSubs(),
			},
			{
				"INFO",
				archive.TagAccountInfo(),
			},
			{
				"JSZ",
				archive.TagAccountJetStream(),
			},
		},
		ServerProfileNames: []profileConfiguration{
			{"goroutine", 1}, // includes aggregated goroutines with tags
			{"goroutine", 2}, // includes full per-goroutine stacks
			{"heap", 0},
			{"allocs", 0},
			{"mutex", 0},
			{"threadcreate", 0},
			{"block", 0},
			{"cpu", 0},
		},
		Detailed: true,
	}
}

type gather struct {
	cfg     *Configuration
	aw      *archive.Writer
	nc      *nats.Conn
	capture *bytes.Buffer
	log     api.Logger
}

func (g *gather) start() error {
	ts := time.Now().UTC()

	if g.cfg.TargetPath == "" {
		g.cfg.TargetPath = filepath.Join(os.TempDir(), fmt.Sprintf("audit-archive-%s.zip", ts.Format("2006-01-02T15-04-05Z")))
	}
	target := g.cfg.TargetPath

	// Create an archive writer
	var err error
	g.aw, err = archive.NewWriter(target)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	defer func() {
		// Add the output of this command (so far) to the archive as additional log artifact
		err = g.aw.AddRaw(bytes.NewReader(g.capture.Bytes()), "log", archive.TagSpecial("audit_gather_log"))
		if err != nil {
			fmt.Printf("Failed to add capture log: %s\n", err)
		}
		g.capture.Reset()

		err := g.aw.Close()
		if err != nil {
			fmt.Printf("Failed to close archive: %s\n", err)
		}
		fmt.Printf("Archive created at: %s\n", target)
	}()
	g.aw.SetTime(ts)

	// Discover servers, create map with servers info
	serverInfoMap, err := g.discoverServers()
	if err != nil {
		return fmt.Errorf("failed to discover servers: %w", err)
	}

	// Discover accounts, create map with count of server for each account
	accountIdsToServersCountMap, systemAccount, err := g.discoverAccounts(serverInfoMap)
	if err != nil {
		return fmt.Errorf("failed to discover accounts: %w", err)
	}

	// Capture server endpoints
	if g.cfg.Include.ServerEndpoints {
		err := g.captureServerEndpoints(serverInfoMap, g.cfg.Detailed)
		if err != nil {
			return fmt.Errorf("failed to capture server endpoints: %w", err)
		}
	} else {
		g.log.Infof("Skipping servers endpoints data gathering")
	}

	// Capture server profiles
	if g.cfg.Include.ServerProfiles {
		err := g.captureServerProfiles(serverInfoMap)
		if err != nil {
			return fmt.Errorf("failed to capture server profiles: %w", err)
		}
	} else {
		g.log.Infof("Skipping server profiles gathering")
	}

	// Capture account endpoints
	if g.cfg.Include.AccountEndpoints {
		err := g.captureAccountEndpoints(serverInfoMap, accountIdsToServersCountMap)
		if err != nil {
			return fmt.Errorf("failed to capture account endpoints: %w", err)
		}
	} else {
		g.log.Infof("Skipping accounts endpoints data gathering")
	}

	// Discover and capture streams in each account
	if g.cfg.Include.Streams {
		g.log.Infof("Gathering streams data...")

		for accountId, numServers := range accountIdsToServersCountMap {
			// Skip system account, JetStream is probably not enabled
			if accountId == systemAccount {
				continue
			}
			err := g.captureAccountStreams(serverInfoMap, accountId, numServers)
			if err != nil {
				g.log.Errorf("Failed to capture streams for account %s: %v", accountId, err)
			}
		}
	} else {
		g.log.Infof("Skipping streams data gathering")
	}

	// Capture metadata
	err = g.captureMetadata()
	if err != nil {
		return fmt.Errorf("failed to capture metadata: %w", err)
	}

	return nil
}

// Capture runtime information about the capture
func (g *gather) captureMetadata() error {
	username := "?"
	currentUser, err := user.Current()
	if err != nil {
		g.log.Errorf("Failed to capture username: %s", err)
	} else {
		username = fmt.Sprintf("%s (%s)", currentUser.Username, currentUser.Name)
	}

	metadata := &archive.AuditMetadata{
		Timestamp:              time.Now().UTC(),
		ConnectedServerName:    g.nc.ConnectedServerName(),
		ConnectedServerVersion: g.nc.ConnectedServerVersion(),
		ConnectURL:             g.nc.ConnectedUrlRedacted(),
		UserName:               username,
	}

	err = g.aw.Add(&metadata, archive.TagSpecial("audit_gather_metadata"))
	if err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

// Discover streams in given account, and capture info for each one
func (g *gather) captureAccountStreams(serverInfoMap map[string]*server.ServerInfo, accountId string, numServers int) error {
	jszOptions := server.JSzOptions{
		Account:    accountId,
		Streams:    true,
		Consumer:   g.cfg.Include.Consumers,
		Config:     true,
		RaftGroups: true,
	}

	jsInfoResponses := make(map[string]*server.ServerAPIJszResponse, numServers)
	err := g.doReqAsync(context.TODO(), jszOptions, "$SYS.REQ.SERVER.PING.JSZ", numServers, func(b []byte) {
		var apiResponse server.ServerAPIJszResponse
		err := json.Unmarshal(b, &apiResponse)
		if err != nil {
			g.log.Errorf("Failed to deserialize JS info response for account %s: %s", accountId, err)
			return
		}

		if apiResponse.Error != nil {
			g.log.Errorf("Received an error from server %s: (%d) %s", apiResponse.Server.Name, apiResponse.Error.ErrCode, apiResponse.Error.Description)
			return
		}

		serverId, serverName := apiResponse.Server.ID, apiResponse.Server.Name

		// Ignore responses from servers not discovered earlier.
		// We are discarding useful data, but limiting additional collection to a fixed set of nodes
		// simplifies querying and analysis. Could always re-run gather if a new server just joined.
		if _, serverKnown := serverInfoMap[serverId]; !serverKnown {
			g.log.Errorf("Ignoring JS info response from unknown server: %s", serverName)
			return
		}

		if _, isDuplicateResponse := jsInfoResponses[serverName]; isDuplicateResponse {
			g.log.Errorf("Ignoring duplicate JS info response for account %s from server %s", accountId, serverName)
			return
		}

		if len(apiResponse.Data.AccountDetails) == 0 {
			// No account details in response, don't bother saving this
			return
		} else if len(apiResponse.Data.AccountDetails) > 1 {
			// Server will respond with multiple accounts if the one specified in the request is not found
			// https://github.com/nats-io/nats-server/pull/5229
			return
		}

		jsInfoResponses[serverName] = &apiResponse
	})
	if err != nil {
		return fmt.Errorf("failed to retrieve account %s streams: %w", accountId, err)
	}

	streamNamesSet := make(map[string]any)

	// Capture stream info from each known replica
	for serverName, jsInfo := range jsInfoResponses {
		// Cases where len(jsInfo.AccountDetails) != 1 are filtered above
		accountDetail := jsInfo.Data.AccountDetails[0]

		for _, streamInfo := range accountDetail.Streams {
			streamName := streamInfo.Name

			_, streamKnown := streamNamesSet[streamName]
			if !streamKnown {
				g.log.Infof("Discovered stream %s in account %s", streamName, accountId)
			}

			clusterTag := archive.TagNoCluster()
			if streamInfo.Cluster != nil {
				clusterTag = archive.TagCluster(streamInfo.Cluster.Name)
			}

			tags := []*archive.Tag{
				archive.TagAccount(accountId),
				archive.TagServer(serverName),
				clusterTag,
				archive.TagStream(streamName),
				archive.TagStreamInfo(),
			}

			err = g.aw.Add(streamInfo, tags...)
			if err != nil {
				return fmt.Errorf("failed to add stream %s info to archive: %w", streamName, err)
			}

			streamNamesSet[streamName] = nil
		}
	}

	g.log.Infof("Discovered %d streams in account %s", len(streamNamesSet), accountId)

	return nil
}

// Capture configured endpoints for each known account
func (g *gather) captureAccountEndpoints(serverInfoMap map[string]*server.ServerInfo, accountIdsToServersCountMap map[string]int) error {
	type Responder struct {
		ClusterName string
		ServerName  string
	}
	capturedCount := 0
	g.log.Infof("Querying %d endpoints for %d known accounts...", len(g.cfg.AccountEndpointConfigs), len(accountIdsToServersCountMap))

	for accountId, serversCount := range accountIdsToServersCountMap {
		for _, endpoint := range g.cfg.AccountEndpointConfigs {
			subject := fmt.Sprintf("$SYS.REQ.ACCOUNT.%s.%s", accountId, endpoint.ApiSuffix)
			endpointResponses := make(map[Responder]io.Reader, serversCount)

			err := g.doReqAsync(context.TODO(), nil, subject, serversCount, func(b []byte) {
				var apiResponse server.ServerAPIResponse
				err := json.Unmarshal(b, &apiResponse)
				if err != nil {
					g.log.Errorf("Failed to deserialize %s response for account %s: %s", endpoint.ApiSuffix, accountId, err)
					return
				}

				if apiResponse.Error != nil {
					g.log.Errorf("Received an error from server %s: (%d) %s", apiResponse.Server.Name, apiResponse.Error.ErrCode, apiResponse.Error.Description)
					return
				}

				serverId := apiResponse.Server.ID

				// Ignore responses from servers not discovered earlier.
				// We are discarding useful data, but limiting additional collection to a fixed set of nodes
				// simplifies querying and analysis. Could always re-run gather if a new server just joined.
				if _, serverKnown := serverInfoMap[serverId]; !serverKnown {
					g.log.Errorf("Ignoring account %s response from unknown server: %s", endpoint.ApiSuffix, serverId)
					return
				}

				buff := bytes.NewBuffer([]byte{})
				err = json.Indent(buff, b, "", "  ")
				if err != nil {
					g.log.Errorf("Failed to indent %s response for account %s: %s", endpoint.ApiSuffix, accountId, err)
					return
				}

				responder := Responder{
					ClusterName: apiResponse.Server.Cluster,
					ServerName:  apiResponse.Server.Name,
				}

				if _, isDuplicateResponse := endpointResponses[responder]; isDuplicateResponse {
					g.log.Errorf("Ignoring duplicate account %s response from server %s", endpoint.ApiSuffix, responder.ServerName)
					return
				}

				endpointResponses[responder] = buff
			})
			if err != nil {
				g.log.Errorf("Failed to request %s for account %s: %s", endpoint.ApiSuffix, accountId, err)
				continue
			}

			// Store all responses for this account endpoint
			for responder, endpointResponse := range endpointResponses {
				clusterTag := archive.TagNoCluster()
				if responder.ClusterName != "" {
					clusterTag = archive.TagCluster(responder.ClusterName)
				}

				tags := []*archive.Tag{
					archive.TagAccount(accountId),
					archive.TagServer(responder.ServerName),
					clusterTag,
					endpoint.TypeTag,
				}

				err = g.aw.AddRaw(endpointResponse, "json", tags...)
				if err != nil {
					return fmt.Errorf("failed to add response to %s to archive: %w", subject, err)
				}

				capturedCount += 1
			}
		}
	}

	g.log.Infof("Captured %d endpoint responses from %d accounts", capturedCount, len(accountIdsToServersCountMap))

	return nil
}

// Capture configured profiles for each known server
func (g *gather) captureServerProfiles(serverInfoMap map[string]*server.ServerInfo) error {
	g.log.Infof("Capturing %d profiles on %d known servers...", len(g.cfg.ServerProfileNames), len(serverInfoMap))

	capturedCount := 0
	timeout := g.cfg.Timeout

	for serverId, serverInfo := range serverInfoMap {
		serverName := serverInfo.Name
		clusterTag := archive.TagNoCluster()
		if serverInfo.Cluster != "" {
			clusterTag = archive.TagCluster(serverInfo.Cluster)
		}

		for _, profile := range g.cfg.ServerProfileNames {
			subject := fmt.Sprintf("$SYS.REQ.SERVER.%s.PROFILEZ", serverId)
			payload := server.ProfilezOptions{
				Name:  profile.name,
				Debug: profile.debug,
			}

			if profile.name == "cpu" {
				payload.Duration = timeout
				g.cfg.Timeout = timeout + 2*time.Second
			} else {
				g.cfg.Timeout = timeout
			}

			responses, err := g.doReq(context.TODO(), payload, subject, 1)
			if err != nil {
				g.log.Errorf("Failed to request %v (%d) profile from server %s: %s", profile, profile.debug, serverName, err)
				continue
			}

			if len(responses) != 1 {
				g.log.Errorf("Unexpected number of responses to %v profile from server %s: %d", profile, serverName, len(responses))
				continue
			}

			responseBytes := responses[0]

			var apiResponse struct {
				Server *server.ServerInfo     `json:"server"`
				Data   *server.ProfilezStatus `json:"data,omitempty"`
				Error  *server.ApiError       `json:"error,omitempty"`
			}

			if err = json.Unmarshal(responseBytes, &apiResponse); err != nil {
				g.log.Errorf("Failed to deserialize %v profile response from server %s: %s", profile, serverName, err)
				continue
			}
			if apiResponse.Error != nil {
				g.log.Errorf("Failed to retrieve %v profile from server %s: %s", profile, serverName, apiResponse.Error.Description)
				continue
			}

			profileStatus := apiResponse.Data
			if profileStatus.Error != "" {
				g.log.Errorf("Failed to retrieve %v profile from server %s: %s", profile, serverName, profileStatus.Error)
				continue
			}

			profileName := profile.name
			if profile.debug > 0 {
				profileName += fmt.Sprintf("_%d", profile.debug)
			}

			tags := []*archive.Tag{
				archive.TagServer(serverName),
				archive.TagServerProfile(),
				archive.TagProfileName(profileName),
				clusterTag,
			}

			err = g.aw.AddRaw(bytes.NewReader(profileStatus.Profile), "prof", tags...)
			if err != nil {
				return fmt.Errorf("failed to add %s profile from to archive: %w", profile.name, err)
			}

			capturedCount += 1
		}
	}

	g.log.Infof("Captured %d server profiles from %d servers", capturedCount, len(serverInfoMap))

	return nil
}

func (g *gather) hasNextPage(endpoint string, rawResponse []byte, pageLimit int) (bool, error) {
	jsonPath, pathKnown := endpointPagingInfo[endpoint]
	if !pathKnown {
		g.log.Debugf("no paging info configured for endpoint: %s", endpoint)
		return false, nil
	}

	var decoded map[string]any
	if err := json.Unmarshal(rawResponse, &decoded); err != nil {
		return false, fmt.Errorf("unable to decode JSON response for %s: %v", endpoint, err)
	}

	currentNode := any(decoded)
	for _, segment := range strings.Split(jsonPath, ".") {
		objectNode, isObject := currentNode.(map[string]any)
		if !isObject {
			return false, fmt.Errorf("expected object at path segment %q in %s, but got %T", segment, endpoint, currentNode)
		}

		nextNode, exists := objectNode[segment]
		if !exists {
			return false, fmt.Errorf("missing key %q in path for endpoint %s", segment, endpoint)
		}
		currentNode = nextNode
	}

	arrayNode, isArray := currentNode.([]any)
	if !isArray {
		return false, fmt.Errorf("expected array at end of path in %s, but got %T", endpoint, currentNode)
	}

	hasReachedPageLimit := len(arrayNode) >= pageLimit
	g.log.Debugf("paging check for %s: array length = %d, page limit = %d", endpoint, len(arrayNode), pageLimit)
	return hasReachedPageLimit, nil
}

func buildServerOptions(apiSuffix string, offset, limit int, detail bool) any {
	if !detail {
		return nil
	}

	switch apiSuffix {
	case "CONNZ":
		return &server.ConnzOptions{
			Subscriptions:       true,
			SubscriptionsDetail: true,
			Offset:              offset,
			Limit:               limit,
		}
	case "GATEWAYZ":
		return server.GatewayzOptions{
			Accounts:                   true,
			AccountSubscriptions:       true,
			AccountSubscriptionsDetail: true,
		}
	case "HEALTHZ":
		return server.HealthzOptions{
			Details: true,
		}
	case "JSZ":
		return server.JSzOptions{
			Accounts:   true,
			Streams:    true,
			Consumer:   true,
			Config:     true,
			RaftGroups: true,
			Offset:     offset,
			Limit:      limit,
		}
	case "LEAFZ":
		return server.LeafzOptions{
			Subscriptions: true,
		}
	case "ROUTEZ":
		return server.RoutezOptions{
			Subscriptions:       true,
			SubscriptionsDetail: true,
		}
	case "SUBSZ":
		return server.SubszOptions{
			Subscriptions: true,
			Offset:        offset,
			Limit:         limit,
		}
	default:
		return nil
	}
}

func (g *gather) captureServerEndpoints(serverInfoMap map[string]*server.ServerInfo, detail bool) error {
	if g.aw == nil {
		return fmt.Errorf("no archive writer supplied")
	}

	g.log.Infof("Querying %d endpoints on %d known servers...", len(g.cfg.ServerEndpointConfigs), len(serverInfoMap))
	capturedCount := 0
	const pageLimit = 1024

	for serverId, serverInfo := range serverInfoMap {
		serverName := serverInfo.Name

		for _, endpoint := range g.cfg.ServerEndpointConfigs {
			if endpoint.ApiSuffix == "JSZ" && !serverInfo.JetStream {
				g.log.Infof("Server %s does not have jetstream enabled - skipping JSZ endpoint", serverName)
				continue
			}

			subject := fmt.Sprintf("$SYS.REQ.SERVER.%s.%s", serverId, endpoint.ApiSuffix)
			offset := 0

			for {
				opts := buildServerOptions(endpoint.ApiSuffix, offset, pageLimit, detail)

				responses, err := g.doReq(context.TODO(), opts, subject, 1)
				if err != nil {
					g.log.Errorf("Failed to request %s from server %s: %s", endpoint.ApiSuffix, serverName, err)
					break
				}
				if len(responses) != 1 {
					g.log.Errorf("Unexpected number of responses to %s from server %s: %d", endpoint.ApiSuffix, serverName, len(responses))
					break
				}

				responseBytes := responses[0]
				var apiResponse server.ServerAPIResponse
				if err := json.Unmarshal(responseBytes, &apiResponse); err != nil {
					g.log.Errorf("Failed to deserialize %s response from server %s: %s", endpoint.ApiSuffix, serverName, err)
					break
				}
				if apiResponse.Error != nil {
					g.log.Errorf("Received error from server %s: (%d) %s", serverName, apiResponse.Error.ErrCode, apiResponse.Error.Description)
					break
				}

				// Pretty-print JSON
				buff := new(bytes.Buffer)
				if err := json.Indent(buff, responseBytes, "", "  "); err != nil {
					g.log.Errorf("Failed to indent %s response from server %s: %s", endpoint.ApiSuffix, serverName, err)
					break
				}

				// Tags for this artifact
				tags := []*archive.Tag{
					archive.TagServer(serverName),
					endpoint.TypeTag,
				}
				if serverInfo.Cluster != "" {
					tags = append(tags, archive.TagCluster(serverInfo.Cluster))
				} else {
					tags = append(tags, archive.TagNoCluster())
				}

				if err := g.aw.AddRaw(buff, "json", tags...); err != nil {
					return fmt.Errorf("failed to add endpoint %s response to archive: %w", subject, err)
				}
				capturedCount++

				g.log.Debugf("Checking paging for endpoint %s", endpoint.ApiSuffix)
				hasMore, err := g.hasNextPage(endpoint.ApiSuffix, responseBytes, pageLimit)

				if err != nil {
					return fmt.Errorf("failed to add endpoint %s response to archive: %w", subject, err)
				}
				if !hasMore {
					break
				}
				offset += pageLimit
			}
		}
	}

	g.log.Infof("Captured %d endpoint responses from %d servers", capturedCount, len(serverInfoMap))
	return nil
}

// Discover accounts by broadcasting a PING and then collecting responses
func (g *gather) discoverAccounts(serverInfoMap map[string]*server.ServerInfo) (map[string]int, string, error) {
	// Broadcast PING.ACCOUNTZ to discover (active) accounts
	// N.B. Inactive accounts (no connections) cannot be discovered this way
	g.log.Infof("Broadcasting PING to discover accounts... ")
	var accountIdsToServersCountMap = make(map[string]int)
	var systemAccount = ""
	err := g.doReqAsync(context.TODO(), nil, "$SYS.REQ.SERVER.PING.ACCOUNTZ", len(serverInfoMap), func(b []byte) {
		var apiResponse server.ServerAPIAccountzResponse
		err := json.Unmarshal(b, &apiResponse)
		if err != nil {
			g.log.Errorf("Failed to deserialize accounts response, ignoring: %v", err)
			return
		}

		serverId, serverName := apiResponse.Server.ID, apiResponse.Server.Name

		// Ignore responses from servers not discovered earlier.
		// We are discarding useful data, but limiting additional collection to a fixed set of nodes
		// simplifies querying and analysis. Could always re-run gather if a new server just joined.
		if _, serverKnown := serverInfoMap[serverId]; !serverKnown {
			g.log.Errorf("Ignoring accounts response from unknown server: %s", serverName)
			return
		}

		if apiResponse.Error != nil {
			g.log.Errorf("Received an error from server %s: (%d) %s", apiResponse.Server.Name, apiResponse.Error.ErrCode, apiResponse.Error.Description)
			return
		}

		g.log.Infof("Discovered %d accounts on server %s", len(apiResponse.Data.Accounts), serverName)

		// Track how many servers known any given account
		for _, accountId := range apiResponse.Data.Accounts {
			_, accountKnown := accountIdsToServersCountMap[accountId]
			if !accountKnown {
				accountIdsToServersCountMap[accountId] = 0
			}
			accountIdsToServersCountMap[accountId] += 1
		}

		// Track system account (normally, only one for the entire ensemble)
		if apiResponse.Data.SystemAccount == "" {
			g.log.Errorf("Server %s system account is not set", serverName)
		} else if systemAccount == "" {
			systemAccount = apiResponse.Data.SystemAccount
			g.log.Infof("Discovered system account name: %s", systemAccount)
		} else if systemAccount != apiResponse.Data.SystemAccount {
			// This should not happen under normal circumstances!
			g.log.Errorf("Multiple system accounts detected (%s, %s)", systemAccount, apiResponse.Data.SystemAccount)
		}
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to discover accounts: %w", err)
	}

	g.log.Infof("Discovered %d accounts over %d servers", len(accountIdsToServersCountMap), len(serverInfoMap))

	return accountIdsToServersCountMap, systemAccount, nil
}

func (g *gather) discoverServers() (map[string]*server.ServerInfo, error) {
	var serverInfoMap = make(map[string]*server.ServerInfo)

	g.log.Infof("Broadcasting PING to discover servers... (this may take a few seconds)")
	err := g.doReqAsync(context.TODO(), nil, "$SYS.REQ.SERVER.PING", doReqAsyncWaitFullTimeoutInterval, func(b []byte) {
		var apiResponse server.ServerAPIResponse
		if err := json.Unmarshal(b, &apiResponse); err != nil {
			g.log.Errorf("Failed to deserialize PING response: %s", err)
			return
		}

		if apiResponse.Error != nil {
			g.log.Errorf("Received an error from server %s: (%d) %s", apiResponse.Server.Name, apiResponse.Error.ErrCode, apiResponse.Error.Description)
			return
		}

		serverId, serverName := apiResponse.Server.ID, apiResponse.Server.Name

		_, exists := serverInfoMap[apiResponse.Server.ID]
		if exists {
			g.log.Errorf("Duplicate server %s (%s) response to PING, ignoring", serverId, serverName)
			return
		}

		serverInfoMap[serverId] = apiResponse.Server
		g.log.Infof("Discovered server '%s' (%s)", serverName, serverId)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to gather server responses: %w", err)
	}
	g.log.Infof("Discovered %d servers", len(serverInfoMap))
	return serverInfoMap, nil
}

func Gather(nc *nats.Conn, conf *Configuration) error {
	var captureLogBuffer bytes.Buffer

	g := &gather{
		cfg:     conf,
		nc:      nc,
		capture: &captureLogBuffer,
		log:     newLogger(&captureLogBuffer, conf.LogLevel),
	}

	return g.start()
}

// doReqAsyncWaitFullTimeoutInterval special value to be passed as `waitFor` argument of doReqAsync to turn off
// "adaptive" timeout and wait for the full interval
const doReqAsyncWaitFullTimeoutInterval = -1

// doReqAsync serializes and sends a request to the given subject and handles multiple responses.
// This function uses the value from `Timeout` CLI flag as upper limit for responses gathering.
// The value of the `waitFor` may shorten the interval during which responses are gathered:
//
//	waitFor < 0  : listen for responses for the full timeout interval
//	waitFor == 0 : (adaptive timeout), after each response, wait a short amount of time for more, then stop
//	waitFor > 0  : stops listening before the timeout if the given number of responses are received
func (g *gather) doReqAsync(ctx context.Context, req any, subj string, waitFor int, cb func([]byte)) error {
	jreq := []byte("{}")
	var err error

	if req != nil {
		switch val := req.(type) {
		case string:
			jreq = []byte(val)
		default:
			jreq, err = json.Marshal(req)
			if err != nil {
				return err
			}
		}
	}

	g.log.Debugf(">>> %s: %s\n", subj, string(jreq))

	var (
		mu       sync.Mutex
		ctr      = 0
		finisher *time.Timer
	)

	// Set deadline, max amount of time this function waits for responses
	ctx, cancel := context.WithTimeout(ctx, g.cfg.Timeout)
	defer cancel()

	// Activate "adaptive timeout". Finisher may trigger early termination
	if waitFor == 0 {
		// First response can take up to Timeout to arrive
		finisher = time.NewTimer(g.cfg.Timeout)
		go func() {
			select {
			case <-finisher.C:
				cancel()
			case <-ctx.Done():
				return
			}
		}()
	}

	errs := make(chan error)
	sub, err := g.nc.Subscribe(g.nc.NewRespInbox(), func(m *nats.Msg) {
		mu.Lock()
		defer mu.Unlock()

		data := m.Data
		compressed := false
		if m.Header.Get("Content-Encoding") == "snappy" {
			compressed = true
			ud, err := io.ReadAll(s2.NewReader(bytes.NewBuffer(data)))
			if err != nil {
				errs <- err
				return
			}
			data = ud
		}

		if compressed {
			g.log.Debugf("<<< (%dB -> %dB) %s", len(m.Data), len(data), string(data))
		} else {
			g.log.Debugf("<<< (%dB) %s", len(data), string(data))
		}

		if m.Header != nil {
			g.log.Debugf("<<< Header: %+v", m.Header)
		}

		// If adaptive timeout is active, set deadline for next response
		if finisher != nil {
			// Stop listening and return if no further responses arrive within this interval
			finisher.Reset(300 * time.Millisecond)
		}

		if m.Header.Get("Status") == "503" {
			errs <- nats.ErrNoResponders
			return
		}

		cb(data)
		ctr++

		// Stop listening if the requested number of responses have been received
		if waitFor > 0 && ctr == waitFor {
			cancel()
		}
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	if waitFor > 0 {
		sub.AutoUnsubscribe(waitFor)
	}

	msg := nats.NewMsg(subj)
	msg.Data = jreq
	if subj != "$SYS.REQ.SERVER.PING" && !strings.HasPrefix(subj, "$SYS.REQ.ACCOUNT") {
		msg.Header.Set("Accept-Encoding", "snappy")
	}
	msg.Reply = sub.Subject

	err = g.nc.PublishMsg(msg)
	if err != nil {
		return err
	}

	select {
	case err = <-errs:
		if err == nats.ErrNoResponders && strings.HasPrefix(subj, "$SYS") {
			return fmt.Errorf("server request failed, ensure the account used has system privileges and appropriate permissions")
		}

		return err
	case <-ctx.Done():
	}

	g.log.Debugf("=== Received %d responses", ctr)

	return nil
}

func (g *gather) doReq(ctx context.Context, req any, subj string, waitFor int) ([][]byte, error) {
	res := [][]byte{}
	mu := sync.Mutex{}

	err := g.doReqAsync(ctx, req, subj, waitFor, func(r []byte) {
		mu.Lock()
		res = append(res, r)
		mu.Unlock()
	})

	return res, err
}
