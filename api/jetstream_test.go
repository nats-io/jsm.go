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
	"fmt"
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

	// nil input returns 0
	v, err = RequiredApiLevel(nil)
	checkErr(t, err, "RequiredApiLevel nil failed")
	if v != 0 {
		t.Fatalf("expected level 0 for nil, got %v", v)
	}

	// non-struct returns 0
	v, err = RequiredApiLevel("hello")
	checkErr(t, err, "RequiredApiLevel string failed")
	if v != 0 {
		t.Fatalf("expected level 0 for string, got %v", v)
	}

	// zero-value struct returns 0
	v, err = RequiredApiLevel(&JSApiStreamCreateRequest{})
	checkErr(t, err, "RequiredApiLevel zero failed")
	if v != 0 {
		t.Fatalf("expected level 0 for zero value, got %v", v)
	}
}

func TestRequiredApiLevelPointerFields(t *testing.T) {
	type inner struct {
		Field bool `api_level:"3"`
	}
	type outer struct {
		Ptr *inner
	}

	// nil pointer field should be skipped
	v, err := RequiredApiLevel(outer{})
	checkErr(t, err, "RequiredApiLevel nil ptr failed")
	if v != 0 {
		t.Fatalf("expected level 0 for nil pointer, got %v", v)
	}

	// non-nil pointer to struct with api_level tag should be detected
	v, err = RequiredApiLevel(outer{Ptr: &inner{Field: true}})
	checkErr(t, err, "RequiredApiLevel ptr failed")
	if v != 3 {
		t.Fatalf("expected level 3, got %v", v)
	}

	// real world: StreamConfig with pointer fields set
	cfg := StreamConfig{
		SubjectTransform: &SubjectTransformConfig{
			Source:      "foo",
			Destination: "bar",
		},
		AllowMsgCounter: true, // level 2
	}
	v, err = RequiredApiLevel(&JSApiStreamCreateRequest{StreamConfig: cfg})
	checkErr(t, err, "RequiredApiLevel StreamConfig ptr fields failed")
	if v != 2 {
		t.Fatalf("expected level 2, got %v", v)
	}
}

func TestRequiredApiLevelStructFieldTag(t *testing.T) {
	// api_level tag on a pointer-to-struct field should be honored even when
	// the nested struct itself carries no api_level tags.
	type inner struct {
		Name string `json:"name,omitempty"`
	}
	type outer struct {
		Nested *inner `json:"nested,omitempty" api_level:"5"`
	}

	v, err := RequiredApiLevel(outer{Nested: &inner{Name: "x"}})
	checkErr(t, err, "RequiredApiLevel outer ptr struct tag failed")
	if v != 5 {
		t.Fatalf("expected level 5 from struct field tag, got %v", v)
	}

	// same expectation for a value (non-pointer) struct field.
	type outerVal struct {
		Nested inner `json:"nested,omitempty" api_level:"5"`
	}
	v, err = RequiredApiLevel(outerVal{Nested: inner{Name: "x"}})
	checkErr(t, err, "RequiredApiLevel outer value struct tag failed")
	if v != 5 {
		t.Fatalf("expected level 5 from value struct field tag, got %v", v)
	}

	// real world: StreamSource.Consumer carries api_level:"4" while
	// StreamConsumerSource has no tags of its own — setting Consumer alone
	// must surface level 4.
	src := StreamSource{
		Name:     "src",
		Consumer: &StreamConsumerSource{Name: "dur"},
	}
	v, err = RequiredApiLevel(src)
	checkErr(t, err, "RequiredApiLevel StreamSource.Consumer failed")
	if v != 4 {
		t.Fatalf("expected level 4 from StreamSource.Consumer tag, got %v", v)
	}

	// nested tag must still win when it is higher than the parent field tag.
	type innerHi struct {
		Field bool `api_level:"7"`
	}
	type outerLo struct {
		Nested *innerHi `api_level:"2"`
	}
	v, err = RequiredApiLevel(outerLo{Nested: &innerHi{Field: true}})
	checkErr(t, err, "RequiredApiLevel nested-higher failed")
	if v != 7 {
		t.Fatalf("expected nested level 7 to win, got %v", v)
	}
}

