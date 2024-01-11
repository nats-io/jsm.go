package advisory

import (
	"github.com/nats-io/jsm.go/api/event"
)

// AccountConnectionsV1 is sent regularly reporting the state of all accounts with
// connections to a particular NATS Server
//
// NATS Schema Type io.nats.server.advisory.v1.account_connections
type AccountConnectionsV1 struct {
	event.NATSEvent

	Server        ServerInfoV1 `json:"server"`
	Account       string       `json:"acc"`
	Conns         int          `json:"conns"`
	LeafNodes     int          `json:"leafnodes"`
	TotalConns    int          `json:"total_conns"`
	Sent          DataStatsV1  `json:"sent"`
	Received      DataStatsV1  `json:"received"`
	SlowConsumers int64        `json:"slow_consumers"`
}

func init() {
	err := event.RegisterTextCompactTemplate("io.nats.server.advisory.v1.account_connections", `{{ .Time | ShortTime }} [Account] {{ .Account }} Server: {{ .Server.Name }} Connections: {{ .Conns }} Leaf Nodes: {{ .LeafNodes }} Slow: {{ .SlowConsumers |Int64Commas  }} Sent: {{ .Sent.Msgs | Int64Commas }} / {{ .Sent.Bytes | IBytes }} Received: {{ .Received.Msgs | Int64Commas }} / {{ .Received.Bytes | IBytes }}`)
	if err != nil {
		panic(err)
	}

	err = event.RegisterTextExtendedTemplate("io.nats.server.advisory.v1.account_connections", `
[{{ .Time | ShortTime }}] [{{ .ID }}] Account Status

      Account: {{ .Account }}
       Server: {{ .Server.Name }}
{{- if .Server.Cluster }}
      Cluster: {{ .Server.Cluster }}
{{- end }}
  Connections: {{ .Conns }}
   Leaf Nodes: {{ .LeafNodes }}
{{- if .Received }}

   Stats:
            Received: {{ .Received.Msgs | Int64Commas }} messages ({{ .Received.Bytes | IBytes }})
           Published: {{ .Sent.Msgs | Int64Commas}} messages ({{ .Sent.Bytes | IBytes }})
      Slow Consumers: {{ .SlowConsumers |Int64Commas  }}
{{- end }}`)
	if err != nil {
		panic(err)
	}
}
