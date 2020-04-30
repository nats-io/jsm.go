package api

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	jsadvisory "github.com/nats-io/jsm.go/api/jetstream/advisory"
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

func validateExpectSuccess(t *testing.T, cfg validator) {
	t.Helper()

	ok, errs := cfg.Validate()
	if !ok {
		t.Fatalf("expected success but got: %v", errs)
	}
}

func validateExpectFailure(t *testing.T, cfg validator) {
	t.Helper()

	ok, errs := cfg.Validate()
	if ok {
		t.Fatalf("expected success but got: %v", errs)
	}
}

func TestValidateStruct(t *testing.T) {
	sc := StreamConfig{
		Name:         "BASIC",
		Retention:    LimitsPolicy,
		MaxConsumers: -1,
		MaxAge:       0,
		MaxBytes:     -1,
		MaxMsgs:      -1,
		Storage:      FileStorage,
		Replicas:     1,
	}

	ok, errs := ValidateStruct(sc, sc.SchemaType())
	if !ok {
		t.Fatalf("expected no errors got %v", errs)
	}

	sc.MaxMsgs = -2
	ok, errs = ValidateStruct(sc, sc.SchemaType())
	if ok || len(errs) != 1 {
		t.Fatal("expected errors got none")
	}

	ja := jsadvisory.JetStreamAPIAuditV1{}
	err := json.Unmarshal([]byte(jetStreamAPIAuditEvent), &ja)
	if err != nil {
		t.Fatalf("could not unmarshal event: %s", err)
	}

	ok, errs = ValidateStruct(ja, "io.nats.jetstream.advisory.v1.api_audit")
	if !ok {
		t.Fatalf("expected no errors got %v", errs)
	}

	ja.Type = ""
	ok, errs = ValidateStruct(ja, "io.nats.jetstream.advisory.v1.api_audit")
	if ok || len(errs) != 1 {
		t.Fatal("expected errors got none")
	}

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

func TestStreamConfiguration(t *testing.T) {
	reset := func() StreamConfig {
		return StreamConfig{
			Name:         "BASIC",
			Retention:    LimitsPolicy,
			MaxConsumers: -1,
			MaxAge:       0,
			MaxBytes:     -1,
			MaxMsgs:      -1,
			Storage:      FileStorage,
			Replicas:     1,
		}
	}

	cfg := reset()
	validateExpectSuccess(t, cfg)

	// invalid names
	cfg = reset()
	cfg.Name = "X.X"
	validateExpectFailure(t, cfg)

	// empty subject list not allowed but no subject list is allowed
	cfg = reset()
	cfg.Subjects = []string{""}
	validateExpectFailure(t, cfg)

	// valid subject
	cfg.Subjects = []string{"bob"}
	validateExpectSuccess(t, cfg)

	// invalid retention
	cfg.Retention = 10
	validateExpectFailure(t, cfg)

	// max consumers >= -1
	cfg = reset()
	cfg.MaxConsumers = -2
	validateExpectFailure(t, cfg)
	cfg.MaxConsumers = 10
	validateExpectSuccess(t, cfg)

	// max messages >= -1
	cfg = reset()
	cfg.MaxMsgs = -2
	validateExpectFailure(t, cfg)
	cfg.MaxMsgs = 10
	validateExpectSuccess(t, cfg)

	// max bytes >= -1
	cfg = reset()
	cfg.MaxBytes = -2
	validateExpectFailure(t, cfg)
	cfg.MaxBytes = 10
	validateExpectSuccess(t, cfg)

	// max age >= 0
	cfg = reset()
	cfg.MaxAge = -1
	validateExpectFailure(t, cfg)
	cfg.MaxAge = time.Second
	validateExpectSuccess(t, cfg)

	// max msg size >= -1
	cfg = reset()
	cfg.MaxMsgSize = -2
	validateExpectFailure(t, cfg)
	cfg.MaxMsgSize = 10
	validateExpectSuccess(t, cfg)

	// storage is valid
	cfg = reset()
	cfg.Storage = 10
	validateExpectFailure(t, cfg)

	// num replicas > 0
	cfg = reset()
	cfg.Replicas = -1
	validateExpectFailure(t, cfg)
	cfg.Replicas = 0
	validateExpectFailure(t, cfg)
}

func TestStreamTemplateConfiguration(t *testing.T) {
	reset := func() StreamTemplateConfig {
		return StreamTemplateConfig{
			Name:       "BASIC_T",
			MaxStreams: 10,
			Config: &StreamConfig{
				Name:         "BASIC",
				Retention:    LimitsPolicy,
				MaxConsumers: -1,
				MaxAge:       0,
				MaxBytes:     -1,
				MaxMsgs:      -1,
				Storage:      FileStorage,
				Replicas:     1,
			},
		}
	}

	cfg := reset()
	validateExpectSuccess(t, cfg)

	cfg.Name = ""
	validateExpectFailure(t, cfg)

	// should also validate config
	cfg = reset()
	cfg.Config.Storage = 10
	validateExpectFailure(t, cfg)

	// unlimited managed streams
	cfg = reset()
	cfg.MaxStreams = 0
	validateExpectSuccess(t, cfg)
}

func TestConsumerConfiguration(t *testing.T) {
	reset := func() ConsumerConfig {
		return ConsumerConfig{
			DeliverPolicy: DeliverAll,
			AckPolicy:     AckExplicit,
			ReplayPolicy:  ReplayInstant,
		}
	}

	cfg := reset()
	validateExpectSuccess(t, cfg)

	// durable name
	cfg = reset()
	cfg.Durable = "bob.bob"
	validateExpectFailure(t, cfg)

	// last policy
	cfg = reset()
	cfg.DeliverPolicy = DeliverLast
	validateExpectSuccess(t, cfg)

	// new policy
	cfg = reset()
	cfg.DeliverPolicy = DeliverNew
	validateExpectSuccess(t, cfg)

	// start sequence policy
	cfg = reset()
	cfg.DeliverPolicy = DeliverByStartSequence
	cfg.OptStartSeq = 10
	validateExpectSuccess(t, cfg)
	cfg.OptStartSeq = 0
	validateExpectFailure(t, cfg)

	// start time policy
	cfg = reset()
	ts := time.Now()
	cfg.DeliverPolicy = DeliverByStartTime
	cfg.OptStartTime = &ts
	validateExpectSuccess(t, cfg)
	cfg.OptStartTime = nil
	validateExpectFailure(t, cfg)

	// ack policy
	cfg = reset()
	cfg.AckPolicy = 10
	validateExpectFailure(t, cfg)
	cfg.AckPolicy = AckExplicit
	validateExpectSuccess(t, cfg)
	cfg.AckPolicy = AckAll
	validateExpectSuccess(t, cfg)
	cfg.AckPolicy = AckNone
	validateExpectSuccess(t, cfg)

	// replay policy
	cfg = reset()
	cfg.ReplayPolicy = 10
	validateExpectFailure(t, cfg)
	cfg.ReplayPolicy = ReplayInstant
	validateExpectSuccess(t, cfg)
	cfg.ReplayPolicy = ReplayOriginal
	validateExpectSuccess(t, cfg)
}

func TestSchemaForEvent(t *testing.T) {
	s, err := SchemaTypeForEvent([]byte(`{"schema":"io.nats.jetstream.metric.v1.consumer_ack"}`))
	checkErr(t, err, "schema extract failed")

	if s != "io.nats.jetstream.metric.v1.consumer_ack" {
		t.Fatalf("expected io.nats.jetstream.metric.v1.consumer_ack got %s", s)
	}

	s, err = SchemaTypeForEvent([]byte(`{}`))
	checkErr(t, err, "schema extract failed")

	if s != "io.nats.unknown_event" {
		t.Fatalf("expected io.nats.unknown_event got %s", s)
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

	a, u, err := SchemaURLForEvent([]byte(`{"schema":"io.nats.jetstream.metric.v1.consumer_ack"}`))
	checkErr(t, err, "parse failed")

	if a != "https://nats.io/schemas/jetstream/metric/v1/consumer_ack.json" {
		t.Fatalf("expected . got %q", a)
	}

	if u.Host != "nats.io" || u.Scheme != "https" || u.Path != "/schemas/jetstream/metric/v1/consumer_ack.json" {
		t.Fatalf("invalid url: %v", u.String())
	}
}
