package natscontext_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/natscontext"
)

// setupEnv copies testdata into a fresh temp dir and points XDG_CONFIG_HOME
// there so test writes never touch the checked-in fixtures.
func setupEnv(t *testing.T) {
	t.Helper()

	tmp := t.TempDir()
	err := os.CopyFS(tmp, os.DirFS("testdata"))
	if err != nil {
		t.Fatalf("failed to copy testdata: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", tmp)
}

func TestContext(t *testing.T) {
	setupEnv(t)

	known := natscontext.KnownContexts()
	knownSet := make(map[string]bool, len(known))
	for _, k := range known {
		knownSet[k] = true
	}
	if !knownSet["gotest"] || !knownSet["other"] {
		t.Fatalf("expected gotest and other in known contexts, got %#v", known)
	}

	selected := natscontext.SelectedContext()
	if selected != "gotest" {
		t.Fatalf("Expected gotest got %q", selected)
	}

	err := natscontext.SelectContext("other")
	if err != nil {
		t.Fatalf("could not select context: %s", err)
	}

	selected = natscontext.SelectedContext()
	if selected != "other" {
		t.Fatalf("Expected other got %q", selected)
	}

	err = natscontext.UnSelectContext()
	if err != nil {
		t.Fatalf("failed to unselect")
	}
	selected = natscontext.SelectedContext()
	if selected != "" {
		t.Fatalf("Expected no context being selected got %q", selected)
	}
	err = natscontext.UnSelectContext()
	if err != nil {
		t.Fatalf("failed to unselect with no selected context: %v", err)
	}

	err = natscontext.SelectContext("nonexisting")
	if err.Error() != "unknown context" {
		t.Fatalf("expected unknown context error got: %v", err)
	}

	err = natscontext.SelectContext("gotest")
	if err != nil {
		t.Fatalf("could not select context: %s", err)
	}

	previousCtx := natscontext.PreviousContext()
	if previousCtx != "other" {
		t.Fatalf("previous context should be %q instead of %q", "other", previousCtx)
	}

	c, err := natscontext.New("", false)
	if err != nil {
		t.Fatalf("could not create empty context: %s", err)
	}

	err = c.Save("not..valid")
	if err == nil {
		t.Fatalf("expected error loading context, received none")
	}

	err = c.Save("/aaaa")
	if err == nil {
		t.Fatalf("expected error loading context, received none")
	}

	// just take whats there
	config, err := natscontext.New("", true)
	if err != nil {
		t.Fatalf("error loading context: %s", err)
	}
	if config.ServerURL() != "demo.nats.io" {
		t.Fatalf("expected demo.nats got %s", config.ServerURL())
	}

	// support overrides
	config, err = natscontext.New("", true, natscontext.WithServerURL("connect.ngs.global"))
	if err != nil {
		t.Fatalf("error loading context: %s", err)
	}
	if config.ServerURL() != "connect.ngs.global" {
		t.Fatalf("expected ngs got %s", config.ServerURL())
	}

	// Disallow multiple credential types
	config, err = natscontext.New("multi_creds", true)
	if err != nil {
		t.Fatalf("error loading context: %s", err)
	}
	err = config.Save("multi_creds")
	if err == nil {
		t.Fatalf("expected error saving context with multiple credentials, received none")
	}

	// Make sure username, password, and token can coexist
	config, err = natscontext.New("user_pass_token_creds", true)
	if err != nil {
		t.Fatalf("error loading context: %s", err)
	}
	err = config.Save("user_pass_token_creds")
	if err != nil {
		t.Fatalf("expected no error when saving a context with username and password")
	}

	// support missing config/context
	t.Setenv("XDG_CONFIG_HOME", "/nonexisting")
	config, err = natscontext.New("", true)
	if err != nil {
		t.Fatalf("error loading context: %s", err)
	}
	if config.ServerURL() != nats.DefaultURL {
		t.Fatalf("expected localhost got %s", config.ServerURL())
	}

	config, err = natscontext.NewFromFile("./testdata/gotest.json")
	if err != nil || (config.Name != "gotest" && config.ServerURL() != "demo.nats.io" && config.Token() != "use-nkeys!") {
		t.Fatalf("could not load context file: %s", err)
	}

	// UserJWT counts as a credential type and must not coexist with creds/nkey
	config, err = natscontext.New("jwt_and_creds", false,
		natscontext.WithUserJWT("somejwt"),
		natscontext.WithCreds("/some/path.creds"),
	)
	if err != nil {
		t.Fatalf("unexpected error creating jwt+creds context: %s", err)
	}
	err = config.Validate()
	if err == nil {
		t.Fatal("expected validation error for jwt+creds conflict, got nil")
	}

	// UserJWT alone should pass validation
	config, err = natscontext.New("jwtonly", false, natscontext.WithUserJWT("somejwt"))
	if err != nil {
		t.Fatalf("unexpected error creating jwt-only context: %s", err)
	}
	err = config.Validate()
	if err != nil {
		t.Fatalf("expected jwt-only context to be valid, got: %v", err)
	}
}

func TestNewFromFileMissing(t *testing.T) {
	_, err := natscontext.NewFromFile("/nonexistent/path/context.json")
	if err == nil {
		t.Fatal("expected error loading nonexistent context file, got nil")
	}
}

func TestDeleteContext(t *testing.T) {
	setupEnv(t)

	// Cannot delete the active context when others exist.
	active := natscontext.SelectedContext()
	if active == "" {
		t.Fatal("expected an active context to be set in testdata")
	}
	err := natscontext.DeleteContext(active)
	if err == nil {
		t.Fatalf("expected error deleting active context %q when others exist, got nil", active)
	}

	// Delete a non-active context.
	err = natscontext.DeleteContext("other")
	if err != nil {
		t.Fatalf("unexpected error deleting non-active context: %v", err)
	}
	if natscontext.IsKnown("other") {
		t.Fatal("context 'other' should not be known after deletion")
	}

	// Deleting a non-existent context is a no-op.
	err = natscontext.DeleteContext("other")
	if err != nil {
		t.Fatalf("deleting already-deleted context should be a no-op, got: %v", err)
	}

	// Deleting the active context when it is the only one should succeed.
	known := natscontext.KnownContexts()
	for _, name := range known {
		if name != active {
			err = natscontext.DeleteContext(name)
			if err != nil {
				t.Fatalf("deleting non-active context %q: %v", name, err)
			}
		}
	}
	err = natscontext.DeleteContext(active)
	if err != nil {
		t.Fatalf("expected to delete sole remaining context %q, got: %v", active, err)
	}
	if natscontext.IsKnown(active) {
		t.Fatalf("context %q should not be known after deletion", active)
	}

	// Reject invalid names.
	err = natscontext.DeleteContext("../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path-traversal name, got nil")
	}
}

