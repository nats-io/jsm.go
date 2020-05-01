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

package jsm

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/jsm.go/api"
)

// DefaultStream is a template configuration with StreamPolicy retention and 1 years maximum age. No storage type or subjects are set
var DefaultStream = api.StreamConfig{
	Retention:    api.LimitsPolicy,
	Discard:      api.DiscardOld,
	MaxConsumers: -1,
	MaxMsgs:      -1,
	MaxBytes:     -1,
	MaxAge:       24 * 365 * time.Hour,
	MaxMsgSize:   -1,
	Replicas:     1,
	NoAck:        false,
}

// DefaultWorkQueue is a template configuration with WorkQueuePolicy retention and 1 years maximum age. No storage type or subjects are set
var DefaultWorkQueue = api.StreamConfig{
	Retention:    api.WorkQueuePolicy,
	Discard:      api.DiscardOld,
	MaxConsumers: -1,
	MaxMsgs:      -1,
	MaxBytes:     -1,
	MaxAge:       24 * 365 * time.Hour,
	MaxMsgSize:   -1,
	Replicas:     1,
	NoAck:        false,
}

// DefaultStreamConfiguration is the configuration that will be used to create new Streams in NewStream
var DefaultStreamConfiguration = DefaultStream

// StreamOption configures a stream
type StreamOption func(o *StreamConfig) error

// Stream represents a JetStream Stream
type Stream struct {
	cfg *StreamConfig
}

type StreamConfig struct {
	api.StreamConfig

	conn  *reqoptions
	ropts []RequestOption
}

// NewStreamFromDefault creates a new stream based on a supplied template and options
func NewStreamFromDefault(name string, dflt api.StreamConfig, opts ...StreamOption) (stream *Stream, err error) {
	cfg, err := NewStreamConfiguration(dflt, opts...)
	if err != nil {
		return nil, err
	}

	cfg.Name = name

	valid, errs := cfg.Validate()
	if !valid {
		return nil, fmt.Errorf("configuration validation failed: %s", strings.Join(errs, ", "))
	}

	var resp api.JetStreamCreateStreamResponse
	err = jsonRequest(fmt.Sprintf(api.JetStreamCreateStreamT, name), &cfg, &resp, cfg.conn)
	if err != nil {
		return nil, err
	}

	// TODO: use new response data to speed this up using resp.StreamInfo
	return LoadStream(name, cfg.ropts...)
}

// LoadOrNewStreamFromDefault loads an existing stream or creates a new one matching opts and template
func LoadOrNewStreamFromDefault(name string, dflt api.StreamConfig, opts ...StreamOption) (stream *Stream, err error) {
	cfg, err := NewStreamConfiguration(DefaultStream, opts...)
	if err != nil {
		return nil, err
	}

	s, err := LoadStream(name, cfg.ropts...)
	if s == nil || err != nil {
		return NewStreamFromDefault(name, dflt, opts...)
	}

	return s, err
}

// NewStream creates a new stream using DefaultStream as a starting template allowing adjustments to be made using options
func NewStream(name string, opts ...StreamOption) (stream *Stream, err error) {
	return NewStreamFromDefault(name, DefaultStream, opts...)
}

// LoadOrNewStreamFromDefault loads an existing stream or creates a new one matching opts
func LoadOrNewStream(name string, opts ...StreamOption) (stream *Stream, err error) {
	return LoadOrNewStreamFromDefault(name, DefaultStream, opts...)
}

// LoadStream loads a stream by name
func LoadStream(name string, opts ...RequestOption) (stream *Stream, err error) {
	conn, err := newreqoptions(opts...)
	if err != nil {
		return nil, err
	}

	stream = &Stream{cfg: &StreamConfig{
		StreamConfig: api.StreamConfig{Name: name},
		conn:         conn,
		ropts:        opts,
	}}

	err = loadConfigForStream(stream)
	if err != nil {
		return nil, err
	}

	return stream, nil
}

