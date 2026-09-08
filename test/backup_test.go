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

package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/backup"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	ntfclient "github.com/synadia-io/orbit.go/ntf-client"
)

func deleteMsg(t testing.TB, stream *jsm.Stream, seq uint64) {
	t.Helper()
	checkErr(t, stream.DeleteMessageRequest(api.JSApiMsgDeleteRequest{Seq: seq}), "delete failed")
}

func snapshotFixture(t testing.TB, mgr *jsm.Manager, stream *jsm.Stream) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "src")
	_, err := stream.SnapshotToDirectory(context.Background(), dir, jsm.SnapshotConsumers())
	checkErr(t, err, "snapshot failed")
	return dir
}

func publishMsg(t testing.TB, nc *nats.Conn, subject, body string, hdr ...string) {
	t.Helper()
	msg := nats.NewMsg(subject)
	msg.Data = []byte(body)
	for i := 0; i+1 < len(hdr); i += 2 {
		msg.Header.Add(hdr[i], hdr[i+1])
	}
	_, err := nc.RequestMsg(msg, time.Second)
	checkErr(t, err, "publish failed")
}

func TestBackupVerifyAndInfoOnServerSnapshot(t *testing.T) {
	withJSServer(t, func(t testing.TB, nc *nats.Conn, mgr *jsm.Manager, _ *ntfclient.Instance) {
		stream, err := mgr.NewStream("ORDERS", jsm.FileStorage(), jsm.Subjects("orders.>"))
		checkErr(t, err, "create failed")
		_, err = stream.NewConsumer(jsm.DurableName("c1"), jsm.FilterStreamBySubject("orders.new"))
		checkErr(t, err, "consumer failed")
		_, err = stream.NewConsumer(jsm.DurableName("c2"))
		checkErr(t, err, "consumer failed")

		for i := 1; i <= 20; i++ {
			msg := nats.NewMsg(fmt.Sprintf("orders.%s", []string{"new", "paid"}[i%2]))
			msg.Data = []byte(RandomString(100 * i))
			if i%3 == 0 {
				msg.Header.Set("X-Batch", fmt.Sprintf("%d", i))
			}
			_, err := nc.RequestMsg(msg, time.Second)
			checkErr(t, err, "publish failed")
		}
		deleteMsg(t, stream, 1)
		deleteMsg(t, stream, 7)

		state, err := stream.State()
		checkErr(t, err, "state failed")

		dir := snapshotFixture(t, mgr, stream)

		rep, err := backup.Verify(dir)
		checkErr(t, err, "verify failed")
		if !rep.Complete || rep.Entries != 22 || rep.Consumers != 2 || rep.Messages != state.Msgs || rep.FirstSeq != state.FirstSeq || rep.LastSeq != state.LastSeq {
			t.Fatalf("unexpected report: %+v vs state %+v", rep, state)
		}

		info, err := backup.Info(dir)
		checkErr(t, err, "info failed")
		if info.Config.Name != "ORDERS" || !reflect.DeepEqual(info.Config.Subjects, []string{"orders.>"}) {
			t.Fatalf("unexpected config: %+v", info.Config)
		}
		if info.Messages != state.Msgs || info.FirstSeq != state.FirstSeq || info.LastSeq != state.LastSeq {
			t.Fatalf("unexpected counts: %+v vs %+v", info, state)
		}
		if info.Bytes != state.Bytes {
			t.Fatalf("stored size mirror disagrees with the server: %d vs %d", info.Bytes, state.Bytes)
		}
		if !info.FirstTime.Equal(state.FirstTime) || !info.LastTime.Equal(state.LastTime) {
			t.Fatalf("unexpected times: %v-%v vs %v-%v", info.FirstTime, info.LastTime, state.FirstTime, state.LastTime)
		}
		if len(info.Consumers) != 2 || info.DeclaredCountsAdvisory || info.Declared.Msgs != state.Msgs || info.Declared.Consumers != 2 || info.Edit != nil {
			t.Fatalf("unexpected declared state: %+v", info)
		}

		deleteMsg(t, stream, 20)
		state, err = stream.State()
		checkErr(t, err, "state failed")
		dir = snapshotFixture(t, mgr, stream)
		info, err = backup.Info(dir)
		checkErr(t, err, "info failed")
		if info.LastSeq != 19 || info.Declared.LastSeq != 20 || state.LastSeq != 20 || info.Messages != state.Msgs {
			t.Fatalf("trailing delete not reflected: archive last %d, declared last %d, state %+v", info.LastSeq, info.Declared.LastSeq, state)
		}
	})
}

