package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	scfs "github.com/nats-io/jsm.go/schemas"
)

// SchemasRepo is the repository holding NATS Schemas
var SchemasRepo = "https://raw.githubusercontent.com/nats-io/jsm.go/master/schemas"

// UnknownMessage is a type returned when parsing an unknown type of event
type UnknownMessage = map[string]any

// Event is a generic NATS Event capable of being converted to CloudEvents format
type Event interface {
	EventType() string
	EventID() string
	EventTime() time.Time
	EventSource() string
	EventSubject() string
	EventTemplate(kind string) (*template.Template, error)
}

// StructValidator is used to validate API structures
type StructValidator interface {
	ValidateStruct(data any, schemaType string) (ok bool, errs []string)
}

// RenderFormat indicates the format to render templates in
type RenderFormat string

const (
	// TextCompactFormat renders a single line view of an event
	TextCompactFormat RenderFormat = "text/compact"
	// TextExtendedFormat renders a multi line full view of an event
	TextExtendedFormat RenderFormat = "text/extended"
	// ApplicationJSONFormat renders as indented JSON
	ApplicationJSONFormat RenderFormat = "application/json"
	// ApplicationCloudEventV1Format renders as a ApplicationCloudEventV1Format v1
	ApplicationCloudEventV1Format RenderFormat = "application/cloudeventv1"
)

var wellKnownSubjectSchemas = map[string]string{
	JSApiStreamCreate:           "io.nats.jetstream.api.v1.stream_create_request",
	JSApiStreamUpdate:           "io.nats.jetstream.api.v1.stream_create_request",
	JSApiStreamNames:            "io.nats.jetstream.api.v1.stream_names_request",
	JSApiStreamList:             "io.nats.jetstream.api.v1.stream_list_request",
	JSApiStreamInfo:             "io.nats.jetstream.api.v1.stream_info_request",
	JSApiStreamPurge:            "io.nats.jetstream.api.v1.stream_purge_request",
	JSApiMsgGet:                 "io.nats.jetstream.api.v1.stream_msg_get_request",
	JSApiStreamSnapshot:         "io.nats.jetstream.api.v1.stream_snapshot_request",
	JSApiStreamRestore:          "io.nats.jetstream.api.v1.stream_restore_request",
	JSApiStreamRemovePeer:       "io.nats.jetstream.api.v1.stream_remove_peer_request",
	JSApiConsumerCreate:         "io.nats.jetstream.api.v1.consumer_create_request",
	JSApiConsumerCreateWithName: "io.nats.jetstream.api.v1.consumer_create_request",
	JSApiDurableCreate:          "io.nats.jetstream.api.v1.consumer_create_request",
	JSApiConsumerCreateEx:       "io.nats.jetstream.api.v1.consumer_create_request",
	JSApiConsumerNames:          "io.nats.jetstream.api.v1.consumer_names_request",
	JSApiConsumerList:           "io.nats.jetstream.api.v1.consumer_list_request",
	JSApiRequestNext:            "io.nats.jetstream.api.v1.consumer_getnext_request",
}

// we dont export this since it's not official, but what this produce will be loadable by the official CE
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

type schemaDetector struct {
	Schema string `json:"schema"`
	Type   string `json:"type"`
}

// IsNatsSchemaType determines if a schema type is a valid NATS type.
// The logic here is currently quite naive while we learn what works best
func IsNatsSchemaType(schemaType string) bool {
	return strings.HasPrefix(schemaType, "io.nats.")
}

// SchemaTypeForWellKnownRequestSubject searches well known subjects, like the JetStream API, and return a schema type that match requests made to it
func SchemaTypeForWellKnownRequestSubject(subject string) string {
	for k, v := range wellKnownSubjectSchemas {
		if SubjectIsSubsetMatch(subject, k) {
			return v
		}
	}

	return ""
}

