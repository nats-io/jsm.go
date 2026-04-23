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

package svcbackend

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nats-io/jsm.go/natscontext"
)

// sentinelCases pairs each natscontext sentinel with its expected
// wire code. Driving every positive-path errors test from one table
// keeps coverage uniform.
var sentinelCases = []struct {
	name     string
	sentinel error
	code     ErrorCode
}{
	{"not_found", natscontext.ErrNotFound, CodeNotFound},
	{"already_exists", natscontext.ErrAlreadyExists, CodeAlreadyExists},
	{"invalid_name", natscontext.ErrInvalidName, CodeInvalidName},
	{"active_context", natscontext.ErrActiveContext, CodeActiveContext},
	{"none_selected", natscontext.ErrNoneSelected, CodeNoneSelected},
	{"read_only", natscontext.ErrReadOnly, CodeReadOnly},
	{"conflict", natscontext.ErrConflict, CodeConflict},
}

func TestErrorFromSentinel(t *testing.T) {
	for _, tc := range sentinelCases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := fmt.Errorf("%w: some context", tc.sentinel)

			er := ErrorFromSentinel(wrapped)
			if er == nil {
				t.Fatalf("expected non-nil ErrorResponse")
			}
			if er.Code != string(tc.code) {
				t.Fatalf("code: got %q want %q", er.Code, tc.code)
			}
			if er.Message == "" {
				t.Fatalf("expected non-empty message")
			}
		})
	}
}

func TestErrorFromSentinelNil(t *testing.T) {
	er := ErrorFromSentinel(nil)
	if er != nil {
		t.Fatalf("expected nil, got %+v", er)
	}
}

func TestErrorFromSentinelUnknown(t *testing.T) {
	er := ErrorFromSentinel(errors.New("payload bytes could leak here"))
	if er == nil {
		t.Fatalf("expected non-nil ErrorResponse")
	}
	if er.Code != string(CodeInternal) {
		t.Fatalf("code: got %q want %q", er.Code, CodeInternal)
	}
	if er.Message != internalMessage {
		t.Fatalf("message: got %q want %q", er.Message, internalMessage)
	}
	if strings.Contains(er.Message, "payload") {
		t.Fatalf("original error leaked into Message: %q", er.Message)
	}
}

func TestSentinelFromError(t *testing.T) {
	for _, tc := range sentinelCases {
		t.Run(tc.name, func(t *testing.T) {
			er := &ErrorResponse{Code: string(tc.code), Message: "detail"}

			err := SentinelFromError(er)
			if err == nil {
				t.Fatalf("expected non-nil error")
			}
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("errors.Is failed for %s: %v", tc.name, err)
			}
			if !strings.Contains(err.Error(), "detail") {
				t.Fatalf("expected message in error, got %q", err.Error())
			}
		})
	}
}

func TestSentinelFromErrorNil(t *testing.T) {
	err := SentinelFromError(nil)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestSentinelFromErrorUnknownCode(t *testing.T) {
	err := SentinelFromError(&ErrorResponse{Code: "something_novel", Message: "detail"})
	if err == nil {
		t.Fatalf("expected non-nil error")
	}

	for _, tc := range sentinelCases {
		if errors.Is(err, tc.sentinel) {
			t.Fatalf("unknown code unexpectedly matched sentinel %s", tc.name)
		}
	}
}

func TestSentinelFromErrorInternal(t *testing.T) {
	err := SentinelFromError(&ErrorResponse{Code: string(CodeInternal), Message: "boom"})
	if err == nil {
		t.Fatalf("expected non-nil error")
	}
}
