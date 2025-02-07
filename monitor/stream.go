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

package monitor

import (
	"strconv"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
)

const (
	MonitorMetaEnabled                = "io.nats.monitor.enabled"
	StreamMonitorMetaLagCritical      = "io.nats.monitor.lag-critical"
	StreamMonitorMetaSeenCritical     = "io.nats.monitor.seen-critical"
	StreamMonitorMetaMinSources       = "io.nats.monitor.min-sources"
	StreamMonitorMetaMaxSources       = "io.nats.monitor.max-sources"
	StreamMonitorMetaPeerExpect       = "io.nats.monitor.peer-expect"
	StreamMonitorMetaPeerLagCritical  = "io.nats.monitor.peer-lag-critical"
	StreamMonitorMetaPeerSeenCritical = "io.nats.monitor.peer-seen-critical"
	StreamMonitorMetaMessagesWarn     = "io.nats.monitor.msgs-warn"
	StreamMonitorMetaMessagesCritical = "io.nats.monitor.msgs-critical"
	StreamMonitorMetaSubjectsWarn     = "io.nats.monitor.subjects-warn"
	StreamMonitorMetaSubjectsCritical = "io.nats.monitor.subjects-critical"
)

type StreamHealthCheckF func(*jsm.Stream, *Result, CheckStreamHealthOptions, api.Logger)

// CheckStreamHealthOptions configures the stream check
type CheckStreamHealthOptions struct {
	// StreamName stream to monitor
	StreamName string `json:"stream_name" yaml:"stream_name"`
	// SourcesLagCritical critical threshold for how many operations behind sources may be
	SourcesLagCritical uint64 `json:"sources_lag_critical" yaml:"sources_lag_critical"`
	// SourcesSeenCritical critical threshold for how long a source stream must have been visible (seconds)
	SourcesSeenCritical float64 `json:"sources_seen_critical" yaml:"sources_seen_critical"`
	// MinSources minimum number of sources to allow
	MinSources int `json:"min_sources" yaml:"min_sources"`
	// MaxSources maximum number of sources to allow
	MaxSources int `json:"max_sources" yaml:"max_sources"`
	// ClusterExpectedPeers how many peers should be in a clustered stream (Replica count)
	ClusterExpectedPeers int `json:"cluster_expected_peers" yaml:"cluster_expected_peers"`
	// ClusterLagCritical critical threshold for how many operations behind cluster peers may be
	ClusterLagCritical uint64 `json:"cluster_lag_critical" yaml:"cluster_lag_critical"`
	// ClusterSeenCritical critical threshold for how long ago a cluster peer should have been seen
	ClusterSeenCritical float64 `json:"cluster_seen_critical" yaml:"cluster_seen_critical"`
	// MessagesWarn is the warning level for number of messages in the stream
	MessagesWarn uint64 `json:"messages_warn" yaml:"messages_warn"`
	// MessagesCrit is the critical level for number of messages in the stream
	MessagesCrit uint64 `json:"messages_critical" yaml:"messages_critical"`
	// SubjectsWarn is the warning level for number of subjects in the stream
	SubjectsWarn int `json:"subjects_warn" yaml:"subjects_warn"`
	// SubjectsCrit is the critical level for number of subjects in the stream
	SubjectsCrit int `json:"subjects_critical" yaml:"subjects_critical"`

	Enabled      bool                 `json:"-" yaml:"-"`
	HealthChecks []StreamHealthCheckF `json:"-" yaml:"-"`
}

type monitorMetaParser struct {
	k  string
	fn func(string) error
}

// ExtractStreamHealthCheckOptions checks stream metadata and populate CheckStreamHealthOptions based on it
func ExtractStreamHealthCheckOptions(metadata map[string]string, extraChecks ...StreamHealthCheckF) (*CheckStreamHealthOptions, error) {
	opts := &CheckStreamHealthOptions{
		HealthChecks: extraChecks,
	}

	return populateStreamHealthCheckOptions(metadata, opts)
}

