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
	"bytes"
	"fmt"
	"io"
	"slices"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type dropKind int

const (
	keep dropKind = iota
	dropSequence
	dropTime
	dropSubject
	dropHeader
	dropPayload
	dropLastPerSubject
	dropKVCompact
)

func (k dropKind) contentFilter() bool {
	return k == dropHeader || k == dropPayload
}

// evalMeta applies the filters that need only the entry header, cheapest first
func (o *editOptions) evalMeta(m *Message) dropKind {
	if (o.hasFirstSeq && m.Seq < o.firstSeq) || (o.hasLastSeq && m.Seq > o.lastSeq) {
		return dropSequence
	}
	if (o.hasAfter && m.Ts < o.after.UnixNano()) || (o.hasBefore && m.Ts >= o.before.UnixNano()) {
		return dropTime
	}
	if matchesAny(o.excludeSubjects, m.Subject) {
		return dropSubject
	}
	if len(o.subjects) > 0 && !matchesAny(o.subjects, m.Subject) {
		return dropSubject
	}
	return keep
}

func matchesAny(patterns []string, subject string) bool {
	for _, p := range patterns {
		if server.SubjectsCollide(p, subject) {
			return true
		}
	}
	return false
}

func (o *editOptions) evalHeaders(h nats.Header) dropKind {
	for _, name := range o.noHeader {
		if len(headerValues(h, name)) > 0 {
			return dropHeader
		}
	}
	if len(o.headerPresent)+len(o.headerValues) == 0 {
		return keep
	}
	for _, name := range o.headerPresent {
		if len(headerValues(h, name)) > 0 {
			return keep
		}
	}
	for _, hm := range o.headerValues {
		if slices.Contains(headerValues(h, hm.name), hm.value) {
			return keep
		}
	}
	return dropHeader
}

func (o *editOptions) evalPayload(payload []byte) dropKind {
	for _, re := range o.excludePayloadMatch {
		if re.Match(payload) {
			return dropPayload
		}
	}
	if len(o.payloadMatch) == 0 {
		return keep
	}
	for _, re := range o.payloadMatch {
		if re.Match(payload) {
			return keep
		}
	}
	return dropPayload
}

// msgBody buffers a prefix of a message entry on demand so filters can look
// at the headers or payload while the remainder still streams to the encoder
type msgBody struct {
	m       *Message
	buf     []byte
	hdr     nats.Header
	decoded bool
}

func (b *msgBody) prefix(n int64) ([]byte, error) {
	if int64(len(b.buf)) >= n {
		return b.buf[:n], nil
	}
	grown := make([]byte, n)
	copy(grown, b.buf)
	if _, err := io.ReadFull(b.m.Body, grown[len(b.buf):]); err != nil {
		return nil, fmt.Errorf("reading message %s seq %d: %w", b.m.Subject, b.m.Seq, err)
	}
	b.buf = grown
	return b.buf, nil
}

// headers decodes the header block once; a message without headers yields nil
func (b *msgBody) headers() (nats.Header, error) {
	if b.decoded {
		return b.hdr, nil
	}
	if b.m.HdrSize > 0 {
		raw, err := b.prefix(b.m.HdrSize)
		if err != nil {
			return nil, err
		}
		if b.hdr, err = nats.DecodeHeadersMsg(raw); err != nil {
			return nil, fmt.Errorf("message %s seq %d: %w", b.m.Subject, b.m.Seq, err)
		}
	}
	b.decoded = true
	return b.hdr, nil
}

func (b *msgBody) payload() ([]byte, error) {
	all, err := b.prefix(b.m.HdrSize + b.m.PayloadSize)
	if err != nil {
		return nil, err
	}
	return all[b.m.HdrSize:], nil
}

func (b *msgBody) reader() io.Reader {
	if len(b.buf) == 0 {
		return b.m.Body
	}
	return io.MultiReader(bytes.NewReader(b.buf), b.m.Body)
}
