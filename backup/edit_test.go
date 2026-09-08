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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/jsm.go/api"
)

var fixtureMsgs = []testMsg{
	{"orders.new", 2, 1_000, "", "order 1"},
	{"orders.paid", 3, 2_000, "NATS/1.0\r\nX-Batch: 7\r\nX-Batch: 8\r\n\r\n", "paid 1"},
	{"orders.new", 5, 3_000, "NATS/1.0\r\nKV-Operation: DEL\r\n\r\n", ""},
	{"orders.shipped", 6, 4_000, "NATS/1.0\r\nNats-Marker-Reason: MaxAge\r\n\r\n", ""},
	{"orders.new", 9, 5_000, "NATS/1.0\r\nNats-TTL: 1s\r\nX-Batch: 7\r\n\r\n", "order 2 urgent"},
	{"audit.log", 10, 6_000, "", "audit entry"},
	{"orders.paid", 12, 7_000, "NATS/1.0\r\nKV-Operation: PUT\r\nNats-Marker-Reason: Remove\r\n\r\n", "paid urgent"},
}

func fixtureState() api.StreamState {
	return api.StreamState{Msgs: 7, Bytes: 12345, FirstSeq: 2, LastSeq: 14, Consumers: 2, NumDeleted: 3, Deleted: []uint64{4, 7, 8}}
}

func fixtureConfig() api.StreamConfig {
	return api.StreamConfig{Name: "ORDERS", Subjects: []string{"orders.>", "audit.*"}, Storage: api.FileStorage, Retention: api.LimitsPolicy, FirstSeq: 2}
}

func fixtureDir(t *testing.T, cfg api.StreamConfig, msgs []testMsg) string {
	t.Helper()
	return fixtureDirWithState(t, cfg, fixtureState(), msgs)
}

func fixtureDirWithState(t *testing.T, cfg api.StreamConfig, st api.StreamState, msgs []testMsg) string {
	t.Helper()
	consumers := map[string][]byte{"c1": consumerJSON(t, "c1"), "c2": consumerJSON(t, "c2")}
	dir := filepath.Join(t.TempDir(), "src")
	writeBackupDir(t, dir, cfg, st, encodeBackup(t, st, consumers, msgs, true))
	return dir
}

func readItems(t *testing.T, dir string) []Item {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, DataFile))
	if err != nil {
		t.Fatal(err)
	}
	items, err := decodeAll(t, data)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	return items
}

func loadMetaFile(t *testing.T, dir string) *metaFile {
	t.Helper()
	sc, err := readMetaFile(filepath.Join(dir, MetaFile))
	if err != nil {
		t.Fatal(err)
	}
	return sc
}

func messagesOf(items []Item) []*Message {
	var out []*Message
	for _, it := range items {
		if m, ok := it.(*Message); ok {
			out = append(out, m)
		}
	}
	return out
}

func consumersOf(items []Item) []*Consumer {
	var out []*Consumer
	for _, it := range items {
		if c, ok := it.(*Consumer); ok {
			out = append(out, c)
		}
	}
	return out
}

func seqsOf(msgs []*Message) []uint64 {
	out := make([]uint64, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.Seq)
	}
	return out
}

func sameMessage(a, b *Message) bool {
	ab, _ := io.ReadAll(a.Body)
	bb, _ := io.ReadAll(b.Body)
	a.Body, b.Body = bytes.NewReader(ab), bytes.NewReader(bb)
	return a.Subject == b.Subject && a.Seq == b.Seq && a.Ts == b.Ts && a.HdrSize == b.HdrSize && a.PayloadSize == b.PayloadSize && bytes.Equal(ab, bb)
}

func edit(t *testing.T, src string, opts ...EditOption) (string, *Result) {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "dst")
	res, err := Edit(context.Background(), src, dst, opts...)
	if err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	return dst, res
}

