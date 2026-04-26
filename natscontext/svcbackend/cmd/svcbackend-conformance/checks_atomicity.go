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
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// atomicityChecks covers the two "MUST be atomic" requirements in the
// spec. They are inherently probabilistic: the best we can do is race
// many workers against one writer and assert no torn read was observed.
// A clean run is reported as WARN (no violation seen but not proven)
// and a torn read is reported as FAIL.
func atomicityChecks() []Check {
	return []Check{
		{
			ID: "behavior.save_load_atomic", Section: "Behavior",
			Title: "ctx.save is atomic vs concurrent ctx.load",
			Modes: []string{"rw"},
			Run: func(ctx context.Context, h *Harness) (Status, string, error) {
				name := h.MintName("atomic_rw")
				v1 := bytes.Repeat([]byte{'A'}, 4096)
				v2 := bytes.Repeat([]byte{'B'}, 4096)

				err := h.Client.Save(ctx, name, v1)
				if err != nil {
					return StatusFail, "seed: " + err.Error(), nil
				}

				var torn atomic.Int64
				var reads atomic.Int64
				var readErr atomic.Value
				var writeErr atomic.Value

				done := make(chan struct{})
				var wg sync.WaitGroup

				// Readers assert that every observed payload is either
				// v1 or v2 in its entirety.
				for i := 0; i < h.Concurrency; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for {
							select {
							case <-done:
								return
							default:
							}
							got, loadErr := h.Client.Load(ctx, name)
							if loadErr != nil {
								readErr.Store(loadErr)
								return
							}
							reads.Add(1)
							if !bytes.Equal(got, v1) && !bytes.Equal(got, v2) {
								torn.Add(1)
							}
						}
					}()
				}

				// Writer flips v1/v2 for the configured iteration count.
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer close(done)
					for i := 0; i < h.Iterations; i++ {
						var target []byte
						if i%2 == 0 {
							target = v2
						} else {
							target = v1
						}
						saveErr := h.Client.Save(ctx, name, target)
						if saveErr != nil {
							writeErr.Store(saveErr)
							return
						}
					}
				}()

				wg.Wait()

				if wErr, ok := writeErr.Load().(error); ok {
					return StatusFail, "writer: " + wErr.Error(), nil
				}
				if rErr, ok := readErr.Load().(error); ok {
					return StatusFail, "reader: " + rErr.Error(), nil
				}

				if torn.Load() > 0 {
					return StatusFail, fmt.Sprintf("observed %d torn reads in %d observations", torn.Load(), reads.Load()), nil
				}
				return StatusWarn, fmt.Sprintf("no torn reads observed in %d observations over %d writes (probabilistic)", reads.Load(), h.Iterations), nil
			},
		},

	}
}
