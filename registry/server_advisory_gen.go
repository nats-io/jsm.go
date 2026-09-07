package registry

import (
	"github.com/nats-io/jsm.go/api/server/advisory"
)

func init() {
	RegisterTypeFactory("io.nats.server.advisory.v1.account_connections", func() any { return &advisory.AccountConnectionsV1{} })
	RegisterTypeFactory("io.nats.server.advisory.v1.client_connect", func() any { return &advisory.ConnectEventMsgV1{} })
	RegisterTypeFactory("io.nats.server.advisory.v1.client_disconnect", func() any { return &advisory.DisconnectEventMsgV1{} })
}
