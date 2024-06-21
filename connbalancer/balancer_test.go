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
		client1, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("USER", "PASS"))
		if err != nil {
			t.Fatalf("could not create client")
		}
		defer client1.Close()
		_, err = client1.SubscribeSync("X.>")
		if err != nil {
			t.Fatalf("sub failed")
		}

		client2, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("USER", "PASS"))
		if err != nil {
			t.Fatalf("could not create client")
		}
		defer client2.Close()

		checkBalanced(t, nc, 0, ConnectionSelector{
			Account:         "USERS",
			SubjectInterest: "foo",
		})

		checkBalanced(t, nc, 1, ConnectionSelector{
			Account:         "USERS",
			SubjectInterest: "X.>",
		})
	})
}

func TestAccountLimit(t *testing.T) {
	withCluster(t, func(t *testing.T, srv []*server.Server, nc *nats.Conn) {
		client1, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("USER", "PASS"))
		if err != nil {
			t.Fatalf("could not create client")
		}
		defer client1.Close()

		client2, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("SYS", "PASS"))
		if err != nil {
			t.Fatalf("could not create client")
		}
		defer client2.Close()

		checkBalanced(t, nc, 0, ConnectionSelector{
			Account: "FOO",
		})

		checkBalanced(t, nc, 1, ConnectionSelector{
			Account: "USERS",
		})
	})
}

func TestClientIdleLimit(t *testing.T) {
	withCluster(t, func(t *testing.T, srv []*server.Server, nc *nats.Conn) {
		client, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("USER", "PASS"))
		if err != nil {
			t.Fatalf("could not create client")
		}
		defer client.Close()

		checkBalanced(t, nc, 0, ConnectionSelector{
			Idle: time.Minute,
		})

		checkBalanced(t, nc, 1, ConnectionSelector{
			Idle: time.Millisecond,
		})
	})
}

func TestServerNameLimit(t *testing.T) {
	withCluster(t, func(t *testing.T, srv []*server.Server, nc *nats.Conn) {
		tests := []struct {
			name         string
			targetServer int
			expect       int
		}{
			{"Only ourselves on selected server", 0, 0},
			{"Only the connection on that server", 2, 1},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				client, err := nats.Connect(srv[2].ClientURL(), nats.UserInfo("USER", "PASS"))
				if err != nil {
					t.Fatalf("could not create client")
				}
				defer client.Close()

				checkBalanced(t, nc, tc.expect, ConnectionSelector{
					ServerName: srv[tc.targetServer].Name(),
				})
			})
		}
	})
}

func checkBalanced(t *testing.T, nc *nats.Conn, expect int, s ConnectionSelector) {
	t.Helper()

	// dont kick ourselves or connections on other servers
	balancer, err := New(nc, 0, api.NewDiscardLogger(), s)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	balanced, err := balancer.Balance(context.Background())
	if err != nil {
		t.Fatalf("balance failed: %v", err)
	}
	if balanced != expect {
		t.Fatalf("Expected to balance %d connections but balanced %d", expect, balanced)
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
