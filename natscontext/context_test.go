package natscontext_test

import (
	"testing"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/natscontext"
)

func TestContext(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "testdata")

	known := natscontext.KnownContexts()
	if len(known) != 2 && known[0] != "gotest" && known[1] != "other" {
		t.Fatalf("expected [gotest,other] got %#v", known)
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
