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

package api

import (
	"testing"
)

func TestRequiredApiLevel(t *testing.T) {
	// a nested struct has a setting that requires a level 2 api
	req := &JSApiStreamCreateRequest{
		StreamConfig: StreamConfig{
			AllowMsgCounter: true,
		},
	}
	v, err := RequiredApiLevel(req)
	checkErr(t, err, "RequiredApiLevel failed")
	if v != 2 {
		t.Fatalf("expected level 2 got %v", v)
	}

	// here we use level 1 features
	creq := &JSApiConsumerCreateRequest{
		Config: ConsumerConfig{
			PriorityGroups: []string{"X"},
			PriorityPolicy: PriorityOverflow,
		},
	}
	v, err = RequiredApiLevel(creq)
	checkErr(t, err, "RequiredApiLevel failed")
	if v != 1 {
		t.Fatalf("expected level 1 got %v", v)
	}

	// here we set the generally level 1 feature to a value thats unique to 2
	// and so the consumer specific leveler should have been called to force this to 2
	creq = &JSApiConsumerCreateRequest{
		Config: ConsumerConfig{
			PriorityGroups: []string{"X"},
			PriorityPolicy: PriorityPrioritized,
		},
	}
	v, err = RequiredApiLevel(creq)
	checkErr(t, err, "RequiredApiLevel failed")
	if v != 2 {
		t.Fatalf("expected level 2 got %v", v)
	}

	// here we detect invalid tags
	type invalidTags struct {
		X string `api_level:"a"`
	}
	_, err = RequiredApiLevel(invalidTags{X: "y"})
	if err.Error() != `invalid api_level tag "a" for field X on type invalidTags` {
		t.Fatalf("invalid error: %v", err)
	}
}
