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
	"fmt"
	"io"
	"time"
)

// Progress receives live notifications as the runner works through the
// check list. A nil Progress disables live output entirely.
type Progress interface {
	// Start is called immediately before a check begins. idx is 1-based.
	Start(idx, total int, c Check)

	// Done is called once the check has produced a Result, including
	// mode/needs/cancel skips that never executed c.Run.
	Done(idx, total int, r Result)

	// Finish is called once, after the last Done, so TTY implementations
	// can clear any in-place status line before the report is rendered.
	Finish()
}

// NewProgress returns a TTY progress renderer when w is a terminal, a
// plain line renderer otherwise. Color is honored only on TTYs.
func NewProgress(w io.Writer, color bool) Progress {
	if w == nil {
		return nil
	}

	if isTerminal(w) {
		return &ttyProgress{w: w, color: color}
	}

	return &lineProgress{w: w}
}

// ttyProgress shows a single live line for the currently-running check
// and replaces it with the result line when the check ends. Uses CR and
// the ANSI "erase-to-end-of-line" sequence to avoid leftover characters
// when a shorter line overwrites a longer one.
type ttyProgress struct {
	w     io.Writer
	color bool
}

func (p *ttyProgress) Start(idx, total int, c Check) {
	fmt.Fprintf(p.w, "\r\x1b[K     [%*d/%d] %s / %s ...",
		digits(total), idx, total, c.Section, c.Title)
}

func (p *ttyProgress) Done(idx, total int, r Result) {
	status := renderStatus(r.Status, p.color)

	detail := ""
	if r.Status != StatusPass && r.Detail != "" {
		detail = "  " + truncate(r.Detail, 60)
	}

	fmt.Fprintf(p.w, "\r\x1b[K%s [%*d/%d] %s / %s  (%s)%s\n",
		status, digits(total), idx, total, r.Section, r.Title,
		r.Elapsed.Round(time.Millisecond), detail)
}

func (p *ttyProgress) Finish() {
	fmt.Fprint(p.w, "\r\x1b[K")
}

// lineProgress prints one line per result, no in-place updates. Safe
// for pipes, CI logs, and non-interactive terminals.
type lineProgress struct {
	w io.Writer
}

func (p *lineProgress) Start(idx, total int, c Check) {}

func (p *lineProgress) Done(idx, total int, r Result) {
	detail := ""
	if r.Status != StatusPass && r.Detail != "" {
		detail = "  " + truncate(r.Detail, 80)
	}

	fmt.Fprintf(p.w, "%-4s [%*d/%d] %s / %s  (%s)%s\n",
		string(r.Status), digits(total), idx, total, r.Section, r.Title,
		r.Elapsed.Round(time.Millisecond), detail)
}

func (p *lineProgress) Finish() {}

// digits returns the number of decimal digits needed to render n, so
// the [idx/total] column can be right-aligned.
func digits(n int) int {
	d := 1
	for n >= 10 {
		n /= 10
		d++
	}

	return d
}
