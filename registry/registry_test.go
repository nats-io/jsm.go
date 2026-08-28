package registry_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/nats-io/jsm.go/api"
	jsadvisory "github.com/nats-io/jsm.go/api/jetstream/advisory"
	"github.com/nats-io/jsm.go/registry"
	scfs "github.com/nats-io/jsm.go/schemas"
)

type cloudEvent struct {
	Type        string          `json:"type"`
	Time        time.Time       `json:"time"`
	ID          string          `json:"id"`
	Source      string          `json:"source"`
	DataSchema  string          `json:"dataschema"`
	SpecVersion string          `json:"specversion"`
	Subject     string          `json:"subject"`
	Data        json.RawMessage `json:"data"`
}

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

func TestTypeForJetStreamRequestSubjectPrefix(t *testing.T) {
	v, err := registry.TypeForJetStreamRequestSubjectPrefix("$JS.API.STREAM.CREATE")
	checkErr(t, err, "failed")
	instance, ok := v.(registry.SchemaManagedApiRequestType)
	if !ok {
		t.Fatalf("expected SchemaManagedApiRequestType got %T", v)
	}
	if instance.SchemaType() != "io.nats.jetstream.api.v1.stream_create_request" {
		t.Fatalf("expected io.nats.jetstream.api.v1.stream_create_request got %s", instance.SchemaType())
	}
}

func TestTypeForJetStreamResponseSubjectPrefix(t *testing.T) {
	v, err := registry.TypeForJetStreamResponseSubjectPrefix("$JS.API.STREAM.CREATE")
	checkErr(t, err, "failed")
	instance, ok := v.(registry.SchemaManagedType)
	if !ok {
		t.Fatalf("expected SchemaManagedType got %T", v)
	}
	if instance.SchemaType() != "io.nats.jetstream.api.v1.stream_create_response" {
		t.Fatalf("expected io.nats.jetstream.api.v1.stream_create_response got %s", instance.SchemaType())
	}
}

func TestTypesForJetStreamSubjectPrefix(t *testing.T) {
	reqv, replyv, err := registry.TypesForJetStreamSubjectPrefix("$JS.API.STREAM.CREATE")
	checkErr(t, err, "failed")

	req, ok := reqv.(registry.SchemaManagedApiRequestType)
	if !ok {
		t.Fatalf("expected SchemaManagedApiRequestType got %T", reqv)
	}

	reply, ok := replyv.(registry.SchemaManagedType)
	if !ok {
		t.Fatalf("expected SchemaManagedType got %T", reqv)
	}

	if req.SchemaType() != "io.nats.jetstream.api.v1.stream_create_request" {
		t.Fatalf("expected io.nats.jetstream.api.v1.stream_create_request got %s", req.SchemaType())
	}
	if reply.SchemaType() != "io.nats.jetstream.api.v1.stream_create_response" {
		t.Fatalf("expected io.nats.jetstream.api.v1.stream_create_response got %s", reply.SchemaType())
	}

	cr, ok := req.(*api.JSApiStreamCreateRequest)
	if !ok {
		t.Fatalf("Invalid type received %T", req)
	}

	prefix, _ := cr.ApiSubjectPrefix()
	if prefix != "$JS.API.STREAM.CREATE" {
		t.Fatalf("expected $JS.API.STREAM.CREATE got %q", prefix)
	}

	format, _ := cr.ApiSubjectFormat()
	if format != "$JS.API.STREAM.CREATE.%s" {
		t.Fatalf("expected $JS.API.STREAM.CREATE.%%s got %q", format)
	}

	pattern, _ := cr.ApiSubjectPattern()
	if pattern != "$JS.API.STREAM.CREATE.*" {
		t.Fatalf("expected $JS.API.STREAM.CREATE.* got %q", pattern)
	}
}

func TestSchemaForRequestSubject(t *testing.T) {
	v, err := registry.TypeForRequestSubject("$JS.API.CONSUMER.CREATE.foo.bar")
	checkErr(t, err, "failed")

	req, ok := v.(registry.SchemaManagedApiRequestType)
	if !ok {
		t.Fatalf("expected SchemaManagedApiRequestType got %T", v)
	}

	if req.SchemaType() != "io.nats.jetstream.api.v1.consumer_create_request" {
		t.Fatalf("expected io.nats.jetstream.api.v1.consumer_create_request got %s", req.SchemaType())
	}

	cr, ok := req.(*api.JSApiConsumerCreateRequest)
	if !ok {
		t.Fatalf("Invalid type received %T", req)
	}

	prefix, _ := cr.ApiSubjectPrefix()
	if prefix != "$JS.API.CONSUMER.CREATE" {
		t.Fatalf("expected $JS.API.CONSUMER.CREATE got %q", prefix)
	}

	format, _ := cr.ApiSubjectFormat()
	if format != "$JS.API.CONSUMER.CREATE.%s.%s" {
		t.Fatalf("expected $JS.API.CONSUMER.CREATE.%%s.%%s got %q", format)
	}

	pattern, _ := cr.ApiSubjectPattern()
	if pattern != "$JS.API.CONSUMER.CREATE.*.>" {
		t.Fatalf("expected $JS.API.CONSUMER.CREATE.*.> got %q", pattern)
	}
}

