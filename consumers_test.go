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
	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go"
)

func setupConsumerTest(t *testing.T) (*server.Server, *nats.Conn, *jsm.Stream) {
	t.Helper()
	srv, nc := startJSServer(t)
	stream, err := jsm.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.FileStorage(), jsm.MaxAge(time.Hour), jsm.Subjects("ORDERS.>"))
	checkErr(t, err, "create failed")

	_, err = nc.Request("ORDERS.new", []byte("order 1"), time.Second)
	checkErr(t, err, "publish failed")

	return srv, nc, stream

}

func TestConsumer_DeliveryPolicyConsistency(t *testing.T) {
	c, err := jsm.NewConsumerConfiguration(jsm.DefaultConsumer)
	checkErr(t, err, "create failed")

	checkPolicy := func(c *jsm.ConsumerConfig, sseq uint64, stime time.Time, dlast, dall bool) {
		t.Helper()

		if c.StreamSeq != sseq {
			t.Fatalf("StreamSeq expected %d got %d", sseq, c.StreamSeq)
		}

		if c.StartTime != stime {
			t.Fatalf("StartTime expected %v got %v", stime, c.StartTime)
		}

		if c.DeliverLast != dlast {
			t.Fatalf("DeliverLast expected %v got %v", dlast, c.DeliverLast)
		}

		if c.DeliverAll != dall {
			t.Fatalf("DeliverAll expected %v got %v", dall, c.DeliverAll)
		}
	}

	checkPolicy(c, 0, time.Time{}, false, true)

	jsm.StartAtSequence(10)(c)
	checkPolicy(c, 10, time.Time{}, false, false)

	now := time.Now()
	jsm.StartAtTime(now)(c)
	checkPolicy(c, 0, now, false, false)

	jsm.DeliverAllAvailable()(c)
	checkPolicy(c, 0, time.Time{}, false, true)

	jsm.StartWithLastReceived()(c)
	checkPolicy(c, 0, time.Time{}, true, false)
}

