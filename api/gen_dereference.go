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

//go:build ignore

// Dereferences the source JSON schemas resolving all definitions and producing
// flat JSON schema files that are easy to load remotely and validate as they are
// standalone single files.
//
// Numbers are carried through as literals so that values like the maximum of a
// Go uint64 survive the round trip unchanged.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	sourceDir       = "schema_source"
	targetDir       = "schemas"
	definitionsFile = "definitions.json"
	indent          = "  "
)

// object is a JSON object that remembers the order its keys were added in,
// encoding/json can not be used for this since it sorts map keys on output
type object struct {
	keys []string
	vals map[string]any
}

func newObject() *object {
	return &object{vals: make(map[string]any)}
}

func (o *object) set(k string, v any) {
	_, known := o.vals[k]
	if !known {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func (o *object) get(k string) (any, bool) {
	v, ok := o.vals[k]
	return v, ok
}

func (o *object) has(k string) bool {
	_, ok := o.vals[k]
	return ok
}

// loader reads and caches the parsed source schemas keyed on their path
// relative to sourceDir
type loader struct {
	docs map[string]any
}

func newLoader() *loader {
	return &loader{docs: make(map[string]any)}
}

func (l *loader) load(file string) (any, error) {
	doc, known := l.docs[file]
	if known {
		return doc, nil
	}

	body, err := os.ReadFile(filepath.Join(sourceDir, filepath.FromSlash(file)))
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()

	doc, err = decode(dec)
	if err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", file, err)
	}

	l.docs[file] = doc

	return doc, nil
}

func decode(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	return decodeValue(dec, tok)
}

func decodeValue(dec *json.Decoder, tok json.Token) (any, error) {
	delim, isDelim := tok.(json.Delim)
	if !isDelim {
		return tok, nil
	}

	switch delim {
	case '{':
		obj := newObject()

		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return nil, err
			}

			key, isString := keyTok.(string)
			if !isString {
				return nil, fmt.Errorf("expected an object key, got %v", keyTok)
			}

			val, err := decode(dec)
			if err != nil {
				return nil, err
			}

			obj.set(key, val)
		}

		_, err := dec.Token()
		if err != nil {
			return nil, err
		}

		return obj, nil

	case '[':
		arr := []any{}

		for dec.More() {
			val, err := decode(dec)
			if err != nil {
				return nil, err
			}

			arr = append(arr, val)
		}

		_, err := dec.Token()
		if err != nil {
			return nil, err
		}

		return arr, nil

	default:
		return nil, fmt.Errorf("unexpected delimiter %v", delim)
	}
}

// reference is a resolved $ref pointing at a location in a specific source file
type reference struct {
	file    string
	pointer []string
}

func (r reference) String() string {
	return r.file + "#/" + strings.Join(r.pointer, "/")
}

// parseRef resolves ref relative to the file it appears in
func parseRef(base string, ref string) (reference, error) {
	file, fragment, _ := strings.Cut(ref, "#")

	if file == "" {
		file = base
	} else {
		file = path.Join(path.Dir(base), file)
	}

	var pointer []string
	for part := range strings.SplitSeq(fragment, "/") {
		if part == "" {
			continue
		}

		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")
		pointer = append(pointer, part)
	}

	if len(pointer) == 0 {
		return reference{}, fmt.Errorf("%q in %s does not reference a definition", ref, base)
	}

	return reference{file: file, pointer: pointer}, nil
}

func (l *loader) resolve(ref reference) (any, error) {
	node, err := l.load(ref.file)
	if err != nil {
		return nil, err
	}

	for _, part := range ref.pointer {
		obj, isObject := node.(*object)
		if !isObject {
			return nil, fmt.Errorf("%s does not resolve, %q is not in an object", ref, part)
		}

		val, known := obj.get(part)
		if !known {
			return nil, fmt.Errorf("%s does not resolve, %q is unknown", ref, part)
		}

		node = val
	}

	return node, nil
}

