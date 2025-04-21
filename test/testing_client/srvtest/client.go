package srvtest

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"testing"
	"time"

	"github.com/nats-io/jsm.go/test/testing_client/api"
	"github.com/nats-io/nats.go"
)

type Client struct {
	address string
	nc      *nats.Conn
}

// New connects to the management service of the test cluster manager
func New(t *testing.T, server string, opts ...nats.Option) *Client {
	t.Helper()

	u, err := url.Parse(server)
	if err != nil {
		t.Fatal("could not parse server URL: %w", err)
	}

	nopts := []nats.Option{
		nats.Timeout(10 * time.Second),
		nats.MaxReconnects(-1),
	}

	nc, err := nats.Connect(server, append(nopts, opts...)...)
	if err != nil {
		t.Fatal("failed to connect to NATS:", err)
	}

	return &Client{nc: nc, address: u.Hostname()}
}

// WithJetStreamServer creates a server running JetStream and connects to it
func (c *Client) WithJetStreamServer(t *testing.T, h func(*testing.T, *nats.Conn, *api.ManagedServer)) {
	t.Helper()

	c.withServer(t, true, h)
}

// WithServer creates a non JetStream server and connects to it
func (c *Client) WithServer(t *testing.T, h func(*testing.T, *nats.Conn, *api.ManagedServer)) {
	t.Helper()

	c.withServer(t, false, h)
}

func (c *Client) withServer(t *testing.T, js bool, h func(*testing.T, *nats.Conn, *api.ManagedServer)) {
	t.Helper()

	srv := c.CreateServer(t, js)

	nc, err := nats.Connect(srv.URL, nats.MaxReconnects(-1))
	if err != nil {
		t.Fatal("failed to connect to NATS:", err)
	}

	h(t, nc, srv)

	nc.Close()
	c.Reset(t)
}

// WithJetStreamCluster creates a cluster with given server count running JetStream and connects to a random server
func (c *Client) WithJetStreamCluster(t *testing.T, servers int, h func(*testing.T, *nats.Conn, []*api.ManagedServer)) {
	t.Helper()

	c.withCluster(t, servers, true, h)
}

// WithCluster creates a non JetStream cluster with given server count and connect to a random server
func (c *Client) WithCluster(t *testing.T, servers int, h func(*testing.T, *nats.Conn, []*api.ManagedServer)) {
	t.Helper()

	c.withCluster(t, servers, false, h)
}

func (c *Client) withCluster(t *testing.T, servers int, js bool, h func(*testing.T, *nats.Conn, []*api.ManagedServer)) {
	t.Helper()

	cluster := c.CreateCluster(t, servers, js)
	if len(cluster) != servers {
		t.Fatalf("expected number of servers to be %d got %d", servers, len(cluster))
	}

	nc, err := nats.Connect(RandomServer(cluster).URL, nats.MaxReconnects(-1))
	if err != nil {
		t.Fatal("failed to connect to NATS:", err)
	}

	if js {
		c.WaitForJetStream(t, nc)
	}

	h(t, nc, cluster)

	nc.Close()
	c.Reset(t)
}

// WaitForJetStream polls '$JS.API.INFO' regularly waiting for Jetstream to be ready, fails after 5 seconds
func (c *Client) WaitForJetStream(t *testing.T, nc *nats.Conn) {
	t.Helper()

	for i := 0; i < 10; i++ {
		_, err := nc.Request("$JS.API.INFO", nil, time.Second)
		if err == nil {
			return
		}

		time.Sleep(500 * time.Millisecond)

		if i == 9 {
			t.Fatalf("jetstream did not become ready")
		}
	}
}

// WithJetStreamSuperCluster creates a super-cluster with given server and cluster counts running JetStream and connects to a random server
func (c *Client) WithJetStreamSuperCluster(t *testing.T, clusters int, servers int, h func(*testing.T, *nats.Conn, []*api.ManagedServer)) {
	t.Helper()

	c.withSuperCluster(t, clusters, servers, true, h)
}

