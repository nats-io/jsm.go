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
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
)

func TestNewStreamFromDefault(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStreamFromDefault("q1", jsm.DefaultWorkQueue, jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.MaxAge(time.Hour))
	checkErr(t, err, "create failed")

	stream.Reset()
	if stream.Name() != "q1" {
		t.Fatalf("incorrect name %q", stream.Name())
	}

	if stream.Storage() != api.MemoryStorage {
		t.Fatalf("incorrect storage, expected memory got %s", stream.Storage())
	}

	if stream.MaxAge() != time.Hour {
		t.Fatalf("incorrect max age, expected 1 hour got %v", stream.MaxAge())
	}
}

func TestLoadOrNewStreamFromDefault(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	names, err := mgr.StreamNames(nil)
	checkErr(t, err, "ls failed")
	if len(names) != 0 {
		t.Fatalf("expected no streams")
	}

	stream, err := mgr.LoadOrNewStreamFromDefault("q1", jsm.DefaultWorkQueue, jsm.Subjects("in.q1"), jsm.MemoryStorage(), jsm.MaxAge(time.Hour))
	checkErr(t, err, "create failed")

	_, err = mgr.LoadOrNewStreamFromDefault("q1", stream.Configuration())
	checkErr(t, err, "load failed")

	names, err = mgr.StreamNames(nil)
	checkErr(t, err, "ls failed")

	if len(names) != 1 || names[0] != "q1" {
		t.Fatalf("expected [q1] got %v", names)
	}
}

func TestNewStream(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
	checkErr(t, err, "create failed")

	if stream.Name() != "q1" {
		t.Fatalf("expected q1 got %s", stream.Name())
	}

	if stream.Storage() != api.FileStorage {
		t.Fatalf("expected file storage got %s", stream.Storage())
	}

	if stream.Retention() != api.LimitsPolicy {
		t.Fatalf("expected stream retention got %s", stream.Retention())
	}
}

func TestLoadOrNewStream(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	names, err := mgr.StreamNames(nil)
	checkErr(t, err, "ls failed")
	if len(names) != 0 {
		t.Fatalf("expected no streams")
	}

	stream, err := mgr.LoadOrNewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
	checkErr(t, err, "create failed")

	if stream.Name() != "q1" {
		t.Fatalf("expected q1 got %s", stream.Name())
	}

	if stream.Storage() != api.FileStorage {
		t.Fatalf("expected file storage got %s", stream.Storage())
	}

	if stream.Retention() != api.LimitsPolicy {
		t.Fatalf("expected stream retention got %s", stream.Retention())
	}

	stream, err = mgr.LoadOrNewStream("q1", jsm.FileStorage())
	checkErr(t, err, "create failed")

	if stream.Name() != "q1" {
		t.Fatalf("expected q1 got %s", stream.Name())
	}
}

func TestLoadStream(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	names, err := mgr.StreamNames(nil)
	checkErr(t, err, "ls failed")
	if len(names) != 0 {
		t.Fatalf("expected no streams")
	}

	_, err = mgr.LoadStream("q1")
	if err == nil {
		t.Fatal("expected error")
	}

	_, err = mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
	checkErr(t, err, "create failed")

	stream, err := mgr.LoadStream("q1")
	checkErr(t, err, "load failed")

	if stream.Name() != "q1" {
		t.Fatalf("expected q1 got %s", stream.Name())
	}

	if stream.Storage() != api.FileStorage {
		t.Fatalf("expected file storage got %s", stream.Storage())
	}
}

func TestStream_Reset(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	orig, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
	checkErr(t, err, "create failed")

	// make original inconsistent
	err = orig.Delete()
	checkErr(t, err, "delete failed")

	// create a new one in its place
	_, err = mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.MemoryStorage())
	checkErr(t, err, "create failed")

	// reload
	err = orig.Reset()
	checkErr(t, err, "reset failed")

	// verify its good
	if orig.Storage() != api.MemoryStorage {
		t.Fatalf("expected memory storage got %s", orig.Storage())
	}
}

