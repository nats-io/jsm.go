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

// Package backendtest provides a reusable contract-conformance suite
// for natscontext.Backend implementations. Third-party backends should
// depend on this package in a _test.go file and invoke
// RunBackendContract against a factory that produces a fresh backend
// for each subtest.
package backendtest

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/nats-io/jsm.go/natscontext"
)

// NewBackendFunc returns a fresh Backend for a single subtest. The
// supplied *testing.T lets filesystem-backed implementations call
// t.TempDir so disk state is cleaned up automatically.
type NewBackendFunc func(t *testing.T) natscontext.Backend

// RunBackendContract executes the full contract suite against
// backends produced by newBackend. Each subtest receives a fresh
// backend so state does not leak between assertions.
func RunBackendContract(t *testing.T, newBackend NewBackendFunc) {
	t.Helper()
	ctx := context.Background()

	t.Run("LoadMissingReturnsNotFound", func(t *testing.T) {
		b := newBackend(t)
		_, err := b.Load(ctx, "missing")
		if !errors.Is(err, natscontext.ErrNotFound) {
			t.Fatalf("Load missing: expected ErrNotFound, got %v", err)
		}
	})

	t.Run("SaveLoadRoundTrip", func(t *testing.T) {
		b := newBackend(t)
		want := []byte(`{"url":"nats://example:4222"}`)
		err := b.Save(ctx, "alpha", want)
		if err != nil {
			t.Fatalf("save: %v", err)
		}
		got, err := b.Load(ctx, "alpha")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("round-trip mismatch: got %q want %q", got, want)
		}
	})

	t.Run("SaveOverwrites", func(t *testing.T) {
		b := newBackend(t)
		err := b.Save(ctx, "alpha", []byte("v1"))
		if err != nil {
			t.Fatalf("first save: %v", err)
		}
		err = b.Save(ctx, "alpha", []byte("v2"))
		if err != nil {
			t.Fatalf("second save: %v", err)
		}
		got, err := b.Load(ctx, "alpha")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if !bytes.Equal(got, []byte("v2")) {
			t.Fatalf("overwrite mismatch: got %q want v2", got)
		}
	})

	t.Run("DeleteMissingIsNoOp", func(t *testing.T) {
		b := newBackend(t)
		err := b.Delete(ctx, "ghost")
		if err != nil {
			t.Fatalf("delete missing: unexpected error %v", err)
		}
	})

	t.Run("DeleteThenLoadReturnsNotFound", func(t *testing.T) {
		b := newBackend(t)
		err := b.Save(ctx, "alpha", []byte("v1"))
		if err != nil {
			t.Fatalf("save: %v", err)
		}
		err = b.Delete(ctx, "alpha")
		if err != nil {
			t.Fatalf("delete: %v", err)
		}
		_, err = b.Load(ctx, "alpha")
		if !errors.Is(err, natscontext.ErrNotFound) {
			t.Fatalf("post-delete load: expected ErrNotFound, got %v", err)
		}
	})

	t.Run("ListReturnsExactlyWhatWasSaved", func(t *testing.T) {
		b := newBackend(t)
		names := []string{"alpha", "bravo", "charlie"}
		for _, n := range names {
			err := b.Save(ctx, n, []byte("x"))
			if err != nil {
				t.Fatalf("save %s: %v", n, err)
			}
		}

		got, err := b.List(ctx)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(got) != len(names) {
			t.Fatalf("list length: got %d want %d (got=%v)", len(got), len(names), got)
		}
		set := map[string]bool{}
		for _, n := range got {
			set[n] = true
		}
		for _, n := range names {
			if !set[n] {
				t.Fatalf("list missing %q, got %v", n, got)
			}
		}
	})

	t.Run("InvalidNameIsRejected", func(t *testing.T) {
		// "bad/name" exercises the core natscontext validator rule
		// (forward slash is always rejected, regardless of backend).
		// Whitespace and subject-wildcard names are deliberately
		// accepted at this layer for historical backward compat, so
		// they are not portable invalid-name probes.
		b := newBackend(t)
		err := b.Save(ctx, "bad/name", []byte("x"))
		if !errors.Is(err, natscontext.ErrInvalidName) {
			t.Fatalf("Save invalid: expected ErrInvalidName, got %v", err)
		}
		_, err = b.Load(ctx, "bad/name")
		if !errors.Is(err, natscontext.ErrInvalidName) {
			t.Fatalf("Load invalid: expected ErrInvalidName, got %v", err)
		}
		err = b.Delete(ctx, "bad/name")
		if !errors.Is(err, natscontext.ErrInvalidName) {
			t.Fatalf("Delete invalid: expected ErrInvalidName, got %v", err)
		}
	})

	probe := newBackend(t)
	_, implementsSelector := probe.(natscontext.Selector)
	if !implementsSelector {
		return
	}

	RunSelectorContract(t, func(t *testing.T) natscontext.Selector {
		return newBackend(t).(natscontext.Selector)
	})
}

