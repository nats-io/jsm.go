package metric

import (
	"time"
)

// ServiceLatencyV1 is the JSON message sent out in response to latency tracking for
// exported services.
//
// NATS Schema Type io.nats.server.metric.v1.service_latency
type ServiceLatencyV1 struct {
	Type           string        `json:"type"`
	ID             string        `json:"id"`
	Time           string        `json:"timestamp"`
	Status         int           `json:"status"`
	Error          string        `json:"description,omitempty"`
	AppName        string        `json:"app,omitempty"`
	RequestStart   time.Time     `json:"start"`
	ServiceLatency time.Duration `json:"svc"`
	NATSLatency    NATSLatencyV1 `json:"nats"`
	TotalLatency   time.Duration `json:"total"`
}

// NATSLatencyV1 represents the internal NATS latencies, including RTTs to clients.
type NATSLatencyV1 struct {
	Requestor time.Duration `json:"req"`
	Responder time.Duration `json:"resp"`
	System    time.Duration `json:"sys"`
}
