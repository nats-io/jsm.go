// auto generated 2026-08-28 10:23:33.041174 +0200 CEST m=+0.742923126

package registry

import (
	"github.com/nats-io/jsm.go/api/server/advisory"
)

func init() {
	RegisterTypeFactory("io.nats.server.advisory.v1.account_connections", func() any { return &advisory.AccountConnectionsV1{} })
	RegisterTypeFactory("io.nats.server.advisory.v1.client_connect", func() any { return &advisory.ConnectEventMsgV1{} })
	RegisterTypeFactory("io.nats.server.advisory.v1.client_disconnect", func() any { return &advisory.DisconnectEventMsgV1{} })
}