type editFixture struct {
	nc     *nats.Conn
	mgr    *jsm.Manager
	dir    string
	pre    api.StreamState
	mid    time.Time
	bodies map[uint64]string
}

func newEditFixture(t testing.TB, nc *nats.Conn, mgr *jsm.Manager, streamOpts ...jsm.StreamOption) *editFixture {
	t.Helper()

	opts := append([]jsm.StreamOption{jsm.FileStorage(), jsm.Subjects("orders.>", "audit.*")}, streamOpts...)
	stream, err := mgr.NewStream("ORDERS", opts...)
	checkErr(t, err, "create failed")
	_, err = stream.NewConsumer(jsm.DurableName("c1"), jsm.FilterStreamBySubject("orders.new"))
	checkErr(t, err, "consumer failed")
	_, err = stream.NewConsumer(jsm.DurableName("c2"))
	checkErr(t, err, "consumer failed")

	first := stream.FirstSequence()
	if first == 0 {
		first = 1
	}
	f := &editFixture{nc: nc, mgr: mgr, bodies: map[uint64]string{}}
	publish := func(offset uint64, subject, body string, hdr ...string) {
		publishMsg(t, nc, subject, body, hdr...)
		f.bodies[first+offset] = body
	}

	publish(0, "orders.new", "order 1")
	publish(1, "orders.paid", "paid 1", "X-Batch", "7")
	publish(2, "orders.new", "order 2 urgent", "X-Batch", "7")
	time.Sleep(10 * time.Millisecond)
	f.mid = time.Now()
	time.Sleep(10 * time.Millisecond)
	publish(3, "audit.log", "audit entry")
	publish(4, "orders.shipped", "shipped urgent", "X-Batch", "8")
	publish(5, "orders.paid", "paid 2")
	deleteMsg(t, stream, first+3)
	delete(f.bodies, first+3)

	f.pre, err = stream.State()
	checkErr(t, err, "state failed")
	f.dir = snapshotFixture(t, mgr, stream)
	checkErr(t, stream.Delete(), "delete failed")

	return f
}

func (f *editFixture) editAndRestore(t testing.TB, opts ...backup.EditOption) (*backup.Result, *jsm.Stream, api.StreamState) {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "edited")
	res, err := backup.Edit(context.Background(), f.dir, dst, opts...)
	checkErr(t, err, "edit failed")
	_, err = backup.Verify(dst)
	checkErr(t, err, "edited backup does not verify")

	_, st, err := f.mgr.RestoreSnapshotFromDirectory(context.Background(), "ORDERS", dst)
	checkErr(t, err, "restore failed")
	stream, err := f.mgr.LoadStream("ORDERS")
	checkErr(t, err, "load failed")

	if st.Bytes != res.State.Bytes {
		t.Fatalf("restored bytes %d differ from the meta file's %d", st.Bytes, res.State.Bytes)
	}
	if st.Msgs != res.State.Msgs || st.FirstSeq != res.State.FirstSeq || st.LastSeq != res.State.LastSeq || st.Consumers != res.State.Consumers {
		t.Fatalf("restored state %+v disagrees with the meta file %+v", st, res.State)
	}

	return res, stream, *st
}

func presentSeqs(t testing.TB, stream *jsm.Stream, st api.StreamState, bodies map[uint64]string) []uint64 {
	t.Helper()
	var seqs []uint64
	if st.Msgs == 0 {
		return seqs
	}
	for seq := st.FirstSeq; seq <= st.LastSeq; seq++ {
		msg, err := stream.ReadMessage(seq)
		if err != nil {
			continue
		}
		if string(msg.Data) != bodies[seq] {
			t.Fatalf("seq %d restored with body %q, want %q", seq, msg.Data, bodies[seq])
		}
		seqs = append(seqs, seq)
	}
	return seqs
}

func TestBackupEditIdentityRestores(t *testing.T) {
	withJSServer(t, func(t testing.TB, nc *nats.Conn, mgr *jsm.Manager, _ *ntfclient.Instance) {
		f := newEditFixture(t, nc, mgr)
		res, stream, st := f.editAndRestore(t)

		if !reflect.DeepEqual(f.pre, st) {
			t.Fatalf("restored state differs from the source:\n%+v\n%+v", f.pre, st)
		}
		if !reflect.DeepEqual(presentSeqs(t, stream, st, f.bodies), []uint64{1, 2, 3, 5, 6}) {
			t.Fatal("restored messages differ from the source")
		}
		names, err := stream.ConsumerNames()
		checkErr(t, err, "consumer names failed")
		if !reflect.DeepEqual(names, []string{"c1", "c2"}) {
			t.Fatalf("unexpected consumers %v", names)
		}
		if res.Report.Kept != 5 || res.Report.ConsumersKept != 2 {
			t.Fatalf("unexpected report %+v", res.Report)
		}
	})
}

