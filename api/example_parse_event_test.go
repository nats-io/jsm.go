package api

import (
	"fmt"

	jsadvisory "github.com/nats-io/jsm.go/api/jetstream/advisory"
)

func ExampleParseEvent() {
	// receivedEvent was received over a transport like NATS, webhook or other medium
	schema, event, err := ParseEvent([]byte(receivedEvent))
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("Event Type: %s\n", schema)
	switch e := event.(type) {
	case *jsadvisory.JetStreamAPIAuditV1:
		fmt.Printf("API Audit: subject: %s in account %s", e.Subject, e.Client.Account)

	default:
		fmt.Printf("Unknown event type %s\n", schema)
		fmt.Printf("%#v\n", e)
	}

	// Output:
	// Event Type: io.nats.jetstream.advisory.v1.api_audit
	// API Audit: subject: $JS.STREAM.LIST in account $G
}
