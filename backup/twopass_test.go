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
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nats-io/jsm.go/api"
)

func TestSubjectStateLastPerSubject(t *testing.T) {
	s := newSubjectState(&editOptions{lastPerSubject: 2})
	s.Observe("a", 1, 10, false)
	s.Observe("b", 2, 20, false)
	s.Observe("a", 3, 30, true)
	s.Observe("a", 4, 40, false)
	s.Observe("c", 5, 50, false)

	res := s.Resolve()
	if res.msgs != 4 || res.bytes != 140 || res.firstSeq != 2 {
		t.Fatalf("unexpected resolution %+v", res)
	}
	for seq, want := range map[uint64]bool{1: false, 3: true, 4: true} {
		if s.Keeps("a", seq) != want {
			t.Fatalf("Keeps(a, %d) = %v", seq, !want)
		}
	}
	if !s.Keeps("b", 2) || !s.Keeps("c", 5) || s.Keeps("d", 5) || s.Keeps("c", 6) {
		t.Fatal("unexpected Keeps answers")
	}
	keys, size := s.Stats()
	if keys != 3 || size != 3+4*16 {
		t.Fatalf("unexpected stats %d %d", keys, size)
	}
}

func TestSubjectStateKVCompact(t *testing.T) {
	s := newSubjectState(&editOptions{kvCompact: true})
	s.Observe("a", 1, 10, false)
	s.Observe("a", 2, 20, false)
	s.Observe("b", 3, 30, false)
	s.Observe("b", 4, 40, true)
	s.Observe("c", 5, 50, true)
	s.Observe("c", 6, 60, false)

	res := s.Resolve()
	if res.msgs != 2 || res.bytes != 80 || res.firstSeq != 2 {
		t.Fatalf("unexpected resolution %+v", res)
	}
	for _, tc := range []struct {
		subject string
		seq     uint64
		want    bool
	}{{"a", 1, false}, {"a", 2, true}, {"b", 3, false}, {"b", 4, false}, {"c", 5, false}, {"c", 6, true}, {"d", 6, false}} {
		if s.Keeps(tc.subject, tc.seq) != tc.want {
			t.Fatalf("Keeps(%s, %d) = %v", tc.subject, tc.seq, !tc.want)
		}
	}
	keys, size := s.Stats()
	if keys != 3 || size != 3+3*24 {
		t.Fatalf("unexpected stats %d %d", keys, size)
	}
}

func TestEditLastPerSubject(t *testing.T) {
	sizeOf := func(seqs ...uint64) uint64 {
		var total uint64
		for _, m := range fixtureMsgs {
			for _, seq := range seqs {
				if m.seq == seq {
					total += storedMsgSize(len(m.subject), int64(len(m.hdr)), int64(len(m.body)))
				}
			}
		}
		return total
	}
	cases := map[string]struct {
		opts    []EditOption
		seqs    []uint64
		dropped DropCounts
		state   api.StreamState
	}{
		"last 1": {[]EditOption{LastPerSubject(1)}, []uint64{6, 9, 10, 12}, DropCounts{LastPerSubject: 3},
			api.StreamState{Msgs: 4, Bytes: sizeOf(6, 9, 10, 12), FirstSeq: 2, LastSeq: 14, Consumers: 2}},
		"last 2": {[]EditOption{LastPerSubject(2)}, []uint64{3, 5, 6, 9, 10, 12}, DropCounts{LastPerSubject: 1},
			api.StreamState{Msgs: 6, Bytes: sizeOf(3, 5, 6, 9, 10, 12), FirstSeq: 2, LastSeq: 14, Consumers: 2}},
		"last 1 with filters": {[]EditOption{LastPerSubject(1), Subjects("orders.>"), NoHeader("Nats-TTL")}, []uint64{5, 6, 12}, DropCounts{Subject: 1, Header: 1, LastPerSubject: 2},
			api.StreamState{Msgs: 3, Bytes: sizeOf(5, 6, 12), FirstSeq: 2, LastSeq: 14, Consumers: 2}},
		"last 1 renumber": {[]EditOption{LastPerSubject(1), Renumber()}, []uint64{1, 2, 3, 4}, DropCounts{LastPerSubject: 3},
			api.StreamState{Msgs: 4, Bytes: sizeOf(6, 9, 10, 12), FirstSeq: 1, LastSeq: 4}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := fixtureDir(t, fixtureConfig(), fixtureMsgs)
			dst, res := edit(t, src, tc.opts...)
			if _, err := Verify(dst); err != nil {
				t.Fatalf("output does not verify: %v", err)
			}
			items := readItems(t, dst)
			if got := seqsOf(messagesOf(items)); !reflect.DeepEqual(got, tc.seqs) {
				t.Fatalf("kept %v, want %v", got, tc.seqs)
			}
			if st := items[0].(*State).State; !reflect.DeepEqual(st, tc.state) {
				t.Fatalf("archive state %+v, want exact %+v", st, tc.state)
			}
			if res.Report.Dropped != tc.dropped || res.Report.Kept != uint64(len(tc.seqs)) {
				t.Fatalf("report %+v, want dropped %+v", res.Report, tc.dropped)
			}
			if res.Report.SubjectStateKeys == 0 || res.Report.SubjectStateBytes == 0 {
				t.Fatalf("subject state not reported: %+v", res.Report)
			}
			if res.State.Msgs != tc.state.Msgs || res.State.Bytes != tc.state.Bytes {
				t.Fatalf("meta file state %+v", res.State)
			}
		})
	}
}

