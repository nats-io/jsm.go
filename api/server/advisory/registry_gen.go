// auto generated 2026-08-28 10:23:33.186638 +0200 CEST m=+0.888385418
package advisory

import (
	"github.com/nats-io/jsm.go/registry/validator"
	scfs "github.com/nats-io/jsm.go/schemas"
)

// Validate performs a JSON Schema validation of the configuration
func (t AccountConnectionsV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.server.advisory.v1.account_connections
func (t AccountConnectionsV1) SchemaType() string {
	return "io.nats.server.advisory.v1.account_connections"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t AccountConnectionsV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/server/advisory/v1/account_connections.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t AccountConnectionsV1) Schema() ([]byte, error) {
	return scfs.Load("server/advisory/v1/account_connections.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t ConnectEventMsgV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.server.advisory.v1.client_connect
func (t ConnectEventMsgV1) SchemaType() string {
	return "io.nats.server.advisory.v1.client_connect"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t ConnectEventMsgV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/server/advisory/v1/client_connect.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t ConnectEventMsgV1) Schema() ([]byte, error) {
	return scfs.Load("server/advisory/v1/client_connect.json")
}

// Validate performs a JSON Schema validation of the configuration
func (t DisconnectEventMsgV1) Validate(v ...validator.StructValidator) (valid bool, errors []string) {
	if len(v) == 0 || v[0] == nil {
		return true, nil
	}

	return v[0].ValidateStruct(t, t.SchemaType())
}

// SchemaType is the NATS schema type io.nats.server.advisory.v1.client_disconnect
func (t DisconnectEventMsgV1) SchemaType() string {
	return "io.nats.server.advisory.v1.client_disconnect"
}

// SchemaID is the url to the JSON Schema for JetStream Consumer Configuration
func (t DisconnectEventMsgV1) SchemaID() string {
	return "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas/server/advisory/v1/client_disconnect.json"
}

// Schema is a JSON Schema document for the JetStream Consumer Configuration
func (t DisconnectEventMsgV1) Schema() ([]byte, error) {
	return scfs.Load("server/advisory/v1/client_disconnect.json")
}
