package serverdata

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/nats-server/v2/server"
)

// newTestArchive creates a temporary archive file, calls the populate function
// to add artifacts, closes the writer, and returns an *AuditArchive ready for testing.
// The caller does not need to clean up; t.TempDir() handles that.
func newTestArchive(t *testing.T, populate func(w *archive.Writer)) *AuditArchive {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.zip")

	w, err := archive.NewWriter(path)
	if err != nil {
		t.Fatal(err)
	}

	populate(w)

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	a, err := NewAuditArchive(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })

	return a
}

func TestArchiveVarz(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		resp := &server.ServerAPIVarzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data:   &server.Varz{MaxConn: 100},
		}
		w.Add(resp, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerVars())
	})

	results, err := a.Varz(server.VarzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Server.Name != "srv-1" {
		t.Errorf("expected server name srv-1, got %s", results[0].Server.Name)
	}
	if results[0].Data.MaxConn != 100 {
		t.Errorf("expected MaxConn 100, got %d", results[0].Data.MaxConn)
	}
}

func TestArchiveConnz(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		resp := &server.ServerAPIConnzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data: &server.Connz{
				NumConns: 2,
				Conns: []*server.ConnInfo{
					{Name: "conn-1", Account: "acct-A"},
					{Name: "conn-2", Account: "acct-A"},
				},
			},
		}
		w.Add(resp,
			archive.TagAccount("acct-A"),
			archive.TagCluster("c1"),
			archive.TagServer("srv-1"),
			archive.TagAccountConnections())
	})

	results, err := a.Connz(server.ConnzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(results[0].Data.Conns) != 2 {
		t.Errorf("expected 2 conns, got %d", len(results[0].Data.Conns))
	}
}

func TestArchiveConnzMergesAccounts(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		// Two accounts on the same server should be merged into one result
		resp1 := &server.ServerAPIConnzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data: &server.Connz{
				Conns: []*server.ConnInfo{{Name: "conn-1"}},
			},
		}
		resp2 := &server.ServerAPIConnzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data: &server.Connz{
				Conns: []*server.ConnInfo{{Name: "conn-2"}},
			},
		}
		w.Add(resp1,
			archive.TagAccount("acct-A"),
			archive.TagCluster("c1"),
			archive.TagServer("srv-1"),
			archive.TagAccountConnections())
		w.Add(resp2,
			archive.TagAccount("acct-B"),
			archive.TagCluster("c1"),
			archive.TagServer("srv-1"),
			archive.TagAccountConnections())
	})

	results, err := a.Connz(server.ConnzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 merged result, got %d", len(results))
	}
	if len(results[0].Data.Conns) != 2 {
		t.Errorf("expected 2 merged conns, got %d", len(results[0].Data.Conns))
	}
}

func TestArchiveRoutez(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		resp := &server.ServerAPIRoutezResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data: &server.Routez{
				Routes: []*server.RouteInfo{{RemoteName: "srv-2"}},
			},
		}
		w.Add(resp, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerRoutes())
	})

	results, err := a.Routez(server.RoutezEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(results[0].Data.Routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(results[0].Data.Routes))
	}
}

func TestArchiveGatewayz(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		resp := &server.ServerAPIGatewayzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data:   &server.Gatewayz{Name: "gw-east"},
		}
		w.Add(resp, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerGateways())
	})

	results, err := a.Gatewayz(server.GatewayzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Data.Name != "gw-east" {
		t.Errorf("expected gateway name gw-east, got %s", results[0].Data.Name)
	}
}

func TestArchiveLeafz(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		resp := &server.ServerAPILeafzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data: &server.Leafz{
				Leafs: []*server.LeafInfo{{Name: "leaf-1"}},
			},
		}
		w.Add(resp, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerLeafs())
	})

	results, err := a.Leafz(server.LeafzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(results[0].Data.Leafs) != 1 {
		t.Errorf("expected 1 leaf, got %d", len(results[0].Data.Leafs))
	}
}

func TestArchiveSubsz(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		resp := &server.ServerAPISubszResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data:   &server.Subsz{Total: 10},
		}
		w.Add(resp, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerSubs())
	})

	results, err := a.Subsz(server.SubszEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Data.Total != 10 {
		t.Errorf("expected 10 subs, got %d", results[0].Data.Total)
	}
}

func TestArchiveJsz(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		resp := &server.ServerAPIJszResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data: &server.JSInfo{
				JetStreamStats: server.JetStreamStats{Memory: 1024},
			},
		}
		w.Add(resp, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerJetStream())
	})

	results, err := a.Jsz(server.JszEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Data.Memory != 1024 {
		t.Errorf("expected Memory 1024, got %d", results[0].Data.Memory)
	}
}

func TestArchiveHealthz(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		resp := &server.ServerAPIHealthzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data:   &server.HealthStatus{Status: "ok"},
		}
		w.Add(resp, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerHealth())
	})

	results, err := a.Healthz(server.HealthzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Data.Status != "ok" {
		t.Errorf("expected status ok, got %s", results[0].Data.Status)
	}
}

