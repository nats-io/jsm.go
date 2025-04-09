// Copyright 2024 The NATS Authors
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

package monitor

import (
	"bytes"
	"regexp"
	"strconv"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
)

// CheckStreamMessageOptions configures the stream message check
type CheckStreamMessageOptions struct {
	// StreamName the name of the stream holding the message
	StreamName string `json:"stream_name" yaml:"stream_name"`
	// Subject the name of the subject to get the message from
	Subject string `json:"subject" yaml:"subject"`
	// AgeWarning warning threshold for message age
	AgeWarning float64 `json:"age_warning" yaml:"age_warning"`
	// AgeCritical critical threshold for message age
	AgeCritical float64 `json:"age_critical" yaml:"age_critical"`
	// Content regular expression body must match against
	Content string `json:"content_regex" yaml:"content_regex"`
	// BodyAsTimestamp use the body as a unix timestamp instead of message timestamp when calculating age
	BodyAsTimestamp bool `json:"body_as_timestamp" yaml:"body_as_timestamp"`
}

func CheckStreamMessage(server string, nopts []nats.Option, jsmOpts []jsm.Option, check *Result, opts CheckStreamMessageOptions) error {
	nc, err := nats.Connect(server, nopts...)
	if check.CriticalIfErr(err, "could not load info: %v", err) {
		return nil
	}
	defer nc.Close()

	mgr, err := jsm.New(nc, jsmOpts...)
	if check.CriticalIfErr(err, "could not load info: %v", err) {
		return nil
	}

	return CheckStreamMessageWithConnection(mgr, check, opts)
}

func CheckStreamMessageWithConnection(mgr *jsm.Manager, check *Result, opts CheckStreamMessageOptions) error {
	msg, err := mgr.ReadLastMessageForSubject(opts.StreamName, opts.Subject)
	if api.IsNatsError(err, 10037) {
		check.Critical("no message found")
		return nil
	}
	if check.CriticalIfErr(err, "msg load failed: %v", err) {
		return nil
	}

	ts := msg.Time
	if opts.BodyAsTimestamp {
		i, err := strconv.ParseInt(string(bytes.TrimSpace(msg.Data)), 10, 64)
		check.CriticalIfErr(err, "invalid timestamp body: %v", err)
		ts = time.Unix(i, 0)
	}

	check.Pd(&PerfDataItem{
		Help:  "The age of the message",
		Name:  "age",
		Value: time.Since(ts).Round(time.Millisecond).Seconds(),
		Warn:  opts.AgeWarning,
		Crit:  opts.AgeCritical,
		Unit:  "s",
	})

	check.Pd(&PerfDataItem{
		Help:  "The size of the message",
		Name:  "size",
		Value: float64(len(msg.Data)),
		Unit:  "B",
	})

	since := time.Since(ts)

	if opts.AgeCritical > 0 && since > secondsToDuration(opts.AgeCritical) {
		check.Critical("%v old", since.Round(time.Millisecond))
	} else if opts.AgeWarning > 0 && since > secondsToDuration(opts.AgeWarning) {
		check.Warn("%v old", time.Since(ts).Round(time.Millisecond))
	}

	if opts.Content != "" {
		re, err := regexp.Compile(opts.Content)
		if check.CriticalIfErr(err, "content regex compile failed: %v", err) {
			return nil
		}

		if !re.Match(msg.Data) {
			check.Critical("does not match regex: %s", re.String())
		}
	}

	check.OkIfNoWarningsOrCriticals("Valid message on %s > %s", opts.StreamName, opts.Subject)

	return nil
}
