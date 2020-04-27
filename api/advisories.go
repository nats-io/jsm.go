package api

import (
	"time"
)

// UnknownEvent is a type returned when parsing an unknown type of event
type UnknownEvent = map[string]interface{}

// ConnectEventMsg is sent when a new connection is made that is part of an account.
type ConnectEventMsg struct {
	Type   string     `json:"type"`
	ID     string     `json:"id"`
	Time   string     `json:"timestamp"`
	Server ServerInfo `json:"server"`
	Client ClientInfo `json:"client"`
}

// DisconnectEventMsg is sent when a new connection previously defined from a
// ConnectEventMsg is closed.
type DisconnectEventMsg struct {
	Type     string     `json:"type"`
	ID       string     `json:"id"`
	Time     string     `json:"timestamp"`
	Server   ServerInfo `json:"server"`
	Client   ClientInfo `json:"client"`
	Sent     DataStats  `json:"sent"`
	Received DataStats  `json:"received"`
	Reason   string     `json:"reason"`
}

// ServerInfo identifies remote servers.
type ServerInfo struct {
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	ID        string    `json:"id"`
	Cluster   string    `json:"cluster,omitempty"`
	Version   string    `json:"ver"`
	Seq       uint64    `json:"seq"`
	JetStream bool      `json:"jetstream"`
	Time      time.Time `json:"time"`
}

// DataStats reports how may msg and bytes. Applicable for both sent and received.
type DataStats struct {
	Msgs  int64 `json:"msgs"`
	Bytes int64 `json:"bytes"`
}

// ClientInfo is detailed information about the client forming a connection.
type ClientInfo struct {
	Start   time.Time  `json:"start,omitempty"`
	Host    string     `json:"host,omitempty"`
	ID      uint64     `json:"id"`
	Account string     `json:"acc"`
	User    string     `json:"user,omitempty"`
	Name    string     `json:"name,omitempty"`
	Lang    string     `json:"lang,omitempty"`
	Version string     `json:"ver,omitempty"`
	RTT     string     `json:"rtt,omitempty"`
	Stop    *time.Time `json:"stop,omitempty"`
}

type ConsumerAckMetric struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Time        string `json:"timestamp"`
	Stream      string `json:"stream"`
	Consumer    string `json:"consumer"`
	ConsumerSeq uint64 `json:"consumer_seq"`
	StreamSeq   uint64 `json:"stream_seq"`
	Delay       int64  `json:"ack_time"`
	Deliveries  uint64 `json:"deliveries"`
}

type ConsumerDeliveryExceededAdvisory struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Time       string `json:"timestamp"`
	Stream     string `json:"stream"`
	Consumer   string `json:"consumer"`
	StreamSeq  uint64 `json:"stream_seq"`
	Deliveries uint64 `json:"deliveries"`
}

type JetStreamAPIAudit struct {
	Type     string         `json:"type"`
	ID       string         `json:"id"`
	Time     string         `json:"timestamp"`
	Server   string         `json:"server"`
	Client   APIAuditClient `json:"client"`
	Subject  string         `json:"subject"`
	Request  string         `json:"request,omitempty"`
	Response string         `json:"response"`
}

type APIAuditClient struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	CID      uint64 `json:"cid"`
	Account  string `json:"account"`
	User     string `json:"user,omitempty"`
	Name     string `json:"name,omitempty"`
	Language string `json:"lang,omitempty"`
	Version  string `json:"version,omitempty"`
}
