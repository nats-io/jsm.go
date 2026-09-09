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
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/klauspost/compress/s2"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats-server/v2/server/archive"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
)

type testMsg struct {
	subject string
	seq     uint64
	ts      int64
	hdr     string
	body    string
}

func (m testMsg) item() *Message {
	return &Message{
		Subject:     m.subject,
		Seq:         m.seq,
		Ts:          m.ts,
		HdrSize:     int64(len(m.hdr)),
		PayloadSize: int64(len(m.body)),
		Body:        strings.NewReader(m.hdr + m.body),
	}
}

func consumerJSON(t *testing.T, name string) []byte {
	t.Helper()
	data, err := json.Marshal(server.SnapshotConsumerState{
		ConsumerConfig: &server.ConsumerConfig{Durable: name, Name: name, FilterSubject: "orders.>"},
		ConsumerState:  &server.ConsumerState{Delivered: server.SequencePair{Consumer: 1, Stream: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func encodeBackup(t *testing.T, st api.StreamState, consumers map[string][]byte, msgs []testMsg, sentinel bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.WriteState(1000, st); err != nil {
		t.Fatal(err)
	}
	for _, name := range slices.Sorted(maps.Keys(consumers)) {
		if err := enc.WriteConsumer(name, 2000, consumers[name]); err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range msgs {
		if err := enc.WriteMessage(m.item()); err != nil {
			t.Fatal(err)
		}
	}
	if sentinel {
		if err := enc.WriteEnd(); err != nil {
			t.Fatal(err)
		}
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func framed(t *testing.T, data []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w := s2.NewWriter(&b)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func rawArchive(t *testing.T, fn func(w *archive.Writer)) []byte {
	t.Helper()
	var buf bytes.Buffer
	s2w := s2.NewWriter(&buf)
	aw := archive.NewWriter(s2w)
	fn(aw)
	if err := s2w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func rawEntry(t *testing.T, w *archive.Writer, name string, ts int64, seq uint64, hdrSize, payloadSize int64, data []byte) {
	t.Helper()
	err := w.WriteHeader(&archive.Header{Name: name, Timestamp: ts, Sequence: seq, HeaderSize: hdrSize, PayloadSize: payloadSize})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil && !errors.Is(err, archive.ErrWriteTooLong) {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
}

func stateJSON(t *testing.T, st api.StreamState) []byte {
	t.Helper()
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func decodeAll(t *testing.T, data []byte) ([]Item, error) {
	t.Helper()
	dec := NewDecoder(bytes.NewReader(data))
	var items []Item
	for {
		item, err := dec.Next()
		if err != nil {
			return items, err
		}
		if m, ok := item.(*Message); ok {
			body, err := io.ReadAll(m.Body)
			if err != nil {
				return items, err
			}
			m.Body = bytes.NewReader(body)
		}
		items = append(items, item)
		if _, ok := item.(End); ok {
			return items, nil
		}
	}
}

func TestCodecRoundTrip(t *testing.T) {
	st := api.StreamState{Msgs: 3, Bytes: 999, FirstSeq: 2, LastSeq: 9, Consumers: 2}
	consumers := map[string][]byte{"c1": consumerJSON(t, "c1"), "c2": consumerJSON(t, "c2")}
	msgs := []testMsg{
		{"orders.new", 2, 10, "", "first"},
		{"orders.paid", 5, 20, "NATS/1.0\r\nKV-Operation: DEL\r\n\r\n", ""},
		{"orders.shipped", 9, 30, "NATS/1.0\r\nX-A: 1\r\n\r\n", strings.Repeat("z", 100000)},
	}

	data := encodeBackup(t, st, consumers, msgs, true)
	items, err := decodeAll(t, data)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(items) != 1+2+3+1 {
		t.Fatalf("expected 7 items, got %d", len(items))
	}

	s := items[0].(*State)
	if s.Ts != 1000 || !reflect.DeepEqual(s.State, st) {
		t.Fatalf("state mismatch: %+v", s)
	}
	for i, name := range []string{"c1", "c2"} {
		c := items[1+i].(*Consumer)
		if c.Name != name || c.Ts != 2000 || !bytes.Equal(c.Data, consumers[name]) {
			t.Fatalf("consumer %d mismatch: %+v", i, c)
		}
	}
	for i, want := range msgs {
		got := items[3+i].(*Message)
		body, _ := io.ReadAll(got.Body)
		if got.Subject != want.subject || got.Seq != want.seq || got.Ts != want.ts || got.HdrSize != int64(len(want.hdr)) || got.PayloadSize != int64(len(want.body)) || string(body) != want.hdr+want.body {
			t.Fatalf("message %d mismatch: %+v", i, got)
		}
	}
	if _, ok := items[6].(End); !ok {
		t.Fatalf("expected End, got %T", items[6])
	}

	dec := NewDecoder(bytes.NewReader(data))
	for range items {
		if _, err := dec.Next(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := dec.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF after End, got %v", err)
	}
}

func TestEncoderRejectsShortBody(t *testing.T) {
	enc := NewEncoder(io.Discard)
	if err := enc.WriteState(0, api.StreamState{}); err != nil {
		t.Fatal(err)
	}
	err := enc.WriteMessage(&Message{Subject: "a", Seq: 1, PayloadSize: 10, Body: strings.NewReader("short")})
	if err == nil || !strings.Contains(err.Error(), "10 declared") {
		t.Fatalf("expected short body error, got %v", err)
	}
}

func TestDecoderRejectsNonV2(t *testing.T) {
	cases := map[string][]byte{
		"empty":    nil,
		"plain s2": s2.Encode(nil, []byte("not an archive at all")),
		"tar-ish": func() []byte {
			var b bytes.Buffer
			w := s2.NewWriter(&b)
			w.Write(make([]byte, 1024))
			w.Close()
			return b.Bytes()
		}(),
		"random": []byte("this is definitely not s2 framed data"),
		"magic only": func() []byte {
			var b bytes.Buffer
			w := s2.NewWriter(&b)
			w.Write([]byte(archive.MagicBytes))
			w.Close()
			return b.Bytes()
		}(),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := decodeAll(t, data)
			if !errors.Is(err, ErrNotV2Backup) {
				t.Fatalf("expected ErrNotV2Backup, got %v", err)
			}
			if !strings.Contains(err.Error(), "NATS Server 2.15") {
				t.Fatalf("error should name the requirement: %v", err)
			}
		})
	}

	_, err := decodeAll(t, framed(t, []byte("not an archive at all")))
	if !errors.Is(err, archive.ErrInvalidArchive) {
		t.Fatalf("expected the reader's ErrInvalidArchive underneath, got %v", err)
	}
	_, err = decodeAll(t, nil)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected the reader's io.EOF underneath, got %v", err)
	}
}

func TestDecoderStateMustBeFirst(t *testing.T) {
	data := rawArchive(t, func(w *archive.Writer) {
		rawEntry(t, w, "consumers/c1", 0, 0, 0, 2, []byte("{}"))
	})
	_, err := decodeAll(t, data)
	if err == nil || !strings.Contains(err.Error(), `expected state.json first, found "consumers/c1"`) {
		t.Fatalf("unexpected error: %v", err)
	}

	data = rawArchive(t, func(w *archive.Writer) {
		rawEntry(t, w, "state.json", 0, 0, 0, 3, []byte("{{{"))
	})
	_, err = decodeAll(t, data)
	if err == nil || !strings.Contains(err.Error(), "error in state.json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecoderConsumerCount(t *testing.T) {
	st := stateJSON(t, api.StreamState{FirstSeq: 1, LastSeq: 1, Consumers: 2})
	data := rawArchive(t, func(w *archive.Writer) {
		rawEntry(t, w, "state.json", 0, 0, 0, int64(len(st)), st)
		c := consumerJSON(t, "c1")
		rawEntry(t, w, "consumers/c1", 0, 0, 0, int64(len(c)), c)
		rawEntry(t, w, "orders.new", 5, 1, 0, 2, []byte("hi"))
	})
	_, err := decodeAll(t, data)
	if err == nil || !strings.Contains(err.Error(), `expected consumer entry (1 of 2 remaining), found "orders.new"`) {
		t.Fatalf("unexpected error: %v", err)
	}

	for name, body := range map[string]string{
		"missing config": `{"state":{}}`,
		"missing state":  `{"config":{"durable_name":"c1"}}`,
		"not json":       `nope`,
	} {
		t.Run(name, func(t *testing.T) {
			st := stateJSON(t, api.StreamState{FirstSeq: 1, LastSeq: 1, Consumers: 1})
			data := rawArchive(t, func(w *archive.Writer) {
				rawEntry(t, w, "state.json", 0, 0, 0, int64(len(st)), st)
				rawEntry(t, w, "consumers/c1", 0, 0, 0, int64(len(body)), []byte(body))
			})
			_, err := decodeAll(t, data)
			if err == nil || !strings.Contains(err.Error(), `consumer "c1"`) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDecoderSequenceInvariants(t *testing.T) {
	withState := func(t *testing.T, st api.StreamState, fn func(w *archive.Writer)) []byte {
		sj := stateJSON(t, st)
		return rawArchive(t, func(w *archive.Writer) {
			rawEntry(t, w, "state.json", 0, 0, 0, int64(len(sj)), sj)
			fn(w)
		})
	}

	t.Run("descending", func(t *testing.T) {
		data := withState(t, api.StreamState{FirstSeq: 1, LastSeq: 3}, func(w *archive.Writer) {
			rawEntry(t, w, "a", 1, 3, 0, 1, []byte("x"))
			rawEntry(t, w, "a", 2, 2, 0, 1, []byte("y"))
		})
		_, err := decodeAll(t, data)
		if err == nil || !strings.Contains(err.Error(), "message sequence 2 out of order after entry #2 message a seq 3") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		data := withState(t, api.StreamState{FirstSeq: 1, LastSeq: 3}, func(w *archive.Writer) {
			rawEntry(t, w, "a", 1, 3, 0, 1, []byte("x"))
			rawEntry(t, w, "a", 2, 3, 0, 1, []byte("y"))
		})
		_, err := decodeAll(t, data)
		if err == nil || !strings.Contains(err.Error(), "message sequence 3 out of order") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("below first_seq", func(t *testing.T) {
		data := withState(t, api.StreamState{FirstSeq: 10, LastSeq: 30}, func(w *archive.Writer) {
			rawEntry(t, w, "a", 1, 9, 0, 1, []byte("x"))
		})
		_, err := decodeAll(t, data)
		if err == nil || !strings.Contains(err.Error(), "message sequence 9 out of order") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("first_seq zero with messages", func(t *testing.T) {
		data := withState(t, api.StreamState{FirstSeq: 0, LastSeq: 0}, func(w *archive.Writer) {
			rawEntry(t, w, "a", 1, 1, 0, 1, []byte("x"))
		})
		_, err := decodeAll(t, data)
		if err == nil || !strings.Contains(err.Error(), "message sequence 1 out of order: state.json has first_seq 0") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("seq zero entry that is not the sentinel", func(t *testing.T) {
		data := withState(t, api.StreamState{FirstSeq: 1, LastSeq: 1}, func(w *archive.Writer) {
			rawEntry(t, w, "a", 1, 1, 0, 1, []byte("x"))
			rawEntry(t, w, "consumers/late", 7, 0, 0, 2, []byte("{}"))
		})
		_, err := decodeAll(t, data)
		if err == nil || !strings.Contains(err.Error(), `expected message sequence, found entry "consumers/late" with sequence 0 after entry #2 message a seq 1`) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("sentinel name ignored", func(t *testing.T) {
		data := withState(t, api.StreamState{FirstSeq: 1, LastSeq: 1}, func(w *archive.Writer) {
			rawEntry(t, w, "a", 1, 1, 0, 1, []byte("x"))
			rawEntry(t, w, "whatever", 0, 0, 0, 0, nil)
		})
		items, err := decodeAll(t, data)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := items[len(items)-1].(End); !ok {
			t.Fatalf("expected End, got %T", items[len(items)-1])
		}
	})

	t.Run("empty backup", func(t *testing.T) {
		data := withState(t, api.StreamState{FirstSeq: 6, LastSeq: 5}, func(w *archive.Writer) {
			rawEntry(t, w, "", 0, 0, 0, 0, nil)
		})
		items, err := decodeAll(t, data)
		if err != nil || len(items) != 2 {
			t.Fatalf("unexpected: %v %d", err, len(items))
		}
	})
}

func TestDecoderTruncation(t *testing.T) {
	st := api.StreamState{FirstSeq: 1, LastSeq: 2}

	t.Run("at entry boundary", func(t *testing.T) {
		data := encodeBackup(t, st, nil, []testMsg{{"a", 1, 1, "", "x"}}, false)
		_, err := decodeAll(t, data)
		if !errors.Is(err, io.EOF) {
			t.Fatalf("expected io.EOF underneath, got %v", err)
		}
		if !strings.Contains(err.Error(), "before the end-of-backup sentinel after entry #2 message a seq 1: EOF") {
			t.Fatalf("unexpected message: %v", err)
		}
	})

	t.Run("mid entry", func(t *testing.T) {
		sj := stateJSON(t, st)
		data := rawArchive(t, func(w *archive.Writer) {
			rawEntry(t, w, "state.json", 0, 0, 0, int64(len(sj)), sj)
			rawEntry(t, w, "a", 1, 1, 0, 100, []byte("only ten b"))
		})
		_, err := decodeAll(t, data)
		if !errors.Is(err, archive.ErrInvalidArchive) {
			t.Fatalf("expected ErrInvalidArchive underneath, got %v", err)
		}
		if !strings.Contains(err.Error(), "archive: invalid archive stream") {
			t.Fatalf("unexpected message: %v", err)
		}
	})

	t.Run("compressed stream cut", func(t *testing.T) {
		data := encodeBackup(t, st, nil, []testMsg{{"a", 1, 1, "", strings.Repeat("x", 50000)}, {"b", 2, 2, "", "y"}}, true)
		_, err := decodeAll(t, data[:len(data)/2])
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}

func writeBackupDir(t *testing.T, dir string, cfg api.StreamConfig, st api.StreamState, data []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, DataFile), data, 0o600); err != nil {
		t.Fatal(err)
	}
	sc, err := (&metaFile{Config: cfg, State: st}).marshal()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, MetaFile), sc, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyAndInfo(t *testing.T) {
	st := api.StreamState{Msgs: 0, Bytes: 0, FirstSeq: 2, LastSeq: 9, Consumers: 1}
	cfg := api.StreamConfig{Name: "ORDERS", Subjects: []string{"orders.>"}}
	msgs := []testMsg{
		{"orders.new", 2, 10, "", "first"},
		{"orders.paid", 5, 20, "NATS/1.0\r\nKV-Operation: DEL\r\n\r\n", ""},
		{"orders.shipped", 9, 30, "", "last"},
	}
	data := encodeBackup(t, st, map[string][]byte{"c1": consumerJSON(t, "c1")}, msgs, true)

	dir := filepath.Join(t.TempDir(), "b")
	writeBackupDir(t, dir, cfg, st, data)

	rep, err := Verify(dir)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if !rep.Complete || rep.Entries != 6 || rep.Consumers != 1 || rep.Messages != 3 || rep.FirstSeq != 2 || rep.LastSeq != 9 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	if rep.LastGood.Kind != "end" || rep.LastGood.Ordinal != 6 {
		t.Fatalf("unexpected last good: %+v", rep.LastGood)
	}

	info, err := Info(dir)
	if err != nil {
		t.Fatal(err)
	}
	var wantBytes uint64
	for _, m := range msgs {
		wantBytes += storedMsgSize(len(m.subject), int64(len(m.hdr)), int64(len(m.body)))
	}
	if info.Config.Name != "ORDERS" || info.Messages != 3 || info.Bytes != wantBytes || info.FirstSeq != 2 || info.LastSeq != 9 {
		t.Fatalf("unexpected info: %+v", info)
	}
	if info.FirstTime.UnixNano() != 10 || info.LastTime.UnixNano() != 30 {
		t.Fatalf("unexpected times: %+v", info)
	}
	if !info.DeclaredCountsAdvisory || !reflect.DeepEqual(info.Declared, st) || len(info.Consumers) != 1 || info.Consumers[0] != "c1" {
		t.Fatalf("unexpected declared: %+v", info)
	}

	truncated := encodeBackup(t, st, map[string][]byte{"c1": consumerJSON(t, "c1")}, msgs, false)
	tdir := filepath.Join(t.TempDir(), "t")
	writeBackupDir(t, tdir, cfg, st, truncated)
	rep, err = Verify(tdir)
	if err == nil {
		t.Fatal("expected verify to fail")
	}
	if rep == nil || rep.Complete || rep.Entries != 5 || rep.Messages != 3 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	if !strings.Contains(err.Error(), "last good entry: entry #5 message orders.shipped seq 9") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := Info(tdir); err == nil {
		t.Fatal("expected info to fail")
	}

	if err := os.Remove(filepath.Join(dir, MetaFile)); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing meta file error, got %v", err)
	}

	writeBackupDir(t, dir, api.StreamConfig{Name: "bad.name"}, st, data)
	if _, err := Verify(dir); err == nil || !strings.Contains(err.Error(), `invalid stream name "bad.name"`) {
		t.Fatalf("expected an invalid name error, got %v", err)
	}

	writeBackupDir(t, dir, api.StreamConfig{Name: "MEM", Storage: api.MemoryStorage}, st, data)
	if _, err := Verify(dir); !errors.Is(err, jsm.ErrMemoryStreamNotSupported) {
		t.Fatalf("expected a memory storage error, got %v", err)
	}
}

func TestVerifyRejectsV1Layout(t *testing.T) {
	dir := t.TempDir()
	writeBackupDir(t, dir, api.StreamConfig{Name: "OLD"}, api.StreamState{}, framed(t, make([]byte, 2048)))

	_, err := Verify(dir)
	if !errors.Is(err, ErrNotV2Backup) {
		t.Fatalf("expected ErrNotV2Backup, got %v", err)
	}
}
