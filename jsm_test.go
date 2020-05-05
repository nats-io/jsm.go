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
	"io/ioutil"
	"testing"
	"time"

	natsd "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"

	"github.com/nats-io/jsm.go"
)

func startJSServer(t *testing.T) (*natsd.Server, *nats.Conn) {
	t.Helper()

	d, err := ioutil.TempDir("", "jstest")
	if err != nil {
		t.Fatalf("temp dir could not be made: %s", err)
	}

	opts := &natsd.Options{
		JetStream: true,
		StoreDir:  d,
		Port:      -1,
		Host:      "localhost",
	}

	s, err := natsd.NewServer(opts)
	if err != nil {
		t.Fatal("server start failed: ", err)
	}

	go s.Start()
	if !s.ReadyForConnections(10 * time.Second) {
		t.Error("nats server did not start")
	}

	// nc, err := nats.Connect("localhost")
	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatalf("client start failed: %s", err)
	}

	jsm.SetConnection(nc)

	return s, nc
}

func checkErr(t *testing.T, err error, m string) {
	t.Helper()
	if err == nil {
		return
	}
	t.Fatal(m + ": " + err.Error())
}

func TestJetStreamEnabled(t *testing.T) {
	srv, nc := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	if !jsm.IsJetStreamEnabled(jsm.WithTimeout(time.Second)) {
		t.Fatalf("expected JS to be enabled")
	}
}

func TestIsErrorResponse(t *testing.T) {
	if jsm.IsErrorResponse(&nats.Msg{Data: []byte("+OK")}) {
		t.Fatalf("OK is Error")
	}

	if !jsm.IsErrorResponse(&nats.Msg{Data: []byte("-ERR 'error'")}) {
		t.Fatalf("ERR is not Error")
	}
}

func TestParseErrorResponse(t *testing.T) {
	checkErr(t, jsm.ParseErrorResponse(&nats.Msg{Data: []byte("+OK")}), "expected nil got error")

	err := jsm.ParseErrorResponse(&nats.Msg{Data: []byte("-ERR 'test error")})
	if err == nil {
		t.Fatalf("expected an error got nil")
	}

	if err.Error() != "test error" {
		t.Fatalf("expected 'test error' got '%v'", err)
	}
}

func TestIsOKResponse(t *testing.T) {
	if !jsm.IsOKResponse(&nats.Msg{Data: []byte("+OK")}) {
		t.Fatalf("OK is Error")
	}

	if jsm.IsOKResponse(&nats.Msg{Data: []byte("-ERR error")}) {
		t.Fatalf("ERR is not Error")
	}
}

func TestIsKnownStream(t *testing.T) {
	srv, nc := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	known, err := jsm.IsKnownStream("ORDERS")
	checkErr(t, err, "known lookup failed")
	if known {
		t.Fatalf("ORDERS should not be known")
	}

	stream, err := jsm.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.MemoryStorage())
	checkErr(t, err, "create failed")

	known, err = jsm.IsKnownStream("ORDERS")
	checkErr(t, err, "known lookup failed")
	if !known {
		t.Fatalf("ORDERS should be known")
	}

	stream.Reset()
	if stream.Storage() != api.MemoryStorage {
		t.Fatalf("ORDERS is not memory storage")
	}
}

func TestIsKnownConsumer(t *testing.T) {
	srv, nc := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	stream, err := jsm.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.MemoryStorage())
	checkErr(t, err, "create failed")

	known, err := jsm.IsKnownConsumer("ORDERS", "NEW")
	checkErr(t, err, "known lookup failed")
	if known {
		t.Fatalf("NEW should not exist")
	}

	_, err = stream.NewConsumerFromDefault(jsm.DefaultConsumer, jsm.DurableName("NEW"))
	checkErr(t, err, "create failed")

	known, err = jsm.IsKnownConsumer("ORDERS", "NEW")
	checkErr(t, err, "known lookup failed")

	if !known {
		t.Fatalf("NEW does not exist")
	}
}

func TestJetStreamAccountInfo(t *testing.T) {
	srv, nc := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	_, err := jsm.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.MemoryStorage())
	checkErr(t, err, "create failed")

	info, err := jsm.JetStreamAccountInfo()
	checkErr(t, err, "info fetch failed")

	if info.Streams != 1 {
		t.Fatalf("received %d message sets expected 1", info.Streams)
	}
}

func TestStreamNames(t *testing.T) {
	srv, nc := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	names, err := jsm.StreamNames()
	checkErr(t, err, "lookup failed")

	if len(names) > 0 {
		t.Fatalf("expected 0 streams got: %v", names)
	}

	for i := 0; i < 510; i++ {
		_, err = jsm.NewStreamFromDefault(fmt.Sprintf("ORDERS_%d", i), jsm.DefaultStream, jsm.MemoryStorage())
		checkErr(t, err, "create failed")
	}

	names, err = jsm.StreamNames()
	checkErr(t, err, "lookup failed")

	if len(names) != 510 || names[0] != "ORDERS_0" && names[509] != "ORDERS_99" {
		t.Fatalf("expected 510 orders got %d", len(names))
	}
}

func TestConsumerNames(t *testing.T) {
	srv, nc := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	_, err := jsm.ConsumerNames("ORDERS")
	if err == nil {
		t.Fatalf("expected err")
	}

	stream, err := jsm.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.MemoryStorage())
	checkErr(t, err, "create failed")

	_, err = jsm.ConsumerNames("ORDERS")
	checkErr(t, err, "lookup failed")

	_, err = stream.NewConsumerFromDefault(jsm.DefaultConsumer, jsm.DurableName("NEW"))
	checkErr(t, err, "create failed")

	names, err := jsm.ConsumerNames("ORDERS")
	checkErr(t, err, "lookup failed")

	if len(names) != 1 || names[0] != "NEW" {
		t.Fatalf("expected [NEW] got %v", names)
	}
}

func TestEachStream(t *testing.T) {
	srv, nc := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	orders, err := jsm.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.MemoryStorage())
	checkErr(t, err, "create failed")

	_, err = jsm.NewStreamFromDefault("ARCHIVE", orders.Configuration(), jsm.Subjects("OTHER"))
	checkErr(t, err, "create failed")

	seen := []string{}
	err = jsm.EachStream(func(s *jsm.Stream) {
		seen = append(seen, s.Name())
	})
	checkErr(t, err, "iteration failed")

	if len(seen) != 2 {
		t.Fatalf("expected 2 got %d", len(seen))
	}

	if seen[0] != "ARCHIVE" || seen[1] != "ORDERS" {
		t.Fatalf("incorrect streams or order, expected [ARCHIVE, ORDERS] got %v", seen)
	}
}

func TestIsKnownStreamTemplate(t *testing.T) {
	srv, _ := startJSServer(t)
	defer srv.Shutdown()

	exists, err := jsm.IsKnownStreamTemplate("orders_templ")
	checkErr(t, err, "is known failed")

	if exists {
		t.Fatalf("found orders_templ when it shouldnt have")
	}

	_, err = jsm.NewStreamTemplate("orders_templ", 1, jsm.DefaultStream, jsm.FileStorage(), jsm.Subjects("ORDERS.*"))
	checkErr(t, err, "new stream template failed")

	exists, err = jsm.IsKnownStreamTemplate("orders_templ")
	checkErr(t, err, "is known failed")

	if !exists {
		t.Fatalf("did not find orders_templ when it should have")
	}
}
