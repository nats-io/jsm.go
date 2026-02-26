package natscontext

import "testing"

func TestContext(t *testing.T) {
	// Do nothing if the string does not start with ~
	t1 := expandHomedir("foo")
	if t1 != "foo" {
		t.Fatalf("failed to expand home directory for string 'foo': %s", t1)
	}

	// Expand ~ to HOMEDIR if string starts with ~
	t2 := expandHomedir("~/foo")
	if t2 == "~/foo" {
		t.Fatalf("failed to expand home directory for string 'foo': %s", t2)
	}

	// Do nothing if there is a ~ but it is not the first character of the string
	t3 := expandHomedir("/~/foo")
	if t3 != "/~/foo" {
		t.Fatalf("failed to expand home directory for string 'foo': %s", t3)
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

func TestNumCreds(t *testing.T) {
	empty := &Context{config: &settings{}}
	if n := numCreds(empty); n != 0 {
		t.Fatalf("expected 0 creds, got %d", n)
	}

	jwtOnly := &Context{config: &settings{UserJwt: "somejwt"}}
	if n := numCreds(jwtOnly); n != 1 {
		t.Fatalf("expected 1 for jwt-only, got %d", n)
	}

	jwtAndCreds := &Context{config: &settings{UserJwt: "somejwt", Creds: "/some/path.creds"}}
	if n := numCreds(jwtAndCreds); n != 2 {
		t.Fatalf("expected 2 for jwt+creds, got %d", n)
	}

	jwtAndNkey := &Context{config: &settings{UserJwt: "somejwt", NKey: "/some/path.nk"}}
	if n := numCreds(jwtAndNkey); n != 2 {
		t.Fatalf("expected 2 for jwt+nkey, got %d", n)
	}
}
