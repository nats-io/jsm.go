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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nats-io/jsm.go/natscontext"
	"github.com/nats-io/jsm.go/natscontext/backendtest"
)

// countingBackend wraps a Backend and counts Load calls. Used to
// prove Registry operations that should short-circuit really do.
type countingBackend struct {
	inner natscontext.Backend
	loads atomic.Int64
}

func (c *countingBackend) Load(ctx context.Context, name string) ([]byte, error) {
	c.loads.Add(1)
	return c.inner.Load(ctx, name)
}
func (c *countingBackend) Save(ctx context.Context, name string, data []byte) error {
	return c.inner.Save(ctx, name, data)
}
func (c *countingBackend) Delete(ctx context.Context, name string) error {
	return c.inner.Delete(ctx, name)
}
func (c *countingBackend) List(ctx context.Context) ([]string, error) {
	return c.inner.List(ctx)
}

func TestFileSelector_Contract(t *testing.T) {
	backendtest.RunSelectorContract(t, func(t *testing.T) natscontext.Selector {
		return natscontext.NewFileSelectorAt(t.TempDir())
	})
}

// TestFileSelector_ReusesSameFilesAsFileBackend asserts that a
// standalone FileSelector and a FileBackend rooted at the same
// directory read and write the same on-disk selection files. This is
// what lets the shared-backend + local-selector composition degrade
// cleanly back to a pure-FileBackend deployment.
func TestFileSelector_ReusesSameFilesAsFileBackend(t *testing.T) {
	dir := t.TempDir()
	bg := context.Background()

	sel := natscontext.NewFileSelectorAt(dir)
	_, err := sel.SetSelected(bg, "alpha")
	if err != nil {
		t.Fatalf("selector set: %v", err)
	}

	onDisk, err := os.ReadFile(filepath.Join(dir, "nats", "context.txt"))
	if err != nil {
		t.Fatalf("read context.txt: %v", err)
	}
	if strings.TrimSpace(string(onDisk)) != "alpha" {
		t.Fatalf("context.txt = %q want alpha", onDisk)
	}

	reg := natscontext.NewRegistry(natscontext.NewFileBackendAt(dir))
	got, err := reg.Selected(bg)
	if err != nil {
		t.Fatalf("backend selected: %v", err)
	}
	if got != "alpha" {
		t.Fatalf("backend sees %q want alpha", got)
	}
}

// TestFileSelector_PreviousTracking verifies that Previous() returns
// the prior selection as it does on FileBackend.
func TestFileSelector_PreviousTracking(t *testing.T) {
	sel := natscontext.NewFileSelectorAt(t.TempDir())
	bg := context.Background()

	_, err := sel.SetSelected(bg, "alpha")
	if err != nil {
		t.Fatalf("set alpha: %v", err)
	}
	_, err = sel.SetSelected(bg, "bravo")
	if err != nil {
		t.Fatalf("set bravo: %v", err)
	}

	if got := sel.Previous(); got != "alpha" {
		t.Fatalf("Previous = %q want alpha", got)
	}
}

// TestNewDefaultFileSelector_MissingEnv asserts the constructor errors
// when neither XDG_CONFIG_HOME nor a resolvable HOME is available. A
// silent zero-root fallback paired with a working remote Backend would
// look like it works until the next process restart, which is strictly
// worse than an up-front error.
func TestNewDefaultFileSelector_MissingEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	// macOS's user.Current reads /etc/passwd first, so this test is only
	// meaningful on systems where clearing HOME is enough. Skip when
	// the constructor still resolves a home directory.
	_, err := natscontext.NewDefaultFileSelector()
	if err == nil {
		t.Skip("system resolves a HOME even with HOME cleared; cannot exercise error path here")
	}
	if !strings.Contains(err.Error(), "cannot determine default config directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRegistry_WithSelectorOverride confirms that WithSelector
// overrides whatever Selector the Backend implements so selection is
// tracked in the caller-supplied selector.
func TestRegistry_WithSelectorOverride(t *testing.T) {
	backendDir := t.TempDir()
	selectorDir := t.TempDir()
	bg := context.Background()

	sel := natscontext.NewFileSelectorAt(selectorDir)

	reg := natscontext.NewRegistry(
		natscontext.NewFileBackendAt(backendDir),
		natscontext.WithSelector(sel),
	)

	nctx, err := natscontext.New("alpha", false, natscontext.WithServerURL("nats://a:4222"))
	if err != nil {
		t.Fatalf("new ctx: %v", err)
	}
	err = reg.Save(bg, nctx, "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	_, err = reg.Select(bg, "alpha")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	backendSelectedFile := filepath.Join(backendDir, "nats", "context.txt")
	_, err = os.Stat(backendSelectedFile)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("backend context.txt should not exist when WithSelector overrides, got stat err=%v", err)
	}

	selectorSelectedFile := filepath.Join(selectorDir, "nats", "context.txt")
	data, err := os.ReadFile(selectorSelectedFile)
	if err != nil {
		t.Fatalf("read selector context.txt: %v", err)
	}
	if strings.TrimSpace(string(data)) != "alpha" {
		t.Fatalf("selector context.txt = %q want alpha", data)
	}
}

// TestRegistry_WithoutSelection asserts that selection tracking is
// disabled even when the Backend implements Selector.
func TestRegistry_WithoutSelection(t *testing.T) {
	reg := natscontext.NewRegistry(
		natscontext.NewFileBackendAt(t.TempDir()),
		natscontext.WithoutSelection(),
	)
	bg := context.Background()

	nctx, err := natscontext.New("alpha", false, natscontext.WithServerURL("nats://a:4222"))
	if err != nil {
		t.Fatalf("new ctx: %v", err)
	}
	err = reg.Save(bg, nctx, "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	_, err = reg.Selected(bg)
	if !errors.Is(err, natscontext.ErrNoneSelected) {
		t.Fatalf("Selected: expected ErrNoneSelected, got %v", err)
	}

	_, err = reg.Select(bg, "alpha")
	if !errors.Is(err, natscontext.ErrReadOnly) {
		t.Fatalf("Select: expected ErrReadOnly, got %v", err)
	}
}

// TestRegistry_WithoutSelection_ShortCircuitsSelect verifies that
// Select against a WithoutSelection registry returns ErrReadOnly
// without probing the backend for the name's existence. A backend
// that counts Load calls proves no lookup happened, and the error
// is ErrReadOnly even for a name that does not exist — under the
// pre-fix code path it would have surfaced ErrNotFound first.
func TestRegistry_WithoutSelection_ShortCircuitsSelect(t *testing.T) {
	backend := &countingBackend{inner: natscontext.NewMemoryBackend()}
	reg := natscontext.NewRegistry(backend, natscontext.WithoutSelection())

	_, err := reg.Select(context.Background(), "never-created")
	if !errors.Is(err, natscontext.ErrReadOnly) {
		t.Fatalf("Select of unknown name: expected ErrReadOnly, got %v", err)
	}
	if got := backend.loads.Load(); got != 0 {
		t.Fatalf("Select must not probe the backend when selection is disabled; Load was called %d times", got)
	}
}

// TestWithSelector_NilPanics confirms the nil-Selector guard.
func TestWithSelector_NilPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic on WithSelector(nil), got none")
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, "nil Selector") {
			t.Fatalf("panic message = %q, want mention of nil Selector", msg)
		}
	}()
	_ = natscontext.WithSelector(nil)
}

