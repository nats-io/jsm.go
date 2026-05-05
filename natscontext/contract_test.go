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

package natscontext_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/nats-io/jsm.go/natscontext"
	"github.com/nats-io/jsm.go/natscontext/backendtest"
)

// TestFileBackendContract runs the shared Backend contract against a
// fresh FileBackend rooted at each subtest's t.TempDir.
func TestFileBackendContract(t *testing.T) {
	backendtest.RunBackendContract(t, func(t *testing.T) natscontext.Backend {
		return natscontext.NewFileBackendAt(t.TempDir())
	})
}

// TestMemoryBackendContract runs the shared Backend contract against
// a fresh MemoryBackend per subtest.
func TestMemoryBackendContract(t *testing.T) {
	backendtest.RunBackendContract(t, func(t *testing.T) natscontext.Backend {
		return natscontext.NewMemoryBackend()
	})
}

// TestSingleFileBackend covers the contract specific to a single-slot
// backend: only the derived logical name is accepted, Save overwrites
// in place, Load on a missing file reports ErrNotFound, and
// ErrInvalidName surfaces when a non-matching name is used. The
// shared RunBackendContract suite assumes arbitrary names work, so
// SingleFileBackend is tested directly rather than through it.
func TestSingleFileBackend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "only.json")
	b := natscontext.NewSingleFileBackend(path)
	ctx := context.Background()

	t.Run("LoadBeforeSaveIsNotFound", func(t *testing.T) {
		_, err := b.Load(ctx, "only")
		if !errors.Is(err, natscontext.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("SaveAndLoadRoundTrip", func(t *testing.T) {
		want := []byte(`{"url":"nats://a:4222"}`)
		err := b.Save(ctx, "only", want)
		if err != nil {
			t.Fatalf("save: %v", err)
		}
		got, err := b.Load(ctx, "")
		if err != nil {
			t.Fatalf("load (empty name): %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("load mismatch: got %q want %q", got, want)
		}
	})

	t.Run("SaveOverwrites", func(t *testing.T) {
		err := b.Save(ctx, "only", []byte("v2"))
		if err != nil {
			t.Fatalf("save: %v", err)
		}
		got, err := b.Load(ctx, "only")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if !bytes.Equal(got, []byte("v2")) {
			t.Fatalf("overwrite mismatch: got %q want v2", got)
		}
	})

	t.Run("ListReportsSingleSlot", func(t *testing.T) {
		names, err := b.List(ctx)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(names) != 1 || names[0] != "only" {
			t.Fatalf("List = %v, want [only]", names)
		}
	})

	t.Run("MismatchedNameLoadIsNotFound", func(t *testing.T) {
		_, err := b.Load(ctx, "other")
		if !errors.Is(err, natscontext.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("MismatchedNameSaveIsInvalidName", func(t *testing.T) {
		err := b.Save(ctx, "other", []byte("v3"))
		if !errors.Is(err, natscontext.ErrInvalidName) {
			t.Fatalf("expected ErrInvalidName, got %v", err)
		}
	})

	t.Run("DeleteRemovesFile", func(t *testing.T) {
		err := b.Delete(ctx, "only")
		if err != nil {
			t.Fatalf("delete: %v", err)
		}
		_, err = b.Load(ctx, "")
		if !errors.Is(err, natscontext.ErrNotFound) {
			t.Fatalf("load after delete: expected ErrNotFound, got %v", err)
		}
	})

	t.Run("DeleteMismatchedNameIsNoOp", func(t *testing.T) {
		err := b.Save(ctx, "only", []byte("present"))
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		err = b.Delete(ctx, "wrong")
		if err != nil {
			t.Fatalf("delete mismatched: %v", err)
		}
		got, err := b.Load(ctx, "")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if !bytes.Equal(got, []byte("present")) {
			t.Fatalf("file content disturbed by mismatched delete: %q", got)
		}
	})
}
