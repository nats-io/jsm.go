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
	"errors"
	"fmt"
	"maps"
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

func TestClusterIndexes(t *testing.T) {
	type srv struct{ id, name, cluster, domain string }

	for _, tc := range []struct {
		name     string
		servers  []srv
		routes   map[string][]string
		expected map[string]int
	}{
		{
			name:     "one routed cluster",
			servers:  []srv{{"A", "n1", "SITE", ""}, {"B", "n2", "SITE", ""}, {"C", "n3", "SITE", ""}},
			routes:   map[string][]string{"A": {"B", "C"}, "B": {"A", "C"}, "C": {"A", "B"}},
			expected: map[string]int{},
		},
		{
			name:     "partial mesh is one cluster",
			servers:  []srv{{"A", "n1", "SITE", ""}, {"B", "n2", "SITE", ""}, {"C", "n3", "SITE", ""}},
			routes:   map[string][]string{"A": {"B"}, "B": {"C"}},
			expected: map[string]int{},
		},
		{
			name:     "route reported by one side only",
			servers:  []srv{{"A", "n1", "SITE", ""}, {"B", "n2", "SITE", ""}},
			routes:   map[string][]string{"B": {"A"}},
			expected: map[string]int{},
		},
		{
			name: "same names ordered by lowest server id",
			servers: []srv{
				{"NB1", "n1", "SITE", ""}, {"NB2", "n2", "SITE", ""},
				{"NA2", "n1", "SITE", ""}, {"NC1", "n2", "SITE", ""},
			},
			routes:   map[string][]string{"NB1": {"NB2"}, "NA2": {"NC1"}},
			expected: map[string]int{"NA2": 0, "NC1": 0, "NB1": 1, "NB2": 1},
		},
		{
			name: "member names order before server id",
			servers: []srv{
				{"NA1", "b1", "SITE", ""},
				{"NZ1", "a1", "SITE", ""}, {"NZ2", "a2", "SITE", ""},
			},
			routes:   map[string][]string{"NZ1": {"NZ2"}},
			expected: map[string]int{"NZ1": 0, "NZ2": 0, "NA1": 1},
		},
		{
			name:     "standalone leaf sites with the same name",
			servers:  []srv{{"NB", "nats-0", "nats-0", ""}, {"NA", "nats-0", "nats-0", ""}},
			expected: map[string]int{"NA": 0, "NB": 1},
		},
		{
			name:     "same cluster name in different domains",
			servers:  []srv{{"A", "n1", "SITE", "x"}, {"B", "n1", "SITE", "y"}},
			expected: map[string]int{},
		},
		{
			name:     "different cluster names",
			servers:  []srv{{"A", "n1", "SITEA", ""}, {"B", "n1", "SITEB", ""}},
			expected: map[string]int{},
		},
		{
			name:     "no cluster name",
			servers:  []srv{{"A", "n1", "", ""}, {"B", "n1", "", ""}},
			expected: map[string]int{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			infos := make(map[string]*server.ServerInfo)
			for _, s := range tc.servers {
				infos[s.id] = &server.ServerInfo{ID: s.id, Name: s.name, Cluster: s.cluster, Domain: s.domain}
			}

			got := clusterIndexes(infos, tc.routes, api.NewDiscardLogger())
			if !maps.Equal(tc.expected, got) {
				t.Fatalf("expected %v, got %v", tc.expected, got)
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

	_, res := runJetStream001(t, reader)
	if res.Outcome != audit.Pass {
		t.Fatalf("expected JETSTREAM_001 to pass, got %s: %s (streams in USERS1: %v)", res.Outcome, res.Examples.String(), reader.AccountStreams("USERS1"))
	}
}

func TestGatherSameClusterNameInDifferentSites(t *testing.T) {
	ntfc := ntfclient.New(t, test.NTFURL())
	defer ntfc.Close(t)

	hub := ntfc.CreateServer(t, true,
		ntfclient.WithJetStream("domain: hub"),
		ntfclient.WithLeafNode("leafnodes { port: {{ .Ports.leafnode }} }"),
	)
	defer hub.Destroy(t)

	relay := ntfc.CreateServer(t, true,
		ntfclient.WithJetStream("domain: relay"),
		ntfclient.WithLeafNode(fmt.Sprintf(`leafnodes {
	port: {{ .Ports.leafnode }}
	remotes: [ { url: "nats://system:password@localhost:%d", account: "$SYS" } ]
}`, hub.Servers[0].Ports["leafnode"])),
	)
	defer relay.Destroy(t)

	accounts := `accounts {
  USERS1 { jetstream: enabled, users: [{ user: user1, password: password }] }
  "$SYS" { users: [{ user: system, password: password }] }
}
system_account: "$SYS"
no_auth_user: user1`

	leafRemotes := make([]string, 2)
	for i, srv := range []*ntfclient.Instance{hub, relay} {
		leafRemotes[i] = fmt.Sprintf(`leafnodes { remotes: [ { url: "nats://system:password@localhost:%d", account: "$SYS" } ] }`, srv.Servers[0].Ports["leafnode"])
	}

	for i, msgs := range []int{5, 9} {
		site := ntfc.CreateCluster(t, 3, true, ntfclient.WithTemplate(fmt.Sprintf(`server_name: n{{ .ServerIndex }}
listen: "{{ .Host }}:{{ .ClientPort }}"
jetstream { store_dir: "{{ .StoreDir }}" }
cluster {
  name: SITE
  listen: "{{ .Host }}:{{ .ClusterPort }}"
  routes: [{{ range $i, $route := .Routes }}{{ if $i }}, {{ end }}"nats://{{ $route }}"{{ end }}]
}
%s
%s
`, leafRemotes[i], accounts)))
		defer site.Destroy(t)

		nc, err := nats.Connect(site.Servers[0].URL, nats.UseOldRequestStyle())
		checkErr(t, err, "connect failed")
		defer nc.Close()
		ntfc.WaitForJetStream(t, nc)

		mgr, err := jsm.New(nc, jsm.WithTimeout(5*time.Second))
		checkErr(t, err, "manager failed")

		deadline := time.Now().Add(20 * time.Second)
		for {
			_, err = mgr.NewStream("ORDERS", jsm.Subjects("orders"), jsm.MemoryStorage(), jsm.Replicas(3))
			if err == nil || time.Now().After(deadline) {
				break
			}
			time.Sleep(250 * time.Millisecond)
		}
		checkErr(t, err, "create stream failed")

		for i := 0; i < msgs; i++ {
			for {
				_, err = nc.Request("orders", []byte("x"), time.Second)
				if !errors.Is(err, nats.ErrNoResponders) || time.Now().After(deadline) {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			checkErr(t, err, "publish failed")
		}
	}

	for _, leafRemote := range leafRemotes {
		standalone := ntfc.CreateServer(t, false, ntfclient.WithTemplate(fmt.Sprintf(`server_name: nats-0
listen: "{{ .Host }}:{{ .ClientPort }}"
%s
%s
`, leafRemote, accounts)))
		defer standalone.Destroy(t)
	}

	sysNC, err := nats.Connect(hub.Servers[0].URL, nats.UserInfo("system", "password"))
	checkErr(t, err, "system connect failed")
	defer sysNC.Close()

	waitForServers(t, sysNC, 10)

	archivePath := filepath.Join(t.TempDir(), "audit.zip")
	cfg := NewCaptureConfiguration()
	cfg.TargetPath = archivePath
	cfg.LogLevel = api.WarnLevel
	cfg.Include.Streams = true
	cfg.Include.ServerEndpoints = true
	checkErr(t, Gather(sysNC, cfg), "gather failed")

	reader, err := archive.NewReader(archivePath)
	checkErr(t, err, "open archive failed")
	defer reader.Close()

	for _, tc := range []struct {
		cluster string
		servers []string
	}{
		{"SITE", []string{"n1", "n2", "n3"}},
		{"nats-0", []string{"nats-0"}},
	} {
		for _, index := range []string{"0", "1"} {
			ref := archive.ClusterRef{Name: tc.cluster, Index: index}
			if servers := reader.ClusterServerNames(ref); !slices.Equal(tc.servers, servers) {
				t.Fatalf("cluster %s: expected servers %v, got %v (clusters: %v)", ref, tc.servers, servers, reader.Clusters())
			}
		}
	}

	expectedStreams := []archive.StreamRef{
		{Name: "ORDERS", Cluster: archive.ClusterRef{Name: "SITE", Index: "0"}},
		{Name: "ORDERS", Cluster: archive.ClusterRef{Name: "SITE", Index: "1"}},
	}
	if streams := reader.AccountStreams("USERS1"); !slices.Equal(expectedStreams, streams) {
		t.Fatalf("expected streams %v, got %v", expectedStreams, streams)
	}

	analysis, res := runJetStream001(t, reader)
	if res.Outcome != audit.Pass {
		t.Fatalf("expected JETSTREAM_001 to pass, got %s: %s", res.Outcome, res.Examples.String())
	}

	var shared []string
	for _, s := range analysis.NumberedClusters {
		shared = append(shared, fmt.Sprintf("%s:%d", s.Name, len(s.Clusters)))
	}
	if expected := []string{"SITE:2", "nats-0:2"}; !slices.Equal(expected, shared) {
		t.Fatalf("expected numbered clusters %v, got %v", expected, shared)
	}
}

func runJetStream001(t *testing.T, reader *archive.Reader) (*audit.Analysis, audit.CheckResult) {
	t.Helper()

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
	return analysis, analysis.Results[idx]
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
