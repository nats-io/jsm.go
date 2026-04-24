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
	"path/filepath"
	"strings"
	"sync"
)

// FileSelector tracks the active context in two files on disk:
//
//	<root>/nats/context.txt           // currently selected context
//	<root>/nats/previous-context.txt   // previously selected context
//
// It implements the Selector interface and is the piece FileBackend
// uses to persist selection. It is also exported so it can be composed
// into a Registry that uses a different Backend — for example, the
// svcbackend Client — when selection is a personal, per-machine choice
// that should not be stored alongside the shared contexts. See the
// "Separating selection from storage" section in BACKENDS.md.
//
// All disk mutations go through write-temp-then-rename with a parent
// directory fsync so a concurrent reader sees either the previous
// value or the new one, never a torn write.
type FileSelector struct {
	root string
	mu   sync.Mutex
}

// NewDefaultFileSelector returns a FileSelector rooted at
// $XDG_CONFIG_HOME, falling back to $HOME/.config. Unlike
// NewDefaultFileBackend it returns an error when neither is resolvable:
// a silent zero-root selector composed with a working remote Backend
// looks like it works until the next process restart, which is a far
// worse failure mode than an up-front error.
func NewDefaultFileSelector() (*FileSelector, error) {
	root, err := defaultRoot()
	if err != nil {
		return nil, fmt.Errorf("cannot determine default config directory for local selection: %w", err)
	}
	return &FileSelector{root: root}, nil
}

// NewFileSelectorAt returns a FileSelector rooted at dir. The selection
// files live under dir/nats/ and the directory is created on first
// write.
func NewFileSelectorAt(dir string) *FileSelector {
	return &FileSelector{root: dir}
}

func (fs *FileSelector) selectedPath() string {
	return filepath.Join(fs.root, "nats", selectedCtxFile)
}

func (fs *FileSelector) previousPath() string {
	return filepath.Join(fs.root, "nats", previousCtxFile)
}

func (fs *FileSelector) ensureTree() error {
	if fs.root == "" {
		return fmt.Errorf("file selector has no root directory")
	}
	return os.MkdirAll(filepath.Join(fs.root, "nats"), 0700)
}

func (fs *FileSelector) readSelection(path string) string {
	if fs.root == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Selected returns the currently selected context, or ErrNoneSelected
// when nothing is selected.
func (fs *FileSelector) Selected(_ context.Context) (string, error) {
	name := fs.readSelection(fs.selectedPath())
	if name == "" {
		return "", ErrNoneSelected
	}
	return name, nil
}

// SetSelected sets name as the active context, returning whatever was
// selected before the swap. Passing an empty name clears the selection.
// Both the selected and previous files are updated via
// write-temp-then-rename so concurrent readers never see a torn value.
func (fs *FileSelector) SetSelected(_ context.Context, name string) (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	err := fs.ensureTree()
	if err != nil {
		return "", err
	}

	previous := fs.readSelection(fs.selectedPath())

	if previous != "" {
		err = writeFileAtomic(fs.previousPath(), []byte(previous), 0600)
		if err != nil {
			return "", err
		}
	}

	if name == "" {
		err = os.Remove(fs.selectedPath())
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return previous, nil
	}

	err = writeFileAtomic(fs.selectedPath(), []byte(name), 0600)
	if err != nil {
		return "", err
	}
	return previous, nil
}

// Previous returns the previously-selected context, or an empty string
// if there is none. It is not part of the Selector interface; callers
// obtain it via the package-level PreviousContext helper, which probes
// the Registry's selector for this method.
func (fs *FileSelector) Previous() string {
	return fs.readSelection(fs.previousPath())
}
