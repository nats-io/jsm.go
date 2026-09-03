// auto generated 2026-08-28 10:23:33.0716 +0200 CEST m=+0.773348709

package registry

import (
	"github.com/nats-io/jsm.go/api/server/zmonitor"
)

func init() {
	RegisterTypeFactory("io.nats.server.monitor.v1.varz", func() any { return &zmonitor.VarzV1{} })
}
