// Copyright 2022-2026 The NATS Authors
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

package jsm

import (
	"testing"

	"github.com/nats-io/jsm.go/api"
)

// makeTestStream returns a minimal Stream with the given config, suitable for
// unit-testing matchers that only inspect stream.Configuration().
func makeTestStream(cfg api.StreamConfig) *Stream {
	return &Stream{cfg: &cfg}
}

func TestMatchApiLevelInvalidMetadata(t *testing.T) {
	stream := makeTestStream(api.StreamConfig{
		Metadata: map[string]string{
			api.JsMetaRequiredServerLevel: "notanumber",
		},
	})

	q := &streamQuery{apiLevel: 1}
	_, err := q.matchApiLevel([]*Stream{stream})
	if err == nil {
		t.Fatal("expected error for invalid API level metadata, got nil")
	}
}

func TestMatchApiLevelValidMetadata(t *testing.T) {
	stream := makeTestStream(api.StreamConfig{
		Metadata: map[string]string{
			api.JsMetaRequiredServerLevel: "2",
		},
	})

	q := &streamQuery{apiLevel: 1}
	matched, err := q.matchApiLevel([]*Stream{stream})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matched) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matched))
	}
}
