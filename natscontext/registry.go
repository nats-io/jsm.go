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
	"encoding/json"
	"errors"
	"fmt"
)

// Registry composes a Backend, a Selector, and a set of
// CredentialResolvers with the context business logic. Package-level
// helpers delegate to a lazily initialized default Registry backed by
// NewDefaultFileBackend; callers wanting a different backend or custom
// resolvers construct a Registry explicitly via Open and pass it
// around.
type Registry struct {
	backend          Backend
	selector         Selector
	selectorOverride bool
	resolvers        map[string]CredentialResolver
}

// NewRegistry returns a Registry using b for storage. If b also
// implements Selector, active-context tracking is enabled unless a
// WithSelector, WithLocalSelector, or WithoutSelection option
// explicitly overrides it. When no resolver-related options are
// supplied the default resolvers (file://, op://, nsc://) are
// registered automatically; pass WithCredentialResolver or
// WithDefaultResolvers to opt in explicitly.
func NewRegistry(b Backend, opts ...RegistryOption) *Registry {
	r := &Registry{
		backend:   b,
		resolvers: map[string]CredentialResolver{},
	}

	s, ok := b.(Selector)
	if ok {
		r.selector = s
	}

	for _, opt := range opts {
		opt(r)
	}

	if len(r.resolvers) == 0 {
		WithDefaultResolvers()(r)
	}

	return r
}

// WithSelector configures the Registry's Selector explicitly,
// overriding whatever Selector the Backend might implement. Passing
// nil panics — use WithoutSelection to disable selection tracking.
//
// The common use case is composing a remote or shared Backend with a
// local FileSelector so that every operator on a shared catalog
// keeps their own active-context state. See "Separating selection
// from storage" in BACKENDS.md.
func WithSelector(s Selector) RegistryOption {
	if s == nil {
		panic("natscontext: WithSelector called with nil Selector; use WithoutSelection to disable selection tracking")
	}

	return func(r *Registry) {
		r.selector = s
		r.selectorOverride = true
	}
}

// WithoutSelection disables selection tracking for the Registry, even
// when the Backend implements Selector. Selected returns
// ErrNoneSelected and Select returns ErrReadOnly.
func WithoutSelection() RegistryOption {
	return func(r *Registry) {
		r.selector = nil
		r.selectorOverride = true
	}
}

// WithLocalSelector tracks the active context in a local file under
// $XDG_CONFIG_HOME/nats/ (falling back to $HOME/.config/nats/),
// independent of the Registry's Backend. This is the right choice
// when the Backend is shared across users or machines — for example
// the svcbackend Client — and each operator wants their own selection
// to persist locally without affecting teammates.
//
// Equivalent to WithSelector(NewDefaultFileSelector()). Panics if
// neither XDG_CONFIG_HOME nor a resolvable HOME directory is
// available; callers on environments where that may occur (CI,
// container images without a writable HOME) should construct a
// FileSelector explicitly and pass it through WithSelector.
func WithLocalSelector() RegistryOption {
	sel, err := NewDefaultFileSelector()
	if err != nil {
		panic(fmt.Sprintf("natscontext: WithLocalSelector: %v", err))
	}

	return WithSelector(sel)
}

// Load returns the named context. If name is empty the currently selected
// context is loaded; if nothing is selected an empty Context with the
// supplied options applied is returned. A backend ErrNotFound is
// propagated to the caller so a stale selection fails fast rather
// than silently dialing nats.DefaultURL.
func (r *Registry) Load(ctx context.Context, name string, opts ...Option) (*Context, error) {
	c := &Context{config: &settings{}, resolvers: r.resolvers}

	if name == "" {
		if r.selector != nil {
			selected, err := r.selector.Selected(ctx)
			if err == nil {
				name = selected
			} else if !errors.Is(err, ErrNoneSelected) {
				return nil, err
			}
		}
		if name == "" {
			c.configureNewContext(opts...)
			return c, nil
		}
	}

	err := ValidateName(name)
	if err != nil {
		return nil, err
	}

	data, err := r.backend.Load(ctx, name)
	if err != nil {
		return nil, err
	}

	c.Name = name
	err = c.unmarshalAndExpand(data)
	if err != nil {
		return nil, err
	}

	p, hasPath := r.backend.(interface{ Path(string) string })
	if hasPath {
		c.path = p.Path(name)
	}

	c.configureNewContext(opts...)

	return c, nil
}

// Save serializes c and stores it in the backend under name. If name is
// empty the current c.Name is used. A set NSCLookup field is migrated
// into Creds as an nsc:// URI before serialization, honoring the
// deprecation path.
func (r *Registry) Save(ctx context.Context, c *Context, name string) error {
	if name != "" {
		c.Name = name
	}

	// migrateNSCLookup runs before Validate so legacy contexts that
	// still carry NSCLookup alongside an explicitly-set Creds field
	// can complete their upgrade: migrate clears NSCLookup so the
	// "too many types of credentials" check no longer trips on the
	// legacy-plus-override combination.
	err := migrateNSCLookup(ctx, c.config)
	if err != nil {
		return err
	}

	err = c.Validate()
	if err != nil {
		return err
	}

	c.config.Name = ""
	data, err := json.MarshalIndent(c.config, "", "  ")
	if err != nil {
		return err
	}

	err = r.backend.Save(ctx, c.Name, data)
	if err != nil {
		return err
	}

	p, hasPath := r.backend.(interface{ Path(string) string })
	if hasPath {
		c.path = p.Path(c.Name)
	}

	return nil
}

