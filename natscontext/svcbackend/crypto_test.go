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

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/nats-io/nkeys"
)

// newCurveKP returns a fresh curve KeyPair and its public key string.
// t.Fatal on any error so tests read linearly.
func newCurveKP(t *testing.T) (nkeys.KeyPair, string) {
	t.Helper()

	kp, err := nkeys.CreateCurveKeys()
	if err != nil {
		t.Fatalf("create curve keys: %v", err)
	}

	pub, err := kp.PublicKey()
	if err != nil {
		t.Fatalf("public key: %v", err)
	}

	return kp, pub
}

func TestSealOpenSaveRoundTrip(t *testing.T) {
	clientKP, clientPub := newCurveKP(t)
	serverKP, serverPub := newCurveKP(t)

	payload := SaveSealed{Data: []byte("hello world"), ReqID: "abc123"}

	sealed, err := sealSaveRequest(clientKP, serverPub, payload)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if sealed == "" {
		t.Fatalf("expected non-empty sealed")
	}

	got, err := openSaveRequest(serverKP, clientPub, sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if !bytes.Equal(got.Data, payload.Data) {
		t.Fatalf("data mismatch: got %q want %q", got.Data, payload.Data)
	}
	if got.ReqID != payload.ReqID {
		t.Fatalf("req_id mismatch: got %q want %q", got.ReqID, payload.ReqID)
	}
}

func TestSealOpenLoadRoundTrip(t *testing.T) {
	clientKP, clientPub := newCurveKP(t)
	serverKP, serverPub := newCurveKP(t)

	payload := LoadSealed{Data: []byte{0x00, 0x01, 0x02, 0xff}, ReqID: "deadbeef"}

	sealed, err := sealLoadReply(serverKP, clientPub, payload)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	got, err := openLoadReply(clientKP, serverPub, sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if !bytes.Equal(got.Data, payload.Data) {
		t.Fatalf("data mismatch: got %v want %v", got.Data, payload.Data)
	}
	if got.ReqID != payload.ReqID {
		t.Fatalf("req_id mismatch: got %q want %q", got.ReqID, payload.ReqID)
	}
}

func TestSealOpenTamperedCiphertextFails(t *testing.T) {
	clientKP, _ := newCurveKP(t)
	_, serverPub := newCurveKP(t)

	sealed, err := sealSaveRequest(clientKP, serverPub, SaveSealed{Data: []byte("payload"), ReqID: "r1"})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Flip a byte well past the xkv1 version header and nonce. The
	// header is 4 bytes ("xkv1") + 24-byte nonce = 28 bytes; any flip
	// past there lands in the AEAD body and MUST fail.
	if len(raw) <= 40 {
		t.Fatalf("unexpected short ciphertext: %d bytes", len(raw))
	}
	raw[35] ^= 0xff

	tampered := base64.StdEncoding.EncodeToString(raw)

	serverKP, _ := newCurveKP(t)
	_, err = openSaveRequest(serverKP, "", tampered)
	if err == nil {
		t.Fatalf("expected open to fail on tampered ciphertext")
	}
}

func TestOpenWithWrongRecipientFails(t *testing.T) {
	clientKP, clientPub := newCurveKP(t)
	_, serverPub := newCurveKP(t)

	sealed, err := sealSaveRequest(clientKP, serverPub, SaveSealed{Data: []byte("x"), ReqID: "r"})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}

	wrongKP, _ := newCurveKP(t)

	_, err = openSaveRequest(wrongKP, clientPub, sealed)
	if err == nil {
		t.Fatalf("expected open to fail with wrong recipient key")
	}
}

func TestNewReqID(t *testing.T) {
	a, err := newReqID()
	if err != nil {
		t.Fatalf("newReqID: %v", err)
	}

	if len(a) != 32 {
		t.Fatalf("expected 32 hex chars, got %d (%q)", len(a), a)
	}

	_, err = hex.DecodeString(a)
	if err != nil {
		t.Fatalf("not hex: %v", err)
	}

	b, err := newReqID()
	if err != nil {
		t.Fatalf("newReqID: %v", err)
	}
	if a == b {
		t.Fatalf("newReqID returned duplicate value %q", a)
	}
}
