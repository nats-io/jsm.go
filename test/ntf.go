package test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
	"github.com/synadia-io/orbit.go/ntf"
	ntfclient "github.com/synadia-io/orbit.go/ntf-client"
)

var ntfSvc *ntf.Service

func RunWithNTF(m *testing.M, lowPort int) {
	if os.Getenv("TESTER_NATS_URL") == "" {
		var err error
		ntfSvc, err = ntf.New(context.Background(), ntf.Options{PortRange: ntf.PortRange{Low: lowPort, High: lowPort + 999}})
		if err != nil {
			log.Fatal(err)
		}
		defer ntfSvc.Close()
	}

	m.Run()
}

func NTFURL() string {
	ntfURL := os.Getenv("TESTER_NATS_URL")

	if ntfURL == "" {
		return ntfSvc.ClientURL()
	}

	return ntfURL
}

func WithJSCluster(t testing.TB, size int, cb func(testing.TB, *nats.Conn, *jsm.Manager)) {
	t.Helper()

	ntfc := ntfclient.New(t, NTFURL())
	defer ntfc.Close(t)
	ntfc.WithJetStreamCluster(t, size, func(t testing.TB, nc *nats.Conn, _ *ntfclient.Instance) {
		mgr, err := jsm.New(nc, jsm.WithTimeout(time.Second))
		if err != nil {
			t.Fatalf("create js manager failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_, err := mgr.JetStreamAccountInfo()
				if err != nil {
					continue
				}

				cb(t, nc, mgr)

				return
			case <-ctx.Done():
				t.Fatalf("jetstream did not become available")
			}
		}
	}, ntfclient.WithConnectOptions(nats.UseOldRequestStyle()))
}