func TestBackupEditFiltersRestore(t *testing.T) {
	withJSServer(t, func(t testing.TB, nc *nats.Conn, mgr *jsm.Manager, _ *ntfclient.Instance) {
		f := newEditFixture(t, nc, mgr)
		res, stream, st := f.editAndRestore(t, backup.Subjects("orders.>"), backup.Before(f.mid), backup.HeaderPresent("X-Batch"), backup.ExcludePayloadMatch(regexp.MustCompile("urgent")))

		if got := presentSeqs(t, stream, st, f.bodies); !reflect.DeepEqual(got, []uint64{2}) {
			t.Fatalf("restored seqs %v, want [2]", got)
		}
		if st.FirstSeq != 2 || st.LastSeq != 6 || st.Msgs != 1 || st.NumDeleted != 4 {
			t.Fatalf("preserve must keep source sequences with gaps up to the source last: %+v", st)
		}
		if st.Consumers != 2 || res.Report.Kept != 1 || res.Report.SourceMessages != 5 {
			t.Fatalf("unexpected state %+v report %+v", st, res.Report)
		}
	})
}

func TestBackupEditRenumberRestores(t *testing.T) {
	withJSServer(t, func(t testing.TB, nc *nats.Conn, mgr *jsm.Manager, _ *ntfclient.Instance) {
		f := newEditFixture(t, nc, mgr, jsm.FirstSequence(1000))
		if f.pre.FirstSeq != 1000 || f.pre.LastSeq != 1005 {
			t.Fatalf("fixture not numbered from 1000: %+v", f.pre)
		}

		res, stream, st := f.editAndRestore(t, backup.Renumber(), backup.ExcludeSubjects("audit.*"))
		if st.FirstSeq != 1 || st.LastSeq != 5 || st.Msgs != 5 || st.NumDeleted != 0 || st.Consumers != 0 {
			t.Fatalf("unexpected restored state %+v", st)
		}
		if stream.FirstSequence() != 0 || res.Config.FirstSeq != 0 {
			t.Fatalf("renumber must clear the configured first sequence: %d", stream.FirstSequence())
		}

		want := []string{"order 1", "paid 1", "order 2 urgent", "shipped urgent", "paid 2"}
		for i, body := range want {
			msg, err := stream.ReadMessage(uint64(i + 1))
			checkErr(t, err, "read failed")
			if string(msg.Data) != body {
				t.Fatalf("seq %d has %q, want %q", i+1, msg.Data, body)
			}
		}
		names, err := stream.ConsumerNames()
		checkErr(t, err, "consumer names failed")
		if len(names) != 0 {
			t.Fatalf("renumber must drop consumers, got %v", names)
		}

		publishMsg(t, f.nc, "orders.new", "after restore")
		st, err = stream.State()
		checkErr(t, err, "state failed")
		if st.LastSeq != 6 || st.Msgs != 6 {
			t.Fatalf("stream did not continue at 6: %+v", st)
		}
	})
}

func TestBackupEditEmptyResultRestores(t *testing.T) {
	withJSServer(t, func(t testing.TB, nc *nats.Conn, mgr *jsm.Manager, _ *ntfclient.Instance) {
		f := newEditFixture(t, nc, mgr)
		_, stream, st := f.editAndRestore(t, backup.Subjects("nothing.*"))
		if st.Msgs != 0 || st.FirstSeq != 7 || st.LastSeq != 6 || st.Consumers != 2 {
			t.Fatalf("expected an empty stream at last+1/last, got %+v", st)
		}

		publishMsg(t, f.nc, "orders.new", "after restore")
		st, err := stream.State()
		checkErr(t, err, "state failed")
		if st.FirstSeq != 7 || st.LastSeq != 7 {
			t.Fatalf("stream did not continue at 7: %+v", st)
		}
	})
}

