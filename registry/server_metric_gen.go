package registry

import (
	"github.com/nats-io/jsm.go/api/server/metric"
)

func init() {
	RegisterTypeFactory("io.nats.server.metric.v1.service_latency", func() any { return &metric.ServiceLatencyV1{} })
}
