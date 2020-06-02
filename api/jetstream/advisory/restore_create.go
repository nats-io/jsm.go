package advisory

import (
	"github.com/nats-io/jsm.go/api/event"
)

// JSRestoreCreateAdvisoryV1 is an advisory sent after a snapshot is successfully started
//
// NATS Schema io.nats.jetstream.advisory.v1.restore_create
type JSRestoreCreateAdvisoryV1 struct {
	event.NATSEvent
	Stream string           `json:"stream"`
	Client APIAuditClientV1 `json:"client"`
}
