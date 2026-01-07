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
		for i := range 10 {
			client, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("USER", "PASS"))
			if err != nil {
				t.Fatalf("could not create client %d: %v", i, err)
			}
			defer client.Close()
		}

		checkBalancedInRange(t, nc, 5, 7, ConnectionSelector{})

		time.Sleep(500 * time.Millisecond)

		checkBalancedInRange(t, nc, 0, 1, ConnectionSelector{})
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

func withCluster(t *testing.T, cb func(t *testing.T, servers []*server.Server, nc *nats.Conn)) {
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
		sa := server.NewAccount("SYSTEM")
		ua := server.NewAccount("USERS")

		opts := &server.Options{
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

	cb(t, servers, nc)
}
