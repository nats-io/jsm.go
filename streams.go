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
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
)

// DefaultStream is a template configuration with StreamPolicy retention and 1 years maximum age. No storage type or subjects are set
var DefaultStream = api.StreamConfig{
	Retention:    api.LimitsPolicy,
	Discard:      api.DiscardOld,
	MaxConsumers: -1,
	MaxMsgs:      -1,
	MaxMsgsPer:   -1,
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
	MaxMsgsPer:   -1,
	MaxBytes:     -1,
	MaxAge:       24 * 365 * time.Hour,
	MaxMsgSize:   -1,
	Replicas:     api.StreamDefaultReplicas,
	NoAck:        false,
}

// DefaultStreamConfiguration is the configuration that will be used to create new Streams in NewStream
var DefaultStreamConfiguration = DefaultStream

// StreamOption configures a stream
type StreamOption func(o *api.StreamConfig) error

// Stream represents a JetStream Stream
type Stream struct {
	cfg      *api.StreamConfig
	lastInfo *api.StreamInfo
	mgr      *Manager

	sync.Mutex
}

var ErrAckStreamIngestsAll = fmt.Errorf("configuration validation failed: streams with no_ack false may not have '>' or '*' as subjects")

// NewStreamFromDefault creates a new stream based on a supplied template and options
func (m *Manager) NewStreamFromDefault(name string, dflt api.StreamConfig, opts ...StreamOption) (stream *Stream, err error) {
	if !IsValidName(name) {
		return nil, fmt.Errorf("%q is not a valid stream name", name)
	}

	cfg, err := NewStreamConfiguration(dflt, opts...)
	if err != nil {
		return nil, err
	}

	cfg.Name = name

	if !cfg.NoAck && (stringsContains(cfg.Subjects, ">") || stringsContains(cfg.Subjects, "*")) {
		return nil, ErrAckStreamIngestsAll
	}

	valid, errs := cfg.Validate(m.validator)
	if !valid {
		return nil, fmt.Errorf("configuration validation failed: %s", strings.Join(errs, ", "))
	}

	var resp api.JSApiStreamCreateResponse
	err = m.jsonRequest(fmt.Sprintf(api.JSApiStreamCreateT, name), &cfg, &resp)
	if err != nil {
		return nil, err
	}

	return m.streamFromConfig(&resp.Config, resp.StreamInfo), nil
}

func (m *Manager) streamFromConfig(cfg *api.StreamConfig, info *api.StreamInfo) (stream *Stream) {
	s := &Stream{cfg: cfg, mgr: m}
	if info != nil {
		s.lastInfo = info
	}

	return s
}

// LoadOrNewStreamFromDefault loads an existing stream or creates a new one matching opts and template
func (m *Manager) LoadOrNewStreamFromDefault(name string, dflt api.StreamConfig, opts ...StreamOption) (stream *Stream, err error) {
	if !IsValidName(name) {
		return nil, fmt.Errorf("%q is not a valid stream name", name)
	}

	for _, o := range opts {
		o(&dflt)
	}
	s, err := m.LoadStream(name)
	if IsNatsError(err, 10059) {
		return m.NewStreamFromDefault(name, dflt)
	}

	return s, err
}

// NewStream creates a new stream using DefaultStream as a starting template allowing adjustments to be made using options
func (m *Manager) NewStream(name string, opts ...StreamOption) (stream *Stream, err error) {
	return m.NewStreamFromDefault(name, DefaultStream, opts...)
}

// LoadOrNewStream loads an existing stream or creates a new one matching opts
func (m *Manager) LoadOrNewStream(name string, opts ...StreamOption) (stream *Stream, err error) {
	return m.LoadOrNewStreamFromDefault(name, DefaultStream, opts...)
}

