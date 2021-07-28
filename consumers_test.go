// Copyright 2020 The NATS Authors
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

package jsm_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"

	"github.com/nats-io/jsm.go/api"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go"
)

func setupConsumerTest(t *testing.T) (*server.Server, *nats.Conn, *jsm.Stream, *jsm.Manager) {
	t.Helper()
	srv, nc, mgr := startJSServer(t)
	stream, err := mgr.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.FileStorage(), jsm.MaxAge(time.Hour), jsm.Subjects("ORDERS.>"))
	checkErr(t, err, "create failed")

	_, err = nc.Request("ORDERS.new", []byte("order 1"), time.Second)
	checkErr(t, err, "publish failed")

	return srv, nc, stream, mgr

}

func TestConsumer_DeliveryPolicyConsistency(t *testing.T) {
	c, err := jsm.NewConsumerConfiguration(jsm.DefaultConsumer)
	checkErr(t, err, "create failed")

	checkPolicy := func(c *api.ConsumerConfig, sseq uint64, stime *time.Time, policy api.DeliverPolicy) {
		t.Helper()

		if c.OptStartSeq != sseq {
			t.Fatalf("Stream expected %d got %d", sseq, c.OptStartSeq)
		}

		if c.OptStartTime != nil && stime != nil {
			if c.OptStartTime.UnixNano() != stime.UnixNano() {
				t.Fatalf("StartTime expected %v got %v", stime, c.OptStartTime)
			}
		} else if c.OptStartTime != nil || stime != nil {
			t.Fatalf("expected StartTime to be nil")
		}

		if c.DeliverPolicy != policy {
			t.Fatalf("DeliverPolicy expected %v got %v", policy, c.DeliverPolicy)
		}
	}

	checkPolicy(c, 0, nil, api.DeliverAll)

	jsm.StartAtSequence(10)(c)
	checkPolicy(c, 10, nil, api.DeliverByStartSequence)

	now := time.Now()
	jsm.StartAtTime(now)(c)
	checkPolicy(c, 0, &now, api.DeliverByStartTime)

	jsm.DeliverAllAvailable()(c)
	checkPolicy(c, 0, nil, api.DeliverAll)

	jsm.StartWithLastReceived()(c)
	checkPolicy(c, 0, nil, api.DeliverLast)

	jsm.StartWithNextReceived()(c)
	checkPolicy(c, 0, nil, api.DeliverNew)
}

