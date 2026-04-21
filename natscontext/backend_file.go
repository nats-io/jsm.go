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

package natscontext

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// FileBackend stores contexts as JSON files under <root>/nats/context/<name>.json
// and tracks the active and previously-selected context in
// <root>/nats/context.txt and <root>/nats/previous-context.txt.
//
// All disk mutations use write-temp-then-rename so concurrent readers never
// observe a partially-written file.
type FileBackend struct {
	root string
	mu   sync.Mutex
}

const (
	selectedCtxFile string = "context.txt"
	previousCtxFile string = "previous-context.txt"
)

// NewDefaultFileBackend returns a FileBackend rooted at $XDG_CONFIG_HOME,
// falling back to $HOME/.config, matching the historical natscontext
// behavior. The backend is returned even if the directory does not yet
// exist; it will be created on first write.
func NewDefaultFileBackend() Backend {
	root, err := defaultRoot()
	if err != nil {
		// Mirror the pre-refactor behavior: reads against a backend with
		// an unresolvable root silently return empty results; only writes
		// surface the error.
		return &FileBackend{root: ""}
	}
	return &FileBackend{root: root}
}

// NewFileBackendAt returns a FileBackend rooted at dir. The backend keeps
// its context files under dir/nats/context/. The directory is created on
// first write.
func NewFileBackendAt(dir string) Backend {
	return &FileBackend{root: dir}
}

func defaultRoot() (string, error) {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg != "" {
		return xdg, nil
	}
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	if u.HomeDir == "" {
		return "", fmt.Errorf("cannot determine home directory")
	}
	return filepath.Join(u.HomeDir, ".config"), nil
}

func (fb *FileBackend) contextDir() string {
	return filepath.Join(fb.root, "nats", "context")
}

func (fb *FileBackend) selectedPath() string {
	return filepath.Join(fb.root, "nats", selectedCtxFile)
}

func (fb *FileBackend) previousPath() string {
	return filepath.Join(fb.root, "nats", previousCtxFile)
}

// Path returns the on-disk path for a context with the given name.
// It does not validate the name or check existence.
func (fb *FileBackend) Path(name string) string {
	return filepath.Join(fb.contextDir(), name+".json")
}

func (fb *FileBackend) ensureTree() error {
	if fb.root == "" {
		return fmt.Errorf("file backend has no root directory")
	}
	return os.MkdirAll(fb.contextDir(), 0700)
}

