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
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/nats-server/v2/server"
)

func RegisterServerChecks(collection *CheckCollection) error {
	return collection.Register(
		Check{
			Code:        "SERVER_001",
			Suite:       "server",
			Name:        "Server Health",
			Description: "All known nodes are healthy",
			Handler:     checkServerHealth,
		},
		Check{
			Code:        "SERVER_002",
			Suite:       "server",
			Name:        "Server Version",
			Description: "All known nodes are running the same software version",
			Handler:     checkServerVersion,
		},
		Check{
			Code:        "SERVER_003",
			Suite:       "server",
			Name:        "Server CPU Usage",
			Description: "CPU usage for all known nodes is below a given threshold",
			Configuration: map[string]*CheckConfiguration{
				"cpu": {
					Key:         "cpu",
					Description: "CPU Limit Threshold",
					Default:     0.9,
					Unit:        PercentageUnit,
				},
			},
			Handler: checkServerCPUUsage,
		},
		Check{
			Code:        "SERVER_004",
			Suite:       "server",
			Name:        "Server Slow Consumers",
			Description: "No node is reporting slow consumers",
			Handler:     checkSlowConsumers,
		},
		Check{
			Code:        "SERVER_005",
			Suite:       "server",
			Name:        "Server Resources Limits ",
			Description: "Resource are below a given threshold compared to the configured limit",
			Configuration: map[string]*CheckConfiguration{
				"memory": {
					Key:         "memory",
					Description: "Threshold for memory usage",
					Default:     0.9,
					Unit:        PercentageUnit,
				},
				"store": {
					Key:         "store",
					Description: "Threshold for store usage",
					Default:     0.9,
					Unit:        PercentageUnit,
				},
			},
			Handler: checkServerResourceLimits,
		},
		Check{
			Code:        "SERVER_006",
			Suite:       "server",
			Name:        "Whitespace in JetStream domains",
			Description: "No JetStream server is configured with whitespace in its domain",
			Handler:     checkJetStreamDomainsForWhitespace,
		},
		Check{
			Code:        "SERVER_007",
			Suite:       "server",
			Name:        "Authentication required",
			Description: "Each server requires authentication",
			Handler:     checkServerAuthRequired,
		},
	)
}

func checkServerAuthRequired(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	notHealthy, healthy := 0, 0

	for _, clusterName := range r.ClusterNames() {
		clusterTag := archive.TagCluster(clusterName)

		for _, serverName := range r.ClusterServerNames(clusterName) {
			serverTag := archive.TagServer(serverName)

			var resp server.ServerAPIVarzResponse
			var varz *server.Varz
			err := r.Load(&resp, clusterTag, serverTag, archive.TagServerVars())
			if errors.Is(err, archive.ErrNoMatches) {
				log.Warnf("Artifact 'VARZ' is missing for server %s", serverName)
				continue
			} else if err != nil {
				return Skipped, fmt.Errorf("failed to load variables for server %s: %w", serverName, err)
			}
			varz = resp.Data

			if varz.AuthRequired {
				healthy++
			} else {
				examples.Add("%s: authentication not required", serverName)
				notHealthy++
			}
		}
	}

	if notHealthy > 0 {
		log.Errorf("%d/%d servers do not require authentication", notHealthy, healthy+notHealthy)
		return PassWithIssues, nil
	}

	log.Infof("%d/%d servers require authentication", healthy, healthy)

	return Pass, nil
}

// checkServerHealth verify all known servers are reporting healthy
func checkServerHealth(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	notHealthy, healthy := 0, 0

	for _, clusterName := range r.ClusterNames() {
		clusterTag := archive.TagCluster(clusterName)

		for _, serverName := range r.ClusterServerNames(clusterName) {
			serverTag := archive.TagServer(serverName)

			var resp server.ServerAPIHealthzResponse
			var health *server.HealthStatus
			err := r.Load(&resp, clusterTag, serverTag, archive.TagServerHealth())
			if errors.Is(err, archive.ErrNoMatches) {
				log.Warnf("Artifact 'HEALTHZ' is missing for server %s", serverName)
				continue
			} else if err != nil {
				return Skipped, fmt.Errorf("failed to load health for server %s: %w", serverName, err)
			}
			health = resp.Data

			if health.Status != "ok" {
				examples.Add("%s: %d - %s", serverName, health.StatusCode, health.Status)
				notHealthy += 1
			} else {
				healthy += 1
			}
		}
	}

	if notHealthy > 0 {
		log.Errorf("%d/%d servers are not healthy", notHealthy, healthy+notHealthy)
		return PassWithIssues, nil
	}

	log.Infof("%d/%d servers are healthy", healthy, healthy)
	return Pass, nil
}

