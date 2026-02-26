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
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"

	"github.com/nats-io/jsm.go"
)

func checkErr(t *testing.T, err error, m string) {
	t.Helper()
	if err == nil {
		return
	}
	t.Fatal(m + ": " + err.Error())
}

func TestAPISubject(t *testing.T) {
	cases := []struct {
		subject string
		prefix  string
		domain  string
		res     string
	}{
		{"$JS.API.FOO", "", "", "$JS.API.FOO"},
		{"$JS.API.FOO", "js.foreign", "", "js.foreign.FOO"},
		{"$JS.API.FOO", "js.foreign", "domain", "$JS.domain.API.FOO"},
		{"$JS.API.FOO", "", "domain", "$JS.domain.API.FOO"},
	}

	for _, tc := range cases {
		res := jsm.APISubject(tc.subject, tc.prefix, tc.domain)
		if res != tc.res {
			t.Fatalf("expected %s got %s", tc.res, res)
		}
	}
}

func TestIsErrorResponse(t *testing.T) {
	if jsm.IsErrorResponse(&nats.Msg{Data: []byte("+OK")}) {
		t.Fatalf("OK is Error")
	}

	if jsm.IsErrorResponse(&nats.Msg{Data: []byte(`{"stream":"test"}`)}) {
		t.Fatalf("OK JSON is Error")
	}

	if !jsm.IsErrorResponse(&nats.Msg{Data: []byte("-ERR 'error'")}) {
		t.Fatalf("ERR is not Error")
	}

	if !jsm.IsErrorResponse(&nats.Msg{Data: []byte(`{"error":{"code":404}}`)}) {
		t.Fatalf("JSON response is not Error")
	}
}

func TestParseErrorResponse(t *testing.T) {
	checkErr(t, jsm.ParseErrorResponse(&nats.Msg{Data: []byte("+OK")}), "expected nil got error")

	cases := []struct {
		err    string
		expect string
	}{
		{"-ERR 'test error", "test error"},
		{`{"error":{"code":404,"description":"test error", "err_code":123}}`, "test error (123)"},
	}
	for _, c := range cases {
		err := jsm.ParseErrorResponse(&nats.Msg{Data: []byte(c.err)})
		if err == nil {
			t.Fatalf("expected an error got nil")
		}

		if err.Error() != c.expect {
			t.Fatalf("expected 'test error' got '%v'", err)
		}
	}
}

func TestIsOKResponse(t *testing.T) {
	if !jsm.IsOKResponse(&nats.Msg{Data: []byte("+OK")}) {
		t.Fatalf("OK is Error")
	}

	if !jsm.IsOKResponse(&nats.Msg{Data: []byte(`{"stream":"x"}`)}) {
		t.Fatalf("JSON response is not Error")
	}

	if jsm.IsOKResponse(&nats.Msg{Data: []byte("-ERR error")}) {
		t.Fatalf("ERR is not Error")
	}

	if jsm.IsOKResponse(&nats.Msg{Data: []byte(`{"error":{"code": 404}}`)}) {
		t.Fatalf("JSON is not Error")
	}
}

func TestIsValidName(t *testing.T) {
	if jsm.IsValidName("") {
		t.Fatalf("did not detect empty string as invalid")
	}

	for _, c := range []string{".", ">", "*", "\x00"} {
		tc := "x" + c + "x"
		if jsm.IsValidName(tc) {
			t.Fatalf("did not detect %q as invalid", tc)
		}
	}
}

func TestIsErrorResponseNil(t *testing.T) {
	if jsm.IsErrorResponse(nil) {
		t.Fatal("nil message should not be an error response")
	}
}

func TestParseErrorResponseNil(t *testing.T) {
	err := jsm.ParseErrorResponse(nil)
	if err == nil {
		t.Fatal("expected error for nil message, got nil")
	}
}

func TestIsOKResponseNil(t *testing.T) {
	if jsm.IsOKResponse(nil) {
		t.Fatal("nil message should not be an OK response")
	}
}

func TestAPISubjectNonMatchingSubject(t *testing.T) {
	// When subject does not start with "$JS.API", it must be returned unchanged.
	cases := []struct {
		subject string
		prefix  string
		domain  string
	}{
		{"CUSTOM.SUBJECT", "myprefix", ""},
		{"CUSTOM.SUBJECT", "", "mydomain"},
	}

	for _, tc := range cases {
		got := jsm.APISubject(tc.subject, tc.prefix, tc.domain)
		if got != tc.subject {
			t.Errorf("APISubject(%q, %q, %q) = %q, want %q (unchanged)", tc.subject, tc.prefix, tc.domain, got, tc.subject)
		}
	}
}

func TestEventSubjectNonMatchingSubject(t *testing.T) {
	// When subject does not start with "$JS.EVENT", it must be returned unchanged.
	got := jsm.EventSubject("CUSTOM.SUBJECT", "myprefix")
	if got != "CUSTOM.SUBJECT" {
		t.Errorf("EventSubject(\"CUSTOM.SUBJECT\", \"myprefix\") = %q, want %q (unchanged)", got, "CUSTOM.SUBJECT")
	}
}

func TestIsNatsError(t *testing.T) {
	pubAck := `{"error":{"code":503,"err_code":10077,"description":"maximum messages exceeded"},"stream":"GOVERNOR_TEST","seq":0}`

	resp := api.JSApiResponse{}
	err := json.Unmarshal([]byte(pubAck), &resp)
	if err != nil {
		t.Fatalf("unmarshal failed: %s", err)
	}

	if !jsm.IsNatsError(resp.ToError(), 10077) {
		t.Fatalf("Expected response to be 10077")
	}

	if jsm.IsNatsError(resp.ToError(), 10076) {
		t.Fatalf("Expected response to not be 10076")
	}

	if jsm.IsNatsError(fmt.Errorf("error"), 10077) {
		t.Fatalf("Non api error is 10077")
	}
}

func TestFilterServerMetadata(t *testing.T) {
	srv, nc, mgr := startJSServer(t)
	defer srv.Shutdown()
	defer nc.Flush()

	s, err := mgr.NewStream("q1", jsm.Subjects("in.q1"), jsm.StreamMetadata(map[string]string{
		"io.nats.monitor.enabled":       "1",
		"io.nats.monitor.lag-critical":  "100",
		"io.nats.monitor.msgs-critical": "500",
		"io.nats.monitor.msgs-warn":     "999",
		"io.nats.monitor.peer-expect":   "3",
		"io.nats.monitor.seen-critical": "5m",
	}))
	checkErr(t, err, "create failed")

	if _, ok := s.Metadata()[api.JsMetaRequiredServerLevel]; !ok {
		t.Fatalf("No server metadata added")
	}

	newMeta := jsm.FilterServerMetadata(s.Metadata())

	expected := map[string]string{
		"io.nats.monitor.enabled":       "1",
		"io.nats.monitor.lag-critical":  "100",
		"io.nats.monitor.msgs-critical": "500",
		"io.nats.monitor.msgs-warn":     "999",
		"io.nats.monitor.peer-expect":   "3",
		"io.nats.monitor.seen-critical": "5m",
	}

	if !reflect.DeepEqual(newMeta, expected) {
		t.Fatalf("all server data was not removed: %v", newMeta)
	}
}
