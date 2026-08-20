// Copyright 2024 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connbalancer

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func TestSubjectInterest(t *testing.T) {
	withCluster(t, func(t *testing.T, srv []*server.Server, nc *nats.Conn) {
		for i := 0; i < 5; i++ {
			client, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("USER", "PASS"))
			if err != nil {
				t.Fatalf("could not create client")
			}
			defer client.Close()
			_, err = client.SubscribeSync("X.>")
			if err != nil {
				t.Fatalf("sub failed")
			}
		}

		client2, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("USER", "PASS"))
		if err != nil {
			t.Fatalf("could not create client")
		}
		defer client2.Close()

		checkBalancedInRange(t, nc, 0, 0, ConnectionSelector{
			Account:         "USERS",
			SubjectInterest: "foo",
		})

		checkBalancedInRange(t, nc, 2, 4, ConnectionSelector{
			Account:         "USERS",
			SubjectInterest: "X.>",
		})
	})
}

func TestAccountLimit(t *testing.T) {
	withCluster(t, func(t *testing.T, srv []*server.Server, nc *nats.Conn) {
		for i := 0; i < 5; i++ {
			client, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("USER", "PASS"))
			if err != nil {
				t.Fatalf("could not create client")
			}
			defer client.Close()
		}

		client2, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("SYS", "PASS"))
		if err != nil {
			t.Fatalf("could not create client")
		}
		defer client2.Close()

		checkBalancedInRange(t, nc, 0, 0, ConnectionSelector{
			Account: "FOO",
		})

		checkBalancedInRange(t, nc, 2, 4, ConnectionSelector{
			Account: "USERS",
		})
	})
}

func TestClientIdleLimit(t *testing.T) {
	withCluster(t, func(t *testing.T, srv []*server.Server, nc *nats.Conn) {
		for i := 0; i < 5; i++ {
			client, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("USER", "PASS"))
			if err != nil {
				t.Fatalf("could not create client")
			}
			defer client.Close()
		}

		checkBalancedInRange(t, nc, 0, 0, ConnectionSelector{
			Idle: time.Minute,
		})

		checkBalancedInRange(t, nc, 2, 4, ConnectionSelector{
			Idle: time.Millisecond,
		})
	})
}

func TestServerNameLimit(t *testing.T) {
	withCluster(t, func(t *testing.T, srv []*server.Server, nc *nats.Conn) {
		t.Run("Only ourselves on selected server", func(t *testing.T) {
			checkBalancedInRange(t, nc, 0, 0, ConnectionSelector{
				ServerName: srv[0].Name(),
			})
		})

		t.Run("Connections on specific server", func(t *testing.T) {
			for i := 0; i < 5; i++ {
				client, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("USER", "PASS"))
				if err != nil {
					t.Fatalf("could not create client")
				}
				defer client.Close()
			}

			checkBalancedInRange(t, nc, 0, 0, ConnectionSelector{
				ServerName: srv[2].Name(),
			})
		})
	})
}

func TestSuccessiveBalanceRuns(t *testing.T) {
	withCluster(t, func(t *testing.T, srv []*server.Server, nc *nats.Conn) {
		const clients = 10

		monitorConns := clusterConnections(srv)

		for i := range clients {
			client, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("USER", "PASS"))
			if err != nil {
				t.Fatalf("could not create client %d: %v", i, err)
			}
			defer client.Close()
		}

		totalConns := monitorConns + clients
		waitForConnections(t, srv, totalConns)

		checkBalancedInRange(t, nc, 5, 7, ConnectionSelector{})

		balancer, err := New(nc, 0, api.NewDiscardLogger(), ConnectionSelector{})
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}

		deadline := time.Now().Add(10 * time.Second)

		for {
			waitForConnections(t, srv, totalConns)

			balanced, err := balancer.Balance(context.Background())
			if err != nil {
				t.Fatalf("balance failed: %v", err)
			}
			if balanced == 0 {
				return
			}

			if time.Now().After(deadline) {
				t.Fatalf("successive balance runs did not converge, last run balanced %d connections", balanced)
			}
		}
	})
}

