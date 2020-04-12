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

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

var timeout = 5 * time.Second
var nc *nats.Conn
var mu sync.Mutex

// Connect connects to NATS and configures it to use the connection in future interactions with JetStream
// Deprecated: Use Request Options to supply the connection
func Connect(servers string, opts ...nats.Option) (err error) {
	mu.Lock()
	defer mu.Unlock()

	// needed so that interest drops are observed by JetStream to stop
	opts = append(opts, nats.UseOldRequestStyle())

	nc, err = nats.Connect(servers, opts...)

	return err
}

// SetTimeout sets the timeout for requests to JetStream
// Deprecated: Use Request Options to supply the timeout
func SetTimeout(t time.Duration) {
	mu.Lock()
	defer mu.Unlock()

	timeout = t
}

// SetConnection sets the connection used to perform requests. Will force using old style requests.
// Deprecated: Use Request Options to supply the connection
func SetConnection(c *nats.Conn) {
	mu.Lock()
	defer mu.Unlock()

	c.Opts.UseOldRequestStyle = true

	nc = c
}

// IsJetStreamEnabled determines if JetStream is enabled for the current account
func IsJetStreamEnabled(opts ...RequestOption) bool {
	ropts, err := newreqoptions(opts...)
	if err != nil {
		return false
	}

	_, err = request(api.JetStreamEnabled, nil, ropts)
	return err == nil
}

// IsErrorResponse checks if the message holds a standard JetStream error
func IsErrorResponse(m *nats.Msg) bool {
	return strings.HasPrefix(string(m.Data), api.ErrPrefix)
}

// ParseErrorResponse parses the JetStream response, if it's an error returns an error instance holding the message else nil
func ParseErrorResponse(m *nats.Msg) error {
	if !IsErrorResponse(m) {
		return nil
	}

	return fmt.Errorf(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(string(m.Data), api.ErrPrefix), " '"), "'"))
}

// IsOKResponse checks if the message holds a standard JetStream error
func IsOKResponse(m *nats.Msg) bool {
	return strings.HasPrefix(string(m.Data), api.OK)
}

// IsKnownStream determines if a Stream is known
func IsKnownStream(stream string, opts ...RequestOption) (bool, error) {
	streams, err := StreamNames(opts...)
	if err != nil {
		return false, err
	}

	for _, s := range streams {
		if s == stream {
			return true, nil
		}
	}

	return false, nil
}

// IsKnownStreamTemplate determines if a StreamTemplate is known
func IsKnownStreamTemplate(template string, opts ...RequestOption) (bool, error) {
	templates, err := StreamTemplateNames(opts...)
	if err != nil {
		return false, err
	}

	for _, s := range templates {
		if s == template {
			return true, nil
		}
	}

	return false, nil
}

// IsKnownConsumer determines if a Consumer is known for a specific Stream
func IsKnownConsumer(stream string, consumer string, opts ...RequestOption) (bool, error) {
	consumers, err := ConsumerNames(stream, opts...)
	if err != nil {
		return false, err
	}

	for _, c := range consumers {
		if c == consumer {
			return true, nil
		}
	}

	return false, nil
}

// JetStreamAccountInfo retrieves information about the current account limits and more
func JetStreamAccountInfo(opts ...RequestOption) (info api.JetStreamAccountStats, err error) {
	conn, err := newreqoptions(opts...)
	if err != nil {
		return api.JetStreamAccountStats{}, err
	}

	response, err := request(api.JetStreamInfo, nil, conn)
	if err != nil {
		return info, err
	}

	err = json.Unmarshal(response.Data, &info)
	if err != nil {
		return info, err
	}

	return info, nil
}

// StreamNames is a sorted list of all known Streams
func StreamNames(opts ...RequestOption) (streams []string, err error) {
	streams = []string{}

	conn, err := newreqoptions(opts...)
	if err != nil {
		return nil, err
	}

	response, err := request(api.JetStreamListStreams, nil, conn)
	if err != nil {
		return streams, err
	}

	err = json.Unmarshal(response.Data, &streams)
	if err != nil {
		return streams, err
	}

	sort.Strings(streams)

	return streams, nil
}

// StreamTemplateNames is a sorted list of all known StreamTemplates
func StreamTemplateNames(opts ...RequestOption) (templates []string, err error) {
	templates = []string{}

	conn, err := newreqoptions(opts...)
	if err != nil {
		return nil, err
	}

	response, err := request(api.JetStreamListTemplates, nil, conn)
	if err != nil {
		return templates, err
	}

	err = json.Unmarshal(response.Data, &templates)
	if err != nil {
		return templates, err
	}

	sort.Strings(templates)

	return templates, nil
}

// ConsumerNames is a sorted list of all known Consumers within a Stream
func ConsumerNames(stream string, opts ...RequestOption) (consumers []string, err error) {
	consumers = []string{}

	conn, err := newreqoptions(opts...)
	if err != nil {
		return nil, err
	}

	response, err := request(fmt.Sprintf(api.JetStreamConsumersT, stream), nil, conn)
	if err != nil {
		return consumers, err
	}

	err = json.Unmarshal(response.Data, &consumers)
	if err != nil {
		return consumers, err
	}

	sort.Strings(consumers)

	return consumers, nil
}

// EachStream iterates over all known Streams
func EachStream(cb func(*Stream), opts ...RequestOption) (err error) {
	names, err := StreamNames()
	if err != nil {
		return err
	}

	for _, s := range names {
		stream, err := LoadStream(s, opts...)
		if err != nil {
			return err
		}

		cb(stream)
	}

	return nil
}

// EachStreamTemplate iterates over all known Stream Templates
func EachStreamTemplate(cb func(*StreamTemplate), opts ...RequestOption) (err error) {
	names, err := StreamTemplateNames(opts...)
	if err != nil {
		return err
	}

	for _, t := range names {
		template, err := LoadStreamTemplate(t, opts...)
		if err != nil {
			return err
		}

		cb(template)
	}

	return nil
}

// Flush flushes the underlying NATS connection
// Deprecated: Use Request Options to supply the connection
func Flush() error {
	nc := Connection()

	if nc == nil {
		return fmt.Errorf("nats connection is not set, use SetConnection()")
	}

	return nc.Flush()
}

// Connection is the active NATS connection being used
// Deprecated: Use Request Options to supply the connection
func Connection() *nats.Conn {
	mu.Lock()
	defer mu.Unlock()

	return nc
}

func request(subj string, data []byte, opts *reqoptions) (res *nats.Msg, err error) {
	if opts == nil || opts.nc == nil {
		return nil, fmt.Errorf("nats connection is not set")
	}

	var ctx context.Context
	var cancel func()

	if opts.ctx == nil {
		ctx, cancel = context.WithTimeout(context.Background(), opts.timeout)
		defer cancel()
	} else {
		ctx = opts.ctx
	}

	res, err = opts.nc.RequestWithContext(ctx, subj, data)
	if err != nil {
		return nil, err
	}

	return res, ParseErrorResponse(res)
}
