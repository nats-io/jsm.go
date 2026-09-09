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
	Name        string              `json:"name,omitempty" yaml:"name"`
	RaftGroup   string              `json:"raft_group,omitempty" yaml:"raft_group"`
	Leader      string              `json:"leader,omitempty" yaml:"leader"`
	LeaderSince *time.Time          `json:"leader_since,omitempty" yaml:"leader_since"`
	SystemAcc   bool                `json:"system_account,omitempty" yaml:"system_account"`
	TrafficAcc  string              `json:"traffic_account,omitempty" yaml:"traffic_account"`
	Replicas    []*PeerInfo         `json:"replicas,omitempty" yaml:"replicas"`
	Desired     *DesiredClusterInfo `json:"desired,omitempty"`
}

// DesiredClusterInfo shows information of the desired set of servers
// that should make up the stream or consumer.
type DesiredClusterInfo struct {
	// When the desired state was recorded on the assignment.
	Created time.Time `json:"created"`
	// Name of the target cluster the group should end up in.
	Name string `json:"name,omitempty"`
	// Replicas are the peers chosen to be the final peer set, where
	// ClusterInfo.Replicas holds the peers that currently host the stream or
	// consumer. Omitted while scaling down until the final peer set is selected.
	Replicas []*DesiredPeerInfo `json:"replicas,omitempty"`
	// Origin is the configuration the reconfiguration can be rolled back to if
	// it is canceled.
	Origin *DesiredClusterInfoOrigin `json:"origin,omitempty"`
	// Status describes what the group leader is currently doing to reach the
	// desired state, or what it is waiting on.
	Status *DesiredClusterInfoStatus `json:"status,omitempty"`
}

// DesiredPeerInfo is a minimal version of PeerInfo that shows information about the desired peer set.
type DesiredPeerInfo struct {
	Name    string `json:"name"`              // Name is the unique name for the peer
	Offline bool   `json:"offline,omitempty"` // Offline indicates if it has not been seen recently
	Peer    string `json:"peer"`              // Peer is the unique ID for the peer
}

type DesiredClusterInfoOrigin struct {
	// Original replicas before it was updated.
	Replicas int `json:"replicas"`
	// Original placement before it was updated.
	Placement *Placement `json:"placement,omitempty"`
	// When changing between retention policies, this retention remains active until unset.
	Retention *RetentionPolicy `json:"retention,omitempty"`
}

// MigrationStatusType classifies a migration status by what has to change for the
// migration to advance, so it can be matched on without parsing the status line.
type MigrationStatusType string

const (
	MigrationStatusMeta        MigrationStatusType = "meta"        // The meta leader must record or advance desired state.
	MigrationStatusMembership  MigrationStatusType = "membership"  // A proposed membership change must commit.
	MigrationStatusSnapshot    MigrationStatusType = "snapshot"    // A snapshot must be installed.
	MigrationStatusCatchup     MigrationStatusType = "catchup"     // Peers must become store-current.
	MigrationStatusQuorum      MigrationStatusType = "quorum"      // More peers must come online before we can act without losing quorum.
	MigrationStatusBlocked     MigrationStatusType = "blocked"     // Another asset must move first, i.e. the stream/consumer ordering constraint.
	MigrationStatusUnavailable MigrationStatusType = "unavailable" // Nothing to do here, we're shutting down, or the assignment is gone.
)

type DesiredClusterInfoStatus struct {
	// Description is a short status line describing what the group leader is currently
	// doing to move this group toward its desired state, or what it's waiting on.
	Description string `json:"description"`
	// Type classifies Description by what has to change for the migration to
	// advance, so it can be matched on without parsing the status line.
	Type MigrationStatusType `json:"type"`
	// Err is the underlying failure behind this status, if it had one. Only set
	// for faults that persist across cycles, never for races that resolve themselves.
	Err string `json:"err,omitempty"`
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
