// Copyright 2020 The NATS Authors
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

// Package jsm provides client helpers for managing and interacting with NATS JetStream
package jsm

//go:generate go run api/gen.go

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

// ErrNoExprLangBuild warns that expression matching is disabled when compiling
// a go binary with the `noexprlang` build tag.
var ErrNoExprLangBuild = fmt.Errorf("binary has been built with `noexprlang` build tag and thus does not support expression matching")

// standard api responses with error embedded
type jetStreamResponseError interface {
	ToError() error
}

// jetstream iterable responses
type apiIterableResponse interface {
	ItemsTotal() int
	ItemsOffset() int
	ItemsLimit() int
	LastPage() bool
}

// jetstream iterable requests
type apiIterableRequest interface {
	SetOffset(o int)
}

// all types generated using the api/gen.go which includes all
// the jetstream api types.  Validate() will force validator all
// of these on every jsonRequest
type apiValidatable interface {
	Validate(...api.StructValidator) (valid bool, errors []string)
	SchemaType() string
}

// IsErrorResponse checks if the message holds a standard JetStream error
func IsErrorResponse(m *nats.Msg) bool {
	if strings.HasPrefix(string(m.Data), api.ErrPrefix) {
		return true
	}

	resp := api.JSApiResponse{}
	err := json.Unmarshal(m.Data, &resp)
	if err != nil {
		return false
	}

	return resp.IsError()
}

// ParseErrorResponse parses the JetStream response, if it's an error returns an error instance holding the message else nil
func ParseErrorResponse(m *nats.Msg) error {
	if !IsErrorResponse(m) {
		return nil
	}

	d := string(m.Data)
	if strings.HasPrefix(d, api.ErrPrefix) {
		return errors.New(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(d, api.ErrPrefix), " '"), "'"))
	}

	resp := api.JSApiResponse{}
	err := json.Unmarshal(m.Data, &resp)
	if err != nil {
		return err
	}

	return resp.ToError()
}

// IsOKResponse checks if the message holds a standard JetStream error
func IsOKResponse(m *nats.Msg) bool {
	if strings.HasPrefix(string(m.Data), api.OK) {
		return true
	}

	resp := api.JSApiResponse{}
	err := json.Unmarshal(m.Data, &resp)
	if err != nil {
		return false
	}

	return !resp.IsError()
}

// IsValidName verifies if n is a valid stream, template or consumer name
func IsValidName(n string) bool {
	if n == "" {
		return false
	}

	return !strings.ContainsAny(n, ">*. /\\")
}

// APISubject returns API subject with prefix applied
func APISubject(subject string, prefix string, domain string) string {
	if domain != "" {
		return fmt.Sprintf("$JS.%s.API", domain) + strings.TrimPrefix(subject, "$JS.API")
	}

	if prefix == "" {
		return subject
	}

	return prefix + strings.TrimPrefix(subject, "$JS.API")
}

// EventSubject returns Event subject with prefix applied
func EventSubject(subject string, prefix string) string {
	if prefix == "" {
		return subject
	}

	return prefix + strings.TrimPrefix(subject, "$JS.EVENT")
}