// WithSuperCluster creates a non JetStream super-cluster with given server and cluster counts and connects to a random server
func (c *Client) WithSuperCluster(t *testing.T, clusters int, servers int, h func(*testing.T, *nats.Conn, []*api.ManagedServer)) {
	t.Helper()

	c.withSuperCluster(t, clusters, servers, false, h)
}

func (c *Client) withSuperCluster(t *testing.T, clusters int, servers int, js bool, h func(*testing.T, *nats.Conn, []*api.ManagedServer)) {
	t.Helper()

	cluster := c.CreateSuperCluster(t, clusters, servers, js)
	if len(cluster) != servers*clusters {
		t.Fatalf("expected number of servers to be %d got %d", servers*clusters, len(cluster))
	}

	nc, err := nats.Connect(RandomServer(cluster).URL, nats.MaxReconnects(-1))
	if err != nil {
		t.Fatal("failed to connect to NATS:", err)
	}

	if js {
		c.WaitForJetStream(t, nc)
	}

	h(t, nc, cluster)

	nc.Close()
	c.Reset(t)
}

// CreateSuperCluster creates a super cluster
func (c *Client) CreateSuperCluster(t *testing.T, clusters int, servers int, js bool) []*api.ManagedServer {
	t.Helper()

	jreq, err := json.Marshal(api.CreateSuperClusterRequest{JetStream: js, Clusters: clusters, Servers: servers})
	if err != nil {
		t.Fatalf("could not marshal CreateSuperClusterRequest: %v", err)
	}

	msg, err := c.nc.Request("tester.create.super-cluster", jreq, 10*time.Second)
	if err != nil {
		t.Fatalf("could not send CreateSuperClusterRequest: %v", err)
	}

	if err := msg.Header.Get("Nats-Service-Error"); err != "" {
		t.Fatalf("Request failed: %v", err)
	}

	resp := api.CreateResponse{}
	err = json.Unmarshal(msg.Data, &resp)
	if err != nil {
		t.Fatalf("could not unmarshal CreateResponse: %v: %v", string(msg.Data), err)
	}

	for _, srv := range resp.Servers {
		srv.URL = fmt.Sprintf("nats://%s:%d", c.address, srv.Port)
	}

	return resp.Servers
}

// CreateCluster creates a cluster
func (c *Client) CreateCluster(t *testing.T, servers int, js bool) []*api.ManagedServer {
	t.Helper()

	jreq, err := json.Marshal(api.CreateClusterRequest{JetStream: js, Servers: servers})
	if err != nil {
		t.Fatalf("could not marshal CreateClusterRequest: %v", err)
	}

	msg, err := c.nc.Request("tester.create.cluster", jreq, 10*time.Second)
	if err != nil {
		t.Fatalf("could not send CreateClusterRequest: %v", err)
	}

	if err := msg.Header.Get("Nats-Service-Error"); err != "" {
		t.Fatalf("Request failed: %v", err)
	}

	resp := api.CreateResponse{}
	err = json.Unmarshal(msg.Data, &resp)
	if err != nil {
		t.Fatalf("could not unmarshal CreateResponse: %v: %v", string(msg.Data), err)
	}

	for _, srv := range resp.Servers {
		srv.URL = fmt.Sprintf("nats://%s:%d", c.address, srv.Port)
	}

	return resp.Servers
}

// CreateServer creates a server
func (c *Client) CreateServer(t *testing.T, js bool) *api.ManagedServer {
	t.Helper()

	jreq, err := json.Marshal(api.CreateServerRequest{JetStream: js})
	if err != nil {
		t.Fatalf("could not marshal CreateServerRequest: %v", err)
	}

	msg, err := c.nc.Request("tester.create.server", jreq, 10*time.Second)
	if err != nil {
		t.Fatalf("could not send CreateServerRequest: %v", err)
	}

	if err := msg.Header.Get("Nats-Service-Error"); err != "" {
		t.Fatalf("Request failed: %v", err)
	}

	resp := api.CreateResponse{}
	err = json.Unmarshal(msg.Data, &resp)
	if err != nil {
		t.Fatalf("could not unmarshal CreateResponse: %v: %v", string(msg.Data), err)
	}

	if len(resp.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(resp.Servers))
	}

	srv := resp.Servers[0]
	srv.URL = fmt.Sprintf("nats://%s:%d", c.address, srv.Port)

	return srv
}

