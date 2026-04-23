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

// svcbackend-conformance verifies that a running natscontext svcbackend
// server satisfies the PROTOCOL.md v1 conformance checklist. It connects
// to the server via a named nats context, exercises every subject, and
// emits a table of PASS / WARN / FAIL / SKIP results.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/choria-io/fisk"

	"github.com/nats-io/jsm.go/natscontext"
	"github.com/nats-io/jsm.go/natscontext/svcbackend"
)

var version = "dev"

func main() {
	app := fisk.New("svcbackend-conformance", "Verify a natscontext svcbackend server against PROTOCOL.md v1")
	app.Author("The NATS Authors <info@nats.io>")
	app.Version(version)
	app.HelpFlag.Short('h')
	app.UsageWriter(os.Stdout)

	var (
		ctxName     string
		prefix      string
		mode        string
		namespace   string
		timeout     time.Duration
		concurrency int
		iterations  int
		logFile     string
		keep        bool
		jsonOut     bool
		verbose     bool
		noColor     bool
		noProgress  bool
	)

	app.Flag("context", "NATS context name to connect with").Short('c').Required().StringVar(&ctxName)
	app.Flag("prefix", "Subject prefix the server was configured with").Default(svcbackend.DefaultPrefix).StringVar(&prefix)
	app.Flag("mode", "Expected server mode (rw, ro, no-sel)").Default("rw").EnumVar(&mode, "rw", "ro", "no-sel")
	app.Flag("namespace", "Prefix used for test-created context names").Default("__conf_").StringVar(&namespace)
	app.Flag("timeout", "Per-request timeout").Default("5s").DurationVar(&timeout)
	app.Flag("concurrency", "Workers for atomicity stress tests").Default("16").IntVar(&concurrency)
	app.Flag("iterations", "Rounds for atomicity stress tests").Default("200").IntVar(&iterations)
	app.Flag("log-file", "Server log file to scan for hygiene checks (optional)").ExistingFileVar(&logFile)
	app.Flag("keep", "Skip cleanup of test-created contexts on exit").BoolVar(&keep)
	app.Flag("json", "Emit a machine-readable JSON report").BoolVar(&jsonOut)
	app.Flag("verbose", "Show PASS rows in addition to WARN/FAIL/SKIP").Short('v').BoolVar(&verbose)
	app.Flag("no-color", "Disable colored output").BoolVar(&noColor)
	app.Flag("no-progress", "Disable live per-check progress on stderr").BoolVar(&noProgress)

	_, err := app.Parse(os.Args[1:])
	fisk.FatalIfError(err, "parse arguments")

	nc, err := natscontext.Connect(ctxName)
	fisk.FatalIfError(err, "connect via context %q", ctxName)
	defer nc.Close()

	client, err := svcbackend.NewClient(nc,
		svcbackend.WithSubjectPrefix(prefix),
		svcbackend.WithTimeout(timeout),
	)
	fisk.FatalIfError(err, "svcbackend client")
	defer client.Close()

	h := &Harness{
		NC:          nc,
		Client:      client,
		Prefix:      prefix,
		Mode:        mode,
		Namespace:   namespace,
		Timeout:     timeout,
		Concurrency: concurrency,
		Iterations:  iterations,
		LogFile:     logFile,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	var progress Progress
	if !noProgress {
		progress = NewProgress(os.Stderr, !noColor)
	}

	report := Run(ctx, h, AllChecks(), progress)

	if !keep {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		h.Cleanup(cleanupCtx)
		cleanupCancel()
	}

	if jsonOut {
		err = report.WriteJSON(os.Stdout)
		fisk.FatalIfError(err, "write json")
	} else {
		err = report.WriteText(os.Stdout, verbose, !noColor)
		fisk.FatalIfError(err, "write text")
	}

	if report.HasFailures() {
		fmt.Fprintln(os.Stderr, "conformance failed")
		os.Exit(1)
	}
}
