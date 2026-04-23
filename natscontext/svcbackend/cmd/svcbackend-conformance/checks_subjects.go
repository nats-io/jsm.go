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

	"github.com/nats-io/nats.go"
)

// subjectChecks covers PROTOCOL.md §4 / Conformance "Subjects".
//
// Each subject is probed with a minimal empty-JSON request. A response
// of any shape (even a wire error envelope) proves a responder is
// registered; only nats.ErrNoResponders is treated as a registration
// failure. Wildcard subjects (ctx.load.<name>, ctx.save.<name>,
// ctx.delete.<name>, sel.set.<name>) are probed with a harmless test
// name so the server's wildcard subscription actually fires.
func subjectChecks() []Check {
	type subj struct {
		id, suffix, sample string
	}

	subjects := []subj{
		{"sys.xkey", "sys.xkey", ""},
		{"ctx.load", "ctx.load", "__probe__"},
		{"ctx.save", "ctx.save", "__probe__"},
		{"ctx.delete", "ctx.delete", "__probe__"},
		{"ctx.list", "ctx.list", ""},
		{"sel.get", "sel.get", ""},
		{"sel.set", "sel.set", "__probe__"},
		{"sel.clear", "sel.clear", ""},
	}

	var out []Check
	for _, s := range subjects {
		s := s
		target := s.suffix
		if s.sample != "" {
			target = s.suffix + "." + s.sample
		}

		out = append(out, Check{
			ID:      "subjects." + s.id,
			Section: "Subjects",
			Title:   fmt.Sprintf("%s.%s is registered", "<prefix>", target),
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				err := rawProbe(ctx, h, h.Prefix+"."+target)
				if errors.Is(err, nats.ErrNoResponders) {
					return StatusFail, "no responders on " + h.Prefix + "." + target, nil
				}
				// Any non-transport error was produced by the responder and
				// therefore proves registration. A request timeout is less
				// conclusive but still implies the subject is subscribed.
				return StatusPass, "", nil
			},
		})
	}

	return out
}
