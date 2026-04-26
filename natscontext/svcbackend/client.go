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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"

	"github.com/nats-io/jsm.go/natscontext"
)

// DefaultPrefix is the subject prefix every conforming server honors
// unless an operator has configured a different prefix.
const DefaultPrefix = "natscontext.v1"

// DefaultTimeout is the per-request deadline used when WithTimeout
// is not supplied.
const DefaultTimeout = 5 * time.Second

// ClientOption configures a Client at construction time.
type ClientOption func(*Client)

// WithTimeout sets the per-request timeout applied when the caller's
// context.Context has no deadline. The default is DefaultTimeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) { c.timeout = d }
}

// WithSubjectPrefix overrides the default "natscontext.v1" subject
// prefix. The override MUST match the prefix the paired server was
// started with.
func WithSubjectPrefix(prefix string) ClientOption {
	return func(c *Client) { c.prefix = prefix }
}

// WithServerKey pre-seeds the cached server xkey public key, skipping
// the sys.xkey discovery round-trip. Intended for tests and fully
// air-gapped deployments.
func WithServerKey(xkeyPub string) ClientOption {
	return func(c *Client) { c.serverPub = xkeyPub }
}

// Client is the reference svcbackend client. It implements
// natscontext.Backend and io.Closer.
//
// The Client deliberately does NOT implement natscontext.Selector.
// Selection is a personal, per-machine choice that a shared service
// backend should not carry; callers construct a Registry that pairs
// this Client with a local Selector (for example
// natscontext.WithLocalSelector) or with WithoutSelection when no
// selection tracking is required.
//
// The caller owns the *nats.Conn; Close tears down per-client state
// but does not close the connection.
type Client struct {
	nc      *nats.Conn
	prefix  string
	timeout time.Duration

	mu        sync.Mutex
	serverPub string
	closed    bool
}

// NewClient returns a Client ready to serve Backend and Selector
// calls against a conforming svcbackend service reachable via nc.
// The server xkey is lazy-fetched on first encrypted call unless
// WithServerKey pre-seeds it.
func NewClient(nc *nats.Conn, opts ...ClientOption) (*Client, error) {
	if nc == nil {
		return nil, errors.New("svcbackend: nil *nats.Conn")
	}

	c := &Client{
		nc:      nc,
		prefix:  DefaultPrefix,
		timeout: DefaultTimeout,
	}
	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// Close releases per-client state. It is safe to call multiple times.
// Close does not touch the *nats.Conn.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.serverPub = ""
	return nil
}

// Load fetches the context payload for name. The request is plaintext;
// the response is sealed with the server's long-term xkey and opened
// with a per-request client ephemeral.
func (c *Client) Load(ctx context.Context, name string) ([]byte, error) {
	err := validateWireName(name)
	if err != nil {
		return nil, err
	}

	serverPub, err := c.ensureServerPub(ctx)
	if err != nil {
		return nil, err
	}

	data, err := c.loadOnce(ctx, name, serverPub)
	if err == nil {
		return data, nil
	}

	// A failure opening the sealed reply indicates the server rotated
	// its xkey. Invalidate, refetch once, retry once. Any other error
	// (envelope-level) propagates unchanged.
	var de decryptError
	if !errors.As(err, &de) {
		return nil, err
	}

	c.invalidateServerPub(serverPub)

	serverPub, refetchErr := c.ensureServerPub(ctx)
	if refetchErr != nil {
		return nil, fmt.Errorf("refetch server xkey after decrypt failure: %w", refetchErr)
	}

	data, err = c.loadOnce(ctx, name, serverPub)
	if err != nil {
		return nil, fmt.Errorf("load failed after server xkey refresh: %w", err)
	}

	return data, nil
}

// Save writes data for name. The plaintext is sealed to the cached
// server xkey with a per-request client ephemeral. A stale_key reply
// (server rotated) triggers a single invalidate/refetch/retry per
// PROTOCOL.md §9.4.
func (c *Client) Save(ctx context.Context, name string, data []byte) error {
	err := validateWireName(name)
	if err != nil {
		return err
	}

	serverPub, err := c.ensureServerPub(ctx)
	if err != nil {
		return err
	}

	er, err := c.saveOnce(ctx, name, data, serverPub)
	if err != nil {
		return err
	}
	if er == nil {
		return nil
	}
	if er.Code != string(CodeStaleKey) {
		return SentinelFromError(er)
	}

	c.invalidateServerPub(serverPub)

	refreshed, refetchErr := c.ensureServerPub(ctx)
	if refetchErr != nil {
		return fmt.Errorf("refetch server xkey after stale_key: %w", refetchErr)
	}
	if refreshed == serverPub {
		return errors.New("svcbackend: sys.xkey returned the same stale key")
	}

	er, err = c.saveOnce(ctx, name, data, refreshed)
	if err != nil {
		return fmt.Errorf("save failed after server xkey refresh: %w", err)
	}
	if er != nil {
		return fmt.Errorf("save failed after server xkey refresh: %w", SentinelFromError(er))
	}
	return nil
}