func TestStream_LoadConsumer(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
	checkErr(t, err, "create failed")

	_, err = stream.NewConsumer(jsm.DurableName("c1"))
	checkErr(t, err, "create failed")

	consumer, err := stream.LoadConsumer("c1")
	checkErr(t, err, "load failed")

	if consumer.Name() != "c1" {
		t.Fatalf("expected consumer c1 got %s", consumer.Name())
	}
}

func TestStream_NewConsumerFromDefault(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
	checkErr(t, err, "create failed")

	consumer, err := stream.NewConsumerFromDefault(jsm.SampledDefaultConsumer, jsm.DurableName("q1"))
	checkErr(t, err, "create failed")
	consumer.Reset()
	if !consumer.IsSampled() {
		t.Fatalf("expected a sampled consumer")
	}
}

func TestStream_LoadOrNewConsumerFromDefault(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
	checkErr(t, err, "create failed")

	_, err = stream.LoadOrNewConsumerFromDefault("q1", jsm.SampledDefaultConsumer, jsm.DurableName("q1"))
	checkErr(t, err, "create failed")

	consumer, err := stream.LoadOrNewConsumerFromDefault("q1", jsm.SampledDefaultConsumer, jsm.DurableName("q1"))
	checkErr(t, err, "load failed")
	if !consumer.IsSampled() {
		t.Fatalf("expected a sampled consumer")
	}
}

func TestStream_UpdateConfiguration(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("ORDERS.new"))
	checkErr(t, err, "create failed")

	if stream.Configuration().Subjects[0] != "ORDERS.new" {
		t.Fatalf("expected [ORDERS.new], got %v", stream.Configuration().Subjects)
	}

	cfg := stream.Configuration()
	cfg.Subjects = []string{"ORDERS.*"}

	err = stream.UpdateConfiguration(cfg)
	checkErr(t, err, "update failed")

	if len(stream.Configuration().Subjects) != 1 {
		t.Fatalf("expected [ORDERS.*], got %v", stream.Configuration().Subjects)
	}

	if stream.Configuration().Subjects[0] != "ORDERS.*" {
		t.Fatalf("expected [ORDERS.*], got %v", stream.Configuration().Subjects)
	}

	err = stream.UpdateConfiguration(stream.Configuration(), jsm.Subjects("ARCHIVE.*"))
	checkErr(t, err, "update failed")

	if len(stream.Configuration().Subjects) != 1 {
		t.Fatalf("expected [ARCHIVE.*], got %v", stream.Configuration().Subjects)
	}

	if stream.Configuration().Subjects[0] != "ARCHIVE.*" {
		t.Fatalf("expected [ARCHIVE.*], got %v", stream.Configuration().Subjects)
	}
}

func TestStream_ConsumerNames(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
	checkErr(t, err, "create failed")

	_, err = stream.NewConsumer(jsm.DurableName("c1"))
	checkErr(t, err, "create failed")

	_, err = stream.NewConsumer(jsm.DurableName("c2"))
	checkErr(t, err, "create failed")

	names, err := stream.ConsumerNames()
	checkErr(t, err, "names failed")

	if len(names) != 2 || names[0] != "c1" || names[1] != "c2" {
		t.Fatalf("expected [c1, c2] got %v", names)
	}
}

func TestStream_Information(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
	checkErr(t, err, "create failed")

	info, err := stream.Information()
	checkErr(t, err, "info failed")

	if info.Config.Name != "q1" {
		t.Fatalf("expected q1 got %s", info.Config.Name)
	}
}

func TestStream_LatestInformation(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
	checkErr(t, err, "create failed")

	info, err := stream.Information()
	checkErr(t, err, "info failed")

	if info.Config.Name != "q1" {
		t.Fatalf("expected q1 got %s", info.Config.Name)
	}

	srv.Shutdown()
	newinfo, err := stream.LatestInformation()
	checkErr(t, err, "latest info failed")
	if info != newinfo {
		t.Fatalf("Latest information is not the same")
	}
}

