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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/textproto"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
)

type jsResult struct {
	bucket  string
	key     string
	val     string
	ts      time.Time
	seq     uint64
	pending uint64

	ocluster string
}

func (j *jsResult) Bucket() string        { return j.bucket }
func (j *jsResult) Key() string           { return j.key }
func (j *jsResult) Value() string         { return j.val }
func (j *jsResult) Created() time.Time    { return j.ts }
func (j *jsResult) Sequence() uint64      { return j.seq }
func (j *jsResult) Delta() uint64         { return j.pending }
func (j *jsResult) OriginCluster() string { return j.ocluster }
func (j *jsResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.genericResult())
}

func (j *jsResult) genericResult() *GenericResult {
	return &GenericResult{
		Bucket:        j.bucket,
		Key:           j.key,
		Val:           j.val,
		Created:       j.ts.UnixNano(),
		Seq:           j.seq,
		OriginCluster: j.ocluster,
	}
}

func jsResultFromStoredMessage(bucket, key string, m *api.StoredMsg, dec func(string) string) (*jsResult, error) {
	res := &jsResult{
		bucket:  bucket,
		key:     key,
		val:     dec(string(m.Data)),
		ts:      m.Time,
		seq:     m.Sequence,
		pending: 0, // we dont know from StoredMsg and we only use this in get last for subject, so 0 is right
	}

	if m.Header != nil || len(m.Header) > 0 {
		hdrs, err := decodeHeadersMsg(m.Header)
		if err != nil {
			return nil, err
		}
		res.ocluster = hdrs.Get("KV-Origin-Cluster")
	}

	return res, nil
}

func jsResultFromMessage(bucket, key string, m *nats.Msg, dec func(string) string) (*jsResult, error) {
	meta, err := jsm.ParseJSMsgMetadata(m)
	if err != nil {
		return nil, err
	}

	return &jsResult{
		bucket:   bucket,
		key:      key,
		val:      dec(string(m.Data)),
		ts:       meta.TimeStamp(),
		seq:      meta.StreamSequence(),
		pending:  meta.Pending(),
		ocluster: m.Header.Get("KV-Origin-Cluster"),
	}, nil
}

const (
	hdrLine   = "NATS/1.0\r\n"
	crlf      = "\r\n"
	hdrPreEnd = len(hdrLine) - len(crlf)
)

func decodeHeadersMsg(data []byte) (http.Header, error) {
	tp := textproto.NewReader(bufio.NewReader(bytes.NewReader(data)))
	if l, err := tp.ReadLine(); err != nil || l != hdrLine[:hdrPreEnd] {
		return nil, fmt.Errorf("could not decode headers")
	}

	mh, err := tp.ReadMIMEHeader()
	if err != nil {
		return nil, fmt.Errorf("could not decode headers")
	}

	return http.Header(mh), nil
}
