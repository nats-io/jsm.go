package advisory

import (
	"time"

	"github.com/nats-io/jsm.go/api/event"
)

// JSConsumerPauseAdvisoryV1 indicates that a consumer was paused or unpaused
type JSConsumerPauseAdvisoryV1 struct {
	event.NATSEvent

	Stream     string    `json:"stream"`
	Consumer   string    `json:"consumer"`
	Paused     bool      `json:"paused"`
	PauseUntil time.Time `json:"pause_until,omitempty"`
	Domain     string    `json:"domain,omitempty"`
}

func init() {
	err := event.RegisterTextCompactTemplate("io.nats.jetstream.advisory.v1.consumer_pause", `{{ .Time | ShortTime }} [Consumer Pause] Consumer: {{ .Stream }} > {{ .Consumer }} Paused: {{ .Paused }}{{ if .Paused }} until {{ .PauseUntil }}{{ end }}`)
	if err != nil {
		panic(err)
	}

	err = event.RegisterTextExtendedTemplate("io.nats.jetstream.advisory.v1.consumer_pause", `
[{{ .Time | ShortTime }}] [{{ .ID }}] Consumer Pause

        Stream: {{ .Stream }}
      Consumer: {{ .Consumer }}
        Paused: {{ .Paused }}
{{- if .Paused }}
         Until: {{ .PauseUntil }}
{{- end }}
{{- if .Domain }}
        Domain: {{ .Domain }}
{{- end }}
`)
	if err != nil {
		panic(err)
	}
}
