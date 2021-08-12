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

// Package election is a JetStream backed leader election system.
//
// It lets one or more processes elect a leader based on competing for
// ownership of a JetStream consumer on a Stream configured to allow maximum
// 1 consumer per stream
//
// JetStream will monitor the health of the leadership by observing the state of
// of a subscription - as long as the subscription exist the leadership is held.
// Subscription propagation is a core capability in NATS and an extremely reliable
// way to determine the health of a connected NATS client.  Once the subscription
// is not valid anymore the Ephemeral consumer is removed, ready for a new leader
// to recreate it. A short grace period exists allowing for short client connection
// interruptions.
//
// The leader will monitor Heartbeats sent from the Ephemeral consumer to it, if
// a number of these heartbeats are missed it will assume it lost access and the
// consumer will be removed. It will then trigger a process of standing down from
// leadership
//
// Together this creates a leadership election system that's highly available and
// works across clusters, super cluster and leafnodes.
//
// This feature is experimental
package election

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nuid"
)

type election struct {
	opts     *options
	last     time.Time
	nc       *nats.Conn
	sub      *nats.Subscription
	isLeader bool
	tries    int
	mgr      *jsm.Manager
	ctx      context.Context
	cancel   func()
	nuid     *nuid.NUID
	mu       sync.Mutex
}

// Backoff controls the interval of campaigns
type Backoff interface {
	// Duration returns the time to sleep for the nth invocation
	Duration(n int) time.Duration
}

// NewElection creates a leader election, it will either create/join a stream called streamName or use the pre-made stream passed as option
func NewElection(name string, wonCb func(), lostCb func(), streamName string, mgr *jsm.Manager, opts ...Option) (*election, error) {
	o, err := newOptions(opts...)
	if err != nil {
		return nil, err
	}

	o.streamName = streamName
	o.name = name
	o.wonCb = wonCb
	o.lostCb = lostCb

	e := &election{
		opts: o,
		mgr:  mgr,
		nc:   mgr.NatsConn(),
		nuid: nuid.New(),
	}

	if o.stream == nil {
		_, err := e.createStream()
		if err != nil {
			return nil, err
		}
	}

	return e, nil
}

func (e *election) debug(format string, a ...interface{}) {
	if e.opts.debug == nil {
		return
	}

	e.opts.debug(fmt.Sprintf(format, a...))
}

func (e *election) createStream() (*jsm.Stream, error) {
	if e.opts.streamName == "" {
		return nil, fmt.Errorf("stream name is required")
	}

	sopts := []jsm.StreamOption{
		jsm.Subjects(fmt.Sprintf("%s.%s.i", e.opts.subjectPrefix, e.opts.streamName)),
		jsm.MaxConsumers(1),
		jsm.MaxMessages(1),
		jsm.DiscardOld(),
		jsm.LimitsRetention(),
	}
	if e.opts.storageType == memoryStorage {
		sopts = append(sopts, jsm.MemoryStorage())
	} else {
		sopts = append(sopts, jsm.FileStorage())
	}

	var err error

	stream, err := e.mgr.LoadOrNewStream(e.opts.streamName, sopts...)
	if err != nil {
		return nil, err
	}

	e.opts.Lock()
	e.opts.stream = stream
	e.opts.Unlock()

	e.debug("Created or loaded stream %s with configuration %+v", stream.Name(), stream.Configuration())

	return stream, nil
}

func (e *election) subscribe() (*nats.Subscription, error) {
	e.mu.Lock()
	if e.sub != nil {
		e.sub.Unsubscribe()
		e.sub = nil
	}
	e.mu.Unlock()

	sub, err := e.nc.Subscribe(fmt.Sprintf("%s.%s.m.%s", e.opts.subjectPrefix, e.opts.streamName, e.nuid.Next()), func(m *nats.Msg) {
		e.mu.Lock()
		e.last = time.Now()
		e.mu.Unlock()
		e.debug("Received heartbeat on %s", m.Sub.Subject)
	})
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	e.sub = sub
	e.mu.Unlock()

	e.debug("Subscribed to %s", sub.Subject)

	return sub, nil
}

