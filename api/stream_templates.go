// Copyright 2020 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"github.com/xeipuuv/gojsonschema"
)

// StreamTemplateConfig is the configuration for a JetStream Stream Template
//
// NATS Schema Type io.nats.jetstream.api.v1.stream_template_configuration
type StreamTemplateConfig struct {
	Name       string        `json:"name"`
	Config     *StreamConfig `json:"config"`
	MaxStreams uint32        `json:"max_streams"`
}

// StreamTemplateInfo
type StreamTemplateInfo struct {
	Config  *StreamTemplateConfig `json:"config"`
	Streams []string              `json:"streams"`
}

// SchemaID is the url to the JSON Schema for JetStream Stream Template Configuration
func (c StreamTemplateConfig) SchemaID() string {
	return "https://nats.io/schemas/jetstream/api/v1/stream_template_configuration.json"
}

// SchemaType is the NATS schema type like io.nats.jetstream.api.v1.stream_configuration
func (c StreamTemplateConfig) SchemaType() string {
	return "io.nats.jetstream.api.v1.stream_template_configuration"
}

// Schema is a Draft 7 JSON Schema for the JetStream Stream Template Configuration
func (c StreamTemplateConfig) Schema() []byte {
	return schemas[c.SchemaType()]
}

func (c StreamTemplateConfig) Validate() (bool, []string) {
	sl := gojsonschema.NewSchemaLoader()
	sl.AddSchema("https://nats.io/schemas/jetstream/api/v1/definitions.json", gojsonschema.NewBytesLoader(schemas["io.nats.jetstream.api.v1.definitions"]))
	sl.AddSchema("https://nats.io/schemas/jetstream/api/v1/stream_configuration.json", gojsonschema.NewBytesLoader(c.Config.Schema()))
	root := gojsonschema.NewBytesLoader(c.Schema())

	js, err := sl.Compile(root)
	if err != nil {
		return false, []string{err.Error()}
	}

	doc := gojsonschema.NewGoLoader(c)

	result, err := js.Validate(doc)
	if err != nil {
		return false, []string{err.Error()}
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

const (
	JetStreamCreateTemplateT = "$JS.TEMPLATE.%s.CREATE"
	JetStreamListTemplates   = "$JS.TEMPLATE.LIST"
	JetStreamTemplateInfoT   = "$JS.TEMPLATE.%s.INFO"
	JetStreamDeleteTemplateT = "$JS.TEMPLATE.%s.DELETE"
)

type JetStreamDeleteTemplateResponse struct {
	JetStreamResponse
	Success bool `json:"success,omitempty"`
}

type JetStreamCreateTemplateResponse struct {
	JetStreamResponse
	*StreamTemplateInfo
}

type JetStreamListTemplatesResponse struct {
	JetStreamResponse
	Templates []string `json:"streams,omitempty"`
}

type JetStreamTemplateInfoResponse struct {
	JetStreamResponse
	*StreamTemplateInfo
}