// TestRegistry_LoadClearsStaleSelection confirms that when the
// Backend returns ErrNotFound for a name produced by the Selector,
// the Registry clears the selector and falls through to the
// "no selection" path rather than failing. This is the guard that
// keeps the shared-backend + local-selector composition workable when
// another operator deletes a context the local pointer still names.
func TestRegistry_LoadClearsStaleSelection(t *testing.T) {
	selectorDir := t.TempDir()
	sel := natscontext.NewFileSelectorAt(selectorDir)
	_, err := sel.SetSelected(context.Background(), "ghost")
	if err != nil {
		t.Fatalf("set selector: %v", err)
	}

	reg := natscontext.NewRegistry(
		natscontext.NewFileBackendAt(t.TempDir()),
		natscontext.WithSelector(sel),
	)

	nctx, err := reg.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("Load with stale selection: %v", err)
	}
	if nctx.Name != "" {
		t.Fatalf("Load should have fallen through to empty context, got Name=%q", nctx.Name)
	}

	_, err = sel.Selected(context.Background())
	if !errors.Is(err, natscontext.ErrNoneSelected) {
		t.Fatalf("selector should be cleared after stale load, got %v", err)
	}
}

// TestRegistry_LoadExplicitNameSurfacesErrNotFound confirms the
// stale-selection guard ONLY kicks in for names that came from the
// selector — an explicit name that does not exist still returns
// ErrNotFound.
func TestRegistry_LoadExplicitNameSurfacesErrNotFound(t *testing.T) {
	reg := natscontext.NewRegistry(natscontext.NewFileBackendAt(t.TempDir()))
	_, err := reg.Load(context.Background(), "missing")
	if !errors.Is(err, natscontext.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// failingBackend returns a fixed error from Load so tests can assert
// Registry operations propagate non-NotFound backend errors verbatim
// rather than collapsing them into ErrNotFound.
type failingBackend struct {
	natscontext.Backend
	loadErr error
}

func (f *failingBackend) Load(_ context.Context, _ string) ([]byte, error) {
	return nil, f.loadErr
}

// TestRegistry_SelectPropagatesBackendErrors pins the contract that
// Select surfaces the backend's real error instead of masking every
// non-nil Load error as ErrNotFound. Transport failures, permission
// issues, and backend-specific ErrInvalidName rejections must reach
// the caller with their sentinels intact.
func TestRegistry_SelectPropagatesBackendErrors(t *testing.T) {
	cases := []struct {
		name    string
		loadErr error
		is      error
	}{
		{"invalid name from backend", fmt.Errorf("%w: rejected by backend", natscontext.ErrInvalidName), natscontext.ErrInvalidName},
		{"transport-style failure", errors.New("nats: connection closed"), nil},
		{"not found still wraps", fmt.Errorf("%w: %q", natscontext.ErrNotFound, "missing"), natscontext.ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inner := natscontext.NewMemoryBackend()
			reg := natscontext.NewRegistry(&failingBackend{Backend: inner, loadErr: tc.loadErr},
				natscontext.WithSelector(natscontext.NewFileSelectorAt(t.TempDir())))

			_, err := reg.Select(context.Background(), "anything")
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if tc.is != nil && !errors.Is(err, tc.is) {
				t.Fatalf("expected error to wrap %v, got %v", tc.is, err)
			}
			if tc.is == nil && errors.Is(err, natscontext.ErrNotFound) {
				t.Fatalf("transport error should not be masked as ErrNotFound, got %v", err)
			}
		})
	}
}
