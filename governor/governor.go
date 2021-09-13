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

	"github.com/google/go-cmp/cmp"
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
	Start(ctx context.Context, name string) (fin Finisher, seq uint64, err error)
}

// Manager controls concurrent executions of work distributed throughout a nats network by using
// a stream as a capped stack where workers reserve a slot and later release the slot
type Manager interface {
	// Limit is the configured maximum entries in the Governor
	Limit() int64
	// MaxAge is the time after which entries will be evicted
	MaxAge() time.Duration
	// Name is the Governor name
	Name() string
	// Replicas is how many data replicas are kept of the data
	Replicas() int
	// SetLimit configures the maximum entries in the Governor and takes immediate effect
	SetLimit(uint64) error
	// SetMaxAge configures the maximum age of entries, takes immediate effect
	SetMaxAge(time.Duration) error
	// SetSubject configures the underlying NATS subject the Governor listens on for entry campaigns
	SetSubject(subj string) error
	// Stream is the underlying JetStream stream
	Stream() *jsm.Stream
	// Subject is the subject the Governor listens on for entry campaigns
	Subject() string
	// Reset resets the governor removing all current entries from it
	Reset() error
	// Active is the number of active entries in the Governor
	Active() (uint64, error)
	// Evict removes an entry from the Governor given its unique id, returns the name that was on that entry
	Evict(entry uint64) (name string, err error)
	// LastActive returns the the since entry was added to the Governor, can be zero time when no entries were added
	LastActive() (time.Time, error)
}

// Backoff controls the interval of checks
type Backoff interface {
	// Duration returns the time to sleep for the nth invocation
	Duration(n int) time.Duration
}

// Logger is a custom logger
type Logger interface {
	Debugf(format string, a ...interface{})
	Infof(format string, a ...interface{})
	Warnf(format string, a ...interface{})
	Errorf(format string, a ...interface{})
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
	noCreate bool

	logger Logger
	cint   time.Duration
	bo     Backoff

	mu sync.Mutex
}