func TestStream_State(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test"))
	checkErr(t, err, "create failed")

	stats, err := stream.State()
	checkErr(t, err, "stats failed")
	if stats.Msgs != 0 {
		t.Fatalf("expected 0 messages got %d", stats.Msgs)
	}

	sub := stream.Subjects()[0]
	nc.Publish(sub, []byte("message 1"))
	nc.Publish(sub, []byte("message 2"))

	stats, err = stream.State()
	checkErr(t, err, "stats failed")
	if stats.Msgs != 2 {
		t.Fatalf("expected 1 messages got %d", stats.Msgs)
	}

	stats, err = stream.State(api.JSApiStreamInfoRequest{SubjectsFilter: ">"})
	checkErr(t, err, "stats failed")
	if stats.Msgs != 2 {
		t.Fatalf("expected 1 messages got %d", stats.Msgs)
	}
	if len(stats.Subjects) == 0 {
		t.Fatalf("did not receive subject info")
	}
	if stats.Subjects[sub] != 2 {
		t.Fatalf("received wrong subject stats")
	}
}

func TestStream_LatestState(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test"))
	checkErr(t, err, "create failed")

	state, err := stream.State()
	checkErr(t, err, "state failed")
	if state.Msgs != 0 {
		t.Fatalf("expected 0 messages got %d", state.Msgs)
	}

	srv.Shutdown()
	newstate, err := stream.LatestState()
	checkErr(t, err, "state failed")
	if !cmp.Equal(newstate, state) {
		t.Fatalf("latest state is not the same as last")
	}
}

func TestStream_Delete(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test"))
	checkErr(t, err, "create failed")

	err = stream.Delete()
	checkErr(t, err, "delete failed")

	names, err := mgr.StreamNames(nil)
	checkErr(t, err, "names failed")

	if len(names) != 0 {
		t.Fatalf("got streams where none should be: %v", names)
	}
}

func TestStream_Dedupe(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test"))
	checkErr(t, err, "create failed")

	for i := 0; i < 4; i++ {
		m := nats.NewMsg(stream.Subjects()[0])
		m.Data = []byte(fmt.Sprintf("message %d", i))
		m.Header.Add("Nats-Msg-Id", strconv.Itoa(i%2))
		nc.PublishMsg(m)
	}

	stats, err := stream.State()
	checkErr(t, err, "stats failed")
	if stats.Msgs != 2 {
		t.Fatalf("expected 2 messages got %d", stats.Msgs)
	}
}

func TestStream_Purge(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test.>"), jsm.MaxMessagesPerSubject(100))
	checkErr(t, err, "create failed")

	for i := 0; i < 100; i++ {
		subj := fmt.Sprintf("test.%d", i%2)
		_, err := nc.Request(subj, []byte(fmt.Sprintf("message %d", i)), time.Second)
		checkErr(t, err, "publish failed")
	}

	checkCnt := func(t *testing.T, count uint64) {
		t.Helper()
		stats, err := stream.State()
		checkErr(t, err, "stats failed")
		if stats.Msgs != count {
			t.Fatalf("expected %d messages got %d", count, stats.Msgs)
		}
	}

	checkCnt(t, 100)

	err = stream.Purge(&api.JSApiStreamPurgeRequest{Subject: "test.1", Keep: 4})
	checkErr(t, err, "purge failed")
	checkCnt(t, 54)

	err = stream.Purge(&api.JSApiStreamPurgeRequest{Subject: "test.0", Keep: 4})
	checkErr(t, err, "purge failed")
	checkCnt(t, 8)

	err = stream.Purge()
	checkErr(t, err, "purge failed")

	checkCnt(t, 0)
}

func TestStream_ReadLastMessageForSubject(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test.*"))
	checkErr(t, err, "create failed")

	for i := 0; i < 10; i++ {
		_, err := nc.Request(fmt.Sprintf("test.%d", i%3), []byte(fmt.Sprintf("message %d", i)), time.Second)
		if err != nil {
			t.Fatalf("request failed: %s", err)
		}
	}

	get := func(t *testing.T, subj string, expect string) {
		t.Helper()

		msg, err := stream.ReadLastMessageForSubject(subj)
		if err != nil {
			t.Fatalf("read failed: %s", err)
		}

		if msg.Subject != subj {
			t.Fatalf("wrong subject %q expected %q", msg.Subject, subj)
		}

		if string(msg.Data) != expect {
			t.Fatalf("wrong body %q expected %q", string(msg.Data), expect)
		}
	}

	get(t, "test.0", "message 9")
	get(t, "test.1", "message 7")
	get(t, "test.2", "message 8")
}

