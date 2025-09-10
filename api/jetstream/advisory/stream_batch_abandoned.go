// Copyright 2025 The NATS Authors
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

// JSStreamBatchAbandonedAdvisoryV1 is an advisory published when the system abandons
// a batch publish operation
//
// NATS Schema: io.nats.jetstream.advisory.v1.stream_batch_abandoned
type JSStreamBatchAbandonedAdvisoryV1 struct {
	event.NATSEvent

	Account string             `json:"account,omitempty"`
	Domain  string             `json:"domain,omitempty"`
	Stream  string             `json:"stream"`
	BatchId string             `json:"batch"`
	Reason  BatchAbandonReason `json:"reason"`
}

type BatchAbandonReason string

const (
	BatchTimeout    BatchAbandonReason = "timeout"
	BatchLarge      BatchAbandonReason = "large"
	BatchIncomplete BatchAbandonReason = "incomplete"
)

func (r BatchAbandonReason) String() string {
	return string(r)
}

func init() {
	err := event.RegisterTextCompactTemplate("io.nats.jetstream.advisory.v1.stream_batch_abandoned", `{{ .Time | ShortTime }} [Batch Abandoned]{{ .Stream }} {{ if .Domain }}in domain {{.Domain}} {{ end }} abandoned batch {{ .BatchId }}: {{ .Reason }}`)
	if err != nil {
		panic(err)
	}

	err = event.RegisterTextExtendedTemplate("io.nats.jetstream.advisory.v1.stream_batch_abandoned", `
[{{ .Time | ShortTime }}] [{{ .ID }}] Publish Batch Abandoned

           Account: {{ .Account }}
            Domain: {{ .Domain }}
            Stream: {{ .Stream }}
             Batch: {{ .BatchId }}
            Reason: {{ .Reason }}`)
	if err != nil {
		panic(err)
	}
}
