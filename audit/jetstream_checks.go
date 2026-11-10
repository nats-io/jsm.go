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
	"fmt"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/jsm.go/monitor"
)

// streamWithConsumers extends StreamInfo with consumer detail as gathered by the audit tool.
type streamWithConsumers struct {
	api.StreamInfo
	ConsumerDetail []api.ConsumerInfo `json:"consumer_detail"`
}

func RegisterJetStreamChecks(collection *CheckCollection) error {
	return collection.Register(
		Check{
			Code:        "JETSTREAM_001",
			Suite:       "jetstream",
			Name:        "Stream Lagging Replicas",
			Description: "All replicas of a stream are keeping up",
			Configuration: map[string]*CheckConfiguration{
				"last_seq": {
					Key:         "last_seq",
					Description: "How far a replica may be behind the highest known last sequence",
					Default:     10,
					Unit:        PercentageUnit,
				},
			},
			Handler: checkStreamLaggingReplicas,
		},
		Check{
			Code:        "JETSTREAM_002",
			Suite:       "jetstream",
			Name:        "Stream High Cardinality",
			Description: "Streams unique subjects do not exceed a given threshold",
			Configuration: map[string]*CheckConfiguration{
				"subjects": {
					Key:         "subjects",
					Description: "Alerting threshold for unique subjects in a stream",
					Default:     1_000_000,
					Unit:        IntUnit,
				},
			},
			Handler: checkStreamHighCardinality,
		},
		Check{
			Code:        "JETSTREAM_003",
			Suite:       "jetstream",
			Name:        "Stream Limits",
			Description: "Stream usage is below the configured limits",
			Configuration: map[string]*CheckConfiguration{
				"messages": {
					Key:         "messages",
					Description: "Alert if messages near configured limit",
					Default:     90,
					Unit:        PercentageUnit,
				},
				"bytes": {
					Key:         "bytes",
					Description: "Alert if size near configured limit",
					Default:     90,
					Unit:        PercentageUnit,
				},
				"consumers": {
					Key:         "consumers",
					Description: "Alert if consumer count near configured limit",
					Default:     90,
					Unit:        PercentageUnit,
				},
			},
			Handler: checkStreamLimits,
		},
		Check{
			Code:        "JETSTREAM_004",
			Suite:       "jetstream",
			Name:        "Stream Metadata based monitoring",
			Description: "Stream health using the 'nats server check stream' metadata",
			Handler:     checkStreamMetadataMonitoring,
		},
		Check{
			Code:        "JETSTREAM_005",
			Suite:       "jetstream",
			Name:        "Consumer Metadata based monitoring",
			Description: "Consumer health using the 'nats server check consumer' metadata",
			Handler:     checkConsumerMetadataMonitoring,
		},
	)
}

// checkStreamLaggingReplicas verifies that in each known stream no replica is too far behind the most up to date (based on stream last sequence)
func checkStreamLaggingReplicas(check *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	typeTag := archive.TagStreamInfo()
	accountNames := r.AccountNames()
	lastSequenceLagThreshold := check.Configuration["last_seq"].Value()

	if len(accountNames) == 0 {
		log.Infof("No accounts found in archive")
	}

	accountsWithStreams := make(map[string]any)
	type accountStream struct {
		account string
		stream  archive.StreamRef
	}
	streamsInspected := make(map[accountStream]any)

	for _, accountName := range accountNames {
		streams := r.AccountStreams(accountName)

		if len(streams) == 0 {
			log.Debugf("No streams found in account: %s", accountName)
		}

		for _, stream := range streams {
			accountsWithStreams[accountName] = nil
			serverNames := r.StreamServerNames(accountName, stream)

			log.Debugf("Inspecting account '%s' stream '%s', found %d servers: %v", accountName, stream, len(serverNames), serverNames)

			// Create map server->streamDetails
			replicasStreamDetails := make(map[string]*api.StreamInfo, len(serverNames))
			streamIsEmpty := true

			for _, serverName := range serverNames {
				err := archive.ForEachStreamArtifact(r, accountName, stream, serverName, typeTag, func(streamDetails *api.StreamInfo) error {
					if streamDetails.State.LastSeq > 0 {
						streamIsEmpty = false
					}

					replicasStreamDetails[serverName] = streamDetails
					streamsInspected[accountStream{accountName, stream}] = nil
					return nil
				})
				if err != nil {
					log.Warnf("Artifact not found: %s for stream %s in account %s by server %s", typeTag, stream, accountName, serverName)
					continue
				}
			}

			// Check that all replicas are not too far behind the replica with the highest message & byte count
			if !streamIsEmpty {
				// Find the highest lastSeq
				highestLastSeq, highestLastSeqServer := uint64(0), ""
				for serverName, streamDetail := range replicasStreamDetails {
					lastSeq := streamDetail.State.LastSeq
					if lastSeq > highestLastSeq {
						highestLastSeq = lastSeq
						highestLastSeqServer = serverName
					}
				}
				log.Debugf("Stream %s / %s highest last sequence: %d @ %s", accountName, stream, highestLastSeq, highestLastSeqServer)

				// Check if some server's sequence is below warning threshold
				maxDelta := uint64(float64(highestLastSeq) * (lastSequenceLagThreshold / 100))
				threshold := uint64(0)
				if maxDelta <= highestLastSeq {
					threshold = highestLastSeq - maxDelta
				}
				for serverName, streamDetail := range replicasStreamDetails {
					lastSeq := streamDetail.State.LastSeq
					if lastSeq < threshold {
						examples.Add("%s/%s server %s lastSequence: %d is behind highest lastSequence: %d on server: %s", accountName, stream, serverName, lastSeq, highestLastSeq, highestLastSeqServer)
					}
				}
			}
		}
	}

	log.Infof("Inspected %d streams across %d accounts", len(streamsInspected), len(accountsWithStreams))

	if examples.Count() > 0 {
		log.Errorf("Found %d replicas lagging >%.0f%% behind", examples.Count(), lastSequenceLagThreshold)
		return Fail, nil
	}

	return Pass, nil
}

