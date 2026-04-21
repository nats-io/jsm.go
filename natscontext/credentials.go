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
	"net/url"
	"strings"
)

// CredentialResolver materializes secret material referenced by a URI.
// Resolvers register with a Registry by URI scheme. Implementations
// MUST NOT log resolved bytes and SHOULD NOT include them in returned
// errors. Implementations SHOULD return a freshly-allocated slice the
// caller may clobber (NATSOptions wipes resolver output once the
// derived nats.Option has been constructed); resolvers that cache
// internally must return copies so the caller's wipe does not corrupt
// the cache.
type CredentialResolver interface {
	// Schemes returns the URI schemes handled by this resolver.
	// Returning "" designates this resolver as the fallback for
	// bare values (no scheme prefix).
	Schemes() []string
	// Resolve returns the raw bytes for ref. ref is supplied
	// unmodified so the resolver can inspect the full URI.
	Resolve(ctx context.Context, ref string) ([]byte, error)
}

// RegistryOption configures a Registry at construction time.
type RegistryOption func(*Registry)

// WithCredentialResolver registers r for each of the schemes it
// advertises. When any WithCredentialResolver or WithDefaultResolvers
// option is passed to Open, automatic default-resolver registration is
// suppressed; combine with WithDefaultResolvers to layer custom
// resolvers on top of the defaults.
func WithCredentialResolver(r CredentialResolver) RegistryOption {
	return func(reg *Registry) {
		for _, scheme := range r.Schemes() {
			reg.resolvers[scheme] = r
		}
	}
}

// WithDefaultResolvers registers the stock resolvers: file:// (and bare
// paths), op://, and nsc://. It is applied automatically when Open is
// called with no resolver-related options.
func WithDefaultResolvers() RegistryOption {
	return func(reg *Registry) {
		for _, r := range defaultResolvers() {
			for _, scheme := range r.Schemes() {
				reg.resolvers[scheme] = r
			}
		}
	}
}

// defaultResolvers returns fresh instances of the stock resolvers.
// Each call returns new instances to avoid accidental sharing of
// mutable state between Registries.
func defaultResolvers() []CredentialResolver {
	return []CredentialResolver{
		&fileResolver{},
		&opResolver{},
		&nscResolver{},
		&envResolver{},
		&dataResolver{},
	}
}

// trimSchemePrefix removes prefix from the front of ref if the
// leading bytes match case-insensitively, otherwise it returns ref
// unchanged. URI schemes are case-insensitive per RFC 3986 and
// parseScheme normalizes via url.Parse, so a value like ENV://NAME or
// FILE:///tmp/x reaches the matching resolver — this helper lets each
// resolver strip the prefix without regard to the original casing.
func trimSchemePrefix(ref, prefix string) string {
	if len(ref) >= len(prefix) && strings.EqualFold(ref[:len(prefix)], prefix) {
		return ref[len(prefix):]
	}

	return ref
}

// parseScheme returns the URI scheme (lowercased) for ref, or the
// empty string when ref does not start with one we should route
// through a resolver. url.Parse handles both "scheme://…" (op, nsc,
// file, env) and opaque "scheme:…" forms (data:), and lowercases the
// scheme for us. The only extra rule is a two-character minimum,
// which rejects Windows drive letters like "C:\\foo" — url.Parse
// would otherwise report "c" as the scheme.
func parseScheme(ref string) string {
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	if len(u.Scheme) < 2 {
		return ""
	}

	return u.Scheme
}

// resolveLiteral treats ref as a literal value unless it begins with a
// URI scheme that matches a registered resolver, or unless a fallback
// resolver is registered under the empty scheme. Resolved refs are
// returned as a string; anything else passes through unchanged.
func resolveLiteral(ctx context.Context, resolvers map[string]CredentialResolver, ref string) (string, error) {
	if ref == "" {
		return "", nil
	}
	scheme := parseScheme(ref)

	r, ok := resolvers[scheme]
	if !ok {
		return ref, nil
	}

	data, err := r.Resolve(ctx, ref)
	if err != nil {
		return "", err
	}

	out := strings.TrimRight(string(data), "\r\n")
	wipeSlice(data)

	return out, nil
}