// saveOnce issues a single ctx.save round-trip against serverPub.
// Each call generates a fresh req_id and a fresh ephemeral keypair so
// a retry does not look like a replay on the wire and does not share
// a sender_pub with the original attempt. A non-nil ErrorResponse is
// a protocol-level reply the caller can inspect (including stale_key
// for the rotation retry path); a non-nil error is a transport-level
// failure.
func (c *Client) saveOnce(ctx context.Context, name string, data []byte, serverPub string) (*ErrorResponse, error) {
	reqID, err := newReqID()
	if err != nil {
		return nil, err
	}

	ephPriv, ephPub, err := newEphemeral()
	if err != nil {
		return nil, err
	}
	defer ephPriv.Wipe()

	sealed, err := sealSaveRequest(ephPriv, serverPub, SaveSealed{Data: data, ReqID: reqID})
	if err != nil {
		return nil, err
	}

	env := SaveRequest{SenderPub: ephPub, Sealed: sealed}

	var resp SaveResponse
	err = c.roundTrip(ctx, c.prefix+".ctx.save."+name, env, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Error, nil
}

// Delete removes name. A missing name is a no-op; invalid names
// fast-fail without a network round-trip.
func (c *Client) Delete(ctx context.Context, name string) error {
	err := validateWireName(name)
	if err != nil {
		return err
	}

	reqID, err := newReqID()
	if err != nil {
		return err
	}

	var resp DeleteResponse
	err = c.roundTrip(ctx, c.prefix+".ctx.delete."+name, DeleteRequest{ReqID: reqID}, &resp)
	if err != nil {
		return err
	}

	return SentinelFromError(resp.Error)
}

// List returns the sorted names of every stored context.
func (c *Client) List(ctx context.Context) ([]string, error) {
	var resp ListResponse
	err := c.roundTrip(ctx, c.prefix+".ctx.list", ListRequest{}, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, SentinelFromError(resp.Error)
	}
	return resp.Names, nil
}

// decryptError marks a Load failure caused by Open returning an
// error. Tagging it lets Load distinguish rotation (retry) from
// other transport errors (propagate).
type decryptError struct{ err error }

func (d decryptError) Error() string { return d.err.Error() }
func (d decryptError) Unwrap() error { return d.err }

// loadOnce issues a single ctx.load round-trip against serverPub and
// returns the payload. A failure to open the sealed response is
// wrapped in decryptError.
func (c *Client) loadOnce(ctx context.Context, name, serverPub string) ([]byte, error) {
	ephPriv, ephPub, err := newEphemeral()
	if err != nil {
		return nil, err
	}
	defer ephPriv.Wipe()

	reqID, err := newReqID()
	if err != nil {
		return nil, err
	}

	env := LoadRequest{ReplyPub: ephPub, ReqID: reqID}

	var resp LoadResponse
	err = c.roundTrip(ctx, c.prefix+".ctx.load."+name, env, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		// A stale_key reply means a hypothetical server-side failure on
		// the sealed path that mirrors the client-side Open failure
		// handled below; route it through the same rotation retry.
		if resp.Error.Code == string(CodeStaleKey) {
			return nil, decryptError{err: fmt.Errorf("server reported stale_key: %s", resp.Error.Message)}
		}
		return nil, SentinelFromError(resp.Error)
	}

	sealed, err := openLoadReply(ephPriv, serverPub, resp.Sealed)
	if err != nil {
		return nil, decryptError{err: err}
	}

	if sealed.ReqID != reqID {
		return nil, decryptError{err: fmt.Errorf("req_id mismatch in sealed reply")}
	}

	return sealed.Data, nil
}

// roundTrip marshals req, sends it on subject, and decodes the reply
// into respOut. It honors caller context cancellation and falls back
// to c.timeout for requests without a deadline.
func (c *Client) roundTrip(ctx context.Context, subject string, req any, respOut any) error {
	err := c.checkOpen()
	if err != nil {
		return err
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	reqCtx, cancel := c.requestContext(ctx)
	defer cancel()

	msg, err := c.nc.RequestWithContext(reqCtx, subject, data)
	if err != nil {
		return fmt.Errorf("nats request %s: %w", subject, err)
	}

	err = json.Unmarshal(msg.Data, respOut)
	if err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	return nil
}

// requestContext returns ctx unchanged if it already has a deadline,
// otherwise it wraps ctx with c.timeout so a slow or missing server
// cannot hang the caller indefinitely.
func (c *Client) requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	_, hasDeadline := ctx.Deadline()
	if hasDeadline {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, c.timeout)
}

// ensureServerPub returns the cached server xkey public key, fetching
// it via sys.xkey under the client mutex if the cache is empty. The
// lock is held across the round-trip so concurrent first calls from
// different goroutines coalesce into a single fetch.
func (c *Client) ensureServerPub(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return "", errors.New("svcbackend: client closed")
	}

	if c.serverPub != "" {
		return c.serverPub, nil
	}

	pub, err := c.fetchServerPub(ctx)
	if err != nil {
		return "", err
	}

	c.serverPub = pub
	return pub, nil
}