// NewSelectorFunc returns a fresh Selector for a single subtest.
type NewSelectorFunc func(t *testing.T) natscontext.Selector

// RunSelectorContract executes the selector contract against selectors
// produced by newSelector. Suitable for Selector-only implementations
// (e.g. the standalone FileSelector) where there is no Backend to run
// the full backend contract against.
func RunSelectorContract(t *testing.T, newSelector NewSelectorFunc) {
	t.Helper()
	ctx := context.Background()

	t.Run("Selector", func(t *testing.T) {
		t.Run("FreshHasNoSelection", func(t *testing.T) {
			s := newSelector(t)
			_, err := s.Selected(ctx)
			if !errors.Is(err, natscontext.ErrNoneSelected) {
				t.Fatalf("fresh Selected: expected ErrNoneSelected, got %v", err)
			}
		})

		t.Run("SelectedAfterSetReturnsName", func(t *testing.T) {
			s := newSelector(t)
			_, err := s.SetSelected(ctx, "alpha")
			if err != nil {
				t.Fatalf("set: %v", err)
			}
			got, err := s.Selected(ctx)
			if err != nil {
				t.Fatalf("selected: %v", err)
			}
			if got != "alpha" {
				t.Fatalf("Selected = %q want alpha", got)
			}
		})

		t.Run("SetSelectedReturnsPrior", func(t *testing.T) {
			s := newSelector(t)
			prev, err := s.SetSelected(ctx, "alpha")
			if err != nil {
				t.Fatalf("set alpha: %v", err)
			}
			if prev != "" {
				t.Fatalf("expected empty prior on first set, got %q", prev)
			}

			prev, err = s.SetSelected(ctx, "bravo")
			if err != nil {
				t.Fatalf("set bravo: %v", err)
			}
			if prev != "alpha" {
				t.Fatalf("expected prior=alpha, got %q", prev)
			}
		})

		t.Run("EmptyNameClearsSelection", func(t *testing.T) {
			s := newSelector(t)
			_, err := s.SetSelected(ctx, "alpha")
			if err != nil {
				t.Fatalf("set alpha: %v", err)
			}
			prev, err := s.SetSelected(ctx, "")
			if err != nil {
				t.Fatalf("clear: %v", err)
			}
			if prev != "alpha" {
				t.Fatalf("clear prior: got %q want alpha", prev)
			}
			_, err = s.Selected(ctx)
			if !errors.Is(err, natscontext.ErrNoneSelected) {
				t.Fatalf("post-clear Selected: expected ErrNoneSelected, got %v", err)
			}
		})

		t.Run("ConcurrentSetSelectedConverges", func(t *testing.T) {
			s := newSelector(t)
			candidates := []string{"alpha", "bravo", "charlie", "delta"}

			var wg sync.WaitGroup
			for _, name := range candidates {
				wg.Add(1)
				go func(n string) {
					defer wg.Done()
					for i := 0; i < 30; i++ {
						_, err := s.SetSelected(ctx, n)
						if err != nil {
							t.Errorf("concurrent set %s: %v", n, err)
							return
						}
					}
				}(name)
			}
			wg.Wait()

			got, err := s.Selected(ctx)
			if err != nil {
				t.Fatalf("final Selected: %v", err)
			}
			found := false
			for _, c := range candidates {
				if c == got {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("final Selected = %q, not one of the candidates %v", got, candidates)
			}
		})
	})
}
