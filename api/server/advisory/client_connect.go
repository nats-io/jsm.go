package advisory

import (
	"time"
)

// ConnectEventMsgV1 is sent when a new connection is made that is part of an account.
//
// NATS Schema Type io.nats.server.advisory.v1.client_connect
type ConnectEventMsgV1 struct {
	Type   string       `json:"type"`
	ID     string       `json:"id"`
	Time   time.Time    `json:"timestamp"`
	Server ServerInfoV1 `json:"server"`
	Client ClientInfoV1 `json:"client"`
}
