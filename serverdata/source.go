package serverdata

import (
	"errors"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

var ErrUnsupportedOption = errors.New("unsupported option")

// RequestFunc defines the callback used when connecting to live servers for data
type RequestFunc func(req any, subj string, waitFor int, nc *nats.Conn) ([][]byte, error)

// ProfilezResponse is the typed wire format of $SYS.REQ.SERVER.PING.PROFILEZ.
// This is mirrored in the Gatherer.
// TODO(ploubser): Extract this type and use it everywhere
type ProfilezResponse struct {
	Server *server.ServerInfo     `json:"server"`
	Data   *server.ProfilezStatus `json:"data,omitempty"`
	Error  *server.ApiError       `json:"error,omitempty"`
}

// Source abstracts server data retrieval
type Source interface {
	Varz(opts server.VarzEventOptions) ([]*server.ServerAPIVarzResponse, error)
	Connz(opts server.ConnzEventOptions) ([]*server.ServerAPIConnzResponse, error)
	Routez(opts server.RoutezEventOptions) ([]*server.ServerAPIRoutezResponse, error)
	Gatewayz(opts server.GatewayzEventOptions) ([]*server.ServerAPIGatewayzResponse, error)
	Leafz(opts server.LeafzEventOptions) ([]*server.ServerAPILeafzResponse, error)
	Subsz(opts server.SubszEventOptions) ([]*server.ServerAPISubszResponse, error)
	Jsz(opts server.JszEventOptions) ([]*server.ServerAPIJszResponse, error)
	Healthz(opts server.HealthzEventOptions) ([]*server.ServerAPIHealthzResponse, error)
	Accountz(opts server.AccountzEventOptions) ([]*server.ServerAPIAccountzResponse, error)
	Statz(opts server.StatszEventOptions) ([]*server.ServerStatsMsg, error)
	Ipqueuesz(opts server.IpqueueszEventOptions) ([]*server.ServerAPIpqueueszResponse, error)
	Raftz(opts server.RaftzEventOptions) ([]*server.ServerAPIRaftzResponse, error)
	Profilez(opts server.ProfilezEventOptions) ([]*ProfilezResponse, error)
	CollectAccounts() ([]*server.AccountDetail, error)

	Close() error
}
