// auto generated 2026-08-28 10:23:32.951769 +0200 CEST m=+0.653519543

package registry

import (
	"github.com/nats-io/jsm.go/api/jetstream/metric"
)

func init() {
	RegisterTypeFactory("io.nats.jetstream.metric.v1.consumer_ack", func() any { return &metric.ConsumerAckMetricV1{} })
}
