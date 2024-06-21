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

package jsm

import (
	"strconv"
	"time"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/monitor"
)

const (
	MonitorEnabled                    = "io.nats.monitor.enabled"
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

type StreamHealthCheck func(*Stream, *monitor.Result, StreamHealthCheckOptions, api.Logger)

type StreamHealthCheckOptions struct {
	Enabled              bool
	SourcesLagCritical   uint64
	SourcesSeenCritical  time.Duration
	MinSources           int
	MaxSources           int
	ClusterExpectedPeers int
	ClusterLagCritical   uint64
	ClusterSeenCritical  time.Duration
	MessagesWarn         uint64
	MessagesCrit         uint64
	SubjectsWarn         int
	SubjectsCrit         int
	HealthChecks         []StreamHealthCheck
}

type monitorMetaParser struct {
	k  string
	fn func(string) error
}

func (s *Stream) HealthCheckOptions(extraChecks ...StreamHealthCheck) (*StreamHealthCheckOptions, error) {
	opts := &StreamHealthCheckOptions{
		HealthChecks: extraChecks,
	}

	var err error
	parser := []monitorMetaParser{
		{MonitorEnabled, func(v string) error {
			opts.Enabled, err = strconv.ParseBool(v)
			return err
		}},
		{StreamMonitorMetaLagCritical, func(v string) error {
			opts.SourcesLagCritical, err = strconv.ParseUint(v, 10, 64)
			return err
		}},
		{StreamMonitorMetaSeenCritical, func(v string) error {
			opts.SourcesSeenCritical, err = parseDuration(v)
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
			opts.ClusterSeenCritical, err = parseDuration(v)
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

	metadata := s.Metadata()

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

func (s *Stream) HealthCheck(opts StreamHealthCheckOptions, check *monitor.Result, log api.Logger) (*monitor.Result, error) {
	if check == nil {
		check = &monitor.Result{
			Check: "stream_status",
			Name:  s.Name(),
		}
	}

	// make sure latest info cache is set as checks accesses it directly
	nfo, err := s.LatestInformation()
	if err != nil {
		return nil, err
	}

	s.checkCluster(nfo, check, opts, log)
	s.checkMessages(nfo, check, opts, log)
	s.checkSubjects(nfo, check, opts, log)
	s.checkSources(nfo, check, opts, log)
	s.checkMirror(nfo, check, opts, log)

	for _, hc := range opts.HealthChecks {
		hc(s, check, opts, log)
	}

	return check, nil
}

func (s *Stream) checkMirror(si *api.StreamInfo, check *monitor.Result, opts StreamHealthCheckOptions, log api.Logger) {
	if (opts.SourcesLagCritical <= 0 && opts.SourcesSeenCritical <= 0) || si.Config.Name == "" || si.Config.Mirror == nil {
		return
	}

	if si.Config.Name == "" {
		log.Debugf("CRITICAL: no configuration present")
		check.Critical("no configuration present")
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
		&monitor.PerfDataItem{Name: "lag", Crit: float64(opts.SourcesLagCritical), Value: float64(state.Lag), Help: "Number of operations this peer is behind its origin"},
		&monitor.PerfDataItem{Name: "active", Crit: opts.SourcesSeenCritical.Seconds(), Unit: "s", Value: state.Active.Seconds(), Help: "Indicates if this peer is active and catching up if lagged"},
	)

	ok := true

	if opts.SourcesLagCritical > 0 && state.Lag >= opts.SourcesLagCritical {
		log.Debugf("CRITICAL: Mirror lag %d", state.Lag)
		check.Critical("Mirror Lag %d", state.Lag)
		ok = false
	}

	if opts.SourcesSeenCritical > 0 && state.Active >= opts.SourcesSeenCritical {
		log.Debugf("CRITICAL: Mirror Seen > %v", state.Active)
		check.Critical("Mirror Seen %v", state.Active)
		ok = false
	}

	if ok {
		check.Ok("Mirror %s", mirror.Name)
	}
}

func (s *Stream) checkSources(si *api.StreamInfo, check *monitor.Result, opts StreamHealthCheckOptions, log api.Logger) {
	sources := si.Sources
	count := len(sources)

	check.Pd(&monitor.PerfDataItem{Name: "sources", Value: float64(len(sources)), Warn: float64(opts.MinSources), Crit: float64(opts.MaxSources), Help: "Number of sources being consumed by this stream"})

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

		if opts.SourcesSeenCritical > 0 && s.Active >= opts.SourcesSeenCritical {
			inactive++
		}
	}

	check.Pd(
		&monitor.PerfDataItem{Name: "sources_lagged", Value: float64(lagged), Help: "Number of sources that are behind more than the configured threshold"},
		&monitor.PerfDataItem{Name: "sources_inactive", Value: float64(inactive), Help: "Number of sources that are inactive"},
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

func (s *Stream) checkSubjects(si *api.StreamInfo, check *monitor.Result, opts StreamHealthCheckOptions, log api.Logger) {
	if opts.SubjectsWarn <= 0 && opts.SubjectsCrit <= 0 {
		return
	}

	ns := si.State.NumSubjects
	lt := opts.SubjectsWarn < opts.SubjectsCrit

	check.Pd(&monitor.PerfDataItem{Name: "subjects", Value: float64(ns), Warn: float64(opts.SubjectsWarn), Crit: float64(opts.SubjectsCrit), Help: "Number of subjects stored in the stream"})

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
func (s *Stream) checkMessages(si *api.StreamInfo, check *monitor.Result, opts StreamHealthCheckOptions, log api.Logger) {
	if opts.MessagesCrit <= 0 && opts.MessagesWarn <= 0 {
		return
	}

	check.Pd(&monitor.PerfDataItem{Name: "messages", Value: float64(si.State.Msgs), Warn: float64(opts.MessagesWarn), Crit: float64(opts.MessagesCrit), Help: "Messages stored in the stream"})

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

func (s *Stream) checkCluster(si *api.StreamInfo, check *monitor.Result, opts StreamHealthCheckOptions, log api.Logger) {
	nfo := si.Cluster
	if nfo == nil && opts.ClusterExpectedPeers <= 0 {
		return
	}

	if nfo == nil {
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

		if opts.ClusterSeenCritical > 0 && p.Active > opts.ClusterSeenCritical {
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
