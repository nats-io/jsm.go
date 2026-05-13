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
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/internal/testclient/testservice"
	natsd "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func testNetworkClient(t *testing.T) *testservice.Client {
	t.Helper()

	tester := os.Getenv("TESTER_NATS_URL")
	if os.Getenv("TESTER_NATS_URL") == "" {
		tester = nats.DefaultURL
	}

	return testservice.New(t, tester)
}

func withJSServer(t *testing.T, cb func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager)) {
	t.Helper()

	client := testNetworkClient(t)
	client.WithJetStreamServer(t, func(t *testing.T, nc *nats.Conn, instance *testservice.Instance) {
		mgr, err := jsm.New(nc, jsm.WithTimeout(time.Second))
		if err != nil {
			t.Fatalf("manager failed: %v", err)
		}

		cb(t, nc, mgr)
	})
}

func withJSCluster(t *testing.T, cb func(*testing.T, *nats.Conn, *jsm.Manager)) {
	t.Helper()

	client := testNetworkClient(t)
	client.WithJetStreamCluster(t, 3, func(t *testing.T, nc *nats.Conn, instance *testservice.Instance) {
		mgr, err := jsm.New(nc, jsm.WithTimeout(time.Second))
		if err != nil {
			t.Fatalf("manager failed: %v", err)
		}

		cb(t, nc, mgr)
	})
}

func withNatsServerWithConfig(t *testing.T, cfile string, cb func(*testing.T, *natsd.Server)) {
	t.Helper()

	d, err := os.MkdirTemp("", "jstest")
	if err != nil {
		t.Fatalf("temp dir could not be made: %s", err)
	}
	defer os.RemoveAll(d)

	af, err := filepath.Abs(cfile)
	if err != nil {
		t.Fatalf("absolute path failed: %v", err)
	}

	opts, err := natsd.ProcessConfigFile(af)
	if err != nil {
		t.Fatalf("config file failed: %v", err)
	}

	opts.StoreDir = d
	opts.Port = -1
	opts.Host = "localhost"
	opts.LogFile = "/dev/stdout"
	opts.Trace = true

	s, err := natsd.NewServer(opts)
	if err != nil {
		t.Fatal("server start failed: ", err)
	}

	go s.Start()
	if !s.ReadyForConnections(10 * time.Second) {
		t.Error("nats server did not start")
	}

	cb(t, s)
}

func streamPublish(t *testing.T, nc *nats.Conn, subj string, msg []byte) {
	_, err := nc.Request(subj, msg, time.Second)
	checkErr(t, err, "publish failed")
}

func TestIsStreamBytesRequired(t *testing.T) {
	withNatsServerWithConfig(t, "testdata/bytes_required.cfg", func(t *testing.T, srv *natsd.Server) {
		cases := []struct {
			user     string
			required bool
		}{
			{"other", false},
			{"a", true},
		}

		for _, tc := range cases {
			t.Run(fmt.Sprintf("User_%s", tc.user), func(t *testing.T) {
				nc, err := nats.Connect(srv.ClientURL(), nats.UserInfo(tc.user, "b"))
				if err != nil {
					t.Fatalf("connection failed: %v", err)
				}

				mgr, _ := jsm.New(nc)

				required, err := mgr.IsStreamMaxBytesRequired()
				if err != nil {
					t.Fatalf("failed: %v", err)
				}
				if required != tc.required {
					t.Fatalf("Expected it to be %t got %t", tc.required, required)
				}
			})
		}
	})
}

func TestJetStreamEnabled(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager) {
		if !mgr.IsJetStreamEnabled() {
			t.Fatalf("expected JS to be enabled")
		}
	})
}

func TestDeleteStream(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager) {
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager) {
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager) {
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager) {
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager) {
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager) {
		numStreams := 2500
		for i := 0; i < numStreams; i++ {
			_, err := mgr.NewStreamFromDefault(fmt.Sprintf("ORDERS_%d", i), jsm.DefaultStream, jsm.Subjects(fmt.Sprintf("ORDERS_%d.>", i)), jsm.MemoryStorage())
			checkErr(t, err, "create failed")
		}

		streams, _, _, err := mgr.Streams(nil)
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager) {
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager) {
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
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager) {
		orders, err := mgr.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.Subjects("ORDERS.*"), jsm.MemoryStorage())
		checkErr(t, err, "create failed")

		_, err = mgr.NewStreamFromDefault("ARCHIVE", orders.Configuration(), jsm.Subjects("OTHER"))
		checkErr(t, err, "create failed")

		var seen []string
		_, _, err = mgr.EachStream(nil, func(s *jsm.Stream) {
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
		_, _, err = mgr.EachStream(&jsm.StreamNamesFilter{Subject: "ORDERS.*"}, func(s *jsm.Stream) {
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

func TestNewOptions(t *testing.T) {
	withJSServer(t, func(t *testing.T, nc *nats.Conn, mgr *jsm.Manager) {
		// valid domain and prefixes
		_, err := jsm.New(nc, jsm.WithDomain("valid"))
		checkErr(t, err, "valid domain should be accepted")

		_, err = jsm.New(nc, jsm.WithAPIPrefix("js.foreign"))
		checkErr(t, err, "valid API prefix should be accepted")

		_, err = jsm.New(nc, jsm.WithEventPrefix("js.foreign"))
		checkErr(t, err, "valid event prefix should be accepted")

		// invalid: standalone wildcards, empty tokens
		for _, bad := range []string{">", "*", "foo.*", "foo.>", "foo..bar"} {
			_, err = jsm.New(nc, jsm.WithDomain(bad))
			if err == nil {
				t.Fatalf("expected error for domain %q, got nil", bad)
			}

			_, err = jsm.New(nc, jsm.WithAPIPrefix(bad))
			if err == nil {
				t.Fatalf("expected error for API prefix %q, got nil", bad)
			}

			_, err = jsm.New(nc, jsm.WithEventPrefix(bad))
			if err == nil {
				t.Fatalf("expected error for event prefix %q, got nil", bad)
			}
		}

		// domain and API prefix together are incompatible
		_, err = jsm.New(nc, jsm.WithDomain("valid"), jsm.WithAPIPrefix("js.foreign"))
		if err == nil {
			t.Fatal("expected error when both domain and API prefix are set, got nil")
		}
	})
}
