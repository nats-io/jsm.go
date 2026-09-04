package test

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	natsd "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/synadia-io/orbit.go/ntf"
	ntfclient "github.com/synadia-io/orbit.go/ntf-client"
)

var ntfSvc *ntf.Service

func TestMain(m *testing.M) {
	var err error

	td, err := os.MkdirTemp("", "")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(td)

	if os.Getenv("TESTER_NATS_URL") == "" {
		ntfSvc, err = ntf.New(context.Background(), ntf.Options{Dir: td})
		if err != nil {
			log.Fatal(err)
		}
		defer ntfSvc.Close()
	}

	m.Run()
}

func ntfServerUrl() string {
	ntfURL := os.Getenv("TESTER_NATS_URL")

	if ntfURL == "" {
		return ntfSvc.ClientURL()
	}

	return ntfURL
}

func withJSCluster(t testing.TB, cb func(testing.TB, *nats.Conn, *jsm.Manager)) {
	t.Helper()

	var err error

	ntfc := ntfclient.New(t, ntfServerUrl())
	ntfc.WithJetStreamCluster(t, 3, func(t testing.TB, nc *nats.Conn, instance *ntfclient.Instance) {
		if !nc.Opts.UseOldRequestStyle {
			nc.Close()
			nc, err = nats.Connect(instance.RandomServer().URL, nats.UseOldRequestStyle())
			checkErr(t, err, "nats connection failed")
			defer nc.Close()
		}
		mgr, err := jsm.New(nc, jsm.WithTimeout(time.Second))
		checkErr(t, err, "create js manager failed")

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
	})
}

func withJSServer(t testing.TB, cb func(testing.TB, *nats.Conn, *jsm.Manager, *ntfclient.Instance)) {
	t.Helper()

	var err error

	ntfc := ntfclient.New(t, ntfServerUrl())
	ntfc.WithJetStreamServer(t, func(t testing.TB, nc *nats.Conn, instance *ntfclient.Instance) {
		if !nc.Opts.UseOldRequestStyle {
			nc.Close()
			nc, err = nats.Connect(instance.RandomServer().URL, nats.UseOldRequestStyle())
			checkErr(t, err, "nats connection failed")
		}
		mgr, err := jsm.New(nc, jsm.WithTimeout(time.Second))
		checkErr(t, err, "create js manager failed")

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

				cb(t, nc, mgr, instance)

				return
			case <-ctx.Done():
				t.Fatalf("jetstream did not become available")
			}
		}
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
