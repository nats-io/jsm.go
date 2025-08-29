// Copyright 2025 The NATS Authors
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

package zmonitor

import (
	"time"
)

// ClusterOptsVarzV1 contains monitoring cluster information
type ClusterOptsVarzV1 struct {
	Name        string   `json:"name,omitempty"`         // Name is the configured cluster name
	Host        string   `json:"addr,omitempty"`         // Host is the host the cluster listens on for connections
	Port        int      `json:"cluster_port,omitempty"` // Port is the port the cluster listens on for connections
	AuthTimeout float64  `json:"auth_timeout,omitempty"` // AuthTimeout is the time cluster connections have to complete authentication
	URLs        []string `json:"urls,omitempty"`         // URLs is the list of cluster URLs
	TLSTimeout  float64  `json:"tls_timeout,omitempty"`  // TLSTimeout is how long TLS operations have to complete
	TLSRequired bool     `json:"tls_required,omitempty"` // TLSRequired indicates if TLS is required for connections
	TLSVerify   bool     `json:"tls_verify,omitempty"`   // TLSVerify indicates if full verification of TLS connections is performed
	PoolSize    int      `json:"pool_size,omitempty"`    // PoolSize is the configured route connection pool size
}

// GatewayOptsVarzV1 contains monitoring gateway information
type GatewayOptsVarzV1 struct {
	Name           string                    `json:"name,omitempty"`            // Name is the configured cluster name
	Host           string                    `json:"host,omitempty"`            // Host is the host the gateway listens on for connections
	Port           int                       `json:"port,omitempty"`            // Port is the post gateway connections listens on
	AuthTimeout    float64                   `json:"auth_timeout,omitempty"`    // AuthTimeout is the time cluster connections have to complete authentication
	TLSTimeout     float64                   `json:"tls_timeout,omitempty"`     // TLSTimeout is how long TLS operations have to complete
	TLSRequired    bool                      `json:"tls_required,omitempty"`    // TLSRequired indicates if TLS is required for connections
	TLSVerify      bool                      `json:"tls_verify,omitempty"`      // TLSVerify indicates if full verification of TLS connections is performed
	Advertise      string                    `json:"advertise,omitempty"`       // Advertise is the URL advertised to remote gateway clients
	ConnectRetries int                       `json:"connect_retries,omitempty"` // ConnectRetries is how many connection attempts the route will make
	Gateways       []RemoteGatewayOptsVarzV1 `json:"gateways,omitempty"`        // Gateways is state of configured gateway remotes
	RejectUnknown  bool                      `json:"reject_unknown,omitempty"`  // RejectUnknown indicates if unknown cluster connections will be rejected
}

// RemoteGatewayOptsVarzV1 contains monitoring remote gateway information
type RemoteGatewayOptsVarzV1 struct {
	Name       string   `json:"name"`                  // Name is the name of the remote gateway
	TLSTimeout float64  `json:"tls_timeout,omitempty"` // TLSTimeout is how long TLS operations have to complete
	URLs       []string `json:"urls,omitempty"`        // URLs is the list of Gateway URLs
}

// LeafNodeOptsVarzV1 contains monitoring leaf node information
type LeafNodeOptsVarzV1 struct {
	Host              string               `json:"host,omitempty"`                 // Host is the host the server listens on
	Port              int                  `json:"port,omitempty"`                 // Port is the port the server listens on
	AuthTimeout       float64              `json:"auth_timeout,omitempty"`         // AuthTimeout is the time Leafnode connections have to complete authentication
	TLSTimeout        float64              `json:"tls_timeout,omitempty"`          // TLSTimeout is how long TLS operations have to complete
	TLSRequired       bool                 `json:"tls_required,omitempty"`         // TLSRequired indicates if TLS is required for connections
	TLSVerify         bool                 `json:"tls_verify,omitempty"`           // TLSVerify indicates if full verification of TLS connections is performed
	Remotes           []RemoteLeafOptsVarz `json:"remotes,omitempty"`              // Remotes is state of configured Leafnode remotes
	TLSOCSPPeerVerify bool                 `json:"tls_ocsp_peer_verify,omitempty"` // TLSOCSPPeerVerify indicates if OCSP verification will be performed
}

