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

package audit

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/nats-server/v2/server"
)

func RegisterClusterChecks(collection *CheckCollection) error {
	return collection.Register(
		Check{
			Code:        "CLUSTER_001",
			Suite:       "cluster",
			Name:        "Cluster Memory Usage Outliers",
			Description: "Memory usage is uniform across nodes in a cluster",
			Configuration: map[string]*CheckConfiguration{
				"memory": {
					Key:         "memory",
					Description: "Threshold of memory usage above average",
					Default:     1.5,
				},
			},
			Handler: checkClusterMemoryUsageOutliers,
		},
		Check{
			Code:        "CLUSTER_002",
			Suite:       "cluster",
			Name:        "Cluster Uniform Gateways",
			Description: "All nodes in a cluster share the same gateways configuration",
			Handler:     checkClusterUniformGatewayConfig,
		},
		Check{
			Code:        "CLUSTER_003",
			Suite:       "cluster",
			Name:        "Cluster High HA Assets",
			Description: "Number of HA assets is below a given threshold",
			Configuration: map[string]*CheckConfiguration{
				"assets": {
					Key:         "assets",
					Description: "Number of HA assets per server",
					Default:     1000,
					Unit:        UIntUnit,
				},
			},
			Handler: checkClusterHighHAAssets,
		},
		Check{
			Code:        "CLUSTER_004",
			Suite:       "cluster",
			Name:        "Whitespace in cluster name",
			Description: "No cluster name contains whitespace",
			Handler:     checkClusterNamesForWhitespace,
		},
	)
}

// checkClusterMemoryUsageOutliers verifies the memory usage of any given node in a cluster is not significantly higher than its peers
func checkClusterMemoryUsageOutliers(check *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	typeTag := archive.TagServerVars()
	clusterNames := r.ClusterNames()
	outlierThreshold := check.Configuration["memory"].Value()

	clustersWithIssuesMap := make(map[string]any, len(clusterNames))

	for _, clusterName := range clusterNames {
		clusterTag := archive.TagCluster(clusterName)

		serverNames := r.ClusterServerNames(clusterName)
		clusterMemoryUsageMap := make(map[string]float64, len(serverNames))
		clusterMemoryUsageTotal := float64(0)
		numServers := 0 // cannot use len(serverNames) as some artifacts may be missing

		for _, serverName := range serverNames {
			serverTag := archive.TagServer(serverName)

			var resp server.ServerAPIVarzResponse
			var serverVarz *server.Varz
			err := r.Load(&resp, clusterTag, serverTag, typeTag)
			if errors.Is(err, archive.ErrNoMatches) {
				log.Warnf("Artifact 'VARZ' is missing for server %s in cluster %s", serverName, clusterName)
				continue
			} else if err != nil {
				return Skipped, fmt.Errorf("failed to load VARZ for server %s in cluster %s: %w", serverName, clusterName, err)
			}
			serverVarz = resp.Data

			numServers += 1
			clusterMemoryUsageMap[serverTag.Value] = float64(serverVarz.Mem)
			clusterMemoryUsageTotal += float64(serverVarz.Mem)
		}

		clusterMemoryUsageMean := clusterMemoryUsageTotal / float64(numServers)
		threshold := clusterMemoryUsageMean * outlierThreshold

		for serverName, serverMemoryUsage := range clusterMemoryUsageMap {
			if serverMemoryUsage > threshold {
				examples.Add("Cluster %s avg: %s, server %s: %s", clusterName, humanize.IBytes(uint64(clusterMemoryUsageMean)), serverName, humanize.IBytes(uint64(serverMemoryUsage)))
				clustersWithIssuesMap[clusterName] = nil
			}
		}
	}

	if len(clustersWithIssuesMap) > 0 {
		log.Errorf(
			"Servers with memory usage above %.1fX the cluster average: %d in %d clusters",
			outlierThreshold,
			examples.Count(),
			len(clustersWithIssuesMap),
		)
		return PassWithIssues, nil
	}

	return Pass, nil
}

