// Copyright 2020 The NATS Authors
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

package jsm

import (
	"strings"
	"testing"
	"time"
)

func TestIsValidNameNullByte(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
	}{
		{"valid", true},
		{"\x00", false},
		{"a\x00b", false},
		{"\x00abc", false},
		{"abc\x00", false},
	}

	for _, tc := range cases {
		got := IsValidName(tc.name)
		if got != tc.valid {
			t.Errorf("IsValidName(%q) = %v, want %v", tc.name, got, tc.valid)
		}
	}
}

func TestIsValidSubjectPrefix(t *testing.T) {
	cases := []struct {
		s     string
		valid bool
	}{
		{"js", true},
		{"js.foreign", true},
		{"a.b.c", true},
		{"", false},
		{"bad>prefix", false},
		{"bad*prefix", false},
		{"bad prefix", false},
		{"a..b", false}, // empty token
		{".a", false},   // leading dot → empty first token
		{"a.", false},   // trailing dot → empty last token
		{"a.bad>b", false},
	}

	for _, tc := range cases {
		got := isValidSubjectPrefix(tc.s)
		if got != tc.valid {
			t.Errorf("isValidSubjectPrefix(%q) = %v, want %v", tc.s, got, tc.valid)
		}
	}
}

func TestParseDurationOverflow(t *testing.T) {
	cases := []string{
		"1000y",
		"1000000w",
		"1000000d",
		"10000M",
	}

	for _, tc := range cases {
		_, err := ParseDuration(tc)
		if err == nil {
			t.Errorf("ParseDuration(%q): expected overflow error, got nil", tc)
		}
	}
}

func TestParseDurationAccumulatedOverflow(t *testing.T) {
	// Two individually-valid durations that together exceed MaxInt64.
	// 200y ≈ 6.3e18 ns, 100y ≈ 3.2e18 ns; together ≈ 9.5e18 > 9.2e18 = MaxInt64.
	_, err := ParseDuration("200y100y")
	if err == nil {
		t.Error("ParseDuration(\"200y100y\"): expected accumulated overflow error, got nil")
	}
}

func TestParseDurationFloat64Precision(t *testing.T) {
	d, err := ParseDuration("1d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 24*time.Hour {
		t.Errorf("1d = %v, want %v", d, 24*time.Hour)
	}

	d, err = ParseDuration("1w")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 7*24*time.Hour {
		t.Errorf("1w = %v, want %v", d, 7*24*time.Hour)
	}
}

func TestLinearBackoffPeriodsStepTooSmall(t *testing.T) {
	// Range is 1 ns, steps = 2: stepSize truncates to 0, max != min → error.
	_, err := LinearBackoffPeriods(2, time.Millisecond, time.Millisecond+1)
	if err == nil {
		t.Fatal("expected error when step range too small, got nil")
	}
}

func TestLinearBackoffPeriodsEqualMinMax(t *testing.T) {
	// Equal min and max is a degenerate but valid range: all steps equal min.
	res, err := LinearBackoffPeriods(3, time.Second, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 3 {
		t.Fatalf("expected 3 periods, got %d", len(res))
	}

	for _, d := range res {
		if d != time.Second {
			t.Errorf("period = %v, want %v", d, time.Second)
		}
	}
}

func TestManagerNewInvalidDomain(t *testing.T) {
	_, err := New(nil, WithDomain("bad>domain"))
	if err == nil {
		t.Fatal("expected error for invalid domain, got nil")
	}

	if !strings.Contains(err.Error(), "domain") {
		t.Errorf("error should mention domain: %v", err)
	}
}

func TestManagerNewInvalidAPIPrefix(t *testing.T) {
	_, err := New(nil, WithAPIPrefix("bad>prefix"))
	if err == nil {
		t.Fatal("expected error for invalid api prefix, got nil")
	}

	if !strings.Contains(err.Error(), "prefix") {
		t.Errorf("error should mention prefix: %v", err)
	}
}

func TestManagerNewInvalidEventPrefix(t *testing.T) {
	_, err := New(nil, WithEventPrefix("bad*prefix"))
	if err == nil {
		t.Fatal("expected error for invalid event prefix, got nil")
	}

	if !strings.Contains(err.Error(), "prefix") {
		t.Errorf("error should mention prefix: %v", err)
	}
}

func TestManagerNewValidDotPrefix(t *testing.T) {
	// Multi-level prefix with dots must be accepted; failure should be nil nc, not prefix.
	_, err := New(nil, WithAPIPrefix("js.foreign"))
	if err != nil && strings.Contains(err.Error(), "prefix") {
		t.Errorf("valid dot-separated prefix was incorrectly rejected: %v", err)
	}
}