// DenyRules Contains lists of subjects not allowed to be imported/exported
type DenyRules struct {
	Exports []string `json:"exports,omitempty"` // Exports are denied exports
	Imports []string `json:"imports,omitempty"` // Imports are denied imports
}

// RemoteLeafOptsVarz contains monitoring remote leaf node information
type RemoteLeafOptsVarz struct {
	LocalAccount      string     `json:"local_account,omitempty"`        // LocalAccount is the local account this leaf is logged into
	TLSTimeout        float64    `json:"tls_timeout,omitempty"`          // TLSTimeout is how long TLS operations have to complete
	URLs              []string   `json:"urls,omitempty"`                 // URLs is the list of URLs for the remote Leafnode connection
	Deny              *DenyRules `json:"deny,omitempty"`                 // Deny is the configured import and exports that the Leafnode may not access
	TLSOCSPPeerVerify bool       `json:"tls_ocsp_peer_verify,omitempty"` // TLSOCSPPeerVerify indicates if OCSP verification will be done
}

// MQTTOptsVarzV1 contains monitoring MQTT information
type MQTTOptsVarzV1 struct {
	Host              string        `json:"host,omitempty"`                 // Host is the host the server listens on
	Port              int           `json:"port,omitempty"`                 // Port is the port the server listens on
	NoAuthUser        string        `json:"no_auth_user,omitempty"`         // NoAuthUser is the user that will be used for unauthenticated connections
	AuthTimeout       float64       `json:"auth_timeout,omitempty"`         // AuthTimeout is how long authentication has to complete
	TLSMap            bool          `json:"tls_map,omitempty"`              // TLSMap indicates if TLS Mapping is enabled
	TLSTimeout        float64       `json:"tls_timeout,omitempty"`          // TLSTimeout is how long TLS operations have to complete
	TLSPinnedCerts    []string      `json:"tls_pinned_certs,omitempty"`     // TLSPinnedCerts is the list of certificates pinned to this connection
	JsDomain          string        `json:"js_domain,omitempty"`            // JsDomain is the JetStream domain used for MQTT state
	AckWait           time.Duration `json:"ack_wait,omitempty"`             // AckWait is how long the internal JetStream state store will allow acks to complete
	MaxAckPending     uint16        `json:"max_ack_pending,omitempty"`      // MaxAckPending is how many outstanding acks the internal JetStream state store will allow
	TLSOCSPPeerVerify bool          `json:"tls_ocsp_peer_verify,omitempty"` // TLSOCSPPeerVerify indicates if OCSP verification will be done
}

// WebsocketOptsVarzV1 contains monitoring websocket information
type WebsocketOptsVarzV1 struct {
	Host              string        `json:"host,omitempty"`                 // Host is the host the server listens on
	Port              int           `json:"port,omitempty"`                 // Port is the port the server listens on
	Advertise         string        `json:"advertise,omitempty"`            // Advertise is the connection URL the server advertises
	NoAuthUser        string        `json:"no_auth_user,omitempty"`         // NoAuthUser is the user that will be used for unauthenticated connections
	JWTCookie         string        `json:"jwt_cookie,omitempty"`           // JWTCookie is the name of a cookie the server will read for the connection JWT
	HandshakeTimeout  time.Duration `json:"handshake_timeout,omitempty"`    // HandshakeTimeout is how long the connection has to complete the websocket setup
	AuthTimeout       float64       `json:"auth_timeout,omitempty"`         // AuthTimeout is how long authentication has to complete
	NoTLS             bool          `json:"no_tls,omitempty"`               // NoTLS indicates if TLS is disabled
	TLSMap            bool          `json:"tls_map,omitempty"`              // TLSMap indicates if TLS Mapping is enabled
	TLSPinnedCerts    []string      `json:"tls_pinned_certs,omitempty"`     // TLSPinnedCerts is the list of certificates pinned to this connection
	SameOrigin        bool          `json:"same_origin,omitempty"`          // SameOrigin indicates if same origin connections are allowed
	AllowedOrigins    []string      `json:"allowed_origins,omitempty"`      // AllowedOrigins list of configured trusted origins
	Compression       bool          `json:"compression,omitempty"`          // Compression indicates if compression is supported
	TLSOCSPPeerVerify bool          `json:"tls_ocsp_peer_verify,omitempty"` // TLSOCSPPeerVerify indicates if OCSP verification will be done
}

