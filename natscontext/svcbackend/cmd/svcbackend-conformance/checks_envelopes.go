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
	"fmt"
	"sort"
	"strings"

	"github.com/nats-io/jsm.go/natscontext/svcbackend"
)

// envelopeChecks covers Conformance "Envelopes".
func envelopeChecks() []Check {
	return []Check{
		{
			ID: "envelopes.list_success_no_error", Section: "Envelopes",
			Title: "ctx.list success response has no error field",
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				var resp svcbackend.ListResponse

				err := rawRequest(ctx, h, h.Prefix+".ctx.list", svcbackend.ListRequest{}, &resp)
				if err != nil {
					return StatusFail, "list: " + err.Error(), nil
				}

				if resp.Error != nil {
					return StatusFail, fmt.Sprintf("success path carries error field: %s", resp.Error.Code), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "envelopes.list_sorted", Section: "Envelopes",
			Title: "ctx.list names are sorted ascending",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				// Seed several names in reverse order, then verify the
				// server sorts them on return.
				seeds := []string{"zeta", "alpha", "mike", "bravo"}
				created := make([]string, 0, len(seeds))
				for _, s := range seeds {
					n := h.MintName("sort_" + s)
					saveErr := h.Client.Save(ctx, n, []byte("x"))
					if saveErr != nil {
						return StatusFail, "save seed: " + saveErr.Error(), nil
					}
					created = append(created, n)
				}

				names, err := h.Client.List(ctx)
				if err != nil {
					return StatusFail, "list: " + err.Error(), nil
				}

				// Intersect with our seeds so unrelated names don't
				// confuse the ordering check.
				want := map[string]struct{}{}
				for _, n := range created {
					want[n] = struct{}{}
				}

				var subset []string
				for _, n := range names {
					_, ok := want[n]
					if ok {
						subset = append(subset, n)
					}
				}

				sorted := make([]string, len(subset))
				copy(sorted, subset)
				sort.Strings(sorted)

				areEqual := strings.Join(subset, "\x00") == strings.Join(sorted, "\x00")
				if !areEqual {
					return StatusFail, fmt.Sprintf("names not sorted: got %v", subset), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "envelopes.no_payload_in_error", Section: "Envelopes",
			Title: "error.message does not leak stored payload bytes",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				// Save a name with a recognizable canary, then try to
				// provoke every error path we can reach and assert the
				// canary never appears in error.message.
				canary := "CONFORMANCE-CANARY-PAYLOAD-LEAK-PROBE"
				name := h.MintName("leak")
				err := h.Client.Save(ctx, name, []byte(canary))
				if err != nil {
					return StatusFail, "save canary: " + err.Error(), nil
				}

				// Hit a missing-name load and a bad-name load; inspect
				// their error messages. (A compliant server returns a
				// short human-readable message; a non-compliant one
				// might echo stored data.)
				probes := []string{
					h.MintName("missing"),
					`foo/bar`,
				}

				for _, p := range probes {
					msg := probeErrorMessage(ctx, h, p)
					if strings.Contains(msg, canary) {
						return StatusFail, fmt.Sprintf("canary appeared in error.message for %q", p), nil
					}
				}

				// Even when the assertion passes, we cannot prove the
				// server doesn't leak in a path we didn't hit. Report
				// PASS but note the probabilistic nature.
				return StatusPass, "", nil
			},
		},
	}
}

// probeErrorMessage issues a raw ctx.load with a real ephemeral reply
// key and returns the response's error.message (empty when the response
// carried no error). Transport errors are stringified so they enter the
// same canary scan.
func probeErrorMessage(ctx context.Context, h *Harness, name string) string {
	kp, err := newCurveKey()
	if err != nil {
		return err.Error()
	}
	defer kp.Wipe()

	pub, err := kp.PublicKey()
	if err != nil {
		return err.Error()
	}

	reqID, err := newHexID()
	if err != nil {
		return err.Error()
	}

	var resp svcbackend.LoadResponse
	err = rawRequest(ctx, h, h.Prefix+".ctx.load."+name, svcbackend.LoadRequest{
		ReplyPub: pub,
		ReqID:    reqID,
	}, &resp)
	if err != nil {
		return err.Error()
	}
	if resp.Error == nil {
		return ""
	}
	return resp.Error.Message
}
