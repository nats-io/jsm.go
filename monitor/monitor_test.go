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

package monitor

import (
	"os"
	"testing"
	"time"
	"unicode/utf8"
)

func TestSecondsToDuration(t *testing.T) {
	tests := []struct {
		input    float64
		expected time.Duration
	}{
		{0, 0},
		{1, time.Second},
		{0.5, 500 * time.Millisecond},
		{2.5, 2500 * time.Millisecond},
		{60, time.Minute},
	}

	for _, tt := range tests {
		got := secondsToDuration(tt.input)
		if got != tt.expected {
			t.Errorf("secondsToDuration(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestFileAccessible(t *testing.T) {
	t.Run("accessible file", func(t *testing.T) {
		f, err := os.CreateTemp("", "monitor-test-*")
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		defer os.Remove(f.Name())

		ok, err := fileAccessible(f.Name())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected file to be accessible")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		ok, err := fileAccessible("/nonexistent/path/file.txt")
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
		if ok {
			t.Fatal("expected false for nonexistent file")
		}
	})

	t.Run("directory path", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "monitor-test-dir-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(dir)

		ok, err := fileAccessible(dir)
		if err == nil {
			t.Fatal("expected error for directory")
		}
		if ok {
			t.Fatal("expected false for directory")
		}
	})
}

func TestRandomString(t *testing.T) {
	t.Run("correct length", func(t *testing.T) {
		for _, length := range []int{0, 1, 10, 100} {
			s := randomString(length)
			if utf8.RuneCountInString(s) != length {
				t.Errorf("randomString(%d) returned length %d", length, utf8.RuneCountInString(s))
			}
		}
	})

	t.Run("valid characters", func(t *testing.T) {
		validRunes := make(map[rune]bool, len(passwordRunes))
		for _, r := range passwordRunes {
			validRunes[r] = true
		}

		s := randomString(1000)
		for _, r := range s {
			if !validRunes[r] {
				t.Errorf("randomString returned invalid character %q", r)
			}
		}
	})

	t.Run("not constant", func(t *testing.T) {
		a := randomString(32)
		b := randomString(32)
		if a == b {
			t.Error("randomString returned identical strings on consecutive calls")
		}
	})
}
