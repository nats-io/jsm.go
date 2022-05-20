// Copyright 2022 The NATS Authors
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

package jsm

import (
	"regexp"
	"strings"
	"time"
)

type streamQuery struct {
	Server         *regexp.Regexp
	Cluster        *regexp.Regexp
	ConsumersLimit *int
	Empty          *bool
	IdlePeriod     *time.Duration
	CreatedPeriod  *time.Duration
	Invert         bool
	Subject        string
}

type StreamQueryOpt func(query *streamQuery) error

// StreamQuerySubjectWildcard limits results to streams with subject interest matching standard a nats wildcard
func StreamQuerySubjectWildcard(s string) StreamQueryOpt {
	return func(q *streamQuery) error {
		q.Subject = s

		return nil
	}
}

// StreamQueryServerName limits results to servers matching a regular expression
func StreamQueryServerName(s string) StreamQueryOpt {
	return func(q *streamQuery) error {
		if s == "" {
			return nil
		}

		re, err := regexp.Compile(s)
		if err != nil {
			return err
		}
		q.Server = re

		return nil
	}
}

// StreamQueryClusterName limits results to servers within a cluster matched by a regular expression
func StreamQueryClusterName(c string) StreamQueryOpt {
	return func(q *streamQuery) error {
		if c == "" {
			return nil
		}

		re, err := regexp.Compile(c)
		if err != nil {
			return err
		}
		q.Cluster = re

		return nil
	}
}

// StreamQueryFewerConsumersThan limits results to streams with fewer than or equal consumers than c
func StreamQueryFewerConsumersThan(c uint) StreamQueryOpt {
	return func(q *streamQuery) error {
		i := int(c)
		q.ConsumersLimit = &i
		return nil
	}
}

// StreamQueryWithoutMessages limits results to streams with no messages
func StreamQueryWithoutMessages() StreamQueryOpt {
	return func(q *streamQuery) error {
		t := true
		q.Empty = &t
		return nil
	}
}

// StreamQueryIdleLongerThan limits results to streams that has not received messages for a period longer than p
func StreamQueryIdleLongerThan(p time.Duration) StreamQueryOpt {
	return func(q *streamQuery) error {
		q.IdlePeriod = &p
		return nil
	}
}

// StreamQueryOlderThan limits the results to streams older than p
func StreamQueryOlderThan(p time.Duration) StreamQueryOpt {
	return func(q *streamQuery) error {
		q.CreatedPeriod = &p
		return nil
	}
}

// StreamQueryInvert inverts the logic of filters, older than becomes newer than and so forth
func StreamQueryInvert() StreamQueryOpt {
	return func(q *streamQuery) error {
		q.Invert = true
		return nil
	}
}

// QueryStreams filters the streams found in JetStream using various filter options
func (m *Manager) QueryStreams(opts ...StreamQueryOpt) ([]*Stream, error) {
	q := &streamQuery{}
	for _, opt := range opts {
		err := opt(q)
		if err != nil {
			return nil, err
		}
	}

	streams, err := m.Streams()
	if err != nil {
		return nil, err
	}

	return q.Filter(streams)
}

func (q *streamQuery) Filter(streams []*Stream) ([]*Stream, error) {
	return q.matchCreatedPeriod(
		q.matchIdlePeriod(
			q.matchEmpty(
				q.matchConsumerLimit(
					q.matchCluster(
						q.matchSubjectWildcard(
							q.matchServer(streams, nil),
						),
					),
				),
			),
		),
	)
}

