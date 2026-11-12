package serverdata

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/nats-io/jsm.go/api/server/zmonitor"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// Live implements Source by querying live servers over NATS
type Live struct {
	nc      *nats.Conn
	reqFn   RequestFunc
	waitFor int
}

// NewLive creates a Live data source that delegates requests to the reqFn callback.
// reqFn is required; passing nil returns an error.
func NewLive(nc *nats.Conn, reqFn RequestFunc, waitFor int) (*Live, error) {
	if reqFn == nil {
		return nil, fmt.Errorf("reqFn is required")
	}
	return &Live{nc: nc, reqFn: reqFn, waitFor: waitFor}, nil
}

func (s *Live) Varz(opts server.VarzEventOptions) ([]*server.ServerAPIVarzResponse, error) {
	return unmarshalResponses[server.ServerAPIVarzResponse](s, opts, "$SYS.REQ.SERVER.PING.VARZ")
}

func (s *Live) Connz(opts server.ConnzEventOptions) ([]*server.ServerAPIConnzResponse, error) {
	return unmarshalResponses[server.ServerAPIConnzResponse](s, opts, "$SYS.REQ.SERVER.PING.CONNZ")
}

func (s *Live) Routez(opts server.RoutezEventOptions) ([]*server.ServerAPIRoutezResponse, error) {
	return unmarshalResponses[server.ServerAPIRoutezResponse](s, opts, "$SYS.REQ.SERVER.PING.ROUTEZ")
}

func (s *Live) Gatewayz(opts server.GatewayzEventOptions) ([]*server.ServerAPIGatewayzResponse, error) {
	return unmarshalResponses[server.ServerAPIGatewayzResponse](s, opts, "$SYS.REQ.SERVER.PING.GATEWAYZ")
}

func (s *Live) Leafz(opts server.LeafzEventOptions) ([]*server.ServerAPILeafzResponse, error) {
	return unmarshalResponses[server.ServerAPILeafzResponse](s, opts, "$SYS.REQ.SERVER.PING.LEAFZ")
}

func (s *Live) Subsz(opts server.SubszEventOptions) ([]*server.ServerAPISubszResponse, error) {
	return unmarshalResponses[server.ServerAPISubszResponse](s, opts, "$SYS.REQ.SERVER.PING.SUBSZ")
}

func (s *Live) Jsz(opts server.JszEventOptions) ([]*server.ServerAPIJszResponse, error) {
	return unmarshalResponses[server.ServerAPIJszResponse](s, opts, "$SYS.REQ.SERVER.PING.JSZ")
}

func (s *Live) Healthz(opts zmonitor.HealthzOptionsV1, filter server.EventFilterOptions) ([]*server.ServerAPIHealthzResponse, error) {
	req := struct {
		zmonitor.HealthzOptionsV1
		server.EventFilterOptions
	}{opts, filter}

	return unmarshalResponses[server.ServerAPIHealthzResponse](s, req, "$SYS.REQ.SERVER.PING.HEALTHZ")
}

func (s *Live) Accountz(opts server.AccountzEventOptions) ([]*server.ServerAPIAccountzResponse, error) {
	return unmarshalResponses[server.ServerAPIAccountzResponse](s, opts, "$SYS.REQ.SERVER.PING.ACCOUNTZ")
}

func (s *Live) Statz(opts server.StatszEventOptions) ([]*server.ServerStatsMsg, error) {
	return unmarshalResponses[server.ServerStatsMsg](s, opts, "$SYS.REQ.SERVER.PING")
}

func (s *Live) Ipqueuesz(opts server.IpqueueszEventOptions) ([]*server.ServerAPIpqueueszResponse, error) {
	return unmarshalResponses[server.ServerAPIpqueueszResponse](s, opts, "$SYS.REQ.SERVER.PING.IPQUEUESZ")
}

func (s *Live) Raftz(opts server.RaftzEventOptions) ([]*server.ServerAPIRaftzResponse, error) {
	return unmarshalResponses[server.ServerAPIRaftzResponse](s, opts, "$SYS.REQ.SERVER.PING.RAFTZ")
}

func (s *Live) Profilez(opts server.ProfilezEventOptions) ([]*ProfilezResponse, error) {
	return unmarshalResponses[ProfilezResponse](s, opts, "$SYS.REQ.SERVER.PING.PROFILEZ")
}

