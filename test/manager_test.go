// Copyright 2021-2022 The NATS Authors
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
	"os"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	testapi "github.com/nats-io/jsm.go/test/testing_client/api"
	"github.com/nats-io/jsm.go/test/testing_client/srvtest"
	"github.com/nats-io/nats.go"
)

func withTesterJetStreamCluster(t *testing.T, fn func(*testing.T, *jsm.Manager, *srvtest.Client, []*testapi.ManagedServer)) {
	t.Helper()

	url := os.Getenv("TESTER_URL")
	if url == "" {
		url = "nats://localhost:4222"
	}

	client := srvtest.New(t, url)
	t.Cleanup(func() {
		client.Reset(t)
	})

	client.WithJetStreamCluster(t, 3, func(t *testing.T, nc *nats.Conn, servers []*testapi.ManagedServer) {
		mgr, err := jsm.New(nc)
		if err != nil {
			t.Fatal(err)
		}

		fn(t, mgr, client, servers)
	})
}

func streamPublish(t *testing.T, nc *nats.Conn, subj string, msg []byte) {
	_, err := nc.Request(subj, msg, time.Second)
	checkErr(t, err, "publish failed")
}

func withJsServer(t *testing.T, fn func(*testing.T, *jsm.Manager, *srvtest.Client, *testapi.ManagedServer)) {
	t.Helper()

	url := os.Getenv("TESTER_URL")
	if url == "" {
		url = "nats://localhost:4222"
	}

	client := srvtest.New(t, url)
	t.Cleanup(func() {
		client.Reset(t)
	})

	client.WithJetStreamServer(t, func(t *testing.T, nc *nats.Conn, server *testapi.ManagedServer) {
		url := nc.ConnectedUrl()
		nc.Close()
		nc, err := nats.Connect(url, nats.MaxReconnects(-1), nats.UseOldRequestStyle())
		if err != nil {
			t.Fatal(err)
		}

		mgr, err := jsm.New(nc, jsm.WithTimeout(time.Second))
		if err != nil {
			t.Fatal(err)
		}

		fn(t, mgr, client, server)
		nc.Close()
	})
}

func TestJetStreamEnabled(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, server *testapi.ManagedServer) {
		if !mgr.IsJetStreamEnabled() {
			t.Fatalf("expected JS to be enabled")
		}
	})
}

func TestDeleteStream(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, server *testapi.ManagedServer) {
		_, err := mgr.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.Subjects("ORDERS.*"), jsm.MemoryStorage())
		checkErr(t, err, "create failed")

		known, err := mgr.IsKnownStream("ORDERS")
		checkErr(t, err, "known lookup failed")
		if !known {
			t.Fatalf("ORDERS should be known")
		}

		err = mgr.DeleteStream("ORDERS")
		checkErr(t, err, "delete failed")

		known, err = mgr.IsKnownStream("ORDERS")
		checkErr(t, err, "known lookup failed")
		if known {
			t.Fatalf("ORDERS should not be known")
		}
	})
}

func TestDeleteConsumer(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, server *testapi.ManagedServer) {
		stream, err := mgr.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.Subjects("ORDERS.*"), jsm.MemoryStorage())
		checkErr(t, err, "create failed")

		known, err := mgr.IsKnownStream("ORDERS")
		checkErr(t, err, "known lookup failed")
		if !known {
			t.Fatalf("ORDERS should be known")
		}

		_, err = stream.NewConsumer(jsm.DurableName("DURABLE"))
		checkErr(t, err, "create failed")

		names, err := stream.ConsumerNames()
		checkErr(t, err, "names failed")
		if len(names) != 1 {
			t.Fatalf("Create failed")
		}

		err = mgr.DeleteConsumer("ORDERS", "DURABLE")
		checkErr(t, err, "delete failed")

		names, err = stream.ConsumerNames()
		checkErr(t, err, "names failed")
		if len(names) != 0 {
			t.Fatalf("Delete failed")
		}
	})
}

func TestIsKnownStream(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, server *testapi.ManagedServer) {
		known, err := mgr.IsKnownStream("ORDERS")
		checkErr(t, err, "known lookup failed")
		if known {
			t.Fatalf("ORDERS should not be known")
		}

		stream, err := mgr.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.Subjects("ORDERS.*"), jsm.MemoryStorage())
		checkErr(t, err, "create failed")

		known, err = mgr.IsKnownStream("ORDERS")
		checkErr(t, err, "known lookup failed")
		if !known {
			t.Fatalf("ORDERS should be known")
		}

		stream.Reset()
		if stream.Storage() != api.MemoryStorage {
			t.Fatalf("ORDERS is not memory storage")
		}
	})
}

func TestIsKnownConsumer(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, server *testapi.ManagedServer) {
		stream, err := mgr.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.Subjects("ORDERS.*"), jsm.MemoryStorage())
		checkErr(t, err, "create failed")

		known, err := mgr.IsKnownConsumer("ORDERS", "NEW")
		checkErr(t, err, "known lookup failed")
		if known {
			t.Fatalf("NEW should not exist")
		}

		_, err = stream.NewConsumerFromDefault(jsm.DefaultConsumer, jsm.DurableName("NEW"))
		checkErr(t, err, "create failed")

		known, err = mgr.IsKnownConsumer("ORDERS", "NEW")
		checkErr(t, err, "known lookup failed")

		if !known {
			t.Fatalf("NEW does not exist")
		}
	})
}

