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
	"bytes"
	"encoding/json"
	"testing"

	"github.com/nats-io/jsm.go/api"
)

// hasNextPage decides whether to request another page from the array length of the current one.
// A server omits an empty collection entirely (`omitempty`), so the page landing exactly on
// `total` carries no key at all. That is end-of-data, not a malformed response, and a full page
// is what makes it reachable: any endpoint holding an exact multiple of pageLimit gets there.
func TestHasNextPage(t *testing.T) {
	g := &gather{log: newLogger(&bytes.Buffer{}, api.InfoLevel)}

	const pageLimit = 1024

	// page marshals a body whose list holds n objects, matching the shape of a real response.
	page := func(key string, n int) string {
		items := make([]any, n)
		for i := range items {
			items[i] = map[string]any{}
		}
		b, err := json.Marshal(map[string]any{"data": map[string]any{key: items}})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return string(b)
	}

	for _, tc := range []struct {
		name     string
		endpoint string
		body     string
		wantMore bool
		wantErr  bool
	}{
		// The regression. Verbatim shape of a real SUBSZ page at offset == total: the paging
		// fields are present and the list key is gone.
		{
			"subsz exhausted page omits the list", "SUBSZ",
			`{"data":{"num_subscriptions":65,"total":1024,"offset":1024,"limit":1024}}`, false, false,
		},
		// Same shape on JSZ, whose account_details is likewise omitempty.
		{
			"jsz exhausted page omits the list", "JSZ",
			`{"data":{"streams":0,"total":1024,"offset":1024,"limit":1024}}`, false, false,
		},
		// A field without omitempty holding a nil slice marshals to null. Same zero-items signal.
		{"null list", "SUBSZ", `{"data":{"subscriptions_list":null}}`, false, false},
		// A field without omitempty holding an empty slice. This is what CONNZ does today.
		{"empty list", "CONNZ", `{"data":{"connections":[]}}`, false, false},
		{"short page ends paging", "SUBSZ", page("subscriptions_list", pageLimit-1), false, false},
		{"full page continues paging", "SUBSZ", page("subscriptions_list", pageLimit), true, false},

		// Errors that must survive the fix, so a mis-specified endpointPagingInfo path is still
		// reported rather than silently truncating collection to a single page.
		{"missing intermediate segment", "SUBSZ", `{"nope":{}}`, false, true},
		{"non-object mid-path", "SUBSZ", `{"data":"scalar"}`, false, true},
		{"non-array at end of path", "SUBSZ", `{"data":{"subscriptions_list":"scalar"}}`, false, true},

		// Endpoints absent from endpointPagingInfo are not paged at all.
		{"unpaged endpoint", "VARZ", `{"data":{}}`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var decoded map[string]any
			if err := json.Unmarshal([]byte(tc.body), &decoded); err != nil {
				t.Fatalf("bad fixture: %v", err)
			}

			more, err := g.hasNextPage(tc.endpoint, decoded, pageLimit)
			switch {
			case tc.wantErr && err == nil:
				t.Fatalf("expected an error, got none (more=%v)", more)
			case !tc.wantErr && err != nil:
				t.Fatalf("unexpected error: %v", err)
			}
			if more != tc.wantMore {
				t.Fatalf("hasNextPage = %v, want %v", more, tc.wantMore)
			}
		})
	}
}
