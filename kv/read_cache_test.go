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
	"testing"
	"time"

	"github.com/nats-io/jsm.go"
	natsd "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func setupBasicCachedTestBucket(t BOrT) (*readCache, *jetStreamStorage, *natsd.Server, *nats.Conn, *jsm.Manager) {
	store, srv, nc, mgr := setupBasicTestBucket(t)
	cache, err := newReadCache(store, store.log)
	if err != nil {
		t.Fatalf("cache failed: %s", err)
	}

	return cache, store, srv, nc, mgr
}

func TestReadCache_GetPut(t *testing.T) {
	cache, store, srv, nc, _ := setupBasicCachedTestBucket(t)
	defer srv.Shutdown()
	defer nc.Close()
	defer store.Close()
	defer cache.Close()

	expectCache := func(t *testing.T, expect int) {
		t.Helper()
		cache.mu.Lock()
		defer cache.mu.Unlock()

		if len(cache.cache) > expect {
			t.Fatalf("cache has %d expected %d", len(cache.cache), expect)
		}
	}

	assertCacheReady := func(t *testing.T) {
		t.Helper()
		cache.mu.Lock()
		defer cache.mu.Unlock()

		if !cache.ready {
			t.Fatalf("cache is not ready")
		}
	}
	_, err := cache.Get("missing")
	if err == nil {
		t.Fatalf("expected an error got none")
	}

	expectCache(t, 0)
	assertCacheReady(t)

	_, err = cache.Put("hello", []byte("world"))
	if err != nil {
		t.Fatalf("put failed: %s", err)
	}

	expectCache(t, 0)
	assertCacheReady(t)

	_, err = cache.Get("hello")
	if err != nil {
		t.Fatalf("get failed: %s", err)
	}

	// let the watch get from the consumer etc
	time.Sleep(20 * time.Millisecond)

	expectCache(t, 1)

	_, err = cache.Get("hello")
	if err != nil {
		t.Fatalf("get failed: %s", err)
	}

	expectCache(t, 1)

	_, err = cache.Put("hello", []byte("wrld"))
	if err != nil {
		t.Fatalf("put failed: %s", err)
	}

	// let the watch get from the consumer etc
	time.Sleep(20 * time.Millisecond)

	expectCache(t, 1)

	val, err := cache.Get("hello")
	if err != nil {
		t.Fatalf("get failed: %s", err)
	}

	assertResultHasStringValue(t, val, "wrld")
}

func TestReadCache_Delete(t *testing.T) {
	cache, store, srv, nc, _ := setupBasicCachedTestBucket(t)
	defer srv.Shutdown()
	defer nc.Close()
	defer store.Close()
	defer cache.Close()

	expectCache := func(t *testing.T, expect int) {
		t.Helper()

		cache.mu.Lock()
		defer cache.mu.Unlock()

		if len(cache.cache) > expect {
			t.Fatalf("cache has %d expected %d", len(cache.cache), expect)
		}
	}

	cache.Put("x", []byte("y"))
	cache.Put("y", []byte("y"))

	time.Sleep(20 * time.Millisecond)

	expectCache(t, 2)

	cache.Delete("x")
	expectCache(t, 1)

	_, err := cache.Get("x")
	if err == nil {
		t.Fatalf("get succeeded")
	}

	_, err = cache.Get("y")
	if err != nil {
		t.Fatalf("'y' get failed: %s", err)
	}

	// now do a delete from the backend directly and wait for watch to
	// deliver the delete operation, the cache should then not have this value
	err = cache.backend.Delete("y")
	if err != nil {
		t.Fatalf("delete failed: %s", err)
	}

	time.Sleep(20 * time.Millisecond)

	expectCache(t, 0)
}

func TestReadCache_Purge(t *testing.T) {
	cache, store, srv, nc, _ := setupBasicCachedTestBucket(t)
	defer srv.Shutdown()
	defer nc.Close()
	defer store.Close()
	defer cache.Close()

	expectCache := func(t *testing.T, expect int) {
		t.Helper()
		cache.mu.Lock()
		defer cache.mu.Unlock()

		if len(cache.cache) > expect {
			t.Fatalf("cache has %d expected %d", len(cache.cache), expect)
		}
	}

	cache.Put("x", []byte("y"))
	cache.Put("y", []byte("y"))

	time.Sleep(20 * time.Millisecond)

	expectCache(t, 2)

	cache.Purge()
	expectCache(t, 0)

	_, err := cache.Get("x")
	if err == nil {
		t.Fatalf("get succeeded")
	}
}

func TestReadCache_Destroy(t *testing.T) {
	cache, store, srv, nc, _ := setupBasicCachedTestBucket(t)
	defer srv.Shutdown()
	defer nc.Close()
	defer store.Close()
	defer cache.Close()

	expectCache := func(t *testing.T, expect int) {
		t.Helper()
		cache.mu.Lock()
		defer cache.mu.Unlock()

		if len(cache.cache) > expect {
			t.Fatalf("cache has %d expected %d", len(cache.cache), expect)
		}
	}

	cache.Put("x", []byte("y"))
	cache.Put("y", []byte("y"))

	time.Sleep(20 * time.Millisecond)

	expectCache(t, 2)

	cache.Destroy()
	expectCache(t, 0)

	_, err := cache.Get("x")
	if err == nil {
		t.Fatalf("get succeeded")
	}
}
