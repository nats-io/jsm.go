// Copyright 2021 The NATS Authors
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

package kv

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
)

type jetStreamStorage struct {
	name          string
	streamName    string
	subjectPrefix string
	bucketSubject string
	stream        *jsm.Stream
	nc            *nats.Conn
	mgr           *jsm.Manager
	mu            sync.Mutex
	opts          *options
	log           Logger
}

func newJetStreamStorage(name string, nc *nats.Conn, opts *options) (Storage, error) {
	if !IsValidBucket(name) {
		return nil, fmt.Errorf("invalid bucket name")
	}

	mgr, err := jsm.New(nc, jsm.WithTimeout(opts.timeout))
	if err != nil {
		return nil, err
	}

	js := &jetStreamStorage{
		name:          name,
		nc:            nc,
		mgr:           mgr,
		subjectPrefix: "kv",
		log:           opts.log,
		opts:          opts,
	}

	if opts.overrideSubjectPrefix != "" {
		js.subjectPrefix = opts.overrideSubjectPrefix
	}

	if opts.overrideStreamName != "" {
		js.streamName = opts.overrideStreamName
	} else {
		js.streamName = js.streamForBucket(name)
	}

	js.bucketSubject = js.subjectForBucket(name)

	return js, nil
}

func (j *jetStreamStorage) Status() (Status, error) {
	stream, err := j.getOrLoadStream()
	if err != nil {
		return nil, err
	}

	state, err := stream.State()
	if err != nil {
		return nil, err
	}

	return &jsStatus{state: state, config: stream.Configuration()}, nil
}

func (j *jetStreamStorage) Close() error {
	j.mu.Lock()
	j.stream = nil
	j.mu.Unlock()

	return nil
}

func (j *jetStreamStorage) encode(val string) string {
	if j.opts.enc == nil {
		return val
	}

	return j.opts.enc(val)
}

func (j *jetStreamStorage) decode(val string) string {
	if j.opts.dec == nil {
		return val
	}

	return j.opts.dec(val)
}

func (j *jetStreamStorage) Put(key string, val string, opts ...PutOption) (seq uint64, err error) {
	ek := j.encode(key)
	if !IsValidKey(ek) {
		return 0, fmt.Errorf("invalid key")
	}

	msg := nats.NewMsg(j.subjectForKey(ek))
	msg.Data = []byte(j.encode(val))

	msg.Header.Add("KV-Origin-Server", j.nc.ConnectedServerName())
	msg.Header.Add("KV-Origin-Cluster", j.nc.ConnectedClusterName())

	if !j.opts.noShare {
		ip, err := j.nc.GetClientIP()
		if err == nil {
			msg.Header.Add("KV-Origin", ip.String())
		}
	}

	res, err := j.nc.RequestMsg(msg, j.opts.timeout)
	if err != nil {
		return 0, err
	}
	pa, err := jsm.ParsePubAck(res)
	if err != nil {
		return 0, err
	}

	return pa.Sequence, nil
}

func (j *jetStreamStorage) JSON(ctx context.Context) ([]byte, error) {
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()

	watch, err := j.WatchBucket(wctx)
	if err != nil {
		return nil, err
	}

	kv := make(map[string]Result)

	for r := range watch.Channel() {
		kv[r.Key()] = r
		if r.Delta() == 0 {
			cancel()
		}
	}

	return json.MarshalIndent(kv, "", "  ")
}

func (j *jetStreamStorage) Get(key string) (Result, error) {
	ek := j.encode(key)
	if !IsValidKey(ek) {
		return nil, fmt.Errorf("invalid key")
	}

	msg, err := j.mgr.ReadLastMessageForSubject(j.streamName, j.subjectForKey(ek))
	if err != nil {
		if apiErr, ok := err.(api.ApiError); ok {
			if apiErr.NatsErrorCode() == 10037 {
				return nil, fmt.Errorf("unknown key: %s", key)
			}
		}

		return nil, err
	}

	return jsResultFromStoredMessage(j.name, key, msg, j.decode)
}

