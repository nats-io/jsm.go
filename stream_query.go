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
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/jsm.go/api"
)

type streamMatcher func([]*Stream) ([]*Stream, error)

type streamQuery struct {
	server         *regexp.Regexp
	cluster        *regexp.Regexp
	consumersLimit *int
	empty          *bool
	idlePeriod     *time.Duration
	createdPeriod  *time.Duration
	invert         bool
	subject        string
	replicas       int
	mirrored       bool
	mirroredIsSet  bool
	sourced        bool
	sourcedIsSet   bool
	expression     string
	leader         string
	matchers       []streamMatcher
	apiLevel       int
}

type StreamQueryOpt func(query *streamQuery) error

// StreamQueryApiLevelMin limits results to assets requiring API Level above or equal to level
func StreamQueryApiLevelMin(level int) StreamQueryOpt {
	return func(q *streamQuery) error {
		q.apiLevel = level
		return nil
	}
}

func StreamQueryIsSourced() StreamQueryOpt {
	return func(q *streamQuery) error {
		q.sourced = true
		q.sourcedIsSet = true
		return nil
	}
}

func StreamQueryIsMirror() StreamQueryOpt {
	return func(q *streamQuery) error {
		q.mirrored = true
		q.mirroredIsSet = true
		return nil
	}
}

// StreamQueryReplicas finds streams with a certain number of replicas or less
func StreamQueryReplicas(r uint) StreamQueryOpt {
	return func(q *streamQuery) error {
		q.replicas = int(r)

		return nil
	}
}

// StreamQuerySubjectWildcard limits results to streams with subject interest matching standard a nats wildcard
func StreamQuerySubjectWildcard(s string) StreamQueryOpt {
	return func(q *streamQuery) error {
		q.subject = s

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
		q.server = re

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
		q.cluster = re

		return nil
	}
}

// StreamQueryFewerConsumersThan limits results to streams with fewer than or equal consumers than c
func StreamQueryFewerConsumersThan(c uint) StreamQueryOpt {
	return func(q *streamQuery) error {
		i := int(c)
		q.consumersLimit = &i
		return nil
	}
}

// StreamQueryWithoutMessages limits results to streams with no messages
func StreamQueryWithoutMessages() StreamQueryOpt {
	return func(q *streamQuery) error {
		t := true
		q.empty = &t
		return nil
	}
}

// StreamQueryIdleLongerThan limits results to streams that has not received messages for a period longer than p
func StreamQueryIdleLongerThan(p time.Duration) StreamQueryOpt {
	return func(q *streamQuery) error {
		q.idlePeriod = &p
		return nil
	}
}

// StreamQueryOlderThan limits the results to streams older than p
func StreamQueryOlderThan(p time.Duration) StreamQueryOpt {
	return func(q *streamQuery) error {
		q.createdPeriod = &p
		return nil
	}
}

// StreamQueryInvert inverts the logic of filters, older than becomes newer than and so forth
func StreamQueryInvert() StreamQueryOpt {
	return func(q *streamQuery) error {
		q.invert = true
		return nil
	}
}

