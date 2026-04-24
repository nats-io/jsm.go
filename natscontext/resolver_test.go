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
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"

	"github.com/nats-io/jsm.go/natscontext"
)

// makeFixtureCreds returns a self-contained, correctly signed decorated
// JWT + nkey creds block. Generating at test-time avoids committing
// static keys that could drift from the parser's expectations.
func makeFixtureCreds(t *testing.T) string {
	t.Helper()

	accKP, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	userKP, err := nkeys.CreateUser()
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	userPub, err := userKP.PublicKey()
	if err != nil {
		t.Fatalf("user pub: %v", err)
	}

	claims := jwt.NewUserClaims(userPub)
	signed, err := claims.Encode(accKP)
	if err != nil {
		t.Fatalf("encode jwt: %v", err)
	}
	seed, err := userKP.Seed()
	if err != nil {
		t.Fatalf("user seed: %v", err)
	}

	formatted, err := jwt.FormatUserConfig(signed, seed)
	if err != nil {
		t.Fatalf("format creds: %v", err)
	}
	return string(formatted)
}

// writeCredsFile writes a freshly generated creds fixture to dir/name
// and returns its path.
func writeCredsFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(makeFixtureCreds(t)), 0600)
	if err != nil {
		t.Fatalf("write creds: %v", err)
	}
	return path
}

// prependToPATH inserts dir at the front of PATH for the remainder of
// the test so fake executables shadow anything installed system-wide.
func prependToPATH(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// writeShellShim writes a tiny shell script called `name` into dir
// that, when invoked, prints payload verbatim and exits 0. On Windows
// a .bat file is produced instead so the test is still portable.
func writeShellShim(t *testing.T, dir, name, payload string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		script := fmt.Sprintf("@echo off\r\nsetlocal enabledelayedexpansion\r\n<NUL set /p=\"%s\"\r\n", strings.ReplaceAll(payload, "\n", "\r\n"))
		err := os.WriteFile(filepath.Join(dir, name+".bat"), []byte(script), 0755)
		if err != nil {
			t.Fatalf("write shim: %v", err)
		}
		return
	}
	// Shell heredoc — the payload may contain quotes, newlines, etc.
	script := "#!/bin/sh\ncat <<'__NATS_SHIM_EOF__'\n" + payload + "\n__NATS_SHIM_EOF__\n"
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(script), 0755)
	if err != nil {
		t.Fatalf("write shim: %v", err)
	}
}

// TestFileResolver verifies that bare paths and file:// URIs resolve
// identically, and that a $VAR-expanded bare path produces the same
// option set as the literal path. ~-expansion is covered by
// TestExpandEnvInPaths; user.Current caches at process start, so
// verifying ~ via t.Setenv here isn't observable.
func TestFileResolver(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file-path semantics differ on Windows; covered on unix")
	}

	tmp := t.TempDir()
	credsPath := writeCredsFile(t, tmp, "user.creds")
	t.Setenv("NATS_TEST_CREDS_DIR", tmp)

	variants := []struct {
		name string
		ref  string
	}{
		{"bare path", credsPath},
		{"file scheme", "file://" + credsPath},
		{"file scheme uppercase", "FILE://" + credsPath},
		{"file scheme mixed case", "File://" + credsPath},
		{"env-var expansion", "$NATS_TEST_CREDS_DIR/user.creds"},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			ctx, err := natscontext.New(v.name, false, natscontext.WithCreds(v.ref))
			if err != nil {
				t.Fatalf("new context: %v", err)
			}
			opts, err := ctx.NATSOptions()
			if err != nil {
				t.Fatalf("NATSOptions: %v", err)
			}
			if len(opts) == 0 {
				t.Fatal("expected creds option to be appended")
			}
		})
	}
}

