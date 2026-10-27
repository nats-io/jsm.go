package serverdata

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// Live implements Source by querying live servers over NATS
type Live struct {
	nc      *nats.Conn
	reqFn   RequestFunc
	waitFor int
}

type OffsetableJszEvent struct {
	server.JszEventOptions
}

type OffsetableConnzEvent struct {
	server.ConnzEventOptions
}

func (s *OffsetableConnzEvent) SetOffset(o int) { s.ConnzOptions.Offset = o }

func (s *OffsetableJszEvent) SetOffset(o int) { s.JSzOptions.Offset = o }

// NewLive creates a Live data source that delegates requests to the reqFn callback.
// reqFn is required; passing nil returns an error.
func NewLive(nc *nats.Conn, reqFn RequestFunc, waitFor int) (*Live, error) {
	if reqFn == nil {
		return nil, fmt.Errorf("reqFn is required")
	}
	return &Live{nc: nc, reqFn: reqFn, waitFor: waitFor}, nil
}

func (s *Live) Varz(opts server.VarzEventOptions) ([]*server.ServerAPIVarzResponse, error) {
	return unmarshalResponses[server.ServerAPIVarzResponse](s, opts, s.Subj("PING", "VARZ"))
}

func (s *Live) Connz(opts server.ConnzEventOptions) ([]*server.ServerAPIConnzResponse, error) {
	return unmarshalResponses[server.ServerAPIConnzResponse](s, opts, s.Subj("PING", "CONNZ"))
}

func (s *Live) Routez(opts server.RoutezEventOptions) ([]*server.ServerAPIRoutezResponse, error) {
	return unmarshalResponses[server.ServerAPIRoutezResponse](s, opts, s.Subj("PING", "ROUTEZ"))
}

func (s *Live) Gatewayz(opts server.GatewayzEventOptions) ([]*server.ServerAPIGatewayzResponse, error) {
	return unmarshalResponses[server.ServerAPIGatewayzResponse](s, opts, s.Subj("PING", "GATEWAYZ"))
}

func (s *Live) Leafz(opts server.LeafzEventOptions) ([]*server.ServerAPILeafzResponse, error) {
	return unmarshalResponses[server.ServerAPILeafzResponse](s, opts, s.Subj("PING", "LEAFZ"))
}

func (s *Live) Subsz(opts server.SubszEventOptions) ([]*server.ServerAPISubszResponse, error) {
	return unmarshalResponses[server.ServerAPISubszResponse](s, opts, s.Subj("PING", "SUBSZ"))
}

func (s *Live) Jsz(opts server.JszEventOptions) ([]*server.ServerAPIJszResponse, error) {
	return unmarshalResponses[server.ServerAPIJszResponse](s, opts, s.Subj("PING", "JSZ"))
}

func (s *Live) Healthz(opts server.HealthzEventOptions) ([]*server.ServerAPIHealthzResponse, error) {
	return unmarshalResponses[server.ServerAPIHealthzResponse](s, opts, s.Subj("PING", "HEALTHZ"))
}

func (s *Live) Accountz(opts server.AccountzEventOptions) ([]*server.ServerAPIAccountzResponse, error) {
	return unmarshalResponses[server.ServerAPIAccountzResponse](s, opts, s.Subj("PING", "ACCOUNTZ"))
}

func (s *Live) Statz(opts server.StatszEventOptions) ([]*server.ServerStatsMsg, error) {
	return unmarshalResponses[server.ServerStatsMsg](s, opts, s.Subj("PING", ""))
}

func (s *Live) Ipqueuesz(opts server.IpqueueszEventOptions) ([]*server.ServerAPIpqueueszResponse, error) {
	return unmarshalResponses[server.ServerAPIpqueueszResponse](s, opts, s.Subj("PING", "IPQUEUESZ"))
}

func (s *Live) Raftz(opts server.RaftzEventOptions) ([]*server.ServerAPIRaftzResponse, error) {
	return unmarshalResponses[server.ServerAPIRaftzResponse](s, opts, s.Subj("PING", "RAFTZ"))
}

func (s *Live) Profilez(opts server.ProfilezEventOptions) ([]*ProfilezResponse, error) {
	return unmarshalResponses[ProfilezResponse](s, opts, s.Subj("PING", "PROFILEZ"))
}

type Offsetable interface{ SetOffset(int) }

func GatherFirstPage(s *Live, opts any, target string, handleResponsePopulateOffset func(raw [][]byte, subj string) error) error {
	target = strings.ToUpper(target)

	subj := s.Subj("PING", target)
	raw, err := s.reqFn(opts, subj, s.waitFor, s.nc)
	if err != nil {
		return fmt.Errorf("%s: %w", subj, err)
	}

	if len(raw) == 0 {
		return nil
	}

	if err := handleResponsePopulateOffset(raw, subj); err != nil {
		return fmt.Errorf("%s: %w", subj, err)
	}
	return nil
}

