// Copyright 2020-2024 The NATS Authors
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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	testapi "github.com/nats-io/jsm.go/test/testing_client/api"
	"github.com/nats-io/jsm.go/test/testing_client/srvtest"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
)

func TestNewStreamFromDefault(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		_, err := mgr.NewStreamFromDefault("q1", jsm.DefaultStream, jsm.Subjects(">"))
		if !errors.Is(err, jsm.ErrAckStreamIngestsAll) {
			t.Fatalf("Expected overlap error got %v", err)
		}

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
	})
}

func TestLoadOrNewStreamFromDefault(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
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
	})
}

func TestNewStream(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
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
	})
}

func TestLoadOrNewStream(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
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
	})
}

func TestLoadStream(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
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
	})
}

func TestLoadFromStreamDetailBytes(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, srv *testapi.ManagedServer) {
		q1, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
		checkErr(t, err, "create failed")
		c1, err := q1.NewConsumer(jsm.DurableName("C1"))
		checkErr(t, err, "create failed")

		_, err = q1.NewConsumer(jsm.DurableName("C2"))
		checkErr(t, err, "create failed")

		_, err = mgr.NatsConn().Request("in.q1", nil, time.Second)
		checkErr(t, err, "publish failed")
		msg, err := c1.NextMsg()
		checkErr(t, err, "next failed")
		err = msg.Ack()
		checkErr(t, err, "ack failed")

		sysNc, err := nats.Connect(srv.URL, nats.UserInfo("system", "password"))
		checkErr(t, err, "sys conn failed")
		defer sysNc.Close()

		jopts, _ := json.Marshal(&server.JSzOptions{Streams: true, Config: true, Consumer: true})
		res, err := sysNc.Request("$SYS.REQ.SERVER.PING.JSZ", jopts, time.Second)
		checkErr(t, err, "jsz failed")
		var jszr server.ServerAPIJszResponse
		checkErr(t, json.Unmarshal(res.Data, &jszr), "jsz failed")

		if jszr.Error != nil {
			t.Fatalf("jsz failed %v", jszr.Error)
		}

		jsz := jszr.Data

		matched := -1
		for i, acct := range jsz.AccountDetails {
			if acct.Name == "USERS1" {
				if len(acct.Streams) == 0 {
					t.Fatalf("jsz should have at least one stream")
				}
				matched = i
			}
		}

		if matched == -1 {
			t.Fatalf("did not find jsz data for USERS1 account")
		}

		sdb, err := json.Marshal(jsz.AccountDetails[matched].Streams[0])
		checkErr(t, err, "json marshal failed")

		stream, consumers, err := mgr.LoadFromStreamDetailBytes(sdb)
		checkErr(t, err, "load failed")

		if stream.Name() != "q1" || !cmp.Equal(stream.Subjects(), []string{"in.q1"}) {
			t.Fatalf("invalid stream")
		}

		state, err := stream.LatestState()
		checkErr(t, err, "latest state failed")

		if state.Msgs != 1 {
			t.Fatalf("invalid state")
		}
		if state.Consumers != 2 {
			t.Fatalf("invalid consumers")
		}

		if len(consumers) != 2 {
			t.Fatalf("invalid consumers")
		}

		if consumers[0].Name() != "C1" || consumers[0].StreamName() != "q1" {
			t.Fatalf("invalid c1 consumer")
		}
		af, err := consumers[0].AcknowledgedFloor()
		checkErr(t, err, "ackfloor failed")
		if af.Stream != 1 {
			t.Fatalf("invalid ack floor")
		}

		if consumers[1].Name() != "C2" || consumers[1].StreamName() != "q1" {
			t.Fatalf("invalid c2 consumer")
		}
	})
}

func TestStream_Reset(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
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
	})
}

func TestStream_LoadConsumer(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
		checkErr(t, err, "create failed")

		_, err = stream.NewConsumer(jsm.DurableName("c1"))
		checkErr(t, err, "create failed")

		consumer, err := stream.LoadConsumer("c1")
		checkErr(t, err, "load failed")

		if consumer.Name() != "c1" {
			t.Fatalf("expected consumer c1 got %s", consumer.Name())
		}
	})
}

