package serverdata

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// newLiveT constructs a Live and fails the test if construction errors.
func newLiveT(t *testing.T, nc *nats.Conn, reqFn RequestFunc, waitFor int) *Live {
	t.Helper()
	src, err := NewLive(nc, reqFn, waitFor)
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// mockReqFn returns a RequestFunc that records calls and returns preset responses.
func mockReqFn(responses [][]byte, err error) (RequestFunc, *[]mockCall) {
	var calls []mockCall
	fn := func(req any, subj string, waitFor int, nc *nats.Conn) ([][]byte, error) {
		calls = append(calls, mockCall{req: req, subj: subj, waitFor: waitFor})
		return responses, err
	}
	return fn, &calls
}

type mockCall struct {
	req     any
	subj    string
	waitFor int
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestServerVarz(t *testing.T) {
	resp := &server.ServerAPIVarzResponse{
		Server: &server.ServerInfo{Name: "srv-1"},
		Data:   &server.Varz{MaxConn: 100},
	}
	reqFn, calls := mockReqFn([][]byte{mustMarshal(t, resp)}, nil)
	src := newLiveT(t, nil, reqFn, 3)

	opts := server.VarzEventOptions{}
	results, err := src.Varz(opts)
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
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	if (*calls)[0].subj != "$SYS.REQ.SERVER.PING.VARZ" {
		t.Errorf("expected VARZ subject, got %s", (*calls)[0].subj)
	}
	if (*calls)[0].waitFor != 3 {
		t.Errorf("expected waitFor 3, got %d", (*calls)[0].waitFor)
	}
}

func TestServerConnz(t *testing.T) {
	resp := &server.ServerAPIConnzResponse{
		Server: &server.ServerInfo{Name: "srv-2"},
		Data:   &server.Connz{NumConns: 42},
	}
	reqFn, _ := mockReqFn([][]byte{mustMarshal(t, resp)}, nil)
	src := newLiveT(t, nil, reqFn, 0)

	results, err := src.Connz(server.ConnzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Data.NumConns != 42 {
		t.Errorf("expected 42 connections, got %d", results[0].Data.NumConns)
	}
}

func TestServerStatz(t *testing.T) {
	resp := &server.ServerStatsMsg{
		Server: server.ServerInfo{Name: "srv-3", Cluster: "c1"},
	}
	reqFn, calls := mockReqFn([][]byte{mustMarshal(t, resp)}, nil)
	src := newLiveT(t, nil, reqFn, 5)

	results, err := src.Statz(server.StatszEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Server.Name != "srv-3" {
		t.Errorf("expected server name srv-3, got %s", results[0].Server.Name)
	}
	if (*calls)[0].subj != "$SYS.REQ.SERVER.PING" {
		t.Errorf("expected PING subject, got %s", (*calls)[0].subj)
	}
}

func TestServerMultipleResponses(t *testing.T) {
	r1 := mustMarshal(t, &server.ServerAPIVarzResponse{
		Server: &server.ServerInfo{Name: "srv-a"},
		Data:   &server.Varz{},
	})
	r2 := mustMarshal(t, &server.ServerAPIVarzResponse{
		Server: &server.ServerInfo{Name: "srv-b"},
		Data:   &server.Varz{},
	})
	reqFn, _ := mockReqFn([][]byte{r1, r2}, nil)
	src := newLiveT(t, nil, reqFn, 2)

	results, err := src.Varz(server.VarzEventOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Server.Name != "srv-a" {
		t.Errorf("expected srv-a, got %s", results[0].Server.Name)
	}
	if results[1].Server.Name != "srv-b" {
		t.Errorf("expected srv-b, got %s", results[1].Server.Name)
	}
}

func TestServerRequestError(t *testing.T) {
	reqFn, _ := mockReqFn(nil, fmt.Errorf("connection lost"))
	src := newLiveT(t, nil, reqFn, 1)

	_, err := src.Varz(server.VarzEventOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServerProfilez(t *testing.T) {
	resp := &ProfilezResponse{
		Server: &server.ServerInfo{Name: "srv-1", ID: "id-1"},
		Data:   &server.ProfilezStatus{Profile: []byte("pprof-bytes")},
	}
	reqFn, calls := mockReqFn([][]byte{mustMarshal(t, resp)}, nil)
	src := newLiveT(t, nil, reqFn, 1)

	opts := server.ProfilezEventOptions{
		ProfilezOptions: server.ProfilezOptions{Name: "heap"},
	}
	results, err := src.Profilez(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Server.Name != "srv-1" {
		t.Errorf("expected server name srv-1, got %s", results[0].Server.Name)
	}
	if string(results[0].Data.Profile) != "pprof-bytes" {
		t.Errorf("expected pprof-bytes, got %q", results[0].Data.Profile)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	if (*calls)[0].subj != "$SYS.REQ.SERVER.PING.PROFILEZ" {
		t.Errorf("unexpected subject: %s", (*calls)[0].subj)
	}
}

func TestServerProfilezError(t *testing.T) {
	resp := &ProfilezResponse{
		Server: &server.ServerInfo{Name: "srv-1"},
		Data:   &server.ProfilezStatus{Error: "Profile \"bogus\" not found"},
	}
	reqFn, _ := mockReqFn([][]byte{mustMarshal(t, resp)}, nil)
	src := newLiveT(t, nil, reqFn, 1)

	results, err := src.Profilez(server.ProfilezEventOptions{
		ProfilezOptions: server.ProfilezOptions{Name: "bogus"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Data.Error == "" {
		t.Error("expected non-empty Data.Error to round-trip")
	}
}

func TestServerUnmarshalError(t *testing.T) {
	reqFn, _ := mockReqFn([][]byte{[]byte("not json")}, nil)
	src := newLiveT(t, nil, reqFn, 1)

	_, err := src.Varz(server.VarzEventOptions{})
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestServerClose(t *testing.T) {
	reqFn, _ := mockReqFn(nil, nil)
	src := newLiveT(t, nil, reqFn, 0)
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewLiveRejectsNilReqFn(t *testing.T) {
	if _, err := NewLive(nil, nil, 0); err == nil {
		t.Fatal("expected error for nil reqFn, got nil")
	}
}

// mockReqFnSequenceWithCalls returns a RequestFunc that returns scripted
// responses in order and records each call for inspection.
func mockReqFnSequenceWithCalls(sequence []mockResponse) (RequestFunc, *[]mockCall) {
	var calls []mockCall
	idx := 0
	fn := func(req any, subj string, waitFor int, nc *nats.Conn) ([][]byte, error) {
		calls = append(calls, mockCall{req: req, subj: subj, waitFor: waitFor})
		if idx >= len(sequence) {
			return nil, fmt.Errorf("unexpected call %d", idx)
		}
		resp := sequence[idx]
		idx++
		return resp.data, resp.err
	}
	return fn, &calls
}

// mockReqFnByCallWithCalls dispatches each call to a user-supplied function and
// records every call. The dispatcher receives the zero-based call index.
func mockReqFnByCallWithCalls(dispatch func(call int, req any, subj string, waitFor int) ([][]byte, error)) (RequestFunc, *[]mockCall) {
	var calls []mockCall
	fn := func(req any, subj string, waitFor int, nc *nats.Conn) ([][]byte, error) {
		idx := len(calls)
		calls = append(calls, mockCall{req: req, subj: subj, waitFor: waitFor})
		return dispatch(idx, req, subj, waitFor)
	}
	return fn, &calls
}

type mockResponse struct {
	data [][]byte
	err  error
}

func TestServerCollectAccountsSingleServer(t *testing.T) {
	resp := &server.ServerAPIJszResponse{
		Server: &server.ServerInfo{Name: "srv-1"},
		Data: &server.JSInfo{
			AccountDetails: []*server.AccountDetail{
				{Name: "beta", Streams: []server.StreamDetail{{Name: "s1"}}},
				{Name: "alpha", Streams: []server.StreamDetail{{Name: "s2"}}},
			},
		},
	}
	reqFn, _ := mockReqFn([][]byte{mustMarshal(t, resp)}, nil)
	src := newLiveT(t, nil, reqFn, 1)

	accounts, err := src.CollectAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}
	// Should be sorted by name
	if accounts[0].Name != "alpha" {
		t.Errorf("expected first account alpha, got %s", accounts[0].Name)
	}
	if accounts[1].Name != "beta" {
		t.Errorf("expected second account beta, got %s", accounts[1].Name)
	}
}

func TestServerCollectAccountsMergesAcrossServers(t *testing.T) {
	r1 := &server.ServerAPIJszResponse{
		Server: &server.ServerInfo{Name: "srv-1"},
		Data: &server.JSInfo{
			AccountDetails: []*server.AccountDetail{
				{Name: "acct-A", Streams: []server.StreamDetail{{Name: "s1"}, {Name: "s2"}}},
			},
		},
	}
	r2 := &server.ServerAPIJszResponse{
		Server: &server.ServerInfo{Name: "srv-2"},
		Data: &server.JSInfo{
			AccountDetails: []*server.AccountDetail{
				{Name: "acct-A", Streams: []server.StreamDetail{{Name: "s2"}, {Name: "s3"}}},
			},
		},
	}
	reqFn, _ := mockReqFn([][]byte{mustMarshal(t, r1), mustMarshal(t, r2)}, nil)
	src := newLiveT(t, nil, reqFn, 2)

	accounts, err := src.CollectAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 merged account, got %d", len(accounts))
	}
	// s1, s2, s3 -- s2 deduplicated
	if len(accounts[0].Streams) != 3 {
		t.Errorf("expected 3 deduplicated streams, got %d", len(accounts[0].Streams))
	}
	// Streams should be sorted
	if accounts[0].Streams[0].Name != "s1" || accounts[0].Streams[1].Name != "s2" || accounts[0].Streams[2].Name != "s3" {
		names := make([]string, len(accounts[0].Streams))
		for i, s := range accounts[0].Streams {
			names[i] = s.Name
		}
		t.Errorf("expected sorted [s1 s2 s3], got %v", names)
	}
}

func TestServerCollectAccountsPaging(t *testing.T) {
	// Build a response with exactly 1024 accounts to trigger paging
	details := make([]*server.AccountDetail, 1024)
	for i := range details {
		details[i] = &server.AccountDetail{Name: fmt.Sprintf("acct-%04d", i)}
	}
	initialResp := &server.ServerAPIJszResponse{
		Server: &server.ServerInfo{Name: "srv-1", ID: "NABC"},
		Data:   &server.JSInfo{AccountDetails: details},
	}

	// Second page has fewer than 1024, ending paging
	page2 := &server.ServerAPIJszResponse{
		Server: &server.ServerInfo{Name: "srv-1", ID: "NABC"},
		Data: &server.JSInfo{
			AccountDetails: []*server.AccountDetail{
				{Name: "acct-extra"},
			},
		},
	}

	seq, calls := mockReqFnSequenceWithCalls([]mockResponse{
		{data: [][]byte{mustMarshal(t, initialResp)}},
		{data: [][]byte{mustMarshal(t, page2)}},
	})
	src := newLiveT(t, nil, seq, 1)

	accounts, err := src.CollectAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1025 {
		t.Fatalf("expected 1025 accounts, got %d", len(accounts))
	}
	if len(*calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(*calls))
	}
	if (*calls)[1].subj != "$SYS.REQ.SERVER.NABC.JSZ" {
		t.Errorf("expected paging via direct ID subject, got %s", (*calls)[1].subj)
	}
	if (*calls)[1].waitFor != 1 {
		t.Errorf("expected waitFor=1 for paging request, got %d", (*calls)[1].waitFor)
	}
}

// TestServerCollectAccountsNameCollision verifies that two servers sharing a
// name but with distinct IDs are paged independently and their accounts merged.
func TestServerCollectAccountsNameCollision(t *testing.T) {
	makePage := func(n int, prefix string) []*server.AccountDetail {
		out := make([]*server.AccountDetail, n)
		for i := range out {
			out[i] = &server.AccountDetail{Name: fmt.Sprintf("%s-%04d", prefix, i)}
		}
		return out
	}

	srvA1 := &server.ServerAPIJszResponse{
		Server: &server.ServerInfo{Name: "node-1", ID: "NA"},
		Data:   &server.JSInfo{AccountDetails: makePage(1024, "a")},
	}
	srvB1 := &server.ServerAPIJszResponse{
		Server: &server.ServerInfo{Name: "node-1", ID: "NB"},
		Data:   &server.JSInfo{AccountDetails: makePage(1024, "b")},
	}
	srvA2 := &server.ServerAPIJszResponse{
		Server: &server.ServerInfo{Name: "node-1", ID: "NA"},
		Data:   &server.JSInfo{AccountDetails: makePage(3, "a-extra")},
	}
	srvB2 := &server.ServerAPIJszResponse{
		Server: &server.ServerInfo{Name: "node-1", ID: "NB"},
		Data:   &server.JSInfo{AccountDetails: makePage(5, "b-extra")},
	}

	// Initial broadcast returns both servers' first pages; per-ID follow-ups
	// return each server's tail. Order of follow-ups is map-iteration-dependent,
	// so the mock dispatches based on subject.
	seq, calls := mockReqFnByCallWithCalls(func(call int, _ any, subj string, _ int) ([][]byte, error) {
		switch {
		case call == 0:
			return [][]byte{mustMarshal(t, srvA1), mustMarshal(t, srvB1)}, nil
		case subj == "$SYS.REQ.SERVER.NA.JSZ":
			return [][]byte{mustMarshal(t, srvA2)}, nil
		case subj == "$SYS.REQ.SERVER.NB.JSZ":
			return [][]byte{mustMarshal(t, srvB2)}, nil
		}
		return nil, fmt.Errorf("unexpected call %d to %s", call, subj)
	})
	src := newLiveT(t, nil, seq, 2)

	accounts, err := src.CollectAccounts()
	if err != nil {
		t.Fatal(err)
	}
	expected := 1024 + 1024 + 3 + 5
	if len(accounts) != expected {
		t.Fatalf("expected %d accounts after merging two name-colliding servers, got %d", expected, len(accounts))
	}
	if len(*calls) != 3 {
		t.Fatalf("expected 3 calls (initial + 2 per-ID follow-ups), got %d", len(*calls))
	}

	// Verify both ID-targeted subjects were actually called. Without keying by
	// ID, the buggy code would have issued only one follow-up under name "node-1"
	// and dropped the other server's tail entirely.
	subjects := map[string]bool{}
	for _, c := range *calls {
		subjects[c.subj] = true
	}
	for _, want := range []string{"$SYS.REQ.SERVER.NA.JSZ", "$SYS.REQ.SERVER.NB.JSZ"} {
		if !subjects[want] {
			t.Errorf("expected follow-up to %s, was not called; subjects seen: %v", want, subjects)
		}
	}

	// Verify a representative tail-page account from each server is present.
	// Count alone could pass with duplicates; checking both unique tails
	// ensures neither server's data was lost.
	names := map[string]bool{}
	for _, a := range accounts {
		names[a.Name] = true
	}
	for _, want := range []string{"a-extra-0000", "a-extra-0002", "b-extra-0000", "b-extra-0004"} {
		if !names[want] {
			t.Errorf("expected merged accounts to include %q, missing", want)
		}
	}
}

func TestServerSubjects(t *testing.T) {
	tests := []struct {
		name   string
		call   func(ds Source) error
		expect string
	}{
		{"Varz", func(ds Source) error { _, err := ds.Varz(server.VarzEventOptions{}); return err }, "$SYS.REQ.SERVER.PING.VARZ"},
		{"Connz", func(ds Source) error { _, err := ds.Connz(server.ConnzEventOptions{}); return err }, "$SYS.REQ.SERVER.PING.CONNZ"},
		{"Routez", func(ds Source) error { _, err := ds.Routez(server.RoutezEventOptions{}); return err }, "$SYS.REQ.SERVER.PING.ROUTEZ"},
		{"Gatewayz", func(ds Source) error { _, err := ds.Gatewayz(server.GatewayzEventOptions{}); return err }, "$SYS.REQ.SERVER.PING.GATEWAYZ"},
		{"Leafz", func(ds Source) error { _, err := ds.Leafz(server.LeafzEventOptions{}); return err }, "$SYS.REQ.SERVER.PING.LEAFZ"},
		{"Subsz", func(ds Source) error { _, err := ds.Subsz(server.SubszEventOptions{}); return err }, "$SYS.REQ.SERVER.PING.SUBSZ"},
		{"Jsz", func(ds Source) error { _, err := ds.Jsz(server.JszEventOptions{}); return err }, "$SYS.REQ.SERVER.PING.JSZ"},
		{"Healthz", func(ds Source) error { _, err := ds.Healthz(server.HealthzEventOptions{}); return err }, "$SYS.REQ.SERVER.PING.HEALTHZ"},
		{"Accountz", func(ds Source) error { _, err := ds.Accountz(server.AccountzEventOptions{}); return err }, "$SYS.REQ.SERVER.PING.ACCOUNTZ"},
		{"Statz", func(ds Source) error { _, err := ds.Statz(server.StatszEventOptions{}); return err }, "$SYS.REQ.SERVER.PING"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := []byte(`{}`)
			reqFn, calls := mockReqFn([][]byte{resp}, nil)
			src := newLiveT(t, nil, reqFn, 1)

			_ = tc.call(src)

			if len(*calls) != 1 {
				t.Fatalf("expected 1 call, got %d", len(*calls))
			}
			if (*calls)[0].subj != tc.expect {
				t.Errorf("expected subject %s, got %s", tc.expect, (*calls)[0].subj)
			}
		})
	}
}
