package registry_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
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
	setSchemasRepo(t, "https://nats.io/schemas")

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
	setSchemasRepo(t, "https://nats.io/schemas")

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
	setSchemasRepo(t, "https://nats.io/schemas")

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

// setSchemasRepo points the registry at repo for the duration of the test
func setSchemasRepo(t *testing.T, repo string) {
	t.Helper()

	orig := registry.SchemasRepo
	t.Cleanup(func() { registry.SchemasRepo = orig })

	registry.SchemasRepo = repo
}

type fakeValidator struct {
	ok        bool
	errs      []string
	gotData   any
	gotType   string
	callCount int
}

func (v *fakeValidator) ValidateStruct(data any, schemaType string) (bool, []string) {
	v.callCount++
	v.gotData = data
	v.gotType = schemaType

	return v.ok, v.errs
}

// Every registered type has to be constructible, know its own schema type and be able to
// load the schema document it names, this catches drift between the registry and the
// generated helpers in the api packages
func TestRegisteredTypes(t *testing.T) {
	types, err := registry.SchemaSearch("")
	checkErr(t, err, "search failed")

	if len(types) < 20 {
		t.Fatalf("did not find enough schemas, got %d", len(types))
	}

	for _, schemaType := range types {
		msg, ok := registry.NewMessage(schemaType)
		if !ok {
			t.Errorf("%s: no factory registered", schemaType)
			continue
		}

		path, err := registry.SchemaFileForType(schemaType)
		if err != nil {
			t.Errorf("%s: %s", schemaType, err)
			continue
		}

		schema, err := registry.Schema(schemaType)
		if err != nil {
			t.Errorf("%s: %s", schemaType, err)
			continue
		}

		if len(schema) == 0 {
			t.Errorf("%s: empty schema", schemaType)
		}

		managed, ok := msg.(registry.SchemaManagedType)
		if !ok {
			// micro types come from nats.go and carry no generated helpers
			if !strings.HasPrefix(schemaType, "io.nats.micro.") {
				t.Errorf("%s: expected SchemaManagedType got %T", schemaType, msg)
			}
			continue
		}

		if managed.SchemaType() != schemaType {
			t.Errorf("%s: reports schema type %q", schemaType, managed.SchemaType())
		}

		own, err := managed.Schema()
		if err != nil {
			t.Errorf("%s: %s", schemaType, err)
			continue
		}

		if !bytes.Equal(own, schema) {
			t.Errorf("%s: Schema() does not match the registry schema", schemaType)
		}

		if !strings.HasSuffix(managed.SchemaID(), "/"+path) {
			t.Errorf("%s: SchemaID %q does not end in %q", schemaType, managed.SchemaID(), path)
		}
	}
}

func TestNewMessageUnknown(t *testing.T) {
	msg, ok := registry.NewMessage("io.nats.does.not.exist")
	if ok {
		t.Fatal("expected an unknown type")
	}

	if _, ok := msg.(*registry.UnknownMessage); !ok {
		t.Fatalf("expected *registry.UnknownMessage got %T", msg)
	}
}

func TestParseMessageUnknown(t *testing.T) {
	schemaType, msg, err := registry.ParseMessage([]byte(`{"type":"io.nats.does.not.exist","hello":"world"}`))
	checkErr(t, err, "parse failed")

	if schemaType != "io.nats.does.not.exist" {
		t.Fatalf("expected io.nats.does.not.exist got %q", schemaType)
	}

	unknown, ok := msg.(*registry.UnknownMessage)
	if !ok {
		t.Fatalf("expected *registry.UnknownMessage got %T", msg)
	}

	if (*unknown)["hello"] != "world" {
		t.Fatalf("invalid body: %v", *unknown)
	}
}

func TestParseMessageInvalidJSON(t *testing.T) {
	_, _, err := registry.ParseMessage([]byte(`{`))
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestSchemaTypeForMessage(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		expected string
		err      bool
	}{
		{"type wins over schema", `{"schema":"io.nats.a","type":"io.nats.b"}`, "io.nats.b", false},
		{"schema when no type", `{"schema":"io.nats.a"}`, "io.nats.a", false},
		{"unknown when empty", `{}`, "io.nats.unknown_message", false},
		{"unknown when both empty", `{"schema":"","type":""}`, "io.nats.unknown_message", false},
		{"invalid json", `not json`, "", true},
		{"wrong field type", `{"type":1}`, "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schemaType, err := registry.SchemaTypeForMessage([]byte(tc.body))
			if tc.err {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}

			checkErr(t, err, "schema extract failed")

			if schemaType != tc.expected {
				t.Fatalf("expected %q got %q", tc.expected, schemaType)
			}
		})
	}
}

