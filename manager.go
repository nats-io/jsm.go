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
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

type Manager struct {
	nc          *nats.Conn
	timeout     time.Duration
	trace       bool
	validator   api.StructValidator
	apiPrefix   string
	eventPrefix string
	domain      string
	pedantic    bool
	apiLEvel    *int

	sync.Mutex
}

func New(nc *nats.Conn, opts ...Option) (*Manager, error) {
	m := &Manager{
		nc:      nc,
		timeout: 5 * time.Second,
	}

	for _, opt := range opts {
		opt(m)
	}

	if m.nc == nil {
		return nil, fmt.Errorf("nats connection not supplied")
	}

	if m.timeout < 500*time.Millisecond {
		m.timeout = 500 * time.Millisecond
	}

	return m, nil
}

// IsPedantic checks if the manager is in pedantic mode
func (m *Manager) IsPedantic() bool {
	return m.pedantic
}

// IsJetStreamEnabled determines if JetStream is enabled for the current account
func (m *Manager) IsJetStreamEnabled() bool {
	info, err := m.JetStreamAccountInfo()
	if err != nil {
		return false
	}

	if info == nil {
		return false
	}

	return true
}

// JetStreamAccountInfo retrieves information about the current account limits and more
func (m *Manager) JetStreamAccountInfo() (info *api.JetStreamAccountStats, err error) {
	var resp api.JSApiAccountInfoResponse
	err = m.jsonRequest(api.JSApiAccountInfo, nil, &resp)
	if err != nil {
		if errors.Is(err, nats.ErrNoResponders) {
			return nil, nats.ErrJetStreamNotEnabled
		}
		return nil, err
	}

	m.Lock()
	m.apiLEvel = &resp.JetStreamAccountStats.API.Level
	m.Unlock()

	return resp.JetStreamAccountStats, nil
}

// MetaApiLevel determines the JetStream API level supported by the meta leader
func (m *Manager) MetaApiLevel(refresh bool) (int, error) {
	m.Lock()
	mlvl := m.apiLEvel
	m.Unlock()

	if !refresh && mlvl != nil {
		return *mlvl, nil
	}

	nfo, err := m.JetStreamAccountInfo()
	if err != nil {
		return 0, err
	}

	return nfo.API.Level, nil
}

// IsStreamMaxBytesRequired determines if the JetStream account requires streams to set a byte limit
func (m *Manager) IsStreamMaxBytesRequired() (bool, error) {
	nfo, err := m.JetStreamAccountInfo()
	if err != nil {
		return false, err
	}

	if nfo.Limits.MaxBytesRequired {
		return true, nil
	}

	for _, t := range nfo.Tiers {
		if t.Limits.MaxBytesRequired {
			return true, nil
		}
	}

	return false, nil
}