// OCSPResponseCacheVarzV1 contains OCSP response cache information
type OCSPResponseCacheVarzV1 struct {
	Type      string `json:"cache_type,omitempty"`               // Type is the kind of cache being used
	Hits      int64  `json:"cache_hits,omitempty"`               // Hits is how many times the cache was able to answer a request
	Misses    int64  `json:"cache_misses,omitempty"`             // Misses is how many times the cache failed to answer a request
	Responses int64  `json:"cached_responses,omitempty"`         // Responses is how many responses are currently stored in the cache
	Revokes   int64  `json:"cached_revoked_responses,omitempty"` // Revokes is how many of the stored cache entries are revokes
	Goods     int64  `json:"cached_good_responses,omitempty"`    // Goods is how many of the stored cache entries are good responses
	Unknowns  int64  `json:"cached_unknown_responses,omitempty"` // Unknowns  is how many of the stored cache entries are unknown responses
}

// JetStreamVarzV1 contains basic runtime information about jetstream
type JetStreamVarzV1 struct {
	Config *JetStreamConfigV1 `json:"config,omitempty"` // Config is the active JetStream configuration
	Stats  *JetStreamStatsV1  `json:"stats,omitempty"`  // Stats is the statistics for the JetStream server
	Meta   *MetaClusterInfoV1 `json:"meta,omitempty"`   // Meta is information about the JetStream metalayer
	Limits *JSLimitOptsV1     `json:"limits,omitempty"` // Limits are the configured JetStream limits
}

// JSLimitOptsV1 are active limits for the meta cluster
type JSLimitOptsV1 struct {
	MaxRequestBatch           int           `json:"max_request_batch,omitempty"`             // MaxRequestBatch is the maximum amount of updates that can be sent in a batch
	MaxAckPending             int           `json:"max_ack_pending,omitempty"`               // MaxAckPending is the server limit for maximum amount of outstanding Acks
	MaxHAAssets               int           `json:"max_ha_assets,omitempty"`                 // MaxHAAssets is the maximum of Streams and Consumers that may have more than 1 replica
	Duplicates                time.Duration `json:"max_duplicate_window,omitempty"`          // Duplicates is the maximum value for duplicate tracking on Streams
	MaxBatchInflightPerStream int           `json:"max_batch_inflight_per_stream,omitempty"` // MaxBatchInflightPerStream is the maximum amount of open batches per stream
	MaxBatchInflightTotal     int           `json:"max_batch_inflight_total,omitempty"`      // MaxBatchInflightTotal is the maximum amount of total open batches per server
	MaxBatchSize              int           `json:"max_batch_size,omitempty"`                // MaxBatchSize is the maximum amount of messages allowed in a batch publish to a Stream
	MaxBatchTimeout           time.Duration `json:"max_batch_timeout,omitempty"`             // MaxBatchTimeout is the maximum time to receive the commit message after receiving the first message of a batch
}

// MetaClusterInfoV1 shows information about the meta group.
type MetaClusterInfoV1 struct {
	Name     string        `json:"name,omitempty"`     // Name is the name of the cluster
	Leader   string        `json:"leader,omitempty"`   // Leader is the server name of the cluster leader
	Peer     string        `json:"peer,omitempty"`     // Peer is unique ID for each peer
	Replicas []*PeerInfoV1 `json:"replicas,omitempty"` // Replicas is a list of known peers
	Size     int           `json:"cluster_size"`       // Size is the known size of the cluster
	Pending  int           `json:"pending"`            // Pending is how many RAFT messages are not yet processed
}