func TestArchiveAccountz(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		resp := &server.ServerAPIAccountzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data: &server.Accountz{
				Accounts: []string{"acct-A", "acct-B"},
			},
		}
		w.Add(resp, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerAccounts())
	})

	results, err := a.Accountz(server.AccountzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(results[0].Data.Accounts) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(results[0].Data.Accounts))
	}
}

func TestArchiveAccountzDetail(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		air := &accountInfoResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data:   &server.AccountInfo{AccountName: "acct-A"},
		}
		w.Add(air,
			archive.TagCluster("c1"),
			archive.TagServer("srv-1"),
			archive.TagAccount("acct-A"),
			archive.TagAccountInfo())
	})

	opts := server.AccountzEventOptions{}
	opts.AccountzOptions.Account = "acct-A"
	results, err := a.Accountz(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Data.Account.AccountName != "acct-A" {
		t.Errorf("expected account acct-A, got %s", results[0].Data.Account.AccountName)
	}
}

func TestArchiveAccountzDetailMissing(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		// Add nothing for the requested account
		resp := &server.ServerAPIVarzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data:   &server.Varz{},
		}
		w.Add(resp, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerVars())
	})

	opts := server.AccountzEventOptions{}
	opts.AccountzOptions.Account = "no-such-account"
	results, err := a.Accountz(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for missing account, got %d", len(results))
	}
}

func TestArchiveStatz(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		varz := &server.ServerAPIVarzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1", Cluster: "c1"},
			Data: &server.Varz{
				Connections:      5,
				TotalConnections: 10,
				InMsgs:           100,
				OutMsgs:          200,
				InBytes:          1000,
				OutBytes:         2000,
				Mem:              4096,
				CPU:              1.5,
				Cores:            4,
				Subscriptions:    50,
				SlowConsumers:    2,
			},
		}
		w.Add(varz, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerVars())

		routez := &server.ServerAPIRoutezResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data: &server.Routez{
				Routes: []*server.RouteInfo{
					{Rid: 1, RemoteName: "srv-2", InMsgs: 10, OutMsgs: 20, InBytes: 100, OutBytes: 200, Pending: 5},
				},
			},
		}
		w.Add(routez, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerRoutes())

		gatewayz := &server.ServerAPIGatewayzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data: &server.Gatewayz{
				OutboundGateways: map[string]*server.RemoteGatewayz{
					"gw-west": {Connection: &server.ConnInfo{OutMsgs: 30, OutBytes: 300}},
				},
			},
		}
		w.Add(gatewayz, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerGateways())
	})

	results, err := a.Statz(server.StatszEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	msg := results[0]
	if msg.Server.Name != "srv-1" {
		t.Errorf("expected server name srv-1, got %s", msg.Server.Name)
	}
	if msg.Stats.Connections != 5 {
		t.Errorf("expected 5 connections, got %d", msg.Stats.Connections)
	}
	if msg.Stats.Sent.Msgs != 200 {
		t.Errorf("expected 200 sent msgs, got %d", msg.Stats.Sent.Msgs)
	}
	if msg.Stats.Received.Msgs != 100 {
		t.Errorf("expected 100 received msgs, got %d", msg.Stats.Received.Msgs)
	}
	if msg.Stats.Mem != 4096 {
		t.Errorf("expected Mem 4096, got %d", msg.Stats.Mem)
	}
	if msg.Stats.CPU != 1.5 {
		t.Errorf("expected CPU 1.5, got %f", msg.Stats.CPU)
	}
	if msg.Stats.NumSubs != 50 {
		t.Errorf("expected 50 subs, got %d", msg.Stats.NumSubs)
	}
	if msg.Stats.SlowConsumers != 2 {
		t.Errorf("expected 2 slow consumers, got %d", msg.Stats.SlowConsumers)
	}

	// Route stats
	if len(msg.Stats.Routes) != 1 {
		t.Fatalf("expected 1 route stat, got %d", len(msg.Stats.Routes))
	}
	if msg.Stats.Routes[0].Name != "srv-2" {
		t.Errorf("expected route name srv-2, got %s", msg.Stats.Routes[0].Name)
	}
	if msg.Stats.Routes[0].Sent.Msgs != 20 {
		t.Errorf("expected route sent 20, got %d", msg.Stats.Routes[0].Sent.Msgs)
	}
	if msg.Stats.Routes[0].Pending != 5 {
		t.Errorf("expected route pending 5, got %d", msg.Stats.Routes[0].Pending)
	}

	// Gateway stats
	if len(msg.Stats.Gateways) != 1 {
		t.Fatalf("expected 1 gateway stat, got %d", len(msg.Stats.Gateways))
	}
	if msg.Stats.Gateways[0].Name != "gw-west" {
		t.Errorf("expected gateway name gw-west, got %s", msg.Stats.Gateways[0].Name)
	}
	if msg.Stats.Gateways[0].Sent.Msgs != 30 {
		t.Errorf("expected gateway sent 30, got %d", msg.Stats.Gateways[0].Sent.Msgs)
	}
}