func TestStream_NewConsumerFromDefault(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
		checkErr(t, err, "create failed")

		consumer, err := stream.NewConsumerFromDefault(jsm.SampledDefaultConsumer, jsm.DurableName("q1"))
		checkErr(t, err, "create failed")
		consumer.Reset()
		if !consumer.IsSampled() {
			t.Fatalf("expected a sampled consumer")
		}
	})
}

func TestStream_LoadOrNewConsumerFromDefault(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
		checkErr(t, err, "create failed")

		_, err = stream.LoadOrNewConsumerFromDefault("q1", jsm.SampledDefaultConsumer, jsm.DurableName("q1"))
		checkErr(t, err, "create failed")

		consumer, err := stream.LoadOrNewConsumerFromDefault("q1", jsm.SampledDefaultConsumer, jsm.DurableName("q1"))
		checkErr(t, err, "load failed")
		if !consumer.IsSampled() {
			t.Fatalf("expected a sampled consumer")
		}
	})
}

func TestStream_UpdateConfiguration(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
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
	})
}

func TestStream_ConsumerNames(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
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
	})
}

func TestStream_Information(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
		checkErr(t, err, "create failed")

		info, err := stream.Information()
		checkErr(t, err, "info failed")

		if info.Config.Name != "q1" {
			t.Fatalf("expected q1 got %s", info.Config.Name)
		}
	})
}

func TestStream_LatestInformation(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, client *srvtest.Client, srv *testapi.ManagedServer) {
		stream, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
		checkErr(t, err, "create failed")

		info, err := stream.Information()
		checkErr(t, err, "info failed")

		if info.Config.Name != "q1" {
			t.Fatalf("expected q1 got %s", info.Config.Name)
		}

		client.StopServer(t, srv)

		newinfo, err := stream.LatestInformation()
		checkErr(t, err, "latest info failed")
		if info != newinfo {
			t.Fatalf("Latest information is not the same")
		}
	})
}

func TestStream_State(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		_, names, err := mgr.Streams(nil)
		checkErr(t, err, "names failed")
		if len(names) != 0 {
			t.Fatalf("expected 0 names, got %d: %v", len(names), names)
		}

		stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test"))
		checkErr(t, err, "create failed")

		stats, err := stream.State()
		checkErr(t, err, "stats failed")
		if stats.Msgs != 0 {
			t.Fatalf("expected 0 messages got %d", stats.Msgs)
		}

		sub := stream.Subjects()[0]
		streamPublish(t, mgr.NatsConn(), sub, []byte("message 1"))
		streamPublish(t, mgr.NatsConn(), sub, []byte("message 2"))

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
	})
}

func TestStream_LatestState(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, client *srvtest.Client, srv *testapi.ManagedServer) {
		stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test"))
		checkErr(t, err, "create failed")

		state, err := stream.State()
		checkErr(t, err, "state failed")
		if state.Msgs != 0 {
			t.Fatalf("expected 0 messages got %d", state.Msgs)
		}

		client.StopServer(t, srv)

		newstate, err := stream.LatestState()
		checkErr(t, err, "state failed")
		if !cmp.Equal(newstate, state) {
			t.Fatalf("latest state is not the same as last")
		}
	})
}

func TestStream_Delete(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test"))
		checkErr(t, err, "create failed")

		err = stream.Delete()
		checkErr(t, err, "delete failed")

		names, err := mgr.StreamNames(nil)
		checkErr(t, err, "names failed")

		if len(names) != 0 {
			t.Fatalf("got streams where none should be: %v", names)
		}
	})
}

func TestStream_Dedupe(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test"))
		checkErr(t, err, "create failed")

		for i := 0; i < 4; i++ {
			m := nats.NewMsg(stream.Subjects()[0])
			m.Data = []byte(fmt.Sprintf("message %d", i))
			m.Header.Add("Nats-Msg-Id", strconv.Itoa(i%2))
			_, err := mgr.NatsConn().RequestMsg(m, time.Second)
			checkErr(t, err, "Publish failed")
		}

		stats, err := stream.State()
		checkErr(t, err, "stats failed")
		if stats.Msgs != 2 {
			t.Fatalf("expected 2 messages got %d", stats.Msgs)
		}
	})
}