func TestBackupEditEmptyRenumberRestores(t *testing.T) {
	withJSServer(t, func(t testing.TB, nc *nats.Conn, mgr *jsm.Manager, _ *ntfclient.Instance) {
		f := newEditFixture(t, nc, mgr)
		_, stream, st := f.editAndRestore(t, backup.Subjects("nothing.*"), backup.Renumber())
		if st.Msgs != 0 || st.FirstSeq != 1 || st.LastSeq != 0 || st.Consumers != 0 {
			t.Fatalf("expected an empty stream at 1/0, got %+v", st)
		}

		publishMsg(t, f.nc, "orders.new", "after restore")
		st, err := stream.State()
		checkErr(t, err, "state failed")
		if st.FirstSeq != 1 || st.LastSeq != 1 {
			t.Fatalf("stream did not start at 1: %+v", st)
		}
	})
}

type kvFixture struct {
	nc   *nats.Conn
	js   nats.JetStreamContext
	mgr  *jsm.Manager
	dir  string
	revA uint64
	msgs uint64
}

func newKVFixture(t testing.TB, nc *nats.Conn, mgr *jsm.Manager) *kvFixture {
	t.Helper()

	js, err := nc.JetStream()
	checkErr(t, err, "jetstream failed")
	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "orders", History: 5})
	checkErr(t, err, "bucket failed")

	put := func(key, value string) {
		_, err := kv.Put(key, []byte(value))
		checkErr(t, err, "put failed")
	}
	put("a", "a1")
	put("a", "a2")
	put("b", "b1")
	checkErr(t, kv.Delete("b"), "delete failed")
	put("c", "c1")
	checkErr(t, kv.Purge("c"), "purge failed")
	put("d", "d1")
	put("e", "e1")
	publishMsg(t, nc, "$KV.orders.e", "", "Nats-Marker-Reason", "MaxAge")
	put("f", "f1")
	publishMsg(t, nc, "$KV.orders.f", "f2", "KV-Operation", "FOO", "Nats-Marker-Reason", "Remove")

	entry, err := kv.Get("a")
	checkErr(t, err, "get failed")

	stream, err := mgr.LoadStream("KV_orders")
	checkErr(t, err, "load failed")
	st, err := stream.State()
	checkErr(t, err, "state failed")

	f := &kvFixture{nc: nc, js: js, mgr: mgr, revA: entry.Revision(), msgs: st.Msgs}
	f.dir = snapshotFixture(t, mgr, stream)
	checkErr(t, stream.Delete(), "delete failed")

	return f
}

func (f *kvFixture) editAndRestore(t testing.TB, opts ...backup.EditOption) (*backup.Result, nats.KeyValue) {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "edited")
	res, err := backup.Edit(context.Background(), f.dir, dst, opts...)
	checkErr(t, err, "edit failed")
	_, err = backup.Verify(dst)
	checkErr(t, err, "edited backup does not verify")

	_, st, err := f.mgr.RestoreSnapshotFromDirectory(context.Background(), "KV_orders", dst)
	checkErr(t, err, "restore failed")
	if st.Msgs != res.State.Msgs || st.Bytes != res.State.Bytes || st.FirstSeq != res.State.FirstSeq || st.LastSeq != res.State.LastSeq {
		t.Fatalf("restored state %+v disagrees with the meta file %+v", st, res.State)
	}

	kv, err := f.js.KeyValue("orders")
	checkErr(t, err, "bucket load failed")
	return res, kv
}

func expectValue(t testing.TB, kv nats.KeyValue, key, value string) nats.KeyValueEntry {
	t.Helper()
	entry, err := kv.Get(key)
	checkErr(t, err, "get "+key+" failed")
	if string(entry.Value()) != value {
		t.Fatalf("key %s has %q, want %q", key, entry.Value(), value)
	}
	history, err := kv.History(key)
	checkErr(t, err, "history failed")
	if len(history) != 1 {
		t.Fatalf("key %s has %d revisions after compaction", key, len(history))
	}
	return entry
}

func expectAbsent(t testing.TB, kv nats.KeyValue, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if _, err := kv.Get(key); !errors.Is(err, nats.ErrKeyNotFound) {
			t.Fatalf("key %s should be gone, got %v", key, err)
		}
		if _, err := kv.History(key); !errors.Is(err, nats.ErrKeyNotFound) {
			t.Fatalf("key %s should have no history, got %v", key, err)
		}
	}
}

