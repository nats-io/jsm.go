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

package gather

import (
	"encoding/json"
	"testing"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats-server/v2/server"
)

func decodeServerResponse(t *testing.T, data any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(server.ServerAPIResponse{Data: data})
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}

	return decoded
}

func TestHasNextPage(t *testing.T) {
	const limit = 4

	cases := []struct {
		name     string
		endpoint string
		decoded  map[string]any
		want     bool
		wantErr  bool
	}{
		{"full page", "SUBSZ", decodeServerResponse(t, &server.Subsz{Subs: make([]server.SubDetail, limit)}), true, false},
		{"partial page", "SUBSZ", decodeServerResponse(t, &server.Subsz{Subs: make([]server.SubDetail, limit-1)}), false, false},
		{"SUBSZ past the end", "SUBSZ", decodeServerResponse(t, &server.Subsz{Offset: limit, Limit: limit, Total: limit}), false, false},
		{"JSZ past the end", "JSZ", decodeServerResponse(t, &server.JSInfo{Total: limit}), false, false},
		{"list is not an array", "SUBSZ", map[string]any{"data": map[string]any{"subscriptions_list": "x"}}, false, true},
	}

	g := &gather{log: newLogger(nil, api.ErrorLevel)}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := g.hasNextPage(tc.endpoint, tc.decoded, limit)
			if (err != nil) != tc.wantErr {
				t.Fatalf("hasNextPage() error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("hasNextPage() = %v, want %v", got, tc.want)
			}
		})
	}
}
