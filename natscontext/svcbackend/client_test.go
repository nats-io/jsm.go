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

package svcbackend_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	nserver "github.com/nats-io/nats-server/v2/server"
	nstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"

	"github.com/nats-io/jsm.go/natscontext"
	"github.com/nats-io/jsm.go/natscontext/backendtest"
	"github.com/nats-io/jsm.go/natscontext/svcbackend"
	"github.com/nats-io/jsm.go/natscontext/svcbackend/internal/testserver"
)

// startNATS brings up an in-process nats-server and returns a
// connection plus the server handle. Cleanup closes the connection
// and shuts the server down in the right order.
func startNATS(t *testing.T) (*nats.Conn, *nserver.Server) {
	t.Helper()

	srv := nstest.RunRandClientPortServer()
	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		srv.Shutdown()
		t.Fatalf("connect: %v", err)
	}

	t.Cleanup(func() {
		nc.Close()
		srv.Shutdown()
	})

	return nc, srv
}

// startTestServer spins up a fresh nats-server, a MemoryBackend, and
// a testserver.Server wrapping it. It returns everything the test
// needs; t.Cleanup stops and closes the right pieces.
func startTestServer(t *testing.T, prefix string, backend natscontext.Backend) (*nats.Conn, *testserver.Server) {
	t.Helper()

	nc, _ := startNATS(t)

	ts, err := testserver.New(nc, backend, prefix)
	if err != nil {
		t.Fatalf("testserver.New: %v", err)
	}
	t.Cleanup(func() { _ = ts.Stop() })

	return nc, ts
}

func TestClient_Contract(t *testing.T) {
	backendtest.RunBackendContract(t, func(t *testing.T) natscontext.Backend {
		backend := natscontext.NewMemoryBackend()
		nc, ts := startTestServer(t, "", backend)

		c, err := svcbackend.NewClient(nc, svcbackend.WithServerKey(ts.PublicKey()))
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		t.Cleanup(func() { _ = c.Close() })

		return c
	})
}

// TestClient_IsNotSelector locks in the contract that the remote
// client deliberately does not implement natscontext.Selector.
// Selection is composed at Registry time with a local selector;
// see the Selection section of the package README.
func TestClient_IsNotSelector(t *testing.T) {
	nc, _ := startNATS(t)
	c, err := svcbackend.NewClient(nc, svcbackend.WithServerKey("XAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	_, ok := any(c).(natscontext.Selector)
	if ok {
		t.Fatalf("svcbackend.Client unexpectedly implements natscontext.Selector")
	}
}

func TestClient_NonDefaultPrefix(t *testing.T) {
	const prefix = "test.prefix.v1"

	backend := natscontext.NewMemoryBackend()
	nc, ts := startTestServer(t, prefix, backend)

	c, err := svcbackend.NewClient(nc,
		svcbackend.WithSubjectPrefix(prefix),
		svcbackend.WithServerKey(ts.PublicKey()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()
	want := []byte("hello")
	err = c.Save(ctx, "alpha", want)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := c.Load(ctx, "alpha")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("mismatch: got %q want %q", got, want)
	}
}

// errorBackend is a Backend implementation whose methods return a
// fixed error. Used to drive the error round-trip test so every
// sentinel is exercised end-to-end.
type errorBackend struct{ err error }

func (e *errorBackend) Load(_ context.Context, _ string) ([]byte, error) { return nil, e.err }
func (e *errorBackend) Save(_ context.Context, _ string, _ []byte) error { return e.err }
func (e *errorBackend) Delete(_ context.Context, _ string) error         { return e.err }
func (e *errorBackend) List(_ context.Context) ([]string, error)         { return nil, e.err }

func TestClient_ErrorRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		sentinel error
	}{
		{"not_found", natscontext.ErrNotFound},
		{"already_exists", natscontext.ErrAlreadyExists},
		{"active_context", natscontext.ErrActiveContext},
		{"read_only", natscontext.ErrReadOnly},
		{"conflict", natscontext.ErrConflict},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			backend := &errorBackend{err: fmt.Errorf("%w: injected", tc.sentinel)}
			nc, ts := startTestServer(t, "", backend)

			c, err := svcbackend.NewClient(nc, svcbackend.WithServerKey(ts.PublicKey()))
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			t.Cleanup(func() { _ = c.Close() })

			ctx := context.Background()
			_, err = c.Load(ctx, "alpha")
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("Load: expected %v, got %v", tc.sentinel, err)
			}
		})
	}
}

