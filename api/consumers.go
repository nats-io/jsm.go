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
	"strings"
	"time"
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

type AckPolicy string

func (p AckPolicy) String() string { return strings.Title(string(p)) }

const (
	AckNone     AckPolicy = "none"
	AckAll      AckPolicy = "all"
	AckExplicit AckPolicy = "explicit"
)

type ReplayPolicy string

func (p ReplayPolicy) String() string { return strings.Title(string(p)) }

const (
	ReplayInstant  ReplayPolicy = "instant"
	ReplayOriginal ReplayPolicy = "original"
)

var (
	AckAck      = []byte(OK)
	AckNak      = []byte("-NAK")
	AckProgress = []byte("+WPI")
	AckNext     = []byte("+NXT")
)

type ConsumerConfig struct {
	Delivery        string        `json:"delivery_subject"`
	Durable         string        `json:"durable_name,omitempty"`
	StreamSeq       uint64        `json:"stream_seq,omitempty"`
	StartTime       time.Time     `json:"start_time,omitempty"`
	DeliverAll      bool          `json:"deliver_all,omitempty"`
	DeliverLast     bool          `json:"deliver_last,omitempty"`
	AckPolicy       AckPolicy     `json:"ack_policy"`
	AckWait         time.Duration `json:"ack_wait,omitempty"`
	MaxDeliver      int           `json:"max_deliver,omitempty"`
	FilterSubject   string        `json:"filter_subject,omitempty"`
	ReplayPolicy    ReplayPolicy  `json:"replay_policy"`
	SampleFrequency string        `json:"sample_freq,omitempty"`
}

type CreateConsumerRequest struct {
	Stream string         `json:"stream_name"`
	Config ConsumerConfig `json:"config"`
}

type ConsumerState struct {
	Delivered   SequencePair      `json:"delivered"`
	AckFloor    SequencePair      `json:"ack_floor"`
	Pending     map[uint64]int64  `json:"pending"`
	Redelivered map[uint64]uint64 `json:"redelivered"`
}

type SequencePair struct {
	ConsumerSeq uint64 `json:"consumer_seq"`
	StreamSeq   uint64 `json:"stream_seq"`
}

type ConsumerInfo struct {
	Stream string         `json:"stream_name"`
	Name   string         `json:"name"`
	Config ConsumerConfig `json:"config"`
	State  ConsumerState  `json:"state"`
}