func TestNextMsg(t *testing.T) {
	srv, nc, stream, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream.Purge()

	consumer, err := stream.NewConsumer(jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"), jsm.DeliverAllAvailable())
	checkErr(t, err, "create failed")

	for i := 0; i <= 100; i++ {
		nc.Publish("ORDERS.new", []byte(fmt.Sprintf("%d", i)))
	}

	for i := 0; i <= 100; i++ {
		msg, err := consumer.NextMsg()
		checkErr(t, err, "NextMsg failed")

		b, err := strconv.Atoi(string(msg.Data))
		checkErr(t, err, fmt.Sprintf("invalid body: %q", string(msg.Data)))

		if b != i {
			t.Fatalf("got message %d expected %d", b, i)
		}

		msg.Ack()
	}
}

func TestNextMsgRequest(t *testing.T) {
	srv, nc, stream, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream.Purge()

	consumer, err := stream.NewConsumer(jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"), jsm.DeliverAllAvailable())
	checkErr(t, err, "create failed")

	for i := 0; i <= 100; i++ {
		_, err = nc.Request("ORDERS.new", []byte(fmt.Sprintf("%d", i)), time.Second)
		checkErr(t, err, "publish failed")
	}

	sub, err := nc.SubscribeSync(nats.NewInbox())
	checkErr(t, err, "subscribe failed")
	defer sub.Unsubscribe()

	consumer.NextMsgRequest(sub.Subject, &api.JSApiConsumerGetNextRequest{Batch: 100})
	for i := 0; i < 100; i++ {
		msg, err := sub.NextMsg(time.Second)
		checkErr(t, err, fmt.Sprintf("NextMsg %d failed", i))
		b, err := strconv.Atoi(string(msg.Data))
		checkErr(t, err, fmt.Sprintf("invalid body: %q", string(msg.Data)))

		if b != i {
			t.Fatalf("got message %d expected %d", b, i)
		}

		msg.Ack()
	}
}

func TestNewConsumer(t *testing.T) {
	srv, nc, stream, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	consumer, err := stream.NewConsumer(jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	consumer.Reset()
	if consumer.AckPolicy() != api.AckExplicit {
		t.Fatalf("expected explicit ack got %s", consumer.AckPolicy())
	}

	if consumer.Name() != "NEW" {
		t.Fatalf("expected NEW got %s", consumer.Name())
	}
}

func TestNewConsumerFromDefaultDurable(t *testing.T) {
	srv, nc, stream, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	consumer, err := stream.NewConsumerFromDefault(jsm.SampledDefaultConsumer, jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	consumer.Reset()
	if consumer.AckPolicy() != api.AckExplicit {
		t.Fatalf("expected explicit ack got %s", consumer.AckPolicy())
	}

	if consumer.Name() != "NEW" {
		t.Fatalf("expected NEW got %s", consumer.Name())
	}

	if !consumer.IsSampled() {
		t.Fatal("expected a sampled consumer")
	}
}

func TestNewConsumerFromDefaultEphemeral(t *testing.T) {
	srv, nc, stream, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	// interest is needed
	nc.Subscribe("out", func(_ *nats.Msg) {})

	consumer, err := stream.NewConsumerFromDefault(jsm.SampledDefaultConsumer, jsm.DeliverySubject("out"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	consumers, err := mgr.ConsumerNames("ORDERS")
	checkErr(t, err, "consumer list failed")
	if len(consumers) != 1 {
		t.Fatalf("expected 1 consumer got %v", consumers)
	}

	if consumer.Name() != consumers[0] {
		t.Fatalf("incorrect consumer name '%s' expected '%s'", consumer.Name(), consumers[0])
	}

	if consumer.IsDurable() {
		t.Fatalf("expected ephemeral consumer got durable")
	}
}

func TestLoadConsumer(t *testing.T) {
	srv, nc, stream, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	_, err := stream.NewConsumerFromDefault(jsm.SampledDefaultConsumer, jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	consumer, err := mgr.LoadConsumer("ORDERS", "NEW")
	checkErr(t, err, "load failed")

	if consumer.AckPolicy() != api.AckExplicit {
		t.Fatalf("expected explicit ack got %s", consumer.AckPolicy())
	}

	if consumer.Name() != "NEW" {
		t.Fatalf("expected NEW got %s", consumer.Name())
	}

	if !consumer.IsSampled() {
		t.Fatal("expected a sampled consumer")
	}
}

func TestLoadOrNewConsumer(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	_, err := mgr.LoadOrNewConsumer("ORDERS", "NEW", jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	consumer, err := mgr.LoadOrNewConsumer("ORDERS", "NEW", jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "load failed")

	if consumer.AckPolicy() != api.AckExplicit {
		t.Fatalf("expected explicit ack got %s", consumer.AckPolicy())
	}

	if consumer.Name() != "NEW" {
		t.Fatalf("expected NEW got %s", consumer.Name())
	}
}

func TestLoadOrNewConsumerFromDefault(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	_, err := mgr.LoadOrNewConsumerFromDefault("ORDERS", "NEW", jsm.SampledDefaultConsumer, jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	consumer, err := mgr.LoadOrNewConsumerFromDefault("ORDERS", "NEW", jsm.SampledDefaultConsumer, jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "load failed")

	if consumer.AckPolicy() != api.AckExplicit {
		t.Fatalf("expected explicit ack got %s", consumer.AckPolicy())
	}

	if consumer.Name() != "NEW" {
		t.Fatalf("expected NEW got %s", consumer.Name())
	}

	if !consumer.IsSampled() {
		t.Fatal("expected a sampled consumer")
	}
}

func TestConsumer_Reset(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	consumer, err := mgr.NewConsumer("ORDERS", jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	err = consumer.Delete()
	checkErr(t, err, "delete failed")

	consumer, err = mgr.NewConsumer("ORDERS", jsm.DurableName("NEW"))
	checkErr(t, err, "create failed")

	err = consumer.Reset()
	checkErr(t, err, "reset failed")

	if consumer.FilterSubject() != "" {
		t.Fatalf("expected no filter got %v", consumer.FilterSubject())
	}
}

func TestNextSubject(t *testing.T) {
	_, err := jsm.NextSubject("", "x")
	if err == nil {
		t.Fatalf("empty stream was accepted")
	}

	_, err = jsm.NextSubject("x", "")
	if err == nil {
		t.Fatalf("empty consumer was accepted")
	}

	s, err := jsm.NextSubject("str", "cons")
	checkErr(t, err, "good subject params failed")

	if s != "$JS.API.CONSUMER.MSG.NEXT.str.cons" {
		t.Fatalf("invalid next subject %q", s)
	}
}

func TestConsumer_NextSubject(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	consumer, err := mgr.NewConsumer("ORDERS", jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	if consumer.NextSubject() != "$JS.API.CONSUMER.MSG.NEXT.ORDERS.NEW" {
		t.Fatalf("expected next subject got %s", consumer.NextSubject())
	}
}

func TestConsumer_SampleSubject(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	consumer, err := mgr.NewConsumerFromDefault("ORDERS", jsm.SampledDefaultConsumer, jsm.DurableName("NEW"))
	checkErr(t, err, "create failed")

	if consumer.AckSampleSubject() != "$JS.EVENT.METRIC.CONSUMER.ACK.ORDERS.NEW" {
		t.Fatalf("expected next subject got %s", consumer.AckSampleSubject())
	}

	unsampled, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("UNSAMPLED"))
	checkErr(t, err, "create failed")

	if unsampled.AckSampleSubject() != "" {
		t.Fatalf("expected empty next subject got %s", consumer.AckSampleSubject())
	}
}

func TestConsumer_DeliveredState(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	durable, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("D"))
	checkErr(t, err, "create failed")

	state, err := durable.DeliveredState()
	checkErr(t, err, "state failed")

	if state.Stream != 0 {
		t.Fatalf("expected stream seq 0 got %d", state.Stream)
	}

	m, err := durable.NextMsg()
	checkErr(t, err, "next failed")
	err = m.Respond(nil)
	checkErr(t, err, "ack failed")

	state, err = durable.DeliveredState()
	checkErr(t, err, "state failed")

	if state.Stream != 1 {
		t.Fatalf("expected stream seq 1 got %d", state.Stream)
	}
}

func TestConsumer_PendingMessageCount(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	durable, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("D"))
	checkErr(t, err, "create failed")

	m, err := durable.NextMsg()
	checkErr(t, err, "next failed")
	pending, err := durable.PendingAcknowledgement()
	checkErr(t, err, "state failed")
	if pending != 1 {
		t.Fatalf("expected pending 1 got %d", pending)
	}
	m.Respond(nil)

	pending, err = durable.PendingAcknowledgement()
	checkErr(t, err, "state failed")
	if pending != 0 {
		t.Fatalf("expected pending 0 got %d", pending)
	}
}

func TestConsumer_RedeliveryCount(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	durable, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("D"), jsm.AckWait(500*time.Millisecond))
	checkErr(t, err, "create failed")

	_, err = durable.NextMsg()
	checkErr(t, err, "next failed")

	redel, err := durable.RedeliveryCount()
	checkErr(t, err, "state failed")
	if redel != 0 {
		t.Fatal("expected 0 redeliveries got %i", redel)
	}

	time.Sleep(500 * time.Millisecond)

	_, err = durable.NextMsg()
	checkErr(t, err, "next failed")

	redel, err = durable.RedeliveryCount()
	checkErr(t, err, "state failed")
	if redel != 1 {
		t.Fatal("expected 1 redliveries got %i", redel)
	}
}

func TestConsumer_PendingMessages(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	durable, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("D"), jsm.AckWait(500*time.Millisecond))
	checkErr(t, err, "create failed")

	pending, err := durable.PendingMessages()
	checkErr(t, err, "pending failed")
	if pending != 1 {
		t.Fatal("expected 1 pending got %i", pending)
	}

	m, err := durable.NextMsg()
	checkErr(t, err, "next failed")
	m.Ack()

	pending, err = durable.PendingMessages()
	checkErr(t, err, "pending failed")
	if pending != 0 {
		t.Fatal("expected 0 pending got %i", pending)
	}
}

func TestConsumer_AcknowledgedState(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	durable, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("D"))
	checkErr(t, err, "create failed")

	state, err := durable.AcknowledgedFloor()
	checkErr(t, err, "state failed")

	if state.Stream != 0 {
		t.Fatalf("expected stream seq 0 got %d", state.Stream)
	}

	m, err := durable.NextMsg()
	checkErr(t, err, "next failed")
	err = m.Respond(nil)
	checkErr(t, err, "ack failed")

	state, err = durable.AcknowledgedFloor()
	checkErr(t, err, "state failed")

	if state.Consumer != 1 {
		t.Fatalf("expected set seq 1 got %d", state.Consumer)
	}
}

func TestConsumer_Configuration(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	durable, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("D"))
	checkErr(t, err, "create failed")

	if durable.Configuration().Durable != "D" {
		t.Fatalf("got wrong config: %+v", durable.Configuration())
	}
}

func TestConsumer_Delete(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	durable, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("D"))
	checkErr(t, err, "create failed")
	if !durable.IsDurable() {
		t.Fatalf("expected durable, got %s", durable.DurableName())
	}

	err = durable.Delete()
	checkErr(t, err, "delete failed")

	names, err := mgr.ConsumerNames("ORDERS")
	checkErr(t, err, "names failed")

	if len(names) != 0 {
		t.Fatalf("expected [] got %v", names)
	}
}

func TestConsumer_IsDurable(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	durable, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("D"))
	checkErr(t, err, "create failed")
	if !durable.IsDurable() {
		t.Fatalf("expected durable, got %s", durable.DurableName())
	}
	durable.Delete()

	// interest is needed before creating a ephemeral push
	nc.Subscribe("out", func(_ *nats.Msg) {})

	_, err = mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DeliverySubject("out"))
	checkErr(t, err, "create failed")

	names, err := mgr.ConsumerNames("ORDERS")
	checkErr(t, err, "names failed")

	if len(names) == 0 {
		t.Fatal("got no consumers")
	}

	eph, err := mgr.LoadConsumer("ORDERS", names[0])
	checkErr(t, err, "load failed")
	if eph.IsDurable() {
		t.Fatalf("expected ephemeral got %q %q", eph.Name(), eph.DurableName())
	}
}

func TestConsumer_IsPullMode(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	push, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("PUSH"), jsm.DeliverySubject("out"))
	checkErr(t, err, "create failed")
	if push.IsPullMode() {
		t.Fatalf("expected push, got %s", push.DeliverySubject())
	}

	pull, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("PULL"))
	checkErr(t, err, "create failed")
	if !pull.IsPullMode() {
		t.Fatalf("expected pull, got %s", pull.DeliverySubject())
	}
}

