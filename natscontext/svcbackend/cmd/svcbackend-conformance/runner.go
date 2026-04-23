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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/natscontext/svcbackend"
)

// Status is the outcome of a single check.
type Status string

const (
	StatusPass Status = "PASS"
	StatusFail Status = "FAIL"
	StatusWarn Status = "WARN"
	StatusSkip Status = "SKIP"
)

// Check is a single conformance verification.
type Check struct {
	// ID is a stable identifier used in JSON output, e.g. "subjects.sys_xkey".
	ID string

	// Section groups the check in the report ("Subjects", "Behavior", ...).
	Section string

	// Title is a one-line human description shown in the table.
	Title string

	// Modes restricts the check to the listed --mode values. Empty means
	// the check runs for every mode.
	Modes []string

	// Needs lists capability tags required to run. Unsatisfied needs turn
	// the check into a SKIP with a matching message. Supported tags:
	//   "log-file" - harness has a server log file configured
	Needs []string

	// Run performs the check. A returned error is reported as FAIL with
	// the error message as detail; use the Status+detail return for
	// non-error outcomes (PASS / WARN / SKIP).
	Run func(ctx context.Context, h *Harness) (Status, string, error)
}

// Result is the outcome of running one Check.
type Result struct {
	ID      string        `json:"id"`
	Section string        `json:"section"`
	Title   string        `json:"title"`
	Status  Status        `json:"status"`
	Detail  string        `json:"detail,omitempty"`
	Elapsed time.Duration `json:"elapsed_ns"`
}

// Harness carries everything a check needs. It owns connections and
// tracks test-created resources for cleanup.
type Harness struct {
	NC     *nats.Conn
	Client *svcbackend.Client

	Prefix      string
	Mode        string
	Namespace   string
	Timeout     time.Duration
	Concurrency int
	Iterations  int
	LogFile     string

	mu      sync.Mutex
	created map[string]struct{}
}

// MintName returns a unique context name prefixed with h.Namespace and
// the supplied tag. The name is recorded for later cleanup.
func (h *Harness) MintName(tag string) string {
	var raw [6]byte
	_, _ = rand.Read(raw[:])
	name := fmt.Sprintf("%s%s_%s", h.Namespace, tag, hex.EncodeToString(raw[:]))

	h.mu.Lock()
	if h.created == nil {
		h.created = map[string]struct{}{}
	}
	h.created[name] = struct{}{}
	h.mu.Unlock()

	return name
}

// Track marks an externally-known name for cleanup.
func (h *Harness) Track(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.created == nil {
		h.created = map[string]struct{}{}
	}
	h.created[name] = struct{}{}
}

// Untrack removes a name from the cleanup set, e.g. after an explicit
// Delete inside a check.
func (h *Harness) Untrack(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.created, name)
}

// Cleanup best-effort deletes every tracked name. Errors are ignored
// because the harness may be running against a --mode=ro server where
// delete is expected to fail.
func (h *Harness) Cleanup(ctx context.Context) {
	h.mu.Lock()
	names := make([]string, 0, len(h.created))
	for n := range h.created {
		names = append(names, n)
	}
	h.created = nil
	h.mu.Unlock()

	for _, n := range names {
		_ = h.Client.Delete(ctx, n)
	}
}

// appliesToMode returns true when c is eligible for h.Mode.
func (c Check) appliesToMode(mode string) bool {
	if len(c.Modes) == 0 {
		return true
	}
	for _, m := range c.Modes {
		if m == mode {
			return true
		}
	}
	return false
}

// unmetNeeds returns a human-readable skip reason when the harness does
// not satisfy c.Needs, or "" when every need is met.
func (c Check) unmetNeeds(h *Harness) string {
	for _, need := range c.Needs {
		switch need {
		case "log-file":
			if h.LogFile == "" {
				return "--log-file not set"
			}
		}
	}
	return ""
}

// Run executes every check, honoring mode and capability gating. Checks
// run sequentially: the protocol has enough internal ordering (write
// then read, etc.) that parallelizing adds bugs without saving time.
// A non-nil Progress receives Start/Done notifications as the run
// proceeds so callers can render live output on stderr.
func Run(ctx context.Context, h *Harness, checks []Check, p Progress) *Report {
	r := &Report{Mode: h.Mode, Prefix: h.Prefix, StartedAt: time.Now()}
	total := len(checks)

	for i, c := range checks {
		idx := i + 1

		if p != nil {
			p.Start(idx, total, c)
		}

		res := runOne(ctx, h, c)

		r.Results = append(r.Results, res)

		if p != nil {
			p.Done(idx, total, res)
		}
	}

	if p != nil {
		p.Finish()
	}

	r.FinishedAt = time.Now()
	return r
}

// runOne resolves a single check to a Result, short-circuiting on mode
// mismatch, unmet needs, or a canceled context before touching c.Run.
func runOne(ctx context.Context, h *Harness, c Check) Result {
	if !c.appliesToMode(h.Mode) {
		return Result{
			ID: c.ID, Section: c.Section, Title: c.Title,
			Status: StatusSkip,
			Detail: fmt.Sprintf("not applicable to --mode=%s", h.Mode),
		}
	}

	skip := c.unmetNeeds(h)
	if skip != "" {
		return Result{
			ID: c.ID, Section: c.Section, Title: c.Title,
			Status: StatusSkip,
			Detail: skip,
		}
	}

	select {
	case <-ctx.Done():
		return Result{
			ID: c.ID, Section: c.Section, Title: c.Title,
			Status: StatusSkip,
			Detail: "canceled",
		}
	default:
	}

	start := time.Now()
	status, detail, err := c.Run(ctx, h)
	elapsed := time.Since(start)

	if err != nil {
		status = StatusFail
		if detail == "" {
			detail = err.Error()
		} else {
			detail = detail + ": " + err.Error()
		}
	}

	return Result{
		ID: c.ID, Section: c.Section, Title: c.Title,
		Status:  status,
		Detail:  detail,
		Elapsed: elapsed,
	}
}
