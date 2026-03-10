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

type mockEntity struct {
	name        string
	clusterInfo api.ClusterInfo
	clusterErr  error
	stepDownErr error
	stepDowns   int
}

func (m *mockEntity) Name() string { return m.name }

func (m *mockEntity) ClusterInfo() (api.ClusterInfo, error) {
	return m.clusterInfo, m.clusterErr
}

func (m *mockEntity) LeaderStepDown(p ...*api.Placement) error {
	if m.stepDownErr != nil {
		return m.stepDownErr
	}
	m.stepDowns++
	return nil
}

func newMockEntity(name, leader, cluster string, replicas ...string) *mockEntity {
	reps := make([]*api.PeerInfo, len(replicas))
	for i, r := range replicas {
		reps[i] = &api.PeerInfo{Name: r}
	}
	return &mockEntity{
		name: name,
		clusterInfo: api.ClusterInfo{
			Name:     cluster,
			Leader:   leader,
			Replicas: reps,
		},
	}
}

func testBalancer() *Balancer {
	return &Balancer{log: api.NewDefaultLogger(api.ErrorLevel)}
}

func TestCalcClusterDistribution(t *testing.T) {
	b := testBalancer()

	tests := []struct {
		name     string
		peers    map[string]*peer
		expected int
	}{
		{
			name:     "no peers",
			peers:    map[string]*peer{},
			expected: 0,
		},
		{
			name: "even split",
			peers: map[string]*peer{
				"s1": {entities: make([]balanceEntity, 3)},
				"s2": {entities: make([]balanceEntity, 3)},
				"s3": {entities: make([]balanceEntity, 3)},
			},
			expected: 3,
		},
		{
			name: "single server",
			peers: map[string]*peer{
				"s1": {entities: make([]balanceEntity, 5)},
			},
			expected: 5,
		},
		{
			name: "uneven split",
			peers: map[string]*peer{
				"s1": {entities: make([]balanceEntity, 5)},
				"s2": {entities: make([]balanceEntity, 3)},
				"s3": {entities: make([]balanceEntity, 2)},
			},
			expected: 4,
		},
		{
			name: "all entities on one peer",
			peers: map[string]*peer{
				"s1": {entities: make([]balanceEntity, 10)},
				"s2": {entities: nil},
				"s3": {entities: nil},
			},
			expected: 4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &cluster{peers: tc.peers}
			got := b.calcClusterDistribution(c)
			if got != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, got)
			}
		})
	}
}

func TestCalcLeaderOffset(t *testing.T) {
	b := testBalancer()

	peers := map[string]*peer{
		"s1": {entities: make([]balanceEntity, 5)},
		"s2": {entities: make([]balanceEntity, 3)},
		"s3": {entities: make([]balanceEntity, 1)},
	}

	b.calcLeaderOffset(peers, 3)

	if peers["s1"].offset != 2 {
		t.Errorf("s1 offset: expected 2, got %d", peers["s1"].offset)
	}
	if peers["s2"].offset != 0 {
		t.Errorf("s2 offset: expected 0, got %d", peers["s2"].offset)
	}
	if peers["s3"].offset != -2 {
		t.Errorf("s3 offset: expected -2, got %d", peers["s3"].offset)
	}
}

func TestIsInPeerGroup(t *testing.T) {
	info := api.ClusterInfo{
		Leader: "s1",
		Replicas: []*api.PeerInfo{
			{Name: "s2"},
			{Name: "s3"},
		},
	}

	if isInPeerGroup(info, "s1") {
		t.Error("leader should not be in peer group via replicas")
	}
	if !isInPeerGroup(info, "s2") {
		t.Error("s2 should be in peer group")
	}
	if isInPeerGroup(info, "s4") {
		t.Error("s4 should not be in peer group")
	}
}

func TestCreateClusterMappings_LeaderlessEntity(t *testing.T) {
	b := testBalancer()
	b.log = api.NewDefaultLogger(api.WarnLevel)

	e := &mockEntity{
		name: "leaderless",
		clusterInfo: api.ClusterInfo{
			Name:     "C1",
			Leader:   "",
			Replicas: []*api.PeerInfo{{Name: "s1"}, {Name: "s2"}},
		},
	}

	clusterMap := map[string]*cluster{}
	clusterMap, err := b.createClusterMappings(e, clusterMap)
	if err != nil {
		t.Fatal(err)
	}

	c := clusterMap["C1"]
	if c == nil {
		t.Fatal("cluster C1 not created")
	}

	// leaderless entity should not be added to any peer's entities
	for name, p := range c.peers {
		if len(p.entities) != 0 {
			t.Errorf("peer %s should have 0 entities, got %d", name, len(p.entities))
		}
	}

	// but peers from replicas should still be tracked
	if len(c.peers) != 2 {
		t.Errorf("expected 2 peers, got %d", len(c.peers))
	}
}

func TestCreateClusterMappings_ClusterInfoError(t *testing.T) {
	b := testBalancer()

	e := &mockEntity{
		name:       "broken",
		clusterErr: fmt.Errorf("connection refused"),
	}

	clusterMap := map[string]*cluster{}
	_, err := b.createClusterMappings(e, clusterMap)
	if err == nil {
		t.Fatal("expected error from ClusterInfo failure")
	}
}