func TestRequiredApiLevelSliceFields(t *testing.T) {
	// real world: StreamConfig.Sources is []*StreamSource — setting Consumer
	// on an entry must surface the api_level:"4" tag on StreamSource.Consumer.
	// without slice descent the level stayed 0 and Nats-Required-Api-Level was
	// omitted, letting non-strict servers silently ignore the consumer block.
	req := &JSApiStreamCreateRequest{
		StreamConfig: StreamConfig{
			Sources: []*StreamSource{
				{
					Name:     "src",
					Consumer: &StreamConsumerSource{Name: "dur"},
				},
			},
		},
	}
	v, err := RequiredApiLevel(req)
	checkErr(t, err, "RequiredApiLevel Sources slice failed")
	if v != 4 {
		t.Fatalf("expected level 4 from Sources[0].Consumer, got %v", v)
	}

	// nil entries in a slice of pointers must be skipped without panicking.
	req = &JSApiStreamCreateRequest{
		StreamConfig: StreamConfig{
			Sources: []*StreamSource{
				nil,
				{Name: "src", Consumer: &StreamConsumerSource{Name: "dur"}},
			},
		},
	}
	v, err = RequiredApiLevel(req)
	checkErr(t, err, "RequiredApiLevel Sources with nil entry failed")
	if v != 4 {
		t.Fatalf("expected level 4 from non-nil source, got %v", v)
	}

	// non-pointer slice element type ([]SubjectTransformConfig) recursing
	// into a struct field with an api_level tag.
	type taggedInner struct {
		Field bool `api_level:"6"`
	}
	type withSlice struct {
		Items []taggedInner
	}
	v, err = RequiredApiLevel(withSlice{Items: []taggedInner{{Field: true}}})
	checkErr(t, err, "RequiredApiLevel value-slice failed")
	if v != 6 {
		t.Fatalf("expected level 6 from value-slice element, got %v", v)
	}

	// empty and nil slices must not raise the level above zero.
	v, err = RequiredApiLevel(withSlice{})
	checkErr(t, err, "RequiredApiLevel nil slice failed")
	if v != 0 {
		t.Fatalf("expected level 0 for nil slice, got %v", v)
	}

	v, err = RequiredApiLevel(withSlice{Items: []taggedInner{}})
	checkErr(t, err, "RequiredApiLevel empty slice failed")
	if v != 0 {
		t.Fatalf("expected level 0 for empty slice, got %v", v)
	}

	// slice of non-struct elements (e.g. []string) must be a no-op.
	type withStringSlice struct {
		Items []string
	}
	v, err = RequiredApiLevel(withStringSlice{Items: []string{"a", "b"}})
	checkErr(t, err, "RequiredApiLevel string slice failed")
	if v != 0 {
		t.Fatalf("expected level 0 for non-struct slice, got %v", v)
	}

	// the highest level among multiple slice entries wins.
	type lvlA struct {
		F bool `api_level:"3"`
	}
	type multi struct {
		Items []*lvlA
	}
	v, err = RequiredApiLevel(multi{Items: []*lvlA{{F: true}, {F: true}}})
	checkErr(t, err, "RequiredApiLevel multi slice failed")
	if v != 3 {
		t.Fatalf("expected level 3 from slice elements, got %v", v)
	}
}

func TestIsNatsErr(t *testing.T) {
	// value ApiError
	valErr := ApiError{Code: 400, ErrCode: 10001, Description: "bad request"}
	if !IsNatsErr(valErr, 10001) {
		t.Fatal("expected IsNatsErr to match value ApiError")
	}
	if IsNatsErr(valErr, 65535) {
		t.Fatal("expected IsNatsErr not to match wrong code")
	}

	// pointer ApiError
	ptrErr := &ApiError{Code: 404, ErrCode: 10014, Description: "not found"}
	if !IsNatsErr(ptrErr, 10014) {
		t.Fatal("expected IsNatsErr to match pointer ApiError")
	}
	if IsNatsErr(ptrErr, 65535) {
		t.Fatal("expected IsNatsErr not to match wrong code on pointer")
	}

	// multiple ids, match any
	if !IsNatsErr(valErr, 65535, 10001) {
		t.Fatal("expected IsNatsErr to match one of multiple ids")
	}

	// nil error
	if IsNatsErr(nil, 10001) {
		t.Fatal("expected IsNatsErr to return false for nil")
	}

	// non-ApiError error
	if IsNatsErr(fmt.Errorf("some error"), 10001) {
		t.Fatal("expected IsNatsErr to return false for non-ApiError")
	}

	// wrapped value error
	wrapped := fmt.Errorf("wrap: %w", valErr)
	if !IsNatsErr(wrapped, 10001) {
		t.Fatal("expected IsNatsErr to match wrapped value ApiError")
	}

	// wrapped pointer error
	wrappedPtr := fmt.Errorf("wrap: %w", ptrErr)
	if !IsNatsErr(wrappedPtr, 10014) {
		t.Fatal("expected IsNatsErr to match wrapped pointer ApiError")
	}
}

