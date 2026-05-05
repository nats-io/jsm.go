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
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/jsm.go/natscontext/svcbackend"
)

// rawRequest sends a raw request to subject and decodes the reply into
// respOut. It bypasses the reference client so checks can exercise
// malformed inputs, invalid names, or unusual request shapes.
//
// A per-request timeout is applied from h.Timeout when ctx has no
// deadline so a stuck server can't hang the suite.
func rawRequest(ctx context.Context, h *Harness, subject string, req any, respOut any) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	reqCtx := ctx
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, h.Timeout)
		defer cancel()
	}

	msg, err := h.NC.RequestWithContext(reqCtx, subject, data)
	if err != nil {
		return fmt.Errorf("nats request %s: %w", subject, err)
	}

	err = json.Unmarshal(msg.Data, respOut)
	if err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}

// rawProbe sends an empty JSON object to subject and reports whether a
// responder exists. Any response (even an error envelope) is evidence
// that a responder is registered.
func rawProbe(ctx context.Context, h *Harness, subject string) error {
	reqCtx := ctx
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, h.Timeout)
		defer cancel()
	}

	_, err := h.NC.RequestWithContext(reqCtx, subject, []byte("{}"))
	return err
}

// sysXKey fetches the server's advertised long-term xkey public key via
// a raw sys.xkey call.
func sysXKey(ctx context.Context, h *Harness) (string, error) {
	var resp svcbackend.SysXKeyResponse

	err := rawRequest(ctx, h, h.Prefix+".sys.xkey", svcbackend.SysXKeyRequest{}, &resp)
	if err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", svcbackend.SentinelFromError(resp.Error)
	}
	if resp.XKeyPub == "" {
		return "", fmt.Errorf("sys.xkey returned empty xkey_pub")
	}
	return resp.XKeyPub, nil
}

// rawLoadSealed issues a ctx.load for name with a throwaway ephemeral
// reply key and returns the raw base64 sealed field from the response.
// The ephemeral is discarded; the caller only inspects envelope shape.
func rawLoadSealed(ctx context.Context, h *Harness, name string) (string, error) {
	kp, err := newCurveKey()
	if err != nil {
		return "", err
	}
	defer kp.Wipe()

	pub, err := kp.PublicKey()
	if err != nil {
		return "", err
	}

	reqID, err := newHexID()
	if err != nil {
		return "", err
	}

	var resp svcbackend.LoadResponse
	err = rawRequest(ctx, h, h.Prefix+".ctx.load."+name, svcbackend.LoadRequest{
		ReplyPub: pub,
		ReqID:    reqID,
	}, &resp)
	if err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", svcbackend.SentinelFromError(resp.Error)
	}
	return resp.Sealed, nil
}

// rawSave publishes a valid sealed SaveRequest under an arbitrary name
// token. It lets checks reach the server's name validator (the reference
// client pre-validates and would short-circuit invalid names locally).
func rawSave(ctx context.Context, h *Harness, name string, payload []byte) error {
	serverPub, err := sysXKey(ctx, h)
	if err != nil {
		return fmt.Errorf("sys.xkey: %w", err)
	}

	eph, err := newCurveKey()
	if err != nil {
		return err
	}
	defer eph.Wipe()

	ephPub, err := eph.PublicKey()
	if err != nil {
		return err
	}

	reqID, err := newHexID()
	if err != nil {
		return err
	}

	sealed, err := sealSave(eph, serverPub, svcbackend.SaveSealed{Data: payload, ReqID: reqID})
	if err != nil {
		return err
	}

	var resp svcbackend.SaveResponse
	err = rawRequest(ctx, h, h.Prefix+".ctx.save."+name, svcbackend.SaveRequest{
		SenderPub: ephPub,
		Sealed:    sealed,
	}, &resp)
	if err != nil {
		return err
	}
	return svcbackend.SentinelFromError(resp.Error)
}
