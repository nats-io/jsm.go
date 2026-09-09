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

package backup

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats-server/v2/server"
)

// EditOption configures Edit; options are validated when Edit runs
type EditOption func(*editOptions)

type headerMatch struct {
	name  string
	value string
}

type editOptions struct {
	subjects            []string
	excludeSubjects     []string
	after               time.Time
	before              time.Time
	hasAfter            bool
	hasBefore           bool
	firstSeq            uint64
	lastSeq             uint64
	hasFirstSeq         bool
	hasLastSeq          bool
	headerPresent       []string
	headerValues        []headerMatch
	noHeader            []string
	payloadMatch        []*regexp.Regexp
	excludePayloadMatch []*regexp.Regexp
	lastPerSubject      int
	hasLastPerSubject   bool
	kvCompact           bool
	renumber            bool
	obfuscate           bool
	keyFile             string
	dryRun              bool
	toolVersion         string
	betweenPasses       func()
}

// Subjects keeps messages whose subject matches at least one of the given NATS subject patterns
func Subjects(subjects ...string) EditOption {
	return func(o *editOptions) { o.subjects = append(o.subjects, subjects...) }
}

// ExcludeSubjects drops messages whose subject matches any of the given NATS subject patterns
func ExcludeSubjects(subjects ...string) EditOption {
	return func(o *editOptions) { o.excludeSubjects = append(o.excludeSubjects, subjects...) }
}

// After keeps messages whose timestamp is at or after t
func After(t time.Time) EditOption {
	return func(o *editOptions) { o.after, o.hasAfter = t, true }
}

// Before keeps messages whose timestamp is before t
func Before(t time.Time) EditOption {
	return func(o *editOptions) { o.before, o.hasBefore = t, true }
}

// FirstSeq keeps messages with a sequence at or above seq
func FirstSeq(seq uint64) EditOption {
	return func(o *editOptions) { o.firstSeq, o.hasFirstSeq = seq, true }
}

// LastSeq keeps messages with a sequence at or below seq
func LastSeq(seq uint64) EditOption {
	return func(o *editOptions) { o.lastSeq, o.hasLastSeq = seq, true }
}

// HeaderPresent keeps messages carrying the named header with at least one value
func HeaderPresent(name string) EditOption {
	return func(o *editOptions) { o.headerPresent = append(o.headerPresent, name) }
}

// HeaderValue keeps messages where any value of the named header equals value exactly
func HeaderValue(name string, value string) EditOption {
	return func(o *editOptions) { o.headerValues = append(o.headerValues, headerMatch{name: name, value: value}) }
}

// NoHeader drops messages carrying the named header
func NoHeader(name string) EditOption {
	return func(o *editOptions) { o.noHeader = append(o.noHeader, name) }
}

// PayloadMatch keeps messages whose payload matches at least one of the given expressions
func PayloadMatch(re *regexp.Regexp) EditOption {
	return func(o *editOptions) { o.payloadMatch = append(o.payloadMatch, re) }
}

// ExcludePayloadMatch drops messages whose payload matches any of the given expressions
func ExcludePayloadMatch(re *regexp.Regexp) EditOption {
	return func(o *editOptions) { o.excludePayloadMatch = append(o.excludePayloadMatch, re) }
}

// LastPerSubject keeps only the newest n messages of every subject in the filtered result
func LastPerSubject(n int) EditOption {
	return func(o *editOptions) { o.lastPerSubject, o.hasLastPerSubject = n, true }
}

// KVCompact reduces a KV bucket backup to the latest revision of every key,
// dropping keys whose latest revision is a delete or purge marker. It is
// applied to the filtered result and requires a KV bucket backup
func KVCompact() EditOption {
	return func(o *editOptions) { o.kvCompact = true }
}

// Renumber numbers the kept messages 1..K and drops all consumers
func Renumber() EditOption {
	return func(o *editOptions) { o.renumber = true }
}

// Obfuscate replaces identifying names and subjects with keyed hashes and
// drops message bodies, writing the reverse mapping to a key file beside
// the target. On a counter stream the bodies are the running totals a
// restored counter needs, so they are kept and the values stay readable
func Obfuscate() EditOption {
	return func(o *editOptions) { o.obfuscate = true }
}

// ObfuscationKeyFile reuses the secret and mappings from an earlier key file so
// several backups obfuscate consistently
func ObfuscationKeyFile(path string) EditOption {
	return func(o *editOptions) { o.keyFile = path }
}

// betweenPasses runs after the first pass of a two-pass edit; tests use it
// to alter the source underneath the held handle
func betweenPasses(fn func()) EditOption {
	return func(o *editOptions) { o.betweenPasses = fn }
}

// DryRun computes the Result without writing anything
func DryRun() EditOption {
	return func(o *editOptions) { o.dryRun = true }
}

