package advisory

import (
	"time"

	"github.com/nats-io/jsm.go/api/event"
)

// JSRestoreCompleteAdvisoryV1 is an advisory sent after a snapshot is successfully started
//
// NATS Schema type io.nats.jetstream.advisory.v1.restore_complete
type JSRestoreCompleteAdvisoryV1 struct {
	event.NATSEvent

	Stream string           `json:"stream"`
	Start  time.Time        `json:"start"`
	End    time.Time        `json:"end"`
	Bytes  int64            `json:"bytes"`
	Client APIAuditClientV1 `json:"client"`
}
