package kv

import (
	"testing"
)

func TestIsReservedKey(t *testing.T) {
	if !IsReservedKey("_kv_x") {
		t.Fatalf("_kv_x was not a reserved key")
	}

	if IsReservedKey("bob") {
		t.Fatalf("bob was a reserved key")
	}
}

func TestIsValidKey(t *testing.T) {
	for _, k := range []string{" x y", "x ", "x!", "xx$", "*", ">", "x.>", "x.*"} {
		if IsValidKey(k) {
			t.Fatalf("%q was valid", k)
		}
	}

	for _, k := range []string{"foo", "_foo", "-foo", "_kv_foo", "foo123", "123", "a/b/c"} {
		if !IsValidKey(k) {
			t.Fatalf("%q was invalid", k)
		}
	}
}

func TestIsValidBucket(t *testing.T) {
	for _, b := range []string{" B", "!", "x/y", "x>", "x.x", "x.*", "x.>", "x*", "*", ">"} {
		if IsValidBucket(b) {
			t.Fatalf("%q was valid", b)
		}
	}

	for _, b := range []string{"B", "b", "123", "1_2_3", "1-2-3"} {
		if !IsValidKey(b) {
			t.Fatalf("%q was invalid", b)
		}
	}
}