func (m *Manager) jsonRequest(subj string, req any, response any) (err error) {
	var body []byte

	switch {
	case req == nil:
		body = []byte("")
	default:
		body, err = json.Marshal(req)
		if err != nil {
			return err
		}

		if string(body) == "null" {
			body = nil
		}
	}

	msg, err := m.request(m.apiSubject(subj), body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(msg.Data, response)
	if err != nil {
		return err
	}

	jsr, ok := response.(jetStreamResponseError)
	if !ok {
		return nil
	}

	if jsr.ToError() != nil {
		return jsr.ToError()
	}

	if m.validator == nil {
		return nil
	}

	jv, ok := response.(apiValidatable)
	if !ok {
		return fmt.Errorf("invalid validator specified")
	}

	valid, errs := jv.Validate(m.validator)
	if valid {
		return nil
	}

	return fmt.Errorf("server response is not a valid %q message: %s", jv.SchemaType(), strings.Join(errs, "\n"))
}

// StreamNamesFilter limits the names being returned by the names API
type StreamNamesFilter struct {
	// Subject filter the names to those consuming messages matching this subject or wildcard
	Subject string `json:"subject,omitempty"`
}

// StreamNames is a sorted list of all known Streams filtered by filter
func (m *Manager) StreamNames(filter *StreamNamesFilter) (names []string, err error) {
	resp := func() apiIterableResponse { return &api.JSApiStreamNamesResponse{} }
	req := &api.JSApiStreamNamesRequest{JSApiIterableRequest: api.JSApiIterableRequest{Offset: 0}}
	if filter != nil {
		req.Subject = filter.Subject
	}

	err = m.iterableRequest(api.JSApiStreamNames, req, resp, func(page any) error {
		apiresp, ok := page.(*api.JSApiStreamNamesResponse)
		if !ok {
			return fmt.Errorf("invalid response type from iterable request")
		}

		names = append(names, apiresp.Streams...)

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(names)

	return names, nil
}

// DeleteStreamMessage deletes a specific message from the Stream without erasing the data, see DeleteMessage() for a safe delete
func (m *Manager) DeleteStreamMessage(stream string, seq uint64, noErase bool) error {
	var resp api.JSApiMsgDeleteResponse
	err := m.jsonRequest(fmt.Sprintf(api.JSApiMsgDeleteT, stream), api.JSApiMsgDeleteRequest{Seq: seq, NoErase: noErase}, &resp)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("unknown error while deleting message %d", seq)
	}

	return nil
}

// ReadLastMessageForSubject reads the last message stored in the stream for a specific subject
func (m *Manager) ReadLastMessageForSubject(stream string, sub string) (msg *api.StoredMsg, err error) {
	var resp api.JSApiMsgGetResponse
	err = m.jsonRequest(fmt.Sprintf(api.JSApiMsgGetT, stream), api.JSApiMsgGetRequest{LastFor: sub}, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Message, nil
}

func (m *Manager) iterableRequest(subj string, req apiIterableRequest, response func() apiIterableResponse, cb func(any) error) (err error) {
	offset := 0
	for {
		req.SetOffset(offset)
		r := response()
		err = m.jsonRequest(subj, req, r)
		if err != nil {
			return err
		}

		err = cb(r)
		if err != nil {
			return err
		}

		if r.LastPage() {
			break
		}

		offset += r.ItemsLimit()
	}

	return nil
}

func (m *Manager) request(subj string, data []byte) (res *nats.Msg, err error) {
	return m.requestWithTimeout(subj, data, m.timeout)
}

func (m *Manager) requestWithTimeout(subj string, data []byte, timeout time.Duration) (res *nats.Msg, err error) {
	if m == nil || m.nc == nil {
		return nil, fmt.Errorf("nats connection is not set")
	}

	var ctx context.Context
	var cancel func()

	if timeout == 0 {
		timeout = m.timeout
	}

	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res, err = m.requestWithContext(ctx, subj, data)
	if err != nil {
		return nil, err
	}

	return res, err
}

func (m *Manager) requestWithContext(ctx context.Context, subj string, data []byte) (res *nats.Msg, err error) {
	if m.trace {
		log.Printf(">>> %s\n%s\n\n", subj, string(data))
	}

	res, err = m.nc.RequestWithContext(ctx, subj, data)
	if err != nil {
		if m.trace {
			log.Printf("<<< %s: %s\n\n", subj, err.Error())
		}

		return res, err
	}

	if m.trace {
		log.Printf("<<< %s\n%s\n\n", subj, string(res.Data))
	}

	return res, ParseErrorResponse(res)
}

// IsKnownStream determines if a Stream is known
func (m *Manager) IsKnownStream(stream string) (bool, error) {
	s, err := m.LoadStream(stream)
	if err != nil {
		jserr, ok := err.(api.ApiError)
		if ok {
			if jserr.NotFoundError() {
				return false, nil
			}
		}

		return false, err
	}

	if s.Name() != stream {
		return false, fmt.Errorf("received invalid stream from load")
	}

	return true, nil
}

// IsKnownConsumer determines if a Consumer is known for a specific Stream
func (m *Manager) IsKnownConsumer(stream string, consumer string) (bool, error) {
	c, err := m.LoadConsumer(stream, consumer)
	if err != nil {
		jserr, ok := err.(api.ApiError)
		if ok {
			if jserr.NotFoundError() {
				return false, nil
			}
		}

		return false, err
	}

	if c.Name() != consumer {
		return false, fmt.Errorf("invalid consumer received from load")
	}

	return true, nil
}

// EachStream iterates over all known Streams, does not handle any streams the cluster could not get data from but returns a list of those
func (m *Manager) EachStream(filter *StreamNamesFilter, cb func(*Stream)) (missing []string, err error) {
	streams, missing, err := m.Streams(filter)
	if err != nil {
		return nil, err
	}

	for _, s := range streams {
		cb(s)
	}

	return missing, nil
}

// Consumers is a sorted list of all known Consumers within a Stream and a list of any consumer names that were known but no details were found
func (m *Manager) Consumers(stream string) (consumers []*Consumer, missing []string, err error) {
	if !IsValidName(stream) {
		return nil, nil, fmt.Errorf("%q is not a valid stream name", stream)
	}

	var (
		cinfo []*api.ConsumerInfo
		resp  = func() apiIterableResponse { return &api.JSApiConsumerListResponse{} }
	)

	err = m.iterableRequest(fmt.Sprintf(api.JSApiConsumerListT, stream), &api.JSApiConsumerListRequest{JSApiIterableRequest: api.JSApiIterableRequest{Offset: 0}}, resp, func(page any) error {
		apiresp, ok := page.(*api.JSApiConsumerListResponse)
		if !ok {
			return fmt.Errorf("invalid response type from iterable request")
		}

		missing = append(missing, apiresp.Missing...)
		cinfo = append(cinfo, apiresp.Consumers...)
		return nil
	})
	if err != nil {
		return consumers, missing, err
	}

	sort.Slice(cinfo, func(i int, j int) bool {
		return cinfo[i].Name < cinfo[j].Name
	})

	for _, c := range cinfo {
		consumer := m.consumerFromCfg(c.Stream, c.Name, &c.Config)
		consumer.lastInfo = c

		consumers = append(consumers, consumer)
	}

	return consumers, missing, nil
}

// ConsumerNames is a sorted list of all known consumers within a stream
func (m *Manager) ConsumerNames(stream string) (names []string, err error) {
	if !IsValidName(stream) {
		return nil, fmt.Errorf("%q is not a valid stream name", stream)
	}

	err = m.iterableRequest(fmt.Sprintf(api.JSApiConsumerNamesT, stream), &api.JSApiConsumerNamesRequest{JSApiIterableRequest: api.JSApiIterableRequest{Offset: 0}}, func() apiIterableResponse { return &api.JSApiConsumerNamesResponse{} }, func(page any) error {
		apiresp, ok := page.(*api.JSApiConsumerNamesResponse)
		if !ok {
			return fmt.Errorf("invalid response type from iterable request")
		}

		names = append(names, apiresp.Consumers...)

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(names)

	return names, nil
}

// Streams is a sorted list of all known Streams and a list of any stream names that were known but no details were found
func (m *Manager) Streams(filter *StreamNamesFilter) ([]*Stream, []string, error) {
	var (
		streams []*Stream
		missing []string
		err     error
		resp    = func() apiIterableResponse { return &api.JSApiStreamListResponse{} }
	)

	req := &api.JSApiStreamListRequest{JSApiIterableRequest: api.JSApiIterableRequest{Offset: 0}}
	if filter != nil {
		req.Subject = filter.Subject
	}

	err = m.iterableRequest(api.JSApiStreamList, req, resp, func(page any) error {
		apiresp, ok := page.(*api.JSApiStreamListResponse)
		if !ok {
			return fmt.Errorf("invalid response type from iterable request")
		}

		sort.Slice(apiresp.Streams, func(i int, j int) bool {
			return apiresp.Streams[i].Config.Name < apiresp.Streams[j].Config.Name
		})

		for _, s := range apiresp.Streams {
			streams = append(streams, m.streamFromConfig(&s.Config, s))
		}

		missing = append(missing, apiresp.Missing...)

		return nil
	})

	return streams, missing, err
}

func (m *Manager) apiSubject(subject string) string {
	return APISubject(subject, m.apiPrefix, m.domain)
}

// MetaLeaderStandDown requests the meta group leader to stand down, must be initiated by a system user
func (m *Manager) MetaLeaderStandDown(placement *api.Placement) error {
	var resp api.JSApiLeaderStepDownResponse
	err := m.jsonRequest(api.JSApiLeaderStepDown, api.JSApiLeaderStepDownRequest{Placement: placement}, &resp)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("unknown error while requesting leader step down")
	}

	return nil
}

// DeleteStream removes a stream without all the drama of loading it etc
func (m *Manager) DeleteStream(stream string) error {
	if stream == "" || strings.ContainsAny(stream, ".>*") {
		return fmt.Errorf("invalid stream name")
	}

	var resp api.JSApiStreamDeleteResponse
	err := m.jsonRequest(fmt.Sprintf(api.JSApiStreamDeleteT, stream), nil, &resp)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("deleting stream %s failed", stream)
	}

	return nil
}

// DeleteConsumer removes a consumer without all the drama of loading it etc
func (m *Manager) DeleteConsumer(stream string, consumer string) error {
	if stream == "" || strings.ContainsAny(stream, ".>*") {
		return fmt.Errorf("invalid stream name")
	}
	if consumer == "" || strings.ContainsAny(consumer, ".>*") {
		return fmt.Errorf("invalid consumer name")
	}

	var resp api.JSApiConsumerDeleteResponse
	err := m.jsonRequest(fmt.Sprintf(api.JSApiConsumerDeleteT, stream, consumer), nil, &resp)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("deleting consumer %s > %s failed", stream, consumer)
	}

	return nil
}

// StreamContainedSubjects queries the stream for the subjects it holds with optional filter
func (m *Manager) StreamContainedSubjects(stream string, filter ...string) (map[string]uint64, error) {
	if len(filter) > 1 {
		return nil, fmt.Errorf("only 1 filter supported")
	}

	f := ">"
	if len(filter) == 1 && filter[0] != "" {
		f = filter[0]
	}

	subjects := make(map[string]uint64)

	resp := func() apiIterableResponse { return &api.JSApiStreamInfoResponse{} }
	req := &api.JSApiStreamInfoRequest{SubjectsFilter: f}

	err := m.iterableRequest(fmt.Sprintf(api.JSApiStreamInfoT, stream), req, resp, func(page any) error {
		apiresp, ok := page.(*api.JSApiStreamInfoResponse)
		if !ok {
			return fmt.Errorf("invalid response type from iterable request")
		}

		for s, i := range apiresp.State.Subjects {
			subjects[s] = i
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return subjects, nil
}

// MetaPeerRemove removes a peer from the JetStream meta cluster, evicting all streams, consumer etc.  Use with extreme caution.
// If id is given it will be used by the server else name, it's generally best to remove by id
func (m *Manager) MetaPeerRemove(name string, id string) error {
	var resp api.JSApiMetaServerRemoveResponse
	err := m.jsonRequest(api.JSApiRemoveServer, api.JSApiMetaServerRemoveRequest{Server: name, Peer: id}, &resp)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("unknown error while removing the peer")
	}

	return nil
}

// MetaPurgeAccount removes all data from an account, must be run in the system account
func (m *Manager) MetaPurgeAccount(account string) error {
	if account == "" {
		return fmt.Errorf("account is required")
	}

	var resp api.JSApiAccountPurgeResponse
	err := m.jsonRequest(fmt.Sprintf(api.JSApiPurgeAccountT, account), nil, &resp)
	if err != nil {
		return err
	}

	if !resp.Initiated {
		return fmt.Errorf("unknown error while purging the account")
	}

	return nil
}

// NatsConn gives access to the underlying NATS Connection
func (m *Manager) NatsConn() *nats.Conn {
	m.Lock()
	defer m.Unlock()

	return m.nc
}
