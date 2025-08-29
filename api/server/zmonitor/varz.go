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

	"github.com/nats-io/jwt/v2"
)

// VarzV1 holds server information
//
// NATS Schema Type io.nats.server.monitor.v1.varz
type VarzV1 struct {
	ID                    string                   `json:"server_id"`                         // ID is the unique server ID generated at start
	Name                  string                   `json:"server_name"`                       // Name is the configured server name, equals ID when not set
	Version               string                   `json:"version"`                           // Version is the version of the running server
	Proto                 int                      `json:"proto"`                             // Proto is the protocol version this server supports
	GitCommit             string                   `json:"git_commit,omitempty"`              // GitCommit is the git repository commit hash that the build corresponds with
	GoVersion             string                   `json:"go"`                                // GoVersion is the version of Go used to build this binary
	Host                  string                   `json:"host"`                              // Host is the hostname the server runs on
	Port                  int                      `json:"port"`                              // Port is the port the server listens on for client connections
	AuthRequired          bool                     `json:"auth_required,omitempty"`           // AuthRequired indicates if users are required to authenticate to join the server
	TLSRequired           bool                     `json:"tls_required,omitempty"`            // TLSRequired indicates if connections must use TLS when connecting to this server
	TLSVerify             bool                     `json:"tls_verify,omitempty"`              // TLSVerify indicates if full TLS verification will be performed
	TLSOCSPPeerVerify     bool                     `json:"tls_ocsp_peer_verify,omitempty"`    // TLSOCSPPeerVerify indicates if the OCSP protocol will be used to verify peers
	IP                    string                   `json:"ip,omitempty"`                      // IP is the IP address the server listens on if set
	ClientConnectURLs     []string                 `json:"connect_urls,omitempty"`            // ClientConnectURLs is the list of URLs NATS clients can use to connect to this server
	WSConnectURLs         []string                 `json:"ws_connect_urls,omitempty"`         // WSConnectURLs is the list of URLs websocket clients can use to connect to this server
	MaxConn               int                      `json:"max_connections"`                   // MaxConn is the maximum amount of connections the server can accept
	MaxSubs               int                      `json:"max_subscriptions,omitempty"`       // MaxSubs is the maximum amount of subscriptions the server can manage
	PingInterval          time.Duration            `json:"ping_interval"`                     // PingInterval is the interval the server will send PING messages during periods of inactivity on a connection
	MaxPingsOut           int                      `json:"ping_max"`                          // MaxPingsOut is the number of unanswered PINGs after which the connection will be considered stale
	HTTPHost              string                   `json:"http_host"`                         // HTTPHost is the HTTP host monitoring connections are accepted on
	HTTPPort              int                      `json:"http_port"`                         // HTTPPort is the port monitoring connections are accepted on
	HTTPBasePath          string                   `json:"http_base_path"`                    // HTTPBasePath is the path prefix for access to monitor endpoints
	HTTPSPort             int                      `json:"https_port"`                        // HTTPSPort is the HTTPS host monitoring connections are accepted on`
	AuthTimeout           float64                  `json:"auth_timeout"`                      // AuthTimeout is the amount of seconds connections have to complete authentication
	MaxControlLine        int32                    `json:"max_control_line"`                  // MaxControlLine is the amount of bytes a signal control message may be
	MaxPayload            int                      `json:"max_payload"`                       // MaxPayload is the maximum amount of bytes a message may have as payload
	MaxPending            int64                    `json:"max_pending"`                       // MaxPending is the maximum amount of unprocessed bytes a connection may have
	Cluster               ClusterOptsVarzV1        `json:"cluster,omitempty"`                 // Cluster is the Cluster state
	Gateway               GatewayOptsVarzV1        `json:"gateway,omitempty"`                 // Gateway is the Super Cluster state
	LeafNode              LeafNodeOptsVarzV1       `json:"leaf,omitempty"`                    // LeafNode is the Leafnode state
	MQTT                  MQTTOptsVarzV1           `json:"mqtt,omitempty"`                    // MQTT is the MQTT state
	Websocket             WebsocketOptsVarzV1      `json:"websocket,omitempty"`               // Websocket is the Websocket client state
	JetStream             JetStreamVarzV1          `json:"jetstream,omitempty"`               // JetStream is the JetStream state
	TLSTimeout            float64                  `json:"tls_timeout"`                       // TLSTimeout is how long TLS operations have to complete
	WriteDeadline         time.Duration            `json:"write_deadline"`                    // WriteDeadline is the maximum time writes to sockets have to complete
	Start                 time.Time                `json:"start"`                             // Start is time when the server was started
	Now                   time.Time                `json:"now"`                               // Now is the current time of the server
	Uptime                string                   `json:"uptime"`                            // Uptime is how long the server has been running
	Mem                   int64                    `json:"mem"`                               // Mem is the resident memory allocation
	Cores                 int                      `json:"cores"`                             // Cores is the number of cores the process has access to
	MaxProcs              int                      `json:"gomaxprocs"`                        // MaxProcs is the configured GOMAXPROCS value
	MemLimit              int64                    `json:"gomemlimit,omitempty"`              // MemLimit is the configured GOMEMLIMIT value
	CPU                   float64                  `json:"cpu"`                               // CPU is the current total CPU usage
	Connections           int                      `json:"connections"`                       // Connections is the current connected connections
	TotalConnections      uint64                   `json:"total_connections"`                 // TotalConnections is the total connections the server have ever handled
	Routes                int                      `json:"routes"`                            // Routes is the number of connected route servers
	Remotes               int                      `json:"remotes"`                           // Remotes is the configured route remote endpoints
	Leafs                 int                      `json:"leafnodes"`                         // Leafs is the number connected leafnode clients
	InMsgs                int64                    `json:"in_msgs"`                           // InMsgs is the number of messages this server received
	OutMsgs               int64                    `json:"out_msgs"`                          // OutMsgs is the number of message this server sent
	InBytes               int64                    `json:"in_bytes"`                          // InBytes is the number of bytes this server received
	OutBytes              int64                    `json:"out_bytes"`                         // OutMsgs is the number of bytes this server sent
	SlowConsumers         int64                    `json:"slow_consumers"`                    // SlowConsumers is the total count of clients that were disconnected since start due to being slow consumers
	StaleConnections      int64                    `json:"stale_connections"`                 // StaleConnections is the total count of stale connections that were detected
	StalledClients        int64                    `json:"stalled_clients"`                   // StalledClients is the total number of times that clients have been stalled.
	Subscriptions         uint32                   `json:"subscriptions"`                     // Subscriptions is the count of active subscriptions
	HTTPReqStats          map[string]uint64        `json:"http_req_stats"`                    // HTTPReqStats is the number of requests each HTTP endpoint received
	ConfigLoadTime        time.Time                `json:"config_load_time"`                  // ConfigLoadTime is the time the configuration was loaded or reloaded
	ConfigDigest          string                   `json:"config_digest"`                     // ConfigDigest is a calculated hash of the current configuration
	Tags                  jwt.TagList              `json:"tags,omitempty"`                    // Tags are the tags assigned to the server in configuration
	Metadata              map[string]string        `json:"metadata,omitempty"`                // Metadata is the metadata assigned to the server in configuration
	TrustedOperatorsJwt   []string                 `json:"trusted_operators_jwt,omitempty"`   // TrustedOperatorsJwt is the JWTs for all trusted operators
	TrustedOperatorsClaim []*jwt.OperatorClaims    `json:"trusted_operators_claim,omitempty"` // TrustedOperatorsClaim is the decoded claims for each trusted operator
	SystemAccount         string                   `json:"system_account,omitempty"`          // SystemAccount is the name of the System account
	PinnedAccountFail     uint64                   `json:"pinned_account_fails,omitempty"`    // PinnedAccountFail is how often user logon fails due to the issuer account not being pinned.
	OCSPResponseCache     *OCSPResponseCacheVarzV1 `json:"ocsp_peer_cache,omitempty"`         // OCSPResponseCache is the state of the OCSP cache
	SlowConsumersStats    *SlowConsumersStatsV1    `json:"slow_consumer_stats"`               // SlowConsumersStats is statistics about all detected Slow Consumer
	StaleConnectionStats  *StaleConnectionStatsV1  `json:"stale_connection_stats,omitempty"`  // StaleConnectionStats are statistics about all detected Stale Connections
	Proxies               *ProxiesOptsVarzV1       `json:"proxies,omitempty"`                 // Proxies hold information about network proxy devices
}
