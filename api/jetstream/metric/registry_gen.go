// auto generated 2026-08-28 10:23:33.161965 +0200 CEST m=+0.863712709
package metric

import (
	"github.com/nats-io/jsm.go/registry/validator"
	scfs "github.com/nats-io/jsm.go/schemas"
)

// Validate performs a JSON Schema validation of the configuration
func (t ConsumerAckMetricV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.jetstream.metric.v1.consumer_ack
func (t ConsumerAckMetricV1) SchemaType() string {
	return "io.nats.jetstream.metric.v1.consumer_ack"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t ConsumerAckMetricV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/jetstream/metric/v1/consumer_ack.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t ConsumerAckMetricV1) Schema() ([]byte, error) {
	return scfs.Load("jetstream/metric/v1/consumer_ack.json")
}