func NewJSGovernorManager(name string, limit uint64, maxAge time.Duration, replicas uint, mgr *jsm.Manager, update bool, opts ...Option) (Manager, error) {
	gov := &jsGMgr{
		name:     name,
		maxAge:   maxAge,
		limit:    limit,
		mgr:      mgr,
		nc:       mgr.NatsConn(),
		replicas: int(replicas),
		cint:     DefaultInterval,
	}

	for _, opt := range opts {
		opt(gov)
	}

	if limit == 0 {
		gov.noCreate = true
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

// WithLogger configures the logger to use, no logging when none is given
func WithLogger(log Logger) Option {
	return func(mgr *jsGMgr) {
		mgr.logger = log
	}
}

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

// WithSubject configures a specific subject for the governor to act on
func WithSubject(s string) Option {
	return func(mgr *jsGMgr) {
		mgr.subj = s
	}
}

func NewJSGovernor(name string, mgr *jsm.Manager, opts ...Option) Governor {
	gov := &jsGMgr{
		name: name,
		mgr:  mgr,
		nc:   mgr.NatsConn(),
		cint: DefaultInterval,
	}

	for _, opt := range opts {
		opt(gov)
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

func (g *jsGMgr) Start(ctx context.Context, name string) (Finisher, uint64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.running {
		return nil, 0, fmt.Errorf("already running")
	}

	g.running = true
	seq := uint64(0)
	tries := 0

	try := func() error {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		g.Debugf("Publishing to %s", g.subj)
		m, err := g.nc.RequestWithContext(ctx, g.subj, []byte(name))
		if err != nil {
			g.Errorf("Publishing to governor %s via %s failed: %s", g.name, g.subj, err)
			return err
		}

		res, err := jsm.ParsePubAck(m)
		if err != nil {
			if !jsm.IsNatsError(err, 10077) {
				g.Errorf("Invalid pub ack: %s", err)
			}

			return err
		}

		seq = res.Sequence

		g.Infof("Got a slot on %s with sequence %d", g.name, seq)

		return nil
	}

	closer := func() error {
		if seq == 0 {
			return nil
		}

		g.mu.Lock()
		defer g.mu.Unlock()
		if !g.running {
			return nil
		}

		g.running = false

		g.Infof("Removing self from %s sequence %d", g.name, seq)
		err := g.mgr.DeleteStreamMessage(g.stream, seq, true)
		if err != nil {
			g.Errorf("Could not remove self from %s: %s", g.name, err)
			return fmt.Errorf("could not remove seq %d: %s", seq, err)
		}

		return nil
	}

	g.Debugf("Starting to campaign every %v for a slot on %s using %s", g.cint, g.name, g.subj)

	err := try()
	if err == nil {
		return closer, seq, nil
	}

	ticker := time.NewTicker(g.cint)

	for {
		select {
		case <-ticker.C:
			tries++
			err = try()
			if err == nil {
				return closer, seq, nil
			}

			if g.bo != nil {
				ticker.Reset(g.bo.Duration(tries))
			}

		case <-ctx.Done():
			g.Infof("Stopping campaigns against %s due to context timeout after %d tries", g.name, tries)
			ticker.Stop()
			return nil, 0, ctx.Err()
		}
	}
}

func (g *jsGMgr) Reset() error {
	return g.str.Purge()
}
func (g *jsGMgr) Stream() *jsm.Stream   { return g.str }
func (g *jsGMgr) Limit() int64          { return g.str.MaxMsgs() }
func (g *jsGMgr) MaxAge() time.Duration { return g.str.MaxAge() }
func (g *jsGMgr) Subject() string       { return g.str.Subjects()[0] }
func (g *jsGMgr) Replicas() int         { return g.str.Replicas() }
func (g *jsGMgr) Name() string          { return g.name }
func (g *jsGMgr) Evict(entry uint64) (string, error) {
	msg, err := g.str.ReadMessage(entry)
	if err != nil {
		return "", err
	}

	return string(msg.Data), g.str.DeleteMessage(entry)
}

func (g *jsGMgr) Active() (uint64, error) {
	nfo, err := g.str.Information()
	if err != nil {
		return 0, err
	}

	return nfo.State.Msgs, nil
}

func (g *jsGMgr) LastActive() (time.Time, error) {
	nfo, err := g.str.Information()
	if err != nil {
		return time.Time{}, err
	}

	return nfo.State.LastTime, nil
}

func (g *jsGMgr) SetSubject(subj string) error {
	g.mu.Lock()
	g.subj = subj
	g.mu.Unlock()

	return g.updateConfig()
}

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

	if g.str.MaxAge() != g.maxAge || g.str.MaxMsgs() != int64(g.limit) || !cmp.Equal([]string{g.streamSubject()}, g.str.Subjects()) {
		err := g.str.UpdateConfiguration(g.str.Configuration(), g.streamOpts()...)
		if err != nil {
			return fmt.Errorf("stream update failed: %s", err)
		}
	}

	return nil
}

func (g *jsGMgr) streamOpts() []jsm.StreamOption {
	opts := []jsm.StreamOption{
		jsm.StreamDescription(fmt.Sprintf("Concurrency Governor %s", g.name)),
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

	if g.noCreate {
		has, err := g.mgr.IsKnownStream(g.stream)
		if err != nil {
			return err
		}

		if !has {
			return fmt.Errorf("unknown governor")
		}
	}

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

func (g *jsGMgr) Debugf(format string, a ...interface{}) {
	if g.logger == nil {
		return
	}
	g.logger.Debugf(format, a...)
}

func (g *jsGMgr) Infof(format string, a ...interface{}) {
	if g.logger == nil {
		return
	}
	g.logger.Infof(format, a...)
}

func (g *jsGMgr) Warnf(format string, a ...interface{}) {
	if g.logger == nil {
		return
	}
	g.logger.Warnf(format, a...)
}

func (g *jsGMgr) Errorf(format string, a ...interface{}) {
	if g.logger == nil {
		return
	}

	g.logger.Errorf(format, a...)
}
