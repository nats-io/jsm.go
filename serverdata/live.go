package serverdata

import (
	"encoding/json"
	"fmt"
	"sort"

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

// CollectSubscriptions pages through Subz responses from all servers and calls handle per page
func (s *Live) CollectSubscriptions(opts server.SubszEventOptions, handle func(resp *server.ServerAPISubszResponse) error) error {
	const defaultSubszLimit = 1024
	pageLimit := opts.Limit
	if pageLimit <= 0 {
		pageLimit = defaultSubszLimit
	}
	opts.Limit = pageLimit

	return pageAll(s, pager[server.ServerAPISubszResponse]{
		Target:      "SUBSZ",
		Limit:       pageLimit,
		StartOffset: opts.SubszOptions.Offset,
		Count: func(resp *server.ServerAPISubszResponse) int {
			if resp == nil || resp.Data == nil {
				return 0
			}
			return len(resp.Data.Subs)
		},
		ServerID: func(resp *server.ServerAPISubszResponse) string {
			if resp == nil || resp.Server == nil {
				return ""
			}
			return resp.Server.ID
		},
		Opts: func(o int) any {
			cp := opts
			cp.SubszOptions.Offset = o
			return cp
		},
	}, handle)
}

// CollectConnections pages through Connz responses from all servers and calls handle per page
func (s *Live) CollectConnections(opts server.ConnzEventOptions, handle func(resp *server.ServerAPIConnzResponse) error) error {
	const defaultConnzLimit = 1024
	pageLimit := opts.Limit
	if pageLimit <= 0 {
		pageLimit = defaultConnzLimit
	}
	opts.Limit = pageLimit

	return pageAll(s, pager[server.ServerAPIConnzResponse]{
		Target:      "CONNZ",
		Limit:       pageLimit,
		StartOffset: opts.ConnzOptions.Offset,
		Count: func(resp *server.ServerAPIConnzResponse) int {
			if resp == nil || resp.Data == nil {
				return 0
			}
			return len(resp.Data.Conns)
		},
		ServerID: func(resp *server.ServerAPIConnzResponse) string {
			if resp == nil || resp.Server == nil {
				return ""
			}
			return resp.Server.ID
		},
		Opts: func(o int) any {
			cp := opts
			cp.ConnzOptions.Offset = o
			return cp
		},
	}, handle)
}

// CollectJsz pages through Jsz responses from all servers and calls handle per page
func (s *Live) CollectJsz(opts server.JszEventOptions, handle func(*server.ServerAPIJszResponse) error) error {
	const defaultJszPageSize = 32

	opts.JSzOptions.Accounts = true
	if opts.JSzOptions.Limit <= 0 {
		opts.JSzOptions.Limit = defaultJszPageSize
	}
	limit := opts.JSzOptions.Limit

	return pageAll(s, pager[server.ServerAPIJszResponse]{
		Target:      "JSZ",
		Limit:       limit,
		StartOffset: opts.JSzOptions.Offset,
		Count: func(resp *server.ServerAPIJszResponse) int {
			if resp == nil || resp.Data == nil {
				return 0
			}
			return len(resp.Data.AccountDetails)
		},
		ServerID: func(resp *server.ServerAPIJszResponse) string {
			if resp == nil || resp.Server == nil {
				return ""
			}
			return resp.Server.ID
		},
		Opts: func(o int) any {
			cp := opts
			cp.JSzOptions.Offset = o
			return cp
		},
	}, handle)
}

// CollectAccounts gathers account-level JetStream metadata from all servers
func (s *Live) CollectAccounts() ([]*server.AccountDetail, error) {
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

	accountMap := map[string]*server.AccountDetail{}
	err := s.CollectJsz(opts, func(resp *server.ServerAPIJszResponse) error {
		if resp.Data == nil {
			return nil
		}
		for _, acct := range resp.Data.AccountDetails {
			if existing, found := accountMap[acct.Name]; found {
				existing.Streams = mergeStreams(existing.Streams, acct.Streams)
			} else {
				cp := *acct
				accountMap[acct.Name] = &cp
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	accounts := make([]*server.AccountDetail, 0, len(accountMap))
	for _, acct := range accountMap {
		accounts = append(accounts, acct)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Name < accounts[j].Name })
	for _, acct := range accounts {
		sort.Slice(acct.Streams, func(i, j int) bool { return acct.Streams[i].Name < acct.Streams[j].Name })
	}
	return accounts, nil
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