// Gets the next page of responses
func NextPage[T Offsetable](s *Live, offsets map[string]int, opts T, target string, pageLimit int, callback func(raw [][]byte, subj string) (responseCount int, err error)) error {
	target = strings.ToUpper(target)

	for serverID, offset := range offsets {
		opts.SetOffset(offset)
		subj := s.Subj(serverID, target)
		raw, err := s.reqFn(opts, subj, 1, s.nc)
		if err != nil {
			return fmt.Errorf("%s: %w", subj, err)
		}

		if len(raw) == 0 {
			delete(offsets, serverID)
			continue
		}

		responseCount, err := callback(raw, subj)
		if err != nil {
			return fmt.Errorf("%s: %w", subj, err)

		}

		if responseCount == pageLimit {
			(offsets)[serverID] += pageLimit
		} else {
			delete(offsets, serverID)
		}
	}

	return nil
}

func (s *Live) Subj(prefix string, target string) string {
	if target != "" {
		target = "." + target
	}
	return fmt.Sprintf("$SYS.REQ.SERVER.%s%s", prefix, target)
}

// HandleSinglePageResponse must return the page's connection count
func (s *Live) CollectConnections(connz server.ConnzEventOptions, HandleSinglePageResponse func(raw [][]byte, subj string) (responseCount int, err error)) error {
	const defaultConnzLimit = 1024
	pageLimit := connz.Limit
	if pageLimit <= 0 {
		pageLimit = defaultConnzLimit
	}
	connz.Limit = pageLimit
	opts := OffsetableConnzEvent{connz}

	offsets := map[string]int{}

	err := GatherFirstPage(s, opts, "CONNZ", func(results [][]byte, subj string) error {
		resps, err := unmarshalResponsesHelper[server.ServerAPIConnzResponse](results, subj)
		if err != nil {
			return err
		}
		for _, resp := range resps {
			if resp.Data == nil || resp.Server == nil {
				continue
			}
			if len(resp.Data.Conns) == pageLimit && resp.Server.ID != "" {
				offsets[resp.Server.ID] = pageLimit
			}
		}
		_, err = HandleSinglePageResponse(results, subj)
		return err
	})

	if err != nil {
		return fmt.Errorf("gather first page response : %w", err)
	}

	for len(offsets) > 0 {
		err := NextPage(s, offsets, &opts, "CONNZ", pageLimit, HandleSinglePageResponse)

		if err != nil {
			return fmt.Errorf("next page response : %w", err)
		}
	}

	return nil
}

// CollectAccounts gathers account-level JetStream metadata from all servers
func (s *Live) CollectAccounts() ([]*server.AccountDetail, error) {
	const pageLimit = 1024

	baseOpts := OffsetableJszEvent{
		server.JszEventOptions{
			JSzOptions: server.JSzOptions{
				Accounts: true,
				Streams:  true,
				Consumer: true,
				Config:   true,
				Limit:    pageLimit,
			},
		},
	}

	accountMap := map[string]*server.AccountDetail{}
	mergeAccounts := func(details []*server.AccountDetail) {
		for _, acct := range details {
			if existing, found := accountMap[acct.Name]; found {
				existing.Streams = mergeStreams(existing.Streams, acct.Streams)
			} else {
				cp := *acct
				accountMap[acct.Name] = &cp
			}
		}
	}

	offsets := map[string]int{}
	err := GatherFirstPage(s, baseOpts, "JSZ", func(results [][]byte, subj string) error {

		initialResponses, err := unmarshalResponsesHelper[server.ServerAPIJszResponse](results, subj)
		if err != nil {
			return err
		}

		for _, resp := range initialResponses {
			if resp.Data == nil || resp.Server == nil {
				continue
			}
			mergeAccounts(resp.Data.AccountDetails)
			if len(resp.Data.AccountDetails) == pageLimit && resp.Server.ID != "" {
				offsets[resp.Server.ID] = pageLimit
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("gather first page response : %w", err)
	}

	// JSZ Limit/Offset is per-server pagination, so follow-ups must address one
	// server at a time. Server IDs are globally unique; server names are not
	// (super-clusters, leaf nodes, and JetStream domains can repeat names).
	// Keying by ID and using $SYS.REQ.SERVER.<ID>.JSZ avoids losing accounts
	// from servers that share a name with another server in the deployment.

	for len(offsets) > 0 {
		err := NextPage(s, offsets, &baseOpts, "JSZ", pageLimit, func(results [][]byte, subj string) (int, error) {
			jsz, err := unmarshalResponsesHelper[server.ServerAPIJszResponse](results, subj)
			if err != nil {
				return 0, err
			}
			if len(jsz) == 0 || jsz[0].Data == nil {
				return 0, nil
			}

			mergeAccounts(jsz[0].Data.AccountDetails)
			return len(jsz[0].Data.AccountDetails), nil
		})

		if err != nil {
			return nil, fmt.Errorf("next page response : %w", err)
		}
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

	resps, err := unmarshalResponsesHelper[T](results, subj)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", subj, err)
	}

	return resps, nil
}

func unmarshalResponsesHelper[T any](results [][]byte, subj string) ([]*T, error) {
	responses := make([]*T, 0, len(results))
	for _, raw := range results {
		resp := new(T)
		if err := json.Unmarshal(raw, resp); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
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