// dereference replaces every $ref below node with the contents of what it
// references. Keys alongside a $ref are kept in their original order and win
// over the same key in the referenced definition, keys only present in the
// definition are appended.
//
// base is the file node was read from, stack holds the references being
// resolved so that a definition cycle is reported rather than recursed into.
func (l *loader) dereference(node any, base string, stack []reference) (any, error) {
	switch val := node.(type) {
	case []any:
		res := make([]any, len(val))

		for i, item := range val {
			item, err := l.dereference(item, base, stack)
			if err != nil {
				return nil, err
			}

			res[i] = item
		}

		return res, nil

	case *object:
		res := newObject()

		for _, key := range val.keys {
			if key == "$ref" {
				continue
			}

			item, err := l.dereference(val.vals[key], base, stack)
			if err != nil {
				return nil, err
			}

			res.set(key, item)
		}

		rawRef, known := val.get("$ref")
		if !known {
			return res, nil
		}

		refStr, isString := rawRef.(string)
		if !isString {
			return nil, fmt.Errorf("$ref in %s is not a string", base)
		}

		ref, err := parseRef(base, refStr)
		if err != nil {
			return nil, err
		}

		for _, seen := range stack {
			if seen.String() == ref.String() {
				return nil, fmt.Errorf("definition cycle resolving %s", ref)
			}
		}

		target, err := l.resolve(ref)
		if err != nil {
			return nil, err
		}

		target, err = l.dereference(target, ref.file, append(stack, ref))
		if err != nil {
			return nil, err
		}

		targetObj, isObject := target.(*object)
		if !isObject {
			return nil, fmt.Errorf("%s does not reference an object", ref)
		}

		for _, key := range targetObj.keys {
			if res.has(key) {
				continue
			}

			res.set(key, targetObj.vals[key])
		}

		return res, nil

	default:
		return node, nil
	}
}

// encode writes v in the same layout jq produces, two space indents with no
// escaping of HTML characters and numbers written exactly as they were read
func encode(buf *bytes.Buffer, v any, depth int) error {
	switch val := v.(type) {
	case *object:
		if len(val.keys) == 0 {
			buf.WriteString("{}")
			return nil
		}

		buf.WriteString("{\n")

		for i, key := range val.keys {
			if i > 0 {
				buf.WriteString(",\n")
			}

			writeIndent(buf, depth+1)
			writeString(buf, key)
			buf.WriteString(": ")

			err := encode(buf, val.vals[key], depth+1)
			if err != nil {
				return err
			}
		}

		buf.WriteString("\n")
		writeIndent(buf, depth)
		buf.WriteString("}")

		return nil

	case []any:
		if len(val) == 0 {
			buf.WriteString("[]")
			return nil
		}

		buf.WriteString("[\n")

		for i, item := range val {
			if i > 0 {
				buf.WriteString(",\n")
			}

			writeIndent(buf, depth+1)

			err := encode(buf, item, depth+1)
			if err != nil {
				return err
			}
		}

		buf.WriteString("\n")
		writeIndent(buf, depth)
		buf.WriteString("]")

		return nil

	case string:
		writeString(buf, val)
		return nil

	case json.Number:
		buf.WriteString(val.String())
		return nil

	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil

	case nil:
		buf.WriteString("null")
		return nil

	default:
		return fmt.Errorf("cannot encode %T", v)
	}
}

func writeIndent(buf *bytes.Buffer, depth int) {
	for range depth {
		buf.WriteString(indent)
	}
}

func writeString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')

	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(buf, `\u%04x`, r)
				continue
			}

			buf.WriteRune(r)
		}
	}

	buf.WriteByte('"')
}

func compile(l *loader, file string) error {
	doc, err := l.load(file)
	if err != nil {
		return err
	}

	res, err := l.dereference(doc, file, nil)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(nil)

	err = encode(buf, res, 0)
	if err != nil {
		return err
	}

	buf.WriteString("\n")

	target := filepath.Join(targetDir, filepath.FromSlash(file))

	err = os.MkdirAll(filepath.Dir(target), 0755)
	if err != nil {
		return err
	}

	return os.WriteFile(target, buf.Bytes(), 0644)
}

func sources() ([]string, error) {
	var files []string

	err := filepath.WalkDir(sourceDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(p) != ".json" {
			return nil
		}
		if d.Name() == definitionsFile {
			return nil
		}

		rel, err := filepath.Rel(sourceDir, p)
		if err != nil {
			return err
		}

		files = append(files, filepath.ToSlash(rel))

		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func main() {
	log.SetFlags(0)

	files, err := sources()
	if err != nil {
		log.Fatalf("could not list %s: %v", sourceDir, err)
	}

	l := newLoader()

	for _, file := range files {
		err = compile(l, file)
		if err != nil {
			log.Fatalf("could not compile %s: %v", file, err)
		}
	}

	log.Printf("Dereferenced %d schemas from %s into %s", len(files), sourceDir, targetDir)
}
