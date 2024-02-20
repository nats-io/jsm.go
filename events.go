package jsm

import (
	"fmt"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats-server/v2/server"
)

// ParseEvent parses event e and returns event as for example *api.ConsumerAckMetric, all unknown
// event schemas will be of type *UnknownMessage
func ParseEvent(e []byte) (schema string, event any, err error) {
	return api.ParseMessage(e)
}

// ServerKindString takes a kind like server.CLIENT and returns a string describing it
func ServerKindString(kind int) string {
	switch kind {
	case server.CLIENT:
		return "Client"
	case server.ROUTER:
		return "Router"
	case server.GATEWAY:
		return "Gateway"
	case server.SYSTEM:
		return "System"
	case server.LEAF:
		return "Leafnode"
	case server.JETSTREAM:
		return "JetStream"
	case server.ACCOUNT:
		return "Account"
	default:
		return "Unknown Kind"
	}
}

// ServerCidString takes a kind like server.CLIENT a similar cid like the server would, eg cid:10
func ServerCidString(kind int, id uint64) string {
	format := "id:%d"

	switch kind {
	case server.CLIENT:
		format = "cid:%d"
	case server.ROUTER:
		format = "rid:%d"
	case server.GATEWAY:
		format = "gid:%d"
	case server.LEAF:
		format = "lid:%d"
	}

	return fmt.Sprintf(format, id)
}
