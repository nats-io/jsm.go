// Copyright 2021 The NATS Authors
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

package kv

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	natsd "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type BOrT interface {
	Helper()
	Fatalf(format string, args ...interface{})
}

type base64Codec struct{}

func (r base64Codec) Encode(v []byte) ([]byte, error) {
	bres := make([]byte, base64.StdEncoding.EncodedLen(len(v)))
	base64.StdEncoding.Encode(bres, v)
	return bres, nil
}

func (r base64Codec) Decode(v []byte) ([]byte, error) {
	dbuff := make([]byte, base64.StdEncoding.DecodedLen(len(v)))
	n, err := base64.StdEncoding.Decode(dbuff, v)
	return dbuff[:n], err
}

func assertEntryHasStringValue(t *testing.T, res Entry, val string) {
	t.Helper()

	if bytes.Equal(res.Value(), []byte(val)) {
		return
	}

	t.Fatalf("%s should have value %q got %q", res.Key(), val, res.Value())
}

func setupBasicTestBucket(t BOrT, so ...Option) (*jetStreamStorage, *natsd.Server, *nats.Conn, *jsm.Manager) {
	t.Helper()

	srv, nc, mgr := startJSServer(t)
	opts, _ := newOpts(so...)
	opts.history = 5
	store, err := newJetStreamStorage("TEST", nc, opts)
	if err != nil {
		t.Fatalf("store create failed: %s", err)
	}

	err = store.CreateBucket()
	if err != nil {
		t.Fatalf("create failed: %s", err)
	}

	return store.(*jetStreamStorage), srv, nc, mgr
}

func TestJetStreamStorage_WithStreamSubjectPrefix(t *testing.T) {
	store, srv, nc, _ := setupBasicTestBucket(t, WithStreamSubjectPrefix("$BOB"))
	defer srv.Shutdown()
	defer nc.Close()
	defer store.Close()

	_, err := store.Put("hello", []byte("world"))
	if err != nil {
		t.Fatalf("put failed: %s", err)
	}

	val, err := store.Get("hello")
	if err != nil {
		t.Fatalf("get failed: %s", err)
	}

	assertEntryHasStringValue(t, val, "world")

	str, err := store.getOrLoadStream()
	if err != nil {
		t.Fatalf("stream load failed: %s", err)
	}

	if !cmp.Equal(str.Subjects(), []string{"$BOB.TEST.>"}) {
		t.Fatalf("invalid stream subjects: %v", str.Subjects())
	}
}

func TestJetStreamStorage_WithStreamName(t *testing.T) {
	store, srv, nc, mgr := setupBasicTestBucket(t, WithStreamName("OVERRIDE"))
	defer srv.Shutdown()
	defer nc.Close()
	defer store.Close()

	_, err := store.Put("hello", []byte("world"))
	if err != nil {
		t.Fatalf("put failed: %s", err)
	}

	val, err := store.Get("hello")
	if err != nil {
		t.Fatalf("get failed: %s", err)
	}

	assertEntryHasStringValue(t, val, "world")

	assertStream := func(t *testing.T, stream string, should bool) {
		known, err := mgr.IsKnownStream(stream)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if should && !known {
			t.Fatalf("%s did not exist", stream)
		} else if !should && known {
			t.Fatalf("%s did exist", stream)
		}
	}

	assertStream(t, "OVERRIDE", true)
	assertStream(t, "KV_TEST", false)
}