// checkClusterUniformGatewayConfig verify that gateways configuration matches for all nodes in each cluster
func checkClusterUniformGatewayConfig(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	for _, clusterName := range r.ClusterNames() {
		clusterTag := archive.TagCluster(clusterName)

		// For each cluster, build a map where they key is a server name in the cluster
		// And the value is a list of configured remote target clusters
		configuredOutboundGateways := make(map[string][]string)
		configuredInboundGateways := make(map[string][]string)
		for _, serverName := range r.ClusterServerNames(clusterName) {
			serverTag := archive.TagServer(serverName)

			var resp server.ServerAPIGatewayzResponse
			var gateways *server.Gatewayz
			err := r.Load(&resp, clusterTag, serverTag, archive.TagServerGateways())
			if errors.Is(err, archive.ErrNoMatches) {
				return Skipped, fmt.Errorf("artifact 'GATEWAYZ' is missing for server %s cluster %s", serverName, clusterName)
			} else if err != nil {
				return Skipped, fmt.Errorf("failed to load GATEWAYZ for server %s: %w", serverName, err)
			}
			gateways = resp.Data

			// Create list of configured outbound gateways for this server
			serverConfiguredOutboundGateways := make([]string, 0, len(gateways.OutboundGateways))
			for targetClusterName, outboundGateway := range gateways.OutboundGateways {
				if outboundGateway.IsConfigured {
					serverConfiguredOutboundGateways = append(serverConfiguredOutboundGateways, targetClusterName)
				}
			}

			// Create list of configured inbound gateways for this server
			serverConfiguredInboundGateways := make([]string, 0, len(gateways.OutboundGateways))
			for sourceClusterName, inboundGateways := range gateways.InboundGateways {
				for _, inboundGateway := range inboundGateways {
					if inboundGateway.IsConfigured {
						serverConfiguredInboundGateways = append(serverConfiguredInboundGateways, sourceClusterName)
						break
					}
				}
			}

			// Sort the lists for easier comparison later
			sort.Strings(serverConfiguredOutboundGateways)
			sort.Strings(serverConfiguredInboundGateways)
			// Store for later comparison against other servers in the cluster
			configuredOutboundGateways[serverName] = serverConfiguredOutboundGateways
			configuredInboundGateways[serverName] = serverConfiguredInboundGateways
		}

		gatewayTypes := []struct {
			gatewayType        string
			configuredGateways map[string][]string
		}{
			{"inbound", configuredInboundGateways},
			{"outbound", configuredOutboundGateways},
		}

		for _, t := range gatewayTypes {
			// Check each server configured gateways against another server in the same cluster
			var previousServerName string
			var previousTargetClusterNames []string
			for serverName, targetClusterNames := range t.configuredGateways {
				if previousTargetClusterNames != nil {
					log.Debugf("Cluster %s - Comparing configured %s gateways of %s (%d) to %s (%d)", clusterName, t.gatewayType, serverName, len(targetClusterNames), previousServerName, len(previousTargetClusterNames))
					if !reflect.DeepEqual(targetClusterNames, previousTargetClusterNames) {
						examples.Add(
							"Cluster %s, %s gateways server %s: %v != server %s: %v",
							clusterName,
							t.gatewayType,
							serverName,
							targetClusterNames,
							previousServerName,
							previousTargetClusterNames,
						)
					}
				}
				previousServerName = serverName
				previousTargetClusterNames = targetClusterNames
			}
		}
	}
	if examples.Count() > 0 {
		log.Errorf("Found %d instance of gateways configurations mismatch", examples.Count())
		return Fail, nil
	}

	return Pass, nil
}

// checkClusterHighHAAssets verifies the number of HA assets is below some the given number for each known server in each known cluster
func checkClusterHighHAAssets(check *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	haAssetsThreshold := check.Configuration["assets"].Value()

	_, err := r.EachClusterServerJsz(func(clusterTag *archive.Tag, serverTag *archive.Tag, err error, jsz *server.ServerAPIJszResponse) error {
		if errors.Is(err, archive.ErrNoMatches) {
			log.Warnf("Artifact 'JSZ' is missing for server %s", serverTag)
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to load variables for server %s: %w", serverTag, err)
		}

		if float64(jsz.Data.HAAssets) > haAssetsThreshold {
			examples.Add("%s: %d HA assets", serverTag, jsz.Data.HAAssets)
		}

		return nil
	})
	if err != nil {
		return Skipped, err
	}

	if examples.Count() > 0 {
		log.Errorf("Found %d servers with JetStream domains containing whitespace", examples.Count())
		return PassWithIssues, nil
	}

	return Pass, nil
}

func checkClusterNamesForWhitespace(_ *Check, reader *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	for _, clusterName := range reader.ClusterNames() {
		if strings.ContainsAny(clusterName, " \n") {
			examples.Add("Cluster: %s", clusterName)
		}
	}

	if examples.Count() > 0 {
		log.Errorf("Found %d clusters with names containing whitespace", examples.Count())
		return Fail, nil
	}

	return Pass, nil
}
