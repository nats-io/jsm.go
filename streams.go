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
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
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

	valid, errs := cfg.Validate(m.validator)
	if !valid {
		return nil, fmt.Errorf("configuration validation failed: %s", strings.Join(errs, ", "))
	}

	var resp api.JSApiStreamCreateResponse

	req := api.JSApiStreamCreateRequest{
		Pedantic:     m.pedantic,
		StreamConfig: *cfg,
	}

	err = m.jsonRequest(fmt.Sprintf(api.JSApiStreamCreateT, name), &req, &resp)
	if err != nil {
		return nil, err
	}

	return m.streamFromConfig(&resp.Config, resp.StreamInfo), nil
}

// LoadFromStreamDetailBytes creates a stream info from the server StreamDetails in json format, the StreamDetails should
// be created with Config and Consumers options set
func (m *Manager) LoadFromStreamDetailBytes(sd []byte) (stream *Stream, consumers []*Consumer, err error) {
	stream = &Stream{
		mgr: m,
	}

	var nfo api.StreamInfo
	err = json.Unmarshal(sd, &nfo)
	if err != nil {
		return nil, nil, err
	}

	stream.lastInfo = &nfo
	stream.cfg = &nfo.Config

	if stream.Name() == "" {
		return nil, nil, fmt.Errorf("invalid stream details, ensure configuration is included")
	}

	var cons struct {
		Consumers []*api.ConsumerInfo `json:"consumer_detail"`
	}
	err = json.Unmarshal(sd, &cons)
	if err != nil {
		return nil, nil, err
	}

	for _, consumer := range cons.Consumers {
		c := Consumer{
			name:     consumer.Name,
			stream:   stream.Name(),
			cfg:      &consumer.Config,
			lastInfo: consumer,
			mgr:      m,
		}

		consumers = append(consumers, &c)
	}

	return stream, consumers, nil
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

func PlacementPreferredLeader(leader string) StreamOption {
	return func(o *api.StreamConfig) error {
		if o.Placement == nil {
			o.Placement = &api.Placement{}
		}

		o.Placement.Preferred = leader

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

func AllowAtomicBatchPublish() StreamOption {
	return func(o *api.StreamConfig) error {
		o.AllowAtomicPublish = true
		return nil
	}
}

func NoAllowAtomicBatchPublish() StreamOption {
	return func(o *api.StreamConfig) error {
		o.AllowAtomicPublish = false
		return nil
	}
}

func AllowCounter() StreamOption {
	return func(o *api.StreamConfig) error {
		o.AllowMsgCounter = true
		return nil
	}
}

func NoAllowCounter() StreamOption {
	return func(o *api.StreamConfig) error {
		o.AllowMsgCounter = false
		return nil
	}
}

func AllowSchedules() StreamOption {
	return func(o *api.StreamConfig) error {
		o.AllowMsgSchedules = true
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

func AllowMsgTTL() StreamOption {
	return func(o *api.StreamConfig) error {
		o.AllowMsgTTL = true
		return nil
	}
}

func SubjectDeleteMarkerTTL(d time.Duration) StreamOption {
	return func(o *api.StreamConfig) error {
		o.SubjectDeleteMarkerTTL = d

		return nil
	}
}

func SubjectTransform(subjectTransform *api.SubjectTransformConfig) StreamOption {
	return func(o *api.StreamConfig) error {
		o.SubjectTransform = subjectTransform
		return nil
	}
}

func AsyncPersistence() StreamOption {
	return func(o *api.StreamConfig) error {
		o.PersistMode = api.AsyncPersistMode
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

	req := api.JSApiStreamUpdateRequest{
		Pedantic:     s.mgr.pedantic,
		StreamConfig: *ncfg,
	}

	var resp api.JSApiStreamUpdateResponse
	err = s.mgr.jsonRequest(fmt.Sprintf(api.JSApiStreamUpdateT, s.Name()), &req, &resp)
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
func (s *Stream) EachConsumer(cb func(consumer *Consumer)) (missing []string, offline map[string]string, err error) {
	consumers, missing, offline, err := s.mgr.Consumers(s.Name())
	if err != nil {
		return nil, nil, err
	}

	for _, c := range consumers {
		cb(c)
	}

	return missing, offline, nil
}

// LatestInformation returns the most recently fetched stream information
func (s *Stream) LatestInformation() (info *api.StreamInfo, err error) {
	nfo := s.lastInfoLocked()

	if nfo != nil {
		return nfo, nil
	}

	return s.Information()
}

func (s *Stream) lastInfoLocked() *api.StreamInfo {
	s.Lock()
	defer s.Unlock()

	return s.lastInfo
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

func (s *Stream) ClusterInfo() (api.ClusterInfo, error) {
	nfo, err := s.LatestInformation()
	if err != nil {
		return api.ClusterInfo{}, err
	}

	return *nfo.Cluster, nil
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
//
// Deprecated: Use DeleteMessageRequest()
func (s *Stream) FastDeleteMessage(seq uint64) error {
	return s.DeleteMessageRequest(api.JSApiMsgDeleteRequest{Seq: seq, NoErase: true})
}

// DeleteMessage deletes a specific message from the Stream by overwriting it with random data, see FastDeleteMessage() to remove the message without over writing data
//
// Deprecated: Use DeleteMessageRequest()
func (s *Stream) DeleteMessage(seq uint64) (err error) {
	return s.DeleteMessageRequest(api.JSApiMsgDeleteRequest{Seq: seq})
}

// DeleteMessageRequest deletes a specific message from the Stream with a full request
func (s *Stream) DeleteMessageRequest(req api.JSApiMsgDeleteRequest) (err error) {
	if req.Seq == 0 {
		return fmt.Errorf("sequence number is required")
	}

	var resp api.JSApiMsgDeleteResponse
	err = s.mgr.jsonRequest(fmt.Sprintf(api.JSApiMsgDeleteT, s.Name()), req, &resp)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("unknown error while deleting message %d", req.Seq)
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

// LeaderStepDown requests the current RAFT group leader in a clustered JetStream to stand down forcing a new election, the election of the next leader can be influenced by placement
func (s *Stream) LeaderStepDown(placement ...*api.Placement) error {
	var p *api.Placement
	if len(placement) > 1 {
		return fmt.Errorf("only one placement option allowed")
	} else if len(placement) == 1 {
		p = placement[0]
	}

	var resp api.JSApiStreamLeaderStepDownResponse
	err := s.mgr.jsonRequest(fmt.Sprintf(api.JSApiStreamLeaderStepDownT, s.Name()), api.JSApiStreamLeaderStepDownRequest{Placement: p}, &resp)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("unknown error while requesting leader step down")
	}

	return nil
}

// DirectGet performs a direct get against the stream, supports Batch and Multi Subject behaviors
func (s *Stream) DirectGet(ctx context.Context, req api.JSApiMsgGetRequest, handler func(msg *nats.Msg)) (numPending uint64, lastSeq uint64, upToSeq uint64, err error) {
	if !s.DirectAllowed() {
		return 0, 0, 0, fmt.Errorf("direct gets are not enabled for %s", s.Name())
	}
	if req.Batch == 0 && req.LastFor != "" {
		return 0, 0, 0, fmt.Errorf("batch size is required")
	}

	rj, err := json.Marshal(req)
	if err != nil {
		return 0, 0, 0, err
	}

	nc := s.mgr.nc

	to, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	timer := time.AfterFunc(s.mgr.timeout, func() {
		cancel(fmt.Errorf("timeout waiting for messages"))
	})
	defer timer.Stop()

	sub, err := nc.Subscribe(nc.NewRespInbox(), func(m *nats.Msg) {
		var err error

		// move the timeout forward by 1 x timeout after getting any messages
		timer.Reset(s.mgr.timeout)

		switch m.Header.Get("Status") {
		case "204": // end of batch
			ls := m.Header.Get("Nats-Last-Sequence")
			upTo := m.Header.Get("Nats-UpTo-Sequence")
			if ls != "" {
				lastSeq, err = strconv.ParseUint(ls, 10, 64)
				if err != nil {
					cancel(fmt.Errorf("invalid last sequence: %w", err))
				}
			}

			if upTo != "" {
				upToSeq, err = strconv.ParseUint(upTo, 10, 64)
				if err != nil {
					cancel(fmt.Errorf("invalid up-to sequence: %w", err))
				}
			}

			cancel(nil)
			return

		case "404": // not found
			cancel(fmt.Errorf("no messages found matching request"))
			return

		case "408": // invalid requests
			cancel(fmt.Errorf("invalid request"))
			return

		case "413": // too many subjects
			cancel(fmt.Errorf("too many subjects requested"))
			return
		}

		np := m.Header.Get("Nats-Num-Pending")
		if np == "" {
			cancel(fmt.Errorf("server does not support batch requests"))
			return
		}

		handler(m)
	})
	if err != nil {
		return 0, 0, 0, err
	}
	defer sub.Unsubscribe()

	msg := nats.NewMsg(s.DirectSubject())
	msg.Data = rj
	msg.Reply = sub.Subject

	err = nc.PublishMsg(msg)

	<-to.Done()

	// if we got canceled without a error its just normal, like on EOB
	err = context.Cause(to)
	if errors.Is(err, context.Canceled) {
		err = nil
	}

	return numPending, lastSeq, upToSeq, err
}

// DirectSubject is the subject to perform direct gets against
func (s *Stream) DirectSubject() string {
	return fmt.Sprintf(api.JSDirectMsgGetT, s.Name())
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

	progress(nfo.State.Msgs, nfo.State.Msgs)

	if len(nfo.State.Deleted) == 0 {
		return nil
	}

	if len(nfo.State.Deleted) == 1 {
		seq := nfo.State.Deleted[0]
		gap(seq, seq)
		progress(seq, 0)
		return nil
	}

	start := nfo.State.Deleted[0]

	for i, seq := range nfo.State.Deleted {
		progress(seq, nfo.State.Msgs-seq)

		// the last deleted message
		if i == len(nfo.State.Deleted)-1 {
			// if its part of a gap we close it off
			if seq-1 == nfo.State.Deleted[i-1] {
				gap(start, seq)
				progress(seq, 0)
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
			progress(seq, 0)
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
func (s *Stream) AtomicBatchPublishAllowed() bool          { return s.cfg.AllowAtomicPublish }
func (s *Stream) CounterAllowed() bool                     { return s.cfg.AllowMsgCounter }
func (s *Stream) SchedulesAllowed() bool                   { return s.cfg.AllowMsgSchedules }
func (s *Stream) MirrorDirectAllowed() bool                { return s.cfg.MirrorDirect }
func (s *Stream) Republish() *api.RePublish                { return s.cfg.RePublish }
func (s *Stream) IsRepublishing() bool                     { return s.Republish() != nil }
func (s *Stream) Metadata() map[string]string              { return s.cfg.Metadata }
func (s *Stream) Compression() api.Compression             { return s.cfg.Compression }
func (s *Stream) FirstSequence() uint64                    { return s.cfg.FirstSeq }
func (s *Stream) AllowMsgTTL() bool                        { return s.cfg.AllowMsgTTL }
func (s *Stream) SubjectDeleteMarkerTTL() time.Duration    { return s.cfg.SubjectDeleteMarkerTTL }
func (s *Stream) ConsumerLimits() api.StreamConsumerLimits { return s.cfg.ConsumerLimits }
func (s *Stream) PersistenceMode() api.PersistModeType     { return s.cfg.PersistMode }
