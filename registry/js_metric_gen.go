package registry

import (
	"github.com/nats-io/jsm.go/api/jetstream/metric"
)

func init() {
	RegisterTypeFactory("io.nats.jetstream.metric.v1.consumer_ack", func() any { return &metric.ConsumerAckMetricV1{} })
}
