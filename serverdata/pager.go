package serverdata

import (
	"encoding/json"
	"fmt"
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
	subj := s.Subj("PING", p.Target)

	resps, err := unmarshalResponses[T](s, p.Opts(0), subj)
	if err != nil {
		return err
	}

	for _, resp := range resps {
		if err := handle(resp); err != nil {
			return err
		}
		if id := p.ServerID(resp); id != "" && p.Count(resp) == p.Limit {
			offsets[p.ServerID(resp)] = p.Limit
		}
	}

	for len(offsets) > 0 {
		for serverID, offset := range offsets {
			subj = s.Subj(serverID, p.Target)

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

			if err := handle(r); err != nil {
				return fmt.Errorf("%s: %w", subj, err)
			}

			if p.Count(r) == p.Limit {
				offsets[serverID] += p.Limit
			} else {
				delete(offsets, serverID)
			}
		}
	}
	return nil
}
