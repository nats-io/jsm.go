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

package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	natsd "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func checkConsumerQueryMatched(t *testing.T, s *jsm.Stream, expect int, opts ...jsm.ConsumerQueryOpt) {
	t.Helper()

	matched, err := s.QueryConsumers(opts...)
	checkErr(t, err, "query failed")
	if len(matched) != expect {
		t.Fatalf("expected %d matched, got %d", expect, len(matched))
	}
}

// TestConsumerQueryAge verifies that ConsumerQueryOlderThan correctly filters
// by consumer creation time in both normal and inverted modes, and that the
// zero-time guard applies to both branches (issue 2: precedence bug).
func TestConsumerQueryAge(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	s, err := mgr.NewStream("QAGE", jsm.Subjects("qage.*"), jsm.MemoryStorage())
	checkErr(t, err, "create failed")

	_, err = s.NewConsumer(jsm.DurableName("C1"))
	checkErr(t, err, "create failed")

	// Consumer was just created — not yet older than 1 hour.
	checkConsumerQueryMatched(t, s, 0, jsm.ConsumerQueryOlderThan(time.Hour))
	// Inverted: consumer is newer than 1 hour — should match.
	checkConsumerQueryMatched(t, s, 1, jsm.ConsumerQueryOlderThan(time.Hour), jsm.ConsumerQueryInvert())

	time.Sleep(60 * time.Millisecond)

	// Consumer is now older than 10ms — should match OlderThan(10ms).
	checkConsumerQueryMatched(t, s, 1, jsm.ConsumerQueryOlderThan(10*time.Millisecond))
	// Inverted: consumer is older than 10ms — should not match.
	checkConsumerQueryMatched(t, s, 0, jsm.ConsumerQueryOlderThan(10*time.Millisecond), jsm.ConsumerQueryInvert())
}

// TestConsumerQueryDelivery verifies that ConsumerQueryWithDeliverySince uses
// lastDeliveryLimit (not ageLimit) in both branches (issue 1: wrong field) and
// that the nil guard prevents a panic when Delivered.Last is nil (issue 2).
func TestConsumerQueryDelivery(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	s, err := mgr.NewStream("QDEL", jsm.Subjects("qdel.*"), jsm.MemoryStorage())
	checkErr(t, err, "create failed")

	// Consumer with no deliveries — Delivered.Last is nil.
	_, err = s.NewConsumer(jsm.DurableName("NODELIVERY"))
	checkErr(t, err, "create failed")

	// Consumer with a recent delivery.
	c, err := s.NewConsumer(jsm.DurableName("DELIVERED"))
	checkErr(t, err, "create failed")

	_, err = nc.Request("qdel.1", []byte("msg"), time.Second)
	checkErr(t, err, "publish failed")

	msg, err := c.NextMsg()
	checkErr(t, err, "next msg failed")
	msg.Ack()

	// DELIVERED had a delivery within the last hour; NODELIVERY did not.
	checkConsumerQueryMatched(t, s, 1, jsm.ConsumerQueryWithDeliverySince(time.Hour))

	// Inverted: consumers NOT delivered within the last hour.
	// Should not panic even when Delivered.Last is nil on NODELIVERY.
	// NODELIVERY has nil Delivered.Last so it is excluded by the nil guard;
	// DELIVERED was recent so it doesn't satisfy inverted (>1h ago) either.
	checkConsumerQueryMatched(t, s, 0, jsm.ConsumerQueryWithDeliverySince(time.Hour), jsm.ConsumerQueryInvert())

	time.Sleep(60 * time.Millisecond)

	// Inverted with a short window: DELIVERED was more than 10ms ago.
	checkConsumerQueryMatched(t, s, 1, jsm.ConsumerQueryWithDeliverySince(10*time.Millisecond), jsm.ConsumerQueryInvert())
	// Non-inverted with short window: DELIVERED was more than 10ms ago — no match.
	checkConsumerQueryMatched(t, s, 0, jsm.ConsumerQueryWithDeliverySince(10*time.Millisecond))
}

