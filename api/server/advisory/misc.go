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
	DataStatsSetV1
	Gateways DataStatsSetV1 `json:"gateways"`
	Routes   DataStatsSetV1 `json:"routes"`
	Leafs    DataStatsSetV1 `json:"leafs"`
}

// DataStatsSetV1 reports how many messages and bytes were processed by a connection
type DataStatsSetV1 struct {
	Msgs  int64 `json:"msgs"`
	Bytes int64 `json:"bytes"`
}