// SchemaForWellKnownRequestSubject searches well known subjects, like the JetStream API, and return the schema if known
func SchemaForWellKnownRequestSubject(subject string) (schema []byte, err error) {
	st := SchemaTypeForWellKnownRequestSubject(subject)
	if st == "" {
		return nil, fmt.Errorf("subject not known")
	}

	return Schema(st)
}

// SchemaSearch searches all known schemas using a regular expression f
func SchemaSearch(f string) ([]string, error) {
	if f == "" {
		f = "."
	}

	r, err := regexp.Compile(f)
	if err != nil {
		return nil, err
	}

	var found []string
	for s := range schemaTypes {
		if r.MatchString(s) {
			found = append(found, s)
		}
	}

	sort.Strings(found)

	return found, nil
}

// SchemaURL parses a typed message m and determines a http address for the JSON schema describing it rooted in SchemasRepo
func SchemaURL(m []byte) (address string, url *url.URL, err error) {
	schema, err := SchemaTypeForMessage(m)
	if err != nil {
		return "", nil, err
	}

	return SchemaURLForType(schema)
}

// SchemaURLForType determines the path to the JSON Schema document describing a typed message given a token like io.nats.jetstream.metric.v1.consumer_ack
func SchemaURLForType(schemaType string) (address string, url *url.URL, err error) {
	if !IsNatsSchemaType(schemaType) {
		return "", nil, fmt.Errorf("unsupported schema type %q", schemaType)
	}

	token := strings.TrimPrefix(schemaType, "io.nats.")
	address = fmt.Sprintf("%s/%s.json", SchemasRepo, strings.ReplaceAll(token, ".", "/"))
	url, err = url.Parse(address)

	return address, url, err
}

// SchemaTypeForMessage retrieves the schema token from a typed message byte stream
// it does this by doing a small JSON unmarshal and is probably not the fastest.
//
// Returns the schema io.nats.unknown_message for unknown messages
func SchemaTypeForMessage(e []byte) (schemaType string, err error) {
	sd := &schemaDetector{}
	err = json.Unmarshal(e, sd)
	if err != nil {
		return "", err
	}

	if sd.Schema == "" && sd.Type == "" {
		sd.Type = "io.nats.unknown_message"
	}

	if sd.Schema != "" && sd.Type == "" {
		sd.Type = sd.Schema
	}

	return sd.Type, nil
}

// Schema returns the JSON schema for a NATS specific Schema type like io.nats.jetstream.advisory.v1.api_audit
func Schema(schemaType string) (schema []byte, err error) {
	path, err := SchemaFileForType(schemaType)
	if err != nil {
		return nil, err
	}

	schema, err = scfs.Load(path)
	if err != nil {
		return nil, err
	}

	return schema, nil
}

// NewMessage creates a new instance of the structure matching schema. When unknown creates a UnknownMessage
func NewMessage(schemaType string) (any, bool) {
	gf, ok := schemaTypes[schemaType]
	if !ok {
		gf = schemaTypes["io.nats.unknown_message"]
	}

	return gf(), ok
}

// ParseMessage parses a typed message m and returns event as for example *api.ConsumerAckMetric, all unknown
// event schemas will be of type *UnknownMessage
func ParseMessage(m []byte) (schemaType string, msg any, err error) {
	schemaType, err = SchemaTypeForMessage(m)
	if err != nil {
		return "", nil, err
	}

	msg, _ = NewMessage(schemaType)
	err = json.Unmarshal(m, msg)

	return schemaType, msg, err
}

// ParseAndValidateMessage parses the data using ParseMessage() and validates it against the detected schema. Will panic with a nil validator.
func ParseAndValidateMessage(m []byte, validator StructValidator) (schemaType string, msg any, err error) {
	schemaType, msg, err = ParseMessage(m)
	if err != nil {
		return "", nil, err
	}

	ok, errs := validator.ValidateStruct(msg, schemaType)
	if !ok {
		return schemaType, nil, errors.New(strings.Join(errs, ","))
	}

	return schemaType, msg, nil
}

