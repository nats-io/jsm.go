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
	"regexp"
)

// Error sentinels returned by Backend, Selector and Registry implementations.
// Callers should prefer errors.Is over string matching.
var (
	ErrNotFound      = errors.New("context not found")
	ErrAlreadyExists = errors.New("context already exists")
	ErrInvalidName   = errors.New("invalid context name")
	ErrActiveContext = errors.New("cannot operate on active context")
	ErrNoneSelected  = errors.New("no context selected")
	ErrReadOnly      = errors.New("backend is read-only")
	ErrConflict      = errors.New("concurrent modification detected")
)

// Backend persists context payloads. Implementations trade opaque bytes;
// encoding, validation, and credential resolution are Registry/Context
// concerns. Payloads MAY contain plaintext secrets — backends transporting
// data off-host MUST use authenticated encrypted transport.
//
// Methods accept a context.Context so remote and network-backed
// implementations can honor caller deadlines and cancellation. Purely
// local backends (file, memory) are permitted to ignore the context.
type Backend interface {
	// Load returns the raw payload for name or ErrNotFound when missing.
	Load(ctx context.Context, name string) ([]byte, error)
	// Save writes data for name, overwriting any existing payload.
	Save(ctx context.Context, name string, data []byte) error
	// Delete removes name. A missing name is a no-op.
	Delete(ctx context.Context, name string) error
	// List returns the names of all stored contexts, sorted.
	List(ctx context.Context) ([]string, error)
}

// Selector tracks the active context. SetSelected MUST be atomic with
// respect to concurrent callers. Backends that cannot track selection
// (e.g. read-only HTTP backends) simply do not implement Selector.
type Selector interface {
	// Selected returns the currently selected context name.
	// Returns ErrNoneSelected when no context is selected.
	Selected(ctx context.Context) (string, error)
	// SetSelected marks name as the active context and returns the
	// previously selected name (empty string if none). Passing an empty
	// name clears the selection.
	SetSelected(ctx context.Context, name string) (previous string, err error)
}

// invalidNameRE matches any character disallowed in a context name:
// whitespace, control characters, and any of . / \ * >. The dot and
// subject-wildcard characters are rejected so names round-trip between
// filesystem and future KV backends.
var invalidNameRE = regexp.MustCompile(`[\s/\\.*>\x00-\x1f\x7f]`)

// ValidateName checks that name conforms to the portable intersection
// accepted by all shipped Backends: non-empty, and none of whitespace,
// control characters, or the characters . / \ * >. Backends MAY surface
// additional rejections via ErrInvalidName for backend-specific reasons.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name is empty", ErrInvalidName)
	}
	if invalidNameRE.MatchString(name) {
		return fmt.Errorf(`%w: %q must not contain whitespace, control characters, or any of . / \ * >`, ErrInvalidName, name)
	}
	return nil
}
