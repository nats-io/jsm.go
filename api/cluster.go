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
	JSApiLeaderStepDown       = "$JS.API.META.LEADER.STEPDOWN"
	JSApiLeaderStepDownPrefix = "$JS.API.META.LEADER.STEPDOWN"
	JSApiLeaderStepDownT      = "$JS.API.META.LEADER.STEPDOWN"
	JSApiRescueRequest        = "$JS.API.META.RESCUE"
	JSApiRescueRequestPrefix  = "$JS.API.META.RESCUE"
	JSApiRescueRequestT       = "$JS.API.META.RESCUE"
	JSApiRemoveServer         = "$JS.API.SERVER.REMOVE"
	JSApiRemoveServerPrefix   = "$JS.API.SERVER.REMOVE"
	JSApiPurgeAccountT        = "$JS.API.ACCOUNT.PURGE.%s"
	JSApiPurgeAccountPrefix   = "$JS.API.ACCOUNT.PURGE"
)

// io.nats.jetstream.api.v1.meta_leader_stepdown_request
type JSApiLeaderStepDownRequest struct {
	Placement *Placement `json:"placement,omitempty"`
}

// io.nats.jetstream.api.v1.meta_leader_stepdown_response
type JSApiLeaderStepDownResponse struct {
	JSApiResponse
	Success bool `json:"success,omitempty"`
}

// io.nats.jetstream.api.v1.meta_server_remove_request
type JSApiMetaServerRemoveRequest struct {
	// Server name of the peer to be removed.
	Server string `json:"peer"`
	// Peer ID of the peer to be removed. If specified this is used
	// instead of the server name.
	Peer string `json:"peer_id,omitempty"`
}

// io.nats.jetstream.api.v1.meta_server_remove_response
type JSApiMetaServerRemoveResponse struct {
	JSApiResponse
	Success bool `json:"success,omitempty"`
}

// io.nats.jetstream.api.v1.account_purge_response
type JSApiAccountPurgeResponse struct {
	JSApiResponse
	Initiated bool `json:"initiated,omitempty"`
}

// JSApiMetaRescueRequest will unsafely lower the meta group's quorum requirement
// on the receiving server for disaster recovery.
//
// io.nats.jetstream.api.v1.meta_rescue_request
type JSApiMetaRescueRequest struct {
	// The new, temporarily lowered, quorum size the receiving servers should
	// apply to the meta group. Must be at least 1 and no larger than the
	// receiving server's current effective quorum.
	QuorumNeeded int `json:"quorum_needed"`
}

// JSApiMetaRescueResponse is the response to a meta rescue request. Since the
// request is a broadcast, each online server responds independently.
//
// io.nats.jetstream.api.v1.meta_rescue_response
type JSApiMetaRescueResponse struct {
	JSApiResponse
	// Server name of the responding server.
	Server string `json:"server"`
	// Server ID of the responding server.
	ServerID string `json:"server_id"`
	// The effective quorum before the rescue was applied.
	PrevQuorum int `json:"prev_quorum,omitempty"`
	// The effective quorum after the rescue was applied.
	NewQuorum int `json:"new_quorum,omitempty"`
}

// ClusterInfo shows information about the underlying set of servers
// that make up the stream or consumer.
type ClusterInfo struct {
	Name        string      `json:"name,omitempty" yaml:"name"`
	RaftGroup   string      `json:"raft_group,omitempty" yaml:"raft_group"`
	Leader      string      `json:"leader,omitempty" yaml:"leader"`
	LeaderSince *time.Time  `json:"leader_since,omitempty" yaml:"leader_since"`
	SystemAcc   bool        `json:"system_account,omitempty" yaml:"system_account"`
	TrafficAcc  string      `json:"traffic_account,omitempty" yaml:"traffic_account"`
	Replicas    []*PeerInfo `json:"replicas,omitempty" yaml:"replicas"`
}

// PeerInfo shows information about all the peers in the cluster that
// are supporting the stream or consumer.
type PeerInfo struct {
	Name    string        `json:"name" yaml:"name"`                 // Name is the unique name for the peer
	Current bool          `json:"current" yaml:"current"`           // Current indicates if it was seen recently and fully caught up
	Offline bool          `json:"offline,omitempty" yaml:"offline"` // Offline indicates if it has not been seen recently
	Active  time.Duration `json:"active" yaml:"active"`             // Active is the nanoseconds since this peer was last seen
	Lag     uint64        `json:"lag,omitempty" yaml:"lag"`         // Lag is how many operations behind it is
	Peer    string        `json:"peer" yaml:"peer"`                 // Peer is the unique ID for the peer
	Pending bool          `json:"pending,omitempty" yaml:"pending"` // Pending indicates the peer is part of the assignment, but is not a peer of the Raft group yet or is being removed
}