// NewStreamConfiguration generates a new configuration based on template modified by opts
func NewStreamConfiguration(template api.StreamConfig, opts ...StreamOption) (*StreamConfig, error) {
	cfg := &StreamConfig{
		StreamConfig: template,
		conn:         dfltreqoptions(),
	}

	for _, o := range opts {
		err := o(cfg)
		if err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

func loadConfigForStream(stream *Stream) (err error) {
	info, err := loadStreamInfo(stream.cfg.Name, stream.cfg.conn)
	if err != nil {
		return err
	}

	stream.cfg.StreamConfig = info.Config

	return nil
}

func loadStreamInfo(stream string, conn *reqoptions) (info *api.StreamInfo, err error) {
	var resp api.JetStreamStreamInfoResponse
	err = jsonRequest(fmt.Sprintf(api.JetStreamStreamInfoT, stream), nil, &resp, conn)
	if err != nil {
		return nil, err
	}

	return resp.StreamInfo, nil
}

func Subjects(s ...string) StreamOption {
	return func(o *StreamConfig) error {
		o.StreamConfig.Subjects = s
		return nil
	}
}

func LimitsRetention() StreamOption {
	return func(o *StreamConfig) error {
		o.StreamConfig.Retention = api.LimitsPolicy
		return nil
	}
}

func InterestRetention() StreamOption {
	return func(o *StreamConfig) error {
		o.StreamConfig.Retention = api.InterestPolicy
		return nil
	}
}

func WorkQueueRetention() StreamOption {
	return func(o *StreamConfig) error {
		o.StreamConfig.Retention = api.WorkQueuePolicy
		return nil
	}
}

func MaxConsumers(m int) StreamOption {
	return func(o *StreamConfig) error {
		o.StreamConfig.MaxConsumers = m
		return nil
	}
}

func MaxMessages(m int64) StreamOption {
	return func(o *StreamConfig) error {
		o.StreamConfig.MaxMsgs = m
		return nil
	}
}

func MaxBytes(m int64) StreamOption {
	return func(o *StreamConfig) error {
		o.StreamConfig.MaxBytes = m
		return nil
	}
}

func MaxAge(m time.Duration) StreamOption {
	return func(o *StreamConfig) error {
		o.StreamConfig.MaxAge = m
		return nil
	}
}

func MaxMessageSize(m int32) StreamOption {
	return func(o *StreamConfig) error {
		o.StreamConfig.MaxMsgSize = m
		return nil
	}
}

func FileStorage() StreamOption {
	return func(o *StreamConfig) error {
		o.StreamConfig.Storage = api.FileStorage
		return nil
	}
}

func MemoryStorage() StreamOption {
	return func(o *StreamConfig) error {
		o.StreamConfig.Storage = api.MemoryStorage
		return nil
	}
}

func Replicas(r int) StreamOption {
	return func(o *StreamConfig) error {
		o.StreamConfig.Replicas = r
		return nil
	}
}

func NoAck() StreamOption {
	return func(o *StreamConfig) error {
		o.StreamConfig.NoAck = true
		return nil
	}
}

func DiscardNew() StreamOption {
	return func(o *StreamConfig) error {
		o.Discard = api.DiscardNew
		return nil
	}
}

func DiscardOld() StreamOption {
	return func(o *StreamConfig) error {
		o.Discard = api.DiscardOld
		return nil
	}
}

func StreamConnection(opts ...RequestOption) StreamOption {
	return func(o *StreamConfig) error {
		for _, opt := range opts {
			opt(o.conn)
		}

		o.ropts = append(o.ropts, opts...)

		return nil
	}
}

// UpdateConfiguration updates the stream using cfg modified by opts, reloads configuration from the server post update
func (s *Stream) UpdateConfiguration(cfg api.StreamConfig, opts ...StreamOption) error {
	ncfg, err := NewStreamConfiguration(cfg, opts...)
	if err != nil {
		return err
	}

	var resp api.JetStreamUpdateStreamResponse
	err = jsonRequest(fmt.Sprintf(api.JetStreamUpdateStreamT, s.Name()), ncfg, &resp, s.cfg.conn)
	if err != nil {
		return err
	}

	return s.Reset()
}

// Reset reloads the Stream configuration from the JetStream server
func (s *Stream) Reset() error {
	return loadConfigForStream(s)
}

// LoadConsumer loads a named consumer related to this Stream
func (s *Stream) LoadConsumer(name string) (*Consumer, error) {
	return LoadConsumer(s.cfg.Name, name, s.cfg.ropts...)
}

// pass our connection info into the descendant consumers but allows opts to override it
func (s *Stream) consumerOpts(opts ...ConsumerOption) []ConsumerOption {
	return append([]ConsumerOption{ConsumerConnection(s.cfg.ropts...)}, opts...)
}

// NewConsumer creates a new consumer in this Stream based on DefaultConsumer
func (s *Stream) NewConsumer(opts ...ConsumerOption) (consumer *Consumer, err error) {
	return NewConsumer(s.Name(), s.consumerOpts(opts...)...)
}

// LoadOrNewConsumer loads or creates a consumer based on these options
func (s *Stream) LoadOrNewConsumer(name string, opts ...ConsumerOption) (consumer *Consumer, err error) {
	return LoadOrNewConsumer(s.Name(), name, s.consumerOpts(opts...)...)
}

// NewConsumerFromDefault creates a new consumer in this Stream based on a supplied template config
func (s *Stream) NewConsumerFromDefault(dflt api.ConsumerConfig, opts ...ConsumerOption) (consumer *Consumer, err error) {
	return NewConsumerFromDefault(s.Name(), dflt, s.consumerOpts(opts...)...)
}

// LoadOrNewConsumer loads or creates a consumer based on these options that adjust supplied template
func (s *Stream) LoadOrNewConsumerFromDefault(name string, deflt api.ConsumerConfig, opts ...ConsumerOption) (consumer *Consumer, err error) {
	return LoadOrNewConsumerFromDefault(s.Name(), name, deflt, s.consumerOpts(opts...)...)
}

// ConsumerNames is a list of all known consumers for this Stream
func (s *Stream) ConsumerNames() (names []string, err error) {
	var resp api.JetStreamConsumersResponse

	err = jsonRequest(fmt.Sprintf(api.JetStreamConsumersT, s.Name()), nil, &resp, s.cfg.conn)
	if err != nil {
		return names, err
	}

	sort.Strings(resp.Consumers)

	return resp.Consumers, nil
}

// EachConsumer calls cb with each known consumer for this stream, error on any error to load consumers
func (s *Stream) EachConsumer(cb func(consumer *Consumer)) error {
	names, err := s.ConsumerNames()
	if err != nil {
		return err
	}

	for _, name := range names {
		c, err := s.LoadConsumer(name)
		if err != nil {
			return err
		}

		cb(c)
	}

	return nil
}

func (s *Stream) Information() (info *api.StreamInfo, err error) {
	return loadStreamInfo(s.Name(), s.cfg.conn)
}

// State retrieves the Stream State
func (s *Stream) State() (stats api.StreamState, err error) {
	info, err := loadStreamInfo(s.Name(), s.cfg.conn)
	if err != nil {
		return stats, err
	}

	return info.State, nil
}

// Delete deletes the Stream, after this the Stream object should be disposed
func (s *Stream) Delete() error {
	var resp api.JetStreamDeleteStreamResponse
	err := jsonRequest(fmt.Sprintf(api.JetStreamDeleteStreamT, s.Name()), nil, &resp, s.cfg.conn)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("unknown failure")
	}

	return nil
}

// Purge deletes all messages from the Stream
func (s *Stream) Purge() error {
	var resp api.JetStreamPurgeStreamResponse
	err := jsonRequest(fmt.Sprintf(api.JetStreamPurgeStreamT, s.Name()), nil, &resp, s.cfg.conn)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("unknown failure")
	}

	return nil
}

