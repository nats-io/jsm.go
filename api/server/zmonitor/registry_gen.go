// auto generated 2026-08-28 10:23:33.236583 +0200 CEST m=+0.938329293
package zmonitor

import (
	"github.com/nats-io/jsm.go/registry/validator"
	scfs "github.com/nats-io/jsm.go/schemas"
)

// Validate performs a JSON Schema validation of the configuration
func (t VarzV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.server.monitor.v1.varz
func (t VarzV1) SchemaType() string {
	return "io.nats.server.monitor.v1.varz"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t VarzV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/server/monitor/v1/varz.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t VarzV1) Schema() ([]byte, error) {
	return scfs.Load("server/monitor/v1/varz.json")
}
