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
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nkeys"
)

// sealLoadReply seals a LoadSealed plaintext for the client's
// per-request ephemeral xkey. It is called server-side when
// responding to ctx.load.
func sealLoadReply(serverPriv nkeys.KeyPair, replyPub string, payload LoadSealed) (string, error) {
	return sealStruct(serverPriv, replyPub, payload)
}

// openLoadReply opens a LoadResponse.Sealed with the client's
// ephemeral priv and the cached server pub.
func openLoadReply(clientEphPriv nkeys.KeyPair, serverPub string, sealed string) (LoadSealed, error) {
	var out LoadSealed
	err := openStruct(clientEphPriv, serverPub, sealed, &out)
	if err != nil {
		return LoadSealed{}, err
	}

	return out, nil
}

// sealSaveRequest seals a SaveSealed plaintext for the server's
// long-term xkey. It is called client-side when preparing a ctx.save.
func sealSaveRequest(clientEphPriv nkeys.KeyPair, serverPub string, payload SaveSealed) (string, error) {
	return sealStruct(clientEphPriv, serverPub, payload)
}

// openSaveRequest opens a SaveRequest.Sealed using the server priv
// and the client's ephemeral pub carried in SaveRequest.SenderPub.
func openSaveRequest(serverPriv nkeys.KeyPair, senderPub string, sealed string) (SaveSealed, error) {
	var out SaveSealed
	err := openStruct(serverPriv, senderPub, sealed, &out)
	if err != nil {
		return SaveSealed{}, err
	}

	return out, nil
}

// sealStruct marshals v to JSON, seals the bytes to recipientPub with
// senderPriv, and base64-encodes the resulting xkv1 blob.
func sealStruct(senderPriv nkeys.KeyPair, recipientPub string, v any) (string, error) {
	plaintext, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal sealed payload: %w", err)
	}

	ct, err := senderPriv.Seal(plaintext, recipientPub)
	if err != nil {
		return "", fmt.Errorf("seal: %w", err)
	}

	return base64.StdEncoding.EncodeToString(ct), nil
}

// openStruct base64-decodes sealed, opens it with recipientPriv using
// senderPub, and unmarshals the plaintext into v.
func openStruct(recipientPriv nkeys.KeyPair, senderPub string, sealed string, v any) error {
	ct, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return fmt.Errorf("decode sealed: %w", err)
	}

	plaintext, err := recipientPriv.Open(ct, senderPub)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}

	err = json.Unmarshal(plaintext, v)
	if err != nil {
		return fmt.Errorf("unmarshal sealed payload: %w", err)
	}

	return nil
}

// newReqID returns a 16-byte random hex-encoded token suitable for
// req_id fields. The value is unique with overwhelming probability
// across practical request rates.
func newReqID() (string, error) {
	var b [16]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}

	return hex.EncodeToString(b[:]), nil
}
