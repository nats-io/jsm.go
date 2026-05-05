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
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nkeys"

	"github.com/nats-io/jsm.go/natscontext/svcbackend"
)

// newCurveKey generates a per-request ephemeral curve keypair.
func newCurveKey() (nkeys.KeyPair, error) {
	kp, err := nkeys.CreateCurveKeys()
	if err != nil {
		return nil, fmt.Errorf("create curve keys: %w", err)
	}
	return kp, nil
}

// newHexID returns a 16-byte random token encoded as 32 hex chars.
func newHexID() (string, error) {
	var b [16]byte

	_, err := rand.Read(b[:])
	if err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}

	return hex.EncodeToString(b[:]), nil
}

// sealSave seals a SaveSealed plaintext to serverPub using senderPriv
// and returns the base64-encoded ciphertext suitable for the wire.
func sealSave(senderPriv nkeys.KeyPair, serverPub string, payload svcbackend.SaveSealed) (string, error) {
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal sealed payload: %w", err)
	}

	ct, err := senderPriv.Seal(plaintext, serverPub)
	if err != nil {
		return "", fmt.Errorf("seal: %w", err)
	}

	return base64.StdEncoding.EncodeToString(ct), nil
}
