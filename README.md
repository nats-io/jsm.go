## Overview

This is a Go based library to manage and interact with JetStream.

This package is the underlying library for the `nats` CLI, our Terraform provider, GitHub Actions and Kubernetes CRDs.
It's essentially a direct wrapping of the JetStream API with few userfriendly features and requires deep technical
knowledge of the JetStream internals.

For typical end users we suggest the [nats.go](https://github.com/nats-io/nats.go) package.

## Initialization

This package is modeled as a `Manager` instance that receives a NATS Connection and sets default timeouts and validation for
all interaction with JetStream.

Multiple Managers can be used in your application each with own timeouts and connection.

```go
mgr, _ := jsm.New(nc, jsm.WithTimeout(10*time.Second))
```

This creates a Manager with a 10 second timeout when accessing the JetStream API. All examples below assume a manager
was created as above.


## Schema Registry

All the JetStream API messages and some events and advisories produced by the NATS Server have JSON Schemas associated with them, the `api` package has a Schema Registry and helpers to discover and interact with these.

The Schema Registry can be accessed on the cli in the `nats schemas` command where you can list, search and view schemas and validate data based on schemas.

### Example Message

To retrieve the Stream State for a specific Stream one accesses the `$JS.API.STREAM.INFO.<stream>` API, this will respond with data like below:

```json
{
  "type": "io.nats.jetstream.api.v1.stream_info_response",
  "config": {
    "name": "TESTING",
    "subjects": [
      "js.in.testing"
    ],
    "retention": "limits",
    "max_consumers": -1,
    "max_msgs": -1,
    "max_bytes": -1,
    "discard": "old",
    "max_age": 0,
    "max_msg_size": -1,
    "storage": "file",
    "num_replicas": 1,
    "duplicate_window": 120000000000
  },
  "created": "2020-10-09T12:40:07.648216464Z",
  "state": {
    "messages": 1,
    "bytes": 81,
    "first_seq": 1017,
    "first_ts": "2020-10-09T19:43:40.867729419Z",
    "last_seq": 1017,
    "last_ts": "2020-10-09T19:43:40.867729419Z",
    "consumer_count": 1
  }
}
```

Here the type of the message is `io.nats.jetstream.api.v1.stream_info_response`, the API package can help parse this into the correct format.

### Message Schemas

Given a message kind one can retrieve the full JSON Schema as bytes:

```go
schema, _ := api.Schema("io.nats.jetstream.api.v1.stream_info_response")
```

Once can also retrieve it based on a specific message content:

```go
schemaType, _ := api.SchemaTypeForMessage(m.Data)
schema, _ := api.Schema(schemaType)
```

Several other Schema related helpers exist to search Schemas, fine URLs and more.  See the `api` [![Reference](https://pkg.go.dev/badge/github.com/nats.io/jsm.go/api)](https://pkg.go.dev/github.com/nats-io/jsm.go/api).

### Parsing Message Content

JetStream will produce metrics about message Acknowledgments, API audits and more, here we subscribe to the metric subject and print a specific received message type.

```go
nc.Subscribe("$JS.EVENT.ADVISORY.>", func(m *nats.Msg){
    kind, msg, _ := api.ParseMessage(m.Data)
    log.Printf("Received message of type %s", kind) // io.nats.jetstream.advisory.v1.api_audit
    
    switch e := event.(type){
    case advisory.JetStreamAPIAuditV1:
        fmt.Printf("Audit event on subject %s from %s\n", e.Subject, e.Client.Name)                
    }
})
```

Above we gain full access to all contents of the message in it's native format, but we need to know in advance what we will get, we can render the messages as text in a generic way though:

```go
nc.Subscribe("$JS.EVENT.ADVISORY.>", func(m *nats.Msg){
    kind, msg, _ := api.ParseMessage(m.Data)

    if kind == "io.nats.unknown_message" {
        return // a message without metadata or of a unknown format was received
    }

    ne, ok := event.(api.Event)
    if !ok {
        return fmt.Errorf("event %q does not implement the Event interface", kind)
    }

    err = api.RenderEvent(os.Stdout, ne, api.TextCompactFormat)
    if err != nil {
        return fmt.Errorf("display failed: %s", err)
    }
})
```

This will produce output like:

```
11:25:49 [JS API] $JS.API.STREAM.INFO.TESTING $G
11:25:52 [JS API] $JS.API.STREAM.NAMES $G
11:25:52 [JS API] $JS.API.STREAM.NAMES $G
11:25:53 [JS API] $JS.API.STREAM.INFO.TESTING $G
```

The `api.TextCompactFormat` is one of a few we support, also `api.TextExtendedFormat` for a full multi line format, `api.ApplicationCloudEventV1Format` for CloudEvents v1 format and `api.ApplicationJSONFormat` for JSON.

## API Validation

The data structures sent to JetStream can be validated before submission to NATS which can speed up user feedback and
provide better errors.

```go
type SchemaValidator struct{}

func (v SchemaValidator) ValidateStruct(data interface{}, schemaType string) (ok bool, errs []string) {
	s, err := api.Schema(schemaType)
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
```

This is a `api.StructValidator` implementation that uses JSON Schema to do deep validation of the structures sent to JetStream.

This can be used by the `Manager` to validate all API access.

```go
mgr, _ := jsm.New(nc, jsm.WithAPIValidation(new(SchemaValidator)))
```
