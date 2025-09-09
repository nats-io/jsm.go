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
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// Subjects used by the JetStream API
const (
	JSAuditAdvisory        = "$JS.EVENT.ADVISORY.API"
	JSMetricPrefix         = "$JS.EVENT.METRIC"
	JSAdvisoryPrefix       = "$JS.EVENT.ADVISORY"
	JSApiAccountInfo       = "$JS.API.INFO"
	JSApiAccountInfoPrefix = "$JS.API.INFO"

	// also update FilterServerMetadata when this changes

	JSMetaCurrentServerLevel   = "_nats.level"
	JSMetaCurrentServerVersion = "_nats.ver"
	JsMetaRequiredServerLevel  = "_nats.req.level"
)

// Responses to requests sent to a server from a client.
const (
	// OK response
	OK = "+OK"
	// ErrPrefix is the ERR prefix response
	ErrPrefix = "-ERR"
)

// Headers for publishing
const (
	// JSMsgId used for tracking duplicates
	JSMsgId = "Nats-Msg-Id"

	// JSExpectedStream only store the message in this stream
	JSExpectedStream = "Nats-Expected-Stream"

	// JSExpectedLastSeq only store the message if stream last sequence matched
	JSExpectedLastSeq = "Nats-Expected-Last-Sequence"

	// JSExpectedLastSubjSeq only stores the message if last sequence for this subject matched
	JSExpectedLastSubjSeq = "Nats-Expected-Last-Subject-Sequence"

	// JSExpectedLastMsgId only stores the message if previous Nats-Msg-Id header value matches this
	JSExpectedLastMsgId = "Nats-Expected-Last-Msg-Id"

	// JSRollup is a header indicating the message being sent should be stored and all past messags should be discarded
	// the value can be either `all` or `sub`
	JSRollup = "Nats-Rollup"

	// JSRollupAll is the value for JSRollup header to replace the entire stream
	JSRollupAll = "all"

	// JSRollupSubject is the value for JSRollup header to replace the a single subject
	JSRollupSubject = "sub"

	// JSMessageTTL sets a TTL per message
	JSMessageTTL = "Nats-TTL"

	// JSSchedulePattern holds a message schedule pattern
	JSSchedulePattern = "Nats-Schedule"

	// JSScheduleTTL sets a TTL on the produced message
	JSScheduleTTL = "Nats-Schedule-TTL"

	// JSScheduleTarget sets the target subject for the produced message
	JSScheduleTarget = "Nats-Schedule-Target"

	// JSRequiredApiLevel indicates that a request requires a certain API level
	JSRequiredApiLevel = "Nats-Required-Api-Level"
)

type JSApiIterableRequest struct {
	Offset int `json:"offset"`
}

func (i *JSApiIterableRequest) SetOffset(o int) { i.Offset = o }