func TestBalanceMultiNodeCluster(t *testing.T) {
	withCluster(t, func(t *testing.T, srv []*server.Server, nc *nats.Conn) {
		for range 15 {
			client, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("USER", "PASS"))
			if err != nil {
				t.Fatalf("could not create client: %v", err)
			}
			defer client.Close()
		}

		for range 3 {
			client, err := nats.Connect(srv[1].ClientURL(), nats.UserInfo("USER", "PASS"))
			if err != nil {
				t.Fatalf("could not create client: %v", err)
			}
			defer client.Close()
		}
		checkBalancedInRange(t, nc, 8, 10, ConnectionSelector{})
	})
}

func TestNewValidation(t *testing.T) {
	nc := &nats.Conn{}

	_, err := New(nc, 0, api.NewDiscardLogger(), ConnectionSelector{
		SubjectInterest: "foo.>",
	})
	if err == nil {
		t.Fatal("expected error when SubjectInterest is set without Account")
	}

	_, err = New(nc, 0, api.NewDiscardLogger(), ConnectionSelector{
		SubjectInterest: "foo.>",
		Account:         "USERS",
	})
	if err != nil {
		t.Fatalf("expected no error when both SubjectInterest and Account are set: %v", err)
	}
}

func checkBalancedInRange(t *testing.T, nc *nats.Conn, min, max int, s ConnectionSelector) {
	t.Helper()

	balancer, err := New(nc, 0, api.NewDiscardLogger(), s)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	balanced, err := balancer.Balance(context.Background())
	if err != nil {
		t.Fatalf("balance failed: %v", err)
	}
	if balanced < min || balanced > max {
		t.Fatalf("Expected to balance %d-%d connections but balanced %d", min, max, balanced)
	}
}

func clusterConnections(srv []*server.Server) int {
	var total int

	for _, s := range srv {
		total += s.NumClients()
	}

	return total
}

func waitForConnections(t *testing.T, srv []*server.Server, expect int) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for {
		total := clusterConnections(srv)
		if total == expect {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("expected %d connections in the cluster but found %d", expect, total)
		}

		time.Sleep(25 * time.Millisecond)
	}
}

func withCluster(t *testing.T, cb func(t *testing.T, servers []*server.Server, nc *nats.Conn)) {
	t.Helper()

	d, err := os.MkdirTemp("", "jstest")
	if err != nil {
		t.Fatalf("temp dir could not be made: %s", err)
	}
	defer os.RemoveAll(d)

	var servers []*server.Server
	var routes []*url.URL

	for i := 1; i <= 3; i++ {
		sa := server.NewAccount("SYSTEM")
		ua := server.NewAccount("USERS")

		opts := &server.Options{
			Port:       -1,
			Host:       "localhost",
			ServerName: fmt.Sprintf("s%d", i),
			LogFile:    "/dev/null",
			Cluster: server.ClusterOpts{
				Name: "TEST",
				Port: -1,
			},
			Routes:        routes,
			Accounts:      []*server.Account{sa, ua},
			SystemAccount: "SYSTEM",
			Users: []*server.User{
				{Account: sa, Username: "SYS", Password: "PASS"},
				{Account: ua, Username: "USER", Password: "PASS"},
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

		routes = append(routes, &url.URL{Host: fmt.Sprintf("localhost:%d", s.ClusterAddr().Port)})
		servers = append(servers, s)
	}

	if len(servers) != 3 {
		t.Fatalf("servers did not start")
	}

	nc, err := nats.Connect(servers[0].ClientURL(), nats.UserInfo("SYS", "PASS"))
	if err != nil {
		t.Fatalf("client start failed: %s", err)
	}
	defer nc.Close()

	waitForClusterReady(t, servers, nc)

	cb(t, servers, nc)
}

func waitForClusterReady(t *testing.T, srv []*server.Server, nc *nats.Conn) {
	t.Helper()

	pinger := &balancer{nc: nc, log: api.NewDiscardLogger()}
	deadline := time.Now().Add(10 * time.Second)

	for {
		res, err := pinger.reqMany(context.Background(), "$SYS.REQ.SERVER.PING", nil, len(srv))
		if err == nil && len(res) == len(srv) {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("cluster did not form, only %d of %d servers answered system pings", len(res), len(srv))
		}

		time.Sleep(25 * time.Millisecond)
	}
}