func TestNextMsg(t *testing.T) {
	srv, nc, stream := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream.Purge()

	consumer, err := jsm.NewConsumer("ORDERS", jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"), jsm.DeliverAllAvailable())
	checkErr(t, err, "create failed")

	for i := 0; i <= 100; i++ {
		nc.Publish("ORDERS.new", []byte(fmt.Sprintf("%d", i)))
	}

	for i := 0; i <= 100; i++ {
		msg, err := consumer.NextMsg(jsm.WithTimeout(500 * time.Millisecond))
		checkErr(t, err, "NextMsg failed")

		b, err := strconv.Atoi(string(msg.Data))
		checkErr(t, err, fmt.Sprintf("invalid body: %q", string(msg.Data)))

		if b != i {
			t.Fatalf("got message %d expected %d", b, i)
		}

		msg.Respond(nil)
	}
}

func TestNewConsumer(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	consumer, err := jsm.NewConsumer("ORDERS", jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	consumer.Reset()
	if consumer.AckPolicy() != server.AckExplicit {
		t.Fatalf("expected explicit ack got %s", consumer.AckPolicy().String())
	}

	if consumer.Name() != "NEW" {
		t.Fatalf("expected NEW got %s", consumer.Name())
	}
}

func TestNewConsumerFromDefaultDurable(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	consumer, err := jsm.NewConsumerFromDefault("ORDERS", jsm.SampledDefaultConsumer, jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	consumer.Reset()
	if consumer.AckPolicy() != server.AckExplicit {
		t.Fatalf("expected explicit ack got %s", consumer.AckPolicy().String())
	}

	if consumer.Name() != "NEW" {
		t.Fatalf("expected NEW got %s", consumer.Name())
	}

	if !consumer.IsSampled() {
		t.Fatal("expected a sampled consumer")
	}
}

func TestNewConsumerFromDefaultEphemeral(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	// interest is needed
	nc.Subscribe("out", func(_ *nats.Msg) {})

	consumer, err := jsm.NewConsumerFromDefault("ORDERS", jsm.SampledDefaultConsumer, jsm.DeliverySubject("out"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	consumers, err := jsm.ConsumerNames("ORDERS")
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
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	_, err := jsm.NewConsumerFromDefault("ORDERS", jsm.SampledDefaultConsumer, jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	consumer, err := jsm.LoadConsumer("ORDERS", "NEW")
	checkErr(t, err, "load failed")

	if consumer.AckPolicy() != server.AckExplicit {
		t.Fatalf("expected explicit ack got %s", consumer.AckPolicy().String())
	}

	if consumer.Name() != "NEW" {
		t.Fatalf("expected NEW got %s", consumer.Name())
	}

	if !consumer.IsSampled() {
		t.Fatal("expected a sampled consumer")
	}
}

func TestLoadOrNewConsumer(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	_, err := jsm.LoadOrNewConsumer("ORDERS", "NEW", jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	consumer, err := jsm.LoadOrNewConsumer("ORDERS", "NEW", jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "load failed")

	if consumer.AckPolicy() != server.AckExplicit {
		t.Fatalf("expected explicit ack got %s", consumer.AckPolicy().String())
	}

	if consumer.Name() != "NEW" {
		t.Fatalf("expected NEW got %s", consumer.Name())
	}
}

func TestLoadOrNewConsumerFromDefault(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	_, err := jsm.LoadOrNewConsumerFromDefault("ORDERS", "NEW", jsm.SampledDefaultConsumer, jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	consumer, err := jsm.LoadOrNewConsumerFromDefault("ORDERS", "NEW", jsm.SampledDefaultConsumer, jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "load failed")

	if consumer.AckPolicy() != server.AckExplicit {
		t.Fatalf("expected explicit ack got %s", consumer.AckPolicy().String())
	}

	if consumer.Name() != "NEW" {
		t.Fatalf("expected NEW got %s", consumer.Name())
	}

	if !consumer.IsSampled() {
		t.Fatal("expected a sampled consumer")
	}
}

func TestConsumer_Reset(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	consumer, err := jsm.NewConsumer("ORDERS", jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	err = consumer.Delete()
	checkErr(t, err, "delete failed")

	consumer, err = jsm.NewConsumer("ORDERS", jsm.DurableName("NEW"))
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

	if s != "$JS.STREAM.str.CONSUMER.cons.NEXT" {
		t.Fatalf("invalid next subject %q", s)
	}
}

func TestConsumer_NextSubject(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	consumer, err := jsm.NewConsumer("ORDERS", jsm.DurableName("NEW"), jsm.FilterStreamBySubject("ORDERS.new"))
	checkErr(t, err, "create failed")

	if consumer.NextSubject() != "$JS.STREAM.ORDERS.CONSUMER.NEW.NEXT" {
		t.Fatalf("expected next subject got %s", consumer.NextSubject())
	}
}

func TestConsumer_SampleSubject(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	consumer, err := jsm.NewConsumerFromDefault("ORDERS", jsm.SampledDefaultConsumer, jsm.DurableName("NEW"))
	checkErr(t, err, "create failed")

	if consumer.AckSampleSubject() != "$JS.EVENT.METRIC.CONSUMER_ACK.ORDERS.NEW" {
		t.Fatalf("expected next subject got %s", consumer.AckSampleSubject())
	}

	unsampled, err := jsm.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("UNSAMPLED"))
	checkErr(t, err, "create failed")

	if unsampled.AckSampleSubject() != "" {
		t.Fatalf("expected empty next subject got %s", consumer.AckSampleSubject())
	}
}

func TestConsumer_State(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	durable, err := jsm.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("D"))
	checkErr(t, err, "create failed")

	state, err := durable.State()
	checkErr(t, err, "state failed")

	if state.Delivered.StreamSeq != 0 {
		t.Fatalf("expected set seq 0 got %d", state.Delivered.StreamSeq)
	}

	m, err := durable.NextMsg()
	checkErr(t, err, "next failed")
	err = m.Respond(nil)
	checkErr(t, err, "ack failed")

	state, err = durable.State()
	checkErr(t, err, "state failed")

	if state.Delivered.StreamSeq != 1 {
		t.Fatalf("expected set seq 1 got %d", state.Delivered.StreamSeq)
	}

}

func TestConsumer_Configuration(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	durable, err := jsm.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("D"))
	checkErr(t, err, "create failed")

	if durable.Configuration().Durable != "D" {
		t.Fatalf("got wrong config: %+v", durable.Configuration())
	}
}

func TestConsumer_Delete(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	durable, err := jsm.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("D"))
	checkErr(t, err, "create failed")
	if !durable.IsDurable() {
		t.Fatalf("expected durable, got %s", durable.DurableName())
	}

	err = durable.Delete()
	checkErr(t, err, "delete failed")

	names, err := jsm.ConsumerNames("ORDERS")
	checkErr(t, err, "names failed")

	if len(names) != 0 {
		t.Fatalf("expected [] got %v", names)
	}
}

func TestConsumer_IsDurable(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	durable, err := jsm.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("D"))
	checkErr(t, err, "create failed")
	if !durable.IsDurable() {
		t.Fatalf("expected durable, got %s", durable.DurableName())
	}
	durable.Delete()

	// interest is needed before creating a ephemeral push
	nc.Subscribe("out", func(_ *nats.Msg) {})

	_, err = jsm.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DeliverySubject("out"))
	checkErr(t, err, "create failed")

	names, err := jsm.ConsumerNames("ORDERS")
	checkErr(t, err, "names failed")

	if len(names) == 0 {
		t.Fatal("got no consumers")
	}

	eph, err := jsm.LoadConsumer("ORDERS", names[0])
	checkErr(t, err, "load failed")
	if eph.IsDurable() {
		t.Fatalf("expected ephemeral got %q %q", eph.Name(), eph.DurableName())
	}
}

func TestConsumer_IsPullMode(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	push, err := jsm.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("PUSH"), jsm.DeliverySubject("out"))
	checkErr(t, err, "create failed")
	if push.IsPullMode() {
		t.Fatalf("expected push, got %s", push.DeliverySubject())
	}

	pull, err := jsm.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("PULL"))
	checkErr(t, err, "create failed")
	if !pull.IsPullMode() {
		t.Fatalf("expected pull, got %s", pull.DeliverySubject())
	}
}

func TestConsumer_IsPushMode(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	push, err := jsm.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("PUSH"), jsm.DeliverySubject("out"))
	checkErr(t, err, "create failed")
	if !push.IsPushMode() {
		t.Fatalf("expected push, got %s", push.DeliverySubject())
	}

	pull, err := jsm.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("PULL"))
	checkErr(t, err, "create failed")
	if pull.IsPushMode() {
		t.Fatalf("expected pull, got %s", pull.DeliverySubject())
	}
}

