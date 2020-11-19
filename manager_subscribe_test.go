package jsm_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	natsd "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go"
)

func TestManager_Subscribe_Durable(t *testing.T) {
	// var orders *jsm.Stream
	var err error

	prep := func(t *testing.T) (*natsd.Server, *nats.Conn, *jsm.Manager) {
		t.Helper()
		srv, nc, mgr := startJSServer(t)
		_, err = mgr.NewStream("ORDERS", jsm.MaxAge(time.Hour), jsm.FileStorage(), jsm.Subjects("ORDERS.*"))
		checkErr(t, err, "stream create failed")
		for i := 0; i < 50; i++ {
			_, err = mgr.Publish("ORDERS.new", []byte(fmt.Sprintf("order %d", i)), jsm.PublishExpectStream())
			checkErr(t, err, "publish failed")
		}
		return srv, nc, mgr
	}

	// sync subscribes where the manager creates an inbox and manage it internally
	t.Run("Sync/HiddenInbox", func(t *testing.T) {
		srv, nc, mgr := prep(t)
		defer os.RemoveAll(srv.JetStreamConfig().StoreDir)
		defer srv.Shutdown()
		defer nc.Close()

		t.Run("Create", func(t *testing.T) {
			sub, err := mgr.SubscribeSync("ORDERS.new", jsm.SubscribeDurable("HIDDEN"))
			checkErr(t, err, "subscribe failed")
			defer sub.Unsubscribe()

			if !sub.Consumer().IsDurable() {
				t.Fatalf("expected HIDDEN to be durable")
			}
			if sub.Consumer().DurableName() != "HIDDEN" {
				t.Fatalf("invalid durable name %q", sub.Consumer().DurableName())
			}
			if sub.Consumer().IsPushMode() {
				t.Fatalf("expected a pull mode consumer")
			}

			for i := 0; i < 5; i++ {
				msg, err := sub.NextMsg()
				checkErr(t, err, "NextMsg failed")

				if string(msg.Data) != fmt.Sprintf("order %d", i) {
					t.Fatalf("expected 'order 1' got %q", msg.Data)
				}

				msg.Ack(nats.AckWaitDuration(time.Second))
			}
		})

		t.Run("Join", func(t *testing.T) {
			_, err := mgr.LoadConsumer("ORDERS", "HIDDEN")
			checkErr(t, err, "load failed")

			sub, err := mgr.SubscribeSync("ORDERS.new", jsm.SubscribeDurable("HIDDEN"))
			checkErr(t, err, "subscribe failed")
			defer sub.Unsubscribe()

			if sub.Consumer().IsPushMode() {
				t.Fatalf("expected a pull consumer")
			}

			for i := 0; i < 5; i++ {
				msg, err := sub.NextMsg()
				checkErr(t, err, "NextMsg failed")

				if string(msg.Data) != fmt.Sprintf("order %d", i+5) {
					t.Fatalf("expected 'order 1' got %q", msg.Data)
				}

				msg.Ack(nats.AckWaitDuration(time.Second))
			}
		})
	})

	// async subscribes where the manager create an inbox and manage it internally
	t.Run("Async/HiddenInbox", func(t *testing.T) {
		srv, nc, mgr := prep(t)
		defer os.RemoveAll(srv.JetStreamConfig().StoreDir)
		defer srv.Shutdown()
		defer nc.Close()

		t.Run("Create", func(t *testing.T) {
			cnt := 0
			done := make(chan struct{})

			sub, err := mgr.Subscribe("ORDERS.new", jsm.SubscribeDurable("HIDDEN"), jsm.SubscribeConsumer(jsm.MaxAckPending(1)), jsm.SubscribeHandler(func(msg *nats.Msg) {
				if string(msg.Data) != fmt.Sprintf("order %d", cnt) {
					t.Fatalf("expected 'order %d' got %q", cnt, msg.Data)
				}

				msg.Ack(nats.AckWaitDuration(time.Second))
				cnt++

				if cnt == 6 {
					close(done)
				}
			}))
			checkErr(t, err, "subscribe failed")
			defer sub.Unsubscribe()

			if !sub.Consumer().IsDurable() {
				t.Fatalf("expected HIDDEN to be durable")
			}
			if sub.Consumer().DurableName() != "HIDDEN" {
				t.Fatalf("invalid durable name %q", sub.Consumer().DurableName())
			}
			if sub.Consumer().IsPullMode() {
				t.Fatalf("expected a push mode consumer")
			}

			select {
			case <-done:
			case <-time.NewTimer(time.Second).C:
				t.Fatalf("timeout")
			}

		})

		t.Run("Join", func(t *testing.T) {
			c, err := mgr.LoadConsumer("ORDERS", "HIDDEN")
			checkErr(t, err, "load failed")

			cnt := 7
			done := make(chan struct{})

			sub, err := mgr.Subscribe("ORDERS.new", jsm.SubscribeDurable("HIDDEN"), jsm.SubscribeConsumer(jsm.MaxAckPending(1)), jsm.SubscribeHandler(func(msg *nats.Msg) {
				if string(msg.Data) != fmt.Sprintf("order %d", cnt) {
					t.Fatalf("expected 'order %d' got %q", cnt, msg.Data)
				}

				msg.Ack(nats.AckWaitDuration(time.Second))
				cnt++

				if cnt == 15 {
					close(done)
				}
			}))
			checkErr(t, err, "subscribe failed")
			defer sub.Unsubscribe()

			if sub.Consumer().DeliverySubject() == c.DeliverySubject() {
				t.Fatalf("delivery subject reused")
			}

			select {
			case <-done:
			case <-time.NewTimer(time.Second).C:
				t.Fatalf("timeout")
			}

			// interest still exist, update should fail
			_, err = mgr.Subscribe("ORDERS.new", jsm.SubscribeDurable("HIDDEN"), jsm.SubscribeConsumer(jsm.MaxAckPending(1)), jsm.SubscribeHandler(func(msg *nats.Msg) {}))
			if err == nil {
				t.Fatalf("moving inbox with interest present did not fail")
			}
		})
	})
}