func TestStream_Purge(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test.>"), jsm.MaxMessagesPerSubject(100))
		checkErr(t, err, "create failed")

		for i := 0; i < 100; i++ {
			subj := fmt.Sprintf("test.%d", i%2)
			_, err := mgr.NatsConn().Request(subj, []byte(fmt.Sprintf("message %d", i)), time.Second)
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
	})
}

func TestStream_ReadLastMessageForSubject(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test.*"))
		checkErr(t, err, "create failed")

		for i := 0; i < 10; i++ {
			_, err := mgr.NatsConn().Request(fmt.Sprintf("test.%d", i%3), []byte(fmt.Sprintf("message %d", i)), time.Second)
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
	})
}

func TestStream_ReadMessage(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test"))
		checkErr(t, err, "create failed")

		_, err = mgr.NatsConn().Request(stream.Subjects()[0], []byte("message 1"), time.Second)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		msg, err := stream.ReadMessage(1)
		checkErr(t, err, "load failed")
		if string(msg.Data) != "message 1" {
			t.Fatalf("expected message 1 got %q", msg.Data)
		}

		_, err = stream.ReadMessage(2)
		if err == nil {
			t.Fatalf("ReadMessage didnt fail on unknown message")
		}
	})
}

func TestStream_DeleteMessage(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		stream, err := mgr.NewStream("q1", jsm.FileStorage(), jsm.Subjects("test"))
		checkErr(t, err, "create failed")

		streamPublish(t, mgr.NatsConn(), stream.Subjects()[0], []byte("message 1"))

		_, err = stream.ReadMessage(1)
		checkErr(t, err, "load failed")

		err = stream.DeleteMessageRequest(api.JSApiMsgDeleteRequest{Seq: 1})
		checkErr(t, err, "delete failed")

		msg, err := stream.ReadMessage(1)
		if err == nil {
			t.Fatalf("deleted messages was loaded %q", msg.Data)
		}
	})
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

func TestPlacementPreferred(t *testing.T) {
	cfg := testStreamConfig()
	err := jsm.PlacementPreferredLeader("n1")(cfg)
	checkErr(t, err, "failed")
	if !cmp.Equal(cfg.Placement.Preferred, "n1") {
		t.Fatalf("expected 'n1'' got %v", &cfg.Placement.Preferred)
	}

	err = jsm.PlacementPreferredLeader("n2")(cfg)
	checkErr(t, err, "failed")
	if !cmp.Equal(cfg.Placement.Preferred, "n2") {
		t.Fatalf("expected 'n2' got %v", cfg.Placement.Preferred)
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

func TestStream_DiscardNewPerSubject(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		s1, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
		checkErr(t, err, "create failed")
		s2, err := mgr.NewStream("q2", jsm.FileStorage(), jsm.DiscardNewPerSubject(), jsm.MaxMessagesPerSubject(10))
		checkErr(t, err, "create failed")

		if s1.DiscardNewPerSubject() {
			t.Fatalf("Expected s1 to not be discard per subject")
		}
		if !s2.DiscardNewPerSubject() {
			t.Fatalf("Expected s2 to be discard per subject")
		}
	})
}

func TestStream_IsMirror(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
		checkErr(t, err, "create failed")
		s, err := mgr.NewStream("q2", jsm.FileStorage(), jsm.Mirror(&api.StreamSource{Name: "q1"}))
		checkErr(t, err, "create failed")

		if !s.IsMirror() {
			t.Fatalf("Expected a mirror")
		}
	})
}