func TestToCloudEvent(t *testing.T) {
	registry.SchemasRepo = "https://nats.io/schemas"

	ja := jsadvisory.JetStreamAPIAuditV1{}
	err := json.Unmarshal([]byte(jetStreamAPIAuditEvent), &ja)
	if err != nil {
		t.Fatalf("could not unmarshal event: %s", err)
	}

	ce, err := registry.ToCloudEventV1(&ja)
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
	s, err := registry.SchemaTypeForMessage([]byte(`{"schema":"io.nats.jetstream.metric.v1.consumer_ack"}`))
	checkErr(t, err, "schema extract failed")

	if s != "io.nats.jetstream.metric.v1.consumer_ack" {
		t.Fatalf("expected io.nats.jetstream.metric.v1.consumer_ack got %s", s)
	}

	s, err = registry.SchemaTypeForMessage([]byte(`{}`))
	checkErr(t, err, "schema extract failed")

	if s != "io.nats.unknown_message" {
		t.Fatalf("expected io.nats.unknown_message got %s", s)
	}
}

func TestSchemaURLForToken(t *testing.T) {
	registry.SchemasRepo = "https://nats.io/schemas"

	a, u, err := registry.SchemaURLForType("io.nats.jetstream.metric.v1.consumer_ack")
	checkErr(t, err, "parse failed")

	if a != "https://nats.io/schemas/jetstream/metric/v1/consumer_ack.json" {
		t.Fatalf("expected https://nats.io/schemas/jetstream/metric/v1/consumer_ack.json got %q", a)
	}

	if u.Host != "nats.io" || u.Scheme != "https" || u.Path != "/schemas/jetstream/metric/v1/consumer_ack.json" {
		t.Fatalf("invalid url: %v", u.String())
	}

	_, _, err = registry.SchemaURLForType("jetstream.metric.v1.consumer_ack")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestSchemaURLForEvent(t *testing.T) {
	registry.SchemasRepo = "https://nats.io/schemas"

	a, u, err := registry.SchemaURL([]byte(`{"schema":"io.nats.jetstream.metric.v1.consumer_ack"}`))
	checkErr(t, err, "parse failed")

	if a != "https://nats.io/schemas/jetstream/metric/v1/consumer_ack.json" {
		t.Fatalf("expected . got %q", a)
	}

	if u.Host != "nats.io" || u.Scheme != "https" || u.Path != "/schemas/jetstream/metric/v1/consumer_ack.json" {
		t.Fatalf("invalid url: %v", u.String())
	}
}

func TestSchemaSearch(t *testing.T) {
	found, err := registry.SchemaSearch("")
	checkErr(t, err, "search failed")
	if len(found) <= 20 {
		t.Fatalf("Did not find enough schemas, got %d", len(found))
	}

	found, err = registry.SchemaSearch("consumer_create")
	checkErr(t, err, "search failed")
	if len(found) != 2 {
		t.Fatalf("Expected [io.nats.jetstream.api.v1.consumer_create_request io.nats.jetstream.api.v1.consumer_create_response] got %v", found)
	}

	if found[0] != "io.nats.jetstream.api.v1.consumer_create_request" || found[1] != "io.nats.jetstream.api.v1.consumer_create_response" {
		t.Fatalf("Expected [io.nats.jetstream.api.v1.consumer_create_request io.nats.jetstream.api.v1.consumer_create_response] got %v", found)
	}
}

func TestSchema(t *testing.T) {
	schema, err := registry.Schema("io.nats.jetstream.api.v1.stream_names_request")
	checkErr(t, err, "failed")

	dat, err := scfs.Load("jetstream/api/v1/stream_names_request.json")
	checkErr(t, err, "failed")

	if !bytes.Equal(schema, dat) {
		t.Fatalf("schemas did not match")
	}
}

func TestSchemaFileForType(t *testing.T) {
	p, err := registry.SchemaFileForType("io.nats.jetstream.metric.v1.consumer_ack")
	checkErr(t, err, "parse failed")

	if p != "jetstream/metric/v1/consumer_ack.json" {
		t.Fatalf("invalid path %s", p)
	}
}
