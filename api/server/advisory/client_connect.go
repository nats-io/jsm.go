package advisory

import (
	"github.com/nats-io/jsm.go/api/event"
)

// ConnectEventMsgV1 is sent when a new connection is made that is part of an account.
//
// NATS Schema Type io.nats.server.advisory.v1.client_connect
type ConnectEventMsgV1 struct {
	event.NATSEvent

	Server ServerInfoV1 `json:"server"`
	Client ClientInfoV1 `json:"client"`
}