func TestArchiveMultipleServers(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		r1 := &server.ServerAPIVarzResponse{
			Server: &server.ServerInfo{Name: "srv-a", ID: "id-a"},
			Data:   &server.Varz{},
		}
		r2 := &server.ServerAPIVarzResponse{
			Server: &server.ServerInfo{Name: "srv-b", ID: "id-b"},
			Data:   &server.Varz{},
		}
		w.Add(r1, archive.TagCluster("c1"), archive.TagServer("srv-a"), archive.TagServerVars())
		w.Add(r2, archive.TagCluster("c1"), archive.TagServer("srv-b"), archive.TagServerVars())
	})

	results, err := a.Varz(server.VarzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	names := map[string]bool{}
	for _, r := range results {
		names[r.Server.Name] = true
	}
	if !names["srv-a"] || !names["srv-b"] {
		t.Errorf("expected srv-a and srv-b, got %v", names)
	}
}

func TestArchiveFilterByName(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		r1 := &server.ServerAPIVarzResponse{
			Server: &server.ServerInfo{Name: "prod-srv-1", ID: "id-1"},
			Data:   &server.Varz{},
		}
		r2 := &server.ServerAPIVarzResponse{
			Server: &server.ServerInfo{Name: "dev-srv-2", ID: "id-2"},
			Data:   &server.Varz{},
		}
		w.Add(r1, archive.TagCluster("c1"), archive.TagServer("prod-srv-1"), archive.TagServerVars())
		w.Add(r2, archive.TagCluster("c1"), archive.TagServer("dev-srv-2"), archive.TagServerVars())
	})

	opts := server.VarzEventOptions{}
	opts.EventFilterOptions.Name = "prod"
	results, err := a.Varz(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 filtered result, got %d", len(results))
	}
	if results[0].Server.Name != "prod-srv-1" {
		t.Errorf("expected prod-srv-1, got %s", results[0].Server.Name)
	}
}

func TestArchiveFilterByCluster(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		r1 := &server.ServerAPIVarzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1", Cluster: "east"},
			Data:   &server.Varz{},
		}
		r2 := &server.ServerAPIVarzResponse{
			Server: &server.ServerInfo{Name: "srv-2", ID: "id-2", Cluster: "west"},
			Data:   &server.Varz{},
		}
		w.Add(r1, archive.TagCluster("east"), archive.TagServer("srv-1"), archive.TagServerVars())
		w.Add(r2, archive.TagCluster("west"), archive.TagServer("srv-2"), archive.TagServerVars())
	})

	opts := server.VarzEventOptions{}
	opts.EventFilterOptions.Cluster = "east"
	results, err := a.Varz(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 filtered result, got %d", len(results))
	}
	if results[0].Server.Cluster != "east" {
		t.Errorf("expected cluster east, got %s", results[0].Server.Cluster)
	}
}

func TestArchiveEmptyResults(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		// Only add varz, nothing else
		resp := &server.ServerAPIVarzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data:   &server.Varz{},
		}
		w.Add(resp, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerVars())
	})

	// Endpoints with no data should return empty slices (no error)
	routez, err := a.Routez(server.RoutezEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(routez) != 0 {
		t.Errorf("expected 0 routez results, got %d", len(routez))
	}

	gatewayz, err := a.Gatewayz(server.GatewayzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(gatewayz) != 0 {
		t.Errorf("expected 0 gatewayz results, got %d", len(gatewayz))
	}

	healthz, err := a.Healthz(server.HealthzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(healthz) != 0 {
		t.Errorf("expected 0 healthz results, got %d", len(healthz))
	}
}

func TestArchiveClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.zip")
	w, err := archive.NewWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()

	a, err := NewAuditArchive(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewAuditArchiveRejectsEmptyPath(t *testing.T) {
	if _, err := NewAuditArchive(""); err == nil {
		t.Fatal("expected error for empty archivePath, got nil")
	}
}

func TestNewAuditArchiveInvalidPath(t *testing.T) {
	_, err := NewAuditArchive("/nonexistent/path/to/archive.zip")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestNewAuditArchiveInvalidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.zip")
	os.WriteFile(path, []byte("not a zip"), 0644)

	_, err := NewAuditArchive(path)
	if err == nil {
		t.Fatal("expected error for invalid zip")
	}
}

func TestArchiveCollectAccountsSingleServer(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		resp := &server.ServerAPIJszResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data: &server.JSInfo{
				AccountDetails: []*server.AccountDetail{
					{Name: "beta", Streams: []server.StreamDetail{{Name: "s2"}, {Name: "s1"}}},
					{Name: "alpha", Streams: []server.StreamDetail{{Name: "s3"}}},
				},
			},
		}
		w.Add(resp, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerJetStream())
	})

	accounts, err := a.CollectAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}
	// Sorted by name
	if accounts[0].Name != "alpha" {
		t.Errorf("expected first account alpha, got %s", accounts[0].Name)
	}
	if accounts[1].Name != "beta" {
		t.Errorf("expected second account beta, got %s", accounts[1].Name)
	}
	// Streams sorted within account
	if accounts[1].Streams[0].Name != "s1" || accounts[1].Streams[1].Name != "s2" {
		t.Errorf("expected streams sorted [s1 s2], got [%s %s]", accounts[1].Streams[0].Name, accounts[1].Streams[1].Name)
	}
}

