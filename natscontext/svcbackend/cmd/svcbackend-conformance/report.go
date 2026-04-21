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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/nats-io/natscli/columns"
	terminal "golang.org/x/term"
)

// Report is the aggregate outcome of a conformance run.
type Report struct {
	Mode       string    `json:"mode"`
	Prefix     string    `json:"prefix"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Results    []Result  `json:"results"`
}

// HasFailures returns true when any result is FAIL.
func (r *Report) HasFailures() bool {
	for _, res := range r.Results {
		if res.Status == StatusFail {
			return true
		}
	}
	return false
}

// Counts returns a tally of results by Status.
func (r *Report) Counts() map[Status]int {
	out := map[Status]int{StatusPass: 0, StatusWarn: 0, StatusFail: 0, StatusSkip: 0}
	for _, res := range r.Results {
		out[res.Status]++
	}
	return out
}

// WriteJSON emits the full report as indented JSON.
func (r *Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteText renders the report as a columns-style summary header and a
// go-pretty table of per-check outcomes. Pass verbose=true to include
// PASS rows; otherwise only WARN/FAIL/SKIP are shown.
func (r *Report) WriteText(w io.Writer, verbose, color bool) error {
	counts := r.Counts()
	elapsed := r.FinishedAt.Sub(r.StartedAt).Round(time.Millisecond)

	cw := columns.New("natscontext svcbackend conformance")
	cw.AddRow("Subject prefix", r.Prefix)
	cw.AddRow("Mode", r.Mode)
	cw.AddRow("Started", r.StartedAt.Format(time.RFC3339))
	cw.AddRow("Elapsed", elapsed.String())
	cw.AddSectionTitle("Totals")
	cw.AddRow("Pass", counts[StatusPass])
	cw.AddRow("Warn", counts[StatusWarn])
	cw.AddRow("Fail", counts[StatusFail])
	cw.AddRow("Skip", counts[StatusSkip])

	err := cw.Frender(w)
	if err != nil {
		return err
	}

	fmt.Fprintln(w)

	tbl := table.NewWriter()
	tbl.SetStyle(table.StyleRounded)
	tbl.Style().Title.Align = text.AlignCenter
	tbl.Style().Format.Header = text.FormatDefault
	tbl.SetTitle("Conformance Results")
	tbl.AppendHeader(table.Row{"Status", "Section", "Check", "Detail", "Time"})

	useColor := color && isTerminal(w)

	var section string
	for _, res := range r.Results {
		if !verbose && res.Status == StatusPass {
			continue
		}

		if res.Section != section && section != "" {
			tbl.AppendSeparator()
		}
		section = res.Section

		tbl.AppendRow(table.Row{
			renderStatus(res.Status, useColor),
			res.Section,
			res.Title,
			truncate(res.Detail, 80),
			res.Elapsed.Round(time.Millisecond),
		})
	}

	// When --verbose is off and every check passed the body is empty.
	// Emit a one-liner so the user isn't confused by the empty frame.
	if !verbose && counts[StatusWarn]+counts[StatusFail]+counts[StatusSkip] == 0 {
		fmt.Fprintln(w, "All checks passed. Re-run with --verbose for per-check detail.")
		return nil
	}

	fmt.Fprintln(w, tbl.Render())
	return nil
}

// renderStatus returns the status string, optionally colorized.
func renderStatus(s Status, useColor bool) string {
	if !useColor {
		return string(s)
	}
	switch s {
	case StatusPass:
		return text.FgGreen.Sprint(string(s))
	case StatusWarn:
		return text.FgYellow.Sprint(string(s))
	case StatusFail:
		return text.FgRed.Sprint(string(s))
	case StatusSkip:
		return text.FgCyan.Sprint(string(s))
	}
	return string(s)
}

// truncate shortens s to max runes with an ellipsis when clipped. go-pretty
// word-wraps on its own, but very long single-token details (e.g. base64
// blobs) produce ugly tables; clipping here keeps the output readable.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// isTerminal reports whether w is an *os.File backed by a terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return terminal.IsTerminal(int(f.Fd()))
}