// LoadStream loads a stream by name
func (m *Manager) LoadStream(name string) (stream *Stream, err error) {
	if !IsValidName(name) {
		return nil, fmt.Errorf("%q is not a valid stream name", name)
	}

	stream = &Stream{
		mgr: m,
		cfg: &api.StreamConfig{Name: name},
	}

	err = m.loadConfigForStream(stream)
	if err != nil {
		return nil, err
	}

	return stream, nil
}

// NewStreamConfiguration generates a new configuration based on template modified by opts
func (m *Manager) NewStreamConfiguration(template api.StreamConfig, opts ...StreamOption) (*api.StreamConfig, error) {
	return NewStreamConfiguration(template, opts...)
}

// NewStreamConfiguration generates a new configuration based on template modified by opts
func NewStreamConfiguration(template api.StreamConfig, opts ...StreamOption) (*api.StreamConfig, error) {
	cfg := &template

	for _, o := range opts {
		err := o(cfg)
		if err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

func (m *Manager) loadConfigForStream(stream *Stream) (err error) {
	info, err := m.loadStreamInfo(stream.cfg.Name, nil)
	if err != nil {
		return err
	}

	stream.Lock()
	stream.cfg = &info.Config
	stream.lastInfo = info
	stream.Unlock()

	return nil
}

func (m *Manager) loadStreamInfo(stream string, req *api.JSApiStreamInfoRequest) (info *api.StreamInfo, err error) {
	var resp api.JSApiStreamInfoResponse
	err = m.jsonRequest(fmt.Sprintf(api.JSApiStreamInfoT, stream), req, &resp)
	if err != nil {
		return nil, err
	}

	return resp.StreamInfo, nil
}

func ConsumerLimits(limits api.StreamConsumerLimits) StreamOption {
	return func(o *api.StreamConfig) error {
		o.ConsumerLimits = limits
		return nil
	}
}

func Subjects(s ...string) StreamOption {
	return func(o *api.StreamConfig) error {
		o.Subjects = s
		return nil
	}
}

// StreamDescription is a textual description of this stream to provide additional context
func StreamDescription(d string) StreamOption {
	return func(o *api.StreamConfig) error {
		o.Description = d
		return nil
	}
}

func LimitsRetention() StreamOption {
	return func(o *api.StreamConfig) error {
		o.Retention = api.LimitsPolicy
		return nil
	}
}

func InterestRetention() StreamOption {
	return func(o *api.StreamConfig) error {
		o.Retention = api.InterestPolicy
		return nil
	}
}

func WorkQueueRetention() StreamOption {
	return func(o *api.StreamConfig) error {
		o.Retention = api.WorkQueuePolicy
		return nil
	}
}

func MaxConsumers(m int) StreamOption {
	return func(o *api.StreamConfig) error {
		o.MaxConsumers = m
		return nil
	}
}

func MaxMessages(m int64) StreamOption {
	return func(o *api.StreamConfig) error {
		o.MaxMsgs = m
		return nil
	}
}

func MaxMessagesPerSubject(m int64) StreamOption {
	return func(o *api.StreamConfig) error {
		o.MaxMsgsPer = m
		return nil
	}
}

func MaxBytes(m int64) StreamOption {
	return func(o *api.StreamConfig) error {
		o.MaxBytes = m
		return nil
	}
}

func MaxAge(m time.Duration) StreamOption {
	return func(o *api.StreamConfig) error {
		o.MaxAge = m
		return nil
	}
}

func MaxMessageSize(m int32) StreamOption {
	return func(o *api.StreamConfig) error {
		o.MaxMsgSize = m
		return nil
	}
}

func FileStorage() StreamOption {
	return func(o *api.StreamConfig) error {
		o.Storage = api.FileStorage
		return nil
	}
}

func MemoryStorage() StreamOption {
	return func(o *api.StreamConfig) error {
		o.Storage = api.MemoryStorage
		return nil
	}
}

func Replicas(r int) StreamOption {
	return func(o *api.StreamConfig) error {
		o.Replicas = r
		return nil
	}
}

func NoAck() StreamOption {
	return func(o *api.StreamConfig) error {
		o.NoAck = true
		return nil
	}
}

func DiscardNew() StreamOption {
	return func(o *api.StreamConfig) error {
		o.Discard = api.DiscardNew
		return nil
	}
}

func DiscardNewPerSubject() StreamOption {
	return func(o *api.StreamConfig) error {
		o.Discard = api.DiscardNew
		o.DiscardNewPer = true
		return nil
	}
}

func DiscardOld() StreamOption {
	return func(o *api.StreamConfig) error {
		o.Discard = api.DiscardOld
		return nil
	}
}

func DuplicateWindow(d time.Duration) StreamOption {
	return func(o *api.StreamConfig) error {
		o.Duplicates = d
		return nil
	}
}

func PlacementCluster(cluster string) StreamOption {
	return func(o *api.StreamConfig) error {
		if o.Placement == nil {
			o.Placement = &api.Placement{}
		}

		o.Placement.Cluster = cluster

		return nil
	}
}

func PlacementTags(tags ...string) StreamOption {
	return func(o *api.StreamConfig) error {
		if o.Placement == nil {
			o.Placement = &api.Placement{}
		}

		o.Placement.Tags = tags

		return nil
	}
}

func Mirror(stream *api.StreamSource) StreamOption {
	return func(o *api.StreamConfig) error {
		o.Mirror = stream

		return nil
	}
}

func AppendSource(source *api.StreamSource) StreamOption {
	return func(o *api.StreamConfig) error {
		o.Sources = append(o.Sources, source)

		return nil
	}
}

func Sources(streams ...*api.StreamSource) StreamOption {
	return func(o *api.StreamConfig) error {
		o.Sources = streams

		return nil
	}
}

func DenyDelete() StreamOption {
	return func(o *api.StreamConfig) error {
		o.DenyDelete = true

		return nil
	}
}

func DenyPurge() StreamOption {
	return func(o *api.StreamConfig) error {
		o.DenyPurge = true

		return nil
	}
}

func AllowRollup() StreamOption {
	return func(o *api.StreamConfig) error {
		o.RollupAllowed = true
		return nil
	}
}

func AllowDirect() StreamOption {
	return func(o *api.StreamConfig) error {
		o.AllowDirect = true
		return nil
	}
}

func NoAllowDirect() StreamOption {
	return func(o *api.StreamConfig) error {
		o.AllowDirect = false
		return nil
	}
}

func MirrorDirect() StreamOption {
	return func(o *api.StreamConfig) error {
		o.MirrorDirect = true
		return nil
	}
}

func NoMirrorDirect() StreamOption {
	return func(o *api.StreamConfig) error {
		o.MirrorDirect = false
		return nil
	}
}

func Republish(m *api.RePublish) StreamOption {
	return func(o *api.StreamConfig) error {
		o.RePublish = m
		return nil
	}
}

func StreamMetadata(meta map[string]string) StreamOption {
	return func(o *api.StreamConfig) error {
		for k := range meta {
			if len(k) == 0 {
				return fmt.Errorf("invalid empty string key in metadata")
			}
		}

		o.Metadata = meta
		return nil
	}
}

func Compression(alg api.Compression) StreamOption {
	return func(o *api.StreamConfig) error {
		o.Compression = alg
		return nil
	}
}

func FirstSequence(seq uint64) StreamOption {
	return func(o *api.StreamConfig) error {
		o.FirstSeq = seq
		return nil
	}
}

func SubjectTransform(subjectTransform *api.SubjectTransformConfig) StreamOption {
	return func(o *api.StreamConfig) error {
		o.SubjectTransform = subjectTransform
		return nil
	}
}

// PageContents creates a StreamPager used to traverse the contents of the stream,
// Close() should be called to dispose of the background consumer and resources
func (s *Stream) PageContents(opts ...PagerOption) (*StreamPager, error) {
	if s.Retention() == api.WorkQueuePolicy && !s.DirectAllowed() {
		return nil, fmt.Errorf("work queue retention streams can only be paged if direct access is allowed")
	}

	pgr := &StreamPager{}
	err := pgr.start(s, s.mgr, opts...)
	if err != nil {
		return nil, err
	}

	return pgr, err
}

// UpdateConfiguration updates the stream using cfg modified by opts, reloads configuration from the server post update
func (s *Stream) UpdateConfiguration(cfg api.StreamConfig, opts ...StreamOption) error {
	ncfg, err := NewStreamConfiguration(cfg, opts...)
	if err != nil {
		return err
	}

	var resp api.JSApiStreamUpdateResponse
	err = s.mgr.jsonRequest(fmt.Sprintf(api.JSApiStreamUpdateT, s.Name()), ncfg, &resp)
	if err != nil {
		return err
	}

	return s.Reset()
}

// Reset reloads the Stream configuration from the JetStream server
func (s *Stream) Reset() error {
	return s.mgr.loadConfigForStream(s)
}

// LoadConsumer loads a named consumer related to this Stream
func (s *Stream) LoadConsumer(name string) (*Consumer, error) {
	return s.mgr.LoadConsumer(s.cfg.Name, name)
}

// NewConsumer creates a new consumer in this Stream based on DefaultConsumer
func (s *Stream) NewConsumer(opts ...ConsumerOption) (consumer *Consumer, err error) {
	return s.mgr.NewConsumer(s.Name(), opts...)
}

// LoadOrNewConsumer loads or creates a consumer based on these options
func (s *Stream) LoadOrNewConsumer(name string, opts ...ConsumerOption) (consumer *Consumer, err error) {
	return s.mgr.LoadOrNewConsumer(s.Name(), name, opts...)
}

// NewConsumerFromDefault creates a new consumer in this Stream based on a supplied template config
func (s *Stream) NewConsumerFromDefault(dflt api.ConsumerConfig, opts ...ConsumerOption) (consumer *Consumer, err error) {
	return s.mgr.NewConsumerFromDefault(s.Name(), dflt, opts...)
}

// LoadOrNewConsumerFromDefault loads or creates a consumer based on these options that adjust supplied template
func (s *Stream) LoadOrNewConsumerFromDefault(name string, deflt api.ConsumerConfig, opts ...ConsumerOption) (consumer *Consumer, err error) {
	return s.mgr.LoadOrNewConsumerFromDefault(s.Name(), name, deflt, opts...)
}

// ConsumerNames is a list of all known consumers for this Stream
func (s *Stream) ConsumerNames() (names []string, err error) {
	return s.mgr.ConsumerNames(s.Name())
}

// EachConsumer calls cb with each known consumer for this stream, error on any error to load consumers
func (s *Stream) EachConsumer(cb func(consumer *Consumer)) (missing []string, err error) {
	consumers, missing, err := s.mgr.Consumers(s.Name())
	if err != nil {
		return nil, err
	}

	for _, c := range consumers {
		cb(c)
	}

	return missing, nil
}

// LatestInformation returns the most recently fetched stream information
func (s *Stream) LatestInformation() (info *api.StreamInfo, err error) {
	s.Lock()
	nfo := s.lastInfo
	s.Unlock()

	if nfo != nil {
		return nfo, nil
	}

	return s.Information()
}

// Information loads the current stream information
func (s *Stream) Information(req ...api.JSApiStreamInfoRequest) (info *api.StreamInfo, err error) {
	if len(req) > 1 {
		return nil, fmt.Errorf("only one request info is accepted")
	}

	var ireq api.JSApiStreamInfoRequest
	if len(req) == 1 {
		ireq = req[0]
	}

	info, err = s.mgr.loadStreamInfo(s.Name(), &ireq)
	if err != nil {
		return nil, err
	}

	s.Lock()
	s.lastInfo = info
	s.Unlock()

	return info, nil
}

// LatestState returns the most recently fetched stream state
func (s *Stream) LatestState() (state api.StreamState, err error) {
	nfo, err := s.LatestInformation()
	if err != nil {
		return api.StreamState{}, err
	}

	return nfo.State, nil
}

// State retrieves the Stream State
func (s *Stream) State(req ...api.JSApiStreamInfoRequest) (stats api.StreamState, err error) {
	info, err := s.Information(req...)
	if err != nil {
		return api.StreamState{}, err
	}

	return info.State, nil
}

// Delete deletes the Stream, after this the Stream object should be disposed
func (s *Stream) Delete() error {
	var resp api.JSApiStreamDeleteResponse
	err := s.mgr.jsonRequest(fmt.Sprintf(api.JSApiStreamDeleteT, s.Name()), nil, &resp)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("unknown failure")
	}

	return nil
}

// Seal updates a stream so that messages can not be added or removed using the API and limits will not be processed - messages will never age out.
// A sealed stream can not be unsealed.
func (s *Stream) Seal() error {
	cfg := s.Configuration()
	cfg.Sealed = true
	return s.UpdateConfiguration(cfg)
}

// Purge deletes messages from the Stream, an optional JSApiStreamPurgeRequest can be supplied to limit the purge to a subset of messages
func (s *Stream) Purge(opts ...*api.JSApiStreamPurgeRequest) error {
	if len(opts) > 1 {
		return fmt.Errorf("only one purge option allowed")
	}

	var req *api.JSApiStreamPurgeRequest
	if len(opts) == 1 {
		req = opts[0]
	}

	var resp api.JSApiStreamPurgeResponse
	err := s.mgr.jsonRequest(fmt.Sprintf(api.JSApiStreamPurgeT, s.Name()), req, &resp)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("unknown failure")
	}

	return nil
}