func TestEditIdentity(t *testing.T) {
	src := fixtureDir(t, fixtureConfig(), fixtureMsgs)
	dst, res := edit(t, src)

	if _, err := Verify(dst); err != nil {
		t.Fatalf("output does not verify: %v", err)
	}

	in, out := readItems(t, src), readItems(t, dst)
	if len(in) != len(out) {
		t.Fatalf("item count differs: %d vs %d", len(in), len(out))
	}

	inState, outState := in[0].(*State), out[0].(*State)
	if outState.Ts != inState.Ts {
		t.Fatalf("state timestamp not copied")
	}
	want := api.StreamState{FirstSeq: 2, LastSeq: 14, Consumers: 2}
	if !reflect.DeepEqual(outState.State, want) {
		t.Fatalf("archive state %+v, want %+v", outState.State, want)
	}

	for i := range in[1:] {
		a, b := in[1+i], out[1+i]
		switch x := a.(type) {
		case *Consumer:
			y := b.(*Consumer)
			if x.Name != y.Name || x.Ts != y.Ts || !bytes.Equal(x.Data, y.Data) {
				t.Fatalf("consumer %d differs", i)
			}
		case *Message:
			if !sameMessage(x, b.(*Message)) {
				t.Fatalf("message %d differs: %+v vs %+v", i, x, b)
			}
		case End:
			if _, ok := b.(End); !ok {
				t.Fatalf("expected End, got %T", b)
			}
		}
	}

	srcBytes, _ := os.ReadFile(filepath.Join(src, DataFile))
	sum := sha256.Sum256(srcBytes)
	sc := loadMetaFile(t, dst)
	if sc.Edit == nil || sc.Edit.SourceDigest != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatalf("unexpected edit block: %+v", sc.Edit)
	}
	if sc.Edit.Version != "" || len(sc.Edit.Options) != 0 || sc.Edit.Obfuscated {
		t.Fatalf("unexpected edit block: %+v", sc.Edit)
	}
	rawMeta, _ := os.ReadFile(filepath.Join(dst, MetaFile))
	if strings.Contains(string(rawMeta), "version") {
		t.Fatal("edit block must omit the version when none was given")
	}
	if !reflect.DeepEqual(sc.Config, fixtureConfig()) {
		t.Fatalf("config not copied verbatim: %+v", sc.Config)
	}
	var wantBytes uint64
	for _, m := range fixtureMsgs {
		wantBytes += storedMsgSize(len(m.subject), int64(len(m.hdr)), int64(len(m.body)))
	}
	wantMeta := api.StreamState{Msgs: 7, Bytes: wantBytes, FirstSeq: 2, LastSeq: 14, Consumers: 2}
	if !reflect.DeepEqual(sc.State, wantMeta) || !reflect.DeepEqual(res.State, wantMeta) {
		t.Fatalf("meta file state %+v, result %+v, want %+v", sc.State, res.State, wantMeta)
	}

	rep := res.Report
	if rep.SourceMessages != 7 || rep.Kept != 7 || rep.Dropped.Total() != 0 || rep.ConsumersKept != 2 || rep.ConsumersDropped != 0 || rep.TombstonesRemovedByContentFilters != 0 {
		t.Fatalf("unexpected report: %+v", rep)
	}
}

func TestEditDeterministic(t *testing.T) {
	src := fixtureDir(t, fixtureConfig(), fixtureMsgs)
	opts := []EditOption{Subjects("orders.>"), HeaderValue("X-Batch", "7"), LastSeq(12)}

	a, _ := edit(t, src, opts...)
	b, _ := edit(t, src, LastSeq(12), HeaderValue("X-Batch", "7"), Subjects("orders.>"))

	for _, name := range []string{DataFile, MetaFile} {
		x, _ := os.ReadFile(filepath.Join(a, name))
		y, _ := os.ReadFile(filepath.Join(b, name))
		if !bytes.Equal(x, y) {
			t.Fatalf("%s differs between two identical edits", name)
		}
	}

	sc := loadMetaFile(t, a)
	want := []string{"subject=orders.>", "last-seq=12", "header=X-Batch:7"}
	if !reflect.DeepEqual(sc.Edit.Options, want) {
		t.Fatalf("canonical options %v, want %v", sc.Edit.Options, want)
	}
}

func TestEditDryRun(t *testing.T) {
	src := fixtureDir(t, fixtureConfig(), fixtureMsgs)
	dst := filepath.Join(t.TempDir(), "dry")

	dry, err := Edit(context.Background(), src, dst, ExcludeSubjects("audit.*"), Renumber(), DryRun())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry run created the target: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(dst)); len(entries) != 0 {
		t.Fatalf("dry run left files behind: %v", entries)
	}

	_, real := edit(t, src, ExcludeSubjects("audit.*"), Renumber())
	if !reflect.DeepEqual(dry, real) {
		t.Fatalf("dry run result differs:\n%+v\n%+v", dry, real)
	}
}

