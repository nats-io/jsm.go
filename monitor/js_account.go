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
	"fmt"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
)

type CheckJetStreamAccountOptions struct {
	MemoryWarning       int     `json:"memory_warning" yaml:"memory_warning"`
	MemoryCritical      int     `json:"memory_critical" yaml:"memory_critical"`
	FileWarning         int     `json:"file_warning" yaml:"file_warning"`
	FileCritical        int     `json:"file_critical" yaml:"file_critical"`
	StreamWarning       int     `json:"stream_warning" yaml:"stream_warning"`
	StreamCritical      int     `json:"stream_critical" yaml:"stream_critical"`
	ConsumersWarning    int     `json:"consumers_warning" yaml:"consumers_warning"`
	ConsumersCritical   int     `json:"consumers_critical" yaml:"consumers_critical"`
	CheckReplicas       bool    `json:"check_replicas" yaml:"check_replicas"`
	ReplicaSeenCritical float64 `json:"replica_seen_critical" yaml:"replica_seen_critical"`
	ReplicaLagCritical  uint64  `json:"replica_lag_critical" yaml:"replica_lag_critical"`

	Resolver func() *api.JetStreamAccountStats `json:"-" yaml:"-"`
}

func CheckJetStreamAccountWithConnection(mgr *jsm.Manager, check *Result, opts CheckJetStreamAccountOptions) error {
	var err error

	if opts.Resolver == nil {
		opts.Resolver = func() *api.JetStreamAccountStats {
			info, err := mgr.JetStreamAccountInfo()
			if check.CriticalIfErrf(err, "JetStream not available: %s", err) {
				return nil
			}
			return info
		}
	}

	info := opts.Resolver()

	err = checkJSAccountInfo(check, &opts, info)
	if check.CriticalIfErrf(err, "JetStream not available: %s", err) {
		return nil
	}

	if opts.CheckReplicas {
		streams, _, err := mgr.Streams(nil)
		if check.CriticalIfErrf(err, "JetStream not available: %s", err) {
			return nil
		}

		err = checkStreamClusterHealth(check, &opts, streams)
		if check.CriticalIfErrf(err, "JetStream not available: %s", err) {
			return nil
		}
	}

	return nil
}

func CheckJetStreamAccount(server string, nopts []nats.Option, jsmOpts []jsm.Option, check *Result, opts CheckJetStreamAccountOptions) error {
	var nc *nats.Conn
	var mgr *jsm.Manager
	var err error

	if opts.Resolver == nil {
		nc, err = nats.Connect(server, nopts...)
		if check.CriticalIfErrf(err, "connection failed: %v", err) {
			return nil
		}
		defer nc.Close()

		mgr, err = jsm.New(nc, jsmOpts...)
		if check.CriticalIfErrf(err, "setup failed: %v", err) {
			return nil
		}
	}

	return CheckJetStreamAccountWithConnection(mgr, check, opts)
}

func checkStreamClusterHealth(check *Result, opts *CheckJetStreamAccountOptions, info []*jsm.Stream) error {
	var okCnt, noLeaderCnt, notEnoughReplicasCnt, critCnt, lagCritCnt, seenCritCnt int

	for _, s := range info {
		nfo, err := s.LatestInformation()
		if err != nil {
			critCnt++
			continue
		}

		if nfo.Config.Replicas == 1 {
			okCnt++
			continue
		}

		if nfo.Cluster == nil {
			critCnt++
			continue
		}

		if nfo.Cluster.Leader == "" {
			noLeaderCnt++
			continue
		}

		if len(nfo.Cluster.Replicas) != s.Replicas()-1 {
			notEnoughReplicasCnt++
			continue
		}

		for _, r := range nfo.Cluster.Replicas {
			if r.Offline {
				critCnt++
				continue
			}

			if r.Active > secondsToDuration(opts.ReplicaSeenCritical) {
				seenCritCnt++
				continue
			}
			if r.Lag > opts.ReplicaLagCritical {
				lagCritCnt++
				continue
			}
		}

		okCnt++
	}

	check.Pd(&PerfDataItem{
		Name:  "replicas_ok",
		Value: float64(okCnt),
		Help:  "Streams with healthy cluster state",
	})

	check.Pd(&PerfDataItem{
		Name:  "replicas_no_leader",
		Value: float64(noLeaderCnt),
		Help:  "Streams with no leader elected",
	})

	check.Pd(&PerfDataItem{
		Name:  "replicas_missing_replicas",
		Value: float64(notEnoughReplicasCnt),
		Help:  "Streams where there are fewer known replicas than configured",
	})

	check.Pd(&PerfDataItem{
		Name:  "replicas_lagged",
		Value: float64(lagCritCnt),
		Crit:  float64(opts.ReplicaLagCritical),
		Help:  fmt.Sprintf("Streams with > %d lagged replicas", opts.ReplicaLagCritical),
	})

	check.Pd(&PerfDataItem{
		Name:  "replicas_not_seen",
		Value: float64(seenCritCnt),
		Crit:  opts.ReplicaSeenCritical,
		Unit:  "s",
		Help:  fmt.Sprintf("Streams with replicas seen > %s ago", secondsToDuration(opts.ReplicaSeenCritical)),
	})

	check.Pd(&PerfDataItem{
		Name:  "replicas_fail",
		Value: float64(critCnt),
		Help:  "Streams unhealthy cluster state",
	})

	if critCnt > 0 || notEnoughReplicasCnt > 0 || noLeaderCnt > 0 || seenCritCnt > 0 || lagCritCnt > 0 {
		check.Criticalf("%d unhealthy streams", critCnt+notEnoughReplicasCnt+noLeaderCnt)
	}

	return nil
}

func checkJSAccountInfo(check *Result, opts *CheckJetStreamAccountOptions, info *api.JetStreamAccountStats) error {
	if info == nil {
		return fmt.Errorf("invalid account status")
	}

	checkVal := func(item string, unit string, warn int, crit int, max int64, current uint64) {
		pct := 0
		if max > 0 {
			pct = int(float64(current) / float64(max) * 100)
		}

		check.Pd(&PerfDataItem{Name: item, Value: float64(current), Unit: unit, Help: fmt.Sprintf("JetStream %s resource usage", item)})
		check.Pd(&PerfDataItem{
			Name:  fmt.Sprintf("%s_pct", item),
			Value: float64(pct),
			Unit:  "%",
			Warn:  float64(warn),
			Crit:  float64(crit),
			Help:  fmt.Sprintf("JetStream %s resource usage in percent", item),
		})

		if warn != -1 && crit != -1 && warn >= crit {
			check.Criticalf("%s: invalid thresholds", item)
			return
		}

		if pct > 100 {
			check.Criticalf("%s: exceed server limits", item)
			return
		}

		if warn >= 0 && crit >= 0 {
			switch {
			case pct > crit:
				check.Criticalf("%d%% %s", pct, item)
			case pct > warn:
				check.Warnf("%d%% %s", pct, item)
			}
		}
	}

	checkVal("memory", "B", opts.MemoryWarning, opts.MemoryCritical, info.Limits.MaxMemory, info.Memory)
	checkVal("storage", "B", opts.FileWarning, opts.FileCritical, info.Limits.MaxStore, info.Store)
	checkVal("streams", "", opts.StreamWarning, opts.StreamCritical, int64(info.Limits.MaxStreams), uint64(info.Streams))
	checkVal("consumers", "", opts.ConsumersWarning, opts.ConsumersCritical, int64(info.Limits.MaxConsumers), uint64(info.Consumers))

	return nil
}