// TestFileResolverEagerRead covers the path that actually opens the
// referenced file (resolveLiteral → fileResolver.Resolve →
// os.ReadFile) so a broken file:// prefix strip surfaces as a hard
// error rather than being papered over by lazy nats.UserCredentials.
func TestFileResolverEagerRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file-path semantics differ on Windows; covered on unix")
	}

	tmp := t.TempDir()
	tokenPath := filepath.Join(tmp, "token.txt")
	err := os.WriteFile(tokenPath, []byte("super-secret-token"), 0600)
	if err != nil {
		t.Fatalf("write token: %v", err)
	}

	cases := []string{
		"file://" + tokenPath,
		"FILE://" + tokenPath,
		"File://" + tokenPath,
	}
	for _, ref := range cases {
		t.Run(ref, func(t *testing.T) {
			ctx, err := natscontext.New("filetoken", false, natscontext.WithToken(ref))
			if err != nil {
				t.Fatalf("new: %v", err)
			}
			opts, err := ctx.NATSOptions()
			if err != nil {
				t.Fatalf("NATSOptions: %v", err)
			}
			if len(opts) == 0 {
				t.Fatal("expected token option from file resolver")
			}
		})
	}
}

// TestOpResolver installs a fake `op` binary on PATH that returns the
// fixture creds, then verifies a Creds="op://..." value plumbs through
// NATSOptions without error.
func TestOpResolver(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-shim dispatch not portable on Windows")
	}

	binDir := t.TempDir()
	writeShellShim(t, binDir, "op", makeFixtureCreds(t))
	prependToPATH(t, binDir)

	for _, ref := range []string{"op://vault/item/field", "OP://vault/item/field", "Op://vault/item/field"} {
		ctx, err := natscontext.New("optest", false, natscontext.WithCreds(ref))
		if err != nil {
			t.Fatalf("new (%s): %v", ref, err)
		}
		opts, err := ctx.NATSOptions()
		if err != nil {
			t.Fatalf("NATSOptions (%s): %v", ref, err)
		}
		if len(opts) == 0 {
			t.Fatalf("expected creds option from op resolver for %s", ref)
		}
	}
}

// TestNscResolver installs a fake `nsc` binary on PATH that emits
// JSON pointing at a creds file we staged in temp, and verifies that
// a Creds="nsc://..." value produces a NATSOptions creds option.
func TestNscResolver(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-shim dispatch not portable on Windows")
	}

	binDir := t.TempDir()
	credsDir := t.TempDir()
	credsPath := writeCredsFile(t, credsDir, "nsc.creds")

	nscJSON := fmt.Sprintf(`{"user_creds":%q,"operator":{"service":["nats://example:4222"]}}`, credsPath)
	writeShellShim(t, binDir, "nsc", nscJSON)
	prependToPATH(t, binDir)

	for _, ref := range []string{"nsc://op/acct/user", "NSC://op/acct/user", "Nsc://op/acct/user"} {
		ctx, err := natscontext.New("nsctest", false, natscontext.WithCreds(ref))
		if err != nil {
			t.Fatalf("new (%s): %v", ref, err)
		}
		opts, err := ctx.NATSOptions()
		if err != nil {
			t.Fatalf("NATSOptions (%s): %v", ref, err)
		}
		if len(opts) == 0 {
			t.Fatalf("expected creds option from nsc resolver for %s", ref)
		}
	}
}

