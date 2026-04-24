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
	"strings"
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

// ValidateName enforces the historical portable rule that shipped
// natscontext users already have on disk: a name is valid iff it is
// non-empty, does not contain the ".." substring, and does not
// contain "/" or "\". The rule is deliberately loose — whitespace,
// control characters, and the NATS subject wildcards "*" and ">" all
// pass at this layer because in-the-wild context names predate a
// stricter validator and must keep working after upgrade.
//
// Backends whose storage model cannot express some of those names
// layer their own rejection on top: for example the shipped
// svcbackend client rejects names containing whitespace, control
// characters, ".", "*", or ">" before publishing, because each of
// those would break the single-token NATS subject the wire protocol
// embeds the name in. Such backends MUST surface their additional
// rejections via ErrInvalidName so callers can tell validation
// failures apart from other errors with errors.Is.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name is empty", ErrInvalidName)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf(`%w: %q must not contain ".."`, ErrInvalidName, name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf(`%w: %q must not contain "/" or "\"`, ErrInvalidName, name)
	}
	return nil
}
