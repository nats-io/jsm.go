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
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/klauspost/compress/s2"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

var tokenRE = regexp.MustCompile(`^[0-9a-v]{16}$`)

func TestObfuscatorTokens(t *testing.T) {
	o, err := newObfuscator("")
	if err != nil {
		t.Fatal(err)
	}
	must := func(v string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	a := must(o.token("orders"))
	if a != must(o.token("orders")) || !tokenRE.MatchString(a) {
		t.Fatalf("token not stable or malformed: %q", a)
	}
	if a == must(o.token("Orders")) || a == must(o.token("orders ")) {
		t.Fatal("distinct tokens must hash differently")
	}
	if must(o.subject("orders.new")) != a+"."+must(o.token("new")) {
		t.Fatal("subjects must hash token by token")
	}
	if must(o.subject("orders.*")) != a+".*" || must(o.subject("orders.>")) != a+".>" {
		t.Fatal("wildcards must pass through")
	}
	if must(o.subject("$KV.bucket.key")) != "$KV."+must(o.token("bucket"))+"."+must(o.token("key")) {
		t.Fatal("$KV root must pass through")
	}
	if must(o.subject("$O.bucket.M.x")) != "$O."+must(o.token("bucket"))+"."+must(o.token("M"))+"."+must(o.token("x")) {
		t.Fatal("$O root must pass through")
	}
	if must(o.destination("dest.{{wildcard(1)}}")) != must(o.token("dest"))+".{{wildcard(1)}}" {
		t.Fatal("mapping functions must pass through destinations")
	}
	if must(o.subject("dest.{{tenant}}")) != must(o.token("dest"))+"."+must(o.token("{{tenant}}")) {
		t.Fatal("mapping function tokens outside destinations must hash")
	}
	if must(o.streamName("KV_bucket")) != "KV_"+must(o.token("bucket")) || must(o.streamName("OBJ_files")) != "OBJ_"+must(o.token("files")) {
		t.Fatal("bucket prefixes must be preserved")
	}
	if must(o.streamName("ORDERS")) != must(o.token("ORDERS")) || must(o.streamName("KV_")) != must(o.token("KV_")) {
		t.Fatal("plain stream names hash whole")
	}
	if must(o.token("")) != "" || must(o.subject("")) != "" {
		t.Fatal("empty values pass through")
	}

	other, _ := newObfuscator("")
	if b, _ := other.token("orders"); a == b {
		t.Fatal("a fresh secret must produce different hashes")
	}
}

func TestObfuscatorKeyFile(t *testing.T) {
	o, _ := newObfuscator("")
	o.token("orders")
	o.token("new")
	path := filepath.Join(t.TempDir(), "k.json")
	if created, err := o.writeKeyFile(path, ""); err != nil || !created {
		t.Fatalf("first write: created %v err %v", created, err)
	}
	if st, _ := os.Stat(path); st.Mode().Perm() != 0o600 {
		t.Fatalf("key file mode %v", st.Mode().Perm())
	}
	if _, err := o.writeKeyFile(path, ""); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected refusal to clobber, got %v", err)
	}
	if created, err := o.writeKeyFile(path, path); err != nil || created {
		t.Fatalf("rewriting the input key file must be allowed and not count as new: created %v err %v", created, err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Fatalf("temp file left behind: %v", entries)
	}

	loaded, err := newObfuscator(path)
	if err != nil {
		t.Fatal(err)
	}
	if h, _ := loaded.token("orders"); h != o.forward["orders"] {
		t.Fatal("loaded secret must reproduce hashes")
	}
	if !reflect.DeepEqual(loaded.reverse, o.reverse) {
		t.Fatal("loaded mappings differ")
	}

	raw, _ := os.ReadFile(path)
	var kf obfuscationKeyFile
	json.Unmarshal(raw, &kf)
	for h := range kf.Map {
		kf.Map[h] = "tampered"
	}
	tampered, _ := json.Marshal(kf)
	os.WriteFile(path, tampered, 0o600)
	if _, err := newObfuscator(path); err == nil || !strings.Contains(err.Error(), "does not match the secret") {
		t.Fatalf("expected a tamper error, got %v", err)
	}

	os.WriteFile(path, []byte(`{"version":2,"secret":"AA==","map":{}}`), 0o600)
	if _, err := newObfuscator(path); err == nil || !strings.Contains(err.Error(), "unsupported key file version") {
		t.Fatalf("expected a version error, got %v", err)
	}
}

func TestObfuscatorMessageHeaders(t *testing.T) {
	o, _ := newObfuscator("")
	hdr, err := nats.DecodeHeadersMsg([]byte("NATS/1.0\r\nKV-Operation: DEL\r\nNats-Marker-Reason: MaxAge\r\nNats-TTL: 5s\r\nNats-Rollup: sub\r\nNats-Expected-Last-Subject-Sequence: 12\r\nNats-Expected-Last-Subject-Sequence-Subject: orders.new\r\nNats-Schedule: @every 1h\r\nNats-Schedule-Source: inbox.orders\r\nNats-Schedule-TTL: 5m\r\nNats-Schedule-Target: jobs.run\r\nNats-Schedule-Time-Zone: Europe/London\r\nX-Batch: seven\r\nX-Batch: eight\r\n\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	subj, out, err := o.message("orders.new", hdr)
	if err != nil {
		t.Fatal(err)
	}
	newSubj, _ := o.subject("orders.new")
	jobs, _ := o.subject("jobs.run")
	inbox, _ := o.subject("inbox.orders")
	seven, _ := o.token("seven")
	eight, _ := o.token("eight")
	want := "NATS/1.0\r\nKV-Operation: DEL\r\nNats-Expected-Last-Subject-Sequence: 12\r\nNats-Expected-Last-Subject-Sequence-Subject: " + newSubj + "\r\nNats-Marker-Reason: MaxAge\r\nNats-Rollup: sub\r\nNats-Schedule: @every 1h\r\nNats-Schedule-Source: " + inbox + "\r\nNats-Schedule-TTL: 5m\r\nNats-Schedule-Target: " + jobs + "\r\nNats-Schedule-Time-Zone: Europe/London\r\nNats-TTL: 5s\r\nX-Batch: " + seven + "\r\nX-Batch: " + eight + "\r\n\r\n"
	if subj != newSubj || string(out) != want {
		t.Fatalf("got\n%q\nwant\n%q", out, want)
	}
	if strings.Contains(string(out), "orders") || strings.Contains(string(out), "seven") {
		t.Fatal("original values leaked")
	}
	if _, err := nats.DecodeHeadersMsg(out); err != nil {
		t.Fatalf("rewritten block does not decode: %v", err)
	}

	subj, out, err = o.message("orders.new", nil)
	if err != nil || subj != newSubj || out != nil {
		t.Fatalf("headerless message: %q %q %v", subj, out, err)
	}
}

func obfuscationFixture(t *testing.T) string {
	t.Helper()
	cfg := fixtureConfig()
	cfg.Description = "secret description"
	cfg.Metadata = map[string]string{"owner": "alice"}
	cfg.Placement = &api.Placement{Cluster: "EAST"}
	cfg.RePublish = &api.RePublish{Source: "orders.*", Destination: "republished.{{wildcard(1)}}"}
	cfg.Sources = []*api.StreamSource{{Name: "UPSTREAM", FilterSubject: "orders.>", External: &api.ExternalStream{ApiPrefix: "$JS.HUB.API"}}}
	cfg.MaxMsgs = 1000
	cfg.MaxAge = 0

	consumer := func(name string) []byte {
		data, err := json.Marshal(server.SnapshotConsumerState{
			ConsumerConfig: &server.ConsumerConfig{Durable: name, Name: name, FilterSubject: "orders.new", DeliverSubject: "delivery-inbox.orders", DeliverGroup: "acme-workers", PriorityGroups: []string{"vip-customers"}, Description: "secret consumer", Metadata: map[string]string{"team": "billing"}},
			ConsumerState:  &server.ConsumerState{Delivered: server.SequencePair{Consumer: 4, Stream: 9}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	msgs := []testMsg{
		{"orders.new", 2, 1_000, "", "order one"},
		{"orders.paid", 3, 2_000, "NATS/1.0\r\nX-Batch: batch-seven\r\n\r\n", "paid one"},
		{"orders.new", 5, 3_000, "NATS/1.0\r\nKV-Operation: DEL\r\n\r\n", ""},
		{"orders.shipped", 6, 4_000, "NATS/1.0\r\nNats-Marker-Reason: MaxAge\r\n\r\n", ""},
		{"orders.new", 9, 5_000, "NATS/1.0\r\nNats-TTL: 1s\r\nNats-Expected-Last-Subject-Sequence-Subject: orders.new\r\n\r\n", "order two"},
		{"audit.log", 10, 6_000, "", "audit entry"},
	}
	st := api.StreamState{Msgs: 6, Bytes: 1, FirstSeq: 2, LastSeq: 14, Consumers: 2}
	dir := filepath.Join(t.TempDir(), "src")
	writeBackupDir(t, dir, cfg, st, encodeBackup(t, st, map[string][]byte{"consumer-one": consumer("consumer-one"), "consumer-two": consumer("consumer-two")}, msgs, true))
	return dir
}

func decompressed(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, err := io.ReadAll(s2.NewReader(f))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func loadKeyFile(t *testing.T, path string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var kf obfuscationKeyFile
	if err := json.Unmarshal(raw, &kf); err != nil {
		t.Fatal(err)
	}
	return kf.Map
}

func TestEditObfuscate(t *testing.T) {
	src := obfuscationFixture(t)
	dst, res := edit(t, src, Obfuscate())
	if _, err := Verify(dst); err != nil {
		t.Fatalf("output does not verify: %v", err)
	}

	entries, _ := os.ReadDir(dst)
	if len(entries) != 2 {
		t.Fatalf("target must hold only the backup files: %v", entries)
	}
	keyFile := dst + keyFileSuffix
	if res.Report.Obfuscation == nil || res.Report.Obfuscation.KeyFile != keyFile {
		t.Fatalf("unexpected obfuscation report %+v", res.Report.Obfuscation)
	}
	if st, err := os.Stat(keyFile); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("key file missing or wrong mode: %v", err)
	}

	archive := string(decompressed(t, filepath.Join(dst, DataFile)))
	metaRaw, _ := os.ReadFile(filepath.Join(dst, MetaFile))
	for _, leak := range []string{"orders", "audit", "ORDERS", "UPSTREAM", "consumer-one", "consumer-two", "batch-seven", "order one", "paid one", "delivery-inbox", "republished", "HUB", "secret", "alice", "billing", "EAST", "acme-workers", "vip-customers"} {
		if strings.Contains(archive, leak) {
			t.Fatalf("archive leaks %q", leak)
		}
		if strings.Contains(string(metaRaw), leak) {
			t.Fatalf("meta file leaks %q", leak)
		}
	}
	for _, keep := range []string{"X-Batch: ", "KV-Operation: DEL", "Nats-Marker-Reason: MaxAge", "Nats-TTL: 1s"} {
		if !strings.Contains(archive, keep) {
			t.Fatalf("archive lost %q", keep)
		}
	}

	items := readItems(t, dst)
	msgs := messagesOf(items)
	if len(msgs) != 6 {
		t.Fatalf("expected 6 messages, got %d", len(msgs))
	}
	var bytesWritten uint64
	for _, m := range msgs {
		if m.PayloadSize != 0 || !tokenRE.MatchString(strings.Split(m.Subject, ".")[0]) {
			t.Fatalf("body kept or subject not hashed: %+v", m)
		}
		bytesWritten += storedMsgSize(len(m.Subject), m.HdrSize, m.PayloadSize)
	}
	if res.Report.Obfuscation.BodiesDropped != 4 || res.State.Bytes != bytesWritten || res.State.Msgs != 6 {
		t.Fatalf("unexpected accounting: report %+v state %+v written %d", res.Report.Obfuscation, res.State, bytesWritten)
	}
	if st := items[0].(*State).State; st.Bytes != 0 || st.Consumers != 2 {
		t.Fatalf("archive state must carry advisory bytes: %+v", st)
	}

	mapping := loadKeyFile(t, keyFile)
	originals := map[string]string{}
	for h, orig := range mapping {
		originals[orig] = h
	}
	for _, want := range []string{"orders", "new", "paid", "shipped", "audit", "log", "ORDERS", "UPSTREAM", "consumer-one", "consumer-two", "batch-seven", "delivery-inbox", "republished", "$JS", "HUB", "API", "acme-workers", "vip-customers"} {
		if _, ok := originals[want]; !ok {
			t.Fatalf("key file lacks a mapping for %q", want)
		}
	}
	if res.Report.Obfuscation.TokensMapped != len(mapping) {
		t.Fatalf("tokens mapped %d vs key file %d", res.Report.Obfuscation.TokensMapped, len(mapping))
	}

	sc := loadMetaFile(t, dst)
	cfg := sc.Config
	if cfg.Name != originals["ORDERS"] || !reflect.DeepEqual(cfg.Subjects, []string{originals["orders"] + ".>", originals["audit"] + ".*"}) {
		t.Fatalf("config identity not hashed: %+v", cfg)
	}
	if cfg.Description != "" || cfg.Metadata != nil || cfg.Placement != nil || cfg.MaxMsgs != 1000 {
		t.Fatalf("descriptive fields must be dropped and limits kept: %+v", cfg)
	}
	if cfg.RePublish.Source != originals["orders"]+".*" || cfg.RePublish.Destination != originals["republished"]+".{{wildcard(1)}}" {
		t.Fatalf("republish not hashed: %+v", cfg.RePublish)
	}
	if cfg.Sources[0].Name != originals["UPSTREAM"] || cfg.Sources[0].External.ApiPrefix != originals["$JS"]+"."+originals["HUB"]+"."+originals["API"] {
		t.Fatalf("source not hashed: %+v", cfg.Sources[0])
	}
	if !sc.Edit.Obfuscated || !reflect.DeepEqual(sc.Edit.Options, []string{"obfuscate"}) {
		t.Fatalf("unexpected edit block %+v", sc.Edit)
	}

	consumers := consumersOf(items)
	for i, orig := range []string{"consumer-one", "consumer-two"} {
		c := consumers[i]
		var scs server.SnapshotConsumerState
		if err := json.Unmarshal(c.Data, &scs); err != nil {
			t.Fatal(err)
		}
		if c.Name != originals[orig] || scs.Durable != originals[orig] || scs.ConsumerConfig.Name != originals[orig] {
			t.Fatalf("consumer identity not hashed consistently: %s %+v", c.Name, scs.ConsumerConfig)
		}
		if scs.FilterSubject != originals["orders"]+"."+originals["new"] || scs.DeliverSubject != originals["delivery-inbox"]+"."+originals["orders"] {
			t.Fatalf("consumer subjects not hashed: %+v", scs.ConsumerConfig)
		}
		if scs.DeliverGroup != originals["acme-workers"] || !reflect.DeepEqual(scs.PriorityGroups, []string{originals["vip-customers"]}) {
			t.Fatalf("consumer groups not hashed: %+v", scs.ConsumerConfig)
		}
		if scs.Description != "" || scs.Metadata != nil || scs.Delivered.Stream != 9 {
			t.Fatalf("consumer description/metadata must be dropped, state kept: %+v", scs)
		}
		if !server.SubjectsCollide(scs.FilterSubject, cfg.Subjects[0]) {
			t.Fatalf("hashed filter %q no longer within hashed subject %q", scs.FilterSubject, cfg.Subjects[0])
		}
	}

	seq9 := msgs[4]
	body, _ := io.ReadAll(seq9.Body)
	if !strings.Contains(string(body), "Nats-Expected-Last-Subject-Sequence-Subject: "+originals["orders"]+"."+originals["new"]) {
		t.Fatalf("subject-valued header not token hashed: %q", body)
	}
	seq3 := msgs[1]
	body, _ = io.ReadAll(seq3.Body)
	if !strings.Contains(string(body), "X-Batch: "+originals["batch-seven"]) {
		t.Fatalf("header value not hashed through the table: %q", body)
	}
}

func TestEditObfuscateDeterminism(t *testing.T) {
	src := obfuscationFixture(t)
	first, _ := edit(t, src, Obfuscate())
	second, _ := edit(t, src, Obfuscate(), ObfuscationKeyFile(first+keyFileSuffix))
	third, _ := edit(t, src, Obfuscate())

	for _, name := range []string{DataFile, MetaFile} {
		a, _ := os.ReadFile(filepath.Join(first, name))
		b, _ := os.ReadFile(filepath.Join(second, name))
		c, _ := os.ReadFile(filepath.Join(third, name))
		if !bytes.Equal(a, b) {
			t.Fatalf("%s differs under the same key file", name)
		}
		if bytes.Equal(a, c) {
			t.Fatalf("%s identical under a fresh secret", name)
		}
	}
	if !reflect.DeepEqual(loadKeyFile(t, first+keyFileSuffix), loadKeyFile(t, second+keyFileSuffix)) {
		t.Fatal("key files differ under the same secret")
	}
}

func TestEditObfuscateDryRunAndRefusals(t *testing.T) {
	src := obfuscationFixture(t)
	parent := t.TempDir()
	dst := filepath.Join(parent, "dst")

	dry, err := Edit(context.Background(), src, dst, Obfuscate(), DryRun())
	if err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(parent); len(entries) != 0 {
		t.Fatalf("dry run wrote files: %v", entries)
	}
	if dry.Report.Obfuscation == nil || dry.Report.Obfuscation.KeyFile != "" || dry.Report.Obfuscation.BodiesDropped != 4 || dry.State.Bytes == 0 {
		t.Fatalf("unexpected dry run result %+v %+v", dry.Report.Obfuscation, dry.State)
	}

	os.WriteFile(dst+keyFileSuffix, []byte("{}"), 0o600)
	_, err = Edit(context.Background(), src, dst, Obfuscate())
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected refusal to clobber a key file, got %v", err)
	}
	if _, err := os.Stat(dst); err == nil {
		t.Fatal("target must not exist after a refused edit")
	}
}

func TestEditObfuscateKVChain(t *testing.T) {
	src := kvDir(t, kvConfig())
	obf, res := edit(t, src, Obfuscate())
	if !strings.HasPrefix(res.Config.Name, "KV_") || res.Config.Subjects[0] != "$KV."+strings.TrimPrefix(res.Config.Name, "KV_")+".>" {
		t.Fatalf("bucket relation lost: %+v", res.Config)
	}

	dst, res := edit(t, obf, KVCompact())
	if res.Report.Kept != 2 || res.Report.Dropped.KVCompact != 7 {
		t.Fatalf("compaction of the obfuscated bucket differs: %+v", res.Report)
	}
	if _, err := Verify(dst); err != nil {
		t.Fatal(err)
	}

	src = kvDir(t, kvConfig())
	_, res = edit(t, src, Obfuscate(), KVCompact(), Renumber())
	if res.Report.Kept != 2 || res.State.Bytes == 0 || res.State.FirstSeq != 1 || res.State.LastSeq != 2 {
		t.Fatalf("obfuscated two-pass renumber: %+v %+v", res.Report, res.State)
	}
}

func TestEditObfuscateFailedCommitKeepsInputKeyFile(t *testing.T) {
	src := kvDir(t, kvConfig())
	first, _ := edit(t, src, Obfuscate())
	keyFile := first + keyFileSuffix
	before := loadKeyFile(t, keyFile)

	parent := t.TempDir()
	dst := filepath.Join(parent, "dst")
	block := func() {
		if err := os.WriteFile(dst, []byte("in the way"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	_, err := Edit(context.Background(), src, dst, Obfuscate(), KVCompact(), ObfuscationKeyFile(keyFile), betweenPasses(block))
	var linkErr *os.LinkError
	if !errors.As(err, &linkErr) || linkErr.Op != "rename" {
		t.Fatalf("expected the commit rename to fail, got %v", err)
	}
	after := loadKeyFile(t, keyFile)
	for h, orig := range before {
		if after[h] != orig {
			t.Fatalf("input key file lost mapping %q", h)
		}
	}
	if _, err := newObfuscator(keyFile); err != nil {
		t.Fatalf("input key file no longer loads: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "dst"+keyFileSuffix)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a key file was left beside the failed target: %v", err)
	}
}
