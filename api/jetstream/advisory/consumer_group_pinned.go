package advisory

import "github.com/nats-io/jsm.go/api/event"

// JSConsumerGroupPinnedAdvisoryV1 is an advisory published when a consumer pinned_client grouped consumer pins a client
//
// NATS Schema Type io.nats.jetstream.advisory.v1.consumer_group_pinned
type JSConsumerGroupPinnedAdvisoryV1 struct {
	event.NATSEvent

	Account        string `json:"account,omitempty"`
	Stream         string `json:"stream"`
	Consumer       string `json:"consumer"`
	Domain         string `json:"domain,omitempty"`
	Group          string `json:"group"`
	PinnedClientId string `json:"pinned_id"`
}

func init() {
	err := event.RegisterTextCompactTemplate("io.nats.jetstream.advisory.v1.consumer_group_pinned", `{{ .Time | ShortTime }} [PINNED] Consumer {{ .Stream }} > {{ .Consumer }} pinned client {{ .PinnedClientId }} for group {{ .Group }}`)
	if err != nil {
		panic(err)
	}

	err = event.RegisterTextExtendedTemplate("io.nats.jetstream.advisory.v1.consumer_group_pinned", `
[{{ .Time | ShortTime }}] [{{ .ID }}] Grouped Consumer Pinned a Client

        Stream: {{ .Stream }}
      Consumer: {{ .Consumer }}
         Group: {{ .Group }}
        Domain: {{ .Domain }}
 Pinned Client: {{ .PinnedClientId }}`)
	if err != nil {
		panic(err)
	}
}
