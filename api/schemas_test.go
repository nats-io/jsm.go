package api

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	jsadvisory "github.com/nats-io/jsm.go/api/jetstream/advisory"
	scfs "github.com/nats-io/jsm.go/schemas"
)

const jetStreamAPIAuditEvent = `{
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
}`

func checkErr(t *testing.T, err error, m string) {
	t.Helper()
	if err == nil {
		return
	}
	t.Fatal(m + ": " + err.Error())
}

func TestToCloudEvent(t *testing.T) {
	SchemasRepo = "https://nats.io/schemas"

	ja := jsadvisory.JetStreamAPIAuditV1{}
	err := json.Unmarshal([]byte(jetStreamAPIAuditEvent), &ja)
	if err != nil {
		t.Fatalf("could not unmarshal event: %s", err)
	}

	ce, err := ToCloudEventV1(&ja)
	if err != nil {
		t.Fatalf("could not create cloud event: %s", err)
	}

	event := &cloudEvent{}
	err = json.Unmarshal(ce, event)
	if err != nil {
		t.Fatalf("could not unmarshal event: %s", err)
	}

	if event.Type != "io.nats.jetstream.advisory.v1.api_audit" {
		t.Fatalf("invalid type: %s", event.Type)
	}

	if event.SpecVersion != "1.0" {
		t.Fatalf("invalid spec version: %s", event.SpecVersion)
	}

	if event.Source != "urn:nats:jetstream" {
		t.Fatalf("invalid event source: %s", event.Source)
	}

	if event.Subject != "advisory" {
		t.Fatalf("invalid subject: %s", event.Subject)
	}

	if event.ID != "uafvZ1UEDIW5FZV6kvLgWA" {
		t.Fatalf("invalid ID: %s", event.ID)
	}

	if event.DataSchema != "https://nats.io/schemas/jetstream/advisory/v1/api_audit.json" {
		t.Fatalf("invalid schema address: %s", event.DataSchema)
	}

	dat := jsadvisory.JetStreamAPIAuditV1{}
	err = json.Unmarshal(event.Data, &dat)
	if err != nil {
		t.Fatalf("could not unmarshal data body: %s", err)
	}

	if !reflect.DeepEqual(dat, ja) {
		t.Fatalf("invalid data: %#v", dat)
	}
}

func TestSchemaForEvent(t *testing.T) {
	s, err := SchemaTypeForMessage([]byte(`{"schema":"io.nats.jetstream.metric.v1.consumer_ack"}`))
	checkErr(t, err, "schema extract failed")

	if s != "io.nats.jetstream.metric.v1.consumer_ack" {
		t.Fatalf("expected io.nats.jetstream.metric.v1.consumer_ack got %s", s)
	}

	s, err = SchemaTypeForMessage([]byte(`{}`))
	checkErr(t, err, "schema extract failed")

	if s != "io.nats.unknown_message" {
		t.Fatalf("expected io.nats.unknown_message got %s", s)
	}
}

func TestSchemaURLForToken(t *testing.T) {
	SchemasRepo = "https://nats.io/schemas"

	a, u, err := SchemaURLForType("io.nats.jetstream.metric.v1.consumer_ack")
	checkErr(t, err, "parse failed")

	if a != "https://nats.io/schemas/jetstream/metric/v1/consumer_ack.json" {
		t.Fatalf("expected https://nats.io/schemas/jetstream/metric/v1/consumer_ack.json got %q", a)
	}

	if u.Host != "nats.io" || u.Scheme != "https" || u.Path != "/schemas/jetstream/metric/v1/consumer_ack.json" {
		t.Fatalf("invalid url: %v", u.String())
	}

	_, _, err = SchemaURLForType("jetstream.metric.v1.consumer_ack")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestSchemaURLForEvent(t *testing.T) {
	SchemasRepo = "https://nats.io/schemas"

	a, u, err := SchemaURL([]byte(`{"schema":"io.nats.jetstream.metric.v1.consumer_ack"}`))
	checkErr(t, err, "parse failed")

	if a != "https://nats.io/schemas/jetstream/metric/v1/consumer_ack.json" {
		t.Fatalf("expected . got %q", a)
	}

	if u.Host != "nats.io" || u.Scheme != "https" || u.Path != "/schemas/jetstream/metric/v1/consumer_ack.json" {
		t.Fatalf("invalid url: %v", u.String())
	}
}

func TestSchemaSearch(t *testing.T) {
	found, err := SchemaSearch("")
	checkErr(t, err, "search failed")
	if len(found) != len(schemaTypes) {
		t.Fatalf("Expected %d matched got %d", len(schemaTypes), len(found))
	}

	found, err = SchemaSearch("consumer_create")
	checkErr(t, err, "search failed")
	if len(found) != 2 {
		t.Fatalf("Expected [io.nats.jetstream.api.v1.consumer_create_request io.nats.jetstream.api.v1.consumer_create_response] got %v", found)
	}

	if found[0] != "io.nats.jetstream.api.v1.consumer_create_request" || found[1] != "io.nats.jetstream.api.v1.consumer_create_response" {
		t.Fatalf("Expected [io.nats.jetstream.api.v1.consumer_create_request io.nats.jetstream.api.v1.consumer_create_response] got %v", found)
	}
}

func TestSchema(t *testing.T) {
	schema, err := Schema("io.nats.jetstream.api.v1.stream_template_names_request")
	checkErr(t, err, "failed")

	dat, err := scfs.Load("jetstream/api/v1/stream_template_names_request.json")
	checkErr(t, err, "failed")

	if !bytes.Equal(schema, dat) {
		t.Fatalf("schemas did not match")
	}
}

func TestSchemaFileForType(t *testing.T) {
	p, err := SchemaFileForType("io.nats.jetstream.metric.v1.consumer_ack")
	checkErr(t, err, "parse failed")

	if p != "jetstream/metric/v1/consumer_ack.json" {
		t.Fatalf("invalid path %s", p)
	}
}
