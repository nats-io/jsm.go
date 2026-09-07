package metric

import (
	"github.com/nats-io/jsm.go/registry/validator"
	scfs "github.com/nats-io/jsm.go/schemas"
)

// Validate performs a JSON Schema validation of the configuration
func (t ServiceLatencyV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.server.metric.v1.service_latency
func (t ServiceLatencyV1) SchemaType() string {
	return "io.nats.server.metric.v1.service_latency"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t ServiceLatencyV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/server/metric/v1/service_latency.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t ServiceLatencyV1) Schema() ([]byte, error) {
	return scfs.Load("server/metric/v1/service_latency.json")
}