func TestConsumer_IsPushMode(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	push, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("PUSH"), jsm.DeliverySubject("out"))
	checkErr(t, err, "create failed")
	if !push.IsPushMode() {
		t.Fatalf("expected push, got %s", push.DeliverySubject())
	}

	pull, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("PULL"))
	checkErr(t, err, "create failed")
	if pull.IsPushMode() {
		t.Fatalf("expected pull, got %s", pull.DeliverySubject())
	}
}

func TestConsumer_IsSampled(t *testing.T) {
	srv, nc, _, mgr := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	sampled, err := mgr.NewConsumerFromDefault("ORDERS", jsm.SampledDefaultConsumer, jsm.DurableName("SAMPLED"))
	checkErr(t, err, "create failed")
	if !sampled.IsSampled() {
		t.Fatalf("expected sampled, got %s", sampled.SampleFrequency())
	}

	unsampled, err := mgr.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("UNSAMPLED"))
	checkErr(t, err, "create failed")
	if unsampled.IsSampled() {
		t.Fatalf("expected un-sampled, got %s", unsampled.SampleFrequency())
	}
}

func testConsumerConfig() *api.ConsumerConfig {
	return &api.ConsumerConfig{
		AckWait:       0,
		AckPolicy:     api.AckExplicit,
		MaxDeliver:    -1,
		ReplayPolicy:  api.ReplayInstant,
		OptStartSeq:   0,
		OptStartTime:  nil,
		DeliverPolicy: api.DeliverAll,
	}
}
func TestAckWait(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.AckWait(time.Hour)(cfg)
	if cfg.AckWait != time.Hour {
		t.Fatalf("expected 1 hour got %v", cfg.AckWait)
	}
}