func TestStream_ReadMessage(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test"))
	checkErr(t, err, "create failed")

	nc.Publish(stream.Subjects()[0], []byte("message 1"))

	msg, err := stream.ReadMessage(1)
	checkErr(t, err, "load failed")
	if string(msg.Data) != "message 1" {
		t.Fatalf("expected message 1 got %q", msg.Data)
	}

	_, err = stream.ReadMessage(2)
	if err == nil {
		t.Fatalf("ReadMessage didnt fail on unknown message")
	}
}

func TestStream_DeleteMessage(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test"))
	checkErr(t, err, "create failed")

	nc.Publish(stream.Subjects()[0], []byte("message 1"))

	_, err = stream.ReadMessage(1)
	checkErr(t, err, "load failed")

	err = stream.DeleteMessage(1)
	checkErr(t, err, "delete failed")

	msg, err := stream.ReadMessage(1)
	if err == nil {
		t.Fatalf("deleted messages was loaded %q", msg.Data)
	}
}

func testStreamConfig() *api.StreamConfig {
	return &api.StreamConfig{
		Subjects:     []string{},
		MaxAge:       -1,
		MaxBytes:     -1,
		MaxMsgSize:   -1,
		MaxMsgs:      -1,
		MaxConsumers: -1,
		Replicas:     -1,
	}
}

func TestFileStorage(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.FileStorage()(cfg)
	checkErr(t, err, "failed")
	if cfg.Storage != api.FileStorage {
		t.Fatalf("expected FileStorage")
	}
}

func TestInterestRetention(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.InterestRetention()(cfg)
	checkErr(t, err, "failed")
	if cfg.Retention != api.InterestPolicy {
		t.Fatalf("expected InterestPolicy")
	}
}

func TestMaxAge(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.MaxAge(time.Hour)(cfg)
	checkErr(t, err, "failed")
	if cfg.MaxAge != time.Hour {
		t.Fatalf("expected 1 hour")
	}
}

func TestMaxBytes(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.MaxBytes(1024)(cfg)
	checkErr(t, err, "failed")
	if cfg.MaxBytes != 1024 {
		t.Fatalf("expected 1024")
	}
}

func TestMaxMessageSize(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.MaxMessageSize(1024)(cfg)
	checkErr(t, err, "failed")
	if cfg.MaxMsgSize != 1024 {
		t.Fatalf("expected 1024")
	}
}

func TestMaxMessages(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.MaxMessages(1024)(cfg)
	checkErr(t, err, "failed")
	if cfg.MaxMsgs != 1024 {
		t.Fatalf("expected 1024")
	}
}

func TestMaxMessagesPerSubject(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.MaxMessagesPerSubject(1024)(cfg)
	checkErr(t, err, "failed")
	if cfg.MaxMsgsPer != 1024 {
		t.Fatalf("expected 1024")
	}
}

func TestMaxObservables(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.MaxConsumers(1024)(cfg)
	checkErr(t, err, "failed")
	if cfg.MaxConsumers != 1024 {
		t.Fatalf("expected 1024")
	}
}

func TestMemoryStorage(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.MemoryStorage()(cfg)
	checkErr(t, err, "memory storage failed")
	if cfg.Storage != api.MemoryStorage {
		t.Fatalf("expected MemoryStorage")
	}
}

func TestDiscardNew(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.DiscardNew()(cfg)
	checkErr(t, err, "discard new failed")
	if cfg.Discard != api.DiscardNew {
		t.Fatal("expected DiscardNew")
	}
}

func TestDiscardOld(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.DiscardOld()(cfg)
	checkErr(t, err, "discard old failed")
	if cfg.Discard != api.DiscardOld {
		t.Fatal("expected DiscardOld")
	}
}