// EachSubsz pages through Subz responses from all servers and calls handle per page
func (s *Live) EachSubsz(opts server.SubszEventOptions, handle func(resp *server.ServerAPISubszResponse) error) error {
	if opts.SubszOptions.Offset != 0 {
		return unsupported("eachsubsz", "offset")
	}
	return pageAll(s, subszPager(opts), handle)
}

// EachConnz pages through Connz responses from all servers and calls handle per page
func (s *Live) EachConnz(opts server.ConnzEventOptions, handle func(resp *server.ServerAPIConnzResponse) error) error {
	if opts.ConnzOptions.Offset != 0 {
		return unsupported("eachconnz", "offset")
	}
	return pageAll(s, connzPager(opts), handle)
}

// EachJsz pages through Jsz responses from all servers and calls handle per page
func (s *Live) EachJsz(opts server.JszEventOptions, handle func(*server.ServerAPIJszResponse) error) error {
	if opts.JSzOptions.Offset != 0 {
		return unsupported("eachjsz", "offset")
	}
	return pageAll(s, jszPager(opts), handle)
}

// CollectAccounts gathers account-level JetStream metadata from all servers
func (s *Live) CollectAccounts() ([]*AccountDetail, error) {
	const pageLimit = 1024

	opts := server.JszEventOptions{
		JSzOptions: server.JSzOptions{
			Accounts: true,
			Streams:  true,
			Consumer: true,
			Config:   true,
			Limit:    pageLimit,
		},
	}

	accounts := accountCollector{}
	// JSZ Limit/Offset is per-server pagination, so follow-ups must address one
	// server at a time. Server IDs are globally unique; server names are not
	// (super-clusters, leaf nodes, and JetStream domains can repeat names).
	// Keying by ID and using $SYS.REQ.SERVER.<ID>.JSZ avoids losing accounts
	// from servers that share a name with another server in the deployment.
	err := s.EachJsz(opts, func(resp *server.ServerAPIJszResponse) error {
		if resp.Data == nil || resp.Server == nil {
			return nil
		}

		accounts.add(resp.Data.Config.Domain, resp.Data.AccountDetails)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return accounts.sorted(), nil
}

// Close is a noop for connecting to live servers. The caller owns the connections
func (s *Live) Close() error {
	return nil
}

func unmarshalResponses[T any](s *Live, opts any, subj string) ([]*T, error) {
	results, err := s.reqFn(opts, subj, s.waitFor, s.nc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", subj, err)
	}

	responses := make([]*T, 0, len(results))
	for _, raw := range results {
		resp := new(T)
		if err := json.Unmarshal(raw, resp); err != nil {
			return nil, fmt.Errorf("unmarshal %s response: %w", subj, err)
		}
		responses = append(responses, resp)
	}

	return responses, nil
}

// AccountDetail is the JetStream detail of an account within one JetStream domain. Domain is empty for servers
// without a JetStream domain.
type AccountDetail struct {
	Domain string
	*server.AccountDetail
}

type accountKey struct {
	domain  string
	account string
}

type accountCollector map[accountKey]*AccountDetail

func (c accountCollector) add(domain string, details []*server.AccountDetail) {
	for _, acct := range details {
		key := accountKey{domain, acct.Name}
		if existing, found := c[key]; found {
			existing.Streams = mergeStreams(existing.Streams, acct.Streams)
		} else {
			cp := *acct
			c[key] = &AccountDetail{Domain: domain, AccountDetail: &cp}
		}
	}
}

func (c accountCollector) sorted() []*AccountDetail {
	accounts := slices.SortedFunc(maps.Values(c), func(a, b *AccountDetail) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.Domain, b.Domain))
	})
	for _, acct := range accounts {
		sort.Slice(acct.Streams, func(i, j int) bool { return acct.Streams[i].Name < acct.Streams[j].Name })
	}
	return accounts
}

func mergeStreams(a, b []server.StreamDetail) []server.StreamDetail {
	seen := map[string]any{}
	var result []server.StreamDetail

	for _, s := range append(a, b...) {
		if _, found := seen[s.Name]; found {
			continue
		}
		seen[s.Name] = struct{}{}
		result = append(result, s)
	}
	return result
}
