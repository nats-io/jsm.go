package serverdata

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats-server/v2/server"
)

type pager[T any] struct {
	Target   string
	Limit    int
	Count    func(*T) int
	ServerID func(*T) string
	Opts     func(offset int) any
}

// pageAll walks every page of a paged system request across all servers,
// passing each server response to handle as it arrives.
//
// Limit/Offset is per-server pagination, so follow-ups must address one
// server at a time. Server IDs are globally unique; server names are not
// (super-clusters, leaf nodes, and JetStream domains can repeat names).
// Keying by ID and using $SYS.REQ.SERVER.<ID>.<TARGET> avoids losing items
// from servers that share a name with another server in the deployment.
//
// The first page is broadcast to all servers. Any server that returns a full
// page is asked for the next one directly, each with its own offset, until it
// returns a short page. Nothing is retained between pages, so memory is bounded
// by one page per server rather than by the size of the fleet.
func pageAll[T any](s *Live, p pager[T], handle func(resp *T) error) error {
	offsets := map[string]int{}
	subj := fmt.Sprintf("$SYS.REQ.SERVER.%s.%s", "PING", p.Target)

	resps, err := unmarshalResponses[T](s, p.Opts(0), subj)
	if err != nil {
		return err
	}

	for _, resp := range resps {
		id := p.ServerID(resp)
		count := p.Count(resp)

		if err := handle(resp); err != nil {
			return err
		}

		if id != "" && count == p.Limit {
			offsets[id] = 0 + p.Limit
		}
	}

	for len(offsets) > 0 {
		for serverID, offset := range offsets {
			subj = fmt.Sprintf("$SYS.REQ.SERVER.%s.%s", serverID, p.Target)

			raw, err := s.reqFn(p.Opts(offset), subj, 1, s.nc)
			if err != nil {
				return fmt.Errorf("%s: %w", subj, err)
			}

			if len(raw) == 0 {
				delete(offsets, serverID)
				continue
			}

			r := new(T)
			if err := json.Unmarshal(raw[0], r); err != nil {
				return fmt.Errorf("%s: unmarshal: %w", subj, err)
			}

			count := p.Count(r)

			if err := handle(r); err != nil {
				return fmt.Errorf("%s: %w", subj, err)
			}

			if count == p.Limit {
				offsets[serverID] += p.Limit
			} else {
				delete(offsets, serverID)
			}
		}
	}
	return nil
}

func connzPager(opts server.ConnzEventOptions) pager[server.ServerAPIConnzResponse] {
	const defaultConnzLimit = 1024

	if opts.Limit <= 0 {
		opts.Limit = defaultConnzLimit
	}

	return pager[server.ServerAPIConnzResponse]{
		Target: "CONNZ",
		Limit:  opts.Limit,
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
	}
}

func jszPager(opts server.JszEventOptions) pager[server.ServerAPIJszResponse] {
	const defaultJszPageSize = 32

	opts.JSzOptions.Accounts = true
	if opts.JSzOptions.Limit <= 0 {
		opts.JSzOptions.Limit = defaultJszPageSize
	}

	return pager[server.ServerAPIJszResponse]{
		Target: "JSZ",
		Limit:  opts.JSzOptions.Limit,
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
	}
}

func subszPager(opts server.SubszEventOptions) pager[server.ServerAPISubszResponse] {
	const defaultSubszLimit = 1024

	opts.SubszOptions.Subscriptions = true
	if opts.Limit <= 0 {
		opts.Limit = defaultSubszLimit
	}

	return pager[server.ServerAPISubszResponse]{
		Target: "SUBSZ",
		Limit:  opts.Limit,
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
	}
}
