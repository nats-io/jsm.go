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

// behaviorChecks covers PROTOCOL.md §8 / Conformance "Behavior".
func behaviorChecks() []Check {
	return []Check{
		{
			ID: "behavior.load_missing", Section: "Behavior",
			Title: "ctx.load of a missing name returns not_found",
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				name := h.MintName("missing")
				// Don't Save() -- we want the not_found path.
				_, err := h.Client.Load(ctx, name)
				if !errors.Is(err, natscontext.ErrNotFound) {
					return StatusFail, fmt.Sprintf("expected not_found, got %v", err), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "behavior.delete_missing_ok", Section: "Behavior",
			Title: "ctx.delete of a missing name succeeds with no error",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				name := h.MintName("del_missing")
				err := h.Client.Delete(ctx, name)
				if err != nil {
					return StatusFail, fmt.Sprintf("expected nil, got %v", err), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "behavior.save_overwrites", Section: "Behavior",
			Title: "ctx.save overwrites existing values (no already_exists)",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				name := h.MintName("overwrite")

				err := h.Client.Save(ctx, name, []byte("first"))
				if err != nil {
					return StatusFail, "first save: " + err.Error(), nil
				}

				err = h.Client.Save(ctx, name, []byte("second"))
				if err != nil {
					return StatusFail, "overwrite save: " + err.Error(), nil
				}

				got, err := h.Client.Load(ctx, name)
				if err != nil {
					return StatusFail, "load after overwrite: " + err.Error(), nil
				}
				if string(got) != "second" {
					return StatusFail, fmt.Sprintf("overwrite lost: got %q", got), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "behavior.sel_get_empty", Section: "Behavior",
			Title: "sel.get returns none_selected when no selection is set",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				_, err := h.Client.SetSelected(ctx, "") // clear
				if err != nil {
					return StatusFail, "clear: " + err.Error(), nil
				}

				_, err = h.Client.Selected(ctx)
				if !errors.Is(err, natscontext.ErrNoneSelected) {
					return StatusFail, fmt.Sprintf("expected none_selected, got %v", err), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "behavior.sel_clear_returns_prev", Section: "Behavior",
			Title: "sel.clear returns the prior selection (empty when none)",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				name := h.MintName("sel_prev")
				err := h.Client.Save(ctx, name, []byte("x"))
				if err != nil {
					return StatusFail, "save: " + err.Error(), nil
				}

				_, err = h.Client.SetSelected(ctx, name)
				if err != nil {
					return StatusFail, "set: " + err.Error(), nil
				}

				prev, err := h.Client.SetSelected(ctx, "")
				if err != nil {
					return StatusFail, "clear: " + err.Error(), nil
				}
				if prev != name {
					return StatusFail, fmt.Sprintf("clear prev: got %q want %q", prev, name), nil
				}

				prev, err = h.Client.SetSelected(ctx, "")
				if err != nil {
					return StatusFail, "clear-again: " + err.Error(), nil
				}
				if prev != "" {
					return StatusFail, fmt.Sprintf("clear-again prev: got %q want empty", prev), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "behavior.immutable_readonly_save", Section: "Behavior",
			Title: "immutable servers return read_only from ctx.save",
			Modes: []string{"ro"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				err := rawSave(ctx, h, h.MintName("ro"), []byte("x"))
				if !errors.Is(err, natscontext.ErrReadOnly) {
					return StatusFail, fmt.Sprintf("expected read_only, got %v", err), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "behavior.immutable_readonly_delete", Section: "Behavior",
			Title: "immutable servers return read_only from ctx.delete",
			Modes: []string{"ro"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				err := h.Client.Delete(ctx, "anything")
				if !errors.Is(err, natscontext.ErrReadOnly) {
					return StatusFail, fmt.Sprintf("expected read_only, got %v", err), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "behavior.no_selection_readonly_get", Section: "Behavior",
			Title: "no-selection servers return read_only from sel.get",
			Modes: []string{"no-sel"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				_, err := h.Client.Selected(ctx)
				if !errors.Is(err, natscontext.ErrReadOnly) {
					return StatusFail, fmt.Sprintf("expected read_only, got %v", err), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "behavior.no_selection_readonly_set", Section: "Behavior",
			Title: "no-selection servers return read_only from sel.set",
			Modes: []string{"no-sel"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				_, err := h.Client.SetSelected(ctx, "anything")
				if !errors.Is(err, natscontext.ErrReadOnly) {
					return StatusFail, fmt.Sprintf("expected read_only, got %v", err), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "behavior.no_selection_readonly_clear", Section: "Behavior",
			Title: "no-selection servers return read_only from sel.clear",
			Modes: []string{"no-sel"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				_, err := h.Client.SetSelected(ctx, "")
				if !errors.Is(err, natscontext.ErrReadOnly) {
					return StatusFail, fmt.Sprintf("expected read_only, got %v", err), nil
				}
				return StatusPass, "", nil
			},
		},
	}
}
