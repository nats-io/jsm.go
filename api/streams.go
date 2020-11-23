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
)

const (
	JSApiStreamCreateT   = "$JS.API.STREAM.CREATE.%s"
	JSApiStreamUpdateT   = "$JS.API.STREAM.UPDATE.%s"
	JSApiStreamNames     = "$JS.API.STREAM.NAMES"
	JSApiStreamList      = "$JS.API.STREAM.LIST"
	JSApiStreamLookup    = "$JS.API.STREAM.LOOKUP"
	JSApiStreamInfoT     = "$JS.API.STREAM.INFO.%s"
	JSApiStreamDeleteT   = "$JS.API.STREAM.DELETE.%s"
	JSApiStreamPurgeT    = "$JS.API.STREAM.PURGE.%s"
	JSApiMsgDeleteT      = "$JS.API.STREAM.MSG.DELETE.%s"
	JSApiMsgGetT         = "$JS.API.STREAM.MSG.GET.%s"
	JSApiStreamSnapshotT = "$JS.API.STREAM.SNAPSHOT.%s"
	JSApiStreamRestoreT  = "$JS.API.STREAM.RESTORE.%s"
)

type StoredMsg struct {
	Subject  string    `json:"subject"`
	Sequence uint64    `json:"seq"`
	Header   []byte    `json:"hdrs,omitempty"`
	Data     []byte    `json:"data,omitempty"`
	Time     time.Time `json:"time"`
}

// io.nats.jetstream.api.v1.stream_lookup_request
type JSApiStreamLookupRequest struct {
	Subject string `json:"subject"`
}

// io.nats.jetstream.api.v1.stream_lookup_response
type JSApiStreamLookupResponse struct {
	JSApiResponse
	Stream   string `json:"stream"`
	Filtered bool   `json:"is_filtered"`
}

// io.nats.jetstream.api.v1.stream_create_request
type JSApiStreamCreateRequest struct {
	StreamConfig
}

// io.nats.jetstream.api.v1.stream_names_request
type JSApiStreamNamesRequest struct {
	JSApiIterableRequest
}

// io.nats.jetstream.api.v1.stream_names_response
type JSApiStreamNamesResponse struct {
	JSApiResponse
	JSApiIterableResponse
	Streams []string `json:"streams"`
}

// io.nats.jetstream.api.v1.stream_list_response
type JSApiStreamListResponse struct {
	JSApiResponse
	JSApiIterableResponse
	Streams []*StreamInfo `json:"streams"`
}

// io.nats.jetstream.api.v1.stream_list_request
type JSApiStreamListRequest struct {
	JSApiIterableRequest
}

// io.nats.jetstream.api.v1.stream_msg_delete_request
type JSApiMsgDeleteRequest struct {
	Seq uint64 `json:"seq"`
}

// io.nats.jetstream.api.v1.stream_msg_delete_response
type JSApiMsgDeleteResponse struct {
	JSApiResponse
	Success bool `json:"success,omitempty"`
}

// io.nats.jetstream.api.v1.stream_create_response
type JSApiStreamCreateResponse struct {
	JSApiResponse
	*StreamInfo
}

// io.nats.jetstream.api.v1.stream_info_response
type JSApiStreamInfoResponse struct {
	JSApiResponse
	*StreamInfo
}

// io.nats.jetstream.api.v1.stream_update_response
type JSApiStreamUpdateResponse struct {
	JSApiResponse
	*StreamInfo
}

// io.nats.jetstream.api.v1.stream_delete_response
type JSApiStreamDeleteResponse struct {
	JSApiResponse
	Success bool `json:"success,omitempty"`
}

// io.nats.jetstream.api.v1.stream_purge_response
type JSApiStreamPurgeResponse struct {
	JSApiResponse
	Success bool   `json:"success,omitempty"`
	Purged  uint64 `json:"purged"`
}

// io.nats.jetstream.api.v1.stream_msg_get_response
type JSApiMsgGetResponse struct {
	JSApiResponse
	Message *StoredMsg `json:"message,omitempty"`
}

// io.nats.jetstream.api.v1.stream_msg_get_request
type JSApiMsgGetRequest struct {
	Seq uint64 `json:"seq"`
}

// io.nats.jetstream.api.v1.stream_snapshot_response
type JSApiStreamSnapshotResponse struct {
	JSApiResponse
	// Estimate of number of blocks for the messages.
	NumBlks int `json:"num_blks"`
	// Block size limit as specified by the stream.
	BlkSize int `json:"blk_size"`
}

// io.nats.jetstream.api.v1.stream_snapshot_request
type JSApiStreamSnapshotRequest struct {
	// Subject to deliver the chunks to for the snapshot.
	DeliverSubject string `json:"deliver_subject"`
	// Do not include consumers in the snapshot.
	NoConsumers bool `json:"no_consumers,omitempty"`
	// Optional chunk size preference. Otherwise server selects.
	ChunkSize int `json:"chunk_size,omitempty"`
	// Check all message's checksums prior to snapshot.
	CheckMsgs bool `json:"jsck,omitempty"`
}

// io.nats.jetstream.api.v1.stream_restore_response
type JSApiStreamRestoreResponse struct {
	JSApiResponse
	// Subject to deliver the chunks to for the snapshot restore.
	DeliverSubject string `json:"deliver_subject"`
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
	Duplicates   time.Duration   `json:"duplicate_window,omitempty"`
}

type StreamInfo struct {
	Config StreamConfig `json:"config"`
	State  StreamState  `json:"state"`
}

type StreamState struct {
	Msgs      uint64    `json:"messages"`
	Bytes     uint64    `json:"bytes"`
	FirstSeq  uint64    `json:"first_seq"`
	FirstTime time.Time `json:"first_ts"`
	LastSeq   uint64    `json:"last_seq"`
	LastTime  time.Time `json:"last_ts"`
	Consumers int       `json:"consumer_count"`
}

// PubAck is the detail you get back from a publish to a stream that was successful.
type PubAck struct {
	Stream    string `json:"stream"`
	Seq       uint64 `json:"seq"`
	Duplicate bool   `json:"duplicate,omitempty"`
}
