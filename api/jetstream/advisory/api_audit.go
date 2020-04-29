package advisory

import (
	"github.com/nats-io/jsm.go/api/event"
)

// JetStreamAPIAuditV1 is a advisory published for any JetStream API access
//
// NATS Schema Type io.nats.jetstream.advisory.v1.api_audit
type JetStreamAPIAuditV1 struct {
	event.NATSEvent

	Server   string           `json:"server"`
	Client   APIAuditClientV1 `json:"client"`
	Subject  string           `json:"subject"`
	Request  string           `json:"request,omitempty"`
	Response string           `json:"response"`
}

type APIAuditClientV1 struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	CID      uint64 `json:"cid"`
	Account  string `json:"account"`
	User     string `json:"user,omitempty"`
	Name     string `json:"name,omitempty"`
	Language string `json:"lang,omitempty"`
	Version  string `json:"version,omitempty"`
}