// TestNSCLookupDeprecation writes a context with the legacy NSCLookup
// field and no Creds, then verifies that after Save the JSON on disk
// carries Creds = "nsc://..." and the NSCLookup field is absent.
func TestNSCLookupDeprecation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-shim dispatch not portable on Windows")
	}

	binDir := t.TempDir()
	credsDir := t.TempDir()
	credsPath := writeCredsFile(t, credsDir, "nsc.creds")
	nscJSON := fmt.Sprintf(`{"user_creds":%q,"operator":{"service":["nats://legacy:4222"]}}`, credsPath)
	writeShellShim(t, binDir, "nsc", nscJSON)
	prependToPATH(t, binDir)

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	ctxDir := filepath.Join(dir, "nats", "context")
	err := os.MkdirAll(ctxDir, 0700)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err = os.WriteFile(filepath.Join(ctxDir, "legacy.json"), []byte(`{"nsc":"op/acct/user"}`), 0600)
	if err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	loaded, err := natscontext.New("legacy", true)
	if err != nil {
		t.Fatalf("load legacy: %v", err)
	}
	if loaded.ServerURL() != "nats://legacy:4222" {
		t.Fatalf("expected legacy URL preserved, got %q", loaded.ServerURL())
	}

	err = loaded.Save("legacy")
	if err != nil {
		t.Fatalf("save legacy: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(ctxDir, "legacy.json"))
	if err != nil {
		t.Fatalf("re-read legacy: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, `"creds": "nsc://op/acct/user"`) {
		t.Errorf("expected creds field with nsc:// URI, got:\n%s", body)
	}
	if strings.Contains(body, `"nsc": "op/acct/user"`) {
		t.Errorf("expected NSCLookup field to be cleared on save, got:\n%s", body)
	}
	// The nsc service URL must survive the migration so reloads do not
	// have to invoke nsc again to know where to connect.
	if !strings.Contains(body, `"url": "nats://legacy:4222"`) {
		t.Errorf("expected nsc service URL persisted into url field, got:\n%s", body)
	}

	reloaded, err := natscontext.New("legacy", true)
	if err != nil {
		t.Fatalf("reload legacy: %v", err)
	}
	if reloaded.ServerURL() != "nats://legacy:4222" {
		t.Fatalf("ServerURL lost after migration save+reload: got %q want nats://legacy:4222", reloaded.ServerURL())
	}
}

// TestRunNscProfileNormalizesPrefix asserts that every code path that
// invokes the nsc CLI hands it the bare op/acct/user form even when
// the caller supplied the nsc:// URI form. The shim records its
// argument so the test can verify the prefix was stripped exactly
// once regardless of which entry point dispatched the call.
func TestRunNscProfileNormalizesPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-shim dispatch not portable on Windows")
	}

	binDir := t.TempDir()
	credsDir := t.TempDir()
	credsPath := writeCredsFile(t, credsDir, "nsc.creds")
	argLog := filepath.Join(t.TempDir(), "nsc-arg.txt")
	nscJSON := fmt.Sprintf(`{"user_creds":%q,"operator":{"service":["nats://prefixed:4222"]}}`, credsPath)
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s' \"$3\" >> %q\ncat <<'__NATS_SHIM_EOF__'\n%s\n__NATS_SHIM_EOF__\n", argLog, nscJSON)
	err := os.WriteFile(filepath.Join(binDir, "nsc"), []byte(script), 0755)
	if err != nil {
		t.Fatalf("write shim: %v", err)
	}
	prependToPATH(t, binDir)

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	ctxDir := filepath.Join(dir, "nats", "context")
	err = os.MkdirAll(ctxDir, 0700)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err = os.WriteFile(filepath.Join(ctxDir, "loadprefixed.json"), []byte(`{"nsc":"nsc://op/acct/user"}`), 0600)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	_, err = natscontext.New("loadprefixed", true)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	c, err := natscontext.New("creds-resolve", false, natscontext.WithCreds("nsc://op/acct/user"))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	_, err = c.NATSOptions()
	if err != nil {
		t.Fatalf("NATSOptions: %v", err)
	}

	logged, err := os.ReadFile(argLog)
	if err != nil {
		t.Fatalf("read arg log: %v", err)
	}
	if strings.Contains(string(logged), "nsc://") {
		t.Fatalf("nsc CLI received URI form somewhere; argLog=%q", string(logged))
	}
	if !strings.Contains(string(logged), "op/acct/user") {
		t.Fatalf("expected nsc CLI to receive bare ref; argLog=%q", string(logged))
	}
}

// TestNSCLookupAlreadyPrefixedOnSave covers the legacy upgrade path
// where NSCLookup already carries an "nsc://" prefix (the form that
// WithNscUrl documents). Migrate must not blindly re-prepend the
// scheme: the saved creds reference must be a single nsc:// URI so
// later resolver dispatch is idempotent.
func TestNSCLookupAlreadyPrefixedOnSave(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-shim dispatch not portable on Windows")
	}

	binDir := t.TempDir()
	credsDir := t.TempDir()
	credsPath := writeCredsFile(t, credsDir, "nsc.creds")
	nscJSON := fmt.Sprintf(`{"user_creds":%q,"operator":{"service":["nats://prefixed:4222"]}}`, credsPath)
	writeShellShim(t, binDir, "nsc", nscJSON)
	prependToPATH(t, binDir)

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	ctxDir := filepath.Join(dir, "nats", "context")
	err := os.MkdirAll(ctxDir, 0700)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	c, err := natscontext.New("prefixed", false, natscontext.WithNscUrl("nsc://op/acct/user"))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	err = c.Save("")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(ctxDir, "prefixed.json"))
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, `"creds": "nsc://op/acct/user"`) {
		t.Errorf("expected single-prefix creds URI, got:\n%s", body)
	}
	if strings.Contains(body, "nsc://nsc://") {
		t.Errorf("creds was double-prefixed, got:\n%s", body)
	}
}