// Load returns the raw JSON payload for name, or ErrNotFound if the
// context does not exist.
func (fb *FileBackend) Load(_ context.Context, name string) ([]byte, error) {
	err := ValidateName(name)
	if err != nil {
		return nil, err
	}
	if fb.root == "" {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}

	data, err := os.ReadFile(fb.Path(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Save writes data for name. Writes are atomic: concurrent readers see
// either the previous payload or the new one, never a partial write.
func (fb *FileBackend) Save(_ context.Context, name string, data []byte) error {
	err := ValidateName(name)
	if err != nil {
		return err
	}

	fb.mu.Lock()
	defer fb.mu.Unlock()

	err = fb.ensureTree()
	if err != nil {
		return err
	}
	return writeFileAtomic(fb.Path(name), data, 0600)
}

// Delete removes the named context. A missing name is a no-op.
func (fb *FileBackend) Delete(_ context.Context, name string) error {
	err := ValidateName(name)
	if err != nil {
		return err
	}
	if fb.root == "" {
		return nil
	}

	fb.mu.Lock()
	defer fb.mu.Unlock()

	err = os.Remove(fb.Path(name))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// List returns the names of all stored contexts. Directory entries that
// are empty, not suffixed with .json, or whose name fails ValidateName
// are skipped — they cannot be loaded or selected anyway, so surfacing
// them would mislead callers.
func (fb *FileBackend) List(_ context.Context) ([]string, error) {
	if fb.root == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(fb.contextDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Size() == 0 {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".json" {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ext)
		if ValidateName(name) != nil {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// Selected returns the currently selected context, or ErrNoneSelected
// when nothing is selected.
func (fb *FileBackend) Selected(_ context.Context) (string, error) {
	name := fb.readSelection(fb.selectedPath())
	if name == "" {
		return "", ErrNoneSelected
	}
	return name, nil
}

// Previous returns the previously-selected context, or an empty string
// if there is none. It is not part of the Selector interface; callers
// obtain it via the package-level PreviousContext helper.
func (fb *FileBackend) Previous() string {
	return fb.readSelection(fb.previousPath())
}

func (fb *FileBackend) readSelection(path string) string {
	if fb.root == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SetSelected sets name as the active context, returning whatever was
// selected before the swap. Passing an empty name clears the selection.
// Both the selected and previous files are updated via write-temp-then-
// rename so concurrent readers never see a torn value.
func (fb *FileBackend) SetSelected(_ context.Context, name string) (string, error) {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	err := fb.ensureTree()
	if err != nil {
		return "", err
	}

	previous := fb.readSelection(fb.selectedPath())

	if previous != "" {
		err = writeFileAtomic(fb.previousPath(), []byte(previous), 0600)
		if err != nil {
			return "", err
		}
	}

	if name == "" {
		err = os.Remove(fb.selectedPath())
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return previous, nil
	}

	err = writeFileAtomic(fb.selectedPath(), []byte(name), 0600)
	if err != nil {
		return "", err
	}
	return previous, nil
}

// SingleFileBackend stores exactly one context at a caller-supplied
// file path. It is the Backend that NewFromFile wraps so loading a
// context from an arbitrary path and saving it back go through the
// same code path as FileBackend. The backend's logical name is
// derived from the file's basename; Load/Save/Delete accept either
// that name or the empty string.
//
// SingleFileBackend deliberately does not implement Selector: there
// is only one slot, so selection tracking has no meaning.
type SingleFileBackend struct {
	path string
	name string
	mu   sync.Mutex
}

// NewSingleFileBackend returns a Backend rooted at a single file path.
// The context's name is derived from the basename (without the .json
// extension) so Registry.Load(name) and Registry.Save(c, name) both
// work when the caller passes the same name.
func NewSingleFileBackend(path string) Backend {
	return &SingleFileBackend{
		path: path,
		name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
	}
}

// Name returns the logical name this backend answers to. Useful for
// NewFromFile which needs to pass the derived name back to
// Registry.Load.
func (sb *SingleFileBackend) Name() string {
	return sb.name
}

// Path returns the on-disk path for name when name matches the
// backend's sole slot. Registry.Load uses this to populate
// Context.path after a successful load.
func (sb *SingleFileBackend) Path(name string) string {
	if name != "" && name != sb.name {
		return ""
	}
	return sb.path
}

// Load returns the bytes at the backend's path. An ErrNotFound is
// returned both for requests naming something other than this
// backend's slot and for a missing file on disk.
func (sb *SingleFileBackend) Load(_ context.Context, name string) ([]byte, error) {
	if name != "" && name != sb.name {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	data, err := os.ReadFile(sb.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, sb.name)
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Save atomically writes data to the backend's path. The name
// argument must be empty or match the backend's logical name;
// renames go through a different Backend.
func (sb *SingleFileBackend) Save(_ context.Context, name string, data []byte) error {
	if name != "" && name != sb.name {
		return fmt.Errorf("%w: single-file backend only accepts name %q, got %q", ErrInvalidName, sb.name, name)
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	return writeFileAtomic(sb.path, data, 0600)
}

// Delete removes the file at the backend's path. A missing file is a
// no-op. Requests naming something other than this backend's slot
// are also no-ops.
func (sb *SingleFileBackend) Delete(_ context.Context, name string) error {
	if name != "" && name != sb.name {
		return nil
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	err := os.Remove(sb.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// List returns the backend's single name when the file exists,
// otherwise an empty slice.
func (sb *SingleFileBackend) List(_ context.Context) ([]string, error) {
	_, err := os.Stat(sb.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []string{sb.name}, nil
}

// writeFileAtomic writes data to path by creating a temp file in the same
// directory, syncing it, renaming it into place, and then fsyncing the
// parent directory for durability. A crash at any point leaves the
// destination either unmodified or containing the new content, never a
// partial write.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = os.Remove(tmpPath)
	}

	err = tmp.Chmod(mode)
	if err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}

	_, err = tmp.Write(data)
	if err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}

	err = tmp.Sync()
	if err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}

	err = tmp.Close()
	if err != nil {
		cleanup()
		return err
	}

	err = os.Rename(tmpPath, path)
	if err != nil {
		cleanup()
		return err
	}

	// Best-effort fsync of the parent directory so the rename is durable.
	// Errors here don't invalidate the write that already landed.
	d, err := os.Open(dir)
	if err == nil {
		_ = d.Sync()
		_ = d.Close()
	}

	return nil
}
