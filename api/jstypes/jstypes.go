// Copyright 2026 The NATS Authors
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

// Package jstypes holds types shared by the api package and the advisory
// packages, the api package imports the advisories to build the schema registry
// so they cannot import it back and would otherwise need their own copies
package jstypes

import "time"

// PeerInfo is information about a specific peer in a cluster
type PeerInfo struct {
	Name    string        `json:"name" yaml:"name"`
	Current bool          `json:"current" yaml:"current"`
	Offline bool          `json:"offline,omitempty" yaml:"offline"`
	Active  time.Duration `json:"active" yaml:"active"`
	Lag     uint64        `json:"lag,omitempty" yaml:"lag"`
	Peer    string        `json:"peer" yaml:"peer"`
}

// LostStreamData indicates msgs that have been lost
type LostStreamData struct {
	// Msgs is the message IDs of lost messages
	Msgs []uint64 `json:"msgs" yaml:"msgs"`
	// Bytes is how many bytes were lost
	Bytes uint64 `json:"bytes" yaml:"bytes"`
}

// StreamState is the state of a Stream
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