func populateStreamHealthCheckOptions(metadata map[string]string, opts *CheckStreamHealthOptions) (*CheckStreamHealthOptions, error) {
	var err error
	parser := []monitorMetaParser{
		{MonitorMetaEnabled, func(v string) error {
			opts.Enabled, err = strconv.ParseBool(v)
			return err
		}},
		{StreamMonitorMetaLagCritical, func(v string) error {
			opts.SourcesLagCritical, err = strconv.ParseUint(v, 10, 64)
			return err
		}},
		{StreamMonitorMetaSeenCritical, func(v string) error {
			p, err := jsm.ParseDuration(v)
			opts.ClusterSeenCritical = p.Seconds()
			return err
		}},
		{StreamMonitorMetaMinSources, func(v string) error {
			opts.MinSources, err = strconv.Atoi(v)
			return err
		}},
		{StreamMonitorMetaMaxSources, func(v string) error {
			opts.MaxSources, err = strconv.Atoi(v)
			return err
		}},
		{StreamMonitorMetaPeerExpect, func(v string) error {
			opts.ClusterExpectedPeers, err = strconv.Atoi(v)
			return err
		}},
		{StreamMonitorMetaPeerLagCritical, func(v string) error {
			opts.ClusterLagCritical, err = strconv.ParseUint(v, 10, 64)
			return err
		}},
		{StreamMonitorMetaPeerSeenCritical, func(v string) error {
			p, err := jsm.ParseDuration(v)
			if err != nil {
				return err
			}
			opts.ClusterSeenCritical = p.Seconds()

			return err
		}},
		{StreamMonitorMetaMessagesWarn, func(v string) error {
			opts.MessagesWarn, err = strconv.ParseUint(v, 10, 64)
			return err
		}},
		{StreamMonitorMetaMessagesCritical, func(v string) error {
			opts.MessagesCrit, err = strconv.ParseUint(v, 10, 64)
			return err
		}},
		{StreamMonitorMetaSubjectsWarn, func(v string) error {
			opts.SubjectsWarn, err = strconv.Atoi(v)
			return err
		}},
		{StreamMonitorMetaSubjectsCritical, func(v string) error {
			opts.SubjectsCrit, err = strconv.Atoi(v)
			return err
		}},
	}

	for _, m := range parser {
		if v, ok := metadata[m.k]; ok {
			err = m.fn(v)
			if err != nil {
				return nil, err
			}
		}
	}

	return opts, nil
}

func CheckStreamInfoHealth(nfo *api.StreamInfo, check *Result, opts CheckStreamHealthOptions, log api.Logger) {
	streamCheckCluster(nfo, check, opts, log)
	streamCheckMessages(nfo, check, opts, log)
	streamCheckSubjects(nfo, check, opts, log)
	streamCheckSources(nfo, check, opts, log)
	streamCheckMirror(nfo, check, opts, log)
}

func CheckStreamHealth(server string, nopts []nats.Option, check *Result, opts CheckStreamHealthOptions, log api.Logger) error {
	if opts.StreamName == "" {
		check.Critical("stream name is required")
		return nil
	}

	nc, err := nats.Connect(server, nopts...)
	if check.CriticalIfErr(err, "could not load info: %v", err) {
		return nil
	}

	mgr, err := jsm.New(nc)
	if check.CriticalIfErr(err, "could not load info: %v", err) {
		return nil
	}

	stream, err := mgr.LoadStream(opts.StreamName)
	if check.CriticalIfErr(err, "could not load info: %v", err) {
		return nil
	}

	_, err = populateStreamHealthCheckOptions(stream.Metadata(), &opts)
	if check.CriticalIfErr(err, "could not configure based on metadata: %v", err) {
		return nil
	}

	// make sure latest info cache is set as checks accesses it directly
	nfo, err := stream.LatestInformation()
	if check.CriticalIfErr(err, "could not load info: %v", err) {
		return nil
	}

	CheckStreamInfoHealth(nfo, check, opts, log)

	for _, hc := range opts.HealthChecks {
		hc(stream, check, opts, log)
	}

	return nil
}

