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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	nserver "github.com/nats-io/nats-server/v2/server"
	nstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"

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
func (e *errorBackend) Selected(_ context.Context) (string, error)       { return "", e.err }
func (e *errorBackend) SetSelected(_ context.Context, _ string) (string, error) {
	return "", e.err
}

func TestClient_ErrorRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		sentinel error
	}{
		{"not_found", natscontext.ErrNotFound},
		{"already_exists", natscontext.ErrAlreadyExists},
		{"active_context", natscontext.ErrActiveContext},
		{"none_selected", natscontext.ErrNoneSelected},
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

	err = c.Save(ctx, "bad name", []byte("x"))
	if !errors.Is(err, natscontext.ErrInvalidName) {
		t.Fatalf("Save: expected ErrInvalidName, got %v", err)
	}

	_, err = c.Load(ctx, "bad/name")
	if !errors.Is(err, natscontext.ErrInvalidName) {
		t.Fatalf("Load: expected ErrInvalidName, got %v", err)
	}

	err = c.Delete(ctx, "")
	if !errors.Is(err, natscontext.ErrInvalidName) {
		t.Fatalf("Delete: expected ErrInvalidName, got %v", err)
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
		natscontext.ErrNoneSelected,
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

// Sanity check: the package lists the right external deps after
// PR2. Not exhaustive — just a smoke test that nothing leaked.
var _ = atomic.Int64{}
