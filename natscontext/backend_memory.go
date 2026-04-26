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
	"fmt"
	"sort"
	"sync"
)

// MemoryBackend stores contexts and selection state in memory behind a
// mutex. It implements both Backend and Selector. Intended primarily
// for tests and for embeds that do not need persistence; do not rely
// on it as a durable store.
type MemoryBackend struct {
	mu       sync.Mutex
	contexts map[string][]byte
	selected string
	previous string
}

// NewMemoryBackend returns a ready-to-use MemoryBackend.
func NewMemoryBackend() Backend {
	return &MemoryBackend{contexts: map[string][]byte{}}
}

// Load returns a copy of the stored bytes for name, or ErrNotFound
// when nothing is stored.
func (m *MemoryBackend) Load(_ context.Context, name string) ([]byte, error) {
	err := ValidateName(name)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	data, ok := m.contexts[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out, nil
}

// Save stores a copy of data under name so callers cannot mutate the
// stored payload after the call returns.
func (m *MemoryBackend) Save(_ context.Context, name string, data []byte) error {
	err := ValidateName(name)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	stored := make([]byte, len(data))
	copy(stored, data)
	m.contexts[name] = stored
	return nil
}

// Delete removes name. A missing name is a no-op.
func (m *MemoryBackend) Delete(_ context.Context, name string) error {
	err := ValidateName(name)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.contexts, name)
	return nil
}

// List returns the stored names, sorted.
func (m *MemoryBackend) List(_ context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	names := make([]string, 0, len(m.contexts))
	for n := range m.contexts {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

// Selected returns the current selection, or ErrNoneSelected when none
// is set.
func (m *MemoryBackend) Selected(_ context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.selected == "" {
		return "", ErrNoneSelected
	}
	return m.selected, nil
}

// SetSelected atomically swaps the active context to name, returning
// the prior value. An empty name clears the selection.
func (m *MemoryBackend) SetSelected(_ context.Context, name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	previous := m.selected
	if previous != "" {
		m.previous = previous
	}
	m.selected = name
	return previous, nil
}

// Previous returns the prior selection. Not part of the Selector
// interface; PreviousContext reaches for it through a type assertion
// the same way it does for FileBackend.
func (m *MemoryBackend) Previous() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.previous
}