func TestNATSOptionsUserJWTOnly(t *testing.T) {
	config, err := natscontext.New("jwtonly", false, natscontext.WithUserJWT("somejwt"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// NATSOptions must not return an error even when UserSeed is absent.
	opts, err := config.NATSOptions()
	if err != nil {
		t.Fatalf("NATSOptions returned unexpected error for jwt-only context: %v", err)
	}
	if len(opts) == 0 {
		t.Fatal("expected at least one NATS option for jwt-only context")
	}
}

func TestExpandEnvInPaths(t *testing.T) {
	tmp := t.TempDir()

	ctxDir := filepath.Join(tmp, "nats", "context")
	err := os.MkdirAll(ctxDir, 0700)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Verify $VAR expansion in all path fields.
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("NATS_TEST_DIR", tmp)

	ctxFile := filepath.Join(ctxDir, "envtest.json")
	content := `{"cert":"$NATS_TEST_DIR/client.crt","key":"$NATS_TEST_DIR/client.key","ca":"$NATS_TEST_DIR/ca.crt","nkey":"$NATS_TEST_DIR/seed.nk","creds":"$NATS_TEST_DIR/user.creds"}`
	err = os.WriteFile(ctxFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("write context: %v", err)
	}

	ctx, err := natscontext.New("envtest", true)
	if err != nil {
		t.Fatalf("loading context: %v", err)
	}

	cases := []struct {
		field string
		got   string
		want  string
	}{
		{"cert", ctx.Certificate(), tmp + "/client.crt"},
		{"key", ctx.Key(), tmp + "/client.key"},
		{"ca", ctx.CA(), tmp + "/ca.crt"},
		{"nkey", ctx.NKey(), tmp + "/seed.nk"},
		{"creds", ctx.Creds(), tmp + "/user.creds"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.field, c.got, c.want)
		}
	}

	// Verify ~ expansion in path fields.
	ctxFile2 := filepath.Join(ctxDir, "tildetest.json")
	content2 := `{"cert":"~/client.crt"}`
	err = os.WriteFile(ctxFile2, []byte(content2), 0600)
	if err != nil {
		t.Fatalf("write tilde context: %v", err)
	}

	ctx2, err := natscontext.New("tildetest", true)
	if err != nil {
		t.Fatalf("loading tilde context: %v", err)
	}
	if ctx2.Certificate() == "~/client.crt" {
		t.Error("cert: ~ was not expanded")
	}
}
