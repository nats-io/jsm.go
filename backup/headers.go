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

package backup

import (
	"strings"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// the server exports only the PURGE operation value; nats.go keeps DEL unexported
const kvOperationDelete = "DEL"

// headerValues collects the values of every header whose name matches
// case-insensitively, the way filter names are typed; server defined headers
// are looked up exact-case with Get, as the server and nats.go do
func headerValues(h nats.Header, name string) []string {
	var out []string
	for k, v := range h {
		if strings.EqualFold(k, name) {
			out = append(out, v...)
		}
	}
	return out
}

// isTombstone mirrors nats.go's KV entry decoding: a KV-Operation header
// decides when present, otherwise a limit marker reason does
func isTombstone(h nats.Header) bool {
	if op := h.Get(server.KVOperation); op != "" {
		return op == kvOperationDelete || op == string(server.KVOperationValuePurge)
	}
	switch h.Get(server.JSMarkerReason) {
	case server.JSMarkerReasonMaxAge, server.JSMarkerReasonPurge, server.JSMarkerReasonRemove:
		return true
	}
	return false
}
