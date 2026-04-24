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

// Package testserver provides a minimal svcbackend server wrapping
// any natscontext.Backend. It exists to back the contract tests in
// client_test.go and MUST NOT be imported from outside this module;
// the crypto and wire handling here deliberately mirror what an
// external server implementation would do, so this package doubles
// as a reference implementation of PROTOCOL.md.
package testserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
	"github.com/nats-io/nkeys"

	"github.com/nats-io/jsm.go/natscontext"
	"github.com/nats-io/jsm.go/natscontext/svcbackend"
)

// DefaultPrefix is the subject prefix the reference client uses when
// no override is configured.
const DefaultPrefix = "natscontext.v1"

// queueGroup is the queue group all endpoints register under so
// replicas load-balance uniformly. The name matches SERVICE.md §4.
const queueGroup = "natscontext"

// Server is a minimal in-process micro service that implements the
// svcbackend protocol against a wrapped natscontext.Backend.
type Server struct {
	svc     micro.Service
	backend natscontext.Backend
	prefix  string

	mu      sync.RWMutex
	xkey    nkeys.KeyPair
	xkeyPub string

	sysXKeyCalls atomic.Int64
}

// New starts a micro service on nc that serves backend over the
// svcbackend protocol using prefix (empty string selects the default).
func New(nc *nats.Conn, backend natscontext.Backend, prefix string) (*Server, error) {
	if prefix == "" {
		prefix = DefaultPrefix
	}

	kp, err := nkeys.CreateCurveKeys()
	if err != nil {
		return nil, fmt.Errorf("create xkey: %w", err)
	}

	pub, err := kp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("xkey public: %w", err)
	}

	svc, err := micro.AddService(nc, micro.Config{
		Name:        "natscontext-svcbackend-testserver",
		Version:     "0.0.0",
		Description: "in-process reference server for svcbackend contract tests",
		QueueGroup:  queueGroup,
	})
	if err != nil {
		return nil, fmt.Errorf("add service: %w", err)
	}

	s := &Server{
		svc:     svc,
		backend: backend,
		prefix:  prefix,
		xkey:    kp,
		xkeyPub: pub,
	}

	err = s.registerEndpoints()
	if err != nil {
		_ = svc.Stop()
		return nil, err
	}

	return s, nil
}

// PublicKey returns the current long-term xkey public key. Exposed
// so tests can skip sys.xkey discovery via svcbackend.WithServerKey.
func (s *Server) PublicKey() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.xkeyPub
}

// Rotate replaces the long-term xkey. Past ctx.load sealed responses
// remain decryptable by their original recipients (the client still
// holds its ephemeral priv). Past ctx.save plaintext can no longer
// be opened by the server.
func (s *Server) Rotate() error {
	kp, err := nkeys.CreateCurveKeys()
	if err != nil {
		return fmt.Errorf("create xkey: %w", err)
	}

	pub, err := kp.PublicKey()
	if err != nil {
		return fmt.Errorf("xkey public: %w", err)
	}

	s.mu.Lock()
	s.xkey = kp
	s.xkeyPub = pub
	s.mu.Unlock()

	return nil
}

// SysXKeyCalls returns the number of sys.xkey requests the server
// has answered. Tests use it to assert discovery happens exactly
// once per Client lifetime (plus once per rotation retry).
func (s *Server) SysXKeyCalls() int64 {
	return s.sysXKeyCalls.Load()
}

// Stop stops the micro service.
func (s *Server) Stop() error {
	return s.svc.Stop()
}

// registerEndpoints wires one micro endpoint per protocol subject.
// The endpoint name is the full subject so the registered subscription
// is unambiguous and micro's own metadata reflects the protocol.
func (s *Server) registerEndpoints() error {
	routes := []struct {
		name    string
		subject string
		handler micro.HandlerFunc
	}{
		{"sys_xkey", s.prefix + ".sys.xkey", s.handleSysXKey},
		{"ctx_load", s.prefix + ".ctx.load.*", s.handleLoad},
		{"ctx_save", s.prefix + ".ctx.save.*", s.handleSave},
		{"ctx_delete", s.prefix + ".ctx.delete.*", s.handleDelete},
		{"ctx_list", s.prefix + ".ctx.list", s.handleList},
	}

	for _, r := range routes {
		err := s.svc.AddEndpoint(
			r.name,
			r.handler,
			micro.WithEndpointSubject(r.subject),
			micro.WithEndpointQueueGroup(queueGroup),
		)
		if err != nil {
			return fmt.Errorf("register %s: %w", r.subject, err)
		}
	}

	return nil
}

// respondJSON marshals v and responds; any marshal or publish error
// is logged-and-dropped because a handler that does not call Respond
// hangs the client until its own timeout.
func respondJSON(req micro.Request, v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		_ = req.Respond(nil)
		return
	}
	_ = req.Respond(raw)
}

// nameFromSubject returns the final token of subject as a context
// name. The caller MUST validate the result before using it.
func nameFromSubject(subject string) string {
	idx := strings.LastIndex(subject, ".")
	if idx < 0 {
		return subject
	}
	return subject[idx+1:]
}

