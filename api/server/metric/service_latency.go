package metric

import (
	"time"

	"github.com/nats-io/jsm.go/api/event"
)

// ServiceLatencyV1 is the JSON message sent out in response to latency tracking for
// exported services.
//
// NATS Schema Type io.nats.server.metric.v1.service_latency
type ServiceLatencyV1 struct {
	event.NATSEvent

	Status         int             `json:"status"`
	Error          string          `json:"description,omitempty"`
	Requestor      LatencyClientV1 `json:"requestor,omitempty"`
	Responder      LatencyClientV1 `json:"responder,omitempty"`
	RequestStart   time.Time       `json:"start"`
	ServiceLatency time.Duration   `json:"service"`
	SystemLatency  time.Duration   `json:"system"`
	TotalLatency   time.Duration   `json:"total"`
}

type LatencyClientV1 struct {
	User   string        `json:"user,omitempty"`
	Name   string        `json:"name,omitempty"`
	RTT    time.Duration `json:"rtt"`
	IP     string        `json:"ip"`
	CID    uint64        `json:"cid"`
	Server string        `json:"server"`
}