// checkStreamHighCardinality verifies that the number of unique subjects is below some magic number for each known stream
func checkStreamHighCardinality(check *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	streamDetailsTag := archive.TagStreamInfo()
	numSubjectsThreshold := check.Configuration["subjects"].Value()

	for _, accountName := range r.AccountNames() {

		for _, stream := range r.AccountStreams(accountName) {
			serverNames := r.StreamServerNames(accountName, stream)

			for _, serverName := range serverNames {
				err := archive.ForEachStreamArtifact(r, accountName, stream, serverName, streamDetailsTag, func(streamDetails *api.StreamInfo) error {
					if float64(streamDetails.State.NumSubjects) > numSubjectsThreshold {
						examples.Add("%s/%s: %d subjects", accountName, stream, streamDetails.State.NumSubjects)
					}
					return nil
				})
				if err != nil {
					log.Warnf("Artifact 'STREAM_DETAILS' is missing for stream %s in account %s", stream, accountName)
					continue
				}
			}
		}
	}

	if examples.Count() > 0 {
		log.Errorf("Found %d streams with subjects cardinality exceeding %.0f", examples.Count(), numSubjectsThreshold)
		return PassWithIssues, nil
	}

	return Pass, nil
}

// checkStreamLimits verifies that the number of messages/bytes/consumers is below a given threshold from the the configured limit for each known stream
func checkStreamLimits(check *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	messagesThreshold := check.Configuration["messages"].Value()
	bytesThreshold := check.Configuration["bytes"].Value()
	consumersThreshold := check.Configuration["consumers"].Value()

	// Check value against limit threshold, create example if exceeded
	checkLimit := func(limitName, accountName string, stream archive.StreamRef, serverName string, value, limit int64, percentThreshold float64) {
		if limit <= 0 {
			// Limit not set
			return
		}
		threshold := int64(float64(limit) * (percentThreshold / 100))
		if value > threshold {
			examples.Add("stream %s (in %s on %s) using %.1f%% of %s limit (%d/%d)", stream, accountName, serverName, float64(value)*100/float64(limit), limitName, value, limit)
		}
	}

	streamDetailsTag := archive.TagStreamInfo()

	for _, accountName := range r.AccountNames() {

		for _, stream := range r.AccountStreams(accountName) {
			serverNames := r.StreamServerNames(accountName, stream)

			for _, serverName := range serverNames {
				err := archive.ForEachStreamArtifact(r, accountName, stream, serverName, streamDetailsTag, func(streamDetails *api.StreamInfo) error {
					checkLimit(
						"messages",
						accountName,
						stream,
						serverName,
						int64(streamDetails.State.Msgs),
						streamDetails.Config.MaxMsgs,
						messagesThreshold,
					)

					checkLimit(
						"bytes",
						accountName,
						stream,
						serverName,
						int64(streamDetails.State.Bytes),
						streamDetails.Config.MaxBytes,
						bytesThreshold,
					)

					checkLimit(
						"consumers",
						accountName,
						stream,
						serverName,
						int64(streamDetails.State.Consumers),
						int64(streamDetails.Config.MaxConsumers),
						consumersThreshold,
					)

					return nil
				})
				if err != nil {
					log.Warnf("Artifact 'STREAM_DETAILS' is missing for stream %s in account %s", stream, accountName)
					continue
				}
			}
		}
	}

	if examples.Count() > 0 {
		log.Errorf("Found %d instances of streams approaching limit", examples.Count())
		return PassWithIssues, nil
	}

	return Pass, nil
}

