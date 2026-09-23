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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

func captureConfig() api.StreamConfig {
	return api.StreamConfig{Name: "CAP", Subjects: []string{"orders.>"}, Storage: api.FileStorage, Replicas: 1}
}

func captureDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "cap")
}

func siblings(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(dir))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestRecorderRoundTrip(t *testing.T) {
	dir := captureDir(t)
	started := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	rec, err := NewRecorder(dir, captureConfig(), SourceInfo{Subjects: []string{"orders.>"}, Started: started})
	if err != nil {
		t.Fatal(err)
	}

	ackTs := time.Date(2026, 9, 22, 10, 1, 0, 123, time.UTC)
	directTs := time.Date(2026, 9, 22, 10, 2, 0, 0, time.UTC)
	plain := nats.NewMsg("orders.new")
	plain.Data = []byte("order 1")
	withHdr := nats.NewMsg("orders.paid")
	withHdr.Data = []byte("paid 1")
	withHdr.Header.Add("X-Batch", "8")
	withHdr.Header.Add("X-Batch", "7")
	withHdr.Header.Set("kv-operation", "PUT")
	withHdr.Reply = fmt.Sprintf("$JS.ACK.ORDERS.cons.1.5.1.%d.0", ackTs.UnixNano())
	direct := nats.NewMsg("orders.shipped")
	direct.Header.Set("Nats-Stream", "ORDERS")
	direct.Header.Set("Nats-Subject", "orders.shipped")
	direct.Header.Set("Nats-Sequence", "9")
	direct.Header.Set("Nats-Time-Stamp", directTs.Format(time.RFC3339Nano))
	direct.Header.Set("Nats-Num-Pending", "0")
	direct.Header.Set("Nats-Last-Sequence", "8")
	direct.Header.Set("X-A", "1")
	inbox := nats.NewMsg("orders.new")
	inbox.Reply = "_INBOX.abc.def"
	inbox.Data = []byte("order 2")

	before := time.Now()
	for _, m := range []*nats.Msg{plain, withHdr} {
		if err := rec.Write(m); err != nil {
			t.Fatal(err)
		}
	}
	if err := rec.WriteDirect(direct); err != nil {
		t.Fatal(err)
	}
	if err := rec.Write(inbox); err != nil {
		t.Fatal(err)
	}
	res, err := rec.Close(3)
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now()

	if res.Dir != dir || res.Messages != 4 {
		t.Fatalf("unexpected result %+v", res)
	}
	if !reflect.DeepEqual(siblings(t, dir), []string{"cap"}) {
		t.Fatalf("staging left behind: %v", siblings(t, dir))
	}
	if _, err := Verify(dir); err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	items := readItems(t, dir)
	st := items[0].(*State)
	if st.Ts != started.UnixNano() || !reflect.DeepEqual(st.State, api.StreamState{FirstSeq: 1, LastSeq: 0}) {
		t.Fatalf("unexpected state entry %+v", st)
	}
	msgs := messagesOf(items)
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	want := []struct {
		subject string
		hdr     string
		body    string
	}{
		{"orders.new", "", "order 1"},
		{"orders.paid", "NATS/1.0\r\nX-Batch: 8\r\nX-Batch: 7\r\nkv-operation: PUT\r\n\r\n", "paid 1"},
		{"orders.shipped", "NATS/1.0\r\nX-A: 1\r\n\r\n", ""},
		{"orders.new", "", "order 2"},
	}
	var bytes uint64
	for i, m := range msgs {
		body, _ := io.ReadAll(m.Body)
		if m.Seq != uint64(i+1) || m.Subject != want[i].subject || string(body) != want[i].hdr+want[i].body || m.HdrSize != int64(len(want[i].hdr)) {
			t.Fatalf("message %d mismatch: %+v %q", i, m, body)
		}
		bytes += storedMsgSize(len(m.Subject), m.HdrSize, m.PayloadSize)
	}
	if msgs[1].Ts != ackTs.UnixNano() {
		t.Fatalf("ack timestamp not used: %d", msgs[1].Ts)
	}
	if msgs[2].Ts != directTs.UnixNano() {
		t.Fatalf("direct get timestamp not used: %d", msgs[2].Ts)
	}
	for _, i := range []int{0, 3} {
		if ts := time.Unix(0, msgs[i].Ts); ts.Before(before) || ts.After(after) {
			t.Fatalf("message %d timestamp %s outside the test window", i, ts)
		}
	}
	if res.Bytes != bytes {
		t.Fatalf("result bytes %d, archive bytes %d", res.Bytes, bytes)
	}

	mf := loadMetaFile(t, dir)
	if !reflect.DeepEqual(mf.Config, captureConfig()) {
		t.Fatalf("unexpected config %+v", mf.Config)
	}
	if !reflect.DeepEqual(mf.State, api.StreamState{Msgs: 4, Bytes: bytes, FirstSeq: 1, LastSeq: 4}) {
		t.Fatalf("unexpected meta state %+v", mf.State)
	}
	if mf.Edit != nil || mf.Source == nil {
		t.Fatalf("unexpected provenance blocks %+v", mf)
	}
	src := mf.Source
	if !reflect.DeepEqual(src.Subjects, []string{"orders.>"}) || src.Stream != "" || !src.Started.Equal(started) || src.Dropped != 3 {
		t.Fatalf("unexpected source block %+v", src)
	}
	if src.Ended.Before(before) || src.Ended.After(after) {
		t.Fatalf("ended %s outside the test window", src.Ended)
	}

	info, err := Info(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Messages != 4 || info.Bytes != bytes || info.FirstSeq != 1 || info.LastSeq != 4 || !info.DeclaredCountsAdvisory {
		t.Fatalf("unexpected info %+v", info)
	}
}

func TestRecorderEmpty(t *testing.T) {
	dir := captureDir(t)
	rec, err := NewRecorder(dir, captureConfig(), SourceInfo{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := rec.Close(0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Messages != 0 || res.Bytes != 0 {
		t.Fatalf("unexpected result %+v", res)
	}
	if _, err := Verify(dir); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	mf := loadMetaFile(t, dir)
	if !reflect.DeepEqual(mf.State, api.StreamState{FirstSeq: 1, LastSeq: 0}) {
		t.Fatalf("unexpected meta state %+v", mf.State)
	}
	if mf.Source.Started.IsZero() || mf.Source.Ended.Before(mf.Source.Started) {
		t.Fatalf("unexpected source times %+v", mf.Source)
	}
	if len(messagesOf(readItems(t, dir))) != 0 {
		t.Fatal("expected no messages")
	}
}

func TestRecorderDiscard(t *testing.T) {
	dir := captureDir(t)
	rec, err := NewRecorder(dir, captureConfig(), SourceInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if len(siblings(t, dir)) != 1 {
		t.Fatalf("expected one staging directory, got %v", siblings(t, dir))
	}
	if err := rec.Write(nats.NewMsg("orders.new")); err != nil {
		t.Fatal(err)
	}
	rec.Discard()
	if len(siblings(t, dir)) != 0 {
		t.Fatalf("discard left %v", siblings(t, dir))
	}
}

func TestRecorderConfig(t *testing.T) {
	dir := captureDir(t)
	cfg := captureConfig()
	cfg.Name = "bad.name"
	if _, err := NewRecorder(dir, cfg, SourceInfo{}); err == nil {
		t.Fatal("invalid name accepted")
	}
	if len(siblings(t, dir)) != 0 {
		t.Fatalf("refused recorder left %v", siblings(t, dir))
	}

	cfg = captureConfig()
	cfg.Storage = api.MemoryStorage
	cfg.FirstSeq = 50
	rec, err := NewRecorder(dir, cfg, SourceInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rec.Close(0); err != nil {
		t.Fatal(err)
	}
	mf := loadMetaFile(t, dir)
	if mf.Config.Storage != api.FileStorage || mf.Config.FirstSeq != 0 {
		t.Fatalf("storage or first sequence not normalised: %+v", mf.Config)
	}

	if _, err := NewRecorder(dir, captureConfig(), SourceInfo{}); err == nil {
		t.Fatal("existing backup directory accepted")
	}
}
