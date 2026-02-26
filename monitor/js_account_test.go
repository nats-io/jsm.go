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

package monitor_test

import (
	"testing"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/monitor"
)

func TestCheckAccountInfo(t *testing.T) {
	setDefaults := func() (*monitor.CheckJetStreamAccountOptions, *api.JetStreamAccountStats, *monitor.Result) {
		info := &api.JetStreamAccountStats{
			JetStreamTier: api.JetStreamTier{
				Memory:    128,
				Store:     1024,
				Streams:   10,
				Consumers: 100,
				Limits: api.JetStreamAccountLimits{
					MaxMemory:    1024,
					MaxStore:     20480,
					MaxStreams:   200,
					MaxConsumers: 1000,
				},
			},
		}

		// cli defaults
		cmd := &monitor.CheckJetStreamAccountOptions{
			ConsumersCritical: -1,
			ConsumersWarning:  -1,
			StreamCritical:    -1,
			StreamWarning:     -1,
			FileCritical:      90,
			FileWarning:       75,
			MemoryCritical:    90,
			MemoryWarning:     75,
			Resolver: func() *api.JetStreamAccountStats {
				return info
			},
		}

		return cmd, info, &monitor.Result{}
	}

	t.Run("No limits, default thresholds", func(t *testing.T) {
		opts, info, check := setDefaults()
		info.Limits = api.JetStreamAccountLimits{}
		assertNoError(t, monitor.CheckJetStreamAccount("", nil, nil, check, *opts))
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.Warnings)
		assertHasPDItem(t, check, "memory=128B memory_pct=0%;75;90 storage=1024B storage_pct=0%;75;90 streams=10 streams_pct=0% consumers=100 consumers_pct=0%")
	})

	t.Run("Limits, default thresholds", func(t *testing.T) {
		opts, _, check := setDefaults()
		assertNoError(t, monitor.CheckJetStreamAccount("", nil, nil, check, *opts))
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.Warnings)
		assertHasPDItem(t, check, "memory=128B memory_pct=12%;75;90 storage=1024B storage_pct=5%;75;90 streams=10 streams_pct=5% consumers=100 consumers_pct=10%")
	})

	t.Run("Limits, Thresholds", func(t *testing.T) {
		t.Run("Usage exceeds max", func(t *testing.T) {
			opts, info, check := setDefaults()
			info.Streams = 300
			assertNoError(t, monitor.CheckJetStreamAccount("", nil, nil, check, *opts))
			assertListEquals(t, check.Criticals, "streams: exceed server limits")
			assertListIsEmpty(t, check.Warnings)
			assertHasPDItem(t, check, "memory=128B memory_pct=12%;75;90 storage=1024B storage_pct=5%;75;90 streams=300 streams_pct=150% consumers=100 consumers_pct=10%")
		})

		t.Run("Invalid thresholds", func(t *testing.T) {
			opts, _, check := setDefaults()
			opts.MemoryWarning = 90
			opts.MemoryCritical = 80
			assertNoError(t, monitor.CheckJetStreamAccount("", nil, nil, check, *opts))
			assertListEquals(t, check.Criticals, "memory: invalid thresholds")
		})

		t.Run("Exceeds warning threshold", func(t *testing.T) {
			opts, info, check := setDefaults()
			info.Memory = 800
			assertNoError(t, monitor.CheckJetStreamAccount("", nil, nil, check, *opts))
			assertHasPDItem(t, check, "memory=800B memory_pct=78%;75;90 storage=1024B storage_pct=5%;75;90 streams=10 streams_pct=5% consumers=100 consumers_pct=10%")
			assertListIsEmpty(t, check.Criticals)
			assertListEquals(t, check.Warnings, "78% memory")
		})

		t.Run("Exceeds critical threshold", func(t *testing.T) {
			opts, info, check := setDefaults()

			info.Memory = 960
			assertNoError(t, monitor.CheckJetStreamAccount("", nil, nil, check, *opts))
			assertHasPDItem(t, check, "memory=960B memory_pct=93%;75;90 storage=1024B storage_pct=5%;75;90 streams=10 streams_pct=5% consumers=100 consumers_pct=10%")
			assertListEquals(t, check.Criticals, "93% memory")
			assertListIsEmpty(t, check.Warnings)
		})

		t.Run("Streams warning threshold", func(t *testing.T) {
			opts, _, check := setDefaults()
			// 10 streams / 200 max = 5%; set warn=4 crit=8 -> warning
			opts.StreamWarning = 4
			opts.StreamCritical = 8
			assertNoError(t, monitor.CheckJetStreamAccount("", nil, nil, check, *opts))
			assertListIsEmpty(t, check.Criticals)
			assertListEquals(t, check.Warnings, "5% streams")
		})

		t.Run("Streams critical threshold", func(t *testing.T) {
			opts, info, check := setDefaults()
			// 180 streams / 200 max = 90%; set warn=70 crit=85 -> critical
			info.Streams = 180
			opts.StreamWarning = 70
			opts.StreamCritical = 85
			assertNoError(t, monitor.CheckJetStreamAccount("", nil, nil, check, *opts))
			assertListEquals(t, check.Criticals, "90% streams")
			assertListIsEmpty(t, check.Warnings)
		})

		t.Run("Consumers warning threshold", func(t *testing.T) {
			opts, _, check := setDefaults()
			// 100 consumers / 1000 max = 10%; set warn=8 crit=15 -> warning
			opts.ConsumersWarning = 8
			opts.ConsumersCritical = 15
			assertNoError(t, monitor.CheckJetStreamAccount("", nil, nil, check, *opts))
			assertListIsEmpty(t, check.Criticals)
			assertListEquals(t, check.Warnings, "10% consumers")
		})

		t.Run("Consumers critical threshold", func(t *testing.T) {
			opts, info, check := setDefaults()
			// 900 consumers / 1000 max = 90%; set warn=70 crit=85 -> critical
			info.Consumers = 900
			opts.ConsumersWarning = 70
			opts.ConsumersCritical = 85
			assertNoError(t, monitor.CheckJetStreamAccount("", nil, nil, check, *opts))
			assertListEquals(t, check.Criticals, "90% consumers")
			assertListIsEmpty(t, check.Warnings)
		})
	})

	t.Run("Nil resolver return is critical", func(t *testing.T) {
		opts, _, check := setDefaults()
		opts.Resolver = func() *api.JetStreamAccountStats { return nil }
		assertNoError(t, monitor.CheckJetStreamAccount("", nil, nil, check, *opts))
		assertListEquals(t, check.Criticals, "JetStream not available: invalid account status")
	})

	t.Run("CheckReplicas with no manager connection is critical", func(t *testing.T) {
		opts, _, check := setDefaults()
		opts.CheckReplicas = true
		// Resolver is set so mgr stays nil inside CheckJetStreamAccount
		assertNoError(t, monitor.CheckJetStreamAccount("", nil, nil, check, *opts))
		assertListEquals(t, check.Criticals, "replica checks require a manager connection")
	})
}
