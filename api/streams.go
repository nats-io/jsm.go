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

	"gopkg.in/yaml.v3"
)

// also update wellKnownSubjectSchemas
const (
	JSAck                           = "$JS.ACK"
	JSAckPrefix                     = "$JS.ACK"
	JSApiAccountPurge               = "$JS.API.ACCOUNT.PURGE.*"
	JSApiAccountPurgePrefix         = "$JS.API.ACCOUNT.PURGE"
	JSApiAccountPurgeT              = "$JS.API.ACCOUNT.PURGE.%s"
	JSApiMsgDelete                  = "$JS.API.STREAM.MSG.DELETE.*"
	JSApiMsgDeletePrefix            = "$JS.API.STREAM.MSG.DELETE"
	JSApiMsgDeleteT                 = "$JS.API.STREAM.MSG.DELETE.%s"
	JSApiMsgGet                     = "$JS.API.STREAM.MSG.GET.*"
	JSApiMsgGetPrefix               = "$JS.API.STREAM.MSG.GET"
	JSApiMsgGetT                    = "$JS.API.STREAM.MSG.GET.%s"
	JSApiServerRemove               = "$JS.API.SERVER.REMOVE"
	JSApiServerRemovePrefix         = "$JS.API.SERVER.REMOVE"
	JSApiServerRemoveT              = "$JS.API.SERVER.REMOVE"
	JSApiStreamCreate               = "$JS.API.STREAM.CREATE.*"
	JSApiStreamCreatePrefix         = "$JS.API.STREAM.CREATE"
	JSApiStreamCreateT              = "$JS.API.STREAM.CREATE.%s"
	JSApiStreamDeletePrefix         = "$JS.API.STREAM.DELETE"
	JSApiStreamDeleteT              = "$JS.API.STREAM.DELETE.%s"
	JSApiStreamInfo                 = "$JS.API.STREAM.INFO.*"
	JSApiStreamInfoPrefix           = "$JS.API.STREAM.INFO"
	JSApiStreamInfoT                = "$JS.API.STREAM.INFO.%s"
	JSApiStreamLeaderStepDown       = "$JS.API.STREAM.LEADER.STEPDOWN.*"
	JSApiStreamLeaderStepDownPrefix = "$JS.API.STREAM.LEADER.STEPDOWN"
	JSApiStreamLeaderStepDownT      = "$JS.API.STREAM.LEADER.STEPDOWN.%s"
	JSApiStreamList                 = "$JS.API.STREAM.LIST"
	JSApiStreamListPrefix           = "$JS.API.STREAM.LIST"
	JSApiStreamListT                = "$JS.API.STREAM.LIST"
	JSApiStreamNames                = "$JS.API.STREAM.NAMES"
	JSApiStreamNamesPrefix          = "$JS.API.STREAM.NAMES"
	JSApiStreamNamesT               = "$JS.API.STREAM.NAMES"
	JSApiStreamPurge                = "$JS.API.STREAM.PURGE.*"
	JSApiStreamPurgePrefix          = "$JS.API.STREAM.PURGE"
	JSApiStreamPurgeT               = "$JS.API.STREAM.PURGE.%s"
	JSApiStreamRemovePeer           = "$JS.API.STREAM.PEER.REMOVE.*"
	JSApiStreamRemovePeerPrefix     = "$JS.API.STREAM.PEER.REMOVE"
	JSApiStreamRemovePeerT          = "$JS.API.STREAM.PEER.REMOVE.%s"
	JSApiStreamRestore              = "$JS.API.STREAM.RESTORE.*"
	JSApiStreamRestorePrefix        = "$JS.API.STREAM.RESTORE"
	JSApiStreamRestoreT             = "$JS.API.STREAM.RESTORE.%s"
	JSApiStreamSnapshot             = "$JS.API.STREAM.SNAPSHOT.*"
	JSApiStreamSnapshotPrefix       = "$JS.API.STREAM.SNAPSHOT"
	JSApiStreamSnapshotT            = "$JS.API.STREAM.SNAPSHOT.%s"
	JSApiStreamUpdate               = "$JS.API.STREAM.UPDATE.*"
	JSApiStreamUpdatePrefix         = "$JS.API.STREAM.UPDATE"
	JSApiStreamUpdateT              = "$JS.API.STREAM.UPDATE.%s"
	JSDirectMsgGet                  = "$JS.API.DIRECT.GET.*"
	JSDirectMsgGetPrefix            = "$JS.API.DIRECT.GET"
	JSDirectMsgGetT                 = "$JS.API.DIRECT.GET.%s"

	StreamDefaultReplicas = 1
	StreamMaxReplicas     = 5
)