var kvMsgs = []testMsg{
	{"$KV.orders.a", 1, 100, "", "a1"},
	{"$KV.orders.b", 2, 200, "", "b1"},
	{"$KV.orders.a", 3, 300, "", "a2"},
	{"$KV.orders.b", 4, 400, "NATS/1.0\r\nKV-Operation: DEL\r\n\r\n", ""},
	{"$KV.orders.c", 5, 500, "", "c1"},
	{"$KV.orders.c", 6, 600, "NATS/1.0\r\nNats-Marker-Reason: MaxAge\r\n\r\n", ""},
	{"$KV.orders.d", 7, 700, "", "d1"},
	{"$KV.orders.d", 8, 800, "NATS/1.0\r\nKV-Operation: FOO\r\nNats-Marker-Reason: Remove\r\n\r\n", "d2"},
	{"$KV.orders.e", 9, 900, "NATS/1.0\r\nKV-Operation: PURGE\r\n\r\n", ""},
}

func kvConfig() api.StreamConfig {
	return api.StreamConfig{Name: "KV_orders", Subjects: []string{"$KV.orders.>"}, Storage: api.FileStorage, MaxMsgsPer: 5}
}

func kvDir(t *testing.T, cfg api.StreamConfig) string {
	t.Helper()
	return fixtureDirWithState(t, cfg, api.StreamState{Msgs: 9, Bytes: 999, FirstSeq: 1, LastSeq: 9, Consumers: 2}, kvMsgs)
}

func TestEditKVCompact(t *testing.T) {
	cases := map[string]struct {
		opts    []EditOption
		seqs    []uint64
		dropped DropCounts
		tombs   uint64
		keys    uint64
	}{
		"compact":               {[]EditOption{KVCompact()}, []uint64{3, 8}, DropCounts{KVCompact: 7}, 0, 5},
		"compact renumber":      {[]EditOption{KVCompact(), Renumber()}, []uint64{1, 2}, DropCounts{KVCompact: 7}, 0, 5},
		"content filter first":  {[]EditOption{KVCompact(), NoHeader("KV-Operation")}, []uint64{2, 3, 7}, DropCounts{Header: 3, KVCompact: 3}, 2, 4},
		"sequence filter first": {[]EditOption{KVCompact(), LastSeq(5)}, []uint64{3, 5}, DropCounts{Sequence: 4, KVCompact: 3}, 0, 3},
		"subject filter first":  {[]EditOption{KVCompact(), Subjects("$KV.orders.b", "$KV.orders.d")}, []uint64{8}, DropCounts{Subject: 5, KVCompact: 3}, 0, 2},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := kvDir(t, kvConfig())
			dst, res := edit(t, src, tc.opts...)
			if _, err := Verify(dst); err != nil {
				t.Fatalf("output does not verify: %v", err)
			}
			if got := seqsOf(messagesOf(readItems(t, dst))); !reflect.DeepEqual(got, tc.seqs) {
				t.Fatalf("kept %v, want %v", got, tc.seqs)
			}
			if res.Report.Dropped != tc.dropped || res.Report.TombstonesRemovedByContentFilters != tc.tombs || res.Report.SubjectStateKeys != tc.keys {
				t.Fatalf("report %+v", res.Report)
			}
		})
	}
}

