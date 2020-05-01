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
	"encoding/json"
	"fmt"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

const (
	JetStreamCreateConsumerT          = "$JS.STREAM.%s.CONSUMER.%s.CREATE"
	JetStreamCreateEphemeralConsumerT = "$JS.STREAM.%s.EPHEMERAL.CONSUMER.CREATE"
	JetStreamConsumersT               = "$JS.STREAM.%s.CONSUMERS"
	JetStreamConsumerInfoT            = "$JS.STREAM.%s.CONSUMER.%s.INFO"
	JetStreamDeleteConsumerT          = "$JS.STREAM.%s.CONSUMER.%s.DELETE"
	JetStreamRequestNextT             = "$JS.STREAM.%s.CONSUMER.%s.NEXT"
	JetStreamMetricConsumerAckPre     = JetStreamMetricPrefix + ".CONSUMER_ACK"
)

type JetStreamDeleteConsumerResponse struct {
	Error   *ApiError `json:"error,omitempty"`
	Success bool      `json:"success,omitempty"`
}

type JetStreamCreateConsumerResponse struct {
	JetStreamResponse
	*ConsumerInfo
}

type JetStreamConsumerInfoResponse struct {
	JetStreamResponse
	*ConsumerInfo
}

type JetStreamConsumersResponse struct {
	JetStreamResponse
	Consumers []string `json:"streams,omitempty"`
}

type AckPolicy int

const (
	AckNone AckPolicy = iota
	AckAll
	AckExplicit
)

func (p AckPolicy) String() string {
	switch p {
	case AckNone:
		return "None"
	case AckAll:
		return "All"
	case AckExplicit:
		return "Explicit"
	default:
		return "Unknown Acknowledgement Policy"
	}
}

func (p *AckPolicy) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case jsonString("none"):
		*p = AckNone
	case jsonString("all"):
		*p = AckAll
	case jsonString("explicit"):
		*p = AckExplicit
	default:
		return fmt.Errorf("can not unmarshal %q", data)
	}

	return nil
}

func (p AckPolicy) MarshalJSON() ([]byte, error) {
	switch p {
	case AckNone:
		return json.Marshal("none")
	case AckAll:
		return json.Marshal("all")
	case AckExplicit:
		return json.Marshal("explicit")
	default:
		return nil, fmt.Errorf("unknown acknowlegement policy %v", p)
	}
}

type ReplayPolicy int

const (
	ReplayInstant ReplayPolicy = iota
	ReplayOriginal
)

func (p ReplayPolicy) String() string {
	switch p {
	case ReplayInstant:
		return "Instant"
	case ReplayOriginal:
		return "Original"
	default:
		return "Unknown Replay Policy"
	}
}

func (p *ReplayPolicy) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case jsonString("instant"):
		*p = ReplayInstant
	case jsonString("original"):
		*p = ReplayOriginal
	default:
		return fmt.Errorf("can not unmarshal %q", data)
	}

	return nil
}

func (p ReplayPolicy) MarshalJSON() ([]byte, error) {
	switch p {
	case ReplayOriginal:
		return json.Marshal("original")
	case ReplayInstant:
		return json.Marshal("instant")
	default:
		return nil, fmt.Errorf("unknown replay policy %v", p)
	}
}

var (
	AckAck      = []byte("+ACK")
	AckNak      = []byte("-NAK")
	AckProgress = []byte("+WPI")
	AckNext     = []byte("+NXT")
)

type DeliverPolicy int

const (
	DeliverAll DeliverPolicy = iota
	DeliverLast
	DeliverNew
	DeliverByStartSequence
	DeliverByStartTime
)

func (p DeliverPolicy) String() string {
	switch p {
	case DeliverAll:
		return "All"
	case DeliverLast:
		return "Last"
	case DeliverNew:
		return "New"
	case DeliverByStartSequence:
		return "By Start Sequence"
	case DeliverByStartTime:
		return "By Start Time"
	default:
		return "Unknown Deliver Policy"
	}
}

func (p *DeliverPolicy) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case jsonString("all"), jsonString("undefined"):
		*p = DeliverAll
	case jsonString("last"):
		*p = DeliverLast
	case jsonString("new"):
		*p = DeliverNew
	case jsonString("by_start_sequence"):
		*p = DeliverByStartSequence
	case jsonString("by_start_time"):
		*p = DeliverByStartTime
	}

	return nil
}

func (p DeliverPolicy) MarshalJSON() ([]byte, error) {
	switch p {
	case DeliverAll:
		return json.Marshal("all")
	case DeliverLast:
		return json.Marshal("last")
	case DeliverNew:
		return json.Marshal("new")
	case DeliverByStartSequence:
		return json.Marshal("by_start_sequence")
	case DeliverByStartTime:
		return json.Marshal("by_start_time")
	default:
		return nil, fmt.Errorf("unknown deliver policy %v", p)
	}
}

// ConsumerConfig is the configuration for a JetStream consumes
//
// NATS Schema Type io.nats.jetstream.api.v1.consumer_configuration
type ConsumerConfig struct {
	Durable         string        `json:"durable_name,omitempty"`
	DeliverSubject  string        `json:"deliver_subject,omitempty"`
	DeliverPolicy   DeliverPolicy `json:"deliver_policy"`
	OptStartSeq     uint64        `json:"opt_start_seq,omitempty"`
	OptStartTime    *time.Time    `json:"opt_start_time,omitempty"`
	AckPolicy       AckPolicy     `json:"ack_policy"`
	AckWait         time.Duration `json:"ack_wait,omitempty"`
	MaxDeliver      int           `json:"max_deliver,omitempty"`
	FilterSubject   string        `json:"filter_subject,omitempty"`
	ReplayPolicy    ReplayPolicy  `json:"replay_policy"`
	SampleFrequency string        `json:"sample_freq,omitempty"`
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (c ConsumerConfig) SchemaID() string {
	return "https://nats.io/schemas/jetstream/api/v1/consumer_configuration.json"
}

// SchemaType is the NATS schema type like io.nats.jetstream.api.v1.stream_configuration
func (c ConsumerConfig) SchemaType() string {
	return "io.nats.jetstream.api.v1.consumer_configuration"
}

// Schema is a Draft 7 JSON Schema for the JetStream Consumer Configuration
func (c ConsumerConfig) Schema() []byte {
	return schemas[c.SchemaType()]
}

func (c ConsumerConfig) Validate() (bool, []string) {
	sl := gojsonschema.NewSchemaLoader()
	sl.AddSchema("https://nats.io/schemas/jetstream/api/v1/definitions.json", gojsonschema.NewBytesLoader(schemas["io.nats.jetstream.api.v1.definitions"]))
	root := gojsonschema.NewBytesLoader(c.Schema())

	js, err := sl.Compile(root)
	if err != nil {
		return false, []string{err.Error()}
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

type CreateConsumerRequest struct {
	Stream string         `json:"stream_name"`
	Config ConsumerConfig `json:"config"`
}

type SequencePair struct {
	ConsumerSeq uint64 `json:"consumer_seq"`
	StreamSeq   uint64 `json:"stream_seq"`
}

type ConsumerInfo struct {
	Stream         string         `json:"stream_name"`
	Name           string         `json:"name"`
	Config         ConsumerConfig `json:"config"`
	Delivered      SequencePair   `json:"delivered"`
	AckFloor       SequencePair   `json:"ack_floor"`
	NumPending     int            `json:"num_pending"`
	NumRedelivered int            `json:"num_redelivered"`
}

func jsonString(s string) string {
	return "\"" + s + "\""
}
