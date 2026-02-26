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
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
)

func requireLen[A any](t *testing.T, s []A, expect int) {
	t.Helper()

	if len(s) != expect {
		t.Fatalf("Expected %d elements in collection: %v", expect, s)
	}
}
func requireEmpty[A any](t *testing.T, s []A) {
	t.Helper()

	if len(s) != 0 {
		t.Fatalf("Expected empty collection: %v", s)
	}
}

func requireElement[S ~[]E, E comparable](t *testing.T, s S, v E) {
	t.Helper()

	if !slices.Contains(s, v) {
		t.Fatalf("Expected %v to contain %v", s, v)
	}
}

func requireRegexElement(t *testing.T, s []string, m string) {
	t.Helper()

	r, err := regexp.Compile(m)
	if err != nil {
		t.Fatalf("invalid regex: %v", err)
	}

	if !slices.ContainsFunc(s, func(s string) bool {
		return r.MatchString(s)
	}) {
		t.Fatalf("Expected %v to contain element matching %q", s, m)
	}
}

func TestConsumer_checkLastAck(t *testing.T) {
	setup := func() (*Result, *api.ConsumerInfo) {
		return &Result{}, &api.ConsumerInfo{}
	}

	t.Run("Should skip without a threshold", func(t *testing.T) {
		check, ci := setup()
		consumerCheckLastAck(ci, check, CheckConsumerHealthOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle no ack floor", func(t *testing.T) {
		check, ci := setup()
		consumerCheckLastAck(ci, check, CheckConsumerHealthOptions{
			LastAckCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "No acks")
	})

	t.Run("Should handle time greater than or equal", func(t *testing.T) {
		check, ci := setup()
		last := time.Now().Add(-time.Hour)
		ci.AckFloor.Last = &last
		consumerCheckLastAck(ci, check, CheckConsumerHealthOptions{
			LastAckCritical: 1,
		}, api.NewDiscardLogger())
		requireLen(t, check.Criticals, 1)
		requireRegexElement(t, check.Criticals, "Last ack .+ ago")
	})

	t.Run("Should be ok otherwise", func(t *testing.T) {
		check, ci := setup()
		last := time.Now()
		ci.AckFloor.Last = &last
		consumerCheckLastAck(ci, check, CheckConsumerHealthOptions{
			LastAckCritical: 1,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireLen(t, check.OKs, 1)
	})
}

func TestConsumer_checkLastDelivery(t *testing.T) {
	setup := func() (*Result, *api.ConsumerInfo) {
		return &Result{}, &api.ConsumerInfo{}
	}

	t.Run("Should skip without a threshold", func(t *testing.T) {
		check, ci := setup()
		consumerCheckLastDelivery(ci, check, CheckConsumerHealthOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle no delivery", func(t *testing.T) {
		check, ci := setup()
		consumerCheckLastDelivery(ci, check, CheckConsumerHealthOptions{
			LastDeliveryCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "No deliveries")
	})

	t.Run("Should handle time greater than or equal", func(t *testing.T) {
		check, ci := setup()
		last := time.Now().Add(-time.Hour)
		ci.Delivered.Last = &last

		consumerCheckLastDelivery(ci, check, CheckConsumerHealthOptions{
			LastDeliveryCritical: 1,
		}, api.NewDiscardLogger())

		requireLen(t, check.Criticals, 1)
		requireRegexElement(t, check.Criticals, "Last delivery .+ ago")
	})

	t.Run("Should be ok otherwise", func(t *testing.T) {
		check, ci := setup()
		last := time.Now()
		ci.Delivered.Last = &last

		consumerCheckLastDelivery(ci, check, CheckConsumerHealthOptions{
			LastDeliveryCritical: 1,
		}, api.NewDiscardLogger())

		requireEmpty(t, check.Criticals)
		requireLen(t, check.OKs, 1)
	})
}

func TestConsumer_checkUnprocessed(t *testing.T) {
	setup := func() (*Result, *api.ConsumerInfo) {
		return &Result{}, &api.ConsumerInfo{}
	}

	t.Run("Should skip without a threshold", func(t *testing.T) {
		check, ci := setup()
		consumerCheckUnprocessed(ci, check, CheckConsumerHealthOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle unprocessed above threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumPending = 10
		consumerCheckUnprocessed(ci, check, CheckConsumerHealthOptions{
			UnprocessedCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Unprocessed Messages: 10")

		check = &Result{}
		consumerCheckUnprocessed(ci, check, CheckConsumerHealthOptions{
			UnprocessedCritical: 10,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Unprocessed Messages: 10")
	})

	t.Run("Should handle below threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumPending = 10
		consumerCheckUnprocessed(ci, check, CheckConsumerHealthOptions{
			UnprocessedCritical: 11,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireElement(t, check.OKs, "Unprocessed Messages: 10")
	})
}

func TestConsumer_checkRedelivery(t *testing.T) {
	setup := func() (*Result, *api.ConsumerInfo) {
		return &Result{}, &api.ConsumerInfo{}
	}

	t.Run("Should skip without a threshold", func(t *testing.T) {
		check, ci := setup()
		consumerCheckRedelivery(ci, check, CheckConsumerHealthOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle redelivery above threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumRedelivered = 10
		consumerCheckRedelivery(ci, check, CheckConsumerHealthOptions{
			RedeliveryCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Redelivered: 10")

		check = &Result{}
		consumerCheckRedelivery(ci, check, CheckConsumerHealthOptions{
			RedeliveryCritical: 10,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Redelivered: 10")
	})

	t.Run("Should handle below threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumRedelivered = 10
		consumerCheckRedelivery(ci, check, CheckConsumerHealthOptions{
			RedeliveryCritical: 11,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireElement(t, check.OKs, "Redelivered: 10")
	})
}

func TestConsumer_checkWaiting(t *testing.T) {
	setup := func() (*Result, *api.ConsumerInfo) {
		return &Result{}, &api.ConsumerInfo{}
	}

	t.Run("Should skip without a threshold", func(t *testing.T) {
		check, ci := setup()
		consumerCheckWaiting(ci, check, CheckConsumerHealthOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle redelivery above threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumWaiting = 10
		consumerCheckWaiting(ci, check, CheckConsumerHealthOptions{
			WaitingCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Waiting Pulls: 10")

		check, ci = setup()
		ci.NumWaiting = 10
		consumerCheckWaiting(ci, check, CheckConsumerHealthOptions{
			WaitingCritical: 10,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Waiting Pulls: 10")
	})

	t.Run("Should handle below threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumWaiting = 10
		consumerCheckWaiting(ci, check, CheckConsumerHealthOptions{
			WaitingCritical: 11,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireElement(t, check.OKs, "Waiting Pulls: 10")
	})
}

func TestConsumer_checkOutstandingAck(t *testing.T) {
	setup := func() (*Result, *api.ConsumerInfo) {
		return &Result{}, &api.ConsumerInfo{}
	}

	t.Run("Should skip without a threshold", func(t *testing.T) {
		check, ci := setup()
		consumerCheckOutstandingAck(ci, check, CheckConsumerHealthOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle outstanding above threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumAckPending = 10
		consumerCheckOutstandingAck(ci, check, CheckConsumerHealthOptions{
			AckOutstandingCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Ack Pending: 10")

		check, ci = setup()
		ci.NumAckPending = 10
		consumerCheckOutstandingAck(ci, check, CheckConsumerHealthOptions{
			AckOutstandingCritical: 10,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Ack Pending: 10")
	})

	t.Run("Should handle below threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumAckPending = 10
		consumerCheckOutstandingAck(ci, check, CheckConsumerHealthOptions{
			AckOutstandingCritical: 11,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireElement(t, check.OKs, "Ack Pending: 10")
	})
}

func TestConsumer_consumerCheckPinned(t *testing.T) {
	setup := func() (*Result, *api.ConsumerInfo) {
		return &Result{}, &api.ConsumerInfo{}
	}

	t.Run("Should skip with pinning check disabled", func(t *testing.T) {
		check, ci := setup()
		consumerCheckPinned(ci, check, CheckConsumerHealthOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle non pinned client consumers", func(t *testing.T) {
		check, ci := setup()
		consumerCheckPinned(ci, check, CheckConsumerHealthOptions{Pinned: true}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Not pinned client priority mode")
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should detect invalid pinned counts", func(t *testing.T) {
		check, ci := setup()
		ci.Config.PriorityGroups = []string{"ONE", "TWO"}
		ci.Config.PriorityPolicy = api.PriorityPinnedClient
		ci.PriorityGroups = []api.PriorityGroupState{
			{Group: "ONE", PinnedClientID: "1"},
		}
		consumerCheckPinned(ci, check, CheckConsumerHealthOptions{Pinned: true}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "1 / 2 pinned clients")
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle healthy consumers", func(t *testing.T) {
		check, ci := setup()
		ci.Config.PriorityGroups = []string{"ONE", "TWO"}
		ci.Config.PriorityPolicy = api.PriorityPinnedClient
		ci.PriorityGroups = []api.PriorityGroupState{
			{Group: "ONE", PinnedClientID: "1"},
			{Group: "TWO", PinnedClientID: "2"},
		}
		consumerCheckPinned(ci, check, CheckConsumerHealthOptions{Pinned: true}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireElement(t, check.OKs, "2 pinned clients")
	})
}

func TestExtractConsumerHealthCheckOptions(t *testing.T) {
	t.Run("Should return empty opts for empty metadata", func(t *testing.T) {
		opts, err := ExtractConsumerHealthCheckOptions(map[string]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Enabled {
			t.Fatal("expected Enabled to be false")
		}
		if opts.AckOutstandingCritical != 0 {
			t.Fatalf("unexpected AckOutstandingCritical: %d", opts.AckOutstandingCritical)
		}
	})

	t.Run("Should parse all integer thresholds", func(t *testing.T) {
		opts, err := ExtractConsumerHealthCheckOptions(map[string]string{
			MonitorMetaEnabled:                        "true",
			ConsumerMonitorMetaOutstandingAckCritical: "5",
			ConsumerMonitorMetaWaitingCritical:        "10",
			ConsumerMonitorMetaUnprocessedCritical:    "20",
			ConsumerMonitorMetaRedeliveryCritical:     "3",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !opts.Enabled {
			t.Fatal("expected Enabled to be true")
		}
		if opts.AckOutstandingCritical != 5 {
			t.Fatalf("expected AckOutstandingCritical 5, got %d", opts.AckOutstandingCritical)
		}
		if opts.WaitingCritical != 10 {
			t.Fatalf("expected WaitingCritical 10, got %d", opts.WaitingCritical)
		}
		if opts.UnprocessedCritical != 20 {
			t.Fatalf("expected UnprocessedCritical 20, got %d", opts.UnprocessedCritical)
		}
		if opts.RedeliveryCritical != 3 {
			t.Fatalf("expected RedeliveryCritical 3, got %d", opts.RedeliveryCritical)
		}
	})

	t.Run("Should parse duration thresholds as seconds", func(t *testing.T) {
		opts, err := ExtractConsumerHealthCheckOptions(map[string]string{
			ConsumerMonitorMetaLastDeliveredCritical: "2m",
			ConsumerMonitorMetaLastAckCritical:       "30s",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.LastDeliveryCritical != 120 {
			t.Fatalf("expected LastDeliveryCritical 120, got %f", opts.LastDeliveryCritical)
		}
		if opts.LastAckCritical != 30 {
			t.Fatalf("expected LastAckCritical 30, got %f", opts.LastAckCritical)
		}
	})

	t.Run("Should parse pinned flag", func(t *testing.T) {
		opts, err := ExtractConsumerHealthCheckOptions(map[string]string{
			ConsumerMonitorMetaPinned: "true",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !opts.Pinned {
			t.Fatal("expected Pinned to be true")
		}
	})

	t.Run("Should propagate extra health check functions", func(t *testing.T) {
		called := false
		extra := ConsumerHealthCheckF(func(_ *jsm.Consumer, _ *Result, _ CheckConsumerHealthOptions, _ api.Logger) {
			called = true
		})
		opts, err := ExtractConsumerHealthCheckOptions(map[string]string{}, extra)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		requireLen(t, opts.HealthChecks, 1)
		_ = called
	})

	t.Run("Should return error on invalid integer", func(t *testing.T) {
		_, err := ExtractConsumerHealthCheckOptions(map[string]string{
			ConsumerMonitorMetaOutstandingAckCritical: "not-a-number",
		})
		if err == nil {
			t.Fatal("expected error for invalid integer")
		}
	})

	t.Run("Should return error on invalid duration", func(t *testing.T) {
		_, err := ExtractConsumerHealthCheckOptions(map[string]string{
			ConsumerMonitorMetaLastDeliveredCritical: "not-a-duration",
		})
		if err == nil {
			t.Fatal("expected error for invalid duration")
		}
	})
}

func TestCheckConsumerInfoHealth(t *testing.T) {
	setup := func() (*Result, *api.ConsumerInfo) {
		return &Result{}, &api.ConsumerInfo{}
	}

	t.Run("Should produce no output when all thresholds are zero", func(t *testing.T) {
		check, ci := setup()
		CheckConsumerInfoHealth(ci, check, CheckConsumerHealthOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should accumulate criticals from multiple checks", func(t *testing.T) {
		check, ci := setup()
		ci.NumAckPending = 10
		ci.NumRedelivered = 5
		CheckConsumerInfoHealth(ci, check, CheckConsumerHealthOptions{
			AckOutstandingCritical: 1,
			RedeliveryCritical:     1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Ack Pending: 10")
		requireElement(t, check.Criticals, "Redelivered: 5")
		requireEmpty(t, check.Warnings)
	})

	t.Run("Should accumulate OKs from multiple passing checks", func(t *testing.T) {
		check, ci := setup()
		ci.NumAckPending = 1
		ci.NumWaiting = 1
		ci.NumPending = 1
		ci.NumRedelivered = 1
		CheckConsumerInfoHealth(ci, check, CheckConsumerHealthOptions{
			AckOutstandingCritical: 10,
			WaitingCritical:        10,
			UnprocessedCritical:    10,
			RedeliveryCritical:     10,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireLen(t, check.OKs, 4)
	})

	t.Run("Should check pinned when configured", func(t *testing.T) {
		check, ci := setup()
		ci.Config.PriorityGroups = []string{"ONE"}
		ci.Config.PriorityPolicy = api.PriorityPinnedClient
		ci.PriorityGroups = []api.PriorityGroupState{
			{Group: "ONE", PinnedClientID: "client-1"},
		}
		CheckConsumerInfoHealth(ci, check, CheckConsumerHealthOptions{Pinned: true}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireElement(t, check.OKs, "1 pinned clients")
	})
}