func TestConsumer_IsSampled(t *testing.T) {
	srv, nc, _ := setupConsumerTest(t)
	defer srv.Shutdown()
	defer nc.Flush()

	sampled, err := jsm.NewConsumerFromDefault("ORDERS", jsm.SampledDefaultConsumer, jsm.DurableName("SAMPLED"))
	checkErr(t, err, "create failed")
	if !sampled.IsSampled() {
		t.Fatalf("expected sampled, got %s", sampled.SampleFrequency())
	}

	unsampled, err := jsm.NewConsumerFromDefault("ORDERS", jsm.DefaultConsumer, jsm.DurableName("UNSAMPLED"))
	checkErr(t, err, "create failed")
	if unsampled.IsSampled() {
		t.Fatalf("expected un-sampled, got %s", unsampled.SampleFrequency())
	}
}

func testConsumerConfig() *jsm.ConsumerConfig {
	return &jsm.ConsumerConfig{ConsumerConfig: server.ConsumerConfig{
		AckWait:      0,
		AckPolicy:    -1,
		MaxDeliver:   -1,
		ReplayPolicy: -1,
		StreamSeq:    0,
		StartTime:    time.Now(),
	}}
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
	if cfg.AckPolicy != server.AckAll {
		t.Fatalf("expected AckAll got %s", cfg.AckPolicy.String())
	}
}

func TestAcknowledgeExplicit(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.AcknowledgeExplicit()(cfg)
	if cfg.AckPolicy != server.AckExplicit {
		t.Fatalf("expected AckExplicit got %s", cfg.AckPolicy.String())
	}
}

func TestAcknowledgeNone(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.AcknowledgeNone()(cfg)
	if cfg.AckPolicy != server.AckNone {
		t.Fatalf("expected AckNone got %s", cfg.AckPolicy.String())
	}
}

func TestDeliverAllAvailable(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.DeliverAllAvailable()(cfg)
	if !cfg.DeliverAll {
		t.Fatal("expected DeliverAll")
	}
}

func TestDeliverySubject(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.DeliverySubject("out")(cfg)
	if cfg.Delivery != "out" {
		t.Fatalf("expected 'out' got %q", cfg.Delivery)
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
	if cfg.ReplayPolicy != server.ReplayOriginal {
		t.Fatalf("expected ReplayOriginal got %s", cfg.ReplayPolicy.String())
	}
}

func TestReplayInstantly(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.ReplayInstantly()(cfg)
	if cfg.ReplayPolicy != server.ReplayInstant {
		t.Fatalf("expected ReplayInstant got %s", cfg.ReplayPolicy.String())
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
	if cfg.StreamSeq != 1024 {
		t.Fatal("expected 1024")
	}
}

func TestStartAtTime(t *testing.T) {
	cfg := testConsumerConfig()
	s := time.Now().Add(-1 * time.Hour)
	jsm.StartAtTime(s)(cfg)
	if cfg.StartTime.Unix() != s.Unix() {
		t.Fatal("expected 1 hour delta")
	}
}

func TestStartAtTimeDelta(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.StartAtTimeDelta(time.Hour)(cfg)
	if cfg.StartTime.Unix() < time.Now().Add(-1*time.Hour-time.Second).Unix() || cfg.StartTime.Unix() > time.Now().Add(-1*time.Hour+time.Second).Unix() {
		t.Fatal("expected ~ 1 hour delta")
	}
}

func TestStartWithLastReceived(t *testing.T) {
	cfg := testConsumerConfig()
	jsm.StartWithLastReceived()(cfg)
	if !cfg.DeliverLast {
		t.Fatal("expected DeliverLast")
	}
}
