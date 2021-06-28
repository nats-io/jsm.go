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

package jsm_test

import (
	"testing"

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

	for _, c := range []string{".", ">", "*"} {
		tc := "x" + c + "x"
		if jsm.IsValidName(tc) {
			t.Fatalf("did not detect %q as invalid", tc)
		}
	}
}