// checkServerVersions verify all known servers are running the same version
func checkServerVersion(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	versionsToServersMap := make(map[string][]string)

	var lastVersionSeen string
	for _, clusterName := range r.ClusterNames() {
		clusterTag := archive.TagCluster(clusterName)

		for _, serverName := range r.ClusterServerNames(clusterName) {
			serverTag := archive.TagServer(serverName)

			var resp server.ServerAPIVarzResponse
			var serverVarz *server.Varz
			err := r.Load(&resp, clusterTag, serverTag, archive.TagServerVars())
			if errors.Is(err, archive.ErrNoMatches) {
				log.Warnf("Artifact 'VARZ' is missing for server %s", serverName)
				continue
			} else if err != nil {
				return Skipped, fmt.Errorf("failed to load variables for server %s: %w", serverTag.Value, err)
			}
			serverVarz = resp.Data

			version := serverVarz.Version

			_, exists := versionsToServersMap[version]
			if !exists {
				// First time encountering this version, create map entry
				versionsToServersMap[version] = []string{}
				// Add one example server for each version
				examples.Add("%s - %s", serverName, version)
			}
			// Add this server to the list running this version
			versionsToServersMap[version] = append(versionsToServersMap[version], serverName)
			lastVersionSeen = version
		}
	}

	if len(versionsToServersMap) == 0 {
		log.Warnf("No servers version information found")
		return Skipped, nil
	} else if len(versionsToServersMap) > 1 {
		log.Errorf("Servers are running %d different versions", len(versionsToServersMap))
		return Fail, nil
	}

	// Map contains exactly one element (i.e. one version)
	examples.clear()

	log.Infof("All servers are running version %s", lastVersionSeen)
	return Pass, nil
}

// checkServerCPUUsage verify CPU usage is below the given threshold for each server
func checkServerCPUUsage(check *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	severVarsTag := archive.TagServerVars()
	cpuThreshold := check.Configuration["cpu"].Value()

	for _, clusterName := range r.ClusterNames() {
		clusterTag := archive.TagCluster(clusterName)
		for _, serverName := range r.ClusterServerNames(clusterName) {
			serverTag := archive.TagServer(serverName)

			var resp server.ServerAPIVarzResponse
			var serverVarz *server.Varz
			err := r.Load(&resp, serverTag, clusterTag, severVarsTag)
			if errors.Is(err, archive.ErrNoMatches) {
				log.Warnf("Artifact 'VARZ' is missing for server %s", serverName)
				continue
			} else if err != nil {
				return Skipped, fmt.Errorf("failed to load VARZ for server %s: %w", serverName, err)
			}
			serverVarz = resp.Data

			// Example: 350% usage with 4 cores => 87.5% averaged
			averageCpuUtilization := serverVarz.CPU / float64(serverVarz.Cores)

			if averageCpuUtilization > cpuThreshold {
				examples.Add("%s - %s: %.1f%%", clusterName, serverName, averageCpuUtilization)
			}
		}
	}

	if examples.Count() > 0 {
		log.Errorf("Found %d servers with >%.0f%% CPU usage", examples.Count(), cpuThreshold)
		return Fail, nil
	}

	return Pass, nil
}