// migrateNSCLookup folds a set NSCLookup field into Creds as an
// nsc://<ref> URI. When both fields are set the explicit Creds wins
// and NSCLookup is cleared without invoking nsc — the user has opted
// out of nsc-driven resolution, so Save must not fail in environments
// where nsc is unavailable (minimal CI/containers). Otherwise, when no
// URL is configured, the nsc profile's Service URL is captured into
// settings.URL so reloads do not have to invoke nsc again to know
// where to connect — pre-refactor behavior where every load
// re-resolved the Service URL is preserved without forcing nsc onto
// the load path.
func migrateNSCLookup(ctx context.Context, s *settings) error {
	if s.NSCLookup == "" {
		return nil
	}

	if s.Creds == "" && s.URL == "" {
		url := s.nscUrl
		if url == "" {
			profile, err := runNscProfile(ctx, s.NSCLookup)
			if err != nil {
				return fmt.Errorf("migrate nsc lookup: %w", err)
			}
			url = profile.Service
		}
		if url != "" {
			s.URL = url
		}
	}

	if s.Creds == "" {
		s.Creds = "nsc://" + trimSchemePrefix(s.NSCLookup, "nsc://")
	}
	s.NSCLookup = ""
	s.nscCreds = ""
	s.nscUrl = ""

	return nil
}

// Delete removes name from the backend. The currently selected context
// cannot be deleted unless it is the only one; in that case the
// selection is cleared as part of the delete.
func (r *Registry) Delete(ctx context.Context, name string) error {
	err := ValidateName(name)
	if err != nil {
		return err
	}

	selected := ""
	if r.selector != nil {
		current, err := r.selector.Selected(ctx)
		if err == nil {
			selected = current
		} else if !errors.Is(err, ErrNoneSelected) {
			return err
		}
	}

	isActive := selected == name

	if isActive {
		all, err := r.backend.List(ctx)
		if err != nil {
			return err
		}
		if len(all) > 1 {
			return fmt.Errorf("%w: %q is the active context", ErrActiveContext, name)
		}
	}

	err = r.backend.Delete(ctx, name)
	if err != nil {
		return err
	}

	if isActive && r.selector != nil {
		_, err := r.selector.SetSelected(ctx, "")
		if err != nil {
			return err
		}
	}

	return nil
}

// List returns the names of all stored contexts.
func (r *Registry) List(ctx context.Context) ([]string, error) {
	return r.backend.List(ctx)
}

// Known reports whether name refers to a stored context.
func (r *Registry) Known(ctx context.Context, name string) bool {
	err := ValidateName(name)
	if err != nil {
		return false
	}

	_, err = r.backend.Load(ctx, name)

	return err == nil
}

// Selected returns the currently selected context, or ErrNoneSelected if
// no context is selected or the backend does not track selection.
func (r *Registry) Selected(ctx context.Context) (string, error) {
	if r.selector == nil {
		return "", ErrNoneSelected
	}

	return r.selector.Selected(ctx)
}

// Select marks name as the active context. The previous active context
// is returned.
func (r *Registry) Select(ctx context.Context, name string) (string, error) {
	err := ValidateName(name)
	if err != nil {
		return "", err
	}

	// Check the selector before hitting the backend: when selection
	// is disabled (WithoutSelection or a backend that does not
	// implement Selector) we must return ErrReadOnly regardless of
	// whether name exists, and a remote backend should not incur an
	// existence round-trip only to be told the operation is refused.
	if r.selector == nil {
		return "", fmt.Errorf("%w: backend does not support selection", ErrReadOnly)
	}

	// Probe existence via backend.Load so non-ErrNotFound errors
	// (transport failures, permission issues, backend-specific
	// validation rejections) propagate with their original sentinel
	// wrapping. Registry.Known collapses all errors into a bool, which
	// would mask those cases as ErrNotFound here.
	_, err = r.backend.Load(ctx, name)
	if err != nil {
		return "", err
	}

	return r.selector.SetSelected(ctx, name)
}

// Unselect clears the active context. The previous active context is
// returned; passing through when nothing was selected is a no-op.
func (r *Registry) Unselect(ctx context.Context) (string, error) {
	if r.selector == nil {
		return "", nil
	}

	return r.selector.SetSelected(ctx, "")
}

// defaultRegistry returns a Registry backed by NewDefaultFileBackend. A
// fresh instance is returned on every call so that changes to
// XDG_CONFIG_HOME (notably t.Setenv in tests) take effect without a
// reset hook. The construction cost is a single user.Current lookup at
// worst; callers that do significant work should keep their own
// Registry.
func defaultRegistry() *Registry {
	return NewRegistry(NewDefaultFileBackend())
}
