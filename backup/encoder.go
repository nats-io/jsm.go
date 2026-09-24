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
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"math"
	"path"
	"slices"

	"github.com/klauspost/compress/s2"
	"github.com/nats-io/nats-server/v2/server/archive"
	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

// Encoder writes a compressed NATSARC1 stream backup with the same framing
// as the server: one archive entry per item. Entries are not flushed
// individually, s2 closes a block when its buffer fills, so the output is
// large blocks rather than one block and one write per message
type Encoder struct {
	s2w *s2.Writer
	aw  *archive.Writer
	buf []byte
}

// NewEncoder writes the s2 compressed archive to w
func NewEncoder(w io.Writer) *Encoder {
	s2w := s2.NewWriter(w)
	return &Encoder{s2w: s2w, aw: archive.NewWriter(s2w), buf: make([]byte, 64*1024)}
}

// stateHeadPad fits the state.json encodeStateHead writes with every field
// at its widest
var stateHeadPad = func() int {
	js, _ := json.Marshal(api.StreamState{Msgs: math.MaxUint64, Bytes: math.MaxUint64, FirstSeq: math.MaxUint64, LastSeq: math.MaxUint64, Consumers: math.MaxInt})
	return len(js)
}()

// encodeStateHead renders the NATSARC1 preamble and state.json as an
// uncompressed s2 stream of the same size for any st, so a writer can reserve
// it before the totals are known and overwrite it after. The JSON is padded
// with spaces, which the server's decoder and ours skip
func encodeStateHead(ts int64, st api.StreamState) ([]byte, error) {
	js, err := json.Marshal(st)
	if err != nil {
		return nil, err
	}
	if len(js) > stateHeadPad {
		return nil, fmt.Errorf("%s is %d bytes, the head has room for %d", stateEntry, len(js), stateHeadPad)
	}
	js = append(js, bytes.Repeat([]byte{' '}, stateHeadPad-len(js))...)

	var out bytes.Buffer
	s2w := s2.NewWriter(&out, s2.WriterUncompressed())
	aw := archive.NewWriter(s2w)
	if err := aw.WriteHeader(&archive.Header{Name: stateEntry, Timestamp: ts, PayloadSize: int64(len(js))}); err != nil {
		return nil, err
	}
	if _, err := aw.Write(js); err != nil {
		return nil, err
	}
	if err := s2w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// newEntryEncoder writes archive entries without the NATSARC1 preamble, as a
// separate s2 stream that follows an encodeStateHead head. s2 readers, the
// server's included, decode consecutive streams as one
func newEntryEncoder(w io.Writer) *Encoder {
	s2w := s2.NewWriter(w)
	return &Encoder{s2w: s2w, aw: archive.NewWriter(&skipWriter{w: s2w, skip: len(archive.MagicBytes)}), buf: make([]byte, 64*1024)}
}

// skipWriter drops the first skip bytes written, the preamble archive.Writer
// always writes before its first entry
type skipWriter struct {
	w    io.Writer
	skip int
}

func (s *skipWriter) Write(p []byte) (int, error) {
	n := len(p)
	if s.skip > 0 {
		k := min(s.skip, len(p))
		s.skip -= k
		p = p[k:]
	}
	if len(p) > 0 {
		if _, err := s.w.Write(p); err != nil {
			return 0, err
		}
	}
	return n, nil
}

// WriteState writes the leading state.json entry
func (e *Encoder) WriteState(ts int64, st api.StreamState) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return e.writeEntry(stateEntry, ts, data)
}

// WriteConsumer writes one consumers/<name> entry
func (e *Encoder) WriteConsumer(name string, ts int64, data []byte) error {
	return e.writeEntry(path.Join("consumers", name), ts, data)
}

// WriteMessage writes one message, streaming exactly HdrSize+PayloadSize bytes from Body
func (e *Encoder) WriteMessage(m *Message) error {
	hdr := &archive.Header{
		Name:        m.Subject,
		Timestamp:   m.Ts,
		Sequence:    m.Seq,
		HeaderSize:  m.HdrSize,
		PayloadSize: m.PayloadSize,
	}
	if err := e.aw.WriteHeader(hdr); err != nil {
		return err
	}

	want := m.HdrSize + m.PayloadSize
	if want > 0 {
		n, err := io.CopyBuffer(e.aw, io.LimitReader(m.Body, want), e.buf)
		if err != nil {
			return err
		}
		if n != want {
			return fmt.Errorf("message %s seq %d: body is %d bytes, %d declared", m.Subject, m.Seq, n, want)
		}
	}

	return nil
}

// writeMessageBytes is WriteMessage for a header block and body already in memory
func (e *Encoder) writeMessageBytes(subject string, seq uint64, ts int64, hdr, body []byte) error {
	h := &archive.Header{
		Name:        subject,
		Timestamp:   ts,
		Sequence:    seq,
		HeaderSize:  int64(len(hdr)),
		PayloadSize: int64(len(body)),
	}
	if err := e.aw.WriteHeader(h); err != nil {
		return err
	}
	if len(hdr) > 0 {
		if _, err := e.aw.Write(hdr); err != nil {
			return err
		}
	}
	if len(body) > 0 {
		if _, err := e.aw.Write(body); err != nil {
			return err
		}
	}
	return nil
}

// WriteEnd writes the end-of-backup sentinel
func (e *Encoder) WriteEnd() error {
	return e.writeEntry("", 0, nil)
}

// Close finishes the archive and the compressed stream. The s2 writer is
// closed even when the archive is left mid-entry, its writer goroutine
// only exits on Close
func (e *Encoder) Close() error {
	err := e.aw.Close()
	if cerr := e.s2w.Close(); err == nil {
		err = cerr
	}
	return err
}

func (e *Encoder) writeEntry(name string, ts int64, data []byte) error {
	hdr := &archive.Header{
		Name:        name,
		Timestamp:   ts,
		PayloadSize: int64(len(data)),
	}
	if err := e.aw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := e.aw.Write(data)
	return err
}

// encodeHeaders renders h as a stored NATS/1.0 header block with keys sorted
// so the output is deterministic. An empty map yields nil
func encodeHeaders(h nats.Header) []byte {
	if len(h) == 0 {
		return nil
	}

	var out bytes.Buffer
	out.WriteString("NATS/1.0\r\n")
	for _, key := range slices.Sorted(maps.Keys(h)) {
		for _, val := range h[key] {
			out.WriteString(key + ": " + val + "\r\n")
		}
	}
	out.WriteString("\r\n")
	return out.Bytes()
}
