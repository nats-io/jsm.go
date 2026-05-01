package natscontext

import (
	"errors"
	"os"
	"testing"
)

func TestContext(t *testing.T) {
	// Do nothing if the string does not start with ~ and has no env vars
	t1 := expandHomedir("foo")
	if t1 != "foo" {
		t.Fatalf("failed to expand home directory for string 'foo': %s", t1)
	}

	// Expand ~ to HOMEDIR if string starts with ~
	t2 := expandHomedir("~/foo")
	if t2 == "~/foo" {
		t.Fatalf("failed to expand home directory for string '~/foo': %s", t2)
	}

	// Do nothing if there is a ~ but it is not the first character of the string
	t3 := expandHomedir("/~/foo")
	if t3 != "/~/foo" {
		t.Fatalf("expected /~/foo unchanged, got: %s", t3)
	}

	// Expand environment variables
	t.Setenv("NATS_TEST_EXPAND", "/some/path")
	t4 := expandHomedir("$NATS_TEST_EXPAND/certs")
	if t4 != "/some/path/certs" {
		t.Fatalf("expected env var expansion, got: %s", t4)
	}

	// Env var combined with ~ is not a realistic case but must not panic
	t5 := expandHomedir("$NATS_TEST_EXPAND")
	if t5 != "/some/path" {
		t.Fatalf("expected %q, got %q", "/some/path", t5)
	}

	// Unset variable expands to empty string (os.ExpandEnv behavior)
	os.Unsetenv("NATS_TEST_UNSET")
	t6 := expandHomedir("$NATS_TEST_UNSET/foo")
	if t6 != "/foo" {
		t.Fatalf("expected unset var to expand to empty string, got %q", t6)
	}
}

func TestExpandHomedirEmpty(t *testing.T) {
	result := expandHomedir("")
	if result != "" {
		t.Fatalf("expected empty string back, got %q", result)
	}
}

func TestWipeSlice(t *testing.T) {
	buf := []byte("secret-data-1234")
	wipeSlice(buf)
	for i, b := range buf {
		if b != 'x' {
			t.Fatalf("byte at index %d not wiped: got %q", i, b)
		}
	}
}

func TestValidateName(t *testing.T) {
	// ValidateName is deliberately loose: historical user contexts
	// persisted before a stricter validator existed, and upgrading
	// must not strand them. The rule is exactly "non-empty, no ".."
	// substring, no / or \". Anything else is a concern for specific
	// backends to layer on top (see svcbackend, which rejects
	// subject-hostile characters before publishing).
	valid := []string{
		"foo", "my-context", "ctx_1", "a",
		"ngs.js",   // the regression-of-record — had to keep working
		"foo.bar",  // single dot mid-name
		"foo bar",  // whitespace — legal at the core
		"foo\tbar", // tab — legal at the core
		"foo*bar",  // subject wildcard — legal at the core
		"foo>bar",  // subject wildcard — legal at the core
		"foo\x00a", // control character — legal at the core
		"foo\x7fa", // DEL — legal at the core
	}
	for _, name := range valid {
		err := ValidateName(name)
		if err != nil {
			t.Errorf("expected %q to be valid, got %v", name, err)
		}
	}

	invalid := []string{
		"",         // empty
		"foo/bar",  // forward slash
		"foo\\bar", // backslash
		"../evil",  // parent traversal with forward slash
		"..\\evil", // parent traversal with backslash
		"foo..bar", // contains ".." substring
		"..",       // bare ".."
		"a..",      // ".." at the end
		"..a",      // ".." at the start
	}
	for _, name := range invalid {
		err := ValidateName(name)
		if err == nil {
			t.Errorf("expected %q to be invalid", name)
			continue
		}
		if !errors.Is(err, ErrInvalidName) {
			t.Errorf("expected %q to wrap ErrInvalidName, got %v", name, err)
		}
	}
}

func TestNumCreds(t *testing.T) {
	empty := &Context{config: &settings{}}
	n := numCreds(empty)
	if n != 0 {
		t.Fatalf("expected 0 creds, got %d", n)
	}

	jwtOnly := &Context{config: &settings{UserJwt: "somejwt"}}
	n = numCreds(jwtOnly)
	if n != 1 {
		t.Fatalf("expected 1 for jwt-only, got %d", n)
	}

	jwtAndCreds := &Context{config: &settings{UserJwt: "somejwt", Creds: "/some/path.creds"}}
	n = numCreds(jwtAndCreds)
	if n != 2 {
		t.Fatalf("expected 2 for jwt+creds, got %d", n)
	}

	jwtAndNkey := &Context{config: &settings{UserJwt: "somejwt", NKey: "/some/path.nk"}}
	n = numCreds(jwtAndNkey)
	if n != 2 {
		t.Fatalf("expected 2 for jwt+nkey, got %d", n)
	}
}