func TestIsNatsError(t *testing.T) {
	valErr := ApiError{Code: 400, ErrCode: 10001}
	ptrErr := &ApiError{Code: 404, ErrCode: 10014}

	if !IsNatsError(valErr, 10001) {
		t.Fatal("expected match on value ApiError")
	}
	if !IsNatsError(ptrErr, 10014) {
		t.Fatal("expected match on pointer ApiError")
	}
	if IsNatsError(nil, 10001) {
		t.Fatal("expected false for nil")
	}
	if IsNatsError(fmt.Errorf("other"), 10001) {
		t.Fatal("expected false for non-ApiError")
	}
}

func TestApiError(t *testing.T) {
	// zero value
	e := ApiError{}
	if e.Error() != "unknown JetStream Error" {
		t.Fatalf("unexpected error string: %s", e.Error())
	}

	// code only, no description
	e = ApiError{Code: 503, ErrCode: 10008}
	if e.Error() != "unknown JetStream 503 Error (10008)" {
		t.Fatalf("unexpected error string: %s", e.Error())
	}

	// full error
	e = ApiError{Code: 400, ErrCode: 10001, Description: "bad request"}
	if e.Error() != "bad request (10001)" {
		t.Fatalf("unexpected error string: %s", e.Error())
	}

	// NotFoundError
	if e.NotFoundError() {
		t.Fatal("400 should not be NotFoundError")
	}
	e.Code = 404
	if !e.NotFoundError() {
		t.Fatal("404 should be NotFoundError")
	}

	// ServerError
	e.Code = 500
	if !e.ServerError() {
		t.Fatal("500 should be ServerError")
	}
	e.Code = 599
	if !e.ServerError() {
		t.Fatal("599 should be ServerError")
	}
	e.Code = 600
	if e.ServerError() {
		t.Fatal("600 should not be ServerError")
	}

	// UserError
	e.Code = 400
	if !e.UserError() {
		t.Fatal("400 should be UserError")
	}
	e.Code = 499
	if !e.UserError() {
		t.Fatal("499 should be UserError")
	}
	e.Code = 500
	if e.UserError() {
		t.Fatal("500 should not be UserError")
	}

	// ErrorCode / NatsErrorCode
	e = ApiError{Code: 400, ErrCode: 10001}
	if e.ErrorCode() != 400 {
		t.Fatalf("expected ErrorCode 400, got %d", e.ErrorCode())
	}
	if e.NatsErrorCode() != 10001 {
		t.Fatalf("expected NatsErrorCode 10001, got %d", e.NatsErrorCode())
	}
}

func TestJSApiResponse(t *testing.T) {
	// no error
	r := JSApiResponse{}
	if r.IsError() {
		t.Fatal("expected no error")
	}
	if r.ToError() != nil {
		t.Fatal("expected nil error")
	}

	// with error
	r = JSApiResponse{Error: &ApiError{Code: 400, ErrCode: 10001, Description: "bad"}}
	if !r.IsError() {
		t.Fatal("expected error")
	}
	err := r.ToError()
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if err.Error() != "bad (10001)" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLastPage(t *testing.T) {
	// exact boundary
	r := JSApiIterableResponse{Total: 10, Offset: 5, Limit: 5}
	if !r.LastPage() {
		t.Fatal("expected last page when offset+limit == total")
	}

	// past boundary
	r = JSApiIterableResponse{Total: 10, Offset: 5, Limit: 10}
	if !r.LastPage() {
		t.Fatal("expected last page when offset+limit > total")
	}

	// not last page
	r = JSApiIterableResponse{Total: 100, Offset: 0, Limit: 10}
	if r.LastPage() {
		t.Fatal("expected not last page")
	}

	// zero total is last page
	r = JSApiIterableResponse{Total: 0, Offset: 0, Limit: 10}
	if !r.LastPage() {
		t.Fatal("expected last page when total is 0")
	}

	// env var override
	t.Setenv("PAGE_TOTAL", "5")
	r = JSApiIterableResponse{Total: 100, Offset: 0, Limit: 10}
	if !r.LastPage() {
		t.Fatal("expected last page with PAGE_TOTAL override")
	}
}
