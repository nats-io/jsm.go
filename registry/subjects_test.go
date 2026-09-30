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

package registry_test

import (
	"strings"
	"testing"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/registry"
)

func TestNormalizeAPISubject(t *testing.T) {
	cases := []struct {
		name    string
		subject string
		expect  string
		ok      bool
	}{
		{"canonical", "$JS.API.STREAM.INFO.ORDERS", "$JS.API.STREAM.INFO.ORDERS", true},
		{"canonical with filter", "$JS.API.CONSUMER.CREATE.ORDERS.C1.orders.new", "$JS.API.CONSUMER.CREATE.ORDERS.C1.orders.new", true},
		{"canonical no arguments", "$JS.API.STREAM.NAMES", "$JS.API.STREAM.NAMES", true},
		{"canonical not validated", "$JS.API.STREAM.INFO", "$JS.API.STREAM.INFO", true},
		{"domain", "$JS.hub.API.STREAM.INFO.ORDERS", "$JS.API.STREAM.INFO.ORDERS", true},
		{"domain named like a verb", "$JS.STREAM.API.STREAM.INFO.ORDERS", "$JS.API.STREAM.INFO.ORDERS", true},
		{"domain without subject", "$JS.hub.API", "$JS.hub.API", false},
		{"single token prefix", "foo.STREAM.INFO.ORDERS", "$JS.API.STREAM.INFO.ORDERS", true},
		{"multi token prefix", "JS.acc.API.STREAM.INFO.ORDERS", "$JS.API.STREAM.INFO.ORDERS", true},
		{"verb only known from responses", "foo.INFO", "$JS.API.INFO", true},
		{"info inside a subject", "orders.INFO.x", "orders.INFO.x", false},
		{"info without prefix", "STREAM.INFO.ORDERS", "STREAM.INFO.ORDERS", false},
		{"info before verb", "a.INFO.STREAM.INFO.ORDERS", "$JS.API.STREAM.INFO.ORDERS", true},
		{"direct get", "foo.DIRECT.GET.ORDERS", "$JS.API.DIRECT.GET.ORDERS", true},
		{"stream named like a verb", "foo.STREAM.INFO.STREAM", "$JS.API.STREAM.INFO.STREAM", true},
		{"prefix without subject", "foo.bar", "foo.bar", false},
		{"no verb", "orders.new.item", "orders.new.item", false},
		{"not an api subject", "$JS.EVENT.ADVISORY.API", "$JS.EVENT.ADVISORY.API", false},
		{"advisory", "$JS.EVENT.ADVISORY.STREAM.CREATED.ORDERS", "$JS.EVENT.ADVISORY.STREAM.CREATED.ORDERS", false},
		{"domain named like a prefix", "$JS.hub.x.API.STREAM.INFO.ORDERS", "$JS.hub.x.API.STREAM.INFO.ORDERS", false},
		{"empty", "", "", false},
		{"bare js", "$JS", "$JS", false},
		{"bare api", "$JS.API", "$JS.API", false},
		{"bare verb", "foo.STREAM", "$JS.API.STREAM", true},
		{"bare api with dot", "$JS.API.", "$JS.API.", false},
		{"empty token", "foo..STREAM.INFO.ORDERS", "foo..STREAM.INFO.ORDERS", false},
		{"empty domain", "$JS..API.STREAM.INFO.ORDERS", "$JS..API.STREAM.INFO.ORDERS", false},
		{"leading dot", ".STREAM.INFO.ORDERS", ".STREAM.INFO.ORDERS", false},
		{"trailing dot", "foo.STREAM.INFO.ORDERS.", "foo.STREAM.INFO.ORDERS.", false},
		{"star wildcard", "$JS.API.STREAM.INFO.*", "$JS.API.STREAM.INFO.*", false},
		{"full wildcard", "$JS.API.>", "$JS.API.>", false},
		{"whitespace", "foo.STREAM.INFO.OR DERS", "foo.STREAM.INFO.OR DERS", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, ok := registry.NormalizeAPISubject(tc.subject)
			if ok != tc.ok {
				t.Fatalf("expected ok %v got %v", tc.ok, ok)
			}
			if res != tc.expect {
				t.Fatalf("expected %q got %q", tc.expect, res)
			}
		})
	}
}