// ToolVersion records the calling tool's version in the meta file's edit
// block; without it the edit block carries no version
func ToolVersion(v string) EditOption {
	return func(o *editOptions) { o.toolVersion = v }
}

func (o *editOptions) validate() error {
	for _, s := range slices.Concat(o.subjects, o.excludeSubjects) {
		if !server.IsValidSubject(s) {
			return fmt.Errorf("invalid subject filter %q", s)
		}
	}
	nanoMin, nanoMax := time.Unix(0, math.MinInt64), time.Unix(0, math.MaxInt64)
	for _, tc := range []struct {
		set  bool
		name string
		t    time.Time
	}{{o.hasAfter, "After", o.after}, {o.hasBefore, "Before", o.before}} {
		if tc.set && (tc.t.Before(nanoMin) || tc.t.After(nanoMax)) {
			return fmt.Errorf("%s %s is outside the supported time range", tc.name, tc.t.Format(time.RFC3339))
		}
	}
	if o.hasAfter && o.hasBefore && !o.after.Before(o.before) {
		return errors.New("After must be earlier than Before")
	}
	if o.hasFirstSeq && o.hasLastSeq && o.firstSeq > o.lastSeq {
		return fmt.Errorf("FirstSeq %d is above LastSeq %d", o.firstSeq, o.lastSeq)
	}
	for _, name := range slices.Concat(o.headerPresent, o.noHeader) {
		if err := validateHeaderName(name); err != nil {
			return err
		}
	}
	for _, hm := range o.headerValues {
		if err := validateHeaderName(hm.name); err != nil {
			return err
		}
		if strings.ContainsAny(hm.value, "\r\n") {
			return fmt.Errorf("invalid header value %q", hm.value)
		}
	}
	for _, re := range slices.Concat(o.payloadMatch, o.excludePayloadMatch) {
		if re == nil {
			return errors.New("payload expression is nil")
		}
	}
	if o.hasLastPerSubject && o.lastPerSubject < 1 {
		return fmt.Errorf("LastPerSubject requires at least 1, got %d", o.lastPerSubject)
	}
	if o.kvCompact && o.lastPerSubject > 0 {
		return errors.New("KVCompact and LastPerSubject are mutually exclusive")
	}
	if o.keyFile != "" && !o.obfuscate {
		return errors.New("ObfuscationKeyFile requires Obfuscate")
	}

	return nil
}

func validateHeaderName(name string) error {
	if name == "" || strings.ContainsAny(name, ": \t\r\n") {
		return fmt.Errorf("invalid header name %q", name)
	}
	return nil
}

func (o *editOptions) perSubject() bool {
	return o.lastPerSubject > 0 || o.kvCompact
}

func (o *editOptions) hasHeaderFilters() bool {
	return len(o.headerPresent)+len(o.headerValues)+len(o.noHeader) > 0
}

func (o *editOptions) hasPayloadFilters() bool {
	return len(o.payloadMatch)+len(o.excludePayloadMatch) > 0
}

// canonical renders the options in a fixed order so equal option sets produce equal meta files
func (o *editOptions) canonical() []string {
	var out []string
	add := func(key string, values []string) {
		values = slices.Clone(values)
		slices.Sort(values)
		for _, v := range values {
			out = append(out, key+"="+v)
		}
	}

	add("subject", o.subjects)
	add("exclude-subject", o.excludeSubjects)
	if o.hasAfter {
		out = append(out, "after="+o.after.UTC().Format(time.RFC3339Nano))
	}
	if o.hasBefore {
		out = append(out, "before="+o.before.UTC().Format(time.RFC3339Nano))
	}
	if o.hasFirstSeq {
		out = append(out, "first-seq="+strconv.FormatUint(o.firstSeq, 10))
	}
	if o.hasLastSeq {
		out = append(out, "last-seq="+strconv.FormatUint(o.lastSeq, 10))
	}
	add("header", o.headerPresent)
	hv := make([]string, 0, len(o.headerValues))
	for _, hm := range o.headerValues {
		hv = append(hv, hm.name+":"+hm.value)
	}
	add("header", hv)
	add("no-header", o.noHeader)
	add("payload-match", regexStrings(o.payloadMatch))
	add("exclude-payload-match", regexStrings(o.excludePayloadMatch))
	if o.lastPerSubject > 0 {
		out = append(out, "last-per-subject="+strconv.Itoa(o.lastPerSubject))
	}
	if o.kvCompact {
		out = append(out, "kv-compact")
	}
	if o.renumber {
		out = append(out, "renumber")
	}
	if o.obfuscate {
		out = append(out, "obfuscate")
	}
	if out == nil {
		out = []string{}
	}

	return out
}

func regexStrings(res []*regexp.Regexp) []string {
	out := make([]string, 0, len(res))
	for _, re := range res {
		out = append(out, re.String())
	}
	return out
}
