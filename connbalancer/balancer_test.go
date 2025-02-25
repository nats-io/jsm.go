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
	"os"
	"testing"
	"time"

	"github.com/nats-io/jsm.go/api"
	testapi "github.com/nats-io/jsm.go/test/testing_client/api"
	"github.com/nats-io/jsm.go/test/testing_client/srvtest"
	"github.com/nats-io/nats.go"
)

func TestSubjectInterest(t *testing.T) {
	withCluster(t, func(t *testing.T, nc *nats.Conn, _ *srvtest.Client, servers []*testapi.ManagedServer) {
		client1, err := nats.Connect(servers[2].URL, nats.UserInfo("user1", "password"))
		if err != nil {
			t.Fatalf("could not create client")
		}
		defer client1.Close()

		_, err = client1.SubscribeSync("X.>")
		if err != nil {
			t.Fatalf("sub failed")
		}

		client2, err := nats.Connect(servers[2].URL, nats.UserInfo("user1", "password"))
		if err != nil {
			t.Fatalf("could not create client")
		}
		defer client2.Close()

		checkBalanced(t, nc, 0, ConnectionSelector{
			Account:         "USERS1",
			SubjectInterest: "foo",
		})

		checkBalanced(t, nc, 1, ConnectionSelector{
			Account:         "USERS1",
			SubjectInterest: "X.>",
		})
	})
}

func TestAccountLimit(t *testing.T) {
	withCluster(t, func(t *testing.T, nc *nats.Conn, _ *srvtest.Client, servers []*testapi.ManagedServer) {
		client1, err := nats.Connect(servers[2].URL)
		if err != nil {
			t.Fatalf("could not create client")
		}
		defer client1.Close()

		client2, err := nats.Connect(servers[2].URL, nats.UserInfo("system", "password"))
		if err != nil {
			t.Fatalf("could not create client")
		}
		defer client2.Close()

		checkBalanced(t, nc, 0, ConnectionSelector{
			Account: "FOO",
		})

		checkBalanced(t, nc, 1, ConnectionSelector{
			Account: "USERS1",
		})
	})
}

func TestClientIdleLimit(t *testing.T) {
	withCluster(t, func(t *testing.T, nc *nats.Conn, _ *srvtest.Client, servers []*testapi.ManagedServer) {
		client, err := nats.Connect(servers[2].URL, nats.UserInfo("user1", "password"))
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
	withCluster(t, func(t *testing.T, nc *nats.Conn, _ *srvtest.Client, servers []*testapi.ManagedServer) {
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
				client, err := nats.Connect(servers[2].URL, nats.UserInfo("user1", "password"))
				if err != nil {
					t.Fatalf("could not create client")
				}
				defer client.Close()

				checkBalanced(t, nc, tc.expect, ConnectionSelector{
					ServerName: servers[tc.targetServer].Name,
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

func withCluster(t *testing.T, fn func(*testing.T, *nats.Conn, *srvtest.Client, []*testapi.ManagedServer)) {
	t.Helper()

	url := os.Getenv("TESTER_URL")
	if url == "" {
		url = "nats://localhost:4222"
	}

	client := srvtest.New(t, url)
	t.Cleanup(func() {
		client.Reset(t)
	})

	client.WithCluster(t, 3, func(t *testing.T, nc *nats.Conn, servers []*testapi.ManagedServer) {
		nc.Close()
		nc, err := nats.Connect(servers[0].URL, nats.UserInfo("system", "password"))
		if err != nil {
			t.Fatalf("could not create system client")
		}

		fn(t, nc, client, servers)
	})
}