// ReadLastMessageForSubject reads the last message stored in the stream for a specific subject
func (s *Stream) ReadLastMessageForSubject(subj string) (*api.StoredMsg, error) {
	return s.mgr.ReadLastMessageForSubject(s.Name(), subj)
}

// ReadMessage loads a message from the stream by its sequence number
func (s *Stream) ReadMessage(seq uint64) (msg *api.StoredMsg, err error) {
	var resp api.JSApiMsgGetResponse
	err = s.mgr.jsonRequest(fmt.Sprintf(api.JSApiMsgGetT, s.Name()), api.JSApiMsgGetRequest{Seq: seq}, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Message, nil
}

// FastDeleteMessage deletes a specific message from the Stream without erasing the data, see DeleteMessage() for a safe delete
func (s *Stream) FastDeleteMessage(seq uint64) error {
	return s.mgr.DeleteStreamMessage(s.Name(), seq, true)
}

// DeleteMessage deletes a specific message from the Stream by overwriting it with random data, see FastDeleteMessage() to remove the message without over writing data
func (s *Stream) DeleteMessage(seq uint64) (err error) {
	var resp api.JSApiMsgDeleteResponse
	err = s.mgr.jsonRequest(fmt.Sprintf(api.JSApiMsgDeleteT, s.Name()), api.JSApiMsgDeleteRequest{Seq: seq}, &resp)
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
	return api.JSAdvisoryPrefix + ".*.*." + s.Name() + ".>"

}

// MetricSubject is a wildcard subscription subject that subscribes to all advisories for this stream
func (s *Stream) MetricSubject() string {
	return api.JSMetricPrefix + ".*.*." + s.Name() + ".*"
}

// RemoveRAFTPeer removes a peer from the group indicating it will not return
func (s *Stream) RemoveRAFTPeer(peer string) error {
	var resp api.JSApiStreamRemovePeerResponse
	err := s.mgr.jsonRequest(fmt.Sprintf(api.JSApiStreamRemovePeerT, s.Name()), api.JSApiStreamRemovePeerRequest{Peer: peer}, &resp)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("unknown error while removing peer %q", peer)
	}

	return nil
}

