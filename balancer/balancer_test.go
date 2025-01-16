package balancer

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func TestBalanceStream(t *testing.T) {
	withJSCluster(t, 3, func(t *testing.T, servers []*server.Server, nc *nats.Conn, mgr *jsm.Manager) error {
		streams := []*jsm.Stream{}
		for i := 1; i < 10; i++ {
			streamName := fmt.Sprintf("tests%d", i)
			subjects := fmt.Sprintf("tests%d.*", i)
			s, err := mgr.NewStream(streamName, jsm.Subjects(subjects), jsm.MemoryStorage(), jsm.Replicas(3))
			if err != nil {
				t.Fatalf("could not create stream %s", err)
			}
			streams = append(streams, s)
			defer s.Delete()
		}

		servers[2].DisableJetStream()
		err := servers[2].EnableJetStream(nil)
		if err != nil {
			return err
		}

		b, err := New(nc, api.NewDefaultLogger(api.DebugLevel))
		if err != nil {
			return err
		}

		count, err := b.BalanceStreams(streams)
		if err != nil {
			return err
		}

		if count == 0 {
			return err
		}
		return nil
	})
}

func TestBalanceConsumer(t *testing.T) {
	withJSCluster(t, 3, func(t *testing.T, servers []*server.Server, nc *nats.Conn, mgr *jsm.Manager) error {
		s, err := mgr.NewStream("TEST_CONSUMER_BALANCE", jsm.Subjects("test.*"), jsm.MemoryStorage(), jsm.Replicas(3))
		if err != nil {
			return err
		}

		defer s.Delete()

		consumers := []*jsm.Consumer{}
		for i := 1; i < 10; i++ {
			consumerName := fmt.Sprintf("testc%d", i)
			c, err := mgr.NewConsumer("TEST_CONSUMER_BALANCE", jsm.ConsumerName(consumerName))
			if err != nil {
				return err
			}
			consumers = append(consumers, c)
			defer c.Delete()
		}

		servers[2].DisableJetStream()
		err = servers[2].EnableJetStream(nil)
		if err != nil {
			return err
		}

		b, err := New(nc, api.NewDefaultLogger(api.DebugLevel))
		if err != nil {
			return err
		}

		count, err := b.BalanceConsumers(consumers)
		if err != nil {
			return err
		}

		if count == 0 {
			return err
		}

		return nil
	})
}

func withJSCluster(t *testing.T, retries int, cb func(*testing.T, []*server.Server, *nats.Conn, *jsm.Manager) error) {
	t.Helper()

	d, err := os.MkdirTemp("", "jstest")
	if err != nil {
		t.Fatalf("temp dir could not be made: %s", err)
	}
	defer os.RemoveAll(d)

	var (
		servers []*server.Server
	)

	for i := 1; i <= 3; i++ {
		opts := &server.Options{
			JetStream:  true,
			StoreDir:   filepath.Join(d, fmt.Sprintf("s%d", i)),
			Port:       -1,
			Host:       "localhost",
			ServerName: fmt.Sprintf("s%d", i),
			LogFile:    "/dev/null",
			Cluster: server.ClusterOpts{
				Name: "TEST",
				Port: 12000 + i,
			},
			Routes: []*url.URL{
				{Host: "localhost:12001"},
				{Host: "localhost:12002"},
				{Host: "localhost:12003"},
			},
		}

		s, err := server.NewServer(opts)
		if err != nil {
			t.Fatalf("server %d start failed: %v", i, err)
		}
		s.ConfigureLogger()
		go s.Start()
		if !s.ReadyForConnections(10 * time.Second) {
			t.Errorf("nats server %d did not start", i)
		}
		defer func() {
			s.Shutdown()
		}()

		servers = append(servers, s)
	}

	if len(servers) != 3 {
		t.Fatalf("servers did not start")
	}

	nc, err := nats.Connect(servers[0].ClientURL(), nats.UseOldRequestStyle())
	if err != nil {
		t.Fatalf("client start failed: %s", err)
	}
	defer nc.Close()

	mgr, err := jsm.New(nc, jsm.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("manager creation failed: %s", err)
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

			for i := 0; i < retries; i++ {
				err = cb(t, servers, nc, mgr)
				if err == nil {
					break
				}
			}

			if err != nil {
				t.Fatal(err)
			}

			return
		case <-ctx.Done():
			t.Fatalf("jetstream did not become available")
		}
	}
}