func streamCheckMirror(si *api.StreamInfo, check *Result, opts CheckStreamHealthOptions, log api.Logger) {
	// We check sources here because they are mutually exclusive with mirrors. If sources are set, mirrors
	// can't be so we can bail out of this check early
	if (opts.SourcesLagCritical <= 0 && opts.SourcesSeenCritical <= 0) || si.Config.Name == "" || len(si.Sources) > 0 {
		return
	}

	mirror := si.Config.Mirror
	state := si.Mirror

	if mirror == nil {
		log.Debugf("CRITICAL: not mirrored")
		check.Critical("not mirrored")
		return
	}

	if state == nil {
		log.Debugf("CRITICAL: invalid state")
		check.Critical("invalid state")
		return
	}

	check.Pd(
		&PerfDataItem{Name: "lag", Crit: float64(opts.SourcesLagCritical), Value: float64(state.Lag), Help: "Number of operations this peer is behind its origin"},
		&PerfDataItem{Name: "active", Crit: opts.SourcesSeenCritical, Unit: "s", Value: state.Active.Seconds(), Help: "Indicates if this peer is active and catching up if lagged"},
	)

	ok := true

	if opts.SourcesLagCritical > 0 && state.Lag >= opts.SourcesLagCritical {
		log.Debugf("CRITICAL: Mirror lag %d", state.Lag)
		check.Critical("Mirror Lag %d", state.Lag)
		ok = false
	}

	if opts.SourcesSeenCritical > 0 && state.Active >= secondsToDuration(opts.SourcesSeenCritical) {
		log.Debugf("CRITICAL: Mirror Seen > %v", state.Active)
		check.Critical("Mirror Seen %v", state.Active)
		ok = false
	}

	if ok {
		check.Ok("Mirror %s", mirror.Name)
	}
}

func streamCheckSources(si *api.StreamInfo, check *Result, opts CheckStreamHealthOptions, log api.Logger) {
	sources := si.Sources
	count := len(sources)

	check.Pd(&PerfDataItem{Name: "sources", Value: float64(len(sources)), Warn: float64(opts.MinSources), Crit: float64(opts.MaxSources), Help: "Number of sources being consumed by this stream"})

	switch {
	case opts.MinSources > 0 && count < opts.MinSources:
		//log.Debugf("CRITICAL: %d/%d sources", count, opts.MinSources)
		check.Critical("%d sources", count)
	case opts.MaxSources > 0 && count > opts.MaxSources:
		//log.Debugf("CRITICAL: %d/%d sources", count, opts.MaxSources)
		check.Critical("%d sources", count)
	default:
		check.Ok("%d sources", count)
	}

	if opts.SourcesLagCritical <= 0 && opts.SourcesSeenCritical <= 0 {
		return
	}

	lagged := 0
	inactive := 0

	for _, s := range sources {
		if opts.SourcesLagCritical > 0 && s.Lag >= opts.SourcesLagCritical {
			lagged++
		}

		if opts.SourcesSeenCritical > 0 && s.Active >= secondsToDuration(opts.SourcesSeenCritical) {
			inactive++
		}
	}

	check.Pd(
		&PerfDataItem{Name: "sources_lagged", Value: float64(lagged), Help: "Number of sources that are behind more than the configured threshold"},
		&PerfDataItem{Name: "sources_inactive", Value: float64(inactive), Help: "Number of sources that are inactive"},
	)

	if lagged > 0 {
		log.Debugf("CRITICAL: %d/%d sources are lagged", lagged, count)
		check.Critical("%d sources are lagged", lagged)
	} else {
		check.Ok("%d sources current", count)
	}
	if inactive > 0 {
		log.Debugf("CRITICAL: %d/%d sources are inactive", inactive, count)
		check.Critical("%d sources are inactive", inactive)
	} else {
		check.Ok("%d sources active", count)
	}
}