func TestIsNatsSchemaType(t *testing.T) {
	if !registry.IsNatsSchemaType("io.nats.jetstream.metric.v1.consumer_ack") {
		t.Fatal("expected a nats type")
	}

	if registry.IsNatsSchemaType("com.example.thing") {
		t.Fatal("expected a non nats type")
	}
}

func TestSchemaFileForTypeUnsupported(t *testing.T) {
	_, err := registry.SchemaFileForType("com.example.thing")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestSchemaURLInvalidMessage(t *testing.T) {
	_, _, err := registry.SchemaURL([]byte(`{`))
	if err == nil {
		t.Fatal("expected an error")
	}

	_, _, err = registry.SchemaURL([]byte(`{"type":"com.example.thing"}`))
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestSchemaUnknownType(t *testing.T) {
	_, err := registry.Schema("com.example.thing")
	if err == nil {
		t.Fatal("expected an error")
	}

	_, err = registry.Schema("io.nats.does.not.exist")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestSchemaSearchInvalidExpression(t *testing.T) {
	_, err := registry.SchemaSearch("[")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestSchemaSearchNoMatches(t *testing.T) {
	found, err := registry.SchemaSearch("this_will_never_match")
	checkErr(t, err, "search failed")

	if len(found) != 0 {
		t.Fatalf("expected no matches got %v", found)
	}
}

func TestTypeForJetStreamRequestSubjectPrefixUnknown(t *testing.T) {
	_, err := registry.TypeForJetStreamRequestSubjectPrefix("$JS.API.NOPE")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestTypeForJetStreamResponseSubjectPrefixUnknown(t *testing.T) {
	_, err := registry.TypeForJetStreamResponseSubjectPrefix("$JS.API.NOPE")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestTypesForJetStreamSubjectPrefixUnknown(t *testing.T) {
	_, _, err := registry.TypesForJetStreamSubjectPrefix("$JS.API.NOPE")
	if err == nil {
		t.Fatal("expected an error")
	}

	// registered as a request but not as a response
	_, _, err = registry.TypesForJetStreamSubjectPrefix("$JS.API.CONSUMER.CREATE.")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestTypeForRequestSubjectUnknown(t *testing.T) {
	_, err := registry.TypeForRequestSubject("nope.nope.nope")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestToCloudEventV1UnknownSchemaType(t *testing.T) {
	ja := jsadvisory.JetStreamAPIAuditV1{}
	err := json.Unmarshal([]byte(jetStreamAPIAuditEvent), &ja)
	checkErr(t, err, "could not unmarshal event")

	ja.Type = "com.example.v1.thing"

	ce, err := registry.ToCloudEventV1(&ja)
	checkErr(t, err, "could not create cloud event")

	event := &cloudEvent{}
	err = json.Unmarshal(ce, event)
	checkErr(t, err, "could not unmarshal event")

	if event.DataSchema != "" {
		t.Fatalf("expected an empty data schema got %q", event.DataSchema)
	}

	if event.Type != "com.example.v1.thing" {
		t.Fatalf("invalid type: %s", event.Type)
	}
}

func TestParseAndValidateMessage(t *testing.T) {
	t.Run("nil validator", func(t *testing.T) {
		_, _, err := registry.ParseAndValidateMessage([]byte(jetStreamAPIAuditEvent), nil)
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("valid message", func(t *testing.T) {
		v := &fakeValidator{ok: true}

		schemaType, msg, err := registry.ParseAndValidateMessage([]byte(jetStreamAPIAuditEvent), v)
		checkErr(t, err, "parse failed")

		if schemaType != "io.nats.jetstream.advisory.v1.api_audit" {
			t.Fatalf("expected io.nats.jetstream.advisory.v1.api_audit got %q", schemaType)
		}

		if _, ok := msg.(*jsadvisory.JetStreamAPIAuditV1); !ok {
			t.Fatalf("expected *advisory.JetStreamAPIAuditV1 got %T", msg)
		}

		if v.callCount != 1 {
			t.Fatalf("expected the validator to be called once, got %d", v.callCount)
		}

		if v.gotType != schemaType {
			t.Fatalf("validator got schema type %q", v.gotType)
		}

		if v.gotData != msg {
			t.Fatalf("validator got %v", v.gotData)
		}
	})

	t.Run("invalid message", func(t *testing.T) {
		v := &fakeValidator{ok: false, errs: []string{"first problem", "second problem"}}

		schemaType, msg, err := registry.ParseAndValidateMessage([]byte(jetStreamAPIAuditEvent), v)
		if err == nil {
			t.Fatal("expected an error")
		}

		if err.Error() != "first problem,second problem" {
			t.Fatalf("invalid error: %s", err)
		}

		if msg != nil {
			t.Fatalf("expected no message got %v", msg)
		}

		if schemaType != "io.nats.jetstream.advisory.v1.api_audit" {
			t.Fatalf("expected io.nats.jetstream.advisory.v1.api_audit got %q", schemaType)
		}
	})

	t.Run("undetectable schema type", func(t *testing.T) {
		v := &fakeValidator{ok: true}

		_, _, err := registry.ParseAndValidateMessage([]byte(`{`), v)
		if err == nil {
			t.Fatal("expected an error")
		}

		if v.callCount != 0 {
			t.Fatal("expected the validator not to be called")
		}
	})

	// the schema type is known before the body fails to parse, callers need it to report the failure
	t.Run("unparsable body", func(t *testing.T) {
		v := &fakeValidator{ok: true}

		schemaType, _, err := registry.ParseAndValidateMessage([]byte(`{"type":"io.nats.jetstream.advisory.v1.api_audit","timestamp":"not a time"}`), v)
		if err == nil {
			t.Fatal("expected an error")
		}

		if schemaType != "io.nats.jetstream.advisory.v1.api_audit" {
			t.Fatalf("expected io.nats.jetstream.advisory.v1.api_audit got %q", schemaType)
		}
	})
}

func TestRenderEvent(t *testing.T) {
	setSchemasRepo(t, "https://nats.io/schemas")

	ja := &jsadvisory.JetStreamAPIAuditV1{}
	err := json.Unmarshal([]byte(jetStreamAPIAuditEvent), ja)
	checkErr(t, err, "could not unmarshal event")

	render := func(t *testing.T, e registry.Event, format registry.RenderFormat) string {
		t.Helper()

		buf := &bytes.Buffer{}
		err := registry.RenderEvent(buf, e, format)
		checkErr(t, err, "render failed")

		return buf.String()
	}

	t.Run(string(registry.TextCompactFormat), func(t *testing.T) {
		out := render(t, ja, registry.TextCompactFormat)
		if !strings.Contains(out, "[JS API] $JS.STREAM.LIST") {
			t.Fatalf("invalid compact render: %q", out)
		}
	})

	t.Run(string(registry.TextExtendedFormat), func(t *testing.T) {
		out := render(t, ja, registry.TextExtendedFormat)
		if !strings.Contains(out, "JetStream API Access") {
			t.Fatalf("invalid extended render: %q", out)
		}
		if !strings.Contains(out, "uafvZ1UEDIW5FZV6kvLgWA") {
			t.Fatalf("invalid extended render: %q", out)
		}
	})

	t.Run(string(registry.ApplicationJSONFormat), func(t *testing.T) {
		out := render(t, ja, registry.ApplicationJSONFormat)

		parsed := &jsadvisory.JetStreamAPIAuditV1{}
		err := json.Unmarshal([]byte(out), parsed)
		checkErr(t, err, "could not unmarshal render")

		if !reflect.DeepEqual(parsed, ja) {
			t.Fatalf("invalid json render: %q", out)
		}
	})

	t.Run(string(registry.ApplicationCloudEventV1Format), func(t *testing.T) {
		out := render(t, ja, registry.ApplicationCloudEventV1Format)

		event := &cloudEvent{}
		err := json.Unmarshal([]byte(out), event)
		checkErr(t, err, "could not unmarshal render")

		if event.SpecVersion != "1.0" {
			t.Fatalf("invalid spec version: %s", event.SpecVersion)
		}

		if event.DataSchema != "https://nats.io/schemas/jetstream/advisory/v1/api_audit.json" {
			t.Fatalf("invalid schema address: %s", event.DataSchema)
		}
	})

	t.Run("unsupported format", func(t *testing.T) {
		err := registry.RenderEvent(&bytes.Buffer{}, ja, registry.RenderFormat("text/nonsense"))
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("unknown template", func(t *testing.T) {
		unknown := &jsadvisory.JetStreamAPIAuditV1{}
		unknown.Type = "io.nats.does.not.exist"

		err := registry.RenderEvent(&bytes.Buffer{}, unknown, registry.TextCompactFormat)
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}
