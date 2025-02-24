package balancer

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

func TestBalancer(t *testing.T) {
	withTesterJetStreamCluster(t, func(t *testing.T, mgr *jsm.Manager, _ *srvtest.Client, servers []*testapi.ManagedServer) {
		var err error

		nc := mgr.NatsConn()
		firstServer := servers[0].Name

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
			if info.Leader != firstServer {
				placement := api.Placement{Preferred: firstServer}
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
			t.Fatalf("create failed: %v", err.Error())
		}

		count, err := b.BalanceStreams(streams)
		if err != nil {
			t.Fatalf("Balance failed: %v", err.Error())
		}

		if count == 0 {
			t.Fatal("Balanceed 0 streams")
		}

		consumers := []*jsm.Consumer{}
		for i := 1; i <= 3; i++ {
			consumerName := fmt.Sprintf("testc%d", i)
			c, err := mgr.NewConsumer("tests1", jsm.DurableName(consumerName), jsm.ConsumerOverrideReplicas(3))
			if err != nil {
				t.Fatalf("could not create consumer %s", err)
			}

			info, _ := c.ClusterInfo()
			if info.Leader != firstServer {
				placement := api.Placement{Preferred: firstServer}
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
			t.Fatalf("Balance failed: %v", err)
		}

		if count == 0 {
			t.Fatal("Balanced 0 consumers")
		}
	})
}

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
