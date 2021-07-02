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

// Package governor controls the concurrency of a network wide process
//
// Using this one can, for example, create CRON jobs that can trigger
// 100s or 1000s concurrently but where most will wait for a set limit
// to complete.  In effect limiting the overall concurrency of these
// execution.
//
// To do this a Stream is created that has a maximum message limit and
// that will reject new entries when full.
//
// Workers will try to place themselves in the Stream, they do their work
// if they succeed and remove themselves from the Stream once they are done.
//
// As a fail safe the stack will evict entries after a set time based on
// Stream max age.
//
// A manager is included to create, observe and edit these streams and the
// nats CLI has a new command build on this library: nats governor
package governor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
)

// DefaultInterval default sleep between tries, set with WithInterval()
const DefaultInterval = 250 * time.Millisecond

// Finisher signals that work is completed releasing the slot on the stack
type Finisher func() error

// Governor controls concurrency of distributed processes using a named governor stream
type Governor interface {
	// Start attempts to get a spot in the Governor, gives up on context, call Finisher to signal end of work
	Start(ctx context.Context, name string) (Finisher, error)
}

// Manager controls concurrent executions of work distributed throughout a nats network by using
// a stream as a capped stack where workers reserve a slot and later release the slot
type Manager interface {
	Limit() int64
	SetLimit(uint64) error
	MaxAge() time.Duration
	SetMaxAge(time.Duration) error
	Name() string
	Stream() *jsm.Stream
}

// Backoff controls the interval of checks
type Backoff interface {
	// Duration returns the time to sleep for the nth invocation
	Duration(n int) time.Duration
}

type jsGMgr struct {
	name     string
	stream   string
	maxAge   time.Duration
	limit    uint64
	mgr      *jsm.Manager
	nc       *nats.Conn
	str      *jsm.Stream
	subj     string
	replicas int
	running  bool

	cint time.Duration
	bo   Backoff

	mu sync.Mutex
}

func NewJSGovernorManager(name string, limit uint64, maxAge time.Duration, replicas uint, mgr *jsm.Manager, update bool) (Manager, error) {
	gov := &jsGMgr{
		name:     name,
		maxAge:   maxAge,
		limit:    limit,
		mgr:      mgr,
		nc:       mgr.NatsConn(),
		replicas: int(replicas),
		cint:     DefaultInterval,
	}

	gov.stream = gov.streamName()
	gov.subj = gov.streamSubject()

	err := gov.loadOrCreate(update)
	if err != nil {
		return nil, err
	}

	return gov, nil
}

type Option func(mgr *jsGMgr)

// WithBackoff sets a backoff policy for gradually reducing try interval
func WithBackoff(p Backoff) Option {
	return func(mgr *jsGMgr) {
		mgr.bo = p
	}
}

// WithInterval sets the interval between tries
func WithInterval(i time.Duration) Option {
	return func(mgr *jsGMgr) {
		mgr.cint = i
	}
}

func NewJSGovernor(name string, mgr *jsm.Manager, opts ...Option) Governor {
	gov := &jsGMgr{
		name: name,
		mgr:  mgr,
		nc:   mgr.NatsConn(),
		cint: DefaultInterval,
	}

	for _, o := range opts {
		o(gov)
	}

	gov.stream = gov.streamName()
	gov.subj = gov.streamSubject()

	return gov
}

func (g *jsGMgr) streamSubject() string {
	if g.subj != "" {
		return g.subj
	}

	return fmt.Sprintf("$GOVERNOR.campaign.%s", g.name)
}

func (g *jsGMgr) streamName() string {
	if g.stream != "" {
		return g.stream
	}

	return StreamName(g.name)
}

func StreamName(governor string) string {
	return fmt.Sprintf("GOVERNOR_%s", governor)
}

func (g *jsGMgr) Start(ctx context.Context, name string) (Finisher, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.running = true
	seq := uint64(0)
	tries := 0

	try := func() error {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		m, err := g.nc.RequestWithContext(ctx, g.subj, []byte(name))
		if err != nil {
			return err
		}

		res, err := jsm.ParsePubAck(m)
		if err != nil {
			return err
		}

		seq = res.Sequence

		return nil
	}

	closer := func() error {
		if seq == 0 {
			return nil
		}

		err := g.mgr.DeleteStreamMessage(g.stream, seq, true)
		if err != nil {
			return fmt.Errorf("could not remove seq %d: %s", seq, err)
		}

		return nil
	}

	err := try()
	if err == nil {
		return closer, nil
	}

	ticker := time.NewTicker(g.cint)

	for {
		select {
		case <-ticker.C:
			tries++
			err = try()
			if err == nil {
				return closer, nil
			}

			if g.bo != nil {
				ticker.Reset(g.bo.Duration(tries))
			}

		case <-ctx.Done():
			ticker.Stop()
			return nil, ctx.Err()
		}
	}
}

func (g *jsGMgr) Stream() *jsm.Stream   { return g.str }
func (g *jsGMgr) Limit() int64          { return g.str.MaxMsgs() }
func (g *jsGMgr) MaxAge() time.Duration { return g.str.MaxAge() }
func (g *jsGMgr) Name() string          { return g.name }
func (g *jsGMgr) SetLimit(limit uint64) error {
	g.mu.Lock()
	g.limit = limit
	g.mu.Unlock()

	return g.updateConfig()
}

func (g *jsGMgr) SetMaxAge(age time.Duration) error {
	g.mu.Lock()
	g.maxAge = age
	g.mu.Unlock()

	return g.updateConfig()
}

func (g *jsGMgr) updateConfig() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.str.MaxAge() != g.maxAge || g.str.MaxMsgs() != int64(g.limit) {
		err := g.str.UpdateConfiguration(g.str.Configuration(), g.streamOpts()...)
		if err != nil {
			return fmt.Errorf("stream update failed: %s", err)
		}
	}

	return nil
}

func (g *jsGMgr) streamOpts() []jsm.StreamOption {
	opts := []jsm.StreamOption{
		jsm.MaxAge(g.maxAge),
		jsm.MaxMessages(int64(g.limit)),
		jsm.Subjects(g.subj),
		jsm.Replicas(g.replicas),
		jsm.LimitsRetention(),
		jsm.FileStorage(),
		jsm.DiscardNew(),
		jsm.DuplicateWindow(0),
	}

	if g.replicas > 0 {
		opts = append(opts, jsm.Replicas(g.replicas))
	}

	return opts
}

func (g *jsGMgr) loadOrCreate(update bool) error {
	opts := g.streamOpts()

	str, err := g.mgr.LoadOrNewStream(g.stream, opts...)
	if err != nil {
		return err
	}

	g.str = str

	if update {
		g.updateConfig()
	}

	return nil
}