type JSApiIterableResponse struct {
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

func (i JSApiIterableResponse) ItemsTotal() int  { return i.Total }
func (i JSApiIterableResponse) ItemsOffset() int { return i.Offset }
func (i JSApiIterableResponse) ItemsLimit() int  { return i.Limit }
func (i JSApiIterableResponse) LastPage() bool {
	// allow the total from the server to be overridden in cases where a
	// server bug would report an incorrect total
	//
	// deliberately hard to discover as this is a not something we want
	// users to do generally and just need it in some sticky situations
	ts := os.Getenv("PAGE_TOTAL")
	if ts != "" {
		total, err := strconv.Atoi(ts)
		if err == nil {
			i.Total = total
		}
	}

	return i.Offset+i.Limit >= i.Total
}

// io.nats.jetstream.api.v1.account_info_response
type JSApiAccountInfoResponse struct {
	JSApiResponse
	*JetStreamAccountStats
}

type JetStreamTier struct {
	Memory         uint64                 `json:"memory"`
	Store          uint64                 `json:"storage"`
	ReservedMemory uint64                 `json:"reserved_memory"`
	ReservedStore  uint64                 `json:"reserved_storage"`
	Streams        int                    `json:"streams"`
	Consumers      int                    `json:"consumers"`
	Limits         JetStreamAccountLimits `json:"limits"`
}

// JetStreamAccountStats returns current statistics about the account's JetStream usage.
type JetStreamAccountStats struct {
	JetStreamTier                          // in case tiers are used, reflects totals with limits not set
	Domain        string                   `json:"domain,omitempty"`
	API           JetStreamAPIStats        `json:"api"`
	Tiers         map[string]JetStreamTier `json:"tiers,omitempty"` // indexed by tier name
}

type JetStreamAPIStats struct {
	Level    int    `json:"level"`
	Total    uint64 `json:"total"`
	Errors   uint64 `json:"errors"`
	Inflight uint64 `json:"inflight,omitempty"`
}

type JetStreamAccountLimits struct {
	MaxMemory            int64 `json:"max_memory"`
	MaxStore             int64 `json:"max_storage"`
	MaxStreams           int   `json:"max_streams"`
	MaxConsumers         int   `json:"max_consumers"`
	MaxAckPending        int   `json:"max_ack_pending"`
	MemoryMaxStreamBytes int64 `json:"memory_max_stream_bytes"`
	StoreMaxStreamBytes  int64 `json:"storage_max_stream_bytes"`
	MaxBytesRequired     bool  `json:"max_bytes_required"`
}

type ApiError struct {
	Code        int    `json:"code"`
	ErrCode     uint16 `json:"err_code,omitempty"`
	Description string `json:"description,omitempty"`
}

// Error implements error
func (e ApiError) Error() string {
	switch {
	case e.Description == "" && e.Code == 0:
		return "unknown JetStream Error"
	case e.Description == "" && e.Code > 0:
		return fmt.Sprintf("unknown JetStream %d Error (%d)", e.Code, e.ErrCode)
	default:
		return fmt.Sprintf("%s (%d)", e.Description, e.ErrCode)
	}
}

// NotFoundError is true when the error is one about a resource not found
func (e ApiError) NotFoundError() bool { return e.Code == 404 }

// ServerError is true when the server returns a 5xx error code
func (e ApiError) ServerError() bool { return e.Code >= 500 && e.Code < 600 }

// UserError is true when the server returns a 4xx error code
func (e ApiError) UserError() bool { return e.Code >= 400 && e.Code < 500 }

// ErrorCode is the JetStream error code
func (e ApiError) ErrorCode() int { return e.Code }

// NatsErrorCode is the unique nats error code, see `nats errors` command
func (e ApiError) NatsErrorCode() uint16 { return e.ErrCode }

// IsNatsError checks if err is a ApiErr matching code
func IsNatsError(err error, code uint16) bool {
	var aep *ApiError
	if errors.As(err, &aep) {
		return aep.NatsErrorCode() == code
	}

	var ae ApiError
	if errors.As(err, &ae) {
		return ae.NatsErrorCode() == code
	}

	return false
}

type JSApiResponse struct {
	Type  string    `json:"type"`
	Error *ApiError `json:"error,omitempty"`
}

// ToError extracts a standard error from a JetStream response
func (r JSApiResponse) ToError() error {
	if r.Error == nil {
		return nil
	}

	return *r.Error
}

// IsError determines if a standard JetStream API response is a error
func (r JSApiResponse) IsError() bool {
	return r.Error != nil
}

// IsNatsErr determines if a error matches ID, if multiple IDs are given if the error matches any of these the function will be true
func IsNatsErr(err error, ids ...uint16) bool {
	if err == nil {
		return false
	}

	ce, ok := err.(ApiError)
	if !ok {
		return false
	}

	for _, id := range ids {
		if ce.ErrCode == id {
			return true
		}
	}

	return false
}

// ApiLevelAware is an interface that can be implemented by a struct to indicate that it requires a specific JetStream API level.
type ApiLevelAware interface {
	RequiredApiLevel() (int, error)
}

// RequiredApiLevel determines the JetStream API level required by a struct, typically a JetStream API Request
// when a structure implement the ApiLevelAware interface that function will be called instead
func RequiredApiLevel(req any) (int, error) {
	return requiredApiLevel(req, false)
}

// determines the api level from struct tags.
//
// it supports calling the struct RequiredApiLevel() function if present unless skip is given
//
// the idea here is that for fields that are new we would mark them up with the api level
// and then we can use this function to determine the required level for a request
//
// for cases where we introduce a change of behavior on a field structs can handle that in their RequiredApiLevel()
// and then call into this function with skip true to determin the level for the rest of the struct, see
// ConsumerConfig for an example of this.
func requiredApiLevel(req any, skip bool) (int, error) {
	val := reflect.ValueOf(req)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if !val.IsValid() {
		return 0, nil
	}

	checker, ok := val.Interface().(ApiLevelAware)
	if !skip && ok {
		return checker.RequiredApiLevel()
	}

	// we only check structs
	if val.Kind() != reflect.Struct {
		return 0, nil
	}

	maxLevel := 0

	for i := 0; i < val.NumField(); i++ {
		typeField := val.Type().Field(i)
		valueField := val.Field(i)

		// zero generally means unset so we skip it
		if valueField.IsZero() {
			continue
		}

		// if its a struct we recurse into it and check all its fields
		if valueField.Kind() == reflect.Struct {
			lvl, err := requiredApiLevel(valueField.Interface(), skip)
			if err != nil {
				return 0, err
			}
			if maxLevel < lvl {
				maxLevel = lvl
			}

			continue
		}

		apiLevel := strings.TrimSpace(typeField.Tag.Get("api_level"))
		if apiLevel == "" {
			continue
		}

		lvl, err := strconv.Atoi(apiLevel)
		if err != nil {
			return 0, fmt.Errorf("invalid api_level tag %q for field %s on type %s", apiLevel, typeField.Name, val.Type().Name())
		}

		if maxLevel < lvl {
			maxLevel = lvl
		}
	}

	return maxLevel, nil
}
