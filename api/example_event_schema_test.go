package api

import (
	"fmt"
	"reflect"
)

func Example() {
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

	// sets a location for the schemas repo
	SchemasRepo = "https://nats.io/schemas"

	stype, err := SchemaTypeForEvent([]byte(receivedEvent))
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Event Type: %s\n", stype)

	address, _, err := SchemaURLForEvent([]byte(receivedEvent))
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Event Schema URL: %s\n", address)

	address, _, err = SchemaURLForType("io.nats.jetstream.advisory.v1.api_audit")
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Type Schema URL: %s\n", address)

	schema, event, err := ParseEvent([]byte(receivedEvent))
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Parsed event with type %q into %s\n", schema, reflect.TypeOf(event))

	// Output:
	// Event Type: io.nats.jetstream.advisory.v1.api_audit
	// Event Schema URL: https://nats.io/schemas/jetstream/advisory/v1/api_audit.json
	// Type Schema URL: https://nats.io/schemas/jetstream/advisory/v1/api_audit.json
	// Parsed event with type "io.nats.jetstream.advisory.v1.api_audit" into *api.JetStreamAPIAudit
}
