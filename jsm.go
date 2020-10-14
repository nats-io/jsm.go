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

// Package jsm provides client helpers for managing and interacting with NATS JetStream
package jsm

//go:generate go run api/gen.go

import (
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go/api"
)

// standard api responses with error embedded
type jetStreamResponseError interface {
	ToError() error
}

// jetstream iterable responses
type apiIterableResponse interface {
	ItemsTotal() int
	ItemsOffset() int
	ItemsLimit() int
	LastPage() bool
}

// jetstream iterable requests
type apiIterableRequest interface {
	SetOffset(o int)
}

// all types generated using the api/gen.go which includes all
// the jetstream api types.  Validate() will force validator all
// of these on every jsonRequest
type apiValidatable interface {
	Validate(...api.StructValidator) (valid bool, errors []string)
	SchemaType() string
}

// IsErrorResponse checks if the message holds a standard JetStream error
// TODO: parse for error response
func IsErrorResponse(m *nats.Msg) bool {
	return strings.HasPrefix(string(m.Data), api.ErrPrefix)
}

// ParseErrorResponse parses the JetStream response, if it's an error returns an error instance holding the message else nil
// TODO: parse json error response
func ParseErrorResponse(m *nats.Msg) error {
	if !IsErrorResponse(m) {
		return nil
	}

	return fmt.Errorf(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(string(m.Data), api.ErrPrefix), " '"), "'"))
}

// IsOKResponse checks if the message holds a standard JetStream error
// TODO: parse json responses
func IsOKResponse(m *nats.Msg) bool {
	return strings.HasPrefix(string(m.Data), api.OK)
}

// IsValidName verifies if n is a valid stream, template or consumer name
func IsValidName(n string) bool {
	if n == "" {
		return false
	}

	return !strings.ContainsAny(n, ">*. ")
}
