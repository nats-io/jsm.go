// Copyright 2026 The NATS Authors
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

package gather

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit"
	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/jsm.go/test"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	ntfclient "github.com/synadia-io/orbit.go/ntf-client"
)

func TestMain(m *testing.M) {
	test.RunWithNTF(m, 26000)
}

func checkErr(t testing.TB, err error, m string) {
	t.Helper()
	if err == nil {
		return
	}
	t.Fatal(m + ": " + err.Error())
}

func decodeServerResponse(t *testing.T, data any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(server.ServerAPIResponse{Data: data})
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}

	return decoded
}

func TestHasNextPage(t *testing.T) {
	const limit = 4

	cases := []struct {
		name     string
		endpoint string
		decoded  map[string]any
		want     bool
		wantErr  bool
	}{
		{"full page", "SUBSZ", decodeServerResponse(t, &server.Subsz{Subs: make([]server.SubDetail, limit)}), true, false},
		{"partial page", "SUBSZ", decodeServerResponse(t, &server.Subsz{Subs: make([]server.SubDetail, limit-1)}), false, false},
		{"SUBSZ past the end", "SUBSZ", decodeServerResponse(t, &server.Subsz{Offset: limit, Limit: limit, Total: limit}), false, false},
		{"JSZ past the end", "JSZ", decodeServerResponse(t, &server.JSInfo{Total: limit}), false, false},
		{"list is not an array", "SUBSZ", map[string]any{"data": map[string]any{"subscriptions_list": "x"}}, false, true},
	}

	g := &gather{log: newLogger(nil, api.ErrorLevel)}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := g.hasNextPage(tc.endpoint, tc.decoded, limit)
			if (err != nil) != tc.wantErr {
				t.Fatalf("hasNextPage() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("hasNextPage() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGatherSameStreamNameInDifferentDomains(t *testing.T) {
	ntfc := ntfclient.New(t, test.NTFURL())
	defer ntfc.Close(t)

	hub := ntfc.CreateServer(t, true,
		ntfclient.WithJetStream("domain: hub"),
		ntfclient.WithLeafNode("leafnodes { port: {{ .Ports.leafnode }} }"),
	)
	defer hub.Destroy(t)

	hubLeafPort := hub.Servers[0].Ports["leafnode"]
	leaf := ntfc.CreateServer(t, true,
		ntfclient.WithJetStream("domain: leaf"),
		ntfclient.WithLeafNode(fmt.Sprintf(`leafnodes {
	remotes: [
		{ url: "nats://user1:password@localhost:%d", account: USERS1 }
		{ url: "nats://system:password@localhost:%d", account: "$SYS" }
	]
}`, hubLeafPort, hubLeafPort)),
	)
	defer leaf.Destroy(t)

	sysNC, err := nats.Connect(hub.Servers[0].URL, nats.UserInfo("system", "password"))
	checkErr(t, err, "system connect failed")
	defer sysNC.Close()

	waitForServers(t, sysNC, 2)

	for _, tc := range []struct {
		url     string
		subject string
		msgs    int
	}{
		{hub.Servers[0].URL, "hub.orders", 100},
		{leaf.Servers[0].URL, "leaf.orders", 1},
	} {
		nc, err := nats.Connect(tc.url, nats.UseOldRequestStyle())
		checkErr(t, err, "connect failed")
		defer nc.Close()

		mgr, err := jsm.New(nc, jsm.WithTimeout(5*time.Second))
		checkErr(t, err, "manager failed")

		_, err = mgr.NewStream("ORDERS", jsm.Subjects(tc.subject), jsm.MemoryStorage())
		checkErr(t, err, "create stream failed")

		for i := 0; i < tc.msgs; i++ {
			_, err = nc.Request(tc.subject, []byte("x"), time.Second)
			checkErr(t, err, "publish failed")
		}
	}

	archivePath := filepath.Join(t.TempDir(), "audit.zip")
	cfg := NewCaptureConfiguration()
	cfg.TargetPath = archivePath
	cfg.LogLevel = api.WarnLevel
	cfg.Include.Streams = true
	checkErr(t, Gather(sysNC, cfg), "gather failed")

	reader, err := archive.NewReader(archivePath)
	checkErr(t, err, "open archive failed")
	defer reader.Close()

	cc, err := audit.NewDefaultCheckCollection()
	checkErr(t, err, "check collection failed")
	var skip []string
	cc.EachCheck(func(c *audit.Check) {
		if c.Code != "JETSTREAM_001" {
			skip = append(skip, c.Code)
		}
	})
	cc.SkipChecks(skip...)

	analysis := cc.Run(reader, 0, api.NewDefaultLogger(api.WarnLevel))
	idx := slices.IndexFunc(analysis.Results, func(r audit.CheckResult) bool { return r.Check.Code == "JETSTREAM_001" })
	if idx == -1 {
		t.Fatal("JETSTREAM_001 result not found")
	}

	res := analysis.Results[idx]
	if res.Outcome != audit.Pass {
		t.Fatalf("expected JETSTREAM_001 to pass, got %s: %s (streams in USERS1: %v)", res.Outcome, res.Examples.String(), reader.AccountStreams("USERS1"))
	}
}

func waitForServers(t *testing.T, nc *nats.Conn, want int) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		inbox := nats.NewInbox()
		sub, err := nc.SubscribeSync(inbox)
		checkErr(t, err, "subscribe failed")

		checkErr(t, nc.PublishRequest("$SYS.REQ.SERVER.PING", inbox, nil), "ping failed")

		got := 0
		for {
			_, err := sub.NextMsg(250 * time.Millisecond)
			if err != nil {
				break
			}
			got++
		}
		sub.Unsubscribe()

		if got >= want {
			return
		}
	}

	t.Fatalf("did not see %d servers", want)
}