// fetchServerPub issues the sys.xkey round-trip. The caller MUST
// hold c.mu.
func (c *Client) fetchServerPub(ctx context.Context) (string, error) {
	data, err := json.Marshal(SysXKeyRequest{})
	if err != nil {
		return "", fmt.Errorf("marshal sys.xkey: %w", err)
	}

	reqCtx, cancel := c.requestContext(ctx)
	defer cancel()

	msg, err := c.nc.RequestWithContext(reqCtx, c.prefix+".sys.xkey", data)
	if err != nil {
		return "", fmt.Errorf("nats request sys.xkey: %w", err)
	}

	var resp SysXKeyResponse
	err = json.Unmarshal(msg.Data, &resp)
	if err != nil {
		return "", fmt.Errorf("unmarshal sys.xkey: %w", err)
	}

	if resp.Error != nil {
		return "", SentinelFromError(resp.Error)
	}
	if resp.XKeyPub == "" {
		return "", errors.New("svcbackend: sys.xkey returned empty xkey_pub")
	}

	return resp.XKeyPub, nil
}

// invalidateServerPub clears the cached server xkey only if it still
// matches the stale value we observed, avoiding a race where another
// goroutine refetched a new key in between.
func (c *Client) invalidateServerPub(stale string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.serverPub == stale {
		c.serverPub = ""
	}
}

// checkOpen returns a sentinel error when the client is closed.
func (c *Client) checkOpen() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.New("svcbackend: client closed")
	}
	return nil
}

// subjectHostileRE matches any character that cannot appear in a
// single NATS subject token: whitespace (which terminates a subject
// in parsing), control characters, the token separator ".", and the
// subject wildcards "*" and ">". The core natscontext.ValidateName
// rule is deliberately loose for historical compat, so any of these
// characters may legally reach the svcbackend client; we reject them
// up-front with ErrInvalidName rather than publish a malformed or
// wildcard-matching subject.
var subjectHostileRE = regexp.MustCompile(`[\s.*>\x00-\x1f\x7f]`)

// validateWireName layers svcbackend-specific rejections on top of
// the portable natscontext.ValidateName. The wire protocol embeds
// the context name as the final token of a NATS subject, which
// constrains the character set beyond what the core validator
// permits. The client fast-fails rather than publishing a subject
// the server's single-token wildcard subscription would never match
// (or worse, a subject whose wildcards leak a request onto an
// unintended tree).
func validateWireName(name string) error {
	err := natscontext.ValidateName(name)
	if err != nil {
		return err
	}
	if subjectHostileRE.MatchString(name) {
		return fmt.Errorf(`%w: %q must not contain whitespace, control characters, or any of . * > — the svcbackend wire protocol carries the name as a single NATS subject token`, natscontext.ErrInvalidName, name)
	}
	return nil
}

// newEphemeral generates a per-request ephemeral curve keypair and
// returns the keypair plus its encoded public key.
func newEphemeral() (nkeys.KeyPair, string, error) {
	kp, err := nkeys.CreateCurveKeys()
	if err != nil {
		return nil, "", fmt.Errorf("create curve keys: %w", err)
	}

	pub, err := kp.PublicKey()
	if err != nil {
		kp.Wipe()
		return nil, "", fmt.Errorf("public key: %w", err)
	}

	return kp, pub, nil
}
