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
	"math"
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

func TestParseJSMsgMetadata_Nil(t *testing.T) {
	_, err := jsm.ParseJSMsgMetadata(nil)
	if err == nil {
		t.Fatal("expected error for nil message, got nil")
	}
}

func TestParseJSMsgMetadataReply_PendingSentinel(t *testing.T) {
	// When the pending field is not a valid number the sentinel math.MaxUint64
	// must be preserved, not silently replaced with 0.
	reply := "$JS.ACK.ORDERS.NEW.1.2.3.1587466354254920000.not-a-number"
	i, err := jsm.ParseJSMsgMetadataReply(reply)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if i.Pending() != math.MaxUint64 {
		t.Fatalf("expected math.MaxUint64 for unparseable pending, got %d", i.Pending())
	}
}

func TestParseJSMsgMetadataReply_PendingValid(t *testing.T) {
	reply := "$JS.ACK.ORDERS.NEW.1.2.3.1587466354254920000.42"
	i, err := jsm.ParseJSMsgMetadataReply(reply)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if i.Pending() != 42 {
		t.Fatalf("expected 42 got %d", i.Pending())
	}
}

func TestParseJSMsgMetadataDirect(t *testing.T) {
	hdr := nats.Header{}
	hdr.Set("Nats-Stream", "ORDERS")
	hdr.Set("Nats-Sequence", "5")
	hdr.Set("Nats-Time-Stamp", "2020-04-21T07:52:34.254920192Z")

	i, err := jsm.ParseJSMsgMetadataDirect(hdr)
	checkErr(t, err, "direct parse failed")

	if i.Stream() != "ORDERS" {
		t.Fatalf("expected ORDERS got %s", i.Stream())
	}
	if i.StreamSequence() != 5 {
		t.Fatalf("expected 5 got %d", i.StreamSequence())
	}
	if i.Consumer() != "" {
		t.Fatalf("expected empty consumer got %s", i.Consumer())
	}
	if i.Pending() != 0 {
		t.Fatalf("expected 0 pending (not set) got %d", i.Pending())
	}
}

func TestParseJSMsgMetadataDirect_WithPending(t *testing.T) {
	hdr := nats.Header{}
	hdr.Set("Nats-Stream", "ORDERS")
	hdr.Set("Nats-Sequence", "7")
	hdr.Set("Nats-Time-Stamp", "2020-04-21T07:52:34.254920192Z")
	hdr.Set("Nats-Num-Pending", "99")

	i, err := jsm.ParseJSMsgMetadataDirect(hdr)
	checkErr(t, err, "direct parse with pending failed")

	if i.Pending() != 99 {
		t.Fatalf("expected 99 got %d", i.Pending())
	}
}

func TestParseJSMsgMetadataDirect_BadSequence(t *testing.T) {
	hdr := nats.Header{}
	hdr.Set("Nats-Stream", "ORDERS")
	hdr.Set("Nats-Sequence", "not-a-number")
	hdr.Set("Nats-Time-Stamp", "2020-04-21T07:52:34.254920192Z")

	_, err := jsm.ParseJSMsgMetadataDirect(hdr)
	if err == nil {
		t.Fatal("expected error for bad sequence header, got nil")
	}
}