func TestBalance_EmptyEntities(t *testing.T) {
	b := testBalancer()

	// server has positive offset but no entities - should not panic
	servers := map[string]*peer{
		"s1": {name: "s1", entities: []balanceEntity{}, offset: 3},
		"s2": {name: "s2", entities: []balanceEntity{}, offset: -3},
	}

	n, err := b.balance(servers, 3, "C1", "stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 stepped down, got %d", n)
	}
}

func TestBalance_AllClusterInfoFail(t *testing.T) {
	b := testBalancer()

	e1 := &mockEntity{name: "e1", clusterErr: fmt.Errorf("fail")}
	e2 := &mockEntity{name: "e2", clusterErr: fmt.Errorf("fail")}

	servers := map[string]*peer{
		"s1": {name: "s1", entities: []balanceEntity{e1, e2}, offset: 2},
		"s2": {name: "s2", entities: []balanceEntity{}, offset: -2},
	}

	// should not panic with rand.IntN(0)
	n, err := b.balance(servers, 2, "C1", "stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 stepped down, got %d", n)
	}
}

func TestBalance_AlreadyBalanced(t *testing.T) {
	b := testBalancer()

	e1 := newMockEntity("e1", "s1", "C1", "s2", "s3")
	e2 := newMockEntity("e2", "s2", "C1", "s1", "s3")
	e3 := newMockEntity("e3", "s3", "C1", "s1", "s2")

	servers := map[string]*peer{
		"s1": {name: "s1", entities: []balanceEntity{e1}, offset: 0},
		"s2": {name: "s2", entities: []balanceEntity{e2}, offset: 0},
		"s3": {name: "s3", entities: []balanceEntity{e3}, offset: 0},
	}

	n, err := b.balance(servers, 1, "C1", "stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 stepped down for already balanced cluster, got %d", n)
	}

	// no step downs should have been called
	for _, e := range []*mockEntity{e1, e2, e3} {
		if e.stepDowns != 0 {
			t.Errorf("%s had %d step downs, expected 0", e.name, e.stepDowns)
		}
	}
}

func TestBalance_StepDownFailure(t *testing.T) {
	b := testBalancer()

	e1 := newMockEntity("e1", "s1", "C1", "s2", "s3")
	e1.stepDownErr = fmt.Errorf("step down refused")
	e2 := newMockEntity("e2", "s1", "C1", "s2", "s3")
	e2.stepDownErr = fmt.Errorf("step down refused")
	e3 := newMockEntity("e3", "s1", "C1", "s2", "s3")
	e3.stepDownErr = fmt.Errorf("step down refused")

	servers := map[string]*peer{
		"s1": {name: "s1", entities: []balanceEntity{e1, e2, e3}, offset: 2},
		"s2": {name: "s2", entities: []balanceEntity{}, offset: -1},
		"s3": {name: "s3", entities: []balanceEntity{}, offset: -1},
	}

	// step down failures cause the balancer to skip entities gracefully;
	// each failure removes the entity and breaks out of the inner loop
	n, err := b.balance(servers, 1, "C1", "stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 stepped down when all step downs fail, got %d", n)
	}
}

func TestBalance_SuccessfulRebalance(t *testing.T) {
	b := testBalancer()

	e1 := newMockEntity("e1", "s1", "C1", "s2", "s3")
	e2 := newMockEntity("e2", "s1", "C1", "s2", "s3")
	e3 := newMockEntity("e3", "s1", "C1", "s2", "s3")

	servers := map[string]*peer{
		"s1": {name: "s1", entities: []balanceEntity{e1, e2, e3}, offset: 2},
		"s2": {name: "s2", entities: []balanceEntity{}, offset: -1},
		"s3": {name: "s3", entities: []balanceEntity{}, offset: -1},
	}

	n, err := b.balance(servers, 1, "C1", "stream")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 stepped down, got %d", n)
	}
}

func TestCreateClusterMappings_MultiCluster(t *testing.T) {
	b := testBalancer()

	e1 := newMockEntity("e1", "s1", "EAST", "s2", "s3")
	e2 := newMockEntity("e2", "s4", "WEST", "s5", "s6")

	clusterMap := map[string]*cluster{}
	var err error

	clusterMap, err = b.createClusterMappings(e1, clusterMap)
	if err != nil {
		t.Fatal(err)
	}
	clusterMap, err = b.createClusterMappings(e2, clusterMap)
	if err != nil {
		t.Fatal(err)
	}

	if len(clusterMap) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusterMap))
	}

	east := clusterMap["EAST"]
	if east == nil {
		t.Fatal("EAST cluster not found")
	}
	if len(east.peers) != 3 {
		t.Errorf("EAST expected 3 peers, got %d", len(east.peers))
	}

	west := clusterMap["WEST"]
	if west == nil {
		t.Fatal("WEST cluster not found")
	}
	if len(west.peers) != 3 {
		t.Errorf("WEST expected 3 peers, got %d", len(west.peers))
	}
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