func TestBackupEditKVCompactRestores(t *testing.T) {
	withJSServer(t, func(t testing.TB, nc *nats.Conn, mgr *jsm.Manager, _ *ntfclient.Instance) {
		f := newKVFixture(t, nc, mgr)
		res, kv := f.editAndRestore(t, backup.KVCompact())

		a := expectValue(t, kv, "a", "a2")
		expectValue(t, kv, "d", "d1")
		expectValue(t, kv, "f", "f2")
		expectAbsent(t, kv, "b", "c", "e")

		if a.Revision() != f.revA {
			t.Fatalf("preserve must keep revisions: %d vs %d", a.Revision(), f.revA)
		}
		if _, err := kv.Update("a", []byte("a3"), f.revA); err != nil {
			t.Fatalf("compare-and-set with the pre-edit revision failed: %v", err)
		}

		keys, err := kv.Keys()
		checkErr(t, err, "keys failed")
		if !reflect.DeepEqual(keys, []string{"a", "d", "f"}) {
			t.Fatalf("unexpected keys %v", keys)
		}
		if res.Report.Kept != 3 || res.Report.SourceMessages != f.msgs || res.Report.Dropped.KVCompact != f.msgs-3 || res.Report.SubjectStateKeys != 6 {
			t.Fatalf("unexpected report %+v", res.Report)
		}

		stream, err := f.mgr.LoadStream("KV_orders")
		checkErr(t, err, "load failed")
		checkErr(t, stream.Delete(), "delete failed")

		res, kv = f.editAndRestore(t, backup.KVCompact(), backup.Renumber())
		a = expectValue(t, kv, "a", "a2")
		if a.Revision() != 1 || res.State.FirstSeq != 1 || res.State.LastSeq != 3 {
			t.Fatalf("renumber should number survivors from 1: revision %d state %+v", a.Revision(), res.State)
		}
		if _, err := kv.Update("a", []byte("a3"), f.revA); err == nil {
			t.Fatal("compare-and-set with the pre-edit revision must fail after renumbering")
		}
		if _, err := kv.Update("a", []byte("a3"), 1); err != nil {
			t.Fatalf("compare-and-set with the new revision failed: %v", err)
		}
	})
}

func TestBackupEditLastPerSubjectRestores(t *testing.T) {
	withJSServer(t, func(t testing.TB, nc *nats.Conn, mgr *jsm.Manager, _ *ntfclient.Instance) {
		f := newEditFixture(t, nc, mgr)
		res, stream, st := f.editAndRestore(t, backup.LastPerSubject(1))
		if got := presentSeqs(t, stream, st, f.bodies); !reflect.DeepEqual(got, []uint64{3, 5, 6}) {
			t.Fatalf("restored seqs %v", got)
		}
		if st.FirstSeq != 3 || st.LastSeq != 6 || st.Consumers != 2 || res.Report.Dropped.LastPerSubject != 2 {
			t.Fatalf("unexpected state %+v report %+v", st, res.Report)
		}
	})

	withJSServer(t, func(t testing.TB, nc *nats.Conn, mgr *jsm.Manager, _ *ntfclient.Instance) {
		f := newEditFixture(t, nc, mgr)
		_, stream, st := f.editAndRestore(t, backup.LastPerSubject(1), backup.Renumber())
		if st.FirstSeq != 1 || st.LastSeq != 3 || st.Msgs != 3 || st.Consumers != 0 {
			t.Fatalf("unexpected renumbered state %+v", st)
		}
		for seq, body := range map[uint64]string{1: "order 2 urgent", 2: "shipped urgent", 3: "paid 2"} {
			msg, err := stream.ReadMessage(seq)
			checkErr(t, err, "read failed")
			if string(msg.Data) != body {
				t.Fatalf("seq %d has %q, want %q", seq, msg.Data, body)
			}
		}
	})
}

func obfuscationMap(t testing.TB, path string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	checkErr(t, err, "key file read failed")
	var kf struct {
		Map map[string]string `json:"map"`
	}
	checkErr(t, json.Unmarshal(raw, &kf), "key file parse failed")
	originals := map[string]string{}
	for hashed, original := range kf.Map {
		originals[original] = hashed
	}
	return originals
}

