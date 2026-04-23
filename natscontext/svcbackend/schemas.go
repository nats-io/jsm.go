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
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

//go:embed schema.v1.json
var schemaV1 []byte

// EndpointSchema carries the per-endpoint request and response JSON
// Schema documents as JSON strings. Each document is a standalone
// 2020-12 schema with the relevant $defs inlined, so a consumer can
// validate without resolving external $refs.
//
// This mirrors the shape nats.go's micro package would use for
// endpoint schema metadata, but nats.go's micro package does not
// currently expose such a type. A local type keeps the protocol
// package free of an external coupling we cannot guarantee.
type EndpointSchema struct {
	Request  string
	Response string
}

// endpointDef names the schema types for one endpoint. Request and
// Response name entries in the root schema's $defs.
type endpointDef struct {
	request  string
	response string
}

// endpointDefs is the authoritative mapping from endpoint name to
// request and response schema def. The endpoint names match the
// subject suffix used by a conforming server.
var endpointDefs = map[string]endpointDef{
	"sys.xkey":   {"SysXKeyRequest", "SysXKeyResponse"},
	"ctx.load":   {"LoadRequest", "LoadResponse"},
	"ctx.save":   {"SaveRequest", "SaveResponse"},
	"ctx.delete": {"DeleteRequest", "DeleteResponse"},
	"ctx.list":   {"ListRequest", "ListResponse"},
	"sel.get":    {"SelectedRequest", "SelectedResponse"},
	"sel.set":    {"SetSelectedRequest", "SetSelectedResponse"},
	"sel.clear":  {"ClearSelectedRequest", "ClearSelectedResponse"},
}

// sealedContext names the sealed-plaintext schema that should be
// inlined alongside the named def even though nothing in the wire
// shape formally $refs it. Non-Go consumers still need to know the
// plaintext structure to build or parse the Sealed payload.
var sealedContext = map[string]string{
	"LoadResponse": "LoadSealed",
	"SaveRequest":  "SaveSealed",
}

var (
	schemaOnce    sync.Once
	schemaDefs    map[string]any
	schemaBaseID  string
	schemaParseEr error
)

// SchemaV1 returns a fresh copy of the full JSON Schema document for
// protocol v1. The returned slice may be freely modified by the
// caller; the embedded canonical bytes are never exposed directly.
func SchemaV1() []byte {
	out := make([]byte, len(schemaV1))
	copy(out, schemaV1)
	return out
}

// EndpointSchemaV1 returns the EndpointSchema for a single endpoint.
// Endpoint names match the subject suffix: "sys.xkey", "ctx.load",
// "ctx.save", "ctx.delete", "ctx.list", "sel.get", "sel.set",
// "sel.clear". The returned .Request / .Response are standalone JSON
// Schema documents with the relevant $defs inlined so a consumer can
// validate without resolving external $refs.
func EndpointSchemaV1(endpoint string) (EndpointSchema, error) {
	err := parseSchema()
	if err != nil {
		return EndpointSchema{}, err
	}

	def, ok := endpointDefs[endpoint]
	if !ok {
		return EndpointSchema{}, fmt.Errorf("svcbackend: unknown endpoint %q", endpoint)
	}

	req, err := buildEndpointDoc(endpoint, "request", def.request)
	if err != nil {
		return EndpointSchema{}, err
	}

	resp, err := buildEndpointDoc(endpoint, "response", def.response)
	if err != nil {
		return EndpointSchema{}, err
	}

	return EndpointSchema{Request: req, Response: resp}, nil
}

// parseSchema parses the embedded schema.v1.json once and caches the
// top-level document, the $defs map, and the base $id. Errors are
// cached and returned unchanged on subsequent calls.
func parseSchema() error {
	schemaOnce.Do(func() {
		var doc map[string]any
		err := json.Unmarshal(schemaV1, &doc)
		if err != nil {
			schemaParseEr = fmt.Errorf("svcbackend: parse embedded schema: %w", err)
			return
		}

		defsRaw, ok := doc["$defs"].(map[string]any)
		if !ok {
			schemaParseEr = fmt.Errorf("svcbackend: embedded schema has no $defs map")
			return
		}

		id, _ := doc["$id"].(string)

		schemaDefs = defsRaw
		schemaBaseID = id
	})
	return schemaParseEr
}

// buildEndpointDoc synthesizes a standalone schema document for
// rootDef and any transitively referenced $defs. role is "request"
// or "response" and is used only to derive the $id suffix.
func buildEndpointDoc(endpoint, role, rootDef string) (string, error) {
	_, ok := schemaDefs[rootDef]
	if !ok {
		return "", fmt.Errorf("svcbackend: schema has no $defs/%s", rootDef)
	}

	inlined := map[string]any{}
	err := collectRefs(rootDef, inlined)
	if err != nil {
		return "", err
	}

	extra, ok := sealedContext[rootDef]
	if ok {
		err = collectRefs(extra, inlined)
		if err != nil {
			return "", err
		}
	}

	doc := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id":     fmt.Sprintf("%s/%s/%s", strings.TrimSuffix(schemaBaseID, "/"), endpoint, role),
		"$ref":    "#/$defs/" + rootDef,
		"$defs":   inlined,
	}

	out, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("svcbackend: marshal endpoint schema: %w", err)
	}

	return string(out), nil
}

// collectRefs recursively inlines the def named name and every def
// transitively reachable through #/$defs/ $refs into out.
func collectRefs(name string, out map[string]any) error {
	_, done := out[name]
	if done {
		return nil
	}

	def, ok := schemaDefs[name]
	if !ok {
		return fmt.Errorf("svcbackend: schema has no $defs/%s", name)
	}

	out[name] = def

	for _, ref := range extractRefs(def) {
		target, ok := strings.CutPrefix(ref, "#/$defs/")
		if !ok {
			continue
		}
		err := collectRefs(target, out)
		if err != nil {
			return err
		}
	}

	return nil
}

// extractRefs walks v and returns every string value found under a
// "$ref" key. Order is deterministic so callers see stable output.
func extractRefs(v any) []string {
	seen := map[string]struct{}{}
	walkRefs(v, seen)

	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

func walkRefs(v any, seen map[string]struct{}) {
	switch node := v.(type) {
	case map[string]any:
		ref, ok := node["$ref"].(string)
		if ok {
			seen[ref] = struct{}{}
		}
		for _, child := range node {
			walkRefs(child, seen)
		}
	case []any:
		for _, child := range node {
			walkRefs(child, seen)
		}
	}
}