func TestJetStreamStorage_Codec(t *testing.T) {
	store, srv, nc, _ := setupBasicTestBucket(t, WithTTL(time.Minute), WithCodec(base64Codec{}))
	defer srv.Shutdown()
	defer nc.Close()
	defer store.Close()

	seq, err := store.Put("hello.world", []byte("world"))
	if err != nil {
		t.Fatalf("put failed: %s", err)
	}

	stream, err := store.getOrLoadStream()
	if err != nil {
		t.Fatalf("stream load failed: %s", err)
	}

	if stream.MaxAge() != time.Minute {
		t.Fatalf("age was not 60s: %v", stream.MaxAge())
	}

	if stream.DuplicateWindow() != time.Minute {
		t.Fatalf("duplicate window is not 60s: %v", stream.DuplicateWindow())
	}

	msg, err := stream.ReadMessage(seq)
	if err != nil {
		t.Fatalf("read failed: %s", err)
	}

	if !bytes.Equal(msg.Data, []byte("d29ybGQ=")) {
		t.Fatalf("encoded string was not stored: %q", msg.Data)
	}

	if msg.Subject != "$KV.TEST.aGVsbG8=.d29ybGQ=" {
		t.Fatalf("subject was not encoded: %s", msg.Subject)
	}

	val, err := store.Get("hello.world")
	if err != nil {
		t.Fatalf("get failed: %s", err)
	}

	if val.Key() != "hello.world" {
		t.Fatalf("received invald key in entry: %v", val.Key())
	}

	assertEntryHasStringValue(t, val, "world")
}