func TestNoAck(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.NoAck()(cfg)
	checkErr(t, err, "failed")
	if !cfg.NoAck {
		t.Fatalf("expected NoAck")
	}
}

func TestReplicas(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.Replicas(1024)(cfg)
	checkErr(t, err, "failed")
	if cfg.Replicas != 1024 {
		t.Fatalf("expected 1024")
	}
}

func TestLimitsRetention(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.LimitsRetention()(cfg)
	checkErr(t, err, "failed")
	if cfg.Retention != api.LimitsPolicy {
		t.Fatalf("expected LimitsPolicy")
	}
}

func TestSubjects(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.Subjects("one", "two")(cfg)
	checkErr(t, err, "failed")
	if len(cfg.Subjects) != 2 || cfg.Subjects[0] != "one" || cfg.Subjects[1] != "two" {
		t.Fatalf("expected [one, two] got %v", cfg.Subjects)
	}
}

func TestWorkQueueRetention(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.WorkQueueRetention()(cfg)
	checkErr(t, err, "failed")
	if cfg.Retention != api.WorkQueuePolicy {
		t.Fatalf("expected WorkQueuePolicy")
	}
}

func TestDuplicateWindow(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.DuplicateWindow(time.Hour)(cfg)
	checkErr(t, err, "failed")
	if cfg.Duplicates != time.Hour {
		t.Fatalf("expected a 1 hour windows got %v", cfg.Duplicates)
	}
}

func TestPlacementCluster(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.PlacementCluster("EAST")(cfg)
	checkErr(t, err, "failed")
	if cfg.Placement.Cluster != "EAST" {
		t.Fatalf("expected EAST got %q", cfg.Placement.Cluster)
	}

	err = jsm.PlacementCluster("WEST")(cfg)
	checkErr(t, err, "failed")
	if cfg.Placement.Cluster != "WEST" {
		t.Fatalf("expected WEST got %q", cfg.Placement.Cluster)
	}
}

func TestPlacementTags(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.PlacementTags("EAST", "SSD")(cfg)
	checkErr(t, err, "failed")
	if !cmp.Equal(cfg.Placement.Tags, []string{"EAST", "SSD"}) {
		t.Fatalf("expected [EAST, SSD] got %q", cfg.Placement.Tags)
	}

	err = jsm.PlacementTags("WEST", "HDD")(cfg)
	checkErr(t, err, "failed")
	if !cmp.Equal(cfg.Placement.Tags, []string{"WEST", "HDD"}) {
		t.Fatalf("expected [WEST, HDD] got %q", cfg.Placement.Tags)
	}
}

func TestSources(t *testing.T) {
	cfg := testStreamConfig()
	expected := []*api.StreamSource{{Name: "one"}, {Name: "two"}}

	err := jsm.Sources(expected...)(cfg)
	checkErr(t, err, "failed")
	if !cmp.Equal(cfg.Sources, expected) {
		t.Fatalf("expected [one, two] got %#v", cfg.Sources)
	}
}

func TestMirror(t *testing.T) {
	cfg := testStreamConfig()
	expected := &api.StreamSource{Name: "one"}
	err := jsm.Mirror(expected)(cfg)
	checkErr(t, err, "failed")
	if !cmp.Equal(cfg.Mirror, expected) {
		t.Fatalf("expected 'one' got %#v", cfg.Mirror)
	}
}

func TestDenyDelete(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.DenyDelete()(cfg)
	if err != nil {
		t.Fatalf("option failed: %s", err)
	}
	if !cfg.DenyDelete {
		t.Fatalf("DenyDelete was not set")
	}
}

func TestDenyPurge(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.DenyPurge()(cfg)
	if err != nil {
		t.Fatalf("option failed: %s", err)
	}
	if !cfg.DenyPurge {
		t.Fatalf("DenyPurge was not set")
	}
}

func TestAllowRollup(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.AllowRollup()(cfg)
	if err != nil {
		t.Fatalf("option failed: %s", err)
	}
	if !cfg.RollupAllowed {
		t.Fatalf("RollupAllowed was not set")
	}
}