type StoredMsg struct {
	Subject  string    `json:"subject"`
	Sequence uint64    `json:"seq"`
	Header   []byte    `json:"hdrs,omitempty"`
	Data     []byte    `json:"data,omitempty"`
	Time     time.Time `json:"time"`
}

// io.nats.jetstream.api.v1.pub_ack_response
type JSPubAckResponse struct {
	Error *ApiError `json:"error,omitempty"`
	PubAck
}

// PubAck is the detail you get back from a publish to a stream that was successful
type PubAck struct {
	Stream    string `json:"stream"`
	Sequence  uint64 `json:"seq"`
	Domain    string `json:"domain,omitempty"`
	Duplicate bool   `json:"duplicate,omitempty"`
	Value     string `json:"val,omitempty"`
	BatchId   string `json:"batch,omitempty"`
	BatchSize int    `json:"count,omitempty"`
}

// io.nats.jetstream.api.v1.stream_info_request
type JSApiStreamInfoRequest struct {
	JSApiIterableRequest
	DeletedDetails bool   `json:"deleted_details,omitempty"`
	SubjectsFilter string `json:"subjects_filter,omitempty"`
}

// io.nats.jetstream.api.v1.stream_create_request
type JSApiStreamCreateRequest struct {
	StreamConfig
	// Pedantic disables server features that would set defaults and adjust the provided config
	Pedantic bool `json:"pedantic,omitempty"`
}

// io.nats.jetstream.api.v1.stream_update_request
type JSApiStreamUpdateRequest struct {
	StreamConfig
	// Pedantic disables server features that would set defaults and adjust the provided config
	Pedantic bool `json:"pedantic,omitempty"`
}

// io.nats.jetstream.api.v1.stream_names_request
type JSApiStreamNamesRequest struct {
	JSApiIterableRequest
	// Subject filter the names to those consuming messages matching this subject or wildcard
	Subject string `json:"subject,omitempty"`
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
	Streams []*StreamInfo     `json:"streams"`
	Missing []string          `json:"missing,omitempty"`
	Offline map[string]string `json:"offline,omitempty"`
}

// io.nats.jetstream.api.v1.stream_list_request
type JSApiStreamListRequest struct {
	JSApiIterableRequest
	// Subject filter the names to those consuming messages matching this subject or wildcard
	Subject string `json:"subject,omitempty"`
}