// -------- handlers --------

func (s *Server) handleSysXKey(req micro.Request) {
	s.sysXKeyCalls.Add(1)

	pub := s.PublicKey()

	respondJSON(req, svcbackend.SysXKeyResponse{XKeyPub: pub})
}

func (s *Server) handleLoad(req micro.Request) {
	name := nameFromSubject(req.Subject())

	err := natscontext.ValidateName(name)
	if err != nil {
		respondJSON(req, svcbackend.LoadResponse{Error: svcbackend.ErrorFromSentinel(err)})
		return
	}

	var in svcbackend.LoadRequest
	err = json.Unmarshal(req.Data(), &in)
	if err != nil {
		respondJSON(req, svcbackend.LoadResponse{Error: &svcbackend.ErrorResponse{Code: string(svcbackend.CodeInternal), Message: "decode request"}})
		return
	}

	data, err := s.backend.Load(context.Background(), name)
	if err != nil {
		respondJSON(req, svcbackend.LoadResponse{Error: svcbackend.ErrorFromSentinel(err)})
		return
	}

	sealed, err := s.sealForClient(in.ReplyPub, svcbackend.LoadSealed{Data: data, ReqID: in.ReqID})
	if err != nil {
		respondJSON(req, svcbackend.LoadResponse{Error: &svcbackend.ErrorResponse{Code: string(svcbackend.CodeInternal), Message: "seal reply"}})
		return
	}

	respondJSON(req, svcbackend.LoadResponse{Sealed: sealed})
}

func (s *Server) handleSave(req micro.Request) {
	name := nameFromSubject(req.Subject())

	err := natscontext.ValidateName(name)
	if err != nil {
		respondJSON(req, svcbackend.SaveResponse{Error: svcbackend.ErrorFromSentinel(err)})
		return
	}

	var in svcbackend.SaveRequest
	err = json.Unmarshal(req.Data(), &in)
	if err != nil {
		respondJSON(req, svcbackend.SaveResponse{Error: &svcbackend.ErrorResponse{Code: string(svcbackend.CodeInternal), Message: "decode request"}})
		return
	}

	var sealed svcbackend.SaveSealed
	err = s.openFromClient(in.SenderPub, in.Sealed, &sealed)
	if err != nil {
		respondJSON(req, svcbackend.SaveResponse{Error: &svcbackend.ErrorResponse{Code: string(svcbackend.CodeStaleKey), Message: "open sealed request"}})
		return
	}

	err = s.backend.Save(context.Background(), name, sealed.Data)
	if err != nil {
		respondJSON(req, svcbackend.SaveResponse{Error: svcbackend.ErrorFromSentinel(err)})
		return
	}

	respondJSON(req, svcbackend.SaveResponse{})
}

func (s *Server) handleDelete(req micro.Request) {
	name := nameFromSubject(req.Subject())

	err := natscontext.ValidateName(name)
	if err != nil {
		respondJSON(req, svcbackend.DeleteResponse{Error: svcbackend.ErrorFromSentinel(err)})
		return
	}

	err = s.backend.Delete(context.Background(), name)
	if err != nil {
		respondJSON(req, svcbackend.DeleteResponse{Error: svcbackend.ErrorFromSentinel(err)})
		return
	}

	respondJSON(req, svcbackend.DeleteResponse{})
}

func (s *Server) handleList(req micro.Request) {
	names, err := s.backend.List(context.Background())
	if err != nil {
		respondJSON(req, svcbackend.ListResponse{Error: svcbackend.ErrorFromSentinel(err)})
		return
	}

	respondJSON(req, svcbackend.ListResponse{Names: names})
}

// -------- crypto helpers --------
//
// The helpers below duplicate what svcbackend exposes internally;
// they live here because a conformant server in any language is
// expected to implement its own seal/open layer. Keeping the test
// server independent of svcbackend's unexported helpers makes this
// file serve as a reference for that contract.

var errNoCurrentKey = errors.New("testserver: no current xkey")

// sealForClient seals payload to recipientPub using the current
// long-term xkey as the sender.
func (s *Server) sealForClient(recipientPub string, payload svcbackend.LoadSealed) (string, error) {
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	s.mu.RLock()
	kp := s.xkey
	s.mu.RUnlock()

	if kp == nil {
		return "", errNoCurrentKey
	}

	ct, err := kp.Seal(plaintext, recipientPub)
	if err != nil {
		return "", fmt.Errorf("seal: %w", err)
	}

	return base64.StdEncoding.EncodeToString(ct), nil
}

// openFromClient opens a sealed SaveRequest payload using the current
// long-term xkey as the recipient.
func (s *Server) openFromClient(senderPub string, sealed string, out *svcbackend.SaveSealed) error {
	ct, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	s.mu.RLock()
	kp := s.xkey
	s.mu.RUnlock()

	if kp == nil {
		return errNoCurrentKey
	}

	plaintext, err := kp.Open(ct, senderPub)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}

	err = json.Unmarshal(plaintext, out)
	if err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	return nil
}