func TestStream_IsSourced(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		_, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.FileStorage())
		checkErr(t, err, "create failed")
		_, err = mgr.NewStream("q2", jsm.Subjects("in.q2"), jsm.FileStorage())
		checkErr(t, err, "create failed")
		s, err := mgr.NewStream("q3", jsm.Subjects("in.q3"), jsm.FileStorage(), jsm.Sources(&api.StreamSource{Name: "q1"}, &api.StreamSource{Name: "q2"}))
		checkErr(t, err, "create failed")

		if !s.IsSourced() {
			t.Fatalf("Expected a synced")
		}
	})
}

func TestStreamDescription(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
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
	})
}

func TestStreamPageContents(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		s, err := mgr.NewStream("m1", jsm.Subjects("test"), jsm.WorkQueueRetention())
		checkErr(t, err, "create failed")

		pager, err := s.PageContents()
		if err.Error() != "work queue retention streams can only be paged if direct access is allowed" {
			t.Fatalf("Expected an error, got: %v", err)
		}
		if pager != nil {
			t.Fatalf("expected a nil pager")
		}

		err = s.UpdateConfiguration(s.Configuration(), jsm.AllowDirect())
		if err != nil {
			t.Fatalf("update failed: %v", err)
		}
		_, err = s.PageContents()
		if err != nil {
			t.Fatalf("paging direct enabled WQ failed: %v", err)
		}
	})
}

func TestFirstSequence(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		s, err := mgr.NewStream("m1", jsm.Subjects("test"), jsm.FirstSequence(1000))
		checkErr(t, err, "create failed")

		if s.FirstSequence() != 1000 {
			t.Fatalf("Expected first sequence to be 1000 got %v", s.FirstSequence())
		}
	})
}

func TestStreamSubjectDeleteMarkerTTL(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		s, err := mgr.NewStream("m1", jsm.Subjects("test"))
		checkErr(t, err, "create failed")

		if s.SubjectDeleteMarkerTTL() != 0 {
			t.Fatalf("Expected DeleteMarkerTTL to be 0 got %v", s.SubjectDeleteMarkerTTL())
		}

		err = s.Delete()
		checkErr(t, err, "delete failed")

		s, err = mgr.NewStream("m1", jsm.Subjects("test"), jsm.AllowMsgTTL(), jsm.SubjectDeleteMarkerTTL(time.Minute))
		checkErr(t, err, "create failed")

		if s.SubjectDeleteMarkerTTL() != time.Minute {
			t.Fatalf("Expected DeleteMarkerTTL to be 1 minute got %v", s.SubjectDeleteMarkerTTL())
		}
	})
}

func TestStreamSealed(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		s, err := mgr.NewStream("m1", jsm.Subjects("test"))
		checkErr(t, err, "create failed")

		_, err = mgr.NatsConn().Request("test", []byte("1"), time.Second)
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

		err = s.DeleteMessageRequest(api.JSApiMsgDeleteRequest{Seq: 1})
		if !jsm.IsNatsError(err, 10109) {
			t.Fatalf("expected err 10109 got %v", err)
		}
	})
}

func TestStream_ContainedSubjects(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		s, err := mgr.NewStream("m1", jsm.Subjects("test.>"))
		checkErr(t, err, "Create failed")

		nc := mgr.NatsConn()

		_, err = nc.Request("test.otherset.e1", []byte("1"), time.Second)
		checkErr(t, err, "Publish failed")
		_, err = nc.Request("test.set.e1", []byte("1"), time.Second)
		checkErr(t, err, "Publish failed")
		_, err = nc.Request("test.set.e2", []byte("1"), time.Second)
		checkErr(t, err, "Publish failed")
		_, err = nc.Request("test.set.e3", []byte("1"), time.Second)
		checkErr(t, err, "Publish failed")
		_, err = nc.Request("test.set.e4", []byte("1"), time.Second)
		checkErr(t, err, "Publish failed")

		subs, err := s.ContainedSubjects()
		checkErr(t, err, "contained failed")
		expected := map[string]uint64{
			"test.otherset.e1": 1,
			"test.set.e2":      1,
			"test.set.e1":      1,
			"test.set.e3":      1,
			"test.set.e4":      1,
		}
		if !reflect.DeepEqual(subs, expected) {
			t.Fatalf("Invalid set: %v", subs)
		}

		delete(expected, "test.otherset.e1")
		subs, err = s.ContainedSubjects("test.set.>")
		checkErr(t, err, "contained failed")
		if !reflect.DeepEqual(subs, expected) {
			t.Fatalf("Invalid set: %v", subs)
		}

		// Test disabled as it keeps timing out since it needs
		// 100001 subjects

		// js, err := nc.JetStream()
		// checkErr(t, err, "js failed")
		// for i := 0; i < 100001; i++ {
		// 	_, err = js.PublishAsync(fmt.Sprintf("test.new.e%d", i), []byte("1"))
		// 	checkErr(t, err, "Publish failed")
		// }
		// <-js.PublishAsyncComplete()
		//
		// subs, err = s.ContainedSubjects("test.new.>")
		// checkErr(t, err, "contained failed")
		// if len(subs) != 100001 {
		// 	t.Fatalf("Expected 100001 subjects got %d", len(subs))
		// }
	})
}

