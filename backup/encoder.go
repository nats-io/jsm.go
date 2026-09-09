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
	"path"

	"github.com/klauspost/compress/s2"
	"github.com/nats-io/nats-server/v2/server/archive"

	"github.com/nats-io/jsm.go/api"
)

// Encoder writes a compressed NATSARC1 stream backup with the same framing
// as the server: one archive entry per item, flushed individually so every
// entry starts its own s2 block
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

	return e.aw.Flush()
}

// WriteEnd writes the end-of-backup sentinel
func (e *Encoder) WriteEnd() error {
	return e.writeEntry("", 0, nil)
}

// Close finishes the archive and the compressed stream
func (e *Encoder) Close() error {
	if err := e.aw.Close(); err != nil {
		return err
	}
	return e.s2w.Close()
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
	if _, err := e.aw.Write(data); err != nil {
		return err
	}
	return e.aw.Flush()
}