func TestEditTargetRules(t *testing.T) {
	src := fixtureDir(t, fixtureConfig(), fixtureMsgs)

	full := filepath.Join(t.TempDir(), "full")
	os.MkdirAll(full, 0o700)
	os.WriteFile(filepath.Join(full, "x"), []byte("x"), 0o600)
	if _, err := Edit(context.Background(), src, full); err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("expected a non-empty target error, got %v", err)
	}

	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, []byte("x"), 0o600)
	if _, err := Edit(context.Background(), src, file); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected a not-a-directory error, got %v", err)
	}

	empty := filepath.Join(t.TempDir(), "empty")
	os.MkdirAll(empty, 0o700)
	if _, err := Edit(context.Background(), src, empty); err != nil {
		t.Fatalf("empty existing dir should be accepted: %v", err)
	}
	if _, err := Verify(empty); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(t.TempDir(), "a", "b", "c")
	if _, err := Edit(context.Background(), src, nested); err != nil {
		t.Fatalf("missing parents should be created: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(nested)); len(entries) != 1 {
		t.Fatalf("staging directory left behind: %v", entries)
	}
}

func TestEditFailureLeavesNothing(t *testing.T) {
	src := fixtureDir(t, fixtureConfig(), fixtureMsgs)
	data, _ := os.ReadFile(filepath.Join(src, DataFile))
	if err := os.WriteFile(filepath.Join(src, DataFile), data[:len(data)-40], 0o600); err != nil {
		t.Fatal(err)
	}

	parent := t.TempDir()
	dst := filepath.Join(parent, "dst")
	_, err := Edit(context.Background(), src, dst)
	if err == nil {
		t.Fatal("expected the truncated source to fail")
	}
	entries, _ := os.ReadDir(parent)
	if len(entries) != 0 {
		t.Fatalf("failed edit left files behind: %v", entries)
	}
}

