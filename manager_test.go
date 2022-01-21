package jsm_test

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	natsd "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func startJSServer(t *testing.T) (*natsd.Server, *nats.Conn, *jsm.Manager) {
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
		LogFile:   "/dev/stdout",
		Trace:     true,
	}

	s, err := natsd.NewServer(opts)
	if err != nil {
		t.Fatal("server start failed: ", err)
	}

	go s.Start()
	if !s.ReadyForConnections(10 * time.Second) {
		t.Error("nats server did not start")
	}

	nc, err := nats.Connect(s.ClientURL(), nats.UseOldRequestStyle())
	if err != nil {
		t.Fatalf("client start failed: %s", err)
	}

	mgr, err := jsm.New(nc, jsm.WithTimeout(time.Second))
	if err != nil {
		t.Fatalf("manager creation failed: %s", err)
	}

	return s, nc, mgr
}

func TestJetStreamEnabled(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

	if !mgr.IsJetStreamEnabled() {
		t.Fatalf("expected JS to be enabled")
	}
}

func TestIsKnownStream(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

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
}

func TestIsKnownConsumer(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

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
}

func TestJetStreamAccountInfo(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

	_, err := mgr.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.Subjects("ORDERS.*"), jsm.MemoryStorage())
	checkErr(t, err, "create failed")

	info, err := mgr.JetStreamAccountInfo()
	checkErr(t, err, "info fetch failed")

	if info.Streams != 1 {
		t.Fatalf("received %d message sets expected 1", info.Streams)
	}
}

func TestStreams(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

	numStreams := 2500
	for i := 0; i < numStreams; i++ {
		_, err := mgr.NewStreamFromDefault(fmt.Sprintf("ORDERS_%d", i), jsm.DefaultStream, jsm.Subjects(fmt.Sprintf("ORDERS_%d.>", i)), jsm.MemoryStorage())
		checkErr(t, err, "create failed")
	}

	streams, err := mgr.Streams()
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
}

func TestStreamNames(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

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
}

func TestConsumerNames(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

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
}

func TestEachStream(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

	orders, err := mgr.NewStreamFromDefault("ORDERS", jsm.DefaultStream, jsm.Subjects("ORDERS.*"), jsm.MemoryStorage())
	checkErr(t, err, "create failed")

	_, err = mgr.NewStreamFromDefault("ARCHIVE", orders.Configuration(), jsm.Subjects("OTHER"))
	checkErr(t, err, "create failed")

	seen := []string{}
	err = mgr.EachStream(func(s *jsm.Stream) {
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
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

	exists, err := mgr.IsKnownStreamTemplate("orders_templ")
	checkErr(t, err, "is known failed")

	if exists {
		t.Fatalf("found orders_templ when it shouldnt have")
	}

	_, err = mgr.NewStreamTemplate("orders_templ", 1, jsm.DefaultStream, jsm.FileStorage(), jsm.Subjects("ORDERS.*"))
	checkErr(t, err, "new stream template failed")

	exists, err = mgr.IsKnownStreamTemplate("orders_templ")
	checkErr(t, err, "is known failed")

	if !exists {
		t.Fatalf("did not find orders_templ when it should have")
	}
}
