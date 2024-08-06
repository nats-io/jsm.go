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
	setDefaults := func() (*monitor.JetStreamAccountOptions, *api.JetStreamAccountStats, *monitor.Result) {
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
		cmd := &monitor.JetStreamAccountOptions{
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
		assertNoError(t, monitor.CheckJetStreamAccount(nil, check, *opts))
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.Warnings)
		assertHasPDItem(t, check, "memory=128B memory_pct=0%;75;90 storage=1024B storage_pct=0%;75;90 streams=10 streams_pct=0% consumers=100 consumers_pct=0%")
	})

	t.Run("Limits, default thresholds", func(t *testing.T) {
		opts, _, check := setDefaults()
		assertNoError(t, monitor.CheckJetStreamAccount(nil, check, *opts))
		assertListIsEmpty(t, check.Criticals)
		assertListIsEmpty(t, check.Warnings)
		assertHasPDItem(t, check, "memory=128B memory_pct=12%;75;90 storage=1024B storage_pct=5%;75;90 streams=10 streams_pct=5% consumers=100 consumers_pct=10%")
	})

	t.Run("Limits, Thresholds", func(t *testing.T) {
		t.Run("Usage exceeds max", func(t *testing.T) {
			opts, info, check := setDefaults()
			info.Streams = 300
			assertNoError(t, monitor.CheckJetStreamAccount(nil, check, *opts))
			assertListEquals(t, check.Criticals, "streams: exceed server limits")
			assertListIsEmpty(t, check.Warnings)
			assertHasPDItem(t, check, "memory=128B memory_pct=12%;75;90 storage=1024B storage_pct=5%;75;90 streams=300 streams_pct=150% consumers=100 consumers_pct=10%")
		})

		t.Run("Invalid thresholds", func(t *testing.T) {
			opts, _, check := setDefaults()
			opts.MemoryWarning = 90
			opts.MemoryCritical = 80
			assertNoError(t, monitor.CheckJetStreamAccount(nil, check, *opts))
			assertListEquals(t, check.Criticals, "memory: invalid thresholds")
		})

		t.Run("Exceeds warning threshold", func(t *testing.T) {
			opts, info, check := setDefaults()
			info.Memory = 800
			assertNoError(t, monitor.CheckJetStreamAccount(nil, check, *opts))
			assertHasPDItem(t, check, "memory=800B memory_pct=78%;75;90 storage=1024B storage_pct=5%;75;90 streams=10 streams_pct=5% consumers=100 consumers_pct=10%")
			assertListIsEmpty(t, check.Criticals)
			assertListEquals(t, check.Warnings, "78% memory")
		})

		t.Run("Exceeds critical threshold", func(t *testing.T) {
			opts, info, check := setDefaults()

			info.Memory = 960
			assertNoError(t, monitor.CheckJetStreamAccount(nil, check, *opts))
			assertHasPDItem(t, check, "memory=960B memory_pct=93%;75;90 storage=1024B storage_pct=5%;75;90 streams=10 streams_pct=5% consumers=100 consumers_pct=10%")
			assertListEquals(t, check.Criticals, "93% memory")
			assertListIsEmpty(t, check.Warnings)
		})
	})
}
