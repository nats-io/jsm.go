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

package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go"
)

func TestParseJSMsgMetadata_New(t *testing.T) {
	cases := []struct {
		meta      string
		pending   uint64
		hasDomain bool
		errorStr  string
	}{
		{"$JS.ACK.ORDERS.NEW.1.2.3.1587466354254920000.10", 10, false, ""},
		{"$JS.ACK.DOMAIN.ORDERS.NEW.1.2.3.1587466354254920000.10", 0, true, "message metadata does not appear to be an ACK"},
		{"$JS.ACK.DOMAIN.ACCOUNT.ORDERS.NEW.1.2.3.1587466354254920000.10", 10, true, ""},
		{"$JS.ACK._.ACCOUNT.ORDERS.NEW.1.2.3.1587466354254920000.10", 10, false, ""},
		{"$JS.ACK.DOMAIN.ACCOUNT.ORDERS.NEW.1.2.3.1587466354254920000.10.future.future", 10, true, ""},
	}

	for _, tc := range cases {
		i, err := jsm.ParseJSMsgMetadata(&nats.Msg{Reply: tc.meta})
		if tc.errorStr != "" {
			if err == nil {
				t.Fatalf("expected error '%s' got nil", tc.errorStr)
			}
			if err.Error() != tc.errorStr {
				t.Fatalf("expected error '%s' got '%s'", tc.errorStr, err)
			}
			continue
		}
		checkErr(t, err, fmt.Sprintf("msg parse failed for '%s'", tc.meta))

		if i.Stream() != "ORDERS" {
			t.Fatalf("expected ORDERS got %s", i.Stream())
		}

		if i.Consumer() != "NEW" {
			t.Fatalf("expected NEW got %s", i.Consumer())
		}

		if i.Delivered() != 1 {
			t.Fatalf("expceted 1 got %d", i.Delivered())
		}

		if i.StreamSequence() != 2 {
			t.Fatalf("expceted 2 got %d", i.StreamSequence())
		}

		if i.ConsumerSequence() != 3 {
			t.Fatalf("expceted 3 got %d", i.ConsumerSequence())
		}

		ts := time.Unix(0, 1587466354254920000)
		if i.TimeStamp() != ts {
			t.Fatalf("expceted %v got %v", ts, i.TimeStamp())
		}

		if i.Pending() != tc.pending {
			t.Fatalf("expected %d got %d", tc.pending, i.Pending())
		}

		if tc.hasDomain && i.Domain() != "DOMAIN" {
			t.Fatalf("expected DOMAIN got %q", i.Domain())
		} else if !tc.hasDomain && i.Domain() != "" {
			t.Fatalf("expected no domain got %q", i.Domain())
		}
	}
}
