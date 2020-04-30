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
)

// Subjects used by the JetStream API
const (
	JetStreamEnabled        = "$JS.ENABLED"
	JetStreamMetricPrefix   = "$JS.EVENT.METRIC"
	JetStreamAdvisoryPrefix = "$JS.EVENT.ADVISORY"
	JetStreamInfo           = "$JS.INFO"
)

// Responses to requests sent to a server from a client.
const (
	// OK response
	OK = "+OK"
	// ERR prefix response
	ErrPrefix = "-ERR"
)

type DiscardPolicy int

const (
	DiscardOld DiscardPolicy = iota
	DiscardNew
)

func (p DiscardPolicy) String() string {
	switch p {
	case DiscardOld:
		return "Old"
	case DiscardNew:
		return "New"
	default:
		return "Unknown Discard Policy"
	}
}

func (p *DiscardPolicy) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case jsonString("old"):
		*p = DiscardOld
	case jsonString("new"):
		*p = DiscardNew
	default:
		return fmt.Errorf("can not unmarshal %q", data)
	}

	return nil
}

func (p DiscardPolicy) MarshalJSON() ([]byte, error) {
	switch p {
	case DiscardOld:
		return json.Marshal("old")
	case DiscardNew:
		return json.Marshal("new")
	default:
		return nil, fmt.Errorf("unknown discard policy %v", p)
	}
}

type StorageType int

const (
	MemoryStorage StorageType = iota
	FileStorage
)

func (t StorageType) String() string {
	switch t {
	case MemoryStorage:
		return "Memory"
	case FileStorage:
		return "File"
	default:
		return "Unknown Storage Type"
	}
}

func (t *StorageType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case jsonString("memory"):
		*t = MemoryStorage
	case jsonString("file"):
		*t = FileStorage
	default:
		return fmt.Errorf("can not unmarshal %q", data)
	}

	return nil
}

func (t StorageType) MarshalJSON() ([]byte, error) {
	switch t {
	case MemoryStorage:
		return json.Marshal("memory")
	case FileStorage:
		return json.Marshal("file")
	default:
		return nil, fmt.Errorf("unknown storage type %q", t)
	}
}

type RetentionPolicy int

const (
	LimitsPolicy RetentionPolicy = iota
	InterestPolicy
	WorkQueuePolicy
)

func (p RetentionPolicy) String() string {
	switch p {
	case LimitsPolicy:
		return "Limits"
	case InterestPolicy:
		return "Interest"
	case WorkQueuePolicy:
		return "WorkQueue"
	default:
		return "Unknown Retention Policy"
	}
}

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

func (p RetentionPolicy) MarshalJSON() ([]byte, error) {
	switch p {
	case LimitsPolicy:
		return json.Marshal("limits")
	case InterestPolicy:
		return json.Marshal("interest")
	case WorkQueuePolicy:
		return json.Marshal("workqueue")
	default:
		return nil, fmt.Errorf("unknown retention policy %q", p)
	}
}

type JetStreamAccountStats struct {
	Memory  uint64                 `json:"memory"`
	Store   uint64                 `json:"storage"`
	Streams int                    `json:"streams"`
	Limits  JetStreamAccountLimits `json:"limits"`
}

type JetStreamAccountLimits struct {
	MaxMemory    int64 `json:"max_memory"`
	MaxStore     int64 `json:"max_storage"`
	MaxStreams   int   `json:"max_streams"`
	MaxConsumers int   `json:"max_consumers"`
}
