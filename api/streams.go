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
	"time"

	"github.com/xeipuuv/gojsonschema"
)

const (
	JSApiStreamCreateT = "$JS.API.STREAM.CREATE.%s"
	JSApiStreamUpdate  = "$JS.API.STREAM.UPDATE.%s"
	JSApiStreamNames   = "$JS.API.STREAM.NAMES"
	JSApiStreamList    = "$JS.API.STREAM.LIST"
	JSApiStreamInfoT   = "$JS.API.STREAM.INFO.%s"
	JSApiStreamDeleteT = "$JS.API.STREAM.DELETE.%s"
	JSApiStreamPurgeT  = "$JS.API.STREAM.PURGE.%s"
	JSApiMsgDeleteT    = "$JS.API.STREAM.MSG.DELETE.%s"
	JSApiMsgGetT       = "$JS.API.STREAM.MSG.GET.%s"
)

type StoredMsg struct {
	Subject  string    `json:"subject"`
	Sequence uint64    `json:"seq"`
	Data     []byte    `json:"data"`
	Time     time.Time `json:"time"`
}

type JSApiStreamNamesResponse struct {
	JSApiResponse
	JSApiIterableResponse
	Streams []string `json:"streams"`
}

type JSApiStreamListResponse struct {
	JSApiResponse
	JSApiIterableResponse
	Streams []*StreamInfo `json:"streams"`
}

type JSApiStreamNamesRequest struct {
	JSApiIterableRequest
}

type JSApiStreamListRequest struct {
	JSApiIterableRequest
}

type JSApiMsgDeleteRequest struct {
	Seq uint64 `json:"seq"`
}

type JSApiMsgDeleteResponse struct {
	JSApiResponse
	Success bool `json:"success,omitempty"`
}

type JSApiStreamCreateResponse struct {
	JSApiResponse
	*StreamInfo
}

type JSApiStreamInfoResponse struct {
	JSApiResponse
	*StreamInfo
}

type JSApiStreamUpdateResponse struct {
	JSApiResponse
	*StreamInfo
}

type JSApiStreamDeleteResponse struct {
	JSApiResponse
	Success bool `json:"success,omitempty"`
}

type JSApiStreamPurgeResponse struct {
	JSApiResponse
	Success bool   `json:"success,omitempty"`
	Purged  uint64 `json:"purged,omitempty"`
}

type JSApiMsgGetResponse struct {
	JSApiResponse
	Message *StoredMsg `json:"message,omitempty"`
}

type JSApiMsgGetRequest struct {
	Seq uint64 `json:"seq"`
}

// StreamConfig is the configuration for a JetStream Stream Template
//
// NATS Schema Type io.nats.jetstream.api.v1.stream_configuration
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
	Discard      DiscardPolicy   `json:"discard"`
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

type StreamState struct {
	Msgs      uint64 `json:"messages"`
	Bytes     uint64 `json:"bytes"`
	FirstSeq  uint64 `json:"first_seq"`
	LastSeq   uint64 `json:"last_seq"`
	Consumers int    `json:"consumer_count"`
}
