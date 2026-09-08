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

package backup

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/klauspost/compress/s2"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats-server/v2/server/archive"

	"github.com/nats-io/jsm.go/api"
)

type decodePhase int

const (
	phaseState decodePhase = iota
	phaseConsumers
	phaseMessages
	phaseDone
)

// Decoder reads a compressed NATSARC1 stream backup as typed items, enforcing
// the invariants the server's restore relies on: state.json first, exactly
// consumer_count consumer entries, strictly ascending message sequences and a
// terminating sentinel
type Decoder struct {
	r             *archive.Reader
	state         *api.StreamState
	consumersLeft int
	phase         decodePhase
	lastSeq       uint64
	ordinal       int
	last          EntryRef
	err           error
}

// NewDecoder reads the s2 compressed archive from r
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: archive.NewReader(s2.NewReader(r))}
}

// LastGood identifies the last entry that decoded cleanly
func (d *Decoder) LastGood() EntryRef {
	return d.last
}

// Next returns the next item; End is returned exactly once, after which io.EOF is returned
func (d *Decoder) Next() (Item, error) {
	if d.err != nil {
		return nil, d.err
	}
	if d.phase == phaseDone {
		return nil, io.EOF
	}

	item, err := d.next()
	if err != nil {
		d.err = err
		return nil, err
	}

	return item, nil
}

func (d *Decoder) next() (Item, error) {
	hdr, err := d.r.Next()
	if err != nil {
		if d.phase == phaseState {
			return nil, fmt.Errorf("%w: %w", ErrNotV2Backup, err)
		}
		return nil, fmt.Errorf("backup ends before the end-of-backup sentinel after %s: %w", d.last, err)
	}

	switch d.phase {
	case phaseState:
		return d.decodeState(hdr)
	case phaseConsumers:
		return d.decodeConsumer(hdr)
	default:
		return d.decodeMessage(hdr)
	}
}

func (d *Decoder) decodeState(hdr *archive.Header) (Item, error) {
	if hdr.Name != stateEntry {
		return nil, fmt.Errorf("expected %s first, found %q", stateEntry, hdr.Name)
	}

	data, err := io.ReadAll(d.r)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", stateEntry, err)
	}

	var st api.StreamState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("error in %s: %w", stateEntry, err)
	}
	if st.Consumers < 0 {
		return nil, fmt.Errorf("error in %s: negative consumer_count %d", stateEntry, st.Consumers)
	}

	d.state = &st
	d.consumersLeft = st.Consumers
	d.lastSeq = st.FirstSeq - 1
	d.phase = phaseConsumers
	if d.consumersLeft == 0 {
		d.phase = phaseMessages
	}
	d.advance("state", stateEntry, 0)

	return &State{Ts: hdr.Timestamp, State: st}, nil
}

func (d *Decoder) decodeConsumer(hdr *archive.Header) (Item, error) {
	name, found := strings.CutPrefix(hdr.Name, consumerPrefix)
	if !found {
		return nil, fmt.Errorf("expected consumer entry (%d of %d remaining), found %q", d.consumersLeft, d.state.Consumers, hdr.Name)
	}

	data, err := io.ReadAll(d.r)
	if err != nil {
		return nil, fmt.Errorf("failed to read consumer %q state: %w", name, err)
	}

	var scs server.SnapshotConsumerState
	if err := json.Unmarshal(data, &scs); err != nil {
		return nil, fmt.Errorf("failed to decode consumer %q state: %w", name, err)
	}
	if scs.ConsumerConfig == nil {
		return nil, fmt.Errorf("consumer %q is missing config", name)
	}
	if scs.ConsumerState == nil {
		return nil, fmt.Errorf("consumer %q is missing state", name)
	}

	d.consumersLeft--
	if d.consumersLeft == 0 {
		d.phase = phaseMessages
	}
	d.advance("consumer", name, 0)

	return &Consumer{Name: name, Ts: hdr.Timestamp, Data: data}, nil
}

func (d *Decoder) decodeMessage(hdr *archive.Header) (Item, error) {
	if hdr.Sequence == 0 {
		if hdr.Timestamp == 0 && hdr.HeaderSize == 0 && hdr.PayloadSize == 0 {
			d.phase = phaseDone
			d.advance("end", "", 0)
			return End{}, nil
		}
		return nil, fmt.Errorf("expected message sequence, found entry %q with sequence 0 after %s", hdr.Name, d.last)
	}

	if hdr.Sequence <= d.lastSeq {
		if d.lastSeq == math.MaxUint64 {
			return nil, fmt.Errorf("message sequence %d out of order: %s has first_seq 0", hdr.Sequence, stateEntry)
		}
		return nil, fmt.Errorf("message sequence %d out of order after %s", hdr.Sequence, d.last)
	}

	d.lastSeq = hdr.Sequence
	d.advance("message", hdr.Name, hdr.Sequence)

	return &Message{
		Subject:     hdr.Name,
		Seq:         hdr.Sequence,
		Ts:          hdr.Timestamp,
		HdrSize:     hdr.HeaderSize,
		PayloadSize: hdr.PayloadSize,
		Body:        d.r,
	}, nil
}

func (d *Decoder) advance(kind, name string, seq uint64) {
	d.ordinal++
	d.last = EntryRef{Ordinal: d.ordinal, Kind: kind, Name: name, Seq: seq}
}