// Reset shuts down and removes all servers
func (c *Client) Reset(t *testing.T) api.ResetResponse {
	t.Helper()

	msg, err := c.nc.Request("tester.reset", nil, 10*time.Second)
	if err != nil {
		t.Fatalf("could not send CreateServerRequest: %v", err)
	}

	if err := msg.Header.Get("Nats-Service-Error"); err != "" {
		t.Fatalf("Request failed: %v", err)
	}

	resp := api.ResetResponse{}
	err = json.Unmarshal(msg.Data, &resp)
	if err != nil {
		t.Fatalf("could not unmarshal CreateResponse: %v: %v", string(msg.Data), err)
	}

	return resp
}

// StopServer stops a single server
func (c *Client) StopServer(t *testing.T, server *api.ManagedServer) *api.StopServerResponse {
	t.Helper()

	if server == nil || server.Name == "" {
		t.Fatal("server is is required")
	}

	req, err := json.Marshal(api.StopServerRequest{Name: server.Name})
	if err != nil {
		t.Fatalf("could not marshal StopServerRequest: %v", err)
	}

	msg, err := c.nc.Request("tester.stop.server", req, 10*time.Second)
	if err != nil {
		t.Fatalf("could not send StopServerRequest: %v", err)
	}

	if err := msg.Header.Get("Nats-Service-Error"); err != "" {
		t.Fatalf("Request failed: %v", err)
	}

	resp := api.StopServerResponse{}
	err = json.Unmarshal(msg.Data, &resp)
	if err != nil {
		t.Fatalf("could not unmarshal StopServerRequest: %v: %v", string(msg.Data), err)
	}

	return &resp
}

// StartServer starts a single server that was previously stopped
func (c *Client) StartServer(t *testing.T, server *api.ManagedServer) *api.StartServerResponse {
	t.Helper()

	if server == nil || server.Name == "" {
		t.Fatal("server is is required")
	}

	req, err := json.Marshal(api.StartServerRequest{Name: server.Name})
	if err != nil {
		t.Fatalf("could not marshal StartServerRequest: %v", err)
	}

	msg, err := c.nc.Request("tester.start.server", req, 10*time.Second)
	if err != nil {
		t.Fatalf("could not send StartServerRequest: %v", err)
	}

	if err := msg.Header.Get("Nats-Service-Error"); err != "" {
		t.Fatalf("Request failed: %v", err)
	}

	resp := api.StartServerResponse{}
	err = json.Unmarshal(msg.Data, &resp)
	if err != nil {
		t.Fatalf("could not unmarshal StartServerResponse: %v: %v", string(msg.Data), err)
	}

	return &resp
}

// Status obtains status of all servers managed by the tester
func (c *Client) Status(t *testing.T) *api.StatusResponse {
	t.Helper()

	msg, err := c.nc.Request("tester.status", nil, 10*time.Second)
	if err != nil {
		t.Fatalf("could not send StatusRequest: %v", err)
	}

	if err := msg.Header.Get("Nats-Service-Error"); err != "" {
		t.Fatalf("Request failed: %v", err)
	}

	resp := api.StatusResponse{}
	err = json.Unmarshal(msg.Data, &resp)
	if err != nil {
		t.Fatalf("could not unmarshal StatusResponse: %v: %v", string(msg.Data), err)
	}

	return &resp
}

// Close closes the connection to the management service
func (c *Client) Close(t *testing.T) {
	t.Helper()

	c.nc.Close()
}

// RandomClusterServer picks a random server in a cluster from the list of running ones
func RandomClusterServer(cluster string, servers []*api.ManagedServer) (*api.ManagedServer, error) {
	matched := []*api.ManagedServer{}
	for _, srv := range servers {
		if srv.Cluster == cluster {
			matched = append(matched, srv)
		}
	}

	if len(matched) == 0 {
		return nil, fmt.Errorf("cluster %q not found", cluster)
	}

	return RandomServer(matched), nil
}

// RandomServer picks a random server
func RandomServer(servers []*api.ManagedServer) *api.ManagedServer {
	return servers[rand.Intn(len(servers))]
}