func TestStream_Compression(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		s, err := mgr.NewStream("f1", jsm.Subjects("test.*"), jsm.FileStorage(), jsm.Compression(api.S2Compression))
		checkErr(t, err, "create failed")

		nfo, err := s.Information()
		checkErr(t, err, "info failed")

		if nfo.Config.Compression != api.S2Compression {
			t.Fatalf("s2 compression was not set")
		}

		if !s.IsCompressed() {
			t.Fatalf("s2 compression was not reported correctly")
		}
	})
}

func TestStream_DetectGaps(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		s, err := mgr.NewStream("m1", jsm.Subjects("test.*"))
		checkErr(t, err, "create failed")

		for i := 1; i <= 3000; i++ {
			_, err := mgr.NatsConn().Request(fmt.Sprintf("test.%d", i), nil, time.Second)
			checkErr(t, err, "publish failed")
		}

		for _, seq := range []uint64{1, 3, 10, 11, 12, 20, 21, 22, 2000, 2001, 2002, 2005} {
			checkErr(t, s.DeleteMessageRequest(api.JSApiMsgDeleteRequest{Seq: seq}), "delete failed")
		}

		gaps := [][2]uint64{}
		progressCnt := 0

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = s.DetectGaps(ctx,
			func(_, pending uint64) { progressCnt++ },
			func(start uint64, end uint64) { gaps = append(gaps, [2]uint64{start, end}) },
		)
		checkErr(t, err, "detection failed")

		if len(gaps) != 5 {
			t.Fatalf("expected 5 gaps got %d", gaps)
		}

		if !cmp.Equal(gaps, [][2]uint64{{3, 3}, {10, 12}, {20, 22}, {2000, 2002}, {2005, 2005}}) {
			t.Fatalf("Invalid gaps detected: %v", gaps)
		}
	})
}

func TestStream_DirectGet(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		s, err := mgr.NewStream("m1", jsm.Subjects("test.*"), jsm.AllowDirect())
		checkErr(t, err, "create failed")

		for i := 1; i <= 1000; i++ {
			_, err = mgr.NatsConn().Request(fmt.Sprintf("test.%d", i%5), []byte(fmt.Sprintf("%d", i)), time.Second)
			checkErr(t, err, "publish failed")
		}

		check := func(t *testing.T, req api.JSApiMsgGetRequest, expectMatched, expectNumPending, expectLastSeq, expectUpToSeq uint64) {
			t.Helper()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			matched := uint64(0)
			numPending, lastSeq, upToSeq, err := s.DirectGet(ctx, req, func(m *nats.Msg) {
				matched++
			})
			checkErr(t, err, "Request failed")

			if matched != expectMatched {
				t.Fatalf("expected %d messages, got %d", expectMatched, matched)
			}
			if numPending != expectNumPending {
				t.Fatalf("expected numPending %d got %d", expectNumPending, numPending)
			}
			if lastSeq != expectLastSeq {
				t.Fatalf("expected lastSeq %d got %d", expectLastSeq, lastSeq)
			}
			if upToSeq != expectUpToSeq {
				t.Fatalf("expected upToSeq %d got %d", expectUpToSeq, upToSeq)
			}
		}

		cases := []struct {
			name                                  string
			request                               api.JSApiMsgGetRequest
			matched, numPending, lastSeq, upToSeq uint64
		}{
			{"Batch", api.JSApiMsgGetRequest{Batch: 100, Seq: 1}, 100, 0, 100, 0},
			{"Multi Batch", api.JSApiMsgGetRequest{Batch: 100, Seq: 1, MultiLastFor: []string{">"}}, 5, 0, 1000, 1000},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				check(t, tc.request, tc.matched, tc.numPending, tc.lastSeq, tc.upToSeq)
			})
		}
	})
}

