package advisory

import "time"

// ServerInfoV1 identifies remote servers.
type ServerInfoV1 struct {
	Name      string            `json:"name"`
	Host      string            `json:"host"`
	ID        string            `json:"id"`
	Cluster   string            `json:"cluster,omitempty"`
	Domain    string            `json:"domain,omitempty"`
	Version   string            `json:"ver"`
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	JetStream bool              `json:"jetstream"` // Whether JetStream is enabled (deprecated in favor of the `ServerCapability`).
	Flags     uint64            `json:"flags"`     // Generic capability flags
	Seq       uint64            `json:"seq"`       // Sequence and Time from the remote server for this message.
	Time      time.Time         `json:"time"`
}

// ClientInfoV1 is detailed information about the client forming a connection.
type ClientInfoV1 struct {
	Start      *time.Time    `json:"start,omitempty"`
	Host       string        `json:"host,omitempty"`
	ID         uint64        `json:"id,omitempty"`
	Account    string        `json:"acc,omitempty"`
	Service    string        `json:"svc,omitempty"`
	User       string        `json:"user,omitempty"`
	Name       string        `json:"name,omitempty"`
	Lang       string        `json:"lang,omitempty"`
	Version    string        `json:"ver,omitempty"`
	RTT        time.Duration `json:"rtt,omitempty"`
	Server     string        `json:"server,omitempty"`
	Cluster    string        `json:"cluster,omitempty"`
	Alternates []string      `json:"alts,omitempty"`
	Stop       *time.Time    `json:"stop,omitempty"`
	Jwt        string        `json:"jwt,omitempty"`
	IssuerKey  string        `json:"issuer_key,omitempty"`
	NameTag    string        `json:"name_tag,omitempty"`
	Tags       []string      `json:"tags,omitempty"`
	Kind       string        `json:"kind,omitempty"`
	ClientType string        `json:"client_type,omitempty"`
	MQTTClient string        `json:"client_id,omitempty"` // This is the MQTT client ID
	Nonce      string        `json:"nonce,omitempty"`
}

// DataStatsV1 reports how may msg and bytes. Applicable for both sent and received.
type DataStatsV1 struct {
	MsgBytesV1
	Gateways *MsgBytesV1 `json:"gateways,omitempty"` // Gateways is usage stats for gatgeway connections, only reported for account level events
	Routes   *MsgBytesV1 `json:"routes,omitempty"`   // Routes is usage stats for route connections, only reported for account level events
	Leafs    *MsgBytesV1 `json:"leafs,omitempty"`    // Leafs is usage stats for leafnode connections, only reported for account level events
}

// MsgBytesV1 reports how many messages and bytes were processed by a connection
type MsgBytesV1 struct {
	Msgs  int64 `json:"msgs"`
	Bytes int64 `json:"bytes"`
}