func checkStreamMetadataMonitoring(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	streamDetailsTag := archive.TagStreamInfo()
	var foundCrit bool

	for _, accountName := range r.AccountNames() {

		for _, stream := range r.AccountStreams(accountName) {
			serverNames := r.StreamServerNames(accountName, stream)

			for _, serverName := range serverNames {
				err := archive.ForEachStreamArtifact(r, accountName, stream, serverName, streamDetailsTag, func(streamDetails *api.StreamInfo) error {
					if streamDetails.Cluster != nil && streamDetails.Cluster.Leader != serverName {
						return nil
					}

					check := &monitor.Result{Name: fmt.Sprintf("%s.%s", accountName, stream.Name), Check: "stream"}

					opts, err := monitor.ExtractStreamHealthCheckOptions(streamDetails.Config.Metadata)
					if err != nil {
						return fmt.Errorf("failed to run health check for stream %s in account %s: %w", stream, accountName, err)
					}

					if !opts.Enabled {
						return nil
					}

					opts.StreamName = stream.Name
					monitor.CheckStreamInfoHealth(streamDetails, check, *opts, log)

					for _, warning := range check.Warnings {
						examples.Add("WARNING: stream %s in %s: %s", stream, accountName, warning)
					}

					for _, crit := range check.Criticals {
						examples.Add("CRITICAL: stream %s in %s: %s", stream, accountName, crit)
						foundCrit = true
					}

					return nil
				})
				if err != nil {
					log.Warnf("Artifact 'STREAM_DETAILS' is missing for stream %s in account %s", stream, accountName)
					continue
				}
			}
		}
	}

	if examples.Count() > 0 {
		log.Errorf("Found %d instances of streams checks failing", examples.Count())
		if foundCrit {
			return Fail, nil
		}
		return PassWithIssues, nil
	}

	return Pass, nil
}

func checkConsumerMetadataMonitoring(_ *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	streamDetailsTag := archive.TagStreamInfo()
	var foundCrit bool

	for _, accountName := range r.AccountNames() {

		for _, stream := range r.AccountStreams(accountName) {
			serverNames := r.StreamServerNames(accountName, stream)

			for _, serverName := range serverNames {
				err := archive.ForEachStreamArtifact(r, accountName, stream, serverName, streamDetailsTag, func(streamDetails *streamWithConsumers) error {
					for _, nfo := range streamDetails.ConsumerDetail {
						if nfo.Cluster != nil && nfo.Cluster.Leader != serverName {
							continue
						}

						check := &monitor.Result{Name: fmt.Sprintf("%s.%s.%s", accountName, stream.Name, nfo.Name), Check: "consumer"}

						opts, err := monitor.ExtractConsumerHealthCheckOptions(nfo.Config.Metadata)
						if err != nil {
							return fmt.Errorf("failed to run health check for consumer %s > %s in account %s: %w", stream, nfo.Name, accountName, err)
						}

						if !opts.Enabled {
							continue
						}

						opts.StreamName = stream.Name
						opts.ConsumerName = nfo.Name

						monitor.CheckConsumerInfoHealth(&nfo, check, *opts, log)

						for _, warning := range check.Warnings {
							examples.Add("WARNING: consumer %s in stream %s in %s: %s", nfo.Name, stream, accountName, warning)
						}

						for _, crit := range check.Criticals {
							examples.Add("CRITICAL: consumer %s in stream %s in %s: %s", nfo.Name, stream, accountName, crit)
							foundCrit = true
						}
					}
					return nil
				})
				if err != nil {
					log.Warnf("Artifact 'STREAM_DETAILS' is missing for stream %s in account %s", stream, accountName)
					continue
				}
			}
		}
	}

	if examples.Count() > 0 {
		log.Errorf("Found %d instances of consumer checks failing", examples.Count())
		if foundCrit {
			return Fail, nil
		}
		return PassWithIssues, nil
	}

	return Pass, nil
}