func TestNormalizeAPISubjectRoundTrip(t *testing.T) {
	subjects := []string{
		"$JS.API.STREAM.CREATE.ORDERS",
		"$JS.API.STREAM.UPDATE.ORDERS",
		"$JS.API.STREAM.INFO.ORDERS",
		"$JS.API.STREAM.LIST",
		"$JS.API.STREAM.NAMES",
		"$JS.API.STREAM.PURGE.ORDERS",
		"$JS.API.STREAM.SNAPSHOT.ORDERS",
		"$JS.API.STREAM.RESTORE.ORDERS",
		"$JS.API.STREAM.MSG.GET.ORDERS",
		"$JS.API.STREAM.MSG.DELETE.ORDERS",
		"$JS.API.STREAM.LEADER.STEPDOWN.ORDERS",
		"$JS.API.STREAM.PEER.REMOVE.ORDERS",
		"$JS.API.CONSUMER.CREATE.ORDERS",
		"$JS.API.CONSUMER.CREATE.ORDERS.C1",
		"$JS.API.CONSUMER.DURABLE.CREATE.ORDERS.C1",
		"$JS.API.CONSUMER.CREATE.ORDERS.C1.orders.new",
		"$JS.API.CONSUMER.INFO.ORDERS.C1",
		"$JS.API.CONSUMER.LIST.ORDERS",
		"$JS.API.CONSUMER.NAMES.ORDERS",
		"$JS.API.CONSUMER.MSG.NEXT.ORDERS.C1",
		"$JS.API.CONSUMER.LEADER.STEPDOWN.ORDERS.C1",
		"$JS.API.CONSUMER.PAUSE.ORDERS.C1",
		"$JS.API.META.LEADER.STEPDOWN",
		"$JS.API.SERVER.REMOVE",
		"$JS.API.INFO",
		"$JS.API.ACCOUNT.PURGE.acc",
		"$JS.API.ACCOUNT.STREAM.MOVE.acc.ORDERS",
		"$JS.API.ACCOUNT.STREAM.CANCEL_MOVE.acc.ORDERS",
		"$JS.API.STREAM.DELETE.ORDERS",
		"$JS.API.STREAM.CANCEL_MOVE.ORDERS",
		"$JS.API.DIRECT.GET.ORDERS",
		"$JS.API.DIRECT.GET.ORDERS.orders.new",
		"$JS.API.CONSUMER.DELETE.ORDERS.C1",
		"$JS.API.CONSUMER.PEER.REMOVE.ORDERS.C1",
	}

	for _, subject := range subjects {
		for _, pd := range [][2]string{{"", ""}, {"", "hub"}, {"foo", ""}, {"JS.acc.API", ""}, {"a.b.c", ""}} {
			in := jsm.APISubject(subject, pd[0], pd[1])

			res, ok := registry.NormalizeAPISubject(in)
			if !ok {
				t.Fatalf("%q did not normalize", in)
			}
			if res != subject {
				t.Fatalf("expected %q from %q got %q", subject, in, res)
			}
		}
	}
}

func TestNormalizeAPISubjectAllocations(t *testing.T) {
	cases := []struct {
		subject string
		allocs  float64
	}{
		{"$JS.API.STREAM.INFO.ORDERS", 0},
		{"$JS.API.STREAM.INFO.*", 0},
		{"foo.bar.baz", 0},
		{"$JS.hub.API.STREAM.INFO.ORDERS", 1},
		{"JS.acc.API.STREAM.INFO.ORDERS", 1},
	}

	for _, tc := range cases {
		allocs := testing.AllocsPerRun(100, func() {
			registry.NormalizeAPISubject(tc.subject)
		})
		if allocs != tc.allocs {
			t.Fatalf("expected %v allocations for %q got %v", tc.allocs, tc.subject, allocs)
		}
	}
}

func FuzzNormalizeAPISubject(f *testing.F) {
	f.Add("$JS.API.STREAM.INFO.ORDERS")
	f.Add("$JS.hub.API.STREAM.INFO.ORDERS")
	f.Add("JS.acc.API.CONSUMER.CREATE.ORDERS.C1.x.y")
	f.Add("$JS..API.")
	f.Add("")

	f.Fuzz(func(t *testing.T, subject string) {
		res, ok := registry.NormalizeAPISubject(subject)
		if !ok {
			if res != subject {
				t.Fatalf("unrecognized %q was changed to %q", subject, res)
			}
			return
		}

		if !strings.HasPrefix(res, "$JS.API.") {
			t.Fatalf("%q normalized to non api subject %q", subject, res)
		}
		if !strings.HasSuffix(subject, strings.TrimPrefix(res, "$JS.API.")) {
			t.Fatalf("%q normalized to %q which is not a suffix", subject, res)
		}

		again, ok := registry.NormalizeAPISubject(res)
		if !ok || again != res {
			t.Fatalf("%q is not stable, got %q", res, again)
		}
	})
}

func BenchmarkNormalizeAPISubject(b *testing.B) {
	for _, subject := range []string{"$JS.API.STREAM.INFO.ORDERS", "$JS.hub.API.STREAM.INFO.ORDERS", "JS.acc.API.CONSUMER.INFO.ORDERS.C1"} {
		b.Run(subject, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				registry.NormalizeAPISubject(subject)
			}
		})
	}
}