// ParsePubAck parses a stream publish response and returns an error if the publish failed or parsing failed
func ParsePubAck(m *nats.Msg) (*api.PubAck, error) {
	if m == nil {
		return nil, fmt.Errorf("no message supplied")
	}

	err := ParseErrorResponse(m)
	if err != nil {
		return nil, err
	}

	res := api.PubAck{}
	err = json.Unmarshal(m.Data, &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

// IsNatsError checks if err is a ApiErr matching code
func IsNatsError(err error, code uint16) bool {
	return api.IsNatsError(err, code)
}

// IsInternalStream indicates if a stream is considered 'internal' by the NATS team,
// that is, it's a backing stream for KV, Object or MQTT state
func IsInternalStream(s string) bool {
	return IsKVBucketStream(s) || IsObjectBucketStream(s) || IsMQTTStateStream(s)
}

// IsKVBucketStream determines if a stream is a KV bucket
func IsKVBucketStream(s string) bool {
	return strings.HasPrefix(s, "KV_")
}

// IsObjectBucketStream determines if a stream is a Object bucket
func IsObjectBucketStream(s string) bool {
	return strings.HasPrefix(s, "OBJ_")
}

// IsMQTTStateStream determines if a stream holds internal MQTT state
func IsMQTTStateStream(s string) bool {
	return strings.HasPrefix(s, "$MQTT_")
}

// LinearBackoffPeriods creates a backoff policy without any jitter suitable for use in a consumer backoff policy
//
// The periods start from min and increase linearly until ~max
func LinearBackoffPeriods(steps uint, min time.Duration, max time.Duration) ([]time.Duration, error) {
	if steps == 0 {
		return nil, fmt.Errorf("steps must be more than 0")
	}
	if min == 0 {
		return nil, fmt.Errorf("minimum retry can not be 0")
	}
	if max == 0 {
		return nil, fmt.Errorf("maximum retry can not be 0")
	}

	if max < min {
		max, min = min, max
	}

	var res []time.Duration

	stepSize := uint(max-min) / steps
	for i := uint(0); i < steps; i += 1 {
		res = append(res, min+time.Duration(i*stepSize).Round(time.Millisecond))
	}

	return res, nil
}

var (
	durationMatcher    = regexp.MustCompile(`([-+]?)(([\d\.]+)([a-zA-Z]+))`)
	errInvalidDuration = fmt.Errorf("invalid duration")
)

// ParseDuration parse durations with additional units over those from
// standard go parser.
//
// In addition to normal go parser time units it also supports
// these.
//
// The reason these are not in go standard lib is due to precision around
// how many days in a month and about leap years and leap seconds. This
// function does nothing to try and correct for those.
//
// * "w", "W" - a week based on 7 days of exactly 24 hours
// * "d", "D" - a day based on 24 hours
// * "M" - a month made of 30 days of 24 hours
// * "y", "Y" - a year made of 365 days of 24 hours each
//
// Valid duration strings can be -1y1d1µs
func ParseDuration(d string) (time.Duration, error) {
	// golang time.ParseDuration has a special case for 0
	if d == "0" {
		return 0 * time.Second, nil
	}

	var (
		r   time.Duration
		neg = 1
	)

	d = strings.TrimSpace(d)

	if len(d) == 0 {
		return r, errInvalidDuration
	}

	parts := durationMatcher.FindAllStringSubmatch(d, -1)
	if len(parts) == 0 {
		return r, errInvalidDuration
	}

	for i, p := range parts {
		if len(p) != 5 {
			return 0, errInvalidDuration
		}

		if i == 0 && p[1] == "-" {
			neg = -1
		}

		switch p[4] {
		case "w", "W":
			val, err := strconv.ParseFloat(p[3], 32)
			if err != nil {
				return 0, fmt.Errorf("%w: %v", errInvalidDuration, err)
			}

			r += time.Duration(val*7*24) * time.Hour

		case "d", "D":
			val, err := strconv.ParseFloat(p[3], 32)
			if err != nil {
				return 0, fmt.Errorf("%w: %v", errInvalidDuration, err)
			}

			r += time.Duration(val*24) * time.Hour

		case "M":
			val, err := strconv.ParseFloat(p[3], 32)
			if err != nil {
				return 0, fmt.Errorf("%w: %v", errInvalidDuration, err)
			}

			r += time.Duration(val*24*30) * time.Hour

		case "Y", "y":
			val, err := strconv.ParseFloat(p[3], 32)
			if err != nil {
				return 0, fmt.Errorf("%w: %v", errInvalidDuration, err)
			}

			r += time.Duration(val*24*365) * time.Hour

		case "ns", "us", "µs", "ms", "s", "m", "h":
			dur, err := time.ParseDuration(p[2])
			if err != nil {
				return 0, fmt.Errorf("%w: %v", errInvalidDuration, err)
			}

			r += dur
		default:
			return 0, fmt.Errorf("%w: invalid unit %v", errInvalidDuration, p[4])
		}
	}

	return time.Duration(neg) * r, nil
}

// FilterServerMetadata copies metadata with the server generated metadata removed
func FilterServerMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}

	nm := map[string]string{}
	reserved := []string{
		api.JSMetaCurrentServerVersion,
		api.JSMetaCurrentServerLevel,
		api.JsMetaRequiredServerLevel,
	}

	for k, v := range metadata {
		if !slices.Contains(reserved, k) {
			nm[k] = v
		}
	}

	return nm
}
