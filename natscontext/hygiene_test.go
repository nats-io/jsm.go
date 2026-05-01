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

package natscontext_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/nats-io/jsm.go/natscontext"
)

// TestNewFromFileSaveRoundTrip verifies that a context loaded from a
// specific file path and then saved writes back to that same path
// rather than silently relocating itself under XDG_CONFIG_HOME. A
// WithServerURL option supplies the observable edit so the saved JSON
// demonstrably differs from what was on disk before.
func TestNewFromFileSaveRoundTrip(t *testing.T) {
	src := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	original := filepath.Join(src, "foo.json")
	err := os.WriteFile(original, []byte(`{"url":"nats://first:4222"}`), 0600)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	loaded, err := natscontext.NewFromFile(original,
		natscontext.WithServerURL("nats://second:4222"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ServerURL() != "nats://second:4222" {
		t.Fatalf("options didn't apply: got %q", loaded.ServerURL())
	}

	err = loaded.Save("")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	data, err := os.ReadFile(original)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if !bytes.Contains(data, []byte(`"url": "nats://second:4222"`)) {
		t.Fatalf("expected new URL persisted in %s; body=\n%s", original, data)
	}

	ghost := filepath.Join(xdg, "nats", "context", "foo.json")
	_, err = os.Stat(ghost)
	if err == nil {
		t.Fatalf("expected no XDG write but found %s", ghost)
	}
}

// TestNewFromFileSaveAsRename verifies the rename path: loading from a
// file and then Save("different-name") writes to XDG as the new name
// and leaves the original file untouched.
func TestNewFromFileSaveAsRename(t *testing.T) {
	src := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	original := filepath.Join(src, "foo.json")
	originalBody := []byte(`{"url":"nats://first:4222"}`)
	err := os.WriteFile(original, originalBody, 0600)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	loaded, err := natscontext.NewFromFile(original)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	err = loaded.Save("bar")
	if err != nil {
		t.Fatalf("save-as: %v", err)
	}

	unchanged, err := os.ReadFile(original)
	if err != nil {
		t.Fatalf("re-read original: %v", err)
	}
	if !bytes.Equal(unchanged, originalBody) {
		t.Fatalf("rename should have left original untouched, body now:\n%s", unchanged)
	}

	renamed := filepath.Join(xdg, "nats", "context", "bar.json")
	_, err = os.Stat(renamed)
	if err != nil {
		t.Fatalf("expected rename target at %s, got %v", renamed, err)
	}
}

// TestNewFromFileDotfile verifies that NewFromFile reads a dotfile
// basename like ".ctx" rather than silently returning a fresh default
// context. filepath.Ext(".ctx") returns the whole basename, so naive
// Ext-stripping produced an empty logical name and the Registry.Load
// path short-circuited to configureNewContext.
func TestNewFromFileDotfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ctx")
	err := os.WriteFile(path, []byte(`{"url":"nats://hidden:4222"}`), 0600)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	c, err := natscontext.NewFromFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.ServerURL() != "nats://hidden:4222" {
		t.Fatalf("dotfile path not read from disk: ServerURL=%q", c.ServerURL())
	}
}

// TestSaveAfterNameMutation covers the rename-via-c.Name idiom that
// pre-Backend-refactor code supported: load a context, mutate the
// exported Name field, then call Save(""). Save must treat the
// mutation as a rename and write to the new XDG slot rather than
// rejecting the save because the loaded SingleFileBackend was rooted
// at the original basename.
func TestSaveAfterNameMutation(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	ctxDir := filepath.Join(xdg, "nats", "context")
	err := os.MkdirAll(ctxDir, 0700)
	if err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	err = os.WriteFile(filepath.Join(ctxDir, "foo.json"), []byte(`{"url":"nats://orig:4222"}`), 0600)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	loaded, err := natscontext.New("foo", true)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	loaded.Name = "bar"
	err = loaded.Save("")
	if err != nil {
		t.Fatalf("save after rename: %v", err)
	}

	renamed := filepath.Join(ctxDir, "bar.json")
	_, err = os.Stat(renamed)
	if err != nil {
		t.Fatalf("expected renamed context at %s, got %v", renamed, err)
	}
}

// TestSaveRecreatesMissingContextDir covers the regression where a
// context loaded via New(..., true) routed Save through a
// SingleFileBackend, which skipped the MkdirAll performed by
// FileBackend.Save. Removing the context directory between load and
// save used to cause Save("") to fail with ENOENT instead of
// recreating the tree.
func TestSaveRecreatesMissingContextDir(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	ctxDir := filepath.Join(xdg, "nats", "context")
	err := os.MkdirAll(ctxDir, 0700)
	if err != nil {
		t.Fatalf("seed dir: %v", err)
	}
	err = os.WriteFile(filepath.Join(ctxDir, "foo.json"), []byte(`{"url":"nats://first:4222"}`), 0600)
	if err != nil {
		t.Fatalf("seed file: %v", err)
	}

	loaded, err := natscontext.New("foo", true, natscontext.WithServerURL("nats://second:4222"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	err = os.RemoveAll(ctxDir)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	err = loaded.Save("")
	if err != nil {
		t.Fatalf("save after dir removal: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(ctxDir, "foo.json"))
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if !bytes.Contains(data, []byte(`"url": "nats://second:4222"`)) {
		t.Fatalf("expected new URL persisted; body=\n%s", data)
	}
}

// TestNscErrorDoesNotLeakCreds stubs nsc with a binary that emits a
// decorated-JWT-looking string on stdout and then fails, and asserts
// the returned error does not echo the JWT.
func TestNscErrorDoesNotLeakCreds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-shim dispatch not portable on Windows")
	}

	const secretMarker = "SUPER-SECRET-JWT-SHOULD-NEVER-APPEAR"
	script := "#!/bin/sh\n" +
		"echo '" + secretMarker + "'\n" +
		"echo 'failed to look up user' 1>&2\n" +
		"exit 3\n"

	binDir := t.TempDir()
	err := os.WriteFile(filepath.Join(binDir, "nsc"), []byte(script), 0755)
	if err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, err := natscontext.New("nscfail", false,
		natscontext.WithCreds("nsc://op/acct/user"))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	_, err = ctx.NATSOptions()
	if err == nil {
		t.Fatal("expected error from failing nsc shim, got nil")
	}
	if strings.Contains(err.Error(), secretMarker) {
		t.Fatalf("error leaked stdout content: %v", err)
	}
}

// TestRedactedString checks that %v and %+v on a Context carrying a
// password, token and user seed never print those values.
func TestRedactedString(t *testing.T) {
	const secret = "this-is-a-secret-do-not-leak-47df2a"
	ctx, err := natscontext.New("redact", false,
		natscontext.WithUser("alice"),
		natscontext.WithPassword(secret),
		natscontext.WithToken(secret),
		natscontext.WithUserSeed(secret),
		natscontext.WithUserJWT(secret),
	)
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	verbs := []string{"%v", "%+v", "%#v", "%s"}
	for _, verb := range verbs {
		t.Run(verb, func(t *testing.T) {
			out := fmt.Sprintf(verb, ctx)
			if strings.Contains(out, secret) {
				t.Fatalf("%s output leaked secret: %s", verb, out)
			}
		})
	}
}

// TestRedactedStringZeroValue verifies that a zero-value context and
// a nil receiver both format without panicking.
func TestRedactedStringZeroValue(t *testing.T) {
	cases := []struct {
		name string
		ctx  *natscontext.Context
	}{
		{"nil receiver", nil},
		{"zero value", &natscontext.Context{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
				_ = fmt.Sprintf(verb, c.ctx)
			}
		})
	}
}

// TestRedactedStringServerURL covers credentials embedded in the
// connection URL itself (user:password@host and token@host forms,
// including comma-separated lists). String must redact the user-info
// portion so %v/%+v on a Context never echoes it back to logs.
func TestRedactedStringServerURL(t *testing.T) {
	const userSecret = "url-pass-do-not-leak-9ce4"
	const tokenSecret = "url-token-do-not-leak-3a17"

	cases := []struct {
		name string
		url  string
		leak []string
	}{
		{
			name: "user and password",
			url:  "nats://carol:" + userSecret + "@host:4222",
			leak: []string{userSecret},
		},
		{
			name: "token in user position",
			url:  "nats://" + tokenSecret + "@host:4222",
			leak: []string{tokenSecret},
		},
		{
			name: "comma separated mixed",
			url:  "nats://carol:" + userSecret + "@h1:4222,nats://" + tokenSecret + "@h2:4222",
			leak: []string{userSecret, tokenSecret},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, err := natscontext.New("urlredact", false, natscontext.WithServerURL(c.url))
			if err != nil {
				t.Fatalf("new: %v", err)
			}
			out := fmt.Sprintf("%v", ctx)
			for _, secret := range c.leak {
				if strings.Contains(out, secret) {
					t.Fatalf("String leaked %q: %s", secret, out)
				}
			}
		})
	}
}

// TestResolvedMaterialWiped asserts that resolver output is
// overwritten after NATSOptions returns, so references held by
// callers (intentional or not) cannot later read the original
// credential bytes.
func TestResolvedMaterialWiped(t *testing.T) {
	seed := []byte(makeFixtureCreds(t))
	capture := &capturingResolver{scheme: "wipe", payload: seed}

	reg := natscontext.NewRegistry(
		natscontext.NewFileBackendAt(t.TempDir()),
		natscontext.WithDefaultResolvers(),
		natscontext.WithCredentialResolver(capture),
	)

	nctx, err := reg.Load(context.Background(), "", natscontext.WithCreds("wipe://cred"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	_, err = nctx.NATSOptions()
	if err != nil {
		t.Fatalf("NATSOptions: %v", err)
	}

	captured := capture.issued()
	if len(captured) == 0 {
		t.Fatal("resolver was not invoked")
	}
	for _, b := range captured {
		if !allWiped(b) {
			t.Fatalf("resolver-returned slice was not wiped after NATSOptions: %q", b)
		}
	}
}

// capturingResolver hands out independent copies of a fixed payload
// on each call and retains references to every byte slice it emitted.
// The test inspects those references to verify the caller wiped
// resolver output after use.
type capturingResolver struct {
	scheme  string
	payload []byte

	mu     sync.Mutex
	issues [][]byte
}

func (r *capturingResolver) Schemes() []string { return []string{r.scheme} }

func (r *capturingResolver) Resolve(_ context.Context, _ string) ([]byte, error) {
	out := make([]byte, len(r.payload))
	copy(out, r.payload)

	r.mu.Lock()
	r.issues = append(r.issues, out)
	r.mu.Unlock()

	return out, nil
}

func (r *capturingResolver) issued() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]byte, len(r.issues))
	copy(out, r.issues)
	return out
}

// allWiped reports whether every byte in b matches wipeSlice's marker
// value. wipeSlice overwrites with 'x', but accept zero as well in
// case the implementation changes: either is a successful wipe.
func allWiped(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	marker := b[0]
	if marker != 'x' && marker != 0 {
		return false
	}
	for _, c := range b {
		if c != marker {
			return false
		}
	}
	return true
}