func TestStreamRepublish(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		s, err := mgr.NewStream("m1", jsm.Subjects("test.*"), jsm.Republish(&api.RePublish{Source: "test.*", Destination: "repub.{{partition(2,1)}}"}))
		checkErr(t, err, "create failed")
		if s.Configuration().RePublish == nil {
			t.Fatalf("expected republish configuration")
		}

		nc := mgr.NatsConn()

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

		mu.Lock()
		defer mu.Unlock()
		if received[0] == 0 || received[1] == 0 {
			t.Fatalf("Expected 2 mapped subjects to get messages: %#v", received)
		}
	})
}

func TestStreamPedantic(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		s, err := mgr.NewStreamFromDefault("TEST", api.StreamConfig{}, jsm.Subjects("test.*"), jsm.MaxAge(time.Second))
		checkErr(t, err, "create failed")

		if s.DuplicateWindow() != time.Second {
			t.Fatalf("expected duplicate window to be overrode, got: %v", s.DuplicateWindow())
		}
		checkErr(t, s.Delete(), "delete failed")

		mgr, err = jsm.New(mgr.NatsConn(), jsm.WithPedanticRequests())
		checkErr(t, err, "manager failed")
		if !mgr.IsPedantic() {
			t.Fatalf("expected mgr to be pedantic")
		}

		_, err = mgr.NewStreamFromDefault("TEST", api.StreamConfig{}, jsm.Subjects("test.*"), jsm.MaxAge(time.Second))
		if !api.IsNatsErr(err, 10157) {
			t.Fatalf("expected pednatic error, got: %v", err)
		}
	})
}

func TestStreamPedanticMirrorDirect(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, _ *testapi.ManagedServer) {
		s, err := mgr.NewStreamFromDefault("TEST", api.StreamConfig{}, jsm.Subjects("test.*"), jsm.NoAllowDirect())
		checkErr(t, err, "create failed")
		if s.DirectAllowed() {
			t.Fatalf("expected direct to be false")
		}

		m, err := mgr.NewStreamFromDefault("TEST_MIRROR", api.StreamConfig{}, jsm.MirrorDirect(), jsm.AllowDirect(), jsm.Mirror(&api.StreamSource{Name: "TEST"}))
		checkErr(t, err, "create failed")
		if m.MirrorDirectAllowed() {
			t.Fatalf("mirror direct was allowed")
		}

		checkErr(t, s.Delete(), "delete failed")
		checkErr(t, m.Delete(), "delete failed")

		mgr, err = jsm.New(mgr.NatsConn(), jsm.WithPedanticRequests())
		checkErr(t, err, "manager failed")
		if !mgr.IsPedantic() {
			t.Fatalf("expected mgr to be pedantic")
		}

		s, err = mgr.NewStreamFromDefault("TEST", api.StreamConfig{}, jsm.Subjects("test.*"), jsm.NoAllowDirect())
		checkErr(t, err, "create failed")
		if s.DirectAllowed() {
			t.Fatalf("expected direct to be false")
		}

		_, err = mgr.NewStreamFromDefault("TEST_MIRROR", api.StreamConfig{}, jsm.MirrorDirect(), jsm.AllowDirect(), jsm.Mirror(&api.StreamSource{Name: "TEST"}))
		if !api.IsNatsErr(err, 10157) {
			t.Fatalf("expected pednatic error, got: %v", err)
		}
	})
}