// LeaderStepDown requests the current RAFT group leader in a clustered JetStream to stand down forcing a new election
func (s *Stream) LeaderStepDown() error {
	var resp api.JSApiStreamLeaderStepDownResponse
	err := s.mgr.jsonRequest(fmt.Sprintf(api.JSApiStreamLeaderStepDownT, s.Name()), nil, &resp)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("unknown error while requesting leader step down")
	}

	return nil
}

// DetectGaps detects interior deletes in a stream, reports progress through the stream and each found gap.
//
// It uses the extended stream info to get the sequences and use that to detect gaps. The Deleted information
// in StreamInfo is capped at some amount so if it determines there are more messages that are deleted in the
// stream it will then make a consumer and walk the remainder of the stream to detect gaps the hard way
func (s *Stream) DetectGaps(ctx context.Context, progress func(seq uint64, pending uint64), gap func(first uint64, last uint64)) error {
	nc := s.mgr.NatsConn()
	msgs := make(chan *nats.Msg, 10000)

	nfo, err := s.Information(api.JSApiStreamInfoRequest{DeletedDetails: true})
	if err != nil {
		return err
	}

	progress(nfo.State.Msgs, 0)

	if len(nfo.State.Deleted) == 0 {
		return nil
	}

	if len(nfo.State.Deleted) == 1 {
		gap(nfo.State.Deleted[0], nfo.State.Deleted[0])
	}

	start := nfo.State.Deleted[0]

	for i, seq := range nfo.State.Deleted {
		progress(seq, nfo.State.Msgs-seq)

		// the last message
		if i == len(nfo.State.Deleted)-1 {
			// if its part of a gap we close it off
			if seq-1 == nfo.State.Deleted[i-1] {
				gap(start, seq)
				return nil
			}

			// else its a start and end of a gap
			gap(seq, seq)
			return nil
		}

		// a normal message that isnt in sequence so its the
		// end and start of a gap
		if nfo.State.Deleted[i+1] != seq+1 {
			gap(start, seq)
			start = nfo.State.Deleted[i+1]
		}
	}

	// if we have more to do than what was returned from stream info
	// we do the hard thing and walk the stream, the consumer will start
	// at the last message after the deleted information
	if len(nfo.State.Deleted) == nfo.State.NumDeleted {
		return nil
	}

	sub, err := nc.ChanSubscribe(nc.NewRespInbox(), msgs)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	_, err = s.NewConsumer(DeliverHeadersOnly(), PushFlowControl(), DeliverySubject(sub.Subject), InactiveThreshold(time.Minute), IdleHeartbeat(time.Second), AcknowledgeNone(), StartAtSequence(nfo.State.Deleted[len(nfo.State.Deleted)-1]+1))
	if err != nil {
		return err
	}

	last := uint64(math.MaxUint64)

	for {
		select {
		case msg := <-msgs:
			if fc := msg.Header.Get("Nats-Consumer-Stalled"); fc != "" {
				nc.Publish(fc, nil)
				continue
			}
			meta, err := ParseJSMsgMetadata(msg)
			if err != nil {
				continue
			}

			progress(meta.StreamSequence(), meta.Pending())

			if meta.Pending() == 0 {
				return nil
			}

			if last == math.MaxUint64 {
				last = meta.StreamSequence()
				continue
			}

			if meta.StreamSequence() != last+1 {
				gap(last+1, meta.StreamSequence()-1)
			}

			last = meta.StreamSequence()

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// IsTemplateManaged determines if this stream is managed by a template
func (s *Stream) IsTemplateManaged() bool { return s.Template() != "" }

// IsMirror determines if this stream is a mirror of another
func (s *Stream) IsMirror() bool { return s.cfg.Mirror != nil }

// IsSourced determines if this stream is sourcing data from another stream. Other streams
// could be synced to this stream and it would not be reported by this property
func (s *Stream) IsSourced() bool { return len(s.cfg.Sources) > 0 }

// IsInternal indicates if a stream is considered 'internal' by the NATS team,
// that is, it's a backing stream for KV, Object or MQTT state
func (s *Stream) IsInternal() bool {
	return IsInternalStream(s.Name())
}

// IsKVBucket determines if a stream is a KV bucket
func (s *Stream) IsKVBucket() bool {
	return IsKVBucketStream(s.Name())
}

// IsObjectBucket determines if a stream is a Object bucket
func (s *Stream) IsObjectBucket() bool {
	return IsObjectBucketStream(s.Name())
}

// IsMQTTState determines if a stream holds internal MQTT state
func (s *Stream) IsMQTTState() bool {
	return IsMQTTStateStream(s.Name())
}

// IsCompressed determines if a stream is compressed
func (s *Stream) IsCompressed() bool {
	return s.Compression() != api.NoCompression
}

// ContainedSubjects queries the stream for the subjects it holds with optional filter
func (s *Stream) ContainedSubjects(filter ...string) (map[string]uint64, error) {
	return s.mgr.StreamContainedSubjects(s.Name(), filter...)
}

func (s *Stream) Configuration() api.StreamConfig          { return *s.cfg }
func (s *Stream) Name() string                             { return s.cfg.Name }
func (s *Stream) Description() string                      { return s.cfg.Description }
func (s *Stream) Subjects() []string                       { return s.cfg.Subjects }
func (s *Stream) Retention() api.RetentionPolicy           { return s.cfg.Retention }
func (s *Stream) DiscardPolicy() api.DiscardPolicy         { return s.cfg.Discard }
func (s *Stream) DiscardNewPerSubject() bool               { return s.cfg.DiscardNewPer }
func (s *Stream) MaxConsumers() int                        { return s.cfg.MaxConsumers }
func (s *Stream) MaxMsgs() int64                           { return s.cfg.MaxMsgs }
func (s *Stream) MaxMsgsPerSubject() int64                 { return s.cfg.MaxMsgsPer }
func (s *Stream) MaxBytes() int64                          { return s.cfg.MaxBytes }
func (s *Stream) MaxAge() time.Duration                    { return s.cfg.MaxAge }
func (s *Stream) MaxMsgSize() int32                        { return s.cfg.MaxMsgSize }
func (s *Stream) Storage() api.StorageType                 { return s.cfg.Storage }
func (s *Stream) Replicas() int                            { return s.cfg.Replicas }
func (s *Stream) NoAck() bool                              { return s.cfg.NoAck }
func (s *Stream) Template() string                         { return s.cfg.Template }
func (s *Stream) DuplicateWindow() time.Duration           { return s.cfg.Duplicates }
func (s *Stream) Mirror() *api.StreamSource                { return s.cfg.Mirror }
func (s *Stream) Sources() []*api.StreamSource             { return s.cfg.Sources }
func (s *Stream) Sealed() bool                             { return s.cfg.Sealed }
func (s *Stream) DeleteAllowed() bool                      { return !s.cfg.DenyDelete }
func (s *Stream) PurgeAllowed() bool                       { return !s.cfg.DenyPurge }
func (s *Stream) RollupAllowed() bool                      { return s.cfg.RollupAllowed }
func (s *Stream) DirectAllowed() bool                      { return s.cfg.AllowDirect }
func (s *Stream) MirrorDirectAllowed() bool                { return s.cfg.MirrorDirect }
func (s *Stream) Republish() *api.RePublish                { return s.cfg.RePublish }
func (s *Stream) IsRepublishing() bool                     { return s.Republish() != nil }
func (s *Stream) Metadata() map[string]string              { return s.cfg.Metadata }
func (s *Stream) Compression() api.Compression             { return s.cfg.Compression }
func (s *Stream) FirstSequence() uint64                    { return s.cfg.FirstSeq }
func (s *Stream) ConsumerLimits() api.StreamConsumerLimits { return s.cfg.ConsumerLimits }
