package advisory

import (
	"github.com/nats-io/jsm.go/api/event"
)

// JSServerRemovedAdvisoryV1 indicates a server has been removed from the cluster.
//
// NATS Schema Type io.nats.jetstream.advisory.v1.server_removed
type JSServerRemovedAdvisoryV1 struct {
	event.NATSEvent

	Server   string `json:"server"`
	ServerID string `json:"server_id"`
	Cluster  string `json:"cluster"`
	Domain   string `json:"domain,omitempty"`
}

func init() {
	err := event.RegisterTextCompactTemplate("io.nats.jetstream.advisory.v1.server_removed", `{{ .Time | ShortTime }} [Server Removed] {{ .Server }} ({{ .ServerID }}){{ if .Cluster }} in cluster {{ .Cluster }}{{end}} was removed`)
	if err != nil {
		panic(err)
	}

	err = event.RegisterTextExtendedTemplate("io.nats.jetstream.advisory.v1.server_removed", `
[{{ .Time | ShortTime }}] [{{ .ID }}] Server Removed

        Server: {{ .Server }} - {{ .ServerID}}
       Cluster: {{ .Cluster }}
{{- if .Domain }}
        Domain: {{ .Domain }}
{{- end }}
`)
	if err != nil {
		panic(err)
	}
}
