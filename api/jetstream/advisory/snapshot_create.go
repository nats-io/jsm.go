package advisory

import (
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/api/event"
	"github.com/nats-io/jsm.go/api/server/advisory"
)

// JSSnapshotCreateAdvisoryV1 is an advisory sent after a snapshot is successfully started
//
// NATS Schema io.nats.jetstream.advisory.v1.snapshot_create
type JSSnapshotCreateAdvisoryV1 struct {
	event.NATSEvent
	Stream string                `json:"stream"`
	Client advisory.ClientInfoV1 `json:"client"`
	State  api.StreamState       `json:"state"`
	Domain string                `json:"domain,omitempty"`
}

func init() {
	err := event.RegisterTextCompactTemplate("io.nats.jetstream.advisory.v1.snapshot_create", `{{ .Time | ShortTime }} [Snapshot Create] {{ .Stream }} {{ .State.Msgs | Uint64Commas }} messages {{ .State.Bytes | Uint64IBytes }}`)
	if err != nil {
		panic(err)
	}

	err = event.RegisterTextExtendedTemplate("io.nats.jetstream.advisory.v1.snapshot_create", `
[{{ .Time | ShortTime }}] [{{ .ID }}] Stream Snapshot Created

        Stream: {{ .Stream }}
      Messages: {{ .State.Msgs | Uint64Commas }}
         Bytes: {{ .State.Bytes | Uint64IBytes }}
     Sequences: {{ .State.FirstSeq | Uint64Commas }} - {{ .State.LastSeq | Uint64Commas }}
        Client:
{{- if .Client.User }}
                      User: {{ .Client.User }} Account: {{ .Client.Account }}
{{- end }}
                      Host: {{ .Client.Host }}
                       ID: {{ .Client.ID }}
{{- if .Client.Name }}
                      Name: {{ .Client.Name }}
{{- end }}
           Library Version: {{ .Client.Version }}  Language: {{ with .Client.Lang }}{{ . }}{{ else }}Unknown{{ end }}
`)
	if err != nil {
		panic(err)
	}
}
