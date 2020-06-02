package advisory

import (
	"github.com/nats-io/jsm.go/api/event"
)

// JSSnapshotCreateAdvisoryV1 is an advisory sent after a snapshot is successfully started
//
// NATS Schema io.nats.jetstream.advisory.v1.snapshot_create
type JSSnapshotCreateAdvisoryV1 struct {
	event.NATSEvent
	Stream  string           `json:"stream"`
	NumBlks int              `json:"blocks"`
	BlkSize int              `json:"block_size"`
	Client  APIAuditClientV1 `json:"client"`
}

func init() {
	err := event.RegisterTextCompactTemplate("io.nats.jetstream.advisory.v1.snapshot_create", `{{ .Time | ShortTime }} [Snapshot Create] {{ .Stream }} {{ .NumBlks }} blocks of {{ .BlkSize }}`)
	if err != nil {
		panic(err)
	}

	err = event.RegisterTextExtendedTemplate("io.nats.jetstream.advisory.v1.snapshot_create", `
[{{ .Time | ShortTime }}] [{{ .ID }}] Stream Snapshot Created

        Stream: {{ .Stream }}
        Blocks: {{ .NumBlks }}
    Block Size: {{ .BlkSize }}
        Client:
{{- if .Client.User }}
               User: {{ .Client.User }} Account: {{ .Client.Account }}
{{- end }}
               Host: {{ HostPort .Client.Host .Client.Port }}
                CID: {{ .Client.CID }}
{{- if .Client.Name }}
               Name: {{ .Client.Name }}
{{- end }}
           Language: {{ .Client.Language }} {{ .Client.Version }}
`)
	if err != nil {
		panic(err)
	}
}
