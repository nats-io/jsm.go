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

package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

const (
	JetStreamCreateStreamT = "$JS.STREAM.%s.CREATE"
	JetStreamUpdateStreamT = "$JS.STREAM.%s.UPDATE"
	JetStreamListStreams   = "$JS.STREAM.LIST"
	JetStreamStreamInfoT   = "$JS.STREAM.%s.INFO"
	JetStreamDeleteStreamT = "$JS.STREAM.%s.DELETE"
	JetStreamPurgeStreamT  = "$JS.STREAM.%s.PURGE"
	JetStreamDeleteMsgT    = "$JS.STREAM.%s.MSG.DELETE"
	JetStreamMsgBySeqT     = "$JS.STREAM.%s.MSG.BYSEQ"
)

type StoredMsg struct {
	Subject  string    `json:"subject"`
	Sequence uint64    `json:"seq"`
	Data     []byte    `json:"data"`
	Time     time.Time `json:"time"`
}

type StreamConfig struct {
	Name         string          `json:"name"`
	Subjects     []string        `json:"subjects,omitempty"`
	Retention    RetentionPolicy `json:"retention"`
	MaxConsumers int             `json:"max_consumers"`
	MaxMsgs      int64           `json:"max_msgs"`
	MaxBytes     int64           `json:"max_bytes"`
	MaxAge       time.Duration   `json:"max_age"`
	MaxMsgSize   int32           `json:"max_msg_size,omitempty"`
	Storage      StorageType     `json:"storage"`
	Replicas     int             `json:"num_replicas"`
	NoAck        bool            `json:"no_ack,omitempty"`
	Template     string          `json:"template_owner,omitempty"`
}

// SchemaID is the url to the JSON Schema for JetStream Stream Configuration
func (c StreamConfig) SchemaID() string {
	return "https://nats.io/schemas/jetstream/api/v1/stream_configuration.json"
}

// SchemaType is the NATS schema type like io.nats.jetstream.api.v1.stream_configuration
func (c StreamConfig) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_configuration"
}

// Schema is a Draft 7 JSON Schema for the JetStream Stream Configuration
func (c StreamConfig) Schema() []byte {
	return schemas[c.SchemaType()]
}

func (c StreamConfig) Validate() (bool, []string) {
	sl := gojsonschema.NewSchemaLoader()
	sl.AddSchema("definitions.json", gojsonschema.NewBytesLoader(schemas["io.nats.jetstream.api.v1.definitions"]))

	js, err := sl.Compile(gojsonschema.NewBytesLoader(c.Schema()))
	if err != nil {
		return false, []string{"compile failed", err.Error()}
	}

	doc := gojsonschema.NewGoLoader(c)

	result, err := js.Validate(doc)
	if err != nil {
		return false, []string{err.Error()}
	}

	if result.Valid() {
		return true, nil
	}

	errors := make([]string, len(result.Errors()))
	for i, verr := range result.Errors() {
		errors[i] = verr.String()
	}

	return false, errors
}

type StreamInfo struct {
	Config StreamConfig `json:"config"`
	State  StreamState  `json:"state"`
}

type RetentionPolicy string

func (p RetentionPolicy) String() string { return strings.Title(string(p)) }

const (
	LimitsPolicy    RetentionPolicy = "limits"
	InterestPolicy  RetentionPolicy = "interest"
	WorkQueuePolicy RetentionPolicy = "workqueue"
)

func (p *RetentionPolicy) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case jsonString("limits"):
		*p = LimitsPolicy
	case jsonString("interest"):
		*p = InterestPolicy
	case jsonString("workqueue"):
		*p = WorkQueuePolicy
	default:
		return fmt.Errorf("can not unmarshal %q", data)
	}

	return nil
}

type StreamState struct {
	Msgs      uint64 `json:"messages"`
	Bytes     uint64 `json:"bytes"`
	FirstSeq  uint64 `json:"first_seq"`
	LastSeq   uint64 `json:"last_seq"`
	Consumers int    `json:"consumer_count"`
}
