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

	"github.com/nats-io/jsm.go/natscontext"
)

// ErrorCode is the wire-level error code string shared by every
// protocol response that can signal failure.
type ErrorCode string

// Wire codes, each mapped 1:1 to a natscontext sentinel. The
// CodeInternal fallback is used when the originating error is not
// one of the known sentinels; the original error is never echoed to
// the wire.
//
// CodeStaleKey is intentionally not mapped to a natscontext sentinel:
// it is a client-internal retry signal that a server returns when
// opening a sealed request fails because the client sealed to a
// prior long-term xkey. See PROTOCOL.md §9.4.
const (
	CodeNotFound      ErrorCode = "not_found"
	CodeAlreadyExists ErrorCode = "already_exists"
	CodeInvalidName   ErrorCode = "invalid_name"
	CodeActiveContext ErrorCode = "active_context"
	CodeReadOnly      ErrorCode = "read_only"
	CodeConflict      ErrorCode = "conflict"
	CodeStaleKey      ErrorCode = "stale_key"
	CodeInternal      ErrorCode = "internal"
)

// internalMessage is the fixed Message used for unknown errors. The
// originating error is intentionally discarded so payload bytes
// captured inside it cannot leak over the wire.
const internalMessage = "internal server error"

// codeToSentinel maps a wire code to the natscontext sentinel it
// represents. CodeInternal has no sentinel and is returned as a bare
// error wrapping the message.
var codeToSentinel = map[ErrorCode]error{
	CodeNotFound:      natscontext.ErrNotFound,
	CodeAlreadyExists: natscontext.ErrAlreadyExists,
	CodeInvalidName:   natscontext.ErrInvalidName,
	CodeActiveContext: natscontext.ErrActiveContext,
	CodeReadOnly:      natscontext.ErrReadOnly,
	CodeConflict:      natscontext.ErrConflict,
}

// sentinelToCode is the inverse of codeToSentinel, used by
// ErrorFromSentinel to classify errors via errors.Is.
var sentinelToCode = []struct {
	sentinel error
	code     ErrorCode
}{
	{natscontext.ErrNotFound, CodeNotFound},
	{natscontext.ErrAlreadyExists, CodeAlreadyExists},
	{natscontext.ErrInvalidName, CodeInvalidName},
	{natscontext.ErrActiveContext, CodeActiveContext},
	{natscontext.ErrReadOnly, CodeReadOnly},
	{natscontext.ErrConflict, CodeConflict},
}

// ErrorFromSentinel classifies err against the natscontext sentinels
// and returns a wire ErrorResponse. Errors that do not match any
// sentinel are reported as CodeInternal with a fixed generic message;
// the originating error is never placed on the wire because it may
// contain payload bytes. A nil err returns nil.
func ErrorFromSentinel(err error) *ErrorResponse {
	if err == nil {
		return nil
	}

	for _, m := range sentinelToCode {
		if errors.Is(err, m.sentinel) {
			return &ErrorResponse{Code: string(m.code), Message: err.Error()}
		}
	}

	return &ErrorResponse{Code: string(CodeInternal), Message: internalMessage}
}

// SentinelFromError converts a wire ErrorResponse back into a Go
// error wrapping the matching natscontext sentinel so callers can
// use errors.Is. Unknown codes are returned as a bare error carrying
// the code and message. A nil er returns nil.
func SentinelFromError(er *ErrorResponse) error {
	if er == nil {
		return nil
	}

	sentinel, ok := codeToSentinel[ErrorCode(er.Code)]
	if !ok {
		return fmt.Errorf("svcbackend: %s: %s", er.Code, er.Message)
	}

	if er.Message == "" {
		return sentinel
	}
	return fmt.Errorf("%w: %s", sentinel, er.Message)
}
