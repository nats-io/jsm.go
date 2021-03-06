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

type jsEntry struct {
	bucket    string
	key       string
	val       []byte
	ts        time.Time
	seq       uint64
	pending   uint64
	operation Operation
}

func (j *jsEntry) Bucket() string       { return j.bucket }
func (j *jsEntry) Key() string          { return j.key }
func (j *jsEntry) Value() []byte        { return j.val }
func (j *jsEntry) Created() time.Time   { return j.ts }
func (j *jsEntry) Sequence() uint64     { return j.seq }
func (j *jsEntry) Delta() uint64        { return j.pending }
func (j *jsEntry) Operation() Operation { return j.operation }
func (j *jsEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.genericEntry())
}

func (j *jsEntry) genericEntry() *GenericEntry {
	return &GenericEntry{
		Bucket:    j.bucket,
		Key:       j.key,
		Val:       j.val,
		Created:   j.ts.UnixNano(),
		Seq:       j.seq,
		Operation: string(j.operation),
	}
}

func jsEntryFromStoredMessage(bucket, key string, m *api.StoredMsg, dec func([]byte) ([]byte, error)) (*jsEntry, error) {
	res := &jsEntry{
		bucket:    bucket,
		key:       key,
		ts:        m.Time,
		seq:       m.Sequence,
		operation: PutOperation,
		pending:   0, // we dont know from StoredMsg and we only use this in get last for subject, so 0 is right
	}

	var err error
	res.val, err = dec(m.Data)
	if err != nil {
		return nil, err
	}

	if m.Header != nil || len(m.Header) > 0 {
		hdrs, err := decodeHeadersMsg(m.Header)
		if err != nil {
			return nil, err
		}

		if op := hdrs.Get(kvOperationHeader); op == delOperationString {
			res.operation = DeleteOperation
		}
	}

	return res, nil
}

func jsEntryFromMessage(bucket, key string, m *nats.Msg, dec func([]byte) ([]byte, error)) (*jsEntry, error) {
	meta, err := jsm.ParseJSMsgMetadata(m)
	if err != nil {
		return nil, err
	}

	res := &jsEntry{
		bucket:    bucket,
		key:       key,
		ts:        meta.TimeStamp(),
		seq:       meta.StreamSequence(),
		pending:   meta.Pending(),
		operation: PutOperation,
	}

	res.val, err = dec(m.Data)
	if err != nil {
		return nil, err
	}

	if op := m.Header.Get(kvOperationHeader); op != "" {
		if op == delOperationString {
			res.operation = DeleteOperation
		}
	}

	return res, nil
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