func TestEditKVCompactGate(t *testing.T) {
	cfg := kvConfig()
	cfg.Subjects = []string{"$KV.orders.>", "extra"}
	src := kvDir(t, cfg)
	_, err := Edit(context.Background(), src, filepath.Join(t.TempDir(), "dst"), KVCompact())
	if err == nil || !strings.Contains(err.Error(), "KVCompact requires a KV bucket backup") {
		t.Fatalf("expected the KV gate to refuse, got %v", err)
	}

	cfg = kvConfig()
	cfg.Name = "KV_other"
	src = kvDir(t, cfg)
	_, err = Edit(context.Background(), src, filepath.Join(t.TempDir(), "dst"), KVCompact())
	if err == nil || !strings.Contains(err.Error(), "KVCompact requires a KV bucket backup") {
		t.Fatalf("expected the KV gate to refuse a mismatched bucket, got %v", err)
	}
}

func TestEditTwoPassDryRun(t *testing.T) {
	src := kvDir(t, kvConfig())
	dst := filepath.Join(t.TempDir(), "dry")
	dry, err := Edit(context.Background(), src, dst, KVCompact(), DryRun())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); err == nil {
		t.Fatal("dry run wrote the target")
	}
	_, real := edit(t, src, KVCompact())
	if !reflect.DeepEqual(dry, real) {
		t.Fatalf("dry run result differs:\n%+v\n%+v", dry, real)
	}
}

func TestEditTwoPassSurvivesRenameSwap(t *testing.T) {
	src := kvDir(t, kvConfig())
	original, _ := os.ReadFile(filepath.Join(src, DataFile))

	swap := func() {
		other := encodeBackup(t, fixtureState(), nil, []testMsg{{"$KV.orders.z", 1, 1, "", "z"}}, true)
		tmp := filepath.Join(src, "replacement")
		if err := os.WriteFile(tmp, other, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(tmp, filepath.Join(src, DataFile)); err != nil {
			t.Fatal(err)
		}
	}

	dst, res := edit(t, src, KVCompact(), betweenPasses(swap))
	if got := seqsOf(messagesOf(readItems(t, dst))); !reflect.DeepEqual(got, []uint64{3, 8}) {
		t.Fatalf("second pass did not read the held file: %v", got)
	}
	sc := loadMetaFile(t, dst)
	if res.Report.Kept != 2 || !strings.HasPrefix(sc.Edit.SourceDigest, "sha256:") {
		t.Fatalf("unexpected result %+v", res.Report)
	}
	if now, _ := os.ReadFile(filepath.Join(src, DataFile)); bytes.Equal(now, original) {
		t.Fatal("the swap did not happen")
	}
}

func TestEditTwoPassDetectsInPlaceChange(t *testing.T) {
	src := kvDir(t, kvConfig())
	parent := t.TempDir()

	appendJunk := func() {
		f, err := os.OpenFile(filepath.Join(src, DataFile), os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte("trailing junk"))
		f.Close()
	}

	_, err := Edit(context.Background(), src, filepath.Join(parent, "dst"), KVCompact(), betweenPasses(appendJunk))
	if err == nil || !strings.Contains(err.Error(), "changed during the edit") {
		t.Fatalf("expected the in-place change to be detected, got %v", err)
	}
	if entries, _ := os.ReadDir(parent); len(entries) != 0 {
		t.Fatalf("failed edit left files behind: %v", entries)
	}
}
