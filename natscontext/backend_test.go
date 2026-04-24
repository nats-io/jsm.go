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
	"sync"
	"testing"

	"github.com/nats-io/jsm.go/natscontext"
)

// TestFileBackendAtomicSave asserts that Save never leaves a partial or
// zero-byte file behind and cleans up its temporary files. Success is
// observed by writing many payloads in quick succession and inspecting
// the resulting directory: every surviving file is one of the payloads
// we wrote, and no orphan .tmp files remain.
func TestFileBackendAtomicSave(t *testing.T) {
	dir := t.TempDir()
	reg := natscontext.NewRegistry(natscontext.NewFileBackendAt(dir))
	bg := context.Background()

	for i := 0; i < 50; i++ {
		nctx, err := natscontext.New(fmt.Sprintf("atomic%d", i), false,
			natscontext.WithServerURL(fmt.Sprintf("nats://host-%d:4222", i)))
		if err != nil {
			t.Fatalf("new ctx: %v", err)
		}
		err = reg.Save(bg, nctx, "")
		if err != nil {
			t.Fatalf("save %d: %v", i, err)
		}

		path := filepath.Join(dir, "nats", "context", fmt.Sprintf("atomic%d.json", i))
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if info.Size() == 0 {
			t.Fatalf("atomic%d: zero-byte file after save", i)
		}
	}

	entries, err := os.ReadDir(filepath.Join(dir, "nats", "context"))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("orphan temp file left behind: %s", e.Name())
		}
	}
}

// TestFileBackendAtomicSelect fires concurrent Select calls against a
// shared FileBackend and verifies the selected/previous files always
// contain whole names (not torn values or empty trailing data) after
// all goroutines complete.
func TestFileBackendAtomicSelect(t *testing.T) {
	dir := t.TempDir()
	reg := natscontext.NewRegistry(natscontext.NewFileBackendAt(dir))
	bg := context.Background()

	const n = 16
	names := make([]string, n)
	for i := 0; i < n; i++ {
		names[i] = fmt.Sprintf("ctx%02d", i)
		nctx, err := natscontext.New(names[i], false,
			natscontext.WithServerURL("nats://localhost:4222"))
		if err != nil {
			t.Fatalf("new %s: %v", names[i], err)
		}
		err = reg.Save(bg, nctx, "")
		if err != nil {
			t.Fatalf("save %s: %v", names[i], err)
		}
	}

	_, err := reg.Select(bg, names[0])
	if err != nil {
		t.Fatalf("initial select: %v", err)
	}

	var wg sync.WaitGroup
	for _, name := range names {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_, err := reg.Select(bg, n)
				if err != nil {
					t.Errorf("select %s: %v", n, err)
					return
				}
			}
		}(name)
	}
	wg.Wait()

	selected, err := reg.Selected(bg)
	if err != nil {
		t.Fatalf("final selected: %v", err)
	}
	if !contains(names, selected) {
		t.Fatalf("selected %q is not one of the candidates %v", selected, names)
	}

	selectedFile := filepath.Join(dir, "nats", "context.txt")
	previousFile := filepath.Join(dir, "nats", "previous-context.txt")

	for _, path := range []string{selectedFile, previousFile} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			t.Errorf("%s is empty after concurrent selects", path)
			continue
		}
		if !contains(names, trimmed) {
			t.Errorf("%s content %q is not a full name (torn write?)", path, trimmed)
		}
	}

	entries, err := os.ReadDir(filepath.Join(dir, "nats"))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("orphan temp file: %s", e.Name())
		}
	}
}

// TestRegistryDelegation asserts that constructing a Registry directly
// against a directory produces the same observable state as routing
// through the package-level functions pointed at the same directory.
func TestRegistryDelegation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	reg := natscontext.NewRegistry(natscontext.NewFileBackendAt(dir))
	bg := context.Background()

	nctx, err := natscontext.New("alpha", false, natscontext.WithServerURL("nats://a:4222"))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	err = reg.Save(bg, nctx, "")
	if err != nil {
		t.Fatalf("registry save: %v", err)
	}

	if !natscontext.IsKnown("alpha") {
		t.Fatalf("IsKnown(alpha) should be true after registry save")
	}
	known := natscontext.KnownContexts()
	if !contains(known, "alpha") {
		t.Fatalf("KnownContexts should include %q, got %v", "alpha", known)
	}

	_, err = reg.Select(bg, "alpha")
	if err != nil {
		t.Fatalf("registry select: %v", err)
	}
	got := natscontext.SelectedContext()
	if got != "alpha" {
		t.Fatalf("SelectedContext = %q, want alpha", got)
	}

	ctxPkg, err := natscontext.New("alpha", true)
	if err != nil {
		t.Fatalf("package load: %v", err)
	}
	if ctxPkg.ServerURL() != "nats://a:4222" {
		t.Fatalf("package load URL = %q, want nats://a:4222", ctxPkg.ServerURL())
	}

	ctxReg, err := reg.Load(bg, "alpha")
	if err != nil {
		t.Fatalf("registry load: %v", err)
	}
	if ctxReg.ServerURL() != ctxPkg.ServerURL() {
		t.Fatalf("registry ServerURL=%q, package ServerURL=%q", ctxReg.ServerURL(), ctxPkg.ServerURL())
	}

	err = natscontext.DeleteContext("alpha")
	if err != nil {
		t.Fatalf("package delete: %v", err)
	}
	list, err := reg.List(bg)
	if err != nil {
		t.Fatalf("registry list: %v", err)
	}
	if contains(list, "alpha") {
		t.Fatalf("alpha should be gone after package-level delete, got %v", list)
	}
}

// TestRegistryNotFound asserts Registry.Load on a missing name surfaces
// ErrNotFound.
func TestRegistryNotFound(t *testing.T) {
	reg := natscontext.NewRegistry(natscontext.NewFileBackendAt(t.TempDir()))
	_, err := reg.Load(context.Background(), "missing")
	if !errors.Is(err, natscontext.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestRegistryInvalidName asserts validation errors surface ErrInvalidName.
func TestRegistryInvalidName(t *testing.T) {
	reg := natscontext.NewRegistry(natscontext.NewFileBackendAt(t.TempDir()))
	_, err := reg.Load(context.Background(), "bad/name")
	if !errors.Is(err, natscontext.ErrInvalidName) {
		t.Fatalf("expected ErrInvalidName, got %v", err)
	}
}

// TestFileBackendDottedName round-trips a name containing a dot
// through FileBackend + FileSelector. Historical contexts such as
// "ngs.js" were persisted before the Backend refactor introduced
// stricter validation; an upgrade that rejected them would strand
// user data, so dots must keep working.
func TestFileBackendDottedName(t *testing.T) {
	dir := t.TempDir()
	reg := natscontext.NewRegistry(natscontext.NewFileBackendAt(dir))
	bg := context.Background()

	nctx, err := natscontext.New("ngs.js", false, natscontext.WithServerURL("nats://ngs:4222"))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	err = reg.Save(bg, nctx, "")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	path := filepath.Join(dir, "nats", "context", "ngs.js.json")
	_, err = os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}

	loaded, err := reg.Load(bg, "ngs.js")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ServerURL() != "nats://ngs:4222" {
		t.Fatalf("ServerURL = %q want nats://ngs:4222", loaded.ServerURL())
	}

	_, err = reg.Select(bg, "ngs.js")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	list, err := reg.List(bg)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !contains(list, "ngs.js") {
		t.Fatalf("list missing ngs.js: %v", list)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