// ToCloudEventV1 turns a NATS Event into a version 1.0 Cloud Event
func ToCloudEventV1(e Event) ([]byte, error) {
	je, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return nil, err
	}

	event := cloudEvent{
		Type:        e.EventType(),
		Time:        e.EventTime(),
		ID:          e.EventID(),
		Source:      e.EventSource(),
		Subject:     e.EventSubject(),
		SpecVersion: "1.0",
		Data:        je,
	}

	address, _, err := SchemaURLForType(e.EventType())
	if err == nil {
		event.DataSchema = address
	}

	return json.MarshalIndent(event, "", "  ")
}

// RenderEvent renders an event in specific format
func RenderEvent(wr io.Writer, e Event, format RenderFormat) error {
	switch format {
	case TextCompactFormat, TextExtendedFormat:
		t, err := e.EventTemplate(string(format))
		if err != nil {
			return err
		}

		return t.Execute(wr, e)

	case ApplicationJSONFormat:
		j, err := json.MarshalIndent(e, "", "  ")
		if err != nil {
			return err
		}

		_, err = wr.Write(j)
		return err

	case ApplicationCloudEventV1Format:
		ce, err := ToCloudEventV1(e)
		if err != nil {
			return err
		}

		_, err = wr.Write(ce)
		return err

	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

// SchemaFileForType determines what file on the file system to load for a particular schema type
func SchemaFileForType(schemaType string) (path string, err error) {
	if !IsNatsSchemaType(schemaType) {
		return "", fmt.Errorf("unsupported schema type %q", schemaType)
	}

	token := strings.TrimPrefix(schemaType, "io.nats.")
	return fmt.Sprintf("%s.json", strings.ReplaceAll(token, ".", "/")), nil
}

const (
	btsep = '.'
	fwc   = '>'
	pwc   = '*'
)

// SubjectIsSubsetMatch tests if a subject matches a standard nats wildcard
func SubjectIsSubsetMatch(subject, test string) bool {
	tsa := [32]string{}
	tts := tokenizeSubjectIntoSlice(tsa[:0], subject)
	return isSubsetMatch(tts, test)
}

// This will test a subject as an array of tokens against a test subject
// Calls into the function isSubsetMatchTokenized
func isSubsetMatch(tokens []string, test string) bool {
	tsa := [32]string{}
	tts := tokenizeSubjectIntoSlice(tsa[:0], test)
	return isSubsetMatchTokenized(tokens, tts)
}

// use similar to append. meaning, the updated slice will be returned
func tokenizeSubjectIntoSlice(tts []string, subject string) []string {
	start := 0
	for i := 0; i < len(subject); i++ {
		if subject[i] == btsep {
			tts = append(tts, subject[start:i])
			start = i + 1
		}
	}
	tts = append(tts, subject[start:])
	return tts
}

// This will test a subject as an array of tokens against a test subject (also encoded as array of tokens)
// and determine if the tokens are matched. Both test subject and tokens
// may contain wildcards. So foo.* is a subset match of [">", "*.*", "foo.*"],
// but not of foo.bar, etc.
func isSubsetMatchTokenized(tokens, test []string) bool {
	// Walk the target tokens
	for i, t2 := range test {
		if i >= len(tokens) {
			return false
		}
		l := len(t2)
		if l == 0 {
			return false
		}
		if t2[0] == fwc && l == 1 {
			return true
		}
		t1 := tokens[i]

		l = len(t1)
		if l == 0 || t1[0] == fwc && l == 1 {
			return false
		}

		if t1[0] == pwc && len(t1) == 1 {
			m := t2[0] == pwc && len(t2) == 1
			if !m {
				return false
			}
			if i >= len(test) {
				return true
			}
			continue
		}
		if t2[0] != pwc && strings.Compare(t1, t2) != 0 {
			return false
		}
	}
	return len(tokens) == len(test)
}