func TestArchiveCollectAccountsMergesAcrossServers(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		r1 := &server.ServerAPIJszResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data: &server.JSInfo{
				AccountDetails: []*server.AccountDetail{
					{Name: "acct-A", Streams: []server.StreamDetail{{Name: "s1"}, {Name: "s2"}}},
				},
			},
		}
		r2 := &server.ServerAPIJszResponse{
			Server: &server.ServerInfo{Name: "srv-2", ID: "id-2"},
			Data: &server.JSInfo{
				AccountDetails: []*server.AccountDetail{
					{Name: "acct-A", Streams: []server.StreamDetail{{Name: "s2"}, {Name: "s3"}}},
				},
			},
		}
		w.Add(r1, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerJetStream())
		w.Add(r2, archive.TagCluster("c1"), archive.TagServer("srv-2"), archive.TagServerJetStream())
	})

	accounts, err := a.CollectAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 merged account, got %d", len(accounts))
	}
	// s1, s2, s3 -- s2 deduplicated, sorted
	if len(accounts[0].Streams) != 3 {
		t.Errorf("expected 3 deduplicated streams, got %d", len(accounts[0].Streams))
	}
	if accounts[0].Streams[0].Name != "s1" || accounts[0].Streams[1].Name != "s2" || accounts[0].Streams[2].Name != "s3" {
		names := make([]string, len(accounts[0].Streams))
		for i, s := range accounts[0].Streams {
			names[i] = s.Name
		}
		t.Errorf("expected sorted [s1 s2 s3], got %v", names)
	}
}

func TestMatchesFilter_NilInputs(t *testing.T) {
	info := &server.ServerInfo{Name: "srv"}
	if !matchesFilter(nil, &server.EventFilterOptions{}) {
		t.Error("nil info should match")
	}
	if !matchesFilter(info, nil) {
		t.Error("nil filter should match")
	}
	if !matchesFilter(nil, nil) {
		t.Error("both nil should match")
	}
}

func TestMatchesFilter_NameHostCluster(t *testing.T) {
	info := &server.ServerInfo{
		Name:    "ProdServer",
		ID:      "NXYZ123",
		Host:    "Host-A",
		Cluster: "East-1",
	}

	tests := []struct {
		name   string
		filter server.EventFilterOptions
		match  bool
	}{
		{"name substring", server.EventFilterOptions{Name: "Prod"}, true},
		{"name wrong case", server.EventFilterOptions{Name: "prod"}, false},
		{"name exact", server.EventFilterOptions{Name: "ProdServer", ExactMatch: true}, true},
		{"name exact mismatch", server.EventFilterOptions{Name: "Prod", ExactMatch: true}, false},
		{"name exact wrong case", server.EventFilterOptions{Name: "prodserver", ExactMatch: true}, false},
		{"id exact match", server.EventFilterOptions{Name: "NXYZ123", ExactMatch: true}, true},
		{"id substring match", server.EventFilterOptions{Name: "NXYZ"}, true},
		{"host substring", server.EventFilterOptions{Host: "Host"}, true},
		{"host wrong case", server.EventFilterOptions{Host: "host"}, false},
		{"cluster substring", server.EventFilterOptions{Cluster: "East"}, true},
		{"cluster wrong case", server.EventFilterOptions{Cluster: "east"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesFilter(info, &tc.filter)
			if got != tc.match {
				t.Errorf("expected %v, got %v", tc.match, got)
			}
		})
	}
}

func TestMatchesFilter_DomainAlwaysExact(t *testing.T) {
	info := &server.ServerInfo{Domain: "hub"}

	tests := []struct {
		name   string
		filter server.EventFilterOptions
		match  bool
	}{
		{"equal", server.EventFilterOptions{Domain: "hub"}, true},
		{"substring", server.EventFilterOptions{Domain: "hu"}, false},
		{"different", server.EventFilterOptions{Domain: "spoke"}, false},
		{"empty filter", server.EventFilterOptions{Domain: ""}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesFilter(info, &tc.filter)
			if got != tc.match {
				t.Errorf("expected %v, got %v", tc.match, got)
			}
		})
	}
}