// PeerInfoV1 shows information about all the peers in the cluster that
// are supporting the stream or consumer.
type PeerInfoV1 struct {
	Name    string        `json:"name"`              // Name is the unique name for the peer
	Current bool          `json:"current"`           // Current indicates if it was seen recently and fully caught up
	Offline bool          `json:"offline,omitempty"` // Offline indicates if it has not been seen recently
	Active  time.Duration `json:"active"`            // Active is the timestamp it was last active
	Lag     uint64        `json:"lag,omitempty"`     // Lag is how many operations behind it is
	Peer    string        `json:"peer"`              // Peer is the unique ID for the peer
}

// SlowConsumersStatsV1 contains information about the slow consumers from different type of connections.
type SlowConsumersStatsV1 struct {
	Clients  uint64 `json:"clients"`  // Clients is how many Clients were slow consumers
	Routes   uint64 `json:"routes"`   // Routes is how many Routes were slow consumers
	Gateways uint64 `json:"gateways"` // Gateways is how many Gateways were slow consumers
	Leafs    uint64 `json:"leafs"`    // Leafs is how many Leafnodes were slow consumers
}

// StaleConnectionStatsV1 contains information about the stale connections from different type of connections.
type StaleConnectionStatsV1 struct {
	Clients  uint64 `json:"clients"`  // Clients is how many Clients were slow consumers
	Routes   uint64 `json:"routes"`   // Routes is how many Routes were slow consumers
	Gateways uint64 `json:"gateways"` // Gateways is how many Gateways were slow consumers
	Leafs    uint64 `json:"leafs"`    // Leafs is how many Leafnodes were slow consumers
}

// ProxiesOptsVarzV1 contains network proxies information
type ProxiesOptsVarzV1 struct {
	Trusted []*ProxyOptsVarzV1 `json:"trusted,omitempty"` // Trusted holds a list of trusted proxies
}

// ProxyOptsVarzV1 contains proxy information
type ProxyOptsVarzV1 struct {
	Key string `json:"key"` // Key is the public key of the trusted proxy
}

// JetStreamConfigV1 determines this server's configuration.
// MaxMemory and MaxStore are in bytes.
type JetStreamConfigV1 struct {
	MaxMemory    int64         `json:"max_memory"`              // MaxMemory is the maximum size of memory type streams
	MaxStore     int64         `json:"max_storage"`             // MaxStore is the maximum size of file store type streams
	StoreDir     string        `json:"store_dir,omitempty"`     // StoreDir is where storage files are stored
	SyncInterval time.Duration `json:"sync_interval,omitempty"` // SyncInterval is how frequently we sync to disk in the background by calling fsync
	SyncAlways   bool          `json:"sync_always,omitempty"`   // SyncAlways indicates flushes are done after every write
	Domain       string        `json:"domain,omitempty"`        // Domain is the JetStream domain
	CompressOK   bool          `json:"compress_ok,omitempty"`   // CompressOK indicates if compression is supported
	UniqueTag    string        `json:"unique_tag,omitempty"`    // UniqueTag is the unique tag assigned to this instance
	Strict       bool          `json:"strict,omitempty"`        // Strict indicates if strict JSON parsing is performed
}

// JetStreamStatsV1 holds statistics about JetStream
type JetStreamStatsV1 struct {
	Memory         uint64              `json:"memory"`
	Store          uint64              `json:"storage"`
	ReservedMemory uint64              `json:"reserved_memory"`
	ReservedStore  uint64              `json:"reserved_storage"`
	Accounts       int                 `json:"accounts"`
	HAAssets       int                 `json:"ha_assets"`
	API            JetStreamAPIStatsV1 `json:"api"`
}

// JetStreamAPIStatsV1 holds stats about the API usage for this server
type JetStreamAPIStatsV1 struct {
	Level    int    `json:"level"`              // Level is the active API level this server implements
	Total    uint64 `json:"total"`              // Total is the total API requests received since start
	Errors   uint64 `json:"errors"`             // Errors is the total API requests that resulted in error responses
	Inflight uint64 `json:"inflight,omitempty"` // Inflight are the number of API requests currently being served
}
