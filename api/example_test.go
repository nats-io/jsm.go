package api

import (
	"fmt"
	"reflect"
)

const receivedEvent = `
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

func Example() {
	// sets a location for the schemas repo
	SchemasRepo = "https://nats.io/schemas"

	// receivedEvent was received over a transport like NATS, webhook or other medium
	stype, err := SchemaTypeForEvent([]byte(receivedEvent))
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Event Type: %s\n", stype)

	// parses the received event and extracts the type, determines the url to fetch a schema
	address, uri, err := SchemaURLForEvent([]byte(receivedEvent))
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Event Schema URL: %s (%s)\n", address, uri.Host)

	// determines the url to fetch for a specific schema kind
	address, uri, err = SchemaURLForType("io.nats.jetstream.advisory.v1.api_audit")
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Type Schema URL: %s (%s)\n", address, uri.Host)

	// parses an event into it a type if supported else map[string]interface{}
	schema, event, err := ParseEvent([]byte(receivedEvent))
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("Parsed event with type %q into %s\n", schema, reflect.TypeOf(event))

	// Output:
	// Event Type: io.nats.jetstream.advisory.v1.api_audit
	// Event Schema URL: https://nats.io/schemas/jetstream/advisory/v1/api_audit.json (nats.io)
	// Type Schema URL: https://nats.io/schemas/jetstream/advisory/v1/api_audit.json (nats.io)
	// Parsed event with type "io.nats.jetstream.advisory.v1.api_audit" into *advisory.JetStreamAPIAuditV1
}
