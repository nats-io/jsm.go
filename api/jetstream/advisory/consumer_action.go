package advisory

import (
	"github.com/nats-io/jsm.go/api/event"
)

// JSConsumerActionAdvisoryV1 is a advisory published on create or delete of a Consumer
//
// NATS Schema Type io.nats.jetstream.advisory.v1.consumer_action
type JSConsumerActionAdvisoryV1 struct {
	event.NATSEvent

	Stream   string               `json:"stream"`
	Consumer string               `json:"consumer"`
	Action   ActionAdvisoryTypeV1 `json:"action"`
}
