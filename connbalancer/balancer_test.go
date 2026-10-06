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
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/test"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	ntfclient "github.com/synadia-io/orbit.go/ntf-client"
	ntfapi "github.com/synadia-io/orbit.go/ntf/api"
)

func TestSubjectInterest(t *testing.T) {
	withCluster(t, func(t *testing.T, srv []*ntfapi.ManagedServer, nc *nats.Conn) {
		for i := 0; i < 5; i++ {
			client, err := nats.Connect(srv[2].URL, nats.UserInfo("USER", "PASS"))
			if err != nil {
				t.Fatalf("could not create client")
			}
			defer client.Close()
			_, err = client.SubscribeSync("X.>")
			if err != nil {
				t.Fatalf("sub failed")
			}
		}

		client2, err := nats.Connect(srv[2].URL, nats.UserInfo("USER", "PASS"))
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
	withCluster(t, func(t *testing.T, srv []*ntfapi.ManagedServer, nc *nats.Conn) {
		for i := 0; i < 5; i++ {
			client, err := nats.Connect(srv[2].URL, nats.UserInfo("USER", "PASS"))
			if err != nil {
				t.Fatalf("could not create client")
			}
			defer client.Close()
		}

		client2, err := nats.Connect(srv[2].URL, nats.UserInfo("SYS", "PASS"))
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
	withCluster(t, func(t *testing.T, srv []*ntfapi.ManagedServer, nc *nats.Conn) {
		for i := 0; i < 5; i++ {
			client, err := nats.Connect(srv[2].URL, nats.UserInfo("USER", "PASS"))
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
	withCluster(t, func(t *testing.T, srv []*ntfapi.ManagedServer, nc *nats.Conn) {
		t.Run("Only ourselves on selected server", func(t *testing.T) {
			checkBalancedInRange(t, nc, 0, 0, ConnectionSelector{
				ServerName: srv[0].Name,
			})
		})

		t.Run("Connections on specific server", func(t *testing.T) {
			for i := 0; i < 5; i++ {
				client, err := nats.Connect(srv[2].URL, nats.UserInfo("USER", "PASS"))
				if err != nil {
					t.Fatalf("could not create client")
				}
				defer client.Close()
			}

			checkBalancedInRange(t, nc, 0, 0, ConnectionSelector{
				ServerName: srv[2].Name,
			})
		})
	})
}

func TestSuccessiveBalanceRuns(t *testing.T) {
	withCluster(t, func(t *testing.T, srv []*ntfapi.ManagedServer, nc *nats.Conn) {
		const clients = 10

		monitorConns := clusterConnections(t, srv, nc)

		for i := range clients {
			client, err := nats.Connect(srv[2].URL, nats.UserInfo("USER", "PASS"))
			if err != nil {
				t.Fatalf("could not create client %d: %v", i, err)
			}
			defer client.Close()
		}

		totalConns := monitorConns + clients
		waitForConnections(t, srv, nc, totalConns)

		checkBalancedInRange(t, nc, 5, 7, ConnectionSelector{})

		balancer, err := New(nc, 0, api.NewDiscardLogger(), ConnectionSelector{})
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}

		deadline := time.Now().Add(10 * time.Second)

		for {
			waitForConnections(t, srv, nc, totalConns)

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
	withCluster(t, func(t *testing.T, srv []*ntfapi.ManagedServer, nc *nats.Conn) {
		for range 15 {
			client, err := nats.Connect(srv[2].URL, nats.UserInfo("USER", "PASS"))
			if err != nil {
				t.Fatalf("could not create client: %v", err)
			}
			defer client.Close()
		}

		for range 3 {
			client, err := nats.Connect(srv[1].URL, nats.UserInfo("USER", "PASS"))
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

func clusterConnections(t *testing.T, srv []*ntfapi.ManagedServer, nc *nats.Conn) int {
	t.Helper()

	pinger := &balancer{nc: nc, log: api.NewDiscardLogger()}
	res, err := pinger.reqMany(context.Background(), "$SYS.REQ.SERVER.PING", nil, len(srv))
	if err != nil {
		t.Fatalf("server ping failed: %v", err)
	}
	if len(res) != len(srv) {
		t.Fatalf("expected %d servers to answer the ping but got %d", len(srv), len(res))
	}

	var total int
	for _, msg := range res {
		var stats server.ServerStatsMsg
		err = json.Unmarshal(msg.Data, &stats)
		if err != nil {
			t.Fatalf("invalid ping response: %v", err)
		}
		total += stats.Stats.Connections
	}

	return total
}

func waitForConnections(t *testing.T, srv []*ntfapi.ManagedServer, nc *nats.Conn, expect int) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for {
		total := clusterConnections(t, srv, nc)
		if total == expect {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("expected %d connections in the cluster but found %d", expect, total)
		}

		time.Sleep(25 * time.Millisecond)
	}
}

func waitForClusterReady(t *testing.T, srv []*ntfapi.ManagedServer, nc *nats.Conn) {
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

func TestMain(m *testing.M) {
	test.RunWithNTF(m, 22000)
}

const clusterAccounts = `
accounts {
	SYSTEM { users = [ { user: "SYS", pass: "PASS" } ] }
	USERS { users = [ { user: "USER", pass: "PASS" } ] }
}
`

func withCluster(t *testing.T, cb func(t *testing.T, servers []*ntfapi.ManagedServer, nc *nats.Conn)) {
	t.Helper()

	ntfc := ntfclient.New(t, test.NTFURL())
	defer ntfc.Close(t)

	instance := ntfc.CreateCluster(t, 3, false,
		ntfclient.WithAccounts(clusterAccounts),
		ntfclient.WithSystemAccount(`system_account: "SYSTEM"`),
		ntfclient.WithAuthorization("# no default user"))
	defer instance.Destroy(t)

	nc, err := nats.Connect(instance.Servers[0].URL, nats.UserInfo("SYS", "PASS"))
	if err != nil {
		t.Fatalf("client start failed: %s", err)
	}
	defer nc.Close()

	waitForClusterReady(t, instance.Servers, nc)

	cb(t, instance.Servers, nc)
}