func TestEditContextCancelled(t *testing.T) {
	src := fixtureDir(t, fixtureConfig(), fixtureMsgs)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Edit(ctx, src, filepath.Join(t.TempDir(), "dst"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestEditOptionValidation(t *testing.T) {
	src := fixtureDir(t, fixtureConfig(), fixtureMsgs)
	now := time.Now()
	cases := map[string]struct {
		opts []EditOption
		want string
	}{
		"bad subject":         {[]EditOption{Subjects("a..b")}, `invalid subject filter "a..b"`},
		"bad exclude":         {[]EditOption{ExcludeSubjects("")}, "invalid subject filter"},
		"after not before":    {[]EditOption{After(now), Before(now)}, "After must be earlier than Before"},
		"after out of range":  {[]EditOption{After(time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC))}, "outside the supported time range"},
		"before out of range": {[]EditOption{Before(time.Date(1500, 1, 1, 0, 0, 0, 0, time.UTC))}, "outside the supported time range"},
		"seq range":           {[]EditOption{FirstSeq(10), LastSeq(5)}, "FirstSeq 10 is above LastSeq 5"},
		"bad header":          {[]EditOption{HeaderPresent("a:b")}, `invalid header name "a:b"`},
		"bad header value":    {[]EditOption{HeaderValue("a", "x\r\n")}, "invalid header value"},
		"nil regex":           {[]EditOption{PayloadMatch(nil)}, "payload expression is nil"},
		"last per subject":    {[]EditOption{LastPerSubject(-1)}, "LastPerSubject requires at least 1"},
		"last per subject 0":  {[]EditOption{LastPerSubject(0)}, "LastPerSubject requires at least 1"},
		"compact and last":    {[]EditOption{KVCompact(), LastPerSubject(2)}, "mutually exclusive"},
		"key without obf":     {[]EditOption{ObfuscationKeyFile("x")}, "ObfuscationKeyFile requires Obfuscate"},
		"kv compact on plain": {[]EditOption{KVCompact()}, "KVCompact requires a KV bucket backup"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Edit(context.Background(), src, filepath.Join(t.TempDir(), "dst"), tc.opts...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}

	if _, err := Edit(context.Background(), t.TempDir(), filepath.Join(t.TempDir(), "dst")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected a missing source error, got %v", err)
	}
}

func TestEditFilters(t *testing.T) {
	cases := map[string]struct {
		opts    []EditOption
		seqs    []uint64
		dropped DropCounts
		tombs   uint64
	}{
		"subjects or":          {[]EditOption{Subjects("orders.new", "audit.>")}, []uint64{2, 5, 9, 10}, DropCounts{Subject: 3}, 0},
		"subject wildcard":     {[]EditOption{Subjects("orders.*")}, []uint64{2, 3, 5, 6, 9, 12}, DropCounts{Subject: 1}, 0},
		"exclude veto":         {[]EditOption{Subjects("orders.>"), ExcludeSubjects("orders.paid")}, []uint64{2, 5, 6, 9}, DropCounts{Subject: 3}, 0},
		"exclude only":         {[]EditOption{ExcludeSubjects("orders.new", "orders.paid")}, []uint64{6, 10}, DropCounts{Subject: 5}, 0},
		"after inclusive":      {[]EditOption{After(time.Unix(0, 3_000))}, []uint64{5, 6, 9, 10, 12}, DropCounts{Time: 2}, 0},
		"before exclusive":     {[]EditOption{Before(time.Unix(0, 3_000))}, []uint64{2, 3}, DropCounts{Time: 5}, 0},
		"window":               {[]EditOption{After(time.Unix(0, 2_000)), Before(time.Unix(0, 6_000))}, []uint64{3, 5, 6, 9}, DropCounts{Time: 3}, 0},
		"sequence range":       {[]EditOption{FirstSeq(5), LastSeq(10)}, []uint64{5, 6, 9, 10}, DropCounts{Sequence: 3}, 0},
		"first seq only":       {[]EditOption{FirstSeq(10)}, []uint64{10, 12}, DropCounts{Sequence: 5}, 0},
		"header present":       {[]EditOption{HeaderPresent("x-batch")}, []uint64{3, 9}, DropCounts{Header: 5}, 2},
		"header value any":     {[]EditOption{HeaderValue("X-BATCH", "8")}, []uint64{3}, DropCounts{Header: 6}, 2},
		"header positives or":  {[]EditOption{HeaderPresent("Nats-TTL"), HeaderValue("KV-Operation", "DEL")}, []uint64{5, 9}, DropCounts{Header: 5}, 1},
		"no header veto":       {[]EditOption{NoHeader("kv-operation"), NoHeader("nats-marker-reason")}, []uint64{2, 3, 9, 10}, DropCounts{Header: 3}, 2},
		"header and no header": {[]EditOption{HeaderPresent("X-Batch"), NoHeader("Nats-TTL")}, []uint64{3}, DropCounts{Header: 6}, 2},
		"payload match":        {[]EditOption{PayloadMatch(regexp.MustCompile("urgent"))}, []uint64{9, 12}, DropCounts{Payload: 5}, 2},
		"payload or":           {[]EditOption{PayloadMatch(regexp.MustCompile("^order 1$")), PayloadMatch(regexp.MustCompile("audit"))}, []uint64{2, 10}, DropCounts{Payload: 5}, 2},
		"payload exclude":      {[]EditOption{ExcludePayloadMatch(regexp.MustCompile("paid"))}, []uint64{2, 5, 6, 9, 10}, DropCounts{Payload: 2}, 0},
		"payload empty body":   {[]EditOption{PayloadMatch(regexp.MustCompile("^$"))}, []uint64{5, 6}, DropCounts{Payload: 5}, 0},
		"and across kinds":     {[]EditOption{Subjects("orders.>"), FirstSeq(3), Before(time.Unix(0, 7_000)), HeaderPresent("X-Batch"), PayloadMatch(regexp.MustCompile("urgent"))}, []uint64{9}, DropCounts{Sequence: 1, Time: 1, Subject: 1, Header: 2, Payload: 1}, 2},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := fixtureDir(t, fixtureConfig(), fixtureMsgs)
			dst, res := edit(t, src, tc.opts...)
			if _, err := Verify(dst); err != nil {
				t.Fatalf("output does not verify: %v", err)
			}
			got := seqsOf(messagesOf(readItems(t, dst)))
			if !reflect.DeepEqual(got, tc.seqs) {
				t.Fatalf("kept %v, want %v", got, tc.seqs)
			}
			if res.Report.Dropped != tc.dropped || res.Report.Kept != uint64(len(tc.seqs)) || res.Report.SourceMessages != 7 {
				t.Fatalf("report %+v, want dropped %+v", res.Report, tc.dropped)
			}
			if res.Report.TombstonesRemovedByContentFilters != tc.tombs {
				t.Fatalf("tombstones removed by content filters %d, want %d", res.Report.TombstonesRemovedByContentFilters, tc.tombs)
			}
			if res.State.Msgs != uint64(len(tc.seqs)) || res.State.FirstSeq != tc.seqs[0] || res.State.LastSeq != 14 || res.State.Consumers != 2 {
				t.Fatalf("unexpected meta file state: %+v", res.State)
			}
		})
	}
}

func TestEditRenumber(t *testing.T) {
	src := fixtureDir(t, fixtureConfig(), fixtureMsgs)
	dst, res := edit(t, src, ExcludeSubjects("orders.new"), Renumber())

	items := readItems(t, dst)
	st := items[0].(*State).State
	if !reflect.DeepEqual(st, api.StreamState{FirstSeq: 1, LastSeq: 0}) {
		t.Fatalf("archive state %+v", st)
	}
	if len(consumersOf(items)) != 0 {
		t.Fatal("consumers should be dropped under renumber")
	}
	msgs := messagesOf(items)
	if !reflect.DeepEqual(seqsOf(msgs), []uint64{1, 2, 3, 4}) {
		t.Fatalf("unexpected seqs %v", seqsOf(msgs))
	}
	if msgs[0].Subject != "orders.paid" || msgs[0].Ts != 2_000 || msgs[3].Subject != "orders.paid" || msgs[3].Ts != 7_000 {
		t.Fatalf("message identity changed: %+v %+v", msgs[0], msgs[3])
	}
	if res.State.FirstSeq != 1 || res.State.LastSeq != 4 || res.State.Msgs != 4 || res.State.Consumers != 0 {
		t.Fatalf("unexpected meta file state %+v", res.State)
	}
	if res.Config.FirstSeq != 0 || loadMetaFile(t, dst).Config.FirstSeq != 0 {
		t.Fatal("renumber must clear StreamConfig.FirstSeq")
	}
	if res.Report.ConsumersDropped != 2 || res.Report.ConsumersKept != 0 {
		t.Fatalf("unexpected consumer counts %+v", res.Report)
	}
}

func TestEditEmptyResult(t *testing.T) {
	src := fixtureDir(t, fixtureConfig(), fixtureMsgs)

	dst, res := edit(t, src, Subjects("nothing.matches"))
	items := readItems(t, dst)
	if len(messagesOf(items)) != 0 || len(consumersOf(items)) != 2 {
		t.Fatalf("unexpected items: %d", len(items))
	}
	st := items[0].(*State).State
	if st.FirstSeq != 2 || st.LastSeq != 14 || st.Consumers != 2 {
		t.Fatalf("preserve must copy the source range: %+v", st)
	}
	if res.State.Msgs != 0 || res.State.Bytes != 0 || res.State.FirstSeq != 15 || res.State.LastSeq != 14 {
		t.Fatalf("meta file must describe the restored empty stream at last+1/last: %+v", res.State)
	}

	dst, res = edit(t, src, Subjects("nothing.matches"), Renumber())
	st = readItems(t, dst)[0].(*State).State
	if st.FirstSeq != 1 || st.LastSeq != 0 || res.State.FirstSeq != 1 || res.State.LastSeq != 0 {
		t.Fatalf("renumber empty result: archive %+v meta file %+v", st, res.State)
	}
}

func TestEditWarnings(t *testing.T) {
	cfg := fixtureConfig()
	cfg.MaxMsgs = 3
	cfg.MaxBytes = 100
	cfg.MaxAge = time.Hour
	src := fixtureDir(t, cfg, fixtureMsgs)

	_, res := edit(t, src)
	joined := strings.Join(res.Report.Warnings, "\n")
	for _, want := range []string{"7 messages kept but max_msgs is 3", "max_bytes is 100", "7 kept messages are already older than max_age 1h0m0s"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing warning %q in %q", want, joined)
		}
	}

	_, res = edit(t, fixtureDir(t, fixtureConfig(), fixtureMsgs))
	if len(res.Report.Warnings) != 0 {
		t.Fatalf("unexpected warnings %v", res.Report.Warnings)
	}
}

func TestEditMetaFileShape(t *testing.T) {
	src := fixtureDir(t, fixtureConfig(), fixtureMsgs)
	dst, _ := edit(t, src, FirstSeq(3), ToolVersion("nats 9.9.9"))

	if v := loadMetaFile(t, dst).Edit.Version; v != "nats 9.9.9" {
		t.Fatalf("caller version not recorded: %q", v)
	}

	raw, _ := os.ReadFile(filepath.Join(dst, MetaFile))
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"config", "state", "edit"} {
		if _, ok := generic[key]; !ok {
			t.Fatalf("meta file lacks %q: %s", key, raw)
		}
	}

	var req api.JSApiStreamRestoreRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("restore readers must still parse the meta file: %v", err)
	}
	if req.Config.Name != "ORDERS" || req.State.Msgs != 6 {
		t.Fatalf("unexpected restore request: %+v", req)
	}
	if strings.Contains(string(raw), "T") && strings.Contains(string(raw), "time") {
		t.Fatalf("meta file must not carry timestamps: %s", raw)
	}
}
