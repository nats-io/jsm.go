// Copyright 2024 The NATS Authors
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

package advisory

import (
	"github.com/nats-io/jsm.go/api/event"
)

// JSAPILimitReachedAdvisoryV1 is an advisory published when the system drops incoming
// API requests in order to protect itself against denial of service.
//
// NATS Schema: io.nats.jetstream.advisory.v1.api_limit_reached
type JSAPILimitReachedAdvisoryV1 struct {
	event.NATSEvent

	Server  string `json:"server"`           // Server that created the event, name or ID
	Domain  string `json:"domain,omitempty"` // Domain the server belongs to
	Dropped int64  `json:"dropped"`          // How many messages did we drop from the queue
}

func init() {
	err := event.RegisterTextCompactTemplate("io.nats.jetstream.advisory.v1.api_limit_reached", `{{ .Time | ShortTime }} [JS API Limit] {{ .Server }} {{ if .Domain }}in domain {{.Domain}} {{ end }}discarded {{ .Dropped | Int64Commas }} messages`)
	if err != nil {
		panic(err)
	}

	err = event.RegisterTextExtendedTemplate("io.nats.jetstream.advisory.v1.api_limit_reached", `
[{{ .Time | ShortTime }}] [{{ .ID }}] API Limit Reached

           Dropped: {{ .Dropped }}
            Domain: {{ .Domain }}`)
	if err != nil {
		panic(err)
	}
}
