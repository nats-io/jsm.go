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

package natscontext_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nats-io/jsm.go/natscontext"
)

// Build a fresh context entirely from options, without reading
// anything from disk. Pass load=true to load a stored context by
// name from the default XDG file backend instead.
func ExampleNew() {
	c, err := natscontext.New("demo", false,
		natscontext.WithServerURL("nats://example.net:4222"),
		natscontext.WithUser("alice"),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(c.ServerURL())
	// Output: nats://example.net:4222
}

// Load a context from a specific JSON file, bypassing the default
// XDG directory. Useful for ad-hoc tooling that ships its own
// context file.
func ExampleNewFromFile() {
	dir, err := os.MkdirTemp("", "natscontext-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "demo.json")
	err = os.WriteFile(path, []byte(`{"url":"nats://example.net:4222"}`), 0o600)
	if err != nil {
		panic(err)
	}

	c, err := natscontext.NewFromFile(path)
	if err != nil {
		panic(err)
	}

	fmt.Println(c.ServerURL())
	// Output: nats://example.net:4222
}

// Register a custom CredentialResolver alongside the stock set.
// Passing any resolver option to NewRegistry suppresses automatic
// default-resolver registration, so pair WithCredentialResolver with
// WithDefaultResolvers to keep file://, op://, nsc://, env://, and
// data: handlers in place. At connect time the context's Token is
// dispatched through exampleResolver because its URI scheme is
// "example".
func ExampleWithCredentialResolver() {
	dir, err := os.MkdirTemp("", "natscontext-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	reg := natscontext.NewRegistry(
		natscontext.NewFileBackendAt(dir),
		natscontext.WithDefaultResolvers(),
		natscontext.WithCredentialResolver(&exampleResolver{}),
	)

	c, err := natscontext.New("demo", false,
		natscontext.WithServerURL("nats://example.net:4222"),
		natscontext.WithToken("example://token-ref"),
	)
	if err != nil {
		panic(err)
	}

	err = reg.Save(context.Background(), c, "demo")
	if err != nil {
		panic(err)
	}
}

type exampleResolver struct{}

func (exampleResolver) Schemes() []string { return []string{"example"} }

func (exampleResolver) Resolve(_ context.Context, _ string) ([]byte, error) {
	return []byte("resolved-token"), nil
}
