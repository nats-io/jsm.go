package api

import (
	"fmt"
)

func ExampleParseEvent() {
	// an event received from some medium like a NATS Subject or an event router
	receivedEvent := `
{
  "type": "io.nats.jetstream.advisory.v1.api_audit",
  "id": "uafvZ1UEDIW5FZV6kvLgWA",
  "timestamp": "2020-04-23T16:51:18.516363Z",
  "server": "NDJWE4SOUJOJT2TY5Y2YQEOAHGAK5VIGXTGKWJSFHVCII4ITI3LBHBUV",
  "client": {
    "host": "::1",
    "port": 57924,
    "cid": 17,
    "account": "$G",
    "name": "NATS CLI",
    "lang": "go",
    "version": "1.9.2"
  },
  "subject": "$JS.STREAM.LIST",
  "response": "[\n  \"ORDERS\"\n]"
}
`

	schema, event, err := ParseEvent([]byte(receivedEvent))
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("Event Type: %s\n", schema)
	switch e := event.(type) {
	case *JetStreamAPIAudit:
		fmt.Printf("API Audit: subject: %s in account %s", e.Subject, e.Client.Account)

	default:
		fmt.Printf("Unknown event type %s\n", schema)
		fmt.Printf("%#v\n", e)
	}

	// Output:
	// Event Type: io.nats.jetstream.advisory.v1.api_audit
	// API Audit: subject: $JS.STREAM.LIST in account $G
}