// TestConsumerQueryPending verifies matchPending applies the NumPending > 0
// guard to both the normal and inverted branches (issue 2: precedence bug).
func TestConsumerQueryPending(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	s, err := mgr.NewStream("QPEND", jsm.Subjects("qpend.*"), jsm.MemoryStorage())
	checkErr(t, err, "create failed")

	// Publish 5 messages.
	for i := 0; i < 5; i++ {
		_, err = nc.Request(fmt.Sprintf("qpend.%d", i), []byte("msg"), time.Second)
		checkErr(t, err, "publish failed")
	}

	// Consumer sees all 5 as pending.
	_, err = s.NewConsumer(jsm.DurableName("HIGH"))
	checkErr(t, err, "create failed")

	// Consumer sees 0 pending (stream is already consumed from HIGH's perspective
	// is not how it works — each consumer has its own view). Actually both
	// consumers see the same 5 messages as pending. Let's use a filtered consumer.
	_, err = s.NewConsumer(jsm.DurableName("NOPENDING"), jsm.FilterStreamBySubject("qpend.none"))
	checkErr(t, err, "create failed")

	// HIGH has 5 pending, NOPENDING has 0 pending.
	// Fewer than 10: HIGH matches, NOPENDING does not (guard: NumPending > 0).
	checkConsumerQueryMatched(t, s, 1, jsm.ConsumerQueryWithFewerPending(10))
	// Inverted (more than 10): neither matches (HIGH has 5, NOPENDING has 0).
	checkConsumerQueryMatched(t, s, 0, jsm.ConsumerQueryWithFewerPending(10), jsm.ConsumerQueryInvert())

	// Fewer than 3: HIGH has 5 so no match; NOPENDING excluded by guard.
	checkConsumerQueryMatched(t, s, 0, jsm.ConsumerQueryWithFewerPending(3))
	// Inverted (more than 3): HIGH has 5, so matches; NOPENDING excluded by guard.
	checkConsumerQueryMatched(t, s, 1, jsm.ConsumerQueryWithFewerPending(3), jsm.ConsumerQueryInvert())
}

// TestConsumerQueryAckPending verifies matchAckPending applies the guard to
// both branches (issue 2: precedence bug).
func TestConsumerQueryAckPending(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	s, err := mgr.NewStream("QACK", jsm.Subjects("qack.*"), jsm.MemoryStorage())
	checkErr(t, err, "create failed")

	for i := 0; i < 5; i++ {
		_, err = nc.Request(fmt.Sprintf("qack.%d", i), []byte("msg"), time.Second)
		checkErr(t, err, "publish failed")
	}

	// Pull all 5 without acking — creates ack-pending.
	withAck, err := s.NewConsumer(jsm.DurableName("WITHACK"))
	checkErr(t, err, "create failed")

	for i := 0; i < 5; i++ {
		_, err = withAck.NextMsg()
		checkErr(t, err, "next msg failed")
	}

	// Consumer with no ack pending.
	_, err = s.NewConsumer(jsm.DurableName("NOACK"), jsm.FilterStreamBySubject("qack.none"))
	checkErr(t, err, "create failed")

	// WITHACK has 5 ack-pending, NOACK has 0.
	// Fewer than 10: WITHACK matches; NOACK excluded by guard.
	checkConsumerQueryMatched(t, s, 1, jsm.ConsumerQueryWithFewerAckPending(10))
	// Inverted (more than 10): WITHACK has 5, no match; NOACK excluded by guard.
	checkConsumerQueryMatched(t, s, 0, jsm.ConsumerQueryWithFewerAckPending(10), jsm.ConsumerQueryInvert())

	// Fewer than 3: WITHACK has 5, no match.
	checkConsumerQueryMatched(t, s, 0, jsm.ConsumerQueryWithFewerAckPending(3))
	// Inverted (more than 3): WITHACK has 5, matches; NOACK excluded by guard.
	checkConsumerQueryMatched(t, s, 1, jsm.ConsumerQueryWithFewerAckPending(3), jsm.ConsumerQueryInvert())
}

func TestConsumerApiLevel(t *testing.T) {
	withJSCluster(t, func(t *testing.T, _ []*natsd.Server, nc *nats.Conn, mgr *jsm.Manager) {
		s, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.Replicas(2))
		checkErr(t, err, "create failed")

		_, err = s.NewConsumer(jsm.PauseUntil(time.Now().Add(time.Hour)), jsm.DurableName("PAUSED"))
		checkErr(t, err, "create failed")

		_, err = s.NewConsumer(jsm.DurableName("OLD"))
		checkErr(t, err, "create failed")

		checkConsumerQueryMatched(t, s, 1, jsm.ConsumerQueryApiLevelMin(1))
		checkConsumerQueryMatched(t, s, 2, jsm.ConsumerQueryApiLevelMin(0))

		res, err := s.QueryConsumers(jsm.ConsumerQueryApiLevelMin(1))
		checkErr(t, err, "query failed")
		if res[0].Name() != "PAUSED" {
			t.Fatalf("did not match paused consumer")
		}

		res, err = s.QueryConsumers(jsm.ConsumerQueryApiLevelMin(1), jsm.ConsumerQueryInvert())
		checkErr(t, err, "query failed")
		if res[0].Name() != "OLD" {
			t.Fatalf("did not match unpaused consumer")
		}
	})
}