func TestMatchesFilter_TagsLowercaseExact(t *testing.T) {
	info := &server.ServerInfo{Tags: []string{"region:us-east", "env:prod"}}

	tests := []struct {
		name  string
		tags  []string
		exact bool
		match bool
	}{
		{"exact tag", []string{"region:us-east"}, false, true},
		{"uppercase filter lowercased", []string{"Region:us-east"}, false, true},
		{"tag substring should not match", []string{"region"}, false, false},
		{"all tags present", []string{"region:us-east", "env:prod"}, false, true},
		{"one tag missing", []string{"region:us-east", "env:staging"}, false, false},
		{"ExactMatch flag ignored for tags", []string{"region:us-east"}, true, true},
		{"ExactMatch flag ignored substring still fails", []string{"region"}, true, false},
		{"whitespace trimmed", []string{"  region:us-east  "}, false, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &server.EventFilterOptions{Tags: tc.tags, ExactMatch: tc.exact}
			got := matchesFilter(info, f)
			if got != tc.match {
				t.Errorf("expected %v, got %v", tc.match, got)
			}
		})
	}
}

func TestArchiveUnsupportedLeafzAccount(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {})

	_, err := a.Leafz(server.LeafzEventOptions{
		LeafzOptions: server.LeafzOptions{Account: "foo"},
	})
	if !errors.Is(err, ErrUnsupportedOption) {
		t.Errorf("expected ErrUnsupportedOption, got %v", err)
	}
}

func TestArchiveUnsupportedHealthzOptions(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {})

	tests := []struct {
		name string
		opts server.HealthzEventOptions
	}{
		{"js-enabled", server.HealthzEventOptions{HealthzOptions: server.HealthzOptions{JSEnabledOnly: true}}},
		{"js-server-only", server.HealthzEventOptions{HealthzOptions: server.HealthzOptions{JSServerOnly: true}}},
		{"account", server.HealthzEventOptions{HealthzOptions: server.HealthzOptions{Account: "a"}}},
		{"stream", server.HealthzEventOptions{HealthzOptions: server.HealthzOptions{Stream: "s"}}},
		{"consumer", server.HealthzEventOptions{HealthzOptions: server.HealthzOptions{Consumer: "c"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := a.Healthz(tc.opts)
			if !errors.Is(err, ErrUnsupportedOption) {
				t.Errorf("expected ErrUnsupportedOption, got %v", err)
			}
		})
	}
}

func TestArchiveUnsupportedGatewayzName(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {})

	_, err := a.Gatewayz(server.GatewayzEventOptions{
		GatewayzOptions: server.GatewayzOptions{Name: "gw"},
	})
	if !errors.Is(err, ErrUnsupportedOption) {
		t.Errorf("expected ErrUnsupportedOption, got %v", err)
	}
}

func TestArchiveUnsupportedJszAccount(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {})

	_, err := a.Jsz(server.JszEventOptions{
		JSzOptions: server.JSzOptions{Account: "foo"},
	})
	if !errors.Is(err, ErrUnsupportedOption) {
		t.Errorf("expected ErrUnsupportedOption, got %v", err)
	}
}

func TestArchiveUnsupportedConnzOptions(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {})

	tests := []struct {
		name string
		opts server.ConnzEventOptions
	}{
		{"account", server.ConnzEventOptions{ConnzOptions: server.ConnzOptions{Account: "a"}}},
		{"filter_subject", server.ConnzEventOptions{ConnzOptions: server.ConnzOptions{FilterSubject: ">"}}},
		{"user", server.ConnzEventOptions{ConnzOptions: server.ConnzOptions{User: "u"}}},
		{"state", server.ConnzEventOptions{ConnzOptions: server.ConnzOptions{State: server.ConnClosed}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := a.Connz(tc.opts)
			if !errors.Is(err, ErrUnsupportedOption) {
				t.Errorf("expected ErrUnsupportedOption, got %v", err)
			}
		})
	}
}

func TestArchiveUnsupportedMultipleFields(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {})

	_, err := a.Healthz(server.HealthzEventOptions{
		HealthzOptions: server.HealthzOptions{Account: "a", Stream: "s"},
	})
	if !errors.Is(err, ErrUnsupportedOption) {
		t.Errorf("expected ErrUnsupportedOption, got %v", err)
	}
}

func TestArchiveIpqueuesz(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		data := server.IpqueueszStatus{"clients": {Pending: 42}}
		resp := &server.ServerAPIpqueueszResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data:   &data,
		}
		w.Add(resp,
			archive.TagAccount("acct-A"),
			archive.TagServer("srv-1"),
			archive.TagCluster("c1"),
			archive.TagAccountIpqueuesz())
	})

	results, err := a.Ipqueuesz(server.IpqueueszEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Server.Name != "srv-1" {
		t.Errorf("expected server name srv-1, got %s", results[0].Server.Name)
	}
	if (*results[0].Data)["clients"].Pending != 42 {
		t.Errorf("unexpected ipqueuesz data: %+v", results[0].Data)
	}
}

