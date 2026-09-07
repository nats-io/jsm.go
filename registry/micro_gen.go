package registry

import (
	"github.com/nats-io/nats.go/micro"
)

func init() {
	RegisterTypeFactory("io.nats.micro.v1.info_response", func() any { return &micro.Info{} })
	RegisterTypeFactory("io.nats.micro.v1.ping_response", func() any { return &micro.Ping{} })
	RegisterTypeFactory("io.nats.micro.v1.stats_response", func() any { return &micro.Stats{} })
}