func (j *jetStreamStorage) Bucket() string        { return j.name }
func (j *jetStreamStorage) BucketSubject() string { return j.bucketSubject }

func (j *jetStreamStorage) WatchBucket(ctx context.Context) (Watch, error) {
	return newJSWatch(ctx, j.streamName, j.name, j.bucketSubject, j.opts.dec, j.nc, j.mgr, j.log)
}

func (j *jetStreamStorage) Watch(ctx context.Context, key string) (Watch, error) {
	ek := j.encode(key)
	if !IsValidKey(ek) {
		return nil, fmt.Errorf("invalid key")
	}

	return newJSWatch(ctx, j.streamName, j.name, j.subjectForKey(j.encode(key)), j.opts.dec, j.nc, j.mgr, j.log)
}

// Delete deletes all values held for a key
func (j *jetStreamStorage) Delete(key string) error {
	ek := j.encode(key)
	if !IsValidKey(ek) {
		return fmt.Errorf("invalid key")
	}

	stream, err := j.getOrLoadStream()
	if err != nil {
		return err
	}

	return stream.Purge(&api.JSApiStreamPurgeRequest{Subject: j.subjectForKey(ek)})
}

func (j *jetStreamStorage) Compact(key string, keep uint64) error {
	ek := j.encode(key)
	if !IsValidKey(ek) {
		return fmt.Errorf("invalid key")
	}

	stream, err := j.getOrLoadStream()
	if err != nil {
		return err
	}

	return stream.Purge(&api.JSApiStreamPurgeRequest{Subject: j.subjectForKey(ek), Keep: keep})
}

func (j *jetStreamStorage) Purge() error {
	stream, err := j.getOrLoadStream()
	if err != nil {
		return err
	}

	return stream.Purge()
}

func (j *jetStreamStorage) Destroy() error {
	stream, err := j.getOrLoadStream()
	if err != nil {
		return err
	}

	err = stream.Delete()
	if err != nil {
		return err
	}

	return nil
}

func (j *jetStreamStorage) loadBucket() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	stream, err := j.mgr.LoadStream(j.streamName)
	if err != nil {
		return err
	}

	j.stream = stream

	return err
}

// CreateBucket creates a bucket matching the supplied options if none exist, else loads the existing bucket, does not try to consolidate configuration if it already exists
func (j *jetStreamStorage) CreateBucket() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	opts := []jsm.StreamOption{
		jsm.MaxMessagesPerSubject(int64(j.opts.history)),
		jsm.LimitsRetention(),
	}

	if j.opts.replicas > 1 {
		opts = append(opts, jsm.Replicas(int(j.opts.replicas)))
	}

	if j.opts.placementCluster != "" {
		opts = append(opts, jsm.PlacementCluster(j.opts.placementCluster))
	}

	// TODO: mirrors
	opts = append(opts, jsm.Subjects(j.bucketSubject))

	if j.opts.ttl > 0 {
		opts = append(opts, jsm.MaxAge(j.opts.ttl))
	}

	stream, err := j.mgr.LoadOrNewStream(j.streamName, opts...)
	if err != nil {
		return err
	}

	j.stream = stream

	return nil
}

func (j *jetStreamStorage) streamForBucket(b string) string {
	return fmt.Sprintf("KV_%s", b)
}

func (j *jetStreamStorage) subjectForBucket(b string) string {
	return fmt.Sprintf("%s.%s.*", j.subjectPrefix, b)
}

func (j *jetStreamStorage) subjectForKey(k string) string {
	return fmt.Sprintf("%s.%s.%s", j.subjectPrefix, j.name, k)
}

func (j *jetStreamStorage) getOrLoadStream() (*jsm.Stream, error) {
	j.mu.Lock()
	stream := j.stream
	j.mu.Unlock()

	if stream != nil {
		return stream, nil
	}

	err := j.loadBucket()
	if err != nil {
		return nil, err
	}

	if stream == nil {
		return nil, fmt.Errorf("no stream found")
	}

	j.mu.Lock()
	stream = j.stream
	j.mu.Unlock()

	return stream, nil
}
