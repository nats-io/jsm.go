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

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/jsm.go/natscontext"
)

// wireTestableBadNames are the invalid name characters that survive
// NATS subject tokenization. The spec forbids `.`, `*`, `>`, whitespace,
// and control characters too, but the NATS subject parser rejects those
// before they reach the server, so they aren't wire-testable. The
// `/` and `\` characters pass through the parser intact, giving us a
// clean way to exercise the server's validator.
var wireTestableBadNames = []struct{ label, name string }{
	{"slash", "foo/bar"},
	{"backslash", `foo\bar`},
}

// nameChecks covers PROTOCOL.md §7 / Conformance "Name validation".
func nameChecks() []Check {
	return []Check{
		{
			ID: "names.save_rejects_bad_chars", Section: "Name validation",
			Title: "ctx.save rejects /, \\ with invalid_name",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				for _, bad := range wireTestableBadNames {
					err := rawSave(ctx, h, bad.name, []byte("x"))
					if !errors.Is(err, natscontext.ErrInvalidName) {
						return StatusFail, fmt.Sprintf("%s name not rejected as invalid_name: %v", bad.label, err), nil
					}
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "names.delete_rejects_bad_chars", Section: "Name validation",
			Title: "ctx.delete rejects /, \\ with invalid_name",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				for _, bad := range wireTestableBadNames {
					err := h.Client.Delete(ctx, bad.name)
					// The reference client pre-validates, so errors.Is
					// picks up a local ErrInvalidName here too.
					if !errors.Is(err, natscontext.ErrInvalidName) {
						return StatusFail, fmt.Sprintf("%s name not rejected as invalid_name: %v", bad.label, err), nil
					}
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "names.load_rejects_bad_chars", Section: "Name validation",
			Title: "ctx.load rejects /, \\ with invalid_name",
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				for _, bad := range wireTestableBadNames {
					_, err := h.Client.Load(ctx, bad.name)
					if !errors.Is(err, natscontext.ErrInvalidName) {
						return StatusFail, fmt.Sprintf("%s name not rejected as invalid_name: %v", bad.label, err), nil
					}
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "names.save_empty_rejected", Section: "Name validation",
			Title: "ctx.save with empty name is rejected",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				// Empty name cannot be encoded into a subject token (NATS
				// rejects ".." / trailing dot). We assert the reference
				// client refuses to send it; that is the contract for
				// operators, and there is no legitimate way for an
				// empty name to reach the server over the wire.
				err := h.Client.Save(ctx, "", []byte("x"))
				if !errors.Is(err, natscontext.ErrInvalidName) {
					return StatusFail, fmt.Sprintf("empty name not rejected: %v", err), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "names.pre_storage_validation", Section: "Name validation",
			Title: "rejected names never produced a stored context",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				for _, bad := range wireTestableBadNames {
					// Attempt a raw save we expect to be rejected.
					_ = rawSave(ctx, h, bad.name, []byte("canary"))

					// The name cannot legally appear in list output. Any
					// appearance means storage was touched before
					// validation.
					names, err := h.Client.List(ctx)
					if err != nil {
						return StatusFail, "list: " + err.Error(), nil
					}
					for _, n := range names {
						if n == bad.name {
							return StatusFail, fmt.Sprintf("rejected name %q appears in list", n), nil
						}
					}
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "names.unreachable_via_wire", Section: "Name validation",
			Title: "names containing ., *, >, whitespace blocked at subject layer",
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				// These characters are forbidden inside NATS subject
				// tokens: the client library refuses to publish and the
				// server's wildcard subscription ("natscontext.v1.ctx.
				// save.*") would not match multi-token names anyway.
				// This check documents that property rather than
				// verifying it -- proving a negative here would require
				// bypassing NATS itself.
				return StatusSkip, "NATS subject rules prevent wire transmission of these characters", nil
			},
		},
	}
}
