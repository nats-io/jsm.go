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
					Default:     90,
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
					Default:     90,
					Unit:        PercentageUnit,
				},
				"store": {
					Key:         "store",
					Description: "Threshold for store usage",
					Default:     90,
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

// checkServerHealth verify all known servers are reporting healthy
func checkServerHealth(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	total, err := r.EachClusterServerHealthz(func(clusterTag *archive.Tag, serverTag *archive.Tag, err error, hz *server.ServerAPIHealthzResponse) error {
		if errors.Is(err, archive.ErrNoMatches) {
			log.Warnf("Artifact 'Healthz' is missing for server %s", serverTag)
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to load variables for server %s: %w", serverTag, err)
		}

		if hz.Data.Status != "ok" && hz.Data.Status != "" {
			examples.Add("%s: %d - %s", serverTag, hz.Data.StatusCode, hz.Data.Status)
		}

		return nil
	})
	if err != nil {
		return Skipped, err
	}

	if examples.Count() > 0 {
		log.Errorf("%d/%d servers are not healthy", examples.Count(), total)
		return PassWithIssues, nil
	}

	log.Infof("%d/%d servers are healthy", total, total)

	return Pass, nil
}

// checkServerVersions verify all known servers are running the same version
func checkServerVersion(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	seenVersions := make(map[string]struct{})
	var lastVersionSeen string

	_, err := r.EachClusterServerVarz(func(clusterTag *archive.Tag, serverTag *archive.Tag, err error, vz *server.ServerAPIVarzResponse) error {
		if errors.Is(err, archive.ErrNoMatches) {
			log.Warnf("Artifact 'VARZ' is missing for server %s", serverTag)
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to load variables for server %s: %w", serverTag, err)
		}

		lastVersionSeen = vz.Data.Version
		_, exists := seenVersions[lastVersionSeen]
		if !exists {
			seenVersions[lastVersionSeen] = struct{}{}
			examples.Add("%s - %s", serverTag, lastVersionSeen)
		}

		return nil
	})
	if err != nil {
		return Skipped, err
	}

	if len(seenVersions) == 0 {
		log.Warnf("No servers version information found")
		return Skipped, nil
	} else if len(seenVersions) > 1 {
		log.Errorf("Servers are running %d different versions", len(seenVersions))
		return Fail, nil
	}

	// Map contains exactly one element (i.e. one version)
	examples.Clear()

	log.Infof("All servers are running version %s", lastVersionSeen)

	return Pass, nil
}

// checkServerCPUUsage verify CPU usage is below the given threshold for each server
func checkServerCPUUsage(check *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	cpuThreshold := check.Configuration["cpu"].Value()

	_, err := r.EachClusterServerVarz(func(clusterTag *archive.Tag, serverTag *archive.Tag, err error, vz *server.ServerAPIVarzResponse) error {
		if errors.Is(err, archive.ErrNoMatches) {
			log.Warnf("Artifact 'VARZ' is missing for server %s", serverTag)
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to load variables for server %s: %w", serverTag, err)
		}

		// Example: 350% usage with 4 cores => 87.5% averaged
		averageCpuUtilization := vz.Data.CPU / float64(vz.Data.Cores)

		if averageCpuUtilization > cpuThreshold {
			examples.Add("%s - %s: %.1f%%", clusterTag, serverTag, averageCpuUtilization)
		}

		return nil
	})
	if err != nil {
		return Skipped, err
	}

	if examples.Count() > 0 {
		log.Errorf("Found %d servers with >%.0f%% CPU usage", examples.Count(), cpuThreshold)
		return Fail, nil
	}

	return Pass, nil
}

// checkSlowConsumers verify that no server is reporting slow consumers
func checkSlowConsumers(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	_, err := r.EachClusterServerVarz(func(clusterTag *archive.Tag, serverTag *archive.Tag, err error, vz *server.ServerAPIVarzResponse) error {
		if errors.Is(err, archive.ErrNoMatches) {
			log.Warnf("Artifact 'VARZ' is missing for server %s", serverTag)
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to load variables for server %s: %w", serverTag, err)
		}

		if slowConsumers := vz.Data.SlowConsumers; slowConsumers > 0 {
			examples.Add("%s/%s: %d slow consumers", clusterTag, serverTag, slowConsumers)
		}

		return nil
	})
	if err != nil {
		return Skipped, err
	}

	if examples.Count() > 0 {
		log.Errorf("Total slow consumers: %d", examples.Count())
		return PassWithIssues, nil
	}

	log.Infof("No slow consumers detected")

	return Pass, nil
}

// checkServerResourceLimits verifies that the resource usage of memory and store is not approaching the reserved amount for each known server
func checkServerResourceLimits(check *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	memoryUsageThreshold := check.Configuration["memory"].Value()
	storeUsageThreshold := check.Configuration["store"].Value()

	_, err := r.EachClusterServerJsz(func(clusterTag *archive.Tag, serverTag *archive.Tag, err error, jsz *server.ServerAPIJszResponse) error {
		if errors.Is(err, archive.ErrNoMatches) {
			log.Warnf("Artifact 'JSZ' is missing for server %s", serverTag)
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to load variables for server %s: %w", serverTag, err)
		}

		if jsz.Data.Config.MaxMemory > 0 {
			threshold := uint64(float64(jsz.Data.Config.MaxMemory) * (memoryUsageThreshold / 100))
			if jsz.Data.Memory > threshold {
				examples.Add("%s memory usage: %s of %s", serverTag, humanize.IBytes(jsz.Data.Memory), humanize.IBytes(uint64(jsz.Data.Config.MaxMemory)))
			}
		}

		if jsz.Data.Config.MaxStore > 0 {
			threshold := uint64(float64(jsz.Data.Config.MaxStore) * (storeUsageThreshold / 100))
			if jsz.Data.Store > threshold {
				examples.Add("%s store usage: %s of %s", serverTag, humanize.IBytes(jsz.Data.Store), humanize.IBytes(uint64(jsz.Data.Config.MaxStore)))
			}
		}

		return nil
	})
	if err != nil {
		return Skipped, err
	}

	if examples.Count() > 0 {
		log.Errorf("Found %d instances of servers approaching reserved usage limit", examples.Count())
		return PassWithIssues, nil
	}

	return Pass, nil
}

// checkJetStreamDomainsForWhitespace verifies that no JetStream server is configured with whitespace in its domain
func checkJetStreamDomainsForWhitespace(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	_, err := r.EachClusterServerJsz(func(clusterTag *archive.Tag, serverTag *archive.Tag, err error, jsz *server.ServerAPIJszResponse) error {
		if errors.Is(err, archive.ErrNoMatches) {
			log.Warnf("Artifact 'JSZ' is missing for server %s", serverTag)
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to load variables for server %s: %w", serverTag, err)
		}

		// check if jetstream domain contains whitespace
		if strings.ContainsAny(jsz.Data.Config.Domain, " \n") {
			examples.Add("Cluster %s Server %s Domain %s", clusterTag, serverTag, jsz.Data.Config.Domain)
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

// checkServerAuthRequired verifies that all servers require authentication.
func checkServerAuthRequired(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	total, err := r.EachClusterServerVarz(func(clusterTag *archive.Tag, serverTag *archive.Tag, err error, vz *server.ServerAPIVarzResponse) error {
		if errors.Is(err, archive.ErrNoMatches) {
			log.Warnf("Artifact 'VARZ' is missing for server %s", serverTag)
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to load VARZ for server %s: %w", serverTag, err)
		}

		if !vz.Data.AuthRequired {
			examples.Add("%s: authentication not required", serverTag)
		}
		return nil
	})
	if err != nil {
		return Skipped, err
	}

	if examples.Count() > 0 {
		log.Errorf("%d/%d servers do not require authentication", examples.Count(), total)
		return PassWithIssues, nil
	}

	log.Infof("%d/%d servers require authentication", total, total)
	return Pass, nil
}
