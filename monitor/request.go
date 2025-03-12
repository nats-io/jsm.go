// Copyright 2025 The NATS Authors
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
	"regexp"
	"time"

	"github.com/nats-io/nats.go"
)

type CheckRequestOptions struct {
	// Subject is the subject to send the request to
	Subject string `json:"subject" yaml:"subject"`
	// Payload is the payload to send to the service
	Payload string `json:"payload" yaml:"payload"`
	// Header is headers to send with the request
	Header map[string]string `json:"header" yaml:"header"`
	//HeaderMatch to send in the payload
	HeaderMatch map[string]string `json:"headers" yaml:"headers"`
	// ResponseMatch applies regular expression match against the payload
	ResponseMatch string `json:"response_match" yaml:"response_match"`
	// ResponseTimeWarn warns when the response takes longer than a certain time
	ResponseTimeWarn time.Duration `json:"response_time_warn" yaml:"response_time_warn"`
	// ResponseTimeCritical logs critical when the response takes longer than a certain time
	ResponseTimeCritical time.Duration `json:"response_time_crit" yaml:"response_time_crit"`
}

func CheckRequest(server string, nopts []nats.Option, check *Result, timeout time.Duration, opts CheckRequestOptions) error {
	nc, err := nats.Connect(server, nopts...)
	if check.CriticalIfErr(err, "could not load info: %v", err) {
		return nil
	}

	return CheckRequestWithConnection(nc, check, timeout, opts)
}

func CheckRequestWithConnection(nc *nats.Conn, check *Result, timeout time.Duration, opts CheckRequestOptions) error {
	if opts.Subject == "" {
		check.Critical("no subject specified")
		return nil
	}

	msg := nats.NewMsg(opts.Subject)
	msg.Data = []byte(opts.Payload)
	for k, v := range opts.Header {
		msg.Header.Add(k, v)
	}

	start := time.Now()
	resp, err := nc.RequestMsg(msg, timeout)
	since := time.Since(start)

	check.Pd(&PerfDataItem{
		Help:  "How long the request took",
		Name:  "time",
		Value: float64(since.Round(time.Millisecond).Seconds()),
		Warn:  opts.ResponseTimeWarn.Seconds(),
		Crit:  opts.ResponseTimeCritical.Seconds(),
		Unit:  "s",
	})
	if check.CriticalIfErr(err, "could not send request: %v", err) {
		return nil
	}

	if opts.ResponseMatch != "" {
		re, err := regexp.Compile(opts.ResponseMatch)
		if check.CriticalIfErr(err, "content regex compile failed: %v", err) {
			return nil
		}

		if !re.Match(resp.Data) {
			check.Critical("response does not match regexp")
		}
	}

	for k, v := range opts.HeaderMatch {
		rv := resp.Header.Get(k)
		if rv != v {
			check.Critical("invalid header %q = %q", k, rv)
		}
	}

	if opts.ResponseTimeCritical > 0 && since > opts.ResponseTimeCritical {
		check.Critical("response took %v", since.Round(time.Millisecond))
	} else if opts.ResponseTimeWarn > 0 && since > opts.ResponseTimeWarn {
		check.Warn("response took %v", since.Round(time.Millisecond))
	}

	check.OkIfNoWarningsOrCriticals("Valid response")

	return nil
}