func TestJetStreamAccountInfo(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, server *testapi.ManagedServer) {
		_, err := mgr.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.Subjects("ORDERS.*"), jsm.MemoryStorage())
		checkErr(t, err, "create failed")

		info, err := mgr.JetStreamAccountInfo()
		checkErr(t, err, "info fetch failed")

		if info.Streams != 1 {
			t.Fatalf("received %d message sets expected 1", info.Streams)
		}
	})
}

func TestStreams(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, server *testapi.ManagedServer) {
		numStreams := 2500
		for i := 0; i < numStreams; i++ {
			_, err := mgr.NewStreamFromDefault(fmt.Sprintf("ORDERS_%d", i), jsm.DefaultStream, jsm.Subjects(fmt.Sprintf("ORDERS_%d.>", i)), jsm.MemoryStorage())
			checkErr(t, err, "create failed")
		}

		streams, _, err := mgr.Streams(nil)
		checkErr(t, err, "streams failed")
		if len(streams) != numStreams {
			t.Fatalf("expected %d orders got %d", numStreams, len(streams))
		}

		names := map[string]struct{}{}
		for _, s := range streams {
			_, ok := names[s.Name()]
			if ok {
				t.Fatalf("Duplicate record for %s", s.Name())
			}

			names[s.Name()] = struct{}{}
		}
	})
}

func TestStreamNames(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, server *testapi.ManagedServer) {
		names, err := mgr.StreamNames(nil)
		checkErr(t, err, "lookup failed")

		if len(names) > 0 {
			t.Fatalf("expected 0 streams got: %v", names)
		}

		numStreams := 2500
		for i := 0; i < numStreams; i++ {
			_, err = mgr.NewStreamFromDefault(fmt.Sprintf("ORDERS_%d", i), jsm.DefaultStream, jsm.Subjects(fmt.Sprintf("ORDERS_%d.>", i)), jsm.MemoryStorage())
			checkErr(t, err, "create failed")
		}

		names, err = mgr.StreamNames(nil)
		checkErr(t, err, "lookup failed")

		if len(names) != numStreams || names[0] != "ORDERS_0" || names[numStreams-1] != "ORDERS_999" {
			t.Fatalf("expected %d orders got %d", numStreams, len(names))
		}

		unames := map[string]struct{}{}
		for _, s := range names {
			_, ok := unames[s]
			if ok {
				t.Fatalf("Duplicate received for %s", s)
			}
			unames[s] = struct{}{}
		}

		names, err = mgr.StreamNames(&jsm.StreamNamesFilter{Subject: ">"})
		checkErr(t, err, "names failed")
		if len(names) != numStreams {
			t.Fatalf("expected %d streams got %d", numStreams, len(names))
		}

		names, err = mgr.StreamNames(&jsm.StreamNamesFilter{Subject: "ORDERS_10.foo"})
		checkErr(t, err, "names failed")
		if len(names) != 1 {
			t.Fatalf("expected 1 stream got %d", len(names))
		}

		names, err = mgr.StreamNames(&jsm.StreamNamesFilter{Subject: "none.foo"})
		checkErr(t, err, "names failed")
		if len(names) != 0 {
			t.Fatalf("expected 0 streams got %d", len(names))
		}
	})
}

func TestConsumerNames(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, server *testapi.ManagedServer) {
		_, err := mgr.ConsumerNames("ORDERS")
		if err == nil {
			t.Fatalf("expected err")
		}

		stream, err := mgr.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.Subjects("ORDERS.*"), jsm.MemoryStorage())
		checkErr(t, err, "create failed")

		_, err = mgr.ConsumerNames("ORDERS")
		checkErr(t, err, "lookup failed")

		_, err = stream.NewConsumerFromDefault(jsm.DefaultConsumer, jsm.DurableName("NEW"))
		checkErr(t, err, "create failed")

		names, err := mgr.ConsumerNames("ORDERS")
		checkErr(t, err, "lookup failed")

		if len(names) != 1 || names[0] != "NEW" {
			t.Fatalf("expected [NEW] got %v", names)
		}
	})
}

func TestEachStream(t *testing.T) {
	withJsServer(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, server *testapi.ManagedServer) {
		orders, err := mgr.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.Subjects("ORDERS.*"), jsm.MemoryStorage())
		checkErr(t, err, "create failed")

		_, err = mgr.NewStreamFromDefault("ARCHIVE", orders.Configuration(), jsm.Subjects("OTHER"))
		checkErr(t, err, "create failed")

		var seen []string
		_, err = mgr.EachStream(nil, func(s *jsm.Stream) {
			seen = append(seen, s.Name())
		})
		checkErr(t, err, "iteration failed")

		if len(seen) != 2 {
			t.Fatalf("expected 2 got %d", len(seen))
		}

		if seen[0] != "ARCHIVE" || seen[1] != "ORDERS" {
			t.Fatalf("incorrect streams or order, expected [ARCHIVE, ORDERS] got %v", seen)
		}

		seen = []string{}
		_, err = mgr.EachStream(&jsm.StreamNamesFilter{Subject: "ORDERS.*"}, func(s *jsm.Stream) {
			seen = append(seen, s.Name())
		})
		checkErr(t, err, "iteration failed")
		if len(seen) != 1 {
			t.Fatalf("expected 1 got %d", len(seen))
		}
		if seen[0] != "ORDERS" {
			t.Fatalf("incorrect streams or order, expected [ORDERS] got %v", seen)
		}
	})
}
