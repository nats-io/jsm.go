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

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"

	"github.com/nats-io/nkeys"

	"github.com/nats-io/jsm.go/natscontext/svcbackend"
)

// cryptoChecks covers PROTOCOL.md §9 / Conformance "Crypto". A
// successful save/load round-trip proves:
//   - the server holds a usable long-term xkey (sys.xkey advertises it),
//   - ctx.save is opened with (server_priv, sender_pub),
//   - ctx.load is sealed with (server_priv, reply_pub),
//   - the sealed field is base64 xkv1.
//
// We still split these into individual checks so a partial failure is
// legible in the report.
func cryptoChecks() []Check {
	return []Check{
		{
			ID: "crypto.sys_xkey_valid", Section: "Crypto",
			Title: "sys.xkey advertises a valid curve public key",
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				pub, err := sysXKey(ctx, h)
				if err != nil {
					return StatusFail, "fetch sys.xkey: " + err.Error(), nil
				}

				// nkeys curve public keys start with 'X'. FromPublicKey
				// validates prefix and checksum.
				_, keyErr := nkeys.FromPublicKey(pub)
				if keyErr != nil {
					return StatusFail, "sys.xkey returned invalid curve key: " + keyErr.Error(), nil
				}
				if len(pub) == 0 || pub[0] != 'X' {
					return StatusFail, "sys.xkey public key does not have curve prefix", nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "crypto.roundtrip", Section: "Crypto",
			Title: "save + load round-trip decrypts to the original payload",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				name := h.MintName("crypto")
				want := []byte(`{"canary":"CONFORMANCE-CANARY-` + name + `"}`)

				err := h.Client.Save(ctx, name, want)
				if err != nil {
					return StatusFail, "save: " + err.Error(), nil
				}

				got, err := h.Client.Load(ctx, name)
				if err != nil {
					return StatusFail, "load: " + err.Error(), nil
				}
				if !bytes.Equal(got, want) {
					return StatusFail, fmt.Sprintf("round-trip mismatch: got %d bytes, want %d bytes", len(got), len(want)), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "crypto.sealed_format", Section: "Crypto",
			Title: "ctx.load sealed field is base64-encoded xkv1 ciphertext",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				// Store something, then do a raw ctx.load and inspect the
				// sealed field. The reference client already opens it on
				// the happy path; here we assert the wire envelope shape.
				name := h.MintName("sealed")
				err := h.Client.Save(ctx, name, []byte(`{"x":1}`))
				if err != nil {
					return StatusFail, "save: " + err.Error(), nil
				}

				sealedB64, rawErr := rawLoadSealed(ctx, h, name)
				if rawErr != nil {
					return StatusFail, "raw load: " + rawErr.Error(), nil
				}

				if sealedB64 == "" {
					return StatusFail, "sealed field empty", nil
				}

				ct, decErr := base64.StdEncoding.DecodeString(sealedB64)
				if decErr != nil {
					return StatusFail, "sealed is not std-base64: " + decErr.Error(), nil
				}

				// xkv1 blobs start with the literal ASCII "xkv1" header.
				if len(ct) < 4 || string(ct[:4]) != "xkv1" {
					return StatusFail, fmt.Sprintf("sealed does not carry xkv1 header (got %q)", safePrefix(ct, 4)), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "crypto.sys_xkey_usable", Section: "Crypto",
			Title: "sys.xkey-advertised key matches the key used to seal responses",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				// The reference client opens ctx.load replies using the
				// server key fetched via sys.xkey. If that succeeds, the
				// advertised key is the current one in use.
				name := h.MintName("adv")
				err := h.Client.Save(ctx, name, []byte("ok"))
				if err != nil {
					return StatusFail, "save: " + err.Error(), nil
				}
				_, err = h.Client.Load(ctx, name)
				if err != nil {
					return StatusFail, "load (key mismatch would fail here): " + err.Error(), nil
				}
				return StatusPass, "", nil
			},
		},

		{
			ID: "crypto.save_stale_key", Section: "Crypto",
			Title: "ctx.save sealed to a non-current xkey returns stale_key",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				// Seal a SaveRequest to a freshly-generated curve public
				// key that the server has never held. The server's Open
				// MUST fail, and PROTOCOL.md §9.4 requires stale_key on
				// the wire so clients can trigger the rotation retry.
				wrong, err := newCurveKey()
				if err != nil {
					return StatusFail, "generate wrong key: " + err.Error(), nil
				}
				defer wrong.Wipe()

				wrongPub, err := wrong.PublicKey()
				if err != nil {
					return StatusFail, "wrong key public: " + err.Error(), nil
				}

				eph, err := newCurveKey()
				if err != nil {
					return StatusFail, "generate ephemeral: " + err.Error(), nil
				}
				defer eph.Wipe()

				ephPub, err := eph.PublicKey()
				if err != nil {
					return StatusFail, "ephemeral public: " + err.Error(), nil
				}

				reqID, err := newHexID()
				if err != nil {
					return StatusFail, "req_id: " + err.Error(), nil
				}

				sealed, err := sealSave(eph, wrongPub, svcbackend.SaveSealed{Data: []byte("x"), ReqID: reqID})
				if err != nil {
					return StatusFail, "seal: " + err.Error(), nil
				}

				name := h.MintName("stalekey")

				var resp svcbackend.SaveResponse
				err = rawRequest(ctx, h, h.Prefix+".ctx.save."+name, svcbackend.SaveRequest{
					SenderPub: ephPub,
					Sealed:    sealed,
				}, &resp)
				if err != nil {
					return StatusFail, "raw save: " + err.Error(), nil
				}

				if resp.Error == nil {
					h.Track(name)
					return StatusFail, "expected stale_key error, got success (server accepted payload sealed to an unknown key)", nil
				}
				if resp.Error.Code != string(svcbackend.CodeStaleKey) {
					return StatusFail, fmt.Sprintf("expected code %q, got %q (message %q)", svcbackend.CodeStaleKey, resp.Error.Code, resp.Error.Message), nil
				}
				return StatusPass, "", nil
			},
		},
	}
}

// safePrefix returns up to n bytes of b as a %q-safe string. Used to
// surface the first bytes of an unexpected ciphertext blob.
func safePrefix(b []byte, n int) string {
	if len(b) < n {
		n = len(b)
	}
	return string(b[:n])
}
