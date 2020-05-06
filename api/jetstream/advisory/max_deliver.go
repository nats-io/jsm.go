package advisory

import (
	"github.com/nats-io/jsm.go/api/event"
)

// ConsumerDeliveryExceededAdvisoryV1 is an advisory published when a consumer
// message reaches max delivery attempts
//
// NATS Schema Type io.nats.jetstream.advisory.v1.max_deliver
type ConsumerDeliveryExceededAdvisoryV1 struct {
	event.NATSEvent

	Stream     string `json:"stream"`
	Consumer   string `json:"consumer"`
	StreamSeq  uint64 `json:"stream_seq"`
	Deliveries uint64 `json:"deliveries"`
}