func TestBackupEditObfuscateRestores(t *testing.T) {
	withJSServer(t, func(t testing.TB, nc *nats.Conn, mgr *jsm.Manager, _ *ntfclient.Instance) {
		f := newEditFixture(t, nc, mgr)
		dst := filepath.Join(t.TempDir(), "edited")
		res, err := backup.Edit(context.Background(), f.dir, dst, backup.Obfuscate())
		checkErr(t, err, "edit failed")
		_, err = backup.Verify(dst)
		checkErr(t, err, "edited backup does not verify")

		originals := obfuscationMap(t, res.Report.Obfuscation.KeyFile)
		if res.Config.Name != originals["ORDERS"] || strings.Contains(res.Config.Name, "ORDERS") {
			t.Fatalf("stream name not hashed: %q", res.Config.Name)
		}

		_, st, err := f.mgr.RestoreSnapshotFromDirectory(context.Background(), res.Config.Name, dst)
		checkErr(t, err, "restore of the obfuscated backup failed")
		if st.Msgs != res.State.Msgs || st.Bytes != res.State.Bytes || st.FirstSeq != res.State.FirstSeq || st.LastSeq != res.State.LastSeq || st.Consumers != 2 {
			t.Fatalf("restored state %+v disagrees with the meta file %+v", st, res.State)
		}

		stream, err := f.mgr.LoadStream(res.Config.Name)
		checkErr(t, err, "load failed")
		names, err := stream.ConsumerNames()
		checkErr(t, err, "consumer names failed")
		want := []string{originals["c1"], originals["c2"]}
		slices.Sort(want)
		if !slices.Equal(names, want) {
			t.Fatalf("consumers not restored under hashed names: %v", names)
		}
		c1, err := stream.LoadConsumer(originals["c1"])
		checkErr(t, err, "consumer load failed")
		if c1.FilterSubject() != originals["orders"]+"."+originals["new"] {
			t.Fatalf("consumer filter not hashed consistently: %q", c1.FilterSubject())
		}

		msg, err := stream.ReadMessage(2)
		checkErr(t, err, "read failed")
		if len(msg.Data) != 0 || msg.Subject != originals["orders"]+"."+originals["paid"] {
			t.Fatalf("message not obfuscated: %+v", msg)
		}
		hdr, err := nats.DecodeHeadersMsg(msg.Header)
		checkErr(t, err, "header decode failed")
		if hdr.Get("X-Batch") != originals["7"] {
			t.Fatalf("header value not hashed: %v", hdr)
		}
	})
}

func TestBackupEditObfuscateKVRestores(t *testing.T) {
	withJSServer(t, func(t testing.TB, nc *nats.Conn, mgr *jsm.Manager, _ *ntfclient.Instance) {
		f := newKVFixture(t, nc, mgr)
		dst := filepath.Join(t.TempDir(), "edited")
		res, err := backup.Edit(context.Background(), f.dir, dst, backup.Obfuscate())
		checkErr(t, err, "edit failed")
		originals := obfuscationMap(t, res.Report.Obfuscation.KeyFile)

		_, st, err := f.mgr.RestoreSnapshotFromDirectory(context.Background(), res.Config.Name, dst)
		checkErr(t, err, "restore failed")
		if st.Msgs != res.State.Msgs || st.Bytes != res.State.Bytes {
			t.Fatalf("restored state %+v disagrees with the meta file %+v", st, res.State)
		}

		ctx := context.Background()
		js, err := jetstream.New(f.nc)
		checkErr(t, err, "jetstream failed")
		kv, err := js.KeyValue(ctx, originals["orders"])
		checkErr(t, err, "bucket load failed")
		for _, key := range []string{"a", "d", "f"} {
			entry, err := kv.Get(ctx, originals[key])
			checkErr(t, err, "get "+key+" failed")
			if len(entry.Value()) != 0 {
				t.Fatalf("value of %s survived obfuscation: %q", key, entry.Value())
			}
		}
		keys, err := kv.Keys(ctx)
		checkErr(t, err, "keys failed")
		if len(keys) != 3 {
			t.Fatalf("expected only a, d and f to be live, got %v", keys)
		}
		for key, want := range map[string]jetstream.KeyValueOp{"b": jetstream.KeyValueDelete, "c": jetstream.KeyValuePurge, "e": jetstream.KeyValuePurge} {
			if _, err := kv.Get(ctx, originals[key]); !errors.Is(err, jetstream.ErrKeyNotFound) {
				t.Fatalf("tombstone of %s lost: %v", key, err)
			}
			history, err := kv.History(ctx, originals[key])
			checkErr(t, err, "history of "+key+" failed")
			if got := history[len(history)-1].Operation(); got != want {
				t.Fatalf("latest op of %s is %v, want %v", key, got, want)
			}
		}
		history, err := kv.History(ctx, originals["a"])
		checkErr(t, err, "history failed")
		if len(history) != 2 {
			t.Fatalf("key a has %d revisions, want 2", len(history))
		}
	})
}
