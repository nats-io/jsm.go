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

package svcbackend

// ErrorResponse carries a wire-level error. Code is one of the values
// defined in errors.go; Message is a human-readable description that
// MUST NOT include payload bytes.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// SysXKeyRequest is the request for natscontext.v1.sys.xkey. It has
// no fields; its JSON encoding is the empty object.
type SysXKeyRequest struct{}

// SysXKeyResponse carries the server's long-term xkey public key.
type SysXKeyResponse struct {
	XKeyPub string         `json:"xkey_pub,omitempty"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

// LoadRequest is the plaintext envelope for natscontext.v1.ctx.load.
// ReplyPub is the client's per-request ephemeral xkey public key; the
// server seals LoadSealed to it. ReqID is echoed back inside LoadSealed
// so the client can correlate the response.
type LoadRequest struct {
	ReplyPub string `json:"reply_pub"`
	ReqID    string `json:"req_id"`
}

// LoadResponse carries the sealed LoadSealed payload.
type LoadResponse struct {
	Sealed string         `json:"sealed,omitempty"`
	Error  *ErrorResponse `json:"error,omitempty"`
}

// LoadSealed is the plaintext that lives inside LoadResponse.Sealed.
// It is never sent on the wire in the clear.
type LoadSealed struct {
	Data  []byte `json:"data"`
	ReqID string `json:"req_id"`
}

// SaveRequest is the envelope for natscontext.v1.ctx.save. SenderPub
// is the client's ephemeral xkey public key; the server uses it to
// open Sealed.
type SaveRequest struct {
	SenderPub string `json:"sender_pub"`
	Sealed    string `json:"sealed"`
}

// SaveResponse carries only an optional error.
type SaveResponse struct {
	Error *ErrorResponse `json:"error,omitempty"`
}

// SaveSealed is the plaintext that lives inside SaveRequest.Sealed.
// It is never sent on the wire in the clear.
type SaveSealed struct {
	Data  []byte `json:"data"`
	ReqID string `json:"req_id"`
}

// DeleteRequest is the envelope for natscontext.v1.ctx.delete.
type DeleteRequest struct {
	ReqID string `json:"req_id"`
}

// DeleteResponse carries only an optional error.
type DeleteResponse struct {
	Error *ErrorResponse `json:"error,omitempty"`
}

// ListRequest is the request for natscontext.v1.ctx.list.
type ListRequest struct{}

// ListResponse carries the sorted names of all stored contexts.
type ListResponse struct {
	Names []string       `json:"names,omitempty"`
	Error *ErrorResponse `json:"error,omitempty"`
}

// SelectedRequest is the request for natscontext.v1.sel.get.
type SelectedRequest struct{}

// SelectedResponse carries the currently selected context name. A
// missing selection is signaled by Error with CodeNoneSelected.
type SelectedResponse struct {
	Name  string         `json:"name,omitempty"`
	Error *ErrorResponse `json:"error,omitempty"`
}

// SetSelectedRequest is the envelope for natscontext.v1.sel.set.
type SetSelectedRequest struct {
	ReqID string `json:"req_id"`
}

// SetSelectedResponse carries the previously selected name, if any.
type SetSelectedResponse struct {
	Previous string         `json:"previous,omitempty"`
	Error    *ErrorResponse `json:"error,omitempty"`
}

// ClearSelectedRequest is the envelope for natscontext.v1.sel.clear.
type ClearSelectedRequest struct {
	ReqID string `json:"req_id"`
}

// ClearSelectedResponse carries the previously selected name, if any.
type ClearSelectedResponse struct {
	Previous string         `json:"previous,omitempty"`
	Error    *ErrorResponse `json:"error,omitempty"`
}