// checkSlowConsumers verify that no server is reporting slow consumers
func checkSlowConsumers(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	totalSlowConsumers := int64(0)

	for _, clusterName := range r.ClusterNames() {
		clusterTag := archive.TagCluster(clusterName)

		for _, serverName := range r.ClusterServerNames(clusterName) {
			serverTag := archive.TagServer(serverName)

			var resp server.ServerAPIVarzResponse
			var serverVarz *server.Varz
			err := r.Load(&resp, clusterTag, serverTag, archive.TagServerVars())
			if err != nil {
				return Skipped, fmt.Errorf("failed to load Varz for server %s: %w", serverName, err)
			}
			serverVarz = resp.Data

			if slowConsumers := serverVarz.SlowConsumers; slowConsumers > 0 {
				examples.Add("%s/%s: %d slow consumers", clusterName, serverName, slowConsumers)
				totalSlowConsumers += slowConsumers
			}
		}
	}

	if totalSlowConsumers > 0 {
		log.Errorf("Total slow consumers: %d", totalSlowConsumers)
		return PassWithIssues, nil
	}

	log.Infof("No slow consumers detected")
	return Pass, nil
}

// checkServerResourceLimits verifies that the resource usage of memory and store is not approaching the reserved amount for each known server
func checkServerResourceLimits(check *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	jsTag := archive.TagServerJetStream()

	memoryUsageThreshold := check.Configuration["memory"].Value()
	storeUsageThreshold := check.Configuration["store"].Value()

	for _, clusterName := range r.ClusterNames() {
		clusterTag := archive.TagCluster(clusterName)
		for _, serverName := range r.ClusterServerNames(clusterName) {
			serverTag := archive.TagServer(serverName)

			var resp server.ServerAPIJszResponse
			var serverJSInfo *server.JSInfo
			err := r.Load(&resp, clusterTag, serverTag, jsTag)
			if errors.Is(err, archive.ErrNoMatches) {
				log.Warnf("Artifact 'JSZ' is missing for server %s cluster %s", serverName, clusterName)
				continue
			} else if err != nil {
				return Skipped, fmt.Errorf("failed to load JSZ for server %s: %w", serverName, err)
			}
			serverJSInfo = resp.Data

			if serverJSInfo.ReservedMemory > 0 {
				threshold := uint64(float64(serverJSInfo.ReservedMemory) * memoryUsageThreshold)
				if serverJSInfo.Memory > threshold {
					examples.Add(
						"%s memory usage: %s of %s",
						serverName,
						humanize.IBytes(serverJSInfo.Memory),
						humanize.IBytes(serverJSInfo.ReservedMemory),
					)
				}
			}

			if serverJSInfo.ReservedStore > 0 {
				threshold := uint64(float64(serverJSInfo.ReservedStore) * storeUsageThreshold)
				if serverJSInfo.Store > threshold {
					examples.Add(
						"%s store usage: %s of %s",
						serverName,
						humanize.IBytes(serverJSInfo.Store),
						humanize.IBytes(serverJSInfo.ReservedStore),
					)
				}
			}
		}
	}

	if examples.Count() > 0 {
		log.Errorf("Found %d instances of servers approaching reserved usage limit", examples.Count())
		return PassWithIssues, nil
	}

	return Pass, nil
}

func checkJetStreamDomainsForWhitespace(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	for _, clusterName := range r.ClusterNames() {
		clusterTag := archive.TagCluster(clusterName)

		for _, serverName := range r.ClusterServerNames(clusterName) {
			serverTag := archive.TagServer(serverName)

			var resp server.ServerAPIJszResponse
			var serverJsz *server.JSInfo
			err := r.Load(&resp, clusterTag, serverTag, archive.TagServerJetStream())
			if err != nil {
				log.Warnf("Artifact 'JSZ' is missing for server %s", serverName)
				continue
			}
			serverJsz = resp.Data

			// check if jetstream domain contains whitespace
			if strings.ContainsAny(serverJsz.Config.Domain, " \n") {
				examples.Add("Cluster %s Server %s Domain %s", clusterName, serverName, serverJsz.Config.Domain)
			}
		}
	}

	if examples.Count() > 0 {
		log.Errorf("Found %d servers with JetStream domains containing whitespace", examples.Count())
		return Fail, nil
	}

	return Pass, nil
}