func TestAcknowledgeAll(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.AcknowledgeAll()(cfg)
	if cfg.AckPolicy != api.AckAll {
		t.Fatalf("expected AckAll got %s", cfg.AckPolicy)
	}
}

func TestAcknowledgeExplicit(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.AcknowledgeExplicit()(cfg)
	if cfg.AckPolicy != api.AckExplicit {
		t.Fatalf("expected AckExplicit got %s", cfg.AckPolicy)
	}
}

func TestAcknowledgeNone(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.AcknowledgeNone()(cfg)
	if cfg.AckPolicy != api.AckNone {
		t.Fatalf("expected AckNone got %s", cfg.AckPolicy)
	}
}

func TestDeliverAllAvailable(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.DeliverAllAvailable()(cfg)
	if cfg.DeliverPolicy != api.DeliverAll {
		t.Fatal("expected DeliverAll")
	}
}

func TestDeliverySubject(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.DeliverySubject("out")(cfg)
	if cfg.DeliverSubject != "out" {
		t.Fatalf("expected 'out' got %q", cfg.DeliverSubject)
	}
}

func TestDurableName(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.DurableName("test")(cfg)
	if cfg.Durable != "test" {
		t.Fatalf("expected 'test' got %q", cfg.Durable)
	}
}

func TestFilterStreamBySubject(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.FilterStreamBySubject("test")(cfg)
	if cfg.FilterSubject != "test" {
		t.Fatalf("expected 'test' got %q", cfg.FilterSubject)
	}
}

