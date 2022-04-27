package advisory

import (
	"github.com/nats-io/jsm.go/api/event"
)

// JSConsumerDeliveryNakAdvisoryV1 is an advisory informing that a message was
// naked by the consumer
//
// NATS Schema: io.nats.jetstream.advisory.v1.nak
type JSConsumerDeliveryNakAdvisoryV1 struct {
	event.NATSEvent

	Stream      string `json:"stream"`
	Consumer    string `json:"consumer"`
	ConsumerSeq uint64 `json:"consumer_seq"`
	StreamSeq   uint64 `json:"stream_seq"`
	Deliveries  uint64 `json:"deliveries"`
	Domain      string `json:"domain,omitempty"`
}

func init() {
	err := event.RegisterTextCompactTemplate("io.nats.jetstream.advisory.v1.nak", `{{ .Time | ShortTime }} [JS Delivery NAKed] {{ .Stream }} ({{ .StreamSeq }}) > {{ .Consumer }}: {{ .Deliveries }} deliveries`)
	if err != nil {
		panic(err)
	}

	err = event.RegisterTextExtendedTemplate("io.nats.jetstream.advisory.v1.nak", `
[{{ .Time | ShortTime }}] [{{ .ID }}] Delivery NAKed

          Consumer: {{ .Stream }} > {{ .Consumer }}
   Stream Sequence: {{ .StreamSeq }}
        Deliveries: {{ .Deliveries }}
            Domain: {{ .Domain }}`)
	if err != nil {
		panic(err)
	}
}