func (q *streamQuery) matchSubjectWildcard(streams []*Stream, err error) ([]*Stream, error) {
	if err != nil {
		return nil, err
	}

	if q.Subject == "" {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		match := false
		for _, subj := range stream.Configuration().Subjects {
			subMatch := SubjectIsSubsetMatch(subj, q.Subject)

			if q.Invert {
				if !subMatch {
					match = true
				}
			} else {
				if subMatch {
					match = true
					break
				}
			}
		}

		if match {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchCreatedPeriod(streams []*Stream, err error) ([]*Stream, error) {
	if err != nil {
		return nil, err
	}

	if q.CreatedPeriod == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		nfo, err := stream.LatestInformation()
		if err != nil {
			return nil, err
		}

		if (!q.Invert && time.Since(nfo.Created) >= *q.CreatedPeriod) || (q.Invert && time.Since(nfo.Created) <= *q.CreatedPeriod) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

// note: ideally we match in addition for ones where no consumer had any messages in this period
// but today that means doing a consumer info on every consumer on every stream thats not viable
func (q *streamQuery) matchIdlePeriod(streams []*Stream, err error) ([]*Stream, error) {
	if err != nil {
		return nil, err
	}

	if q.IdlePeriod == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		state, err := stream.LatestState()
		if err != nil {
			return nil, err
		}

		lt := time.Since(state.LastTime)
		should := lt > *q.IdlePeriod

		if (!q.Invert && should) || (q.Invert && !should) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchEmpty(streams []*Stream, err error) ([]*Stream, error) {
	if err != nil {
		return nil, err
	}

	if q.Empty == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		state, err := stream.LatestState()
		if err != nil {
			return nil, err
		}

		if (!q.Invert && state.Msgs == 0) || (q.Invert && state.Msgs > 0) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchConsumerLimit(streams []*Stream, err error) ([]*Stream, error) {
	if err != nil {
		return nil, err
	}
	if q.ConsumersLimit == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		state, err := stream.LatestState()
		if err != nil {
			return nil, err
		}

		if (q.Invert && state.Consumers >= *q.ConsumersLimit) || !q.Invert && state.Consumers <= *q.ConsumersLimit {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchCluster(streams []*Stream, err error) ([]*Stream, error) {
	if err != nil {
		return nil, err
	}
	if q.Cluster == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		nfo, err := stream.LatestInformation()
		if err != nil {
			return nil, err
		}

		should := false
		if nfo.Cluster != nil {
			should = q.Cluster.MatchString(nfo.Cluster.Name)
		}

		// without cluster info its included if inverted
		if (!q.Invert && should) || (q.Invert && !should) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchServer(streams []*Stream, err error) ([]*Stream, error) {
	if err != nil {
		return nil, err
	}
	if q.Server == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		nfo, err := stream.LatestInformation()
		if err != nil {
			return nil, err
		}

		should := false

		if nfo.Cluster != nil {
			for _, r := range nfo.Cluster.Replicas {
				if q.Server.MatchString(r.Name) {
					should = true
					break
				}
			}

			if q.Server.MatchString(nfo.Cluster.Leader) {
				should = true
			}
		}

		// if no cluster info was present we wont include the stream
		// unless invert is set then we can
		if (!q.Invert && should) || (q.Invert && !should) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

const (
	btsep = '.'
	fwc   = '>'
	pwc   = '*'
)

// SubjectIsSubsetMatch tests if a subject matches a standard nats wildcard
func SubjectIsSubsetMatch(subject, test string) bool {
	tsa := [32]string{}
	tts := tokenizeSubjectIntoSlice(tsa[:0], subject)
	return isSubsetMatch(tts, test)
}

// This will test a subject as an array of tokens against a test subject
// Calls into the function isSubsetMatchTokenized
func isSubsetMatch(tokens []string, test string) bool {
	tsa := [32]string{}
	tts := tokenizeSubjectIntoSlice(tsa[:0], test)
	return isSubsetMatchTokenized(tokens, tts)
}

// use similar to append. meaning, the updated slice will be returned
func tokenizeSubjectIntoSlice(tts []string, subject string) []string {
	start := 0
	for i := 0; i < len(subject); i++ {
		if subject[i] == btsep {
			tts = append(tts, subject[start:i])
			start = i + 1
		}
	}
	tts = append(tts, subject[start:])
	return tts
}

// This will test a subject as an array of tokens against a test subject (also encoded as array of tokens)
// and determine if the tokens are matched. Both test subject and tokens
// may contain wildcards. So foo.* is a subset match of [">", "*.*", "foo.*"],
// but not of foo.bar, etc.
func isSubsetMatchTokenized(tokens, test []string) bool {
	// Walk the target tokens
	for i, t2 := range test {
		if i >= len(tokens) {
			return false
		}
		l := len(t2)
		if l == 0 {
			return false
		}
		if t2[0] == fwc && l == 1 {
			return true
		}
		t1 := tokens[i]

		l = len(t1)
		if l == 0 || t1[0] == fwc && l == 1 {
			return false
		}

		if t1[0] == pwc && len(t1) == 1 {
			m := t2[0] == pwc && len(t2) == 1
			if !m {
				return false
			}
			if i >= len(test) {
				return true
			}
			continue
		}
		if t2[0] != pwc && strings.Compare(t1, t2) != 0 {
			return false
		}
	}
	return len(tokens) == len(test)
}
