// Copyright 2026 The NATS Authors
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

package svcbackend

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// allEndpoints lists every endpoint name EndpointSchemaV1 must
// resolve; kept in sync with endpointDefs by TestEndpointDefsMatchAll.
var allEndpoints = []string{
	"sys.xkey",
	"ctx.load",
	"ctx.save",
	"ctx.delete",
	"ctx.list",
	"sel.get",
	"sel.set",
	"sel.clear",
}

func TestSchemaV1Parses(t *testing.T) {
	raw := SchemaV1()
	if len(raw) == 0 {
		t.Fatalf("expected non-empty schema bytes")
	}

	var doc map[string]any
	err := json.Unmarshal(raw, &doc)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := doc["$defs"].(map[string]any); !ok {
		t.Fatalf("schema missing $defs map")
	}
}

func TestSchemaV1ReturnsDefensiveCopy(t *testing.T) {
	a := SchemaV1()
	b := SchemaV1()

	if !bytes.Equal(a, b) {
		t.Fatalf("two calls returned different bytes")
	}

	a[0] = 0
	c := SchemaV1()
	if bytes.Equal(a, c) {
		t.Fatalf("mutating returned slice affected subsequent call")
	}
}

func TestEndpointSchemaV1All(t *testing.T) {
	for _, name := range allEndpoints {
		t.Run(name, func(t *testing.T) {
			es, err := EndpointSchemaV1(name)
			if err != nil {
				t.Fatalf("endpoint schema: %v", err)
			}

			if es.Request == "" {
				t.Fatalf("empty Request")
			}
			if es.Response == "" {
				t.Fatalf("empty Response")
			}

			for role, doc := range map[string]string{"request": es.Request, "response": es.Response} {
				var parsed map[string]any
				err = json.Unmarshal([]byte(doc), &parsed)
				if err != nil {
					t.Fatalf("%s not valid JSON: %v", role, err)
				}

				schemaURI, _ := parsed["$schema"].(string)
				if schemaURI != "https://json-schema.org/draft/2020-12/schema" {
					t.Fatalf("%s $schema = %q", role, schemaURI)
				}

				ref, _ := parsed["$ref"].(string)
				if !strings.HasPrefix(ref, "#/$defs/") {
					t.Fatalf("%s $ref = %q", role, ref)
				}

				defs, ok := parsed["$defs"].(map[string]any)
				if !ok {
					t.Fatalf("%s missing $defs", role)
				}

				target := strings.TrimPrefix(ref, "#/$defs/")
				if _, ok := defs[target]; !ok {
					t.Fatalf("%s $defs missing target %q", role, target)
				}
			}
		})
	}
}

func TestEndpointSchemaV1Unknown(t *testing.T) {
	_, err := EndpointSchemaV1("ctx.nope")
	if err == nil {
		t.Fatalf("expected error for unknown endpoint")
	}
}

func TestEndpointDefsMatchAll(t *testing.T) {
	if len(endpointDefs) != len(allEndpoints) {
		t.Fatalf("endpointDefs (%d) and allEndpoints (%d) drift", len(endpointDefs), len(allEndpoints))
	}
	for _, name := range allEndpoints {
		if _, ok := endpointDefs[name]; !ok {
			t.Fatalf("endpointDefs missing %q", name)
		}
	}
}

// TestEnvelopesMatchSchemaStructurally marshals a populated sample of
// every envelope struct and asserts the resulting JSON object is
// compatible with its $defs schema: every required field is present
// and no unknown fields appear.
func TestEnvelopesMatchSchemaStructurally(t *testing.T) {
	err := parseSchema()
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	cases := []struct {
		defName string
		sample  any
	}{
		{"SysXKeyRequest", SysXKeyRequest{}},
		{"SysXKeyResponse", SysXKeyResponse{XKeyPub: "XAAAA"}},
		{"LoadRequest", LoadRequest{ReplyPub: "XAAAA", ReqID: "rid"}},
		{"LoadResponse", LoadResponse{Sealed: "c2VhbA=="}},
		{"LoadSealed", LoadSealed{Data: []byte("x"), ReqID: "rid"}},
		{"SaveRequest", SaveRequest{SenderPub: "XAAAA", Sealed: "c2VhbA=="}},
		{"SaveResponse", SaveResponse{}},
		{"SaveSealed", SaveSealed{Data: []byte("x"), ReqID: "rid"}},
		{"DeleteRequest", DeleteRequest{ReqID: "rid"}},
		{"DeleteResponse", DeleteResponse{}},
		{"ListRequest", ListRequest{}},
		{"ListResponse", ListResponse{Names: []string{"a", "b"}}},
		{"SelectedRequest", SelectedRequest{}},
		{"SelectedResponse", SelectedResponse{Name: "active"}},
		{"SetSelectedRequest", SetSelectedRequest{ReqID: "rid"}},
		{"SetSelectedResponse", SetSelectedResponse{Previous: "old"}},
		{"ClearSelectedRequest", ClearSelectedRequest{ReqID: "rid"}},
		{"ClearSelectedResponse", ClearSelectedResponse{Previous: "old"}},
	}

	for _, tc := range cases {
		t.Run(tc.defName, func(t *testing.T) {
			raw, err := json.Marshal(tc.sample)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var obj map[string]any
			err = json.Unmarshal(raw, &obj)
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			def, ok := schemaDefs[tc.defName].(map[string]any)
			if !ok {
				t.Fatalf("schema missing $defs/%s", tc.defName)
			}

			required, _ := def["required"].([]any)
			for _, r := range required {
				key, _ := r.(string)
				if _, present := obj[key]; !present {
					t.Fatalf("required field %q missing from marshaled sample", key)
				}
			}

			additional, hasAP := def["additionalProperties"].(bool)
			if hasAP && !additional {
				props, _ := def["properties"].(map[string]any)
				for field := range obj {
					if _, known := props[field]; !known {
						t.Fatalf("unknown field %q in marshaled sample for %s", field, tc.defName)
					}
				}
			}
		})
	}
}

// TestErrorResponseMatchesSchema covers the Error def indirectly, by
// marshaling an ErrorResponse and checking structure. Kept separate
// because ErrorResponse is carried inside most envelopes rather than
// appearing at the top level of a request/response root.
func TestErrorResponseMatchesSchema(t *testing.T) {
	err := parseSchema()
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	raw, err := json.Marshal(ErrorResponse{Code: string(CodeNotFound), Message: "missing"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var obj map[string]any
	err = json.Unmarshal(raw, &obj)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	def, _ := schemaDefs["Error"].(map[string]any)
	required, _ := def["required"].([]any)
	for _, r := range required {
		key, _ := r.(string)
		if _, present := obj[key]; !present {
			t.Fatalf("required field %q missing", key)
		}
	}
}
