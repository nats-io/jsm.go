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

// isBalanced checks if counts are within [min-tolerance, max+tolerance] where min/max
// is the ideal distribution floor/ceiling.
// Tolerance is higher when Sx > Rx since we cannot balance perfectly.
func isBalanced(leaderCounts map[string]int, totalEntities, serverCount, replicaCount int) bool {
	minExpected := totalEntities / serverCount
	maxExpected := minExpected
	if totalEntities%serverCount != 0 {
		maxExpected++
	}

	tolerance := 1
	if serverCount > replicaCount {
		tolerance = max(2, maxExpected)
	}

	for _, count := range leaderCounts {
		if count < minExpected-tolerance || count > maxExpected+tolerance {
			return false
		}
	}
	return true
}

func getStreamDistribution(mgr *jsm.Manager, streams []*jsm.Stream) (map[string]int, error) {
	distribution := make(map[string]int)
	for _, s := range streams {
		reloaded, err := mgr.LoadStream(s.Name())
		if err != nil {
			return nil, err
		}
		info, err := reloaded.ClusterInfo()
		if err != nil {
			return nil, err
		}
		if info.Leader != "" {
			distribution[info.Leader]++
		}
		for _, r := range info.Replicas {
			if _, ok := distribution[r.Name]; !ok {
				distribution[r.Name] = 0
			}
		}
	}
	return distribution, nil
}

func getConsumerDistribution(mgr *jsm.Manager, streamName string, consumers []*jsm.Consumer) (map[string]int, error) {
	distribution := make(map[string]int)
	for _, c := range consumers {
		reloaded, err := mgr.LoadConsumer(streamName, c.Name())
		if err != nil {
			return nil, err
		}
		info, err := reloaded.ClusterInfo()
		if err != nil {
			return nil, err
		}
		if info.Leader != "" {
			distribution[info.Leader]++
		}
		for _, r := range info.Replicas {
			if _, ok := distribution[r.Name]; !ok {
				distribution[r.Name] = 0
			}
		}
	}
	return distribution, nil
}

func TestBalancer(t *testing.T) {
	withJSCluster(t, 3, func(t *testing.T, servers []*server.Server, nc *nats.Conn, mgr *jsm.Manager) error {
		streams := []*jsm.Stream{}
		for i := 1; i <= 10; i++ {
			streamName := fmt.Sprintf("tests%d", i)
			subjects := fmt.Sprintf("tests%d.*", i)
			s, err := mgr.NewStream(streamName, jsm.Subjects(subjects), jsm.MemoryStorage(), jsm.Replicas(3))
			if err != nil {
				t.Fatalf("could not create stream %s", err)
			}
			streams = append(streams, s)
			defer s.Delete()
		}

		b, err := New(nc, api.NewDefaultLogger(api.DebugLevel))
		if err != nil {
			return err
		}

		_, err = b.BalanceStreams(streams)
		if err != nil {
			return err
		}

		time.Sleep(500 * time.Millisecond)

		streamDist, err := getStreamDistribution(mgr, streams)
		if err != nil {
			return err
		}
		if !isBalanced(streamDist, len(streams), len(streamDist), 3) {
			return fmt.Errorf("streams are not balanced after BalanceStreams: %v", streamDist)
		}

		consumers := []*jsm.Consumer{}
		for i := 1; i <= 10; i++ {
			consumerName := fmt.Sprintf("testc%d", i)
			c, err := mgr.NewConsumer("tests1", jsm.DurableName(consumerName), jsm.ConsumerOverrideReplicas(3))
			if err != nil {
				return err
			}
			consumers = append(consumers, c)
			defer c.Delete()
		}

		_, err = b.BalanceConsumers(consumers)
		if err != nil {
			return err
		}

		time.Sleep(500 * time.Millisecond)

		consumerDist, err := getConsumerDistribution(mgr, "tests1", consumers)
		if err != nil {
			return err
		}
		if !isBalanced(consumerDist, len(consumers), len(consumerDist), 3) {
			return fmt.Errorf("consumers are not balanced after BalanceConsumers: %v", consumerDist)
		}

		return nil
	})
}

func TestBalancer_FiveNodeCluster(t *testing.T) {
	withJSCluster(t, 5, func(t *testing.T, servers []*server.Server, nc *nats.Conn, mgr *jsm.Manager) error {
		streams := []*jsm.Stream{}

		for i := 1; i <= 5; i++ {
			streamName := fmt.Sprintf("fivetests_stream_%d", i)
			subjects := fmt.Sprintf("fivetest.%d.*", i)
			s, err := mgr.NewStream(streamName, jsm.Subjects(subjects), jsm.MemoryStorage(), jsm.Replicas(3))
			if err != nil {
				t.Fatalf("could not create stream: %v", err)
			}
			streams = append(streams, s)
			defer s.Delete()
		}

		b, err := New(nc, api.NewDefaultLogger(api.DebugLevel))
		if err != nil {
			return err
		}

		_, err = b.BalanceStreams(streams)
		if err != nil {
			return err
		}

		time.Sleep(500 * time.Millisecond)

		streamDist, err := getStreamDistribution(mgr, streams)
		if err != nil {
			return err
		}
		if !isBalanced(streamDist, len(streams), len(streamDist), 3) {
			return fmt.Errorf("streams are not balanced after BalanceStreams: %v", streamDist)
		}

		return nil
	})
}

func withJSCluster(t *testing.T, clusterSize int, cb func(*testing.T, []*server.Server, *nats.Conn, *jsm.Manager) error) {
	t.Helper()

	d, err := os.MkdirTemp("", "jstest")
	if err != nil {
		t.Fatalf("temp dir could not be made: %s", err)
	}
	defer os.RemoveAll(d)

	var (
		servers []*server.Server
	)
	routes := []*url.URL{}
	for j := 1; j <= clusterSize; j++ {
		routes = append(routes, &url.URL{Host: fmt.Sprintf("localhost:%d", 12000+j)})
	}

	for i := 1; i <= clusterSize; i++ {
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
			Routes: routes,
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

	if len(servers) != clusterSize {
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