func TestClient_ValidateNameFastFail(t *testing.T) {
	// A disconnected NATS connection guarantees any actual network
	// call fails loudly. Getting ErrInvalidName back means the client
	// rejected the name before touching the wire.
	nc, _ := startNATS(t)
	nc.Close()

	c, err := svcbackend.NewClient(nc,
		svcbackend.WithServerKey("XAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()

	// Covers both tiers of rejection: names the core natscontext
	// validator blocks (empty, "/"), and names the core accepts for
	// historical compat that the svcbackend wire protocol cannot
	// carry (whitespace, ".", "*", ">", control chars). All must
	// surface ErrInvalidName before any wire traffic.
	badNames := []string{
		"",         // empty — rejected by core
		"bad/name", // path separator — rejected by core
		"..",       // bare ".." — rejected by core
		"bad name", // whitespace — svcbackend-tier rejection
		"ngs.js",   // dot — svcbackend-tier rejection (historical core-valid name)
		"foo.bar",  // dot — svcbackend-tier rejection
		"foo*bar",  // NATS wildcard — svcbackend-tier rejection
		"foo>bar",  // NATS wildcard — svcbackend-tier rejection
		"foo\tbar", // control — svcbackend-tier rejection
	}

	for _, name := range badNames {
		t.Run(fmt.Sprintf("Save(%q)", name), func(t *testing.T) {
			err := c.Save(ctx, name, []byte("x"))
			if !errors.Is(err, natscontext.ErrInvalidName) {
				t.Fatalf("expected ErrInvalidName, got %v", err)
			}
		})
		t.Run(fmt.Sprintf("Load(%q)", name), func(t *testing.T) {
			_, err := c.Load(ctx, name)
			if !errors.Is(err, natscontext.ErrInvalidName) {
				t.Fatalf("expected ErrInvalidName, got %v", err)
			}
		})
		t.Run(fmt.Sprintf("Delete(%q)", name), func(t *testing.T) {
			err := c.Delete(ctx, name)
			if !errors.Is(err, natscontext.ErrInvalidName) {
				t.Fatalf("expected ErrInvalidName, got %v", err)
			}
		})
	}
}

func TestClient_ServerKeyDiscovery(t *testing.T) {
	backend := natscontext.NewMemoryBackend()
	nc, ts := startTestServer(t, "", backend)

	c, err := svcbackend.NewClient(nc)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if got := ts.SysXKeyCalls(); got != 0 {
		t.Fatalf("pre-call SysXKeyCalls = %d, want 0", got)
	}

	ctx := context.Background()

	err = c.Save(ctx, "alpha", []byte("x"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	first := ts.SysXKeyCalls()
	if first != 1 {
		t.Fatalf("post-first-save SysXKeyCalls = %d, want 1", first)
	}

	_, err = c.Load(ctx, "alpha")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	err = c.Save(ctx, "alpha", []byte("y"))
	if err != nil {
		t.Fatalf("save 2: %v", err)
	}

	if got := ts.SysXKeyCalls(); got != first {
		t.Fatalf("post-subsequent-calls SysXKeyCalls = %d, want %d (no additional fetches)", got, first)
	}
}

func TestClient_XKeyRotation(t *testing.T) {
	backend := natscontext.NewMemoryBackend()
	nc, ts := startTestServer(t, "", backend)

	c, err := svcbackend.NewClient(nc, svcbackend.WithServerKey(ts.PublicKey()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()
	err = c.Save(ctx, "alpha", []byte("v1"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	baseline := ts.SysXKeyCalls()

	err = ts.Rotate()
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}

	data, err := c.Load(ctx, "alpha")
	if err != nil {
		t.Fatalf("load after rotate: %v", err)
	}
	if !bytes.Equal(data, []byte("v1")) {
		t.Fatalf("load after rotate: got %q want v1", data)
	}

	got := ts.SysXKeyCalls() - baseline
	if got != 1 {
		t.Fatalf("expected exactly 1 sys.xkey refetch after rotation, got %d", got)
	}

	err = c.Save(ctx, "alpha", []byte("v2"))
	if err != nil {
		t.Fatalf("save after rotate: %v", err)
	}

	if extra := ts.SysXKeyCalls() - baseline; extra != 1 {
		t.Fatalf("expected 1 total sys.xkey refetch, got %d", extra)
	}
}

// TestClient_SaveRotationTransparent verifies that when the server
// rotates its long-term xkey between a Load (which primed the cache)
// and a Save (which seals to the cached pub), the Save succeeds
// transparently and does exactly one sys.xkey refetch.
func TestClient_SaveRotationTransparent(t *testing.T) {
	backend := natscontext.NewMemoryBackend()
	nc, ts := startTestServer(t, "", backend)

	c, err := svcbackend.NewClient(nc, svcbackend.WithServerKey(ts.PublicKey()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()

	err = c.Save(ctx, "alpha", []byte("v1"))
	if err != nil {
		t.Fatalf("save v1: %v", err)
	}

	baseline := ts.SysXKeyCalls()

	err = ts.Rotate()
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}

	err = c.Save(ctx, "alpha", []byte("v2"))
	if err != nil {
		t.Fatalf("save after rotate: %v", err)
	}

	if got := ts.SysXKeyCalls() - baseline; got != 1 {
		t.Fatalf("expected exactly 1 sys.xkey refetch on save rotation, got %d", got)
	}

	// Load through the server to confirm the new payload landed.
	got, err := c.Load(ctx, "alpha")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !bytes.Equal(got, []byte("v2")) {
		t.Fatalf("payload: got %q want v2", got)
	}
}

// TestClient_SaveStaleKeyShortCircuit verifies that if sys.xkey
// returns the same pub after a stale_key retry (buggy or caching
// sys.xkey responder) the client surfaces a dedicated error rather
// than retrying forever or blaming "server xkey refresh".
func TestClient_SaveStaleKeyShortCircuit(t *testing.T) {
	nc, _ := startNATS(t)

	// Any real curve key works — it just needs to be a value the
	// client will accept as a recipient pub. The raw ctx.save
	// responder returns stale_key unconditionally, so the client
	// never actually seals-and-opens against this key.
	kp, err := nkeys.CreateCurveKeys()
	if err != nil {
		t.Fatalf("create curve keys: %v", err)
	}
	fixedPub, err := kp.PublicKey()
	if err != nil {
		t.Fatalf("public key: %v", err)
	}

	const prefix = "fixedkey.v1"

	// sys.xkey always advertises fixedPub. The ctx.save endpoint
	// always returns stale_key. With a constant-advertised key the
	// post-stale_key refetch comes back unchanged and the client
	// must short-circuit instead of retrying.
	_, err = nc.Subscribe(prefix+".sys.xkey", func(m *nats.Msg) {
		resp, _ := json.Marshal(svcbackend.SysXKeyResponse{XKeyPub: fixedPub})
		_ = m.Respond(resp)
	})
	if err != nil {
		t.Fatalf("subscribe sys.xkey: %v", err)
	}

	var saveCalls atomic.Int32
	_, err = nc.Subscribe(prefix+".ctx.save.*", func(m *nats.Msg) {
		saveCalls.Add(1)
		resp, _ := json.Marshal(svcbackend.SaveResponse{Error: &svcbackend.ErrorResponse{
			Code:    string(svcbackend.CodeStaleKey),
			Message: "open sealed request",
		}})
		_ = m.Respond(resp)
	})
	if err != nil {
		t.Fatalf("subscribe ctx.save: %v", err)
	}

	c, err := svcbackend.NewClient(nc, svcbackend.WithSubjectPrefix(prefix))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	err = c.Save(context.Background(), "alpha", []byte("x"))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "same stale key") {
		t.Fatalf("expected same-stale-key message, got %q", err.Error())
	}

	// The client should have issued exactly one ctx.save (the retry
	// is short-circuited after the same-key refetch check). If it
	// issued two, the short-circuit didn't fire.
	if got := saveCalls.Load(); got != 1 {
		t.Fatalf("ctx.save calls: got %d, want 1", got)
	}
}

func TestClient_Cancellation(t *testing.T) {
	// Start the NATS server WITHOUT a svcbackend service so any
	// request blocks forever. Cancel the context mid-flight and
	// assert the client returns promptly.
	nc, _ := startNATS(t)

	c, err := svcbackend.NewClient(nc,
		svcbackend.WithServerKey("XAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"),
		svcbackend.WithTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = c.List(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected cancellation error, got nil")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("List did not return promptly after cancel: %v", elapsed)
	}
}

func TestClient_TamperedCiphertext(t *testing.T) {
	// A raw NATS responder replaces the usual testserver for
	// ctx.load.* and answers with a well-formed LoadResponse whose
	// Sealed field is garbage. The sys.xkey endpoint still needs
	// to answer so WithServerKey is unused on purpose — we want
	// to verify the decrypt-failure path, not cache behavior.
	nc, _ := startNATS(t)

	backend := natscontext.NewMemoryBackend()

	ts, err := testserver.New(nc, backend, "")
	if err != nil {
		t.Fatalf("testserver.New: %v", err)
	}
	t.Cleanup(func() { _ = ts.Stop() })

	// Override ctx.load.* with a tampering responder on a higher
	// priority queue group. Actually micro already claims the
	// subject with its queue group, so subscribe on a *different*
	// subject tree and point the client at it.
	const tamperPrefix = "tamper.v1"

	_, err = nc.Subscribe(tamperPrefix+".sys.xkey", func(m *nats.Msg) {
		resp, _ := json.Marshal(svcbackend.SysXKeyResponse{XKeyPub: ts.PublicKey()})
		_ = m.Respond(resp)
	})
	if err != nil {
		t.Fatalf("subscribe sys.xkey: %v", err)
	}

	_, err = nc.Subscribe(tamperPrefix+".ctx.load.*", func(m *nats.Msg) {
		resp, _ := json.Marshal(svcbackend.LoadResponse{Sealed: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"})
		_ = m.Respond(resp)
	})
	if err != nil {
		t.Fatalf("subscribe ctx.load: %v", err)
	}

	c, err := svcbackend.NewClient(nc, svcbackend.WithSubjectPrefix(tamperPrefix))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	_, err = c.Load(context.Background(), "alpha")
	if err == nil {
		t.Fatalf("expected error from tampered response")
	}
	// The error MUST NOT be a natscontext sentinel — tampering is
	// a transport-layer failure, not a protocol-level one.
	for _, s := range []error{
		natscontext.ErrNotFound,
		natscontext.ErrAlreadyExists,
		natscontext.ErrInvalidName,
		natscontext.ErrActiveContext,
		natscontext.ErrReadOnly,
		natscontext.ErrConflict,
	} {
		if errors.Is(err, s) {
			t.Fatalf("tampered response unexpectedly mapped to sentinel %v: %v", s, err)
		}
	}
	if !strings.Contains(err.Error(), "decrypt") &&
		!strings.Contains(err.Error(), "open") &&
		!strings.Contains(err.Error(), "refresh") {
		t.Fatalf("error message should reference decrypt failure, got %q", err.Error())
	}
}

func TestClient_CloseIdempotent(t *testing.T) {
	nc, _ := startNATS(t)

	c, err := svcbackend.NewClient(nc, svcbackend.WithServerKey("XAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	err = c.Close()
	if err != nil {
		t.Fatalf("first Close: %v", err)
	}
	err = c.Close()
	if err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestClient_NilConn(t *testing.T) {
	_, err := svcbackend.NewClient(nil)
	if err == nil {
		t.Fatalf("expected error for nil nc")
	}
}

// TestClient_LocalSelectorComposition verifies the "shared storage,
// local selection" pattern: a Registry composed with svcbackend.Client
// as Backend and natscontext.FileSelector as Selector via
// WithLocalSelector MUST NOT send any sel.* traffic to the service.
// Selection is tracked entirely in the local file, and contexts still
// round-trip through the remote Backend.
func TestClient_LocalSelectorComposition(t *testing.T) {
	backend := natscontext.NewMemoryBackend()
	nc, ts := startTestServer(t, "", backend)

	c, err := svcbackend.NewClient(nc, svcbackend.WithServerKey(ts.PublicKey()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	var selTraffic atomic.Int32
	sub, err := nc.Subscribe(svcbackend.DefaultPrefix+".sel.>", func(*nats.Msg) {
		selTraffic.Add(1)
	})
	if err != nil {
		t.Fatalf("subscribe sel.>: %v", err)
	}
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	selectorDir := t.TempDir()
	reg := natscontext.NewRegistry(
		c,
		natscontext.WithSelector(natscontext.NewFileSelectorAt(selectorDir)),
	)

	bg := context.Background()
	nctx, err := natscontext.New("alpha", false, natscontext.WithServerURL("nats://a:4222"))
	if err != nil {
		t.Fatalf("new ctx: %v", err)
	}
	err = reg.Save(bg, nctx, "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	_, err = reg.Select(bg, "alpha")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	got, err := reg.Selected(bg)
	if err != nil {
		t.Fatalf("selected: %v", err)
	}
	if got != "alpha" {
		t.Fatalf("Selected = %q want alpha", got)
	}

	// Give any stray sel.* traffic a chance to round-trip before we
	// assert zero. A flushed subscription on the same connection is
	// enough — a message already in flight would have delivered.
	err = nc.Flush()
	if err != nil {
		t.Fatalf("nc.Flush: %v", err)
	}

	if n := selTraffic.Load(); n != 0 {
		t.Fatalf("sel.* traffic count = %d, want 0", n)
	}

	// And the local file backs the selection.
	data, err := os.ReadFile(filepath.Join(selectorDir, "nats", "context.txt"))
	if err != nil {
		t.Fatalf("read local context.txt: %v", err)
	}
	if strings.TrimSpace(string(data)) != "alpha" {
		t.Fatalf("local context.txt = %q want alpha", data)
	}
}

// Sanity check: the package lists the right external deps after
// PR2. Not exhaustive — just a smoke test that nothing leaked.
var _ = atomic.Int64{}
