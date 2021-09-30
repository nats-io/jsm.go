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
	"sync"
)

type readCache struct {
	backend Storage
	cache   map[string]Entry
	watch   Watch
	ctx     context.Context
	cancel  func()
	ready   bool
	log     Logger
	mu      sync.Mutex
}

func newReadCache(b Storage, log Logger) (*readCache, error) {
	if b == nil {
		return nil, fmt.Errorf("storage is required")
	}

	cache := &readCache{
		backend: b,
		cache:   map[string]Entry{},
		log:     log,
	}

	var err error
	cache.ctx, cache.cancel = context.WithCancel(context.Background())

	cache.watch, err = b.Watch(cache.ctx, ">")
	if err != nil {
		cache.Close()
		return nil, err
	}

	go cache.watcher()

	return cache, nil
}

func (c *readCache) Bucket() string          { return c.backend.Bucket() }
func (c *readCache) BucketSubject() string   { return c.backend.BucketSubject() }
func (c *readCache) CreateBucket() error     { return c.backend.CreateBucket() }
func (c *readCache) Status() (Status, error) { return c.backend.Status() }

func (c *readCache) Watch(ctx context.Context, key string) (Watch, error) {
	return c.backend.Watch(ctx, key)
}

func (c *readCache) Put(key string, val []byte, opts ...PutOption) (uint64, error) {
	// put on the backend can fail making this invalidate not needed but
	// put can also succeed and then error due to network timeouts etc, so
	// safest possible thing is to always invalidate on put regardless of outcome
	c.mu.Lock()
	delete(c.cache, key)
	c.mu.Unlock()

	return c.backend.Put(key, val, opts...)
}

func (c *readCache) Destroy() error {
	c.mu.Lock()
	c.cache = map[string]Entry{}
	c.mu.Unlock()

	return c.backend.Destroy()
}

func (c *readCache) Purge(key string) error {
	c.mu.Lock()
	delete(c.cache, key) // possible race here if a update is enroute from the watched bucket that, probably ok
	c.mu.Unlock()

	return c.backend.Purge(key)
}

func (c *readCache) Delete(key string) error {
	c.mu.Lock()
	delete(c.cache, key) // possible race here if a update is enroute from the watched bucket that, probably ok
	c.mu.Unlock()

	return c.backend.Delete(key)
}

func (c *readCache) History(ctx context.Context, key string) ([]Entry, error) {
	return c.backend.History(ctx, key)
}

func (c *readCache) Get(key string) (Entry, error) {
	c.mu.Lock()
	entry, ok := c.cache[key]
	ready := c.ready
	c.mu.Unlock()

	if ready && ok {
		return entry, nil
	}

	res, err := c.backend.Get(key)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cache[key] = res
	c.mu.Unlock()

	return res, nil
}

func (c *readCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = nil

	// stop the watch
	if c.cancel != nil {
		c.cancel()
	}

	if c.watch != nil {
		c.watch.Close()
	}

	return c.backend.Close()
}

func (c *readCache) watcher() {
	for {
		select {
		case result, ok := <-c.watch.Channel():
			if !ok { // channel is closed we should shut down
				c.mu.Lock()
				c.ready = false
				c.mu.Unlock()

				return
			}

			// we got a message so channel isn't closed but it's a nil,
			// that's signaling that the stream is empty so we should go ready
			if result == nil {
				c.mu.Lock()
				c.ready = true
				c.mu.Unlock()

			} else {
				c.mu.Lock()

				switch result.Operation() {
				case DeleteOperation:
					delete(c.cache, result.Key())
				case PutOperation:
					c.cache[result.Key()] = result
				}

				ready := c.ready
				c.mu.Unlock()

				// once we have reached the end of the data stream we have all latest values
				// signal ready ensuring cache reads only get latest data
				if !ready && result.Delta() == 0 {
					c.mu.Lock()
					c.ready = true
					c.mu.Unlock()
				}
			}

		case <-c.ctx.Done():
			return
		}
	}
}
