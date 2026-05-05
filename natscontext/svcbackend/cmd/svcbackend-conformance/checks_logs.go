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
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nats-io/jsm.go/natscontext/svcbackend"
)

// logChecks covers PROTOCOL.md §10 / Conformance "Logging". They require
// operator cooperation via --log-file and therefore default to SKIP.
func logChecks() []Check {
	return []Check{
		{
			ID: "logs.no_payload_in_log", Section: "Logging",
			Title: "payload bytes do not appear in the server log",
			Modes: []string{"rw"},
			Needs: []string{"log-file"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				// Save a name with a recognizable canary, then grep the
				// log for it. If it appears, the server logged payload
				// bytes -- a hygiene violation.
				canary := fmt.Sprintf("CONFORMANCE-CANARY-LOG-PROBE-%d", os.Getpid())
				name := h.MintName("log_probe")

				err := h.Client.Save(ctx, name, []byte(canary))
				if err != nil {
					return StatusFail, "save canary: " + err.Error(), nil
				}
				_, err = h.Client.Load(ctx, name)
				if err != nil {
					return StatusFail, "load canary: " + err.Error(), nil
				}

				hit, lineErr := grepLog(h.LogFile, canary)
				if lineErr != nil {
					return StatusFail, "scan log: " + lineErr.Error(), nil
				}
				if hit != "" {
					return StatusFail, "canary found in log: " + hit, nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "logs.req_id_logged", Section: "Logging",
			Title: "req_id appears in the server log for recent requests",
			Modes: []string{"rw"},
			Needs: []string{"log-file"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				// Issue a save with a known req_id, then grep for it.
				// This check is recommended (SHOULD), not required; a
				// missing entry reports WARN rather than FAIL.
				reqID, err := newHexID()
				if err != nil {
					return StatusFail, err.Error(), nil
				}

				// Drive a raw save so we control the req_id.
				name := h.MintName("reqid")
				serverPub, pubErr := sysXKey(ctx, h)
				if pubErr != nil {
					return StatusFail, "sys.xkey: " + pubErr.Error(), nil
				}

				eph, ekErr := newCurveKey()
				if ekErr != nil {
					return StatusFail, ekErr.Error(), nil
				}
				defer eph.Wipe()

				ephPub, ephErr := eph.PublicKey()
				if ephErr != nil {
					return StatusFail, ephErr.Error(), nil
				}

				sealed, sealErr := sealSave(eph, serverPub, svcbackend.SaveSealed{Data: []byte("log-probe"), ReqID: reqID})
				if sealErr != nil {
					return StatusFail, sealErr.Error(), nil
				}

				var resp svcbackend.SaveResponse
				rawErr := rawRequest(ctx, h, h.Prefix+".ctx.save."+name, svcbackend.SaveRequest{SenderPub: ephPub, Sealed: sealed}, &resp)
				if rawErr != nil {
					return StatusFail, "save: " + rawErr.Error(), nil
				}
				if resp.Error != nil {
					return StatusFail, "save: " + resp.Error.Code, nil
				}

				hit, lineErr := grepLog(h.LogFile, reqID)
				if lineErr != nil {
					return StatusFail, "scan log: " + lineErr.Error(), nil
				}
				if hit == "" {
					return StatusWarn, "req_id not found in log (SHOULD requirement)", nil
				}
				return StatusPass, "", nil
			},
		},
	}
}

// grepLog returns the first line of path that contains needle, or "" if
// not found. Lines longer than 64 KiB are truncated; log hygiene probes
// don't care about monster lines.
func grepLog(path, needle string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, needle) {
			return line, nil
		}
	}
	return "", sc.Err()
}