// io.nats.jetstream.api.v1.stream_msg_delete_request
type JSApiMsgDeleteRequest struct {
	// Seq is the message sequence to delete
	Seq uint64 `json:"seq"`
	// NoErase avoids overwriting the message data with random bytes
	NoErase bool `json:"no_erase,omitempty"`
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
	JSApiIterableResponse
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

// JSApiStreamPurgeRequest is optional request information to the purge API.
// Subject will filter the purge request to only messages that match the subject, which can have wildcards.
// Sequence will purge up to but not including this sequence and can be combined with subject filtering.
// Keep will specify how many messages to keep. This can also be combined with subject filtering.
// Note that Sequence and Keep are mutually exclusive, so both can not be set at the same time.
type JSApiStreamPurgeRequest struct {
	// Purge up to but not including sequence.
	Sequence uint64 `json:"seq,omitempty"`
	// Subject to match against messages for the purge command.
	Subject string `json:"filter,omitempty"`
	// Number of messages to keep.
	Keep uint64 `json:"keep,omitempty"`
}

// io.nats.jetstream.api.v1.stream_msg_get_response
type JSApiMsgGetResponse struct {
	JSApiResponse
	Message *StoredMsg `json:"message,omitempty"`
}

// io.nats.jetstream.api.v1.stream_msg_get_request
type JSApiMsgGetRequest struct {
	Seq     uint64 `json:"seq,omitempty"`
	LastFor string `json:"last_by_subj,omitempty"`
	NextFor string `json:"next_by_subj,omitempty"`

	// Batch support. Used to request more then one msg at a time.
	// Can be used with simple starting seq, but also NextFor with wildcards.
	Batch int `json:"batch,omitempty"`
	// This will make sure we limit how much data we blast out. If not set we will
	// inherit the slow consumer default max setting of the server. Default is MAX_PENDING_SIZE.
	MaxBytes int `json:"max_bytes,omitempty"`
	// Return messages as of this start time.
	StartTime *time.Time `json:"start_time,omitempty"`

	// Multiple response support. Will get the last msgs matching the subjects. These can include wildcards.
	MultiLastFor []string `json:"multi_last,omitempty"`
	// Only return messages up to this sequence. If not set, will be last sequence for the stream.
	UpToSeq uint64 `json:"up_to_seq,omitempty"`
	// Only return messages up to this time.
	UpToTime *time.Time `json:"up_to_time,omitempty"`
}

// io.nats.jetstream.api.v1.stream_snapshot_response
type JSApiStreamSnapshotResponse struct {
	JSApiResponse
	Config StreamConfig `json:"config"`
	State  StreamState  `json:"state"`
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

// io.nats.jetstream.api.v1.stream_restore_request
type JSApiStreamRestoreRequest struct {
	Config StreamConfig `json:"config"`
	State  StreamState  `json:"state"`
}

// io.nats.jetstream.api.v1.stream_restore_response
type JSApiStreamRestoreResponse struct {
	JSApiResponse
	// Subject to deliver the chunks to for the snapshot restore.
	DeliverSubject string `json:"deliver_subject"`
}

// io.nats.jetstream.api.v1.stream_remove_peer_request
type JSApiStreamRemovePeerRequest struct {
	// Server name of the peer to be removed.
	Peer string `json:"peer"`
}

// io.nats.jetstream.api.v1.stream_remove_peer_response
type JSApiStreamRemovePeerResponse struct {
	JSApiResponse
	Success bool `json:"success,omitempty"`
}

// io.nats.jetstream.api.v1.stream_leader_stepdown_request
type JSApiStreamLeaderStepDownRequest struct {
	Placement *Placement `json:"placement,omitempty"`
}

// io.nats.jetstream.api.v1.stream_leader_stepdown_response
type JSApiStreamLeaderStepDownResponse struct {
	JSApiResponse
	Success bool `json:"success,omitempty"`
}

type PersistModeType int

const (
	// DefaultPersistMode specifies the default persist mode. Writes to the stream will immediately be flushed.
	// The publish acknowledgement will be sent after the persisting completes.
	DefaultPersistMode = PersistModeType(iota)
	// AsyncPersistMode specifies writes to the stream will be flushed asynchronously.
	// The publish acknowledgement may be sent before the persisting completes.
	// This means writes could be lost if they weren't flushed prior to a hard kill of the server.
	AsyncPersistMode
)

func (wc PersistModeType) String() string {
	switch wc {
	case DefaultPersistMode:
		return "Default"
	case AsyncPersistMode:
		return "Async"
	default:
		return "Unknown Persist Mode Type"
	}
}

func (wc PersistModeType) MarshalJSON() ([]byte, error) {
	switch wc {
	case DefaultPersistMode:
		return json.Marshal("default")
	case AsyncPersistMode:
		return json.Marshal("async")
	default:
		return nil, fmt.Errorf("can not marshal %v", wc)
	}
}

func (wc PersistModeType) MarshalYAML() (any, error) {
	switch wc {
	case DefaultPersistMode:
		return "default", nil
	case AsyncPersistMode:
		return "async", nil
	default:
		return nil, fmt.Errorf("can not marshal %v", wc)
	}
}

func (wc *PersistModeType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case jsonString("default"), jsonString(""):
		*wc = DefaultPersistMode
	case jsonString("async"):
		*wc = AsyncPersistMode
	default:
		return fmt.Errorf("can not unmarshal %q", data)
	}

	return nil
}

func (wc *PersistModeType) UnmarshalYAML(data *yaml.Node) error {
	switch data.Value {
	case "", "default":
		*wc = DefaultPersistMode
	case "async":
		*wc = AsyncPersistMode
	default:
		return fmt.Errorf("can not unmarshal %q", data.Value)
	}

	return nil
}

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

func (p *DiscardPolicy) UnmarshalYAML(data *yaml.Node) error {
	switch data.Value {
	case "old":
		*p = DiscardOld
	case "new":
		*p = DiscardNew
	default:
		return fmt.Errorf("can not unmarshal %q", data.Value)
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

func (p DiscardPolicy) MarshalYAML() (any, error) {
	switch p {
	case DiscardOld:
		return "old", nil
	case DiscardNew:
		return "new", nil
	default:
		return nil, fmt.Errorf("unknown discard policy %v", p)
	}
}

type StorageType int

const (
	FileStorage StorageType = iota
	MemoryStorage
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

func (t *StorageType) UnmarshalYAML(data *yaml.Node) error {
	switch data.Value {
	case "memory":
		*t = MemoryStorage
	case "file":
		*t = FileStorage
	default:
		return fmt.Errorf("can not unmarshal %q", data.Value)
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

func (t StorageType) MarshalYAML() (any, error) {
	switch t {
	case MemoryStorage:
		return "memory", nil
	case FileStorage:
		return "file", nil
	default:
		return "", nil
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

func (p *RetentionPolicy) UnmarshalYAML(data *yaml.Node) error {
	switch data.Value {
	case "limits":
		*p = LimitsPolicy
	case "interest":
		*p = InterestPolicy
	case "workqueue":
		*p = WorkQueuePolicy
	default:
		return fmt.Errorf("can not unmarshal %q", data.Value)
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

func (p RetentionPolicy) MarshalYAML() (any, error) {
	switch p {
	case LimitsPolicy:
		return "limits", nil
	case InterestPolicy:
		return "interest", nil
	case WorkQueuePolicy:
		return "workqueue", nil
	default:
		return nil, fmt.Errorf("unknown retention policy %q", p)
	}
}

type Compression int

const (
	NoCompression Compression = iota
	S2Compression
)

func (p Compression) String() string {
	switch p {
	case NoCompression:
		return "None"
	case S2Compression:
		return "S2 Compression"
	default:
		return "Unknown Compression"
	}
}

func (p *Compression) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case jsonString("none"), jsonString(""):
		*p = NoCompression
	case jsonString("s2"):
		*p = S2Compression
	default:
		return fmt.Errorf("can not unmarshal %q", data)
	}

	return nil
}

func (p *Compression) UnmarshalYAML(data *yaml.Node) error {
	switch data.Value {
	case "none", "":
		*p = NoCompression
	case "s2":
		*p = S2Compression
	default:
		return fmt.Errorf("can not unmarshal %q", data.Value)
	}

	return nil
}

func (p Compression) MarshalJSON() ([]byte, error) {
	switch p {
	case NoCompression:
		return json.Marshal("none")
	case S2Compression:
		return json.Marshal("s2")
	default:
		return nil, fmt.Errorf("unknown compression %q", p)
	}
}

func (p Compression) MarshalYAML() (any, error) {
	switch p {
	case NoCompression:
		return "none", nil
	case S2Compression:
		return "s2", nil
	default:
		return nil, fmt.Errorf("unknown compression %q", p)
	}
}

// StreamConfig is the configuration for a JetStream Stream Template
//
// NATS Schema Type io.nats.jetstream.api.v1.stream_configuration
type StreamConfig struct {
	// A unique name for the string, cannot contain dots, spaces or wildcard characters
	Name         string          `json:"name" yaml:"name"`
	Description  string          `json:"description,omitempty" yaml:"description"`
	Subjects     []string        `json:"subjects,omitempty" yaml:"subjects"`
	Retention    RetentionPolicy `json:"retention" yaml:"retention"`
	MaxConsumers int             `json:"max_consumers" yaml:"max_consumers"`
	MaxMsgsPer   int64           `json:"max_msgs_per_subject" yaml:"max_msgs_per_subject"`
	MaxMsgs      int64           `json:"max_msgs" yaml:"max_msgs"`
	MaxBytes     int64           `json:"max_bytes" yaml:"max_bytes"`
	MaxAge       time.Duration   `json:"max_age" yaml:"max_age"`
	MaxMsgSize   int32           `json:"max_msg_size,omitempty" yaml:"max_msg_size"`
	Storage      StorageType     `json:"storage" yaml:"storage"`
	Discard      DiscardPolicy   `json:"discard" yaml:"discard"`
	Replicas     int             `json:"num_replicas" yaml:"num_replicas"`
	NoAck        bool            `json:"no_ack,omitempty" yaml:"no_ack"`
	Template     string          `json:"template_owner,omitempty" yaml:"-"`
	Duplicates   time.Duration   `json:"duplicate_window,omitempty" yaml:"duplicate_window"`
	Placement    *Placement      `json:"placement,omitempty" yaml:"placement"`
	Mirror       *StreamSource   `json:"mirror,omitempty" yaml:"mirror"`
	Sources      []*StreamSource `json:"sources,omitempty" yaml:"sources"`
	Compression  Compression     `json:"compression,omitempty" yaml:"compression"`
	// Allow applying a subject transform to incoming messages before doing anything else
	SubjectTransform *SubjectTransformConfig `json:"subject_transform,omitempty" yaml:"subject_transform"`
	// Allow republish of the message after being sequenced and stored.
	RePublish *RePublish `json:"republish,omitempty" yaml:"republish"`
	// Sealed will seal a stream so no messages can get out or in.
	Sealed bool `json:"sealed" yaml:"sealed"`
	// DenyDelete will restrict the ability to delete messages.
	DenyDelete bool `json:"deny_delete" yaml:"deny_delete"`
	// DenyPurge will restrict the ability to purge messages.
	DenyPurge bool `json:"deny_purge" yaml:"deny_purge"`
	// AllowRollup allows messages to be placed into the system and purge
	// all older messages using a special msg header.
	RollupAllowed bool `json:"allow_rollup_hdrs" yaml:"allow_rollup_hdrs"`
	// Allow higher performance, direct access to get individual messages.
	AllowDirect bool `json:"allow_direct" yaml:"allow_direct"`
	// Allow higher performance and unified direct access for mirrors as well.
	MirrorDirect bool `json:"mirror_direct" yaml:"mirror_direct"`
	// Allow KV like semantics to also discard new on a per subject basis
	DiscardNewPer bool `json:"discard_new_per_subject,omitempty" yaml:"discard_new_per_subject"`
	// FirstSeq sets a custom starting position for the stream
	FirstSeq uint64 `json:"first_seq,omitempty" yaml:"first_seq"`
	// Metadata is additional metadata for the Consumer.
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata"`
	// AllowMsgTTL allows header initiated per-message TTLs. If disabled,
	// then the `NATS-TTL` header will be ignored.
	AllowMsgTTL bool `json:"allow_msg_ttl,omitempty" yaml:"allow_msg_ttl" api_level:"1"`
	// SubjectDeleteMarkerTTL enables and sets a duration for adding server markers for delete, purge and max age limits
	SubjectDeleteMarkerTTL time.Duration `json:"subject_delete_marker_ttl,omitempty" yaml:"subject_delete_marker_ttl" api_level:"1"`
	// The following defaults will apply to consumers when created against
	// this stream, unless overridden manually. They also represent the maximum values that
	// these properties may have
	ConsumerLimits StreamConsumerLimits `json:"consumer_limits" yaml:"consumer_limits"`
	// AllowAtomicPublish allows atomic batch publishing into the stream.
	AllowAtomicPublish bool `json:"allow_atomic,omitempty" yaml:"allow_atomic" api_level:"2"`
	// AllowMsgCounter allows a stream to use (only) counter CRDTs.
	AllowMsgCounter bool `json:"allow_msg_counter,omitempty" yaml:"allow_msg_counter" api_level:"2"`
	// AllowMsgSchedules allows the scheduling of messages.
	AllowMsgSchedules bool `json:"allow_msg_schedules,omitempty" yaml:"allow_msg_schedules" api_level:"2"`
	// PersistMode allows to opt-in to different persistence mode settings.
	PersistMode PersistModeType `json:"persist_mode,omitempty" yaml:"persist_mode" api_level:"2"`
}

// StreamConsumerLimits describes limits and defaults for consumers created on a stream
type StreamConsumerLimits struct {
	InactiveThreshold time.Duration `json:"inactive_threshold,omitempty" yaml:"inactive_threshold"`
	MaxAckPending     int           `json:"max_ack_pending,omitempty" yaml:"max_ack_pending"`
}

// Placement describes stream placement requirements for a stream or leader
type Placement struct {
	Cluster   string   `json:"cluster,omitempty" yaml:"cluster"`
	Tags      []string `json:"tags,omitempty" yaml:"tags"`
	Preferred string   `json:"preferred,omitempty" yaml:"preferred"`
}

// StreamSourceInfo shows information about an upstream stream source.
type StreamSourceInfo struct {
	Name              string                   `json:"name" yaml:"name"`
	External          *ExternalStream          `json:"external,omitempty" yaml:"external"`
	Lag               uint64                   `json:"lag" yaml:"lag"`
	Active            time.Duration            `json:"active" yaml:"active"`
	Error             *ApiError                `json:"error,omitempty" yaml:"error"`
	FilterSubject     string                   `json:"filter_subject,omitempty" yaml:"filter_subject"`
	SubjectTransforms []SubjectTransformConfig `json:"subject_transforms,omitempty" yaml:"subject_transforms"`
}

// LostStreamData indicates msgs that have been lost during file checks and recover due to corruption
type LostStreamData struct {
	// Message IDs of lost messages
	Msgs []uint64 `json:"msgs" yaml:"msgs"`
	// How many bytes were lost
	Bytes uint64 `json:"bytes" yaml:"bytes"`
}

// StreamSource dictates how streams can source from other streams.
type StreamSource struct {
	Name              string                   `json:"name" yaml:"name"`
	OptStartSeq       uint64                   `json:"opt_start_seq,omitempty" yaml:"opt_start_seq"`
	OptStartTime      *time.Time               `json:"opt_start_time,omitempty" yaml:"opt_start_time"`
	FilterSubject     string                   `json:"filter_subject,omitempty" yaml:"filter_subject"`
	External          *ExternalStream          `json:"external,omitempty" yaml:"external"`
	SubjectTransforms []SubjectTransformConfig `json:"subject_transforms,omitempty" yaml:"subject_transforms"`
}

// ExternalStream allows you to qualify access to a stream source in another account.
type ExternalStream struct {
	ApiPrefix     string `json:"api" yaml:"api"`
	DeliverPrefix string `json:"deliver" yaml:"deliver"`
}

type StreamInfo struct {
	Config     StreamConfig        `json:"config" yaml:"config"`
	Created    time.Time           `json:"created" yaml:"created"`
	State      StreamState         `json:"state" yaml:"state"`
	Cluster    *ClusterInfo        `json:"cluster,omitempty" yaml:"cluster"`
	Mirror     *StreamSourceInfo   `json:"mirror,omitempty" yaml:"mirror"`
	Sources    []*StreamSourceInfo `json:"sources,omitempty" yaml:"sources"`
	Alternates []StreamAlternate   `json:"alternates,omitempty" yaml:"alternates"`
	TimeStamp  time.Time           `json:"ts" yaml:"ts"`
}

type StreamAlternate struct {
	Name    string `json:"name" yaml:"name"`
	Domain  string `json:"domain,omitempty" yaml:"domain"`
	Cluster string `json:"cluster" yaml:"cluster"`
}

type StreamState struct {
	Msgs        uint64            `json:"messages" yaml:"messages"`
	Bytes       uint64            `json:"bytes" yaml:"bytes"`
	FirstSeq    uint64            `json:"first_seq" yaml:"first_seq"`
	FirstTime   time.Time         `json:"first_ts" yaml:"first_ts"`
	LastSeq     uint64            `json:"last_seq" yaml:"last_seq"`
	LastTime    time.Time         `json:"last_ts" yaml:"last_ts"`
	NumDeleted  int               `json:"num_deleted,omitempty" yaml:"num_deleted"`
	Deleted     []uint64          `json:"deleted,omitempty" yaml:"deleted"`
	NumSubjects int               `json:"num_subjects,omitempty" yaml:"num_subjects"`
	Subjects    map[string]uint64 `json:"subjects,omitempty" yaml:"subjects"`
	Lost        *LostStreamData   `json:"lost,omitempty" yaml:"lost"`
	Consumers   int               `json:"consumer_count" yaml:"consumer_count"`
}

// SubjectTransformConfig is for applying a subject transform (to matching messages) before doing anything else
// when a new message is received
type SubjectTransformConfig struct {
	Source      string `json:"src" yaml:"src"`
	Destination string `json:"dest" yaml:"dest"`
}

// RePublish allows a source subject to be mapped to a destination subject for republishing.
type RePublish struct {
	Source      string `json:"src,omitempty" yaml:"src"`
	Destination string `json:"dest" yaml:"dest"`
	HeadersOnly bool   `json:"headers_only,omitempty" yaml:"headers_only"`
}