// StreamQueryLeaderServer finds clustered streams where a certain node is the leader
func StreamQueryLeaderServer(server string) StreamQueryOpt {
	return func(q *streamQuery) error {
		q.leader = server
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

	q.matchers = []streamMatcher{
		q.matchExpression,
		q.matchCreatedPeriod,
		q.matchIdlePeriod,
		q.matchEmpty,
		q.matchConsumerLimit,
		q.matchCluster,
		q.matchSubjectWildcard,
		q.matchServer,
		q.matchReplicas,
		q.matchSourced,
		q.matchMirrored,
		q.matchLeaderServer,
		q.matchApiLevel,
	}

	streams, _, _, err := m.Streams(nil)
	if err != nil {
		return nil, err
	}

	return q.Filter(streams)
}

func (q *streamQuery) Filter(streams []*Stream) ([]*Stream, error) {
	matched := streams[:]
	var err error

	for _, matcher := range q.matchers {
		matched, err = matcher(matched)
		if err != nil {
			return nil, err
		}
	}

	return matched, nil
}

func (q *streamQuery) matchLeaderServer(streams []*Stream) ([]*Stream, error) {
	if q.leader == "" {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		nfo, err := stream.LatestInformation()
		if err != nil {
			return nil, err
		}

		if nfo.Cluster == nil {
			continue
		}

		if (!q.invert && nfo.Cluster.Leader == q.leader) || (q.invert && nfo.Cluster.Leader != q.leader) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchMirrored(streams []*Stream) ([]*Stream, error) {
	if !q.mirroredIsSet {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		if (!q.invert && stream.IsMirror()) || (q.invert && !stream.IsMirror()) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchSourced(streams []*Stream) ([]*Stream, error) {
	if !q.sourcedIsSet {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		if (!q.invert && stream.IsSourced()) || (q.invert && !stream.IsSourced()) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchReplicas(streams []*Stream) ([]*Stream, error) {
	if q.replicas == 0 {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		if (q.invert && stream.Replicas() >= q.replicas) || (!q.invert && stream.Replicas() <= q.replicas) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchSubjectWildcard(streams []*Stream) ([]*Stream, error) {
	if q.subject == "" {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		match := false
		for _, subj := range stream.Configuration().Subjects {
			subMatch := SubjectIsSubsetMatch(subj, q.subject)

			if q.invert {
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

func (q *streamQuery) matchCreatedPeriod(streams []*Stream) ([]*Stream, error) {
	if q.createdPeriod == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		nfo, err := stream.LatestInformation()
		if err != nil {
			return nil, err
		}

		if (!q.invert && time.Since(nfo.Created) >= *q.createdPeriod) || (q.invert && time.Since(nfo.Created) <= *q.createdPeriod) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

// note: ideally we match in addition for ones where no consumer had any messages in this period
// but today that means doing a consumer info on every consumer on every stream thats not viable
func (q *streamQuery) matchIdlePeriod(streams []*Stream) ([]*Stream, error) {
	if q.idlePeriod == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		state, err := stream.LatestState()
		if err != nil {
			return nil, err
		}

		lt := time.Since(state.LastTime)
		should := lt > *q.idlePeriod

		if (!q.invert && should) || (q.invert && !should) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchEmpty(streams []*Stream) ([]*Stream, error) {
	if q.empty == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		state, err := stream.LatestState()
		if err != nil {
			return nil, err
		}

		if (!q.invert && state.Msgs == 0) || (q.invert && state.Msgs > 0) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchConsumerLimit(streams []*Stream) ([]*Stream, error) {
	if q.consumersLimit == nil {
		return streams, nil
	}

	var matched []*Stream
	for _, stream := range streams {
		state, err := stream.LatestState()
		if err != nil {
			return nil, err
		}

		if (q.invert && state.Consumers >= *q.consumersLimit) || !q.invert && state.Consumers <= *q.consumersLimit {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchApiLevel(streams []*Stream) ([]*Stream, error) {
	if q.apiLevel <= 0 {
		return streams, nil
	}

	var matched []*Stream

	for _, stream := range streams {
		var v string
		var requiredLevel int

		meta := stream.Configuration().Metadata
		if len(meta) > 0 {
			v = meta[api.JsMetaRequiredServerLevel]
			if v != "" {
				requiredLevel, _ = strconv.Atoi(v)
			}
		}

		if (!q.invert && requiredLevel >= q.apiLevel) || (q.invert && requiredLevel < q.apiLevel) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchCluster(streams []*Stream) ([]*Stream, error) {
	if q.cluster == nil {
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
			should = q.cluster.MatchString(nfo.Cluster.Name)
		}

		// without cluster info its included if inverted
		if (!q.invert && should) || (q.invert && !should) {
			matched = append(matched, stream)
		}
	}

	return matched, nil
}

func (q *streamQuery) matchServer(streams []*Stream) ([]*Stream, error) {
	if q.server == nil {
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
				if q.server.MatchString(r.Name) {
					should = true
					break
				}
			}

			if q.server.MatchString(nfo.Cluster.Leader) {
				should = true
			}
		}

		// if no cluster info was present we wont include the stream
		// unless invert is set then we can
		if (!q.invert && should) || (q.invert && !should) {
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