func (e *election) manageLeadership(wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(10 * time.Millisecond)

	lost := func(reason string) {
		e.debug("Lost leadership: %s", reason)
		ticker.Stop()
		e.mu.Lock()
		e.opts.lostCb()
		e.sub.Unsubscribe()
		e.sub = nil
		e.isLeader = false
		e.tries = 0
		e.mu.Unlock()
	}

	for {
		select {
		case <-ticker.C:
			e.mu.Lock()
			seen := e.last
			e.mu.Unlock()

			if time.Since(seen) >= time.Duration(e.opts.missedHBThreshold)*e.opts.hbInterval {
				lost(fmt.Sprintf("heartbeat threshold missed, last seen: %v", time.Since(seen)))
				return
			}

		case <-e.ctx.Done():
			lost("context canceled")
			return
		}
	}
}

func ctxSleep(ctx context.Context, duration time.Duration) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	sctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	<-sctx.Done()

	return ctx.Err()
}

func (e *election) isLeaderUnlocked() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.isLeader
}

func (e *election) getStream() *jsm.Stream {
	e.opts.Lock()
	defer e.opts.Unlock()

	return e.opts.stream
}

func (e *election) standDownGrace(ctx context.Context) error {
	return ctxSleep(ctx, time.Duration(e.opts.missedHBThreshold)*e.opts.hbInterval)
}

func (e *election) campaign(wg *sync.WaitGroup) {
	try := func() (int, error) {
		e.mu.Lock()
		leader := e.isLeader
		sub := e.sub
		ctx := e.ctx
		e.tries++
		try := e.tries
		e.mu.Unlock()

		e.debug("Campaigning for leadership try %d", try)

		var err error

		if leader {
			return try, nil
		}

		if sub == nil {
			sub, err = e.subscribe()
			if err != nil {
				return try, err
			}
		}

		stream := e.getStream()
		if stream == nil {
			stream, err = e.createStream()
			if err != nil {
				return try, err
			}
		}

		cons, err := stream.NewConsumer(
			jsm.IdleHeartbeat(e.opts.hbInterval),
			jsm.DeliverySubject(sub.Subject),
			jsm.ConsumerDescription(fmt.Sprintf("Candidate %s", e.opts.name)),
		)
		if err != nil {
			if aerr, ok := err.(api.ApiError); ok {
				if aerr.NatsErrorCode() == 10059 { // stream not found error
					e.createStream()
				}
			}

			return try, err
		}

		e.debug("Became leader on consumer %s with hb interval %v", cons.Name(), cons.Heartbeat())

		e.mu.Lock()
		e.last = time.Now()
		e.mu.Unlock()

		// give a bit of grace to the past leader to realize is not leader anymore
		err = e.standDownGrace(ctx)
		if err != nil {
			return try, err
		}

		wg.Add(1)
		go e.manageLeadership(wg)

		e.mu.Lock()
		e.debug("Became leader on try %d", try)
		e.isLeader = true
		e.opts.wonCb()
		e.tries = 0
		e.mu.Unlock()

		return 0, nil
	}

	time.Sleep(time.Duration(rand.Intn(int(e.opts.campaignInterval.Milliseconds()))))
	try()

	var ticker *time.Ticker
	if e.opts.bo != nil {
		ticker = time.NewTicker(e.opts.bo.Duration(0))
	} else {
		ticker = time.NewTicker(e.opts.campaignInterval)
	}

	for {
		select {
		case <-ticker.C:
			if e.isLeaderUnlocked() {
				continue
			}

			c, _ := try()
			if e.opts.bo != nil {
				ticker.Reset(e.opts.bo.Duration(c))
			}

		case <-e.ctx.Done():
			ticker.Stop()
			return
		}
	}
}

// Start begins campaigning for for leadership, this will function will block and continue until ctx is canceled or Stop() is called
func (e *election) Start(ctx context.Context) {
	e.mu.Lock()
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.mu.Unlock()

	wg := &sync.WaitGroup{}

	e.campaign(wg)

	wg.Wait()
}

// Stop can optionally be called to stop the leadership election system without canceling the context passed to Start()
func (e *election) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cancel != nil {
		e.cancel()
	}

	if e.sub != nil {
		e.sub.Unsubscribe()
		e.sub = nil
	}

	if e.opts.lostCb != nil {
		e.opts.lostCb()
	}
}
