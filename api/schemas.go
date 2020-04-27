package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// SchemasRepo is the repository holding NATS Schemas
var SchemasRepo = "https://raw.githubusercontent.com/nats-io/jetstream/master/schemas"

type schemaDetector struct {
	Schema string `json:"schema"`
	Type   string `json:"type"`
}

type validator interface {
	Validate() (bool, []string)
}

// IsNatsEventType determines if a event type is a valid NATS type
func IsNatsEventType(schemaType string) bool {
	return strings.HasPrefix(schemaType, "io.nats.")
}

// SchemaURLForEvent parses event e and determines a http address for the JSON schema describing it rooted in SchemasRepo
func SchemaURLForEvent(e []byte) (address string, url *url.URL, err error) {
	schema, err := SchemaTypeForEvent(e)
	if err != nil {
		return "", nil, err
	}

	return SchemaURLForType(schema)
}

// SchemaURLForType determines the path to the JSON Schema document describing an event given a token like io.nats.jetstream.metric.v1.consumer_ack
func SchemaURLForType(schemaType string) (address string, url *url.URL, err error) {
	if !IsNatsEventType(schemaType) {
		return "", nil, fmt.Errorf("unsupported schema type %q", schemaType)
	}

	token := strings.TrimPrefix(schemaType, "io.nats.")
	address = fmt.Sprintf("%s/%s.json", SchemasRepo, strings.ReplaceAll(token, ".", "/"))
	url, err = url.Parse(address)

	return address, url, err
}

// SchemaTypeForEvent retrieves the schema token from an event byte stream
// it does this by doing a small JSON unmarshal and is probably not the fastest
func SchemaTypeForEvent(e []byte) (schemaType string, err error) {
	sd := &schemaDetector{}
	err = json.Unmarshal(e, sd)
	if err != nil {
		return "", err
	}

	if sd.Schema == "" && sd.Type == "" {
		sd.Type = "io.nats.unknown_event"
	}

	if sd.Schema != "" && sd.Type == "" {
		sd.Type = sd.Schema
	}

	return sd.Type, nil
}

// Schema returns the JSON schema for a NATS specific Schema type like io.nats.jetstream.advisory.v1.api_audit
func Schema(schemaType string) (schema []byte, err error) {
	schema, ok := schemas[schemaType]
	if !ok {
		return nil, fmt.Errorf("unknown schema %s", schemaType)
	}

	return schema, nil
}

// NewEvent creates a new instance of the structure matching schema. When unknown creates a UnknownEvent
func NewEvent(schemaType string) (interface{}, bool) {
	gf, ok := schemaTypes[schemaType]
	if !ok {
		gf = schemaTypes["io.nats.unknown_event"]
	}

	return gf(), ok
}

// ValidateStruct validates data matches schemaType like io.nats.jetstream.advisory.v1.api_audit
func ValidateStruct(data interface{}, schemaType string) (ok bool, errs []string) {
	// some types have complex validation needs involving many schemas, those can
	// validate themselves so we defer to that
	v, ok := data.(validator)
	if ok {
		return v.Validate()
	}

	// other more basic types can be validated directly against their schemaType
	s, err := Schema(schemaType)
	if err != nil {
		return false, []string{"unknown schema type %s", schemaType}
	}

	ls := gojsonschema.NewBytesLoader(s)
	ld := gojsonschema.NewGoLoader(data)
	result, err := gojsonschema.Validate(ls, ld)
	if err != nil {
		return false, []string{fmt.Sprintf("validation failed: %s", err)}
	}

	if result.Valid() {
		return true, nil
	}

	errors := make([]string, len(result.Errors()))
	for i, verr := range result.Errors() {
		errors[i] = verr.String()
	}

	return false, errors
}

// ParseEvent parses event e and returns event as for example *api.ConsumerAckMetric, all unknown
// event schemas will be of type *UnknownEvent
func ParseEvent(e []byte) (schemaType string, event interface{}, err error) {
	schemaType, err = SchemaTypeForEvent(e)
	if err != nil {
		return "", nil, err
	}

	event, _ = NewEvent(schemaType)
	err = json.Unmarshal(e, event)

	return schemaType, event, err
}
