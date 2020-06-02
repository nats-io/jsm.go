package advisory

import (
	"github.com/nats-io/jsm.go/api/event"
)

// JSSnapshotCreateAdvisoryV1 is an advisory sent after a snapshot is successfully started
//
// NATS Schema io.nats.jetstream.advisory.v1.snapshot_create
type JSSnapshotCreateAdvisoryV1 struct {
	event.NATSEvent
	Stream  string           `json:"stream"`
	NumBlks int              `json:"blocks"`
	BlkSize int              `json:"block_size"`
	Client  APIAuditClientV1 `json:"client"`
}
