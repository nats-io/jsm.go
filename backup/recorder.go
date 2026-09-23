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
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
)

// Recorder writes the messages a subscription delivers into a stream
// backup. A stream capture, SourceInfo.Stream set, keeps stream sequences as
// a snapshot does and skips redeliveries. A core capture numbers messages
// 1..N. The directory appears once Close commits it. Not safe for
// concurrent use
type Recorder struct {
	tgt      *target
	arc      io.WriteCloser
	enc      *Encoder
	cfg      api.StreamConfig
	src      SourceInfo
	preserve bool
	started  bool
	first    uint64
	last     uint64
	msgs     uint64
	bytes    uint64
	err      error
	done     bool
}

// RecorderResult describes a committed capture
type RecorderResult struct {
	Dir      string
	Messages uint64
	Bytes    uint64
}

var errRecorderClosed = errors.New("recorder is closed")

// directGetEnvelope are the headers the server appends to a stored
// message in a batched direct get response
var directGetEnvelope = []string{server.JSStream, server.JSSubject, server.JSSequence, server.JSTimeStamp, server.JSNumPending, server.JSLastSequence}

// NewRecorder opens a capture into dir, which must not exist or must be
// empty. cfg is the configuration the backup restores with. Storage is
// forced to file, memory streams cannot be restored. A core capture clears
// FirstSeq since its messages start at 1. A zero src.Started is set to now
func NewRecorder(dir string, cfg api.StreamConfig, src SourceInfo) (*Recorder, error) {
	if !jsm.IsValidName(cfg.Name) {
		return nil, fmt.Errorf("invalid stream name %q", cfg.Name)
	}
	tgt, err := newTarget(dir)
	if err != nil {
		return nil, err
	}
	preserve := src.Stream != ""
	cfg.Storage = api.FileStorage
	if !preserve {
		cfg.FirstSeq = 0
	}
	if src.Started.IsZero() {
		src.Started = time.Now()
	}

	arc, err := tgt.archive()
	if err != nil {
		tgt.discard()
		return nil, err
	}

	return &Recorder{tgt: tgt, arc: arc, enc: NewEncoder(arc), cfg: cfg, src: src, preserve: preserve}, nil
}

// Write appends m with its headers as received. Sequence and timestamp come
// from the JetStream ack reply subject, a core message is timestamped now.
// After an error the recorder is unusable and the caller should Discard it
func (r *Recorder) Write(m *nats.Msg) error {
	ts := time.Now()
	var seq uint64
	if strings.HasPrefix(m.Reply, "$JS.ACK.") {
		if info, err := jsm.ParseJSMsgMetadataReply(m.Reply); err == nil {
			ts = info.TimeStamp()
			seq = info.StreamSequence()
		}
	}
	return r.write(m.Subject, seq, ts, m.Header, m.Data)
}

// WriteDirect appends m, a batched direct get response, as the stored
// message it carries. The server appends its envelope after the stored
// headers, which can use the same names, so the envelope is the last value
// of each
func (r *Recorder) WriteDirect(m *nats.Msg) error {
	h := make(nats.Header, len(m.Header))
	for k, v := range m.Header {
		h[k] = v
	}

	env := make(map[string]string, len(directGetEnvelope))
	for _, k := range directGetEnvelope {
		vals := h.Values(k)
		if len(vals) == 0 {
			return fmt.Errorf("direct get response on %s has no %s header", m.Subject, k)
		}
		env[k] = vals[len(vals)-1]
		if len(vals) == 1 {
			h.Del(k)
		} else {
			h[k] = vals[:len(vals)-1]
		}
	}

	ts, err := time.Parse(time.RFC3339Nano, env[server.JSTimeStamp])
	if err != nil {
		return fmt.Errorf("direct get response on %s: %w", m.Subject, err)
	}
	seq, err := strconv.ParseUint(env[server.JSSequence], 10, 64)
	if err != nil {
		return fmt.Errorf("direct get response on %s: %w", m.Subject, err)
	}

	return r.write(env[server.JSSubject], seq, ts, h, m.Data)
}

func (r *Recorder) write(subject string, seq uint64, ts time.Time, h nats.Header, data []byte) error {
	if r.err != nil {
		return r.err
	}
	if r.done {
		return errRecorderClosed
	}

	switch {
	case !r.preserve:
		seq = r.last + 1
	case seq == 0:
		r.err = fmt.Errorf("message on %s carries no stream sequence", subject)
		return r.err
	case seq <= r.last:
		return nil
	}

	if !r.started {
		if err := r.writeState(seq); err != nil {
			return err
		}
	}

	hdr := encodeHeaders(h)
	if err := r.enc.writeMessageBytes(subject, seq, ts.UnixNano(), hdr, data); err != nil {
		r.err = err
		return err
	}

	if r.msgs == 0 {
		r.first = seq
	}
	r.last = seq
	r.msgs++
	r.bytes += storedMsgSize(len(subject), int64(len(hdr)), int64(len(data)))

	return nil
}

// writeState writes state.json at the first message because restore starts
// the stream at its first_seq. last_seq one below is the empty range restore
// accepts
func (r *Recorder) writeState(first uint64) error {
	r.started = true
	if err := r.enc.WriteState(r.src.Started.UnixNano(), api.StreamState{FirstSeq: first, LastSeq: first - 1}); err != nil {
		r.err = err
		return err
	}
	return nil
}

// Close writes the sentinel and the meta file and commits the backup.
// dropped is recorded in the source block. A failed Close discards the
// staging directory. Zero messages restores as an empty stream
func (r *Recorder) Close(dropped uint64) (*RecorderResult, error) {
	if r.done {
		return nil, errRecorderClosed
	}
	r.done = true
	if r.err != nil {
		r.Discard()
		return nil, r.err
	}

	var err error
	if !r.started {
		err = r.writeState(1)
	}
	if err == nil {
		err = r.enc.WriteEnd()
	}
	if err == nil {
		err = r.enc.Close()
	}
	if err == nil {
		err = r.arc.Close()
		r.arc = nil
	}
	if err != nil {
		r.Discard()
		return nil, err
	}

	r.src.Ended = time.Now()
	r.src.Dropped = dropped
	mf := &metaFile{
		Config: r.cfg,
		State:  api.StreamState{Msgs: r.msgs, Bytes: r.bytes, FirstSeq: max(r.first, 1), LastSeq: r.last},
		Source: &r.src,
	}
	if err := r.tgt.writeMeta(mf); err != nil {
		r.Discard()
		return nil, err
	}
	if err := r.tgt.commit(); err != nil {
		r.Discard()
		return nil, err
	}

	return &RecorderResult{Dir: r.tgt.final, Messages: r.msgs, Bytes: r.bytes}, nil
}

// Discard abandons the capture and removes the staging directory
func (r *Recorder) Discard() {
	r.done = true
	if r.arc != nil {
		r.enc.Close()
		r.arc.Close()
		r.arc = nil
	}
	r.tgt.discard()
}
