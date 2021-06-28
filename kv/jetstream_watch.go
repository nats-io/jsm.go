// Copyright 2021 The NATS Authors
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

package kv

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
)

type jsWatch struct {
	streamName string
	bucket     string
	subj       string
	mgr        *jsm.Manager
	cons       *jsm.Consumer
	sub        *nats.Subscription
	nc         *nats.Conn
	lastSeen   time.Time
	outCh      chan Result
	ctx        context.Context
	dec        func(string) string
	cancel     func()
	log        Logger
	running    bool
	latestOnly bool
	mu         sync.Mutex
}

func newJSWatch(ctx context.Context, stramName string, bucket string, subj string, dec func(string) string, nc *nats.Conn, mgr *jsm.Manager, log Logger) (*jsWatch, error) {
	w := &jsWatch{
		streamName: stramName,
		bucket:     bucket,
		subj:       subj,
		nc:         nc,
		mgr:        mgr,
		log:        log,
		dec:        dec,
		latestOnly: !(strings.HasSuffix(subj, "*") || strings.HasSuffix(subj, ">")),
		outCh:      make(chan Result, 1000),
	}

	w.ctx, w.cancel = context.WithCancel(ctx)

	err := w.start()
	if err != nil {
		w.cancel()
		return nil, err
	}

	return w, nil
}

func (w *jsWatch) Channel() chan Result {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.outCh
}

func (w *jsWatch) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.ctx == nil {
		return nil
	}

	w.cancel()

	return nil
}

func (w *jsWatch) decode(v string) string {
	if w.dec == nil {
		return v
	}

	return w.dec(v)
}

func (w *jsWatch) handler(m *nats.Msg) {
	w.mu.Lock()
	w.lastSeen = time.Now()
	w.mu.Unlock()

	parts := strings.Split(m.Subject, ".")
	if m.Header.Get("Status") == "100" && m.Reply != "" {
		m.Respond(nil)
		return
	}

	if len(parts) == 0 {
		return
	}

	key := parts[len(parts)-1]

	res, err := jsResultFromMessage(w.bucket, key, m, w.decode)
	if err != nil {
		return
	}

	// only latest values
	if w.latestOnly && res.Delta() != 0 {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}

	select {
	case w.outCh <- res:
		m.Ack()
	default:
	}
}

func (w *jsWatch) consumerHealthManager() {
	// todo: use a backoff
	ticker := time.NewTicker(500 * time.Millisecond)

	// give the first check some time
	w.mu.Lock()
	w.lastSeen = time.Now()
	w.mu.Unlock()

	recreate := func() error {
		w.mu.Lock()
		w.sub.Unsubscribe()
		w.sub = nil
		w.cons = nil
		w.mu.Unlock()

		return w.createConsumer()
	}

	for {
		select {
		case <-ticker.C:
			w.mu.Lock()
			seen := w.lastSeen
			w.mu.Unlock()

			if time.Since(seen) > 3*time.Second {
				w.log.ErrorF("consumer failed, no heartbeats since %s", seen)

				err := recreate()
				if err != nil {
					w.log.ErrorF("recreating failed consumer failed: %s", err)
				}
			}

		case <-w.ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func (w *jsWatch) createConsumer() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var err error

	if w.sub != nil || w.cons != nil {
		return fmt.Errorf("already running")
	}

	w.sub, err = w.nc.Subscribe(nats.NewInbox(), w.handler)
	if err != nil {
		return err
	}

	opts := []jsm.ConsumerOption{
		jsm.FilterStreamBySubject(w.subj),
		jsm.DeliverySubject(w.sub.Subject),
		jsm.IdleHeartbeat(2 * time.Second),
		jsm.AcknowledgeExplicit(),
	}

	if w.latestOnly {
		opts = append(opts, jsm.StartWithLastReceived())
	}

	w.cons, err = w.mgr.NewConsumer(w.streamName, opts...)
	if err != nil {
		return err
	}

	state, err := w.cons.LatestState()
	if err != nil {
		return err
	}

	// if no messages are pending or delivered we should tell the watcher
	// we have nothing and it should assume its up to date
	if !w.latestOnly && state.NumPending+state.Delivered.Consumer == 0 {
		w.outCh <- nil
	}

	return nil
}

func (w *jsWatch) start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("already running")
	}
	w.running = true
	w.mu.Unlock()

	go func() {
		<-w.ctx.Done()

		w.mu.Lock()
		defer w.mu.Unlock()

		w.running = false
		if w.outCh != nil {
			close(w.outCh)
		}

		if w.sub != nil {
			w.sub.Unsubscribe()
		}
	}()

	err := w.createConsumer()
	if err != nil {
		return err
	}

	go w.consumerHealthManager()

	return nil
}