// LoadMessage loads a message from the message set by its sequence number
func (s *Stream) LoadMessage(seq int) (msg api.StoredMsg, err error) {
	response, err := request(fmt.Sprintf(api.JetStreamMsgBySeqT, s.Name()), []byte(strconv.Itoa(seq)), s.cfg.conn)
	if err != nil {
		return api.StoredMsg{}, err
	}

	if IsErrorResponse(response) {
		return api.StoredMsg{}, fmt.Errorf("could not load message: %s", ParseErrorResponse(response))
	}

	msg = api.StoredMsg{}
	err = json.Unmarshal(response.Data, &msg)
	if err != nil {
		return api.StoredMsg{}, err
	}

	return msg, nil
}

// DeleteMessage deletes a specific message from the Stream by overwriting it with random data
func (s *Stream) DeleteMessage(seq int) (err error) {
	var resp api.JetStreamDeleteMsgResponse
	err = jsonRequest(fmt.Sprintf(api.JetStreamDeleteMsgT, s.Name()), api.JetStreamDeleteMsgRequest{Seq: uint64(seq)}, &resp, s.cfg.conn)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("unknown error while deleting message %d", seq)
	}

	return nil
}

// AdvisorySubject is a wildcard subscription subject that subscribes to all advisories for this stream
func (s *Stream) AdvisorySubject() string {
	return api.JetStreamAdvisoryPrefix + "." + "*" + "." + s.Name() + ".*"
}

// MetricSubject is a wildcard subscription subject that subscribes to all advisories for this stream
func (s *Stream) MetricSubject() string {
	return api.JetStreamMetricPrefix + "." + "*" + "." + s.Name() + ".*"
}

// IsTemplateManaged determines if this stream is managed by a template
func (s *Stream) IsTemplateManaged() bool { return s.Template() != "" }

func (s *Stream) Configuration() api.StreamConfig { return s.cfg.StreamConfig }
func (s *Stream) Name() string                    { return s.cfg.Name }
func (s *Stream) Subjects() []string              { return s.cfg.Subjects }
func (s *Stream) Retention() api.RetentionPolicy  { return s.cfg.Retention }
func (s *Stream) MaxConsumers() int               { return s.cfg.MaxConsumers }
func (s *Stream) MaxMsgs() int64                  { return s.cfg.MaxMsgs }
func (s *Stream) MaxBytes() int64                 { return s.cfg.MaxBytes }
func (s *Stream) MaxAge() time.Duration           { return s.cfg.MaxAge }
func (s *Stream) MaxMsgSize() int32               { return s.cfg.MaxMsgSize }
func (s *Stream) Storage() api.StorageType        { return s.cfg.Storage }
func (s *Stream) Replicas() int                   { return s.cfg.Replicas }
func (s *Stream) NoAck() bool                     { return s.cfg.NoAck }
func (s *Stream) Template() string                { return s.cfg.Template }