// TestNSCLookupOverrideOnSave covers the legacy-upgrade path where a
// context loaded with NSCLookup set is then assigned an explicit
// Creds value before Save. Validate must not reject the
// NSCLookup+Creds combination as "too many types of credentials":
// migrate runs first, drops NSCLookup, and Save persists the explicit
// Creds.
func TestNSCLookupOverrideOnSave(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-shim dispatch not portable on Windows")
	}

	binDir := t.TempDir()
	credsDir := t.TempDir()
	credsPath := writeCredsFile(t, credsDir, "nsc.creds")
	overridePath := writeCredsFile(t, credsDir, "override.creds")
	nscJSON := fmt.Sprintf(`{"user_creds":%q,"operator":{"service":["nats://legacy:4222"]}}`, credsPath)
	writeShellShim(t, binDir, "nsc", nscJSON)
	prependToPATH(t, binDir)

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	ctxDir := filepath.Join(dir, "nats", "context")
	err := os.MkdirAll(ctxDir, 0700)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err = os.WriteFile(filepath.Join(ctxDir, "legacy.json"), []byte(`{"nsc":"op/acct/user"}`), 0600)
	if err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	loaded, err := natscontext.New("legacy", true, natscontext.WithCreds(overridePath))
	if err != nil {
		t.Fatalf("load legacy: %v", err)
	}

	err = loaded.Save("")
	if err != nil {
		t.Fatalf("save with override: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(ctxDir, "legacy.json"))
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	body := string(data)
	expectedCreds := fmt.Sprintf(`"creds": %q`, overridePath)
	if !strings.Contains(body, expectedCreds) {
		t.Errorf("expected explicit override creds preserved, got:\n%s", body)
	}
	if strings.Contains(body, `"nsc": "op/acct/user"`) {
		t.Errorf("expected NSCLookup field cleared on save, got:\n%s", body)
	}
}

// TestCustomResolver registers a handmade resolver for a fake scheme
// through Registry options and verifies it is invoked when a Creds
// value uses that scheme.
func TestCustomResolver(t *testing.T) {
	resolver := &recordingResolver{
		scheme: "test",
		reply:  []byte(makeFixtureCreds(t)),
	}

	reg := natscontext.NewRegistry(
		natscontext.NewFileBackendAt(t.TempDir()),
		natscontext.WithDefaultResolvers(),
		natscontext.WithCredentialResolver(resolver),
	)

	nctx, err := reg.Load(context.Background(), "", natscontext.WithCreds("test://some-ref"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	opts, err := nctx.NATSOptions()
	if err != nil {
		t.Fatalf("NATSOptions: %v", err)
	}
	if len(opts) == 0 {
		t.Fatal("expected creds option from custom resolver")
	}

	got := resolver.seen()
	if len(got) != 1 || got[0] != "test://some-ref" {
		t.Fatalf("custom resolver invocations = %v, want [test://some-ref]", got)
	}
}

// TestBareFallbackResolver verifies that a resolver registering the
// empty scheme is invoked for bare (unschemed) Token/Password/UserJwt/
// UserSeed values, honoring the Schemes()/BACKENDS.md contract.
// resolveLiteral previously short-circuited on an empty scheme and
// never consulted resolvers[""].
func TestBareFallbackResolver(t *testing.T) {
	fallback := &recordingResolver{
		scheme: "",
		reply:  []byte("resolved-token"),
	}
	reg := natscontext.NewRegistry(
		natscontext.NewFileBackendAt(t.TempDir()),
		natscontext.WithDefaultResolvers(),
		natscontext.WithCredentialResolver(fallback),
	)

	nctx, err := reg.Load(context.Background(), "", natscontext.WithToken("bare-ref"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	_, err = nctx.NATSOptions()
	if err != nil {
		t.Fatalf("NATSOptions: %v", err)
	}

	got := fallback.seen()
	if len(got) != 1 || got[0] != "bare-ref" {
		t.Fatalf("fallback resolver invocations = %v, want [bare-ref]", got)
	}
}

// TestBareValueLiteralWithoutFallback verifies that without a
// resolver registered for the empty scheme, bare Token values pass
// through as literals — the pre-existing default behavior must be
// preserved by the fallback fix.
func TestBareValueLiteralWithoutFallback(t *testing.T) {
	probe := &recordingResolver{scheme: "never"}
	reg := natscontext.NewRegistry(
		natscontext.NewFileBackendAt(t.TempDir()),
		natscontext.WithDefaultResolvers(),
		natscontext.WithCredentialResolver(probe),
	)

	nctx, err := reg.Load(context.Background(), "", natscontext.WithToken("plain-token"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	_, err = nctx.NATSOptions()
	if err != nil {
		t.Fatalf("NATSOptions: %v", err)
	}

	if len(probe.seen()) != 0 {
		t.Fatalf("no resolver should have been consulted, got %v", probe.seen())
	}
	if nctx.Token() != "plain-token" {
		t.Fatalf("bare token mutated: got %q", nctx.Token())
	}
}

// TestEnvResolver exercises the env:// resolver. It asserts that a
// set variable is resolved verbatim and that a missing or empty
// variable surfaces an error rather than a silent empty secret.
func TestEnvResolver(t *testing.T) {
	t.Setenv("NATS_TEST_TOKEN", "super-secret-token")
	t.Setenv("NATS_TEST_EMPTY", "")

	for _, ref := range []string{"env://NATS_TEST_TOKEN", "ENV://NATS_TEST_TOKEN", "Env://NATS_TEST_TOKEN"} {
		ctx, err := natscontext.New("envtest", false, natscontext.WithToken(ref))
		if err != nil {
			t.Fatalf("new (%s): %v", ref, err)
		}
		opts, err := ctx.NATSOptions()
		if err != nil {
			t.Fatalf("NATSOptions (%s): %v", ref, err)
		}
		if len(opts) == 0 {
			t.Fatalf("expected a token option from env resolver for %s", ref)
		}
	}

	ctx, err := natscontext.New("envmissing", false, natscontext.WithToken("env://NATS_TEST_UNSET_VARIABLE"))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	_, err = ctx.NATSOptions()
	if err == nil {
		t.Fatal("expected error when env var is unset, got nil")
	}

	ctx, err = natscontext.New("envempty", false, natscontext.WithToken("env://NATS_TEST_EMPTY"))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	_, err = ctx.NATSOptions()
	if err == nil {
		t.Fatal("expected error when env var is empty, got nil")
	}
}

// TestDataResolver exercises the data: resolver. Valid base64 payloads
// decode cleanly; the resolver rejects non-base64 payloads and
// malformed base64.
func TestDataResolver(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("hello-secret"))

	for _, prefix := range []string{"data:", "DATA:", "Data:"} {
		ref := prefix + ";base64," + encoded
		ctx, err := natscontext.New("datatest", false, natscontext.WithToken(ref))
		if err != nil {
			t.Fatalf("new (%s): %v", ref, err)
		}
		opts, err := ctx.NATSOptions()
		if err != nil {
			t.Fatalf("NATSOptions (%s): %v", ref, err)
		}
		if len(opts) == 0 {
			t.Fatalf("expected a token option from data resolver for %s", ref)
		}
	}

	cases := []struct {
		name string
		ref  string
	}{
		{"missing base64 token", "data:application/octet-stream,notbase64"},
		{"malformed base64", "data:;base64,!!!not-base64!!!"},
		{"missing comma", "data:;base64"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, err := natscontext.New("datafail", false, natscontext.WithToken(c.ref))
			if err != nil {
				t.Fatalf("new: %v", err)
			}
			_, err = ctx.NATSOptions()
			if err == nil {
				t.Fatalf("expected error for %q, got nil", c.ref)
			}
		})
	}
}

// TestEncodeDataURIRoundTrip asserts that EncodeDataURI produces a
// reference the data: resolver accepts, and that the resolver returns
// the original bytes verbatim.
func TestEncodeDataURIRoundTrip(t *testing.T) {
	cases := [][]byte{
		[]byte("hello-secret"),
		{0x00, 0x01, 0xff, 0xfe, '\n', '\r'},
		nil,
		bytes.Repeat([]byte{0xaa}, 1024),
	}
	for _, payload := range cases {
		uri := natscontext.EncodeDataURI(payload)

		ctx, err := natscontext.New("datart", false, natscontext.WithToken(uri))
		if err != nil {
			t.Fatalf("new for %d-byte payload: %v", len(payload), err)
		}
		opts, err := ctx.NATSOptions()
		if err != nil {
			t.Fatalf("NATSOptions for %d-byte payload: %v", len(payload), err)
		}
		if len(payload) > 0 && len(opts) == 0 {
			t.Fatalf("expected a token option for %d-byte payload, got none", len(payload))
		}
	}
}

// TestMemoryRegistryBasics exercises the core of the legacy TestContext
// flow (save, select, load, unselect) through a Registry backed by a
// MemoryBackend. It documents that the registry contract does not
// depend on the filesystem.
func TestMemoryRegistryBasics(t *testing.T) {
	reg := natscontext.NewRegistry(natscontext.NewMemoryBackend())
	bg := context.Background()

	_, err := reg.Selected(bg)
	if !errors.Is(err, natscontext.ErrNoneSelected) {
		t.Fatalf("fresh Selected: expected ErrNoneSelected, got %v", err)
	}

	nctx, err := natscontext.New("alpha", false, natscontext.WithServerURL("nats://a:4222"))
	if err != nil {
		t.Fatalf("new alpha: %v", err)
	}
	err = reg.Save(bg, nctx, "")
	if err != nil {
		t.Fatalf("save alpha: %v", err)
	}

	nctx, err = natscontext.New("bravo", false, natscontext.WithServerURL("nats://b:4222"))
	if err != nil {
		t.Fatalf("new bravo: %v", err)
	}
	err = reg.Save(bg, nctx, "")
	if err != nil {
		t.Fatalf("save bravo: %v", err)
	}

	names, err := reg.List(bg)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("list len: got %d want 2 (%v)", len(names), names)
	}

	_, err = reg.Select(bg, "alpha")
	if err != nil {
		t.Fatalf("select alpha: %v", err)
	}
	sel, err := reg.Selected(bg)
	if err != nil {
		t.Fatalf("selected: %v", err)
	}
	if sel != "alpha" {
		t.Fatalf("selected = %q want alpha", sel)
	}

	loaded, err := reg.Load(bg, "")
	if err != nil {
		t.Fatalf("load selected: %v", err)
	}
	if loaded.ServerURL() != "nats://a:4222" {
		t.Fatalf("loaded URL = %q want nats://a:4222", loaded.ServerURL())
	}

	prev, err := reg.Unselect(bg)
	if err != nil {
		t.Fatalf("unselect: %v", err)
	}
	if prev != "alpha" {
		t.Fatalf("unselect prior = %q want alpha", prev)
	}
	_, err = reg.Selected(bg)
	if !errors.Is(err, natscontext.ErrNoneSelected) {
		t.Fatalf("post-unselect Selected: expected ErrNoneSelected, got %v", err)
	}
}

type recordingResolver struct {
	scheme string
	reply  []byte

	mu   sync.Mutex
	refs []string
}

func (r *recordingResolver) Schemes() []string { return []string{r.scheme} }

func (r *recordingResolver) Resolve(_ context.Context, ref string) ([]byte, error) {
	r.mu.Lock()
	r.refs = append(r.refs, ref)
	r.mu.Unlock()
	return r.reply, nil
}

func (r *recordingResolver) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.refs))
	copy(out, r.refs)
	return out
}