func streamCheckSubjects(si *api.StreamInfo, check *Result, opts CheckStreamHealthOptions, log api.Logger) {
	if opts.SubjectsWarn <= 0 && opts.SubjectsCrit <= 0 {
		return
	}

	ns := si.State.NumSubjects
	lt := opts.SubjectsWarn < opts.SubjectsCrit

	check.Pd(&PerfDataItem{Name: "subjects", Value: float64(ns), Warn: float64(opts.SubjectsWarn), Crit: float64(opts.SubjectsCrit), Help: "Number of subjects stored in the stream"})

	switch {
	case lt && ns >= opts.SubjectsCrit:
		log.Debugf("CRITICAL subjects %d <= %d", ns, opts.SubjectsCrit)
		check.Critical("%d subjects", ns)
	case lt && ns >= opts.SubjectsWarn:
		log.Debugf("WARNING subjects %d >= %d", ns, opts.SubjectsWarn)
		check.Warn("%d subjects", ns)
	case !lt && ns <= opts.SubjectsCrit:
		check.Critical("%d subjects", ns)
		log.Debugf("CRITICAL subjects %d <= %d", ns, opts.SubjectsCrit)
	case !lt && ns <= opts.SubjectsWarn:
		check.Warn("%d subjects", ns)
		log.Debugf("WARNING subjects %d >= %d", ns, opts.SubjectsWarn)
	default:
		check.Ok("%d subjects", ns)
	}
}

// TODO: support inverting logic and also in cli
func streamCheckMessages(si *api.StreamInfo, check *Result, opts CheckStreamHealthOptions, log api.Logger) {
	if opts.MessagesCrit <= 0 && opts.MessagesWarn <= 0 {
		return
	}

	check.Pd(&PerfDataItem{Name: "messages", Value: float64(si.State.Msgs), Warn: float64(opts.MessagesWarn), Crit: float64(opts.MessagesCrit), Help: "Messages stored in the stream"})

	if opts.MessagesCrit > 0 && si.State.Msgs <= opts.MessagesCrit {
		log.Debugf("CRITICAL: %d messages", si.State.Msgs)
		check.Critical("%d messages", si.State.Msgs)
		return
	}

	if opts.MessagesWarn > 0 && si.State.Msgs <= opts.MessagesWarn {
		log.Debugf("WARNING: %d messages expected <= %d", si.State.Msgs, opts.MessagesWarn)
		check.Warn("%d messages", si.State.Msgs)
		return
	}

	check.Ok("%d messages", si.State.Msgs)
}

func streamCheckCluster(si *api.StreamInfo, check *Result, opts CheckStreamHealthOptions, log api.Logger) {
	nfo := si.Cluster

	if (nfo == nil || si.Config.Replicas <= 1) && opts.ClusterExpectedPeers <= 0 {
		return
	}

	if nfo == nil || si.Config.Replicas <= 1 {
		log.Debugf("Stream is not clustered")
		check.Critical("Stream is not clustered")
		return
	}

	hasLeader := nfo.Leader != ""
	nPeer := len(nfo.Replicas)
	if hasLeader {
		nPeer++
	} else {
		check.Critical("No leader")
		log.Debugf("No leader found")
		return
	}

	if nPeer != opts.ClusterExpectedPeers {
		log.Debugf("Expected %d replicas got %d", opts.ClusterExpectedPeers, nPeer)
		check.Critical("Expected %d replicas got %d", opts.ClusterExpectedPeers, nPeer)
		return
	} else {
		check.Ok("%d peers", nPeer)
	}

	inactive := 0
	lagged := 0
	offline := 0

	for _, p := range nfo.Replicas {
		if opts.ClusterLagCritical > 0 && p.Lag > opts.ClusterLagCritical {
			lagged++
		}

		if opts.ClusterSeenCritical > 0 && p.Active > secondsToDuration(opts.ClusterSeenCritical) {
			inactive++
		}

		if p.Offline {
			offline++
		}
	}

	if offline > 0 {
		log.Debugf("CRITICAL: %d replicas are offline", offline)
		check.Critical("%d replicas offline", offline)
	}

	switch {
	case opts.ClusterLagCritical <= 0:
	case lagged > 0:
		//log.Debugf("CRITICAL: %d replicas are lagged", lagged)
		check.Critical("%d replicas lagged", lagged)
	default:
		check.Ok("replicas are current")
	}

	switch {
	case opts.ClusterSeenCritical <= 0:
	case inactive > 0:
		//log.Debugf("CRITICAL: %d replicas are inactive", inactive)
		check.Critical("%d replicas inactive", inactive)
	default:
		check.Ok("replicas are active")
	}
}
