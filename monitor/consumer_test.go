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
		consumerCheckLastAck(ci, check, ConsumerHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle no ack floor", func(t *testing.T) {
		check, ci := setup()
		consumerCheckLastAck(ci, check, ConsumerHealthCheckOptions{
			LastAckCritical: time.Second,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "No acks")
	})

	t.Run("Should handle time greater than or equal", func(t *testing.T) {
		check, ci := setup()
		last := time.Now().Add(-time.Hour)
		ci.AckFloor.Last = &last
		consumerCheckLastAck(ci, check, ConsumerHealthCheckOptions{
			LastAckCritical: time.Second,
		}, api.NewDiscardLogger())
		requireLen(t, check.Criticals, 1)
		requireRegexElement(t, check.Criticals, "Last ack .+ ago")
	})

	t.Run("Should be ok otherwise", func(t *testing.T) {
		check, ci := setup()
		last := time.Now()
		ci.AckFloor.Last = &last
		consumerCheckLastAck(ci, check, ConsumerHealthCheckOptions{
			LastAckCritical: time.Second,
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
		consumerCheckLastDelivery(ci, check, ConsumerHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle no delivery", func(t *testing.T) {
		check, ci := setup()
		consumerCheckLastDelivery(ci, check, ConsumerHealthCheckOptions{
			LastDeliveryCritical: time.Second,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "No deliveries")
	})

	t.Run("Should handle time greater than or equal", func(t *testing.T) {
		check, ci := setup()
		last := time.Now().Add(-time.Hour)
		ci.Delivered.Last = &last

		consumerCheckLastDelivery(ci, check, ConsumerHealthCheckOptions{
			LastDeliveryCritical: time.Second,
		}, api.NewDiscardLogger())

		requireLen(t, check.Criticals, 1)
		requireRegexElement(t, check.Criticals, "Last delivery .+ ago")
	})

	t.Run("Should handle time greater than or equal", func(t *testing.T) {
		check, ci := setup()
		last := time.Now().Add(-time.Hour)
		ci.Delivered.Last = &last

		consumerCheckLastDelivery(ci, check, ConsumerHealthCheckOptions{
			LastDeliveryCritical: time.Second,
		}, api.NewDiscardLogger())

		requireLen(t, check.Criticals, 1)
		requireRegexElement(t, check.Criticals, "Last delivery .+ ago")
	})

	t.Run("Should be ok otherwise", func(t *testing.T) {
		check, ci := setup()
		last := time.Now()
		ci.Delivered.Last = &last

		consumerCheckLastDelivery(ci, check, ConsumerHealthCheckOptions{
			LastDeliveryCritical: time.Second,
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
		consumerCheckRedelivery(ci, check, ConsumerHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle redelivery above threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumRedelivered = 10
		consumerCheckRedelivery(ci, check, ConsumerHealthCheckOptions{
			RedeliveryCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Redelivered: 10")

		check = &Result{}
		consumerCheckRedelivery(ci, check, ConsumerHealthCheckOptions{
			RedeliveryCritical: 10,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Redelivered: 10")
	})

	t.Run("Should handle below threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumRedelivered = 10
		consumerCheckRedelivery(ci, check, ConsumerHealthCheckOptions{
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
		consumerCheckWaiting(ci, check, ConsumerHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle redelivery above threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumWaiting = 10
		consumerCheckWaiting(ci, check, ConsumerHealthCheckOptions{
			WaitingCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Waiting Pulls: 10")

		check, ci = setup()
		ci.NumWaiting = 10
		consumerCheckWaiting(ci, check, ConsumerHealthCheckOptions{
			WaitingCritical: 10,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Waiting Pulls: 10")
	})

	t.Run("Should handle below threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumWaiting = 10
		consumerCheckWaiting(ci, check, ConsumerHealthCheckOptions{
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
		consumerCheckOutstandingAck(ci, check, ConsumerHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle outstanding above threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumAckPending = 10
		consumerCheckOutstandingAck(ci, check, ConsumerHealthCheckOptions{
			AckOutstandingCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Ack Pending: 10")

		check, ci = setup()
		ci.NumAckPending = 10
		consumerCheckOutstandingAck(ci, check, ConsumerHealthCheckOptions{
			AckOutstandingCritical: 10,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Ack Pending: 10")
	})

	t.Run("Should handle below threshold", func(t *testing.T) {
		check, ci := setup()
		ci.NumAckPending = 10
		consumerCheckOutstandingAck(ci, check, ConsumerHealthCheckOptions{
			AckOutstandingCritical: 11,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireElement(t, check.OKs, "Ack Pending: 10")
	})
}