func TestMaxDeliveryAttempts(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.MaxDeliveryAttempts(10)(cfg)
	if cfg.MaxDeliver != 10 {
		t.Fatalf("expected 10 got %q", cfg.MaxDeliver)
	}

	err := jsm.MaxDeliveryAttempts(0)
	if err == nil {
		t.Fatalf("expected 0 deliveries to fail")
	}
}

func TestReplayAsReceived(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.ReplayAsReceived()(cfg)
	if cfg.ReplayPolicy != api.ReplayOriginal {
		t.Fatalf("expected ReplayOriginal got %s", cfg.ReplayPolicy)
	}
}

func TestReplayInstantly(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.ReplayInstantly()(cfg)
	if cfg.ReplayPolicy != api.ReplayInstant {
		t.Fatalf("expected ReplayInstant got %s", cfg.ReplayPolicy)
	}
}

func TestSamplePercent(t *testing.T) {
	cfg := testConsumerConfig()
	err := jsm.SamplePercent(200)(cfg)
	if err == nil {
		t.Fatal("impossible percent didnt error")
	}

	err = jsm.SamplePercent(-1)(cfg)
	if err == nil {
		t.Fatal("impossible percent didnt error")
	}

	err = jsm.SamplePercent(0)(cfg)
	checkErr(t, err, "good percent errored")
	if cfg.SampleFrequency != "" {
		t.Fatal("expected empty string")
	}

	err = jsm.SamplePercent(20)(cfg)
	checkErr(t, err, "good percent errored")
	if cfg.SampleFrequency != "20%" {
		t.Fatal("expected 20 pct string")
	}
}