func TestArchiveIpqueueszFilters(t *testing.T) {
	build := func(t *testing.T) *AuditArchive {
		return newTestArchive(t, func(w *archive.Writer) {
			data := server.IpqueueszStatus{
				"clients":  {Pending: 42},
				"requests": {Pending: 0, InProgress: 0},
				"replies":  {Pending: 0, InProgress: 5},
			}
			resp := &server.ServerAPIpqueueszResponse{
				Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
				Data:   &data,
			}
			w.Add(resp,
				archive.TagAccount("acct-A"),
				archive.TagServer("srv-1"),
				archive.TagCluster("c1"),
				archive.TagAccountIpqueuesz())
		})
	}

	t.Run("All=false drops empty queues", func(t *testing.T) {
		a := build(t)
		results, err := a.Ipqueuesz(server.IpqueueszEventOptions{})
		if err != nil {
			t.Fatal(err)
		}
		got := *results[0].Data
		if _, ok := got["requests"]; ok {
			t.Errorf("expected empty queue 'requests' to be filtered out")
		}
		if _, ok := got["clients"]; !ok {
			t.Errorf("expected 'clients' to be present")
		}
		if _, ok := got["replies"]; !ok {
			t.Errorf("expected 'replies' (in-progress > 0) to be present")
		}
	})

	t.Run("All=true keeps empty queues", func(t *testing.T) {
		a := build(t)
		results, err := a.Ipqueuesz(server.IpqueueszEventOptions{
			IpqueueszOptions: server.IpqueueszOptions{All: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(*results[0].Data) != 3 {
			t.Errorf("expected 3 queues with All=true, got %d", len(*results[0].Data))
		}
	})

	t.Run("Filter narrows by substring", func(t *testing.T) {
		a := build(t)
		results, err := a.Ipqueuesz(server.IpqueueszEventOptions{
			IpqueueszOptions: server.IpqueueszOptions{All: true, Filter: "re"},
		})
		if err != nil {
			t.Fatal(err)
		}
		got := *results[0].Data
		if _, ok := got["requests"]; !ok {
			t.Errorf("expected 'requests' (matches 're') to be present")
		}
		if _, ok := got["replies"]; !ok {
			t.Errorf("expected 'replies' (matches 're') to be present")
		}
		if _, ok := got["clients"]; ok {
			t.Errorf("expected 'clients' to be filtered out")
		}
	})
}

func TestArchiveIpqueueszEmpty(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {})

	results, err := a.Ipqueuesz(server.IpqueueszEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestArchiveRaftz(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		data := server.RaftzStatus{
			"meta": {"g1": server.RaftzGroup{ID: "g1", State: "Leader"}},
		}
		resp := &server.ServerAPIRaftzResponse{
			Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
			Data:   &data,
		}
		w.Add(resp,
			archive.TagAccount("acct-A"),
			archive.TagServer("srv-1"),
			archive.TagCluster("c1"),
			archive.TagAccountRaftz())
	})

	results, err := a.Raftz(server.RaftzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Server.Name != "srv-1" {
		t.Errorf("expected server name srv-1, got %s", results[0].Server.Name)
	}
	if (*results[0].Data)["meta"]["g1"].ID != "g1" {
		t.Errorf("unexpected raftz data: %+v", results[0].Data)
	}
}

func TestArchiveRaftzFilters(t *testing.T) {
	build := func(t *testing.T) *AuditArchive {
		return newTestArchive(t, func(w *archive.Writer) {
			data := server.RaftzStatus{
				"$SYS": {
					"meta": server.RaftzGroup{ID: "meta", State: "Leader"},
				},
				"acct-A": {
					"g1": server.RaftzGroup{ID: "g1", State: "Follower"},
					"g2": server.RaftzGroup{ID: "g2", State: "Leader"},
				},
			}
			resp := &server.ServerAPIRaftzResponse{
				Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
				Data:   &data,
			}
			w.Add(resp,
				archive.TagAccount("acct-A"),
				archive.TagServer("srv-1"),
				archive.TagCluster("c1"),
				archive.TagAccountRaftz())
		})
	}

	t.Run("AccountFilter narrows to one account", func(t *testing.T) {
		a := build(t)
		results, err := a.Raftz(server.RaftzEventOptions{
			RaftzOptions: server.RaftzOptions{AccountFilter: "acct-A"},
		})
		if err != nil {
			t.Fatal(err)
		}
		got := *results[0].Data
		if _, ok := got["$SYS"]; ok {
			t.Errorf("expected $SYS account to be filtered out")
		}
		if len(got["acct-A"]) != 2 {
			t.Errorf("expected both groups under acct-A, got %d", len(got["acct-A"]))
		}
	})

	t.Run("GroupFilter narrows to one group", func(t *testing.T) {
		a := build(t)
		results, err := a.Raftz(server.RaftzEventOptions{
			RaftzOptions: server.RaftzOptions{GroupFilter: "g1"},
		})
		if err != nil {
			t.Fatal(err)
		}
		got := *results[0].Data
		if _, ok := got["$SYS"]; ok {
			t.Errorf("expected $SYS to be dropped (no g1)")
		}
		if _, ok := got["acct-A"]["g1"]; !ok {
			t.Errorf("expected g1 under acct-A")
		}
		if _, ok := got["acct-A"]["g2"]; ok {
			t.Errorf("expected g2 to be filtered out")
		}
	})

	t.Run("AccountFilter + GroupFilter combined", func(t *testing.T) {
		a := build(t)
		results, err := a.Raftz(server.RaftzEventOptions{
			RaftzOptions: server.RaftzOptions{AccountFilter: "acct-A", GroupFilter: "g2"},
		})
		if err != nil {
			t.Fatal(err)
		}
		got := *results[0].Data
		if len(got) != 1 {
			t.Fatalf("expected 1 account in result, got %d", len(got))
		}
		if _, ok := got["acct-A"]["g2"]; !ok {
			t.Errorf("expected g2 under acct-A")
		}
		if _, ok := got["acct-A"]["g1"]; ok {
			t.Errorf("expected g1 to be filtered out")
		}
	})
}

func TestArchiveRaftzEmpty(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {})

	results, err := a.Raftz(server.RaftzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func addProfile(t *testing.T, w *archive.Writer, cluster, srv, name string, debug int, body []byte) {
	t.Helper()
	tagName := name
	if debug > 0 {
		tagName = fmt.Sprintf("%s_%d", name, debug)
	}
	tags := []*archive.Tag{
		archive.TagServer(srv),
		archive.TagServerProfile(),
		archive.TagProfileName(tagName),
	}
	if cluster == "" {
		tags = append(tags, archive.TagNoCluster())
	} else {
		tags = append(tags, archive.TagCluster(cluster))
	}
	if err := w.AddRaw(bytes.NewReader(body), "prof", tags...); err != nil {
		t.Fatal(err)
	}
}

// TestArchiveProfilezRoundTrip pins forEachProfileArtifact's hardcoded
// tag-value strings against archive.Writer's output.
func TestArchiveProfilezRoundTrip(t *testing.T) {
	wantBytes := []byte("pprof-heap-payload")
	a := newTestArchive(t, func(w *archive.Writer) {
		addProfile(t, w, "c1", "srv-1", "heap", 0, wantBytes)
	})

	results, err := a.Profilez(server.ProfilezEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected non-empty results (canary: hardcoded tag-value strings must match audit/archive constants)")
	}
	if !bytes.Equal(results[0].Data.Profile, wantBytes) {
		t.Fatalf("profile bytes mismatch: got %q want %q", results[0].Data.Profile, wantBytes)
	}
	if results[0].Server.Name != "srv-1" {
		t.Errorf("expected Server.Name=srv-1, got %q", results[0].Server.Name)
	}
	if results[0].Server.Cluster != "c1" {
		t.Errorf("expected Server.Cluster=c1, got %q", results[0].Server.Cluster)
	}
}

func TestArchiveProfilezAll(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		addProfile(t, w, "c1", "srv-1", "heap", 0, []byte("heap-1"))
		addProfile(t, w, "c1", "srv-1", "goroutine", 2, []byte("goroutine_2-1"))
		addProfile(t, w, "c2", "srv-2", "heap", 0, []byte("heap-2"))
	})

	results, err := a.Profilez(server.ProfilezEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Default opts have Name="" Debug=0; goroutine_2 is filtered out.
	if len(results) != 2 {
		t.Fatalf("expected 2 results (default opts filter Debug=0), got %d", len(results))
	}
}

func TestArchiveProfilezFilterByName(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		addProfile(t, w, "c1", "srv-1", "heap", 0, []byte("h"))
		addProfile(t, w, "c1", "srv-1", "cpu", 0, []byte("c"))
		addProfile(t, w, "c1", "srv-1", "mutex", 0, []byte("m"))
	})

	results, err := a.Profilez(server.ProfilezEventOptions{
		ProfilezOptions: server.ProfilezOptions{Name: "cpu"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if string(results[0].Data.Profile) != "c" {
		t.Errorf("expected cpu bytes, got %q", results[0].Data.Profile)
	}
}

func TestArchiveProfilezFilterByDebug(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		addProfile(t, w, "c1", "srv-1", "goroutine", 0, []byte("g0"))
		addProfile(t, w, "c1", "srv-1", "goroutine", 1, []byte("g1"))
		addProfile(t, w, "c1", "srv-1", "goroutine", 2, []byte("g2"))
	})

	results, err := a.Profilez(server.ProfilezEventOptions{
		ProfilezOptions: server.ProfilezOptions{Name: "goroutine", Debug: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if string(results[0].Data.Profile) != "g2" {
		t.Errorf("expected g2, got %q", results[0].Data.Profile)
	}
}

func TestArchiveProfilezFilterByCluster(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		addProfile(t, w, "c1", "srv-1", "heap", 0, []byte("h1"))
		addProfile(t, w, "c2", "srv-2", "heap", 0, []byte("h2"))
	})

	results, err := a.Profilez(server.ProfilezEventOptions{
		EventFilterOptions: server.EventFilterOptions{Cluster: "c2", ExactMatch: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Server.Cluster != "c2" {
		t.Errorf("expected cluster c2, got %q", results[0].Server.Cluster)
	}
}

func TestArchiveProfilezFilterByName_EventFilter(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		addProfile(t, w, "c1", "srv-1", "heap", 0, []byte("h1"))
		addProfile(t, w, "c1", "srv-2", "heap", 0, []byte("h2"))
	})

	results, err := a.Profilez(server.ProfilezEventOptions{
		EventFilterOptions: server.EventFilterOptions{Name: "srv-2", ExactMatch: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Server.Name != "srv-2" {
		t.Errorf("expected srv-2, got %q", results[0].Server.Name)
	}
}

func TestArchiveProfilezServerInfoFromVarz(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		vz := &server.ServerAPIVarzResponse{
			Server: &server.ServerInfo{
				Name: "srv-1", ID: "id-1", Cluster: "c1", Domain: "d1",
			},
			Data: &server.Varz{},
		}
		if err := w.Add(vz, archive.TagCluster("c1"), archive.TagServer("srv-1"), archive.TagServerVars()); err != nil {
			t.Fatal(err)
		}
		addProfile(t, w, "c1", "srv-1", "heap", 0, []byte("h"))
	})

	results, err := a.Profilez(server.ProfilezEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Server.ID != "id-1" {
		t.Errorf("expected ID=id-1 (from Varz), got %q", results[0].Server.ID)
	}
	if results[0].Server.Domain != "d1" {
		t.Errorf("expected Domain=d1 (from Varz), got %q", results[0].Server.Domain)
	}
}

func TestArchiveProfilezServerInfoWithoutVarz(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		addProfile(t, w, "c1", "srv-1", "heap", 0, []byte("h"))
	})

	results, err := a.Profilez(server.ProfilezEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Server.ID != "" {
		t.Errorf("expected empty ID without Varz, got %q", results[0].Server.ID)
	}
	if results[0].Server.Name != "srv-1" {
		t.Errorf("expected Name=srv-1 from tag, got %q", results[0].Server.Name)
	}
}

func TestArchiveProfilezUnclusteredMapsToEmptyCluster(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		addProfile(t, w, "", "srv-x", "heap", 0, []byte("h"))
	})

	results, err := a.Profilez(server.ProfilezEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Server.Cluster != "" {
		t.Errorf("expected empty Cluster for unclustered server, got %q", results[0].Server.Cluster)
	}
}

func TestArchiveProfilezUnsupportedDuration(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		addProfile(t, w, "c1", "srv-1", "cpu", 0, []byte("c"))
	})

	_, err := a.Profilez(server.ProfilezEventOptions{
		ProfilezOptions: server.ProfilezOptions{Name: "cpu", Duration: time.Second},
	})
	if !errors.Is(err, ErrUnsupportedOption) {
		t.Fatalf("expected ErrUnsupportedOption, got %v", err)
	}
}

func TestArchiveProfilezUnsupportedHost(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {})
	_, err := a.Profilez(server.ProfilezEventOptions{
		EventFilterOptions: server.EventFilterOptions{Host: "10.0.0.1"},
	})
	if !errors.Is(err, ErrUnsupportedOption) {
		t.Fatalf("expected ErrUnsupportedOption, got %v", err)
	}
}

func TestArchiveProfilezEmpty(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {})

	results, err := a.Profilez(server.ProfilezEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

// TestArchiveProfilezManifestPresent locks the manifest path our code hardcodes.
func TestArchiveProfilezManifestPresent(t *testing.T) {
	a := newTestArchive(t, func(w *archive.Writer) {
		addProfile(t, w, "c1", "srv-1", "heap", 0, []byte("h"))
	})

	zr, err := zip.OpenReader(a.archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	found := false
	for _, f := range zr.File {
		if f.Name == "capture/misc/manifest.json" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("manifest not at expected path capture/misc/manifest.json")
	}
}

func TestSplitProfileName(t *testing.T) {
	cases := []struct {
		in    string
		name  string
		debug int
	}{
		{"heap", "heap", 0},
		{"cpu", "cpu", 0},
		{"goroutine", "goroutine", 0},
		{"goroutine_1", "goroutine", 1},
		{"goroutine_2", "goroutine", 2},
		{"mutex", "mutex", 0},
		{"threadcreate", "threadcreate", 0},
		{"weird_suffix", "weird_suffix", 0},
		{"trailing_", "trailing_", 0},
		{"", "", 0},
	}
	for _, c := range cases {
		name, debug := splitProfileName(c.in)
		if name != c.name || debug != c.debug {
			t.Errorf("splitProfileName(%q) = (%q,%d), want (%q,%d)", c.in, name, debug, c.name, c.debug)
		}
	}
}
