package jsm_test

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go"
)

func TestManager_Publish(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer os.RemoveAll(srv.JetStreamConfig().StoreDir)
	defer srv.Shutdown()
	defer nc.Close()

	_, err := mgr.NewStream("ORDERS", jsm.MaxAge(time.Hour), jsm.FileStorage(), jsm.Subjects("ORDERS.*"))
	checkErr(t, err, "stream create failed")

	// publish and create expect any
	res, err := mgr.Publish("ORDERS.new", []byte("order 1"), jsm.PublishExpectStream("ORDERS"))

	checkErr(t, err, "publish failed")
	if res.Stream != "ORDERS" {
		t.Fatalf("expected stream ORDERS got %#v", res.PubAck)
	}
	if res.Seq != 1 {
		t.Fatalf("expected sequence 1 got %#v", res.PubAck)
	}
	if res.Duplicate {
		t.Fatalf("expected non dupe message got %#v", res.PubAck)
	}

	str, err := mgr.LoadStream("ORDERS")
	checkErr(t, err, "failed to load ORDERS")
	if str.Configuration().Subjects[0] != "ORDERS.*" {
		t.Fatalf("invalid subjeects on ORDERS")
	}

	// publish and create expect ORDERS while existing already
	res, err = mgr.Publish("ORDERS.new", []byte("order 2"), jsm.PublishExpectStream("ORDERS"))
	checkErr(t, err, "publish failed")
	if res.Seq != 2 {
		t.Fatalf("expected sequence 2 got %#v", res.PubAck)
	}

	msg := &nats.Msg{Subject: "ORDERS.new", Data: []byte("order 3"), Header: http.Header{}}
	msg.Header.Set("Hello", "World")
	res, err = mgr.PublishMsg(msg, jsm.PublishExpectStream("ORDERS"))
	checkErr(t, err, "publish failed")
	if res.Seq != 3 {
		t.Fatalf("expected sequence 3 got %#v", res.PubAck)
	}

	smsg, err := str.ReadMessage(3)
	checkErr(t, err, "read msg failed")
	if len(smsg.Header) == 0 {
		t.Fatalf("header lost")
	}
	if string(smsg.Data) != "order 3" {
		t.Fatalf("data lost: %q", smsg.Data)
	}
}
