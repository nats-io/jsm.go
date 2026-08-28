// auto generated 2026-08-28 10:23:33.008274 +0200 CEST m=+0.710023751

package registry

import (
	"github.com/nats-io/jsm.go/api/server/metric"
)

func init() {
	RegisterTypeFactory("io.nats.server.metric.v1.service_latency", func() any { return &metric.ServiceLatencyV1{} })
}
