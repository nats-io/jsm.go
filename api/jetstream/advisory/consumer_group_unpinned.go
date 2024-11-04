package advisory

import "github.com/nats-io/jsm.go/api/event"

// JSConsumerGroupUnPinnedAdvisoryV1 is an advisory published when a consumer pinned_client grouped consumer unpins a client
//
// NATS Schema Type io.nats.jetstream.advisory.v1.consumer_group_unpinned
type JSConsumerGroupUnPinnedAdvisoryV1 struct {
	event.NATSEvent

	Account  string `json:"account,omitempty"`
	Stream   string `json:"stream"`
	Consumer string `json:"consumer"`
	Domain   string `json:"domain,omitempty"`
	Group    string `json:"group"`
	Reason   string `json:"reason"`
}

func init() {
	err := event.RegisterTextCompactTemplate("io.nats.jetstream.advisory.v1.consumer_group_unpinned", `{{ .Time | ShortTime }} [UNPINNED] Consumer {{ .Stream }} > {{ .Consumer }} unpinned client for group {{ .Group }}: {{ .Reason }}`)
	if err != nil {
		panic(err)
	}

	err = event.RegisterTextExtendedTemplate("io.nats.jetstream.advisory.v1.consumer_group_unpinned", `
[{{ .Time | ShortTime }}] [{{ .ID }}] Grouped Consumer Un-Pinned a Client

        Stream: {{ .Stream }}
      Consumer: {{ .Consumer }}
         Group: {{ .Group }}
        Domain: {{ .Domain }}
        Reason: {{ .Reason }}`)
	if err != nil {
		panic(err)
	}
}