func TestStartAtSequence(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.StartAtSequence(1024)(cfg)
	if cfg.DeliverPolicy != api.DeliverByStartSequence || cfg.OptStartSeq != 1024 {
		t.Fatal("expected 1024")
	}
}

func TestStartAtTime(t *testing.T) {
	cfg := testConsumerConfig()
	s := time.Now().Add(-1 * time.Hour)
	jsm.StartAtTime(s)(cfg)
	if cfg.DeliverPolicy != api.DeliverByStartTime || cfg.OptStartTime.Unix() != s.Unix() {
		t.Fatal("expected 1 hour delta")
	}
}

func TestStartAtTimeDelta(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.StartAtTimeDelta(time.Hour)(cfg)
	if cfg.DeliverPolicy != api.DeliverByStartTime || cfg.OptStartTime.Unix() < time.Now().Add(-1*time.Hour-time.Second).Unix() || cfg.OptStartTime.Unix() > time.Now().Add(-1*time.Hour+time.Second).Unix() {
		t.Fatal("expected ~ 1 hour delta")
	}
}

func TestStartWithLastReceived(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.StartWithLastReceived()(cfg)
	if cfg.DeliverPolicy != api.DeliverLast {
		t.Fatal("expected DeliverLast")
	}
}
func TestStartWithNextReceived(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.StartWithNextReceived()(cfg)
	if cfg.DeliverPolicy != api.DeliverNew {
		t.Fatal("expected DeliverNew")
	}
}

func TestRateLimitBitPerSecond(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.RateLimitBitsPerSecond(10)(cfg)
	if cfg.RateLimit != 10 {
		t.Fatal("expected RateLimit==10")
	}
}

func TestMaxAckOutstanding(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.MaxAckPending(10)(cfg)
	if cfg.MaxAckPending != 10 {
		t.Fatal("expected MaxAckPending==10")
	}
}

func TestIdleHeartbeat(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.IdleHeartbeat(time.Second)(cfg)
	if cfg.Heartbeat != time.Second {
		t.Fatalf("expected Heartbeat==1s")
	}
}

func TestPushFlowControl(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.PushFlowControl()(cfg)
	if !cfg.FlowControl {
		t.Fatalf("expected FlowControl==true")
	}
}

func TestMaxWaiting(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.MaxWaiting(10)(cfg)
	if cfg.MaxWaiting != 10 {
		t.Fatalf("expected MaxWaiting==10")
	}
}

func TestConsumerDescription(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	s, err := mgr.NewStream("m1", jsm.StreamDescription("test description"), jsm.MemoryStorage())
	checkErr(t, err, "create failed")
	c, err := s.NewConsumer(jsm.ConsumerDescription("test consumer description"), jsm.DurableName("X"))
	checkErr(t, err, "create failed")
	if c.Description() != "test consumer description" {
		t.Fatalf("invalid description %q", c.Description())
	}

	nfo, err := c.State()
	checkErr(t, err, "state failed")
	if nfo.Config.Description != "test consumer description" {
		t.Fatalf("invalid description %q", c.Description())
	}
}
