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

func TestBalancer(t *testing.T) {
	withJSCluster(t, func(t *testing.T, servers []*server.Server, nc *nats.Conn, mgr *jsm.Manager) error {
		var err error
		waitTime := 100 * time.Millisecond
		streams := []*jsm.Stream{}
		for i := 1; i <= 3; i++ {
			streamName := fmt.Sprintf("tests%d", i)
			subjects := fmt.Sprintf("tests%d.*", i)
			s, err := mgr.NewStream(streamName, jsm.Subjects(subjects), jsm.MemoryStorage(), jsm.Replicas(3))
			if err != nil {
				t.Fatalf("could not create stream %s", err)
			}
			info, _ := s.ClusterInfo()
			if info.Leader != "s1" {
				placement := api.Placement{Preferred: "s1"}
				err = s.LeaderStepDown(&placement)
				if err != nil {
					t.Fatalf("could not move stream %s", err)
				}
			}

			var ns *jsm.Stream

			for i := 1; i <= 5; i++ {
				ns, err = mgr.LoadStream(streamName)
				if err != nil {
					t.Fatal(err)
				}
				info, _ := ns.ClusterInfo()
				if info.Leader != "" {
					break
				}
				if i == 5 {
					t.Fatalf("could not load stream %s after %dms", streamName, i*int(waitTime))
				}
				time.Sleep(waitTime)
			}

			streams = append(streams, ns)
			defer s.Delete()
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

		consumers := []*jsm.Consumer{}
		for i := 1; i <= 3; i++ {
			consumerName := fmt.Sprintf("testc%d", i)
			c, err := mgr.NewConsumer("tests1", jsm.DurableName(consumerName), jsm.ConsumerOverrideReplicas(3))
			if err != nil {
				return err
			}

			info, _ := c.ClusterInfo()
			if info.Leader != "s1" {
				placement := api.Placement{Preferred: "s1"}
				err = c.LeaderStepDown(&placement)
				if err != nil {
					t.Fatalf("could not move consumer %s", err)
				}
			}

			var nc *jsm.Consumer

			for i := 1; i <= 5; i++ {
				nc, err = mgr.LoadConsumer("tests1", consumerName)
				if err == nil {
					info, _ := nc.ClusterInfo()
					if info.Leader != "" {
						break
					}
				}

				if i == 5 {
					t.Fatalf("could not load stream %s after %dms", consumerName, i*int(waitTime))
				}
				time.Sleep(waitTime)
			}

			consumers = append(consumers, nc)
			defer c.Delete()
		}

		count, err = b.BalanceConsumers(consumers)
		if err != nil {
			return err
		}

		if count == 0 {
			return err
		}

		return nil
	})
}

func withJSCluster(t *testing.T, cb func(*testing.T, []*server.Server, *nats.Conn, *jsm.Manager) error) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

			err = cb(t, servers, nc, mgr)

			if err != nil {
				t.Fatal(err)
			}

			return
		case <-ctx.Done():
			t.Fatalf("jetstream did not become available")
		}
	}
}
