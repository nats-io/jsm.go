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
	"testing"
	"time"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/monitor"
)

func TestConsumer_checkLastAck(t *testing.T) {
	setup := func() (*Consumer, *monitor.Result, *api.ConsumerInfo) {
		return &Consumer{}, &monitor.Result{}, &api.ConsumerInfo{}
	}

	t.Run("Should skip without a threshold", func(t *testing.T) {
		c, check, ci := setup()
		c.checkLastAck(ci, check, ConsumerHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle no ack floor", func(t *testing.T) {
		c, check, ci := setup()
		c.checkLastAck(ci, check, ConsumerHealthCheckOptions{
			LastAckCritical: time.Second,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "No acks")
	})

	t.Run("Should handle time greater than or equal", func(t *testing.T) {
		c, check, ci := setup()
		last := time.Now().Add(-time.Hour)
		ci.AckFloor.Last = &last
		c.checkLastAck(ci, check, ConsumerHealthCheckOptions{
			LastAckCritical: time.Second,
		}, api.NewDiscardLogger())
		requireLen(t, check.Criticals, 1)
		requireRegexElement(t, check.Criticals, "Last ack .+ ago")
	})

	t.Run("Should be ok otherwise", func(t *testing.T) {
		c, check, ci := setup()
		last := time.Now()
		ci.AckFloor.Last = &last
		c.checkLastAck(ci, check, ConsumerHealthCheckOptions{
			LastAckCritical: time.Second,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireLen(t, check.OKs, 1)
	})
}

func TestConsumer_checkLastDelivery(t *testing.T) {
	setup := func() (*Consumer, *monitor.Result, *api.ConsumerInfo) {
		return &Consumer{}, &monitor.Result{}, &api.ConsumerInfo{}
	}

	t.Run("Should skip without a threshold", func(t *testing.T) {
		c, check, ci := setup()
		c.checkLastDelivery(ci, check, ConsumerHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle no delivery", func(t *testing.T) {
		c, check, ci := setup()
		c.checkLastDelivery(ci, check, ConsumerHealthCheckOptions{
			LastDeliveryCritical: time.Second,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "No deliveries")
	})

	t.Run("Should handle time greater than or equal", func(t *testing.T) {
		c, check, ci := setup()
		last := time.Now().Add(-time.Hour)
		ci.Delivered.Last = &last

		c.checkLastDelivery(ci, check, ConsumerHealthCheckOptions{
			LastDeliveryCritical: time.Second,
		}, api.NewDiscardLogger())

		requireLen(t, check.Criticals, 1)
		requireRegexElement(t, check.Criticals, "Last delivery .+ ago")
	})

	t.Run("Should handle time greater than or equal", func(t *testing.T) {
		c, check, ci := setup()
		last := time.Now().Add(-time.Hour)
		ci.Delivered.Last = &last

		c.checkLastDelivery(ci, check, ConsumerHealthCheckOptions{
			LastDeliveryCritical: time.Second,
		}, api.NewDiscardLogger())

		requireLen(t, check.Criticals, 1)
		requireRegexElement(t, check.Criticals, "Last delivery .+ ago")
	})

	t.Run("Should be ok otherwise", func(t *testing.T) {
		c, check, ci := setup()
		last := time.Now()
		ci.Delivered.Last = &last

		c.checkLastDelivery(ci, check, ConsumerHealthCheckOptions{
			LastDeliveryCritical: time.Second,
		}, api.NewDiscardLogger())

		requireEmpty(t, check.Criticals)
		requireLen(t, check.OKs, 1)
	})
}

func TestConsumer_checkUnprocessed(t *testing.T) {
	setup := func() (*Consumer, *monitor.Result, *api.ConsumerInfo) {
		return &Consumer{}, &monitor.Result{}, &api.ConsumerInfo{}
	}

	t.Run("Should skip without a threshold", func(t *testing.T) {
		c, check, ci := setup()
		c.checkRedelivery(ci, check, ConsumerHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle redelivery above threshold", func(t *testing.T) {
		c, check, ci := setup()
		ci.NumRedelivered = 10
		c.checkRedelivery(ci, check, ConsumerHealthCheckOptions{
			RedeliveryCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Redelivered: 10")

		check = &monitor.Result{}
		c.checkRedelivery(ci, check, ConsumerHealthCheckOptions{
			RedeliveryCritical: 10,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Redelivered: 10")
	})

	t.Run("Should handle below threshold", func(t *testing.T) {
		c, check, ci := setup()
		ci.NumRedelivered = 10
		c.checkRedelivery(ci, check, ConsumerHealthCheckOptions{
			RedeliveryCritical: 11,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireElement(t, check.OKs, "Redelivered: 10")
	})
}

func TestConsumer_checkWaiting(t *testing.T) {
	setup := func() (*Consumer, *monitor.Result, *api.ConsumerInfo) {
		return &Consumer{}, &monitor.Result{}, &api.ConsumerInfo{}
	}

	t.Run("Should skip without a threshold", func(t *testing.T) {
		c, check, ci := setup()
		c.checkWaiting(ci, check, ConsumerHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle redelivery above threshold", func(t *testing.T) {
		c, check, ci := setup()
		ci.NumWaiting = 10
		c.checkWaiting(ci, check, ConsumerHealthCheckOptions{
			WaitingCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Waiting Pulls: 10")

		c, check, ci = setup()
		ci.NumWaiting = 10
		c.checkWaiting(ci, check, ConsumerHealthCheckOptions{
			WaitingCritical: 10,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Waiting Pulls: 10")
	})

	t.Run("Should handle below threshold", func(t *testing.T) {
		c, check, ci := setup()
		ci.NumWaiting = 10
		c.checkWaiting(ci, check, ConsumerHealthCheckOptions{
			WaitingCritical: 11,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireElement(t, check.OKs, "Waiting Pulls: 10")
	})
}

func TestConsumer_checkOutstandingAck(t *testing.T) {
	setup := func() (*Consumer, *monitor.Result, *api.ConsumerInfo) {
		return &Consumer{}, &monitor.Result{}, &api.ConsumerInfo{}
	}

	t.Run("Should skip without a threshold", func(t *testing.T) {
		c, check, ci := setup()
		c.checkOutstandingAck(ci, check, ConsumerHealthCheckOptions{}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireEmpty(t, check.Warnings)
		requireEmpty(t, check.OKs)
	})

	t.Run("Should handle outstanding above threshold", func(t *testing.T) {
		c, check, ci := setup()
		ci.NumAckPending = 10
		c.checkOutstandingAck(ci, check, ConsumerHealthCheckOptions{
			AckOutstandingCritical: 1,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Ack Pending: 10")

		c, check, ci = setup()
		ci.NumAckPending = 10
		c.checkOutstandingAck(ci, check, ConsumerHealthCheckOptions{
			AckOutstandingCritical: 10,
		}, api.NewDiscardLogger())
		requireElement(t, check.Criticals, "Ack Pending: 10")
	})

	t.Run("Should handle below threshold", func(t *testing.T) {
		c, check, ci := setup()
		ci.NumAckPending = 10
		c.checkOutstandingAck(ci, check, ConsumerHealthCheckOptions{
			AckOutstandingCritical: 11,
		}, api.NewDiscardLogger())
		requireEmpty(t, check.Criticals)
		requireElement(t, check.OKs, "Ack Pending: 10")
	})
}