func TestStream_IsMirror(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
	checkErr(t, err, "create failed")
	s, err := mgr.NewStream("q2", jsm.FileStorage(), jsm.Mirror(&api.StreamSource{Name: "q1"}))
	checkErr(t, err, "create failed")

	if !s.IsMirror() {
		t.Fatalf("Expected a mirror")
	}
}

func TestStream_IsSourced(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
	checkErr(t, err, "create failed")
	_, err = mgr.NewStream("q2", jsm.Subjects("in.q2"), jsm.FileStorage())
	checkErr(t, err, "create failed")
	s, err := mgr.NewStream("q3", jsm.Subjects("in.q3"), jsm.FileStorage(), jsm.Sources(&api.StreamSource{Name: "q1"}, &api.StreamSource{Name: "q2"}))
	checkErr(t, err, "create failed")

	if !s.IsSourced() {
		t.Fatalf("Expected a synced")
	}
}

func TestStreamDescription(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	s, err := mgr.NewStream("m1", jsm.StreamDescription("test description"))
	checkErr(t, err, "create failed")
	if s.Description() != "test description" {
		t.Fatalf("invalid description %q", s.Description())
	}

	nfo, err := s.Information()
	checkErr(t, err, "info failed")

	if nfo.Config.Description != "test description" {
		t.Fatalf("invalid description %q", s.Description())
	}
}

func TestStreamPageContents(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

	s, err := mgr.NewStream("m1", jsm.Subjects("test"), jsm.WorkQueueRetention())
	checkErr(t, err, "create failed")

	pager, err := s.PageContents()
	if err.Error() != "work queue retention streams can not be paged" {
		t.Fatalf("Expected an error, got: %v", err)
	}
	if pager != nil {
		t.Fatalf("expected a nil pager")
	}
}

func TestStreamSealed(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	s, err := mgr.NewStream("m1", jsm.Subjects("test"))
	checkErr(t, err, "create failed")

	_, err = nc.Request("test", []byte("1"), time.Second)
	checkErr(t, err, "publish failed")

	err = s.Seal()
	checkErr(t, err, "seal update failed")

	nfo, err := s.LatestInformation()
	checkErr(t, err, "info failed")

	if !nfo.Config.Sealed {
		t.Fatalf("expected a sealed stream")
	}

	if !s.Sealed() {
		t.Fatalf("expected a sealed stream")
	}

	err = s.DeleteMessage(1)
	if !jsm.IsNatsError(err, 10109) {
		t.Fatalf("expected err 10109 got %v", err)
	}
}

func TestStreamRepublish(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	s, err := mgr.NewStream("m1", jsm.Subjects("test.*"), jsm.Republish(&api.SubjectMapping{Source: "test.*", Destination: "repub.{{partition(2,1)}}"}))
	checkErr(t, err, "create failed")
	if s.Configuration().RePublish == nil {
		t.Fatalf("expected republish configuration")
	}

	received := map[int]int{0: 0, 1: 0}
	mu := sync.Mutex{}
	s1, err := nc.Subscribe("repub.1", func(m *nats.Msg) {
		mu.Lock()
		received[1]++
		mu.Unlock()
	})
	checkErr(t, err, "sub failed")
	s2, err := nc.Subscribe("repub.0", func(m *nats.Msg) {
		mu.Lock()
		received[0]++
		mu.Unlock()
	})
	checkErr(t, err, "sub failed")

	for i := 0; i < 100; i++ {
		_, err := nc.Request(fmt.Sprintf("test.%d", i), []byte("hello"), time.Second)
		checkErr(t, err, "req failed")
	}

	s1.Drain()
	s2.Drain()

	nfo, err := s.State()
	if err != nil {
		t.Fatalf("state failed: %v", err)
	}
	if nfo.Msgs != 100 {
		t.Fatalf("Stream did not have 100 messages: %d", nfo.Msgs)
	}

	if received[0] == 0 || received[1] == 0 {
		t.Fatalf("Expected 2 mapped subjects to get messages: %#v", received)
	}
}