func TestJetStreamStorage_WatchBucket(t *testing.T) {
	store, srv, nc, _ := setupBasicTestBucket(t)
	defer srv.Shutdown()
	defer nc.Close()

	for m := 0; m < 10; m++ {
		_, err := store.Put("key", []byte(strconv.Itoa(m)))
		if err != nil {
			t.Fatalf("put failed: %s", err)
		}
	}

	status, _ := store.Status()
	if status.Values() != 5 {
		t.Fatalf("expected 5 messages got %d", status.Values())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	watch, err := store.Watch(ctx, "")
	if err != nil {
		t.Fatalf("watch failed: %s", err)
	}

	cnt := 9
	kills := 0
	var latest Entry

	// similar to the Watch() test but now we make sure we get old values and new values and that
	// even after a consumer outage we do not get duplicate messages over the channel
	for {
		select {
		case r, ok := <-watch.Channel():
			if !ok {
				// channel is closed: check we got the last message we sent
				assertEntryHasStringValue(t, latest, strconv.Itoa(cnt))

				if kills == 0 {
					t.Fatalf("did not kill the consumer during the test")
				}

				return
			}

			latest = r

			// value should be that from the last pass through the watch loop or the initial from the warm up for loop
			assertEntryHasStringValue(t, latest, strconv.Itoa(cnt))

			// after 10 the test is done, close the watch, channel close handler
			// will verify we got what we needed
			if cnt == 20 {
				watch.Close()
				continue
			}

			cnt++

			_, err = store.Put("key", []byte(strconv.Itoa(cnt)))
			if err != nil {
				t.Fatalf("put failed: %s", err)
			}

			// after a few we kill the consumer to test recover
			if cnt == 15 {
				kills++
				watch.(*jsWatch).cons.Delete()
			}

		case <-ctx.Done():
			t.Fatalf("timeout running test")
		}
	}
}

func TestJetStreamStorage_WatchEndNotify(t *testing.T) {
	store, srv, nc, _ := setupBasicTestBucket(t)
	defer srv.Shutdown()
	defer nc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	watch, err := store.Watch(ctx, "x.>")
	if err != nil {
		t.Fatalf(": %s", err)
	}
	e := <-watch.Channel()
	if e != nil {
		t.Fatalf("expected nil value got %+v", e)
	}
	_, err = store.Put("x.y", []byte("hello"))
	if err != nil {
		t.Fatalf(": %s", err)
	}
	e = <-watch.Channel()
	if string(e.Value()) != "hello" {
		t.Fatalf("Expected hello got: %q", e.Value())
	}
	if e.Key() != "x.y" {
		t.Fatalf("Expected key x.y got: %v", e.Key())
	}
}

func TestJetStreamStorage_Watch(t *testing.T) {
	store, srv, nc, _ := setupBasicTestBucket(t)
	defer srv.Shutdown()
	defer nc.Close()

	for m := 0; m < 10; m++ {
		_, err := store.Put("key", []byte(strconv.Itoa(m)))
		if err != nil {
			t.Fatalf("put failed: %s", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	watch, err := store.Watch(ctx, "key")
	if err != nil {
		t.Fatalf("watch failed: %s", err)
	}

	status, _ := store.Status()
	if status.Values() != 5 {
		t.Fatalf("expected 5 messages got %d", status.Values())
	}

	cnt := 9
	kills := 0
	var latest Entry

	for {
		select {
		case r, ok := <-watch.Channel():
			if !ok {
				// channel is closed: check we got the last message we sent
				assertEntryHasStringValue(t, latest, strconv.Itoa(cnt))

				if kills == 0 {
					t.Fatalf("did not kill the consumer during the test")
				}

				return
			}

			latest = r

			// value should be that from the last pass through the watch loop or the initial from the warm up for loop
			assertEntryHasStringValue(t, latest, strconv.Itoa(cnt))

			// we should always only get the latest value
			if r.Delta() != 0 {
				t.Fatalf("received non latest message %+v", r)
			}

			// after 10 the test is done, close the watch, channel close handler
			// will verify we got what we needed
			if cnt == 20 {
				watch.Close()
				continue
			}

			cnt++

			_, err = store.Put("key", []byte(strconv.Itoa(cnt)))
			if err != nil {
				t.Fatalf("put failed: %s", err)
			}

			// after a few we kill the consumer to test recover
			if cnt == 15 {
				kills++
				watch.(*jsWatch).cons.Delete()
			}

		case <-ctx.Done():
			t.Fatalf("timeout running test")
		}
	}
}

func TestJetStreamStorage_CompactAndPurge(t *testing.T) {
	store, srv, nc, _ := setupBasicTestBucket(t)
	defer srv.Shutdown()
	defer nc.Close()

	for i := 0; i < 5; i++ {
		_, err := store.Put("x", []byte(strconv.Itoa(i)))
		if err != nil {
			t.Fatalf("put failed: %s", err)
		}

		_, err = store.Put("y", []byte(strconv.Itoa(i)))
		if err != nil {
			t.Fatalf("put failed: %s", err)
		}
	}

	checkCount := func(t *testing.T, subj string, expect uint64) {
		c, err := store.stream.NewConsumer(jsm.DurableName("X"), jsm.FilterStreamBySubject(subj))
		if err != nil {
			t.Fatalf("consumer failed: %s", err)
		}
		defer c.Delete()

		state, _ := c.LatestState()
		cnt := state.NumPending + state.Delivered.Consumer
		if cnt != expect {
			t.Fatalf("expected 5 messages got: %d", cnt)
		}
	}

	checkCount(t, store.subjectForKey("x"), 5)

	err := store.Purge()
	if err != nil {
		t.Fatalf("purge failed: %s", err)
	}

	checkCount(t, store.subjectForKey("x"), 0)
	checkCount(t, store.subjectForKey("y"), 0)
	checkCount(t, store.subjectForKey("z"), 0)
}

func TestJetStreamStorage_Delete(t *testing.T) {
	store, srv, nc, _ := setupBasicTestBucket(t)
	defer srv.Shutdown()
	defer nc.Close()

	store.Put("x", []byte("x"))
	store.Put("x", []byte("y"))
	store.Put("x", []byte("z"))
	store.Put("y", []byte("y"))
	store.Put("z", []byte("y"))

	res, err := store.Get("x")
	if err != nil {
		t.Fatalf("get failed: %s", err)
	}
	assertEntryHasStringValue(t, res, "z")

	err = store.Delete("x")
	if err != nil {
		t.Fatalf("delete failed: %s", err)
	}

	_, err = store.Get("x")
	if err != ErrUnknownKey {
		t.Fatalf("expected unknown key error: %v", err)
	}

	res, err = store.Get("z")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	assertEntryHasStringValue(t, res, "y")

	res, err = store.Get("y")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	assertEntryHasStringValue(t, res, "y")

	err = store.Delete("x.>")
	if err != ErrInvalidKey {
		t.Fatalf("Expected invalid key error doing Delete() on wildcard: %v", err)
	}
}

func TestJetStreamStorage_Status(t *testing.T) {
	store, srv, nc, _ := setupBasicTestBucket(t)
	defer srv.Shutdown()
	defer nc.Close()

	store.Put("x", []byte("y"))
	store.Put("y", []byte("y"))
	store.Put("z", []byte("y"))

	status, err := store.Status()
	if err != nil {
		t.Fatalf("status failed: %s", err)
	}

	if status.Values() != 3 {
		t.Fatalf("invalid values %d", status.Values())
	}

	if ok, failed := status.Replicas(); ok != 1 || failed != 0 {
		t.Fatalf("invalid replicas ok: %d failed: %d", ok, failed)
	}

	if status.BucketLocation() != "unknown" {
		t.Fatalf("invalid cluster %q", status.BucketLocation())
	}

	if status.History() != 5 {
		t.Fatalf("invalid history %d", status.History())
	}
}

func TestJetStreamStorage_Put(t *testing.T) {
	store, srv, nc, _ := setupBasicTestBucket(t)
	defer srv.Shutdown()
	defer nc.Close()

	for i := uint64(1); i <= 100; i++ {
		seq, err := store.Put("hello", []byte("world"))
		if err != nil {
			t.Fatalf("put failed: %s", err)
		}

		if seq != i {
			t.Fatalf("invalid sequence %d received", seq)
		}

		res, err := store.Get("hello")
		if err != nil {
			t.Fatalf("get failed: %s", err)
		}

		if res.Key() != "hello" {
			t.Fatalf("incorrect key: %s", res.Key())
		}
		assertEntryHasStringValue(t, res, "world")
		if res.Sequence() != seq {
			t.Fatalf("incorrect seq: %d", res.Sequence())
		}
		if res.Delta() != 0 {
			t.Fatalf("incorrect delta: %d", res.Delta())
		}

		// within reasonable grace period
		if res.Created().Before(time.Now().Add(-1 * time.Second)) {
			t.Fatalf("incorrect create time: %v", res.Created())
		}
	}

	// test . in keys
	_, err := store.Put("x.y.hello", []byte("world.world.world"))
	if err != nil {
		t.Fatalf("put failed: %s", err)
	}
	res, err := store.Get("x.y.hello")
	if err != nil {
		t.Fatalf("get failed: %s", err)
	}
	if string(res.Value()) != "world.world.world" {
		t.Fatalf("got invalid value %q", res.Value())
	}
	if res.Key() != "x.y.hello" {
		t.Fatalf("got invalid key %q", res.Key())
	}

	seq, err := store.Put("hello", []byte("world"))
	if err != nil {
		t.Fatalf("put failed: %s", err)
	}

	_, err = store.Put("x.>", []byte("world"))
	if err == nil {
		t.Fatalf("Expected error doing Put() on wildcard key")
	}

	// test only if last value seq
	_, err = store.Put("hello", []byte("world"), OnlyIfLastValueSequence(seq-1))
	if err != nil {
		apiErr, ok := err.(api.ApiError)
		if ok {
			if apiErr.NatsErrorCode() != 10071 {
				t.Fatalf("Expected error 10071, got %v", apiErr)
			}
		} else {
			t.Fatalf("Expected err 10071 got, got generic error: %v", err)
		}
	}

	_, err = store.Put("hello", []byte("world"), OnlyIfLastValueSequence(seq))
	if err != nil {
		t.Fatalf("Expected correct sequence put to succeed: %s", err)
	}
}

func TestJetStreamStorage_History(t *testing.T) {
	store, srv, nc, _ := setupBasicTestBucket(t, WithHistory(5))
	defer srv.Shutdown()
	defer nc.Close()

	publish := uint64(5)

	for i := uint64(1); i <= publish; i++ {
		_, err := store.Put("k", []byte(fmt.Sprintf("val%d", i)))
		if err != nil {
			t.Fatalf("put failed: %s", err)
		}
	}

	hist, err := store.History(context.Background(), "k")
	if err != nil {
		t.Fatalf("history failed: %s", err)
	}

	if len(hist) != int(publish) {
		t.Fatalf("expected %d history got %d", publish, len(hist))
	}

	for i, r := range hist {
		assertEntryHasStringValue(t, r, fmt.Sprintf("val%d", i+1))
	}

	// invalid keys
	_, err = store.History(context.Background(), ">")
	if err != ErrInvalidKey {
		t.Fatalf("Expected invalid key error doing History() for wildcard: %s", err)
	}

	// unknown key
	_, err = store.History(context.Background(), "x.y")
	if err != ErrUnknownKey {
		t.Fatalf("Expected unknown key error doing History() for unknown key: %s", err)
	}

}

func TestJetStreamStorage_Get(t *testing.T) {
	store, srv, nc, _ := setupBasicTestBucket(t)
	defer srv.Shutdown()
	defer nc.Close()

	for i := uint64(1); i <= 1000; i++ {
		_, err := store.Put(fmt.Sprintf("k%d", i), []byte(fmt.Sprintf("val%d", i)))
		if err != nil {
			t.Fatalf("put failed: %s", err)
		}
	}

	state, err := store.stream.State()
	if err != nil {
		t.Fatalf("state failed: %s", err)
	}
	if state.Msgs != 1000 {
		t.Fatalf("expected 1000 messages got %d", state.Msgs)
	}

	for i := uint64(1); i <= 1000; i++ {
		key := fmt.Sprintf("k%d", i)
		res, err := store.Get(key)
		if err != nil {
			t.Fatalf("get failed: %s", err)
		}

		if res.Key() != key {
			t.Fatalf("invalid key: %s", res.Key())
		}
		assertEntryHasStringValue(t, res, fmt.Sprintf("val%d", i))
		if res.Sequence() != i {
			t.Fatalf("invalid sequence: %d", res.Sequence())
		}
	}

	res, err := store.Get("UNKNOWN")
	if err == nil {
		t.Fatalf("expected an error")
	}
	if res != nil {
		t.Fatalf("expected nil result")
	}

	_, err = store.Get(">")
	if err == nil {
		t.Fatalf("expected error dong Get() on wildcard")
	}
}

func TestJetStreamStorage_Close(t *testing.T) {
	store, srv, nc, _ := setupBasicTestBucket(t)
	defer srv.Shutdown()
	defer nc.Close()

	if store.stream == nil {
		t.Fatalf("load failed")
	}

	err := store.Close()
	if err != nil {
		t.Fatalf("close failed: %s", err)
	}

	if store.stream != nil {
		t.Fatalf("close failed, stream is not nil")
	}
}

func TestJetStreamStorage_UnknownBucket(t *testing.T) {
	srv, nc, _ := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

	kv, err := NewClient(nc, "MISSING")
	if err != nil {
		t.Fatalf("new failed: %s", err)
	}

	_, err = kv.Status()
	if err != ErrUnknownBucket {
		t.Fatalf("Unexpected error: %s", err)
	}
}

func TestJetStreamStorage_CreateBucket(t *testing.T) {
	srv, nc, _ := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

	opts, _ := newOpts(WithHistory(5), WithTTL(24*time.Hour))

	st, err := newJetStreamStorage("TEST", nc, opts)
	if err != nil {
		t.Fatalf("store create failed: %s", err)
	}

	store := st.(*jetStreamStorage)
	err = store.CreateBucket()
	if err != nil {
		t.Fatalf("create failed: %s", err)
	}

	if store.stream == nil {
		t.Fatalf("no stream stored")
	}

	if store.stream.Name() != "KV_TEST" {
		t.Fatalf("invalid stream name %s", store.stream.Name())
	}

	if store.stream.MaxAge() != 24*time.Hour {
		t.Fatalf("invalid stream retention: %v", store.stream.MaxAge())
	}

	if store.stream.MaxMsgsPerSubject() != 5 {
		t.Fatalf("invalid stream retention: %v", store.stream.MaxMsgsPerSubject())
	}
}

func TestJetStreamStorage_Destroy(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Close()

	opts, _ := newOpts()
	store, err := newJetStreamStorage("TEST", nc, opts)
	if err != nil {
		t.Fatalf("store create failed: %s", err)
	}

	err = store.CreateBucket()
	if err != nil {
		t.Fatalf("create failed: %s", err)
	}

	err = store.Destroy()
	if err != nil {
		t.Fatalf("destroy failed: %s", err)
	}

	known, err := mgr.IsKnownStream("KV_TEST")
	if err != nil {
		t.Fatalf("known failed: %s", err)
	}

	if known {
		t.Fatalf("stream existed after Destroy()")
	}
}

func BenchmarkJetStreamPut(b *testing.B) {
	store, srv, nc, _ := setupBasicTestBucket(b)
	defer srv.Shutdown()
	defer nc.Close()
	defer store.Close()

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		key := fmt.Sprintf("k%d", n%10)
		val := strconv.Itoa(n)
		_, err := store.Put(key, []byte(val))
		if err != nil {
			b.Fatalf("put failed: %s", err)
		}
	}
}

func BenchmarkReadCacheGet(b *testing.B) {
	b.StopTimer()
	store, srv, nc, _ := setupBasicTestBucket(b)
	defer srv.Shutdown()
	defer nc.Close()
	defer store.Close()

	cached, err := newReadCache(store, store.log)
	if err != nil {
		b.Fatalf("cache setup failed: %s", err)
	}
	defer cached.Close()

	seq, err := cached.Put("hello", []byte("world"))
	if err != nil {
		b.Fatalf("put failed: %s", err)
	}

	var res Entry

	b.StartTimer()

	for n := 0; n < b.N; n++ {
		res, err = cached.Get("hello")
		if err != nil {
			b.Fatalf("get failed: %s", err)
		}
		if res.Sequence() != seq {
			b.Fatalf("got wrong sequence: %d", res.Sequence())
		}
	}
}

func BenchmarkJetStreamGet(b *testing.B) {
	b.StopTimer()
	store, srv, nc, _ := setupBasicTestBucket(b)
	defer srv.Shutdown()
	defer nc.Close()
	defer store.Close()

	seq, err := store.Put("hello", []byte("world"))
	if err != nil {
		b.Fatalf("put failed: %s", err)
	}
	b.StartTimer()

	var res Entry
	for n := 0; n < b.N; n++ {
		res, err = store.Get("hello")
		if err != nil {
			b.Fatalf("get failed: %s", err)
		}
		if res.Sequence() != seq {
			b.Fatalf("got wrong sequence: %d", res.Sequence())
		}
	}
}

func BenchmarkJetStreamPutGet(b *testing.B) {
	b.StopTimer()
	store, srv, nc, _ := setupBasicTestBucket(b)
	defer srv.Shutdown()
	defer nc.Close()
	defer store.Close()

	b.StartTimer()

	for n := 0; n < b.N; n++ {
		key := fmt.Sprintf("k%d", n%10)
		_, err := store.Put(key, []byte(strconv.Itoa(n)))
		if err != nil {
			b.Fatalf("put failed: %s", err)
		}

		_, err = store.Get(key)
		if err != nil {
			b.Fatalf("get failed: %s", err)
		}
	}
}

func startJSServer(t BOrT) (*natsd.Server, *nats.Conn, *jsm.Manager) {
	t.Helper()

	d, err := ioutil.TempDir("", "jstest")
	if err != nil {
		t.Fatalf("temp dir could not be made: %s", err)
	}

	opts := &natsd.Options{
		ServerName: "test.example.net",
		JetStream:  true,
		StoreDir:   d,
		Port:       -1,
		Host:       "localhost",
		LogFile:    "/tmp/server.log",
		// Trace:        true,
		// TraceVerbose: true,
		Cluster: natsd.ClusterOpts{Name: "gotest"},
	}

	s, err := natsd.NewServer(opts)
	if err != nil {
		t.Fatalf("server start failed: %s", err)
	}

	go s.Start()
	if !s.ReadyForConnections(10 * time.Second) {
		t.Fatalf("nats server did not start")
	}

	// s.ConfigureLogger()

	nc, err := nats.Connect(s.ClientURL(), nats.UseOldRequestStyle(), nats.MaxReconnects(100))
	if err != nil {
		t.Fatalf("client start failed: %s", err)
	}

	mgr, err := jsm.New(nc, jsm.WithTimeout(time.Second))
	if err != nil {
		t.Fatalf("manager creation failed: %s", err)
	}

	return s, nc, mgr
}
