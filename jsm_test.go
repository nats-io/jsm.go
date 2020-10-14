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

func TestIsErrorResponse(t *testing.T) {
	if jsm.IsErrorResponse(&nats.Msg{Data: []byte("+OK")}) {
		t.Fatalf("OK is Error")
	}

	if !jsm.IsErrorResponse(&nats.Msg{Data: []byte("-ERR 'error'")}) {
		t.Fatalf("ERR is not Error")
	}
}

func TestParseErrorResponse(t *testing.T) {
	checkErr(t, jsm.ParseErrorResponse(&nats.Msg{Data: []byte("+OK")}), "expected nil got error")

	err := jsm.ParseErrorResponse(&nats.Msg{Data: []byte("-ERR 'test error")})
	if err == nil {
		t.Fatalf("expected an error got nil")
	}

	if err.Error() != "test error" {
		t.Fatalf("expected 'test error' got '%v'", err)
	}
}

func TestIsOKResponse(t *testing.T) {
	if !jsm.IsOKResponse(&nats.Msg{Data: []byte("+OK")}) {
		t.Fatalf("OK is Error")
	}

	if jsm.IsOKResponse(&nats.Msg{Data: []byte("-ERR error")}) {
		t.Fatalf("ERR is not Error")
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
