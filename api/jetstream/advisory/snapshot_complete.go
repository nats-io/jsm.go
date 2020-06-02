package advisory

import (
	"time"

	"github.com/nats-io/jsm.go/api/event"
)

// JSSnapshotCompleteAdvisoryV1 is an advisory sent after a snapshot is successfully started
//
// NATS Schema Type io.nats.jetstream.advisory.v1.snapshot_complete
type JSSnapshotCompleteAdvisoryV1 struct {
	event.NATSEvent

	Stream string           `json:"stream"`
	Start  time.Time        `json:"start"`
	End    time.Time        `json:"end"`
	Client APIAuditClientV1 `json:"client"`
}
