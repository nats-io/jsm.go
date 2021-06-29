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
	"sync"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nuid"
)

type election struct {
	opts     *options
	last     time.Time
	nc       *nats.Conn
	sub      *nats.Subscription
	isLeader bool
	ctx      context.Context
	cancel   func()
	nuid     *nuid.NUID
	mu       sync.Mutex
}

// NewElection creates a leader election, it will either create/join a stream called streamName or use the pre-made stream passed as option
func NewElection(name string, wonCb func(), lostCb func(), streamName string, mgr *jsm.Manager, opts ...Option) (*election, error) {
	o, err := newOptions(opts...)
	if err != nil {
		return nil, err
	}

	o.name = name
	o.wonCb = wonCb
	o.lostCb = lostCb

	if o.stream == nil {
		if streamName == "" {
			return nil, fmt.Errorf("stream name is required")
		}

		o.stream, err = mgr.LoadOrNewStream(streamName, jsm.FileStorage(), jsm.MaxConsumers(1), jsm.MaxMessages(1), jsm.DiscardOld(), jsm.LimitsRetention())
		if err != nil {
			return nil, err
		}
	}

	return &election{
		opts: o,
		nc:   mgr.NatsConn(),
		nuid: nuid.New(),
	}, nil
}

func (e *election) subscribe() (*nats.Subscription, error) {
	sub, err := e.nc.Subscribe(fmt.Sprintf("$LE.m.%s.%s", e.nuid.Next(), e.opts.name), func(m *nats.Msg) {
		e.mu.Lock()
		e.last = time.Now()
		e.mu.Unlock()
	})
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	e.sub = sub
	e.mu.Unlock()

	return sub, nil
}

func (e *election) manageLeadership(wg *sync.WaitGroup) {
	defer wg.Done()

	ticker := time.NewTicker(10 * time.Millisecond)

	lost := func() {
		ticker.Stop()
		e.mu.Lock()
		e.sub.Unsubscribe()
		e.isLeader = false
		e.opts.lostCb()
		e.mu.Unlock()
	}

	for {
		select {
		case <-ticker.C:
			e.mu.Lock()
			seen := e.last
			e.mu.Unlock()

			if time.Since(seen) > time.Duration(e.opts.missedHBThreshold)*e.opts.hbInterval {
				lost()
				return
			}

		case <-e.ctx.Done():
			lost()
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

func (e *election) campaign(wg *sync.WaitGroup) {
	try := func() error {
		e.mu.Lock()
		leader := e.isLeader
		sub := e.sub
		ctx := e.ctx
		e.mu.Unlock()

		var err error

		if leader {
			return nil
		}

		if sub == nil {
			sub, err = e.subscribe()
			if err != nil {
				return err
			}
		}

		_, err = e.opts.stream.NewConsumer(jsm.IdleHeartbeat(e.opts.hbInterval), jsm.DeliverySubject(sub.Subject))
		if err != nil {
			return err
		}

		// give a bit of grace to the past leader to realize is not leader anymore
		err = ctxSleep(ctx, time.Duration(e.opts.missedHBThreshold)*e.opts.hbInterval)
		if err != nil {
			return err
		}

		e.mu.Lock()
		e.last = time.Now()

		wg.Add(1)
		go e.manageLeadership(wg)

		e.isLeader = true
		e.opts.wonCb()
		e.mu.Unlock()

		return nil
	}

	// time.Sleep(e.opts.campaignInterval)
	// try()

	ticker := time.NewTicker(e.opts.campaignInterval)

	for {
		select {
		case <-ticker.C:
			try()

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
	}

	if e.opts.lostCb != nil {
		e.opts.lostCb()
	}
}
