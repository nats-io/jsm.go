package registry

import (
	"github.com/nats-io/jsm.go/api/server/zmonitor"
)

func init() {
	RegisterTypeFactory("io.nats.server.monitor.v1.varz", func() any { return &zmonitor.VarzV1{} })
}
