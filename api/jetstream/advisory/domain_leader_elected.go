package advisory

import (
	"github.com/nats-io/jsm.go/api/event"
)

// JSDomainLeaderElectedV1 is a advisory published when a domain leader is elected
//
// NATS Schema Type io.nats.jetstream.advisory.v1.domain_leader_elected
type JSDomainLeaderElectedV1 struct {
	event.NATSEvent

	Leader   string        `json:"leader"`
	Cluster  string        `json:"cluster"`
	Domain   string        `json:"domain,omitempty"`
	Replicas []*PeerInfoV1 `json:"replicas"`
}

func init() {
	err := event.RegisterTextCompactTemplate("io.nats.jetstream.advisory.v1.domain_leader_elected", `{{ .Time | ShortTime }} [RAFT] Domain leader elected {{ .Leader }} in cluster {{.Cluster}} leader of {{ .Replicas | len }} peers`)
	if err != nil {
		panic(err)
	}

	err = event.RegisterTextExtendedTemplate("io.nats.jetstream.advisory.v1.domain_leader_elected", `
[{{ .Time | ShortTime }}] [{{ .ID }}] Domain Leader Election

        Leader: {{ .Leader }}
       Cluster: {{ .Cluster }}
{{- if .Domain }}
        Domain: {{ .Domain }}
{{- end }}
      Replicas:
{{ range .Replicas }}
             Name: {{ .Name }}
          Current: {{ .Current }}
           Active: {{ .Active }}
{{ end }}
`)
	if err != nil {
		panic(err)
	}
}
