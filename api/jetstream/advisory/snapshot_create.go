package advisory

import (
	"time"

	"github.com/nats-io/jsm.go/api/event"
	"github.com/nats-io/jsm.go/api/server/advisory"
)

// LostStreamDataV1 indicates msgs that have been lost
type LostStreamDataV1 struct {
	Msgs  []uint64 `json:"msgs"`
	Bytes uint64   `json:"bytes"`
}

// StreamStateV1 duplicates api.StreamState which cannot be used here since the
// api package imports this one
type StreamStateV1 struct {
	Msgs        uint64            `json:"messages"`
	Bytes       uint64            `json:"bytes"`
	FirstSeq    uint64            `json:"first_seq"`
	FirstTime   time.Time         `json:"first_ts"`
	LastSeq     uint64            `json:"last_seq"`
	LastTime    time.Time         `json:"last_ts"`
	NumSubjects int               `json:"num_subjects,omitempty"`
	Subjects    map[string]uint64 `json:"subjects,omitempty"`
	NumDeleted  int               `json:"num_deleted,omitempty"`
	Deleted     []uint64          `json:"deleted,omitempty"`
	Lost        *LostStreamDataV1 `json:"lost,omitempty"`
	Consumers   int               `json:"consumer_count"`
}

// JSSnapshotCreateAdvisoryV1 is an advisory sent after a snapshot is successfully started
//
// NATS Schema io.nats.jetstream.advisory.v1.snapshot_create
type JSSnapshotCreateAdvisoryV1 struct {
	event.NATSEvent
	Stream  string                `json:"stream"`
	NumBlks int64                 `json:"blocks"`
	BlkSize int64                 `json:"block_size"`
	Client  advisory.ClientInfoV1 `json:"client"`
	State   StreamStateV1         `json:"state"`
	Domain  string                `json:"domain,omitempty"`
}

func init() {
	err := event.RegisterTextCompactTemplate("io.nats.jetstream.advisory.v1.snapshot_create", `{{ .Time | ShortTime }} [Snapshot Create] {{ .Stream }} {{ .NumBlks | Int64Commas }} blocks of {{ .BlkSize | IBytes }}`)
	if err != nil {
		panic(err)
	}

	err = event.RegisterTextExtendedTemplate("io.nats.jetstream.advisory.v1.snapshot_create", `
[{{ .Time | ShortTime }}] [{{ .ID }}] Stream Snapshot Created

        Stream: {{ .Stream }}
        Blocks: {{ .NumBlks | Int64Commas }}
    Block Size: {{ .BlkSize | IBytes }}
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
