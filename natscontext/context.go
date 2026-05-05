// Copyright 2020-2026 The NATS Authors
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

// Package natscontext stores named sets of NATS connection settings and
// resolves the credentials they reference at connect time.
//
// The package composes two orthogonal abstractions:
//
//   - A Backend persists context payloads as opaque bytes. Shipped
//     backends include FileBackend (XDG-based JSON files),
//     SingleFileBackend (one context at a caller-supplied path), and
//     MemoryBackend (for tests and embeds).
//   - A CredentialResolver materializes secret material referenced by
//     URI. Stock resolvers handle file://, op://, nsc://, env://, and
//     data: inline payloads.
//
// A Registry composes one Backend with one or more CredentialResolvers;
// construct one explicitly with NewRegistry for custom storage or
// resolver sets. Package-level helpers (New, NewFromFile, Connect,
// KnownContexts, SelectContext, and so on) delegate to a lazily
// initialized default Registry backed by NewDefaultFileBackend, which
// reads and writes under ~/.config/nats (or $XDG_CONFIG_HOME/nats):
//
//	~/.config/nats/context/<name>.json   // one file per context
//	~/.config/nats/context.txt            // currently selected context
//
// See BACKENDS.md for the Backend and CredentialResolver contracts and
// guidance on adding new implementations.
package natscontext

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"runtime"
	"strings"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats-server/v2/server/certstore"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

type Option func(c *settings)

type settings struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description"`
	URL         string `json:"url"`
	nscUrl      string
	SocksProxy  string `json:"socks_proxy"`
	Token       string `json:"token"`
	User        string `json:"user"`
	Password    string `json:"password"`
	Creds       string `json:"creds"`
	nscCreds    string
	NKey        string `json:"nkey"`
	Cert        string `json:"cert"`
	Key         string `json:"key"`
	CA          string `json:"ca"`
	// NSCLookup is loaded forever for backward compatibility but MUST
	// NOT be set on new contexts. Registry.Save rewrites it into Creds
	// as an nsc://<path> URI and clears the field.
	//
	// Deprecated: use Creds with an nsc:// scheme instead.
	NSCLookup           string   `json:"nsc"`
	JSDomain            string   `json:"jetstream_domain"`
	JSAPIPrefix         string   `json:"jetstream_api_prefix"`
	JSEventPrefix       string   `json:"jetstream_event_prefix"`
	InboxPrefix         string   `json:"inbox_prefix"`
	UserJwt             string   `json:"user_jwt"`
	UserSeed            string   `json:"user_seed"`
	ColorScheme         string   `json:"color_scheme"`
	TLSFirst            bool     `json:"tls_first"`
	WinCertStoreType    string   `json:"windows_cert_store"`
	WinCertStoreMatchBy string   `json:"windows_cert_match_by"`
	WinCertStoreMatch   string   `json:"windows_cert_match"`
	WinCertStoreCaMatch []string `json:"windows_ca_certs_match"`
}

type Context struct {
	Name      string `json:"-"`
	config    *settings
	path      string
	resolvers map[string]CredentialResolver
}

// String returns a redacted summary of the context so %s, %v, and %+v
// never bleed plaintext credentials. Callers that need the raw
// material should reach for the typed accessors (Creds, Token, …).
func (c *Context) String() string {
	if c == nil {
		return "natscontext.Context{<nil>}"
	}
	url := ""
	if c.config != nil {
		url = redactServerURL(c.ServerURL())
	}

	return fmt.Sprintf("natscontext.Context{name=%q url=%q auth=%s credentials=<redacted>}",
		c.Name, url, c.authLabel())
}

// redactServerURL returns urls with any user-info component replaced
// by "xxxxx@". NATS server URLs may carry user:password@host or
// token@host forms (the comma-separated list form is also supported),
// either of which would otherwise leak through Context.String. Each
// element is parsed independently; elements that do not parse as URLs
// or that carry no user-info pass through unchanged.
func redactServerURL(urls string) string {
	if urls == "" {
		return ""
	}
	parts := strings.Split(urls, ",")
	for i, p := range parts {
		u, err := url.Parse(strings.TrimSpace(p))
		if err != nil || u.User == nil {
			continue
		}
		u.User = url.User("xxxxx")
		parts[i] = u.String()
	}

	return strings.Join(parts, ",")
}

// GoString mirrors String so %#v is safe too.
func (c *Context) GoString() string {
	return c.String()
}

// authLabel identifies which credential scheme a context is configured
// with without revealing any of the material itself.
func (c *Context) authLabel() string {
	if c == nil || c.config == nil {
		return "none"
	}
	switch {
	case c.config.User != "":
		return "user/password"
	case c.config.Creds != "" || c.config.nscCreds != "":
		return "creds"
	case c.config.NKey != "":
		return "nkey"
	case c.config.UserJwt != "":
		return "user_jwt"
	case c.config.Token != "":
		return "token"
	default:
		return "none"
	}
}

// String on *settings makes %+v on a *Context (which embeds a
// *settings pointer) safe even when callers reach into the unexported
// field via reflection-based dumpers.
func (s *settings) String() string {
	if s == nil {
		return "<nil>"
	}

	return "<redacted>"
}

// GoString keeps %#v output in lockstep with String.
func (s *settings) GoString() string {
	return s.String()
}

// New loads a new configuration context. If name is empty the current active
// one will be loaded.  If load is false no loading of existing data is done
// this is mainly useful to create new empty contexts.
//
// When opts is supplied those settings will override what was loaded or supply
// values for an empty context
func New(name string, load bool, opts ...Option) (*Context, error) {
	if !load {
		c := &Context{Name: name, config: &settings{}}
		c.configureNewContext(opts...)
		return c, nil
	}

	return defaultRegistry().Load(context.Background(), name, opts...)
}

// NewFromFile loads a new configuration context from the given
// filename. It constructs a SingleFileBackend so the load, unmarshal,
// path-capture, and resolver-population logic is identical to a
// Registry.Load against any other Backend — no separate disk IO path.
//
// When opts is supplied those settings will override what was loaded
// or supply values for an empty context.
func NewFromFile(filename string, opts ...Option) (*Context, error) {
	backend := NewSingleFileBackend(filename)
	name := backend.(*SingleFileBackend).Name()

	return NewRegistry(backend).Load(context.Background(), name, opts...)
}

// unmarshalAndExpand decodes data into c.config and expands ~ and
// environment variables in path-bearing fields. It is shared by
// Registry.Load and NewFromFile so both code paths post-process loaded
// payloads identically. When the deprecated NSCLookup field is set it
// is resolved eagerly so legacy callers still see ServerURL() populated
// from the nsc output.
func (c *Context) unmarshalAndExpand(data []byte) error {
	err := json.Unmarshal(data, c.config)
	if err != nil {
		return err
	}

	c.config.Creds = expandHomedir(c.config.Creds)
	c.config.NKey = expandHomedir(c.config.NKey)
	c.config.Cert = expandHomedir(c.config.Cert)
	c.config.Key = expandHomedir(c.config.Key)
	c.config.CA = expandHomedir(c.config.CA)

	if c.config.NSCLookup != "" {
		profile, err := runNscProfile(context.Background(), c.config.NSCLookup)
		if err != nil {
			return err
		}
		c.config.nscCreds = profile.UserCreds
		c.config.nscUrl = profile.Service
	}

	return nil
}

func (c *Context) configureNewContext(opts ...Option) {
	// apply supplied overrides
	for _, opt := range opts {
		opt(c.config)
	}

	if c.config.NSCLookup == "" && c.config.URL == "" && c.config.nscUrl == "" {
		c.config.URL = nats.DefaultURL
	}
}

// Connect connects to the NATS server configured by the named context, empty name connects to selected context
func Connect(name string, opts ...nats.Option) (*nats.Conn, error) {
	nctx, err := New(name, true)
	if err != nil {
		return nil, err
	}

	return nctx.Connect(opts...)
}

// DeleteContext deletes a context with a given name, the active context
// can not be deleted unless it's the only context.
func DeleteContext(name string) error {
	return defaultRegistry().Delete(context.Background(), name)
}

// IsKnown determines if a context is known
func IsKnown(name string) bool {
	return defaultRegistry().Known(context.Background(), name)
}

// ContextPath is the path on disk to store the context. The default
// filesystem backend is intrinsically path-based; this helper is not
// available on Registry because non-filesystem backends have no
// meaningful path.
func ContextPath(name string) (string, error) {
	err := ValidateName(name)
	if err != nil {
		return "", err
	}

	root, err := defaultRoot()
	if err != nil {
		return "", err
	}

	b, ok := NewFileBackendAt(root).(*FileBackend)
	if !ok {
		return "", fmt.Errorf("default backend does not expose a path")
	}

	return b.Path(name), nil
}

// KnownContexts is a list of known context names.
func KnownContexts() []string {
	list, err := defaultRegistry().List(context.Background())
	if err != nil || list == nil {
		return []string{}
	}

	return list
}

// SelectedContext returns the name of the current selected context, empty when none is selected.
func SelectedContext() string {
	name, err := defaultRegistry().Selected(context.Background())
	if err != nil {
		return ""
	}

	return name
}

// PreviousContext returns the name of the previous selected context, empty if it hasn't been selected before.
func PreviousContext() string {
	reg := defaultRegistry()
	type previouser interface {
		Previous() string
	}

	// A configured Selector owns "previous"; probe the backend only when
	// the Registry has no Selector. Falling back across the two would
	// mix selection state from different sources.
	if reg.selector != nil {
		p, ok := reg.selector.(previouser)
		if !ok {
			return ""
		}
		return p.Previous()
	}

	p, ok := reg.backend.(previouser)
	if !ok {
		return ""
	}

	return p.Previous()
}

// Connect connects to the configured NATS server
func (c *Context) Connect(opts ...nats.Option) (*nats.Conn, error) {
	nopts, err := c.NATSOptions(opts...)
	if err != nil {
		return nil, err
	}

	return nats.Connect(c.ServerURL(), nopts...)
}

// JSMOptions creates options for the jsm manager
func (c *Context) JSMOptions(opts ...jsm.Option) ([]jsm.Option, error) {
	jsmopts := []jsm.Option{
		jsm.WithAPIPrefix(c.JSAPIPrefix()),
		jsm.WithEventPrefix(c.JSEventPrefix()),
		jsm.WithDomain(c.JSDomain()),
	}

	return append(jsmopts, opts...), nil
}

// NATSOptions creates NATS client configuration based on the contents of the context.
func (c *Context) NATSOptions(opts ...nats.Option) ([]nats.Option, error) {
	ctx := context.Background()
	resolvers := c.activeResolvers()

	var nopts []nats.Option

	switch {
	case c.User() != "":
		password, err := resolveLiteral(ctx, resolvers, c.Password())
		if err != nil {
			return nil, fmt.Errorf("password: %w", err)
		}
		nopts = append(nopts, nats.UserInfo(c.User(), password))

	case c.Creds() != "":
		opt, err := buildCredsOption(ctx, resolvers, c.Creds())
		if err != nil {
			return nil, err
		}
		nopts = append(nopts, opt)

	case c.NKey() != "":
		opt, err := buildNkeyOption(ctx, resolvers, c.NKey())
		if err != nil {
			return nil, err
		}
		nopts = append(nopts, opt)

	case c.UserJWT() != "" && c.UserSeed() != "":
		jwt, err := resolveLiteral(ctx, resolvers, c.UserJWT())
		if err != nil {
			return nil, fmt.Errorf("user_jwt: %w", err)
		}
		seed, err := resolveLiteral(ctx, resolvers, c.UserSeed())
		if err != nil {
			return nil, fmt.Errorf("user_seed: %w", err)
		}
		nopts = append(nopts, nats.UserJWTAndSeed(jwt, seed))

	case c.UserJWT() != "" && c.UserSeed() == "":
		jwt, err := resolveLiteral(ctx, resolvers, c.UserJWT())
		if err != nil {
			return nil, fmt.Errorf("user_jwt: %w", err)
		}
		userCB := func() (string, error) {
			return jwt, nil
		}
		sigCB := func(nonce []byte) ([]byte, error) {
			return nil, nil
		}
		nopts = append(nopts, nats.UserJWT(userCB, sigCB))
	}

	if c.Token() != "" {
		token, err := resolveLiteral(ctx, resolvers, c.Token())
		if err != nil {
			return nil, fmt.Errorf("token: %w", err)
		}
		nopts = append(nopts, nats.Token(token))
	}

	if c.Certificate() != "" && c.Key() != "" {
		certPath, err := requireFilePath(c.Certificate(), "cert")
		if err != nil {
			return nil, err
		}
		keyPath, err := requireFilePath(c.Key(), "key")
		if err != nil {
			return nil, err
		}
		nopts = append(nopts, nats.ClientCert(certPath, keyPath))
	}

	if c.CA() != "" {
		caPath, err := requireFilePath(c.CA(), "ca")
		if err != nil {
			return nil, err
		}
		nopts = append(nopts, nats.RootCAs(caPath))
	}

	if c.SocksProxy() != "" {
		nopts = append(nopts, nats.SetCustomDialer(c.SOCKSDialer()))
	}

	if c.InboxPrefix() != "" {
		nopts = append(nopts, nats.CustomInboxPrefix(c.InboxPrefix()))
	}

	if c.TLSHandshakeFirst() {
		nopts = append(nopts, nats.TLSHandshakeFirst())
	}

	csOpts, err := c.certStoreNatsOptions()
	if err != nil {
		return nil, err
	}
	nopts = append(nopts, csOpts...)

	nopts = append(nopts, opts...)

	return nopts, nil
}

// activeResolvers returns the resolver set that should drive scheme
// dispatch for this context. Registry.Load and NewFromFile populate
// c.resolvers, but contexts built by options alone (e.g. New(name,
// false, WithCreds(...))) fall back to the package-wide defaults.
func (c *Context) activeResolvers() map[string]CredentialResolver {
	if c.resolvers != nil {
		return c.resolvers
	}

	return defaultRegistry().resolvers
}

// buildCredsOption turns a Creds URI into a nats.Option. File-scheme
// references (including bare paths) are passed to nats.UserCredentials
// so credentials are read lazily at dial time. Non-file schemes are
// resolved eagerly to bytes and routed through nats.UserJWT with
// callbacks so nats.go never sees a path.
func buildCredsOption(ctx context.Context, resolvers map[string]CredentialResolver, ref string) (nats.Option, error) {
	scheme := parseScheme(ref)
	if scheme == "" || scheme == "file" {
		return nats.UserCredentials(filePathFromRef(ref)), nil
	}

	r, ok := resolvers[scheme]
	if !ok {
		return nil, fmt.Errorf("creds: no resolver registered for scheme %q", scheme)
	}
	data, err := r.Resolve(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("creds: %w", err)
	}
	defer wipeSlice(data)

	jwt, err := nkeys.ParseDecoratedJWT(data)
	if err != nil {
		return nil, fmt.Errorf("creds: %w", err)
	}
	kp, err := nkeys.ParseDecoratedNKey(data)
	if err != nil {
		return nil, fmt.Errorf("creds: %w", err)
	}

	userCB := func() (string, error) {
		return jwt, nil
	}
	sigCB := func(nonce []byte) ([]byte, error) {
		return kp.Sign(nonce)
	}

	return nats.UserJWT(userCB, sigCB), nil
}

// buildNkeyOption turns an NKey URI into a nats.Option. File-scheme
// references use nats.NkeyOptionFromSeed for byte-for-byte parity with
// the pre-refactor behavior; other schemes resolve the seed bytes and
// construct the option from nats.Nkey directly.
func buildNkeyOption(ctx context.Context, resolvers map[string]CredentialResolver, ref string) (nats.Option, error) {
	scheme := parseScheme(ref)
	if scheme == "" || scheme == "file" {
		return nats.NkeyOptionFromSeed(filePathFromRef(ref))
	}

	r, ok := resolvers[scheme]
	if !ok {
		return nil, fmt.Errorf("nkey: no resolver registered for scheme %q", scheme)
	}
	data, err := r.Resolve(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("nkey: %w", err)
	}
	defer wipeSlice(data)

	kp, err := nkeys.ParseDecoratedNKey(data)
	if err != nil {
		return nil, fmt.Errorf("nkey: %w", err)
	}
	pub, err := kp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("nkey: %w", err)
	}
	sigCB := func(nonce []byte) ([]byte, error) {
		return kp.Sign(nonce)
	}

	return nats.Nkey(pub, sigCB), nil
}

// requireFilePath returns a resolvable filesystem path for a TLS field
// (cert, key, CA). Supporting non-file schemes here needs in-memory
// nats.go equivalents that do not yet exist, so non-file schemes are
// rejected with a clear error instead of being silently ignored.
func requireFilePath(ref, field string) (string, error) {
	scheme := parseScheme(ref)
	if scheme != "" && scheme != "file" {
		return "", fmt.Errorf("%s: scheme %q not supported (file/bare only)", field, scheme)
	}

	return filePathFromRef(ref), nil
}

func (c *Context) parseWinCertStoreType(t string) (certstore.StoreType, error) {
	storeTypeString := t
	switch storeTypeString {
	case "machine":
		storeTypeString = "windowslocalmachine"
	case "user":
		storeTypeString = "windowscurrentuser"
	}

	return certstore.ParseCertStore(storeTypeString)
}

func (c *Context) certStoreNatsOptions() ([]nats.Option, error) {
	if c.config.WinCertStoreType == "" {
		return nil, nil
	}

	storeType, err := c.parseWinCertStoreType(c.config.WinCertStoreType)
	if err != nil {
		return nil, err
	}

	matchBy, err := certstore.ParseCertMatchBy(c.config.WinCertStoreMatchBy)
	if err != nil {
		return nil, err
	}

	tlsc := &tls.Config{}
	err = certstore.TLSConfig(storeType, matchBy, c.config.WinCertStoreMatch, c.config.WinCertStoreCaMatch, true, tlsc)
	if err != nil {
		return nil, err
	}

	if tlsc.ClientCAs != nil {
		tlsc.RootCAs = tlsc.ClientCAs
		tlsc.ClientCAs = nil
	}

	// if no ca match was given but we have CA as a file lets pull in that file here
	if len(c.config.WinCertStoreCaMatch) == 0 && c.config.CA != "" {
		caPath, err := requireFilePath(c.config.CA, "ca")
		if err != nil {
			return nil, err
		}

		rootCAs, _ := x509.SystemCertPool()
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}

		certs, err := os.ReadFile(caPath)
		if err != nil {
			return nil, err
		}

		ok := rootCAs.AppendCertsFromPEM(certs)
		if !ok {
			return nil, fmt.Errorf("failed to append CA certificates from %s", caPath)
		}

		tlsc.RootCAs = rootCAs
	}

	return []nats.Option{nats.Secure(tlsc)}, nil
}

func expandHomedir(path string) string {
	path = os.ExpandEnv(path)

	if len(path) == 0 || path[0] != '~' {
		return path
	}

	usr, err := user.Current()
	if err != nil {
		return path
	}

	return strings.Replace(path, "~", usr.HomeDir, 1)
}

func numCreds(c *Context) int {
	i := 0
	creds := []string{
		c.config.User,
		c.config.Creds,
		c.config.NKey,
		c.config.NSCLookup,
		c.config.UserJwt,
	}

	for _, c := range creds {
		if c != "" {
			i++
		}
	}

	return i
}

// UnSelectContext clears the active context selection.
func UnSelectContext() error {
	_, err := defaultRegistry().Unselect(context.Background())

	return err
}

// SelectContext sets the given context to be the default, error if it does not exist.
func SelectContext(name string) error {
	_, err := defaultRegistry().Select(context.Background(), name)

	return err
}

func (c *Context) MarshalJSON() ([]byte, error) {
	c.config.Name = c.Name

	return json.MarshalIndent(c.config, "", "  ")
}

func (c *Context) Validate() error {
	err := ValidateName(c.Name)
	if err != nil {
		return err
	}

	if numCreds(c) > 1 {
		return errors.New("too many types of credentials. Choose only one from 'user/token', 'creds', 'nkey', 'nsc'")
	}

	if c.config.WinCertStoreType != "" {
		_, err := c.parseWinCertStoreType(c.config.WinCertStoreType)
		if err != nil {
			return err
		}
	}

	if c.config.WinCertStoreMatchBy != "" {
		_, err := certstore.ParseCertMatchBy(c.config.WinCertStoreMatchBy)
		if err != nil {
			return err
		}
	}

	if c.config.WinCertStoreType != "" && c.config.WinCertStoreMatch == "" {
		return fmt.Errorf("windows certificate store requires a matcher")
	}

	return nil
}

// Save saves the current context to name. When the context was loaded
// from a specific file (via NewFromFile, or via a Registry that
// recorded a path) and the effective save name matches the loaded
// basename, Save writes back to that original path through a
// SingleFileBackend-backed Registry. Anything else — an explicit
// rename via the name argument, or a rename via mutation of c.Name
// before Save("") — is treated as a rename and goes through the
// package-level default Registry so it lands in the XDG filesystem
// hierarchy. Callers using a custom Registry should prefer
// Registry.Save directly.
func (c *Context) Save(name string) error {
	if c.path != "" {
		backend := NewSingleFileBackend(c.path).(*SingleFileBackend)
		effective := name
		if effective == "" {
			effective = c.Name
		}
		if effective == backend.Name() {
			return NewRegistry(backend).Save(context.Background(), c, name)
		}
	}

	return defaultRegistry().Save(context.Background(), c, name)
}

// WithServerURL supplies the url(s) to connect to nats with
func WithServerURL(url string) Option {
	return func(s *settings) {
		if url != "" {
			s.URL = url
		}
	}
}

// ServerURL is the configured server urls, 'nats://localhost:4222' if not set
func (c *Context) ServerURL() string {
	switch {
	case c.config.URL != "":
		return c.config.URL
	case c.config.nscUrl != "":
		return c.config.nscUrl
	default:
		return nats.DefaultURL
	}
}

// WithUser sets the username
func WithUser(u string) Option {
	return func(s *settings) {
		if u != "" {
			s.User = u
		}
	}
}

// User is the configured username, empty if not set
func (c *Context) User() string { return c.config.User }

// WithPassword sets the password
func WithPassword(p string) Option {
	return func(s *settings) {
		if p != "" {
			s.Password = p
		}
	}
}

// Password retrieves the configured password, empty if not set
func (c *Context) Password() string { return c.config.Password }

// WithCreds sets the credentials file
func WithCreds(c string) Option {
	return func(s *settings) {
		if c != "" {
			s.Creds = c
		}
	}
}

// Creds retrieves the configured credentials file path, empty if not set
func (c *Context) Creds() string {
	switch {
	case c.config.Creds != "":
		return c.config.Creds
	case c.config.nscCreds != "":
		return c.config.nscCreds
	default:
		return ""
	}
}

// WithNKey sets the nkey path
func WithNKey(n string) Option {
	return func(s *settings) {
		if n != "" {
			s.NKey = n
		}
	}
}

// Token retrieves the configured token, empty if not set
func (c *Context) Token() string { return c.config.Token }

// WithToken sets the token to use for authentication
func WithToken(t string) Option {
	return func(s *settings) {
		if t != "" {
			s.Token = t
		}
	}
}

// NKey retrieves the configured nkey path, empty if not set
func (c *Context) NKey() string { return c.config.NKey }

// WithCertificate sets the path to the public certificate
func WithCertificate(c string) Option {
	return func(s *settings) {
		if c != "" {
			s.Cert = c
		}
	}
}

// Certificate retrieves the path to the public certificate, empty if not set
func (c *Context) Certificate() string { return c.config.Cert }

// WithKey sets the private key path to use
func WithKey(k string) Option {
	return func(s *settings) {
		if k != "" {
			s.Key = k
		}
	}
}

// Key retrieves the private key path, empty if not set
func (c *Context) Key() string { return c.config.Key }

// WithCA sets the CA certificate path to use
func WithCA(ca string) Option {
	return func(s *settings) {
		if ca != "" {
			s.CA = ca
		}
	}
}

// CA retrieves the CA file path, empty if not set
func (c *Context) CA() string { return c.config.CA }

// WithDescription sets a freiendly description for this context
func WithDescription(d string) Option {
	return func(s *settings) {
		if d != "" {
			s.Description = d
		}
	}
}

// ColorScheme is a color scheme hint for CLI tools, valid values depend on the tool
func (c *Context) ColorScheme() string { return c.config.ColorScheme }

// WithColorScheme allows a color scheme to be recorded, valid values depend on the tool
func WithColorScheme(scheme string) Option {
	return func(s *settings) {
		if scheme != "" {
			s.ColorScheme = scheme
		}
	}
}

// WithNscUrl queries nsc for a credential based on a url like nsc://<operator>/<account>/<user>
func WithNscUrl(u string) Option {
	return func(s *settings) {
		s.NSCLookup = u
	}
}

// NscURL is the url used to resolve credentials in nsc. Returns ""
// for contexts that have been saved since the legacy field was
// migrated into Creds; use Creds() with parseScheme to check for the
// nsc:// scheme instead.
//
// Deprecated: use Creds() and inspect for an nsc:// scheme.
func (c *Context) NscURL() string { return c.config.NSCLookup }

// Description retrieves the description, empty if not set
func (c *Context) Description() string { return c.config.Description }

// Path returns the path on disk for a loaded context, empty when not saved or loaded
func (c *Context) Path() string { return c.path }

// WithJSAPIPrefix sets the prefix to use for JetStream API
func WithJSAPIPrefix(p string) Option {
	return func(s *settings) {
		if p != "" {
			s.JSAPIPrefix = p
		}
	}
}

// JSAPIPrefix is the subject prefix to use when accessing JetStream API
func (c *Context) JSAPIPrefix() string { return c.config.JSAPIPrefix }

// WithJSEventPrefix sets the prefix to use for JetStream Events
func WithJSEventPrefix(p string) Option {
	return func(s *settings) {
		if p != "" {
			s.JSEventPrefix = p
		}
	}
}

// JSEventPrefix is the subject prefix to use when accessing JetStream events
func (c *Context) JSEventPrefix() string { return c.config.JSEventPrefix }

func WithJSDomain(domain string) Option {
	return func(s *settings) {
		if domain != "" {
			s.JSDomain = domain
		}
	}
}

// JSDomain is the configured JetStream domain
func (c *Context) JSDomain() string {
	return c.config.JSDomain
}

// WithInboxPrefix sets a custom prefix for request-reply inboxes
func WithInboxPrefix(p string) Option {
	return func(s *settings) {
		if p != "" {
			s.InboxPrefix = p
		}
	}
}

// InboxPrefix is the configured inbox prefix for request-reply inboxes
func (c *Context) InboxPrefix() string {
	return c.config.InboxPrefix
}

// WithUserJWT sets the user jwt
func WithUserJWT(p string) Option {
	return func(s *settings) {
		if p != "" {
			s.UserJwt = p
		}
	}
}

// WithUserSeed sets the user seed
func WithUserSeed(p string) Option {
	return func(s *settings) {
		if p != "" {
			s.UserSeed = p
		}
	}
}

// UserJWT retrieves the configured user jwt, empty if not set
func (c *Context) UserJWT() string {
	return c.config.UserJwt
}

// UserSeed retrieves the configured user seed, empty if not set
func (c *Context) UserSeed() string {
	return c.config.UserSeed
}

// WithSocksProxy sets the SOCKS5 Proxy.
// To explicitly remove an already configured proxy, use the string "none".
func WithSocksProxy(p string) Option {
	return func(s *settings) {
		if p == "none" || p == "NONE" || p == "-" {
			s.SocksProxy = ""
		} else if p != "" {
			s.SocksProxy = p
		}
	}
}

// SocksProxy retrieves the configured SOCKS5 Proxy, empty if not set
func (c *Context) SocksProxy() string {
	return c.config.SocksProxy
}

// WithTLSHandshakeFirst configures the client to send TLS handshakes before waiting for server INFO
func WithTLSHandshakeFirst() Option {
	return func(s *settings) {
		s.TLSFirst = true
	}
}

// TLSHandshakeFirst configures the connection to do a TLS Handshake before expecting server INFO
func (c *Context) TLSHandshakeFirst() bool {
	return c.config.TLSFirst
}

// WithWindowsCertStore configures TLS to use a Windows Certificate Store. Valid values are "user" or "machine"
func WithWindowsCertStore(storeType string) Option {
	return func(s *settings) {
		if storeType != "" {
			s.WinCertStoreType = storeType
		}
	}
}

// WindowsCertStore indicates if the cert store should be used and which type
func (c *Context) WindowsCertStore() string { return c.config.WinCertStoreType }

// WithWindowsCertStoreMatchBy configures Matching behavior for Windows Certificate Store. Valid values are "issuer" or "subject"
func WithWindowsCertStoreMatchBy(matchBy string) Option {
	return func(s *settings) {
		if matchBy != "" {
			s.WinCertStoreMatchBy = matchBy
		}
	}
}

// WindowsCertStoreMatchBy indicates which property will be used to search in the store
func (c *Context) WindowsCertStoreMatchBy() string { return c.config.WinCertStoreMatchBy }

// WithWindowsCertStoreMatch configures the matcher query to select certificates with, see WithWindowsCertStoreMatchBy
func WithWindowsCertStoreMatch(match string) Option {
	return func(s *settings) {
		if match != "" {
			s.WinCertStoreMatch = match
		}
	}
}

// WindowsCertStoreMatch is the string to use when searching a certificate in the windows certificate store
func (c *Context) WindowsCertStoreMatch() string { return c.config.WinCertStoreMatch }

// WithWindowsCaCertsMatch configures criteria used to search for Certificate Authorities in the windows certificate store
func WithWindowsCaCertsMatch(match ...string) Option {
	return func(s *settings) {
		if len(match) > 0 {
			s.WinCertStoreCaMatch = match
		}
	}
}

// WindowsCaCertsMatch are criteria used to search for Certificate Authorities in the windows certificate store
func (c *Context) WindowsCaCertsMatch() []string { return c.config.WinCertStoreCaMatch }

func wipeSlice(buf []byte) {
	for i := range buf {
		buf[i] = 'x'
	}
	// KeepAlive prevents the compiler from treating the loop as a dead store
	// and eliding the writes before the slice is garbage collected.
	runtime.KeepAlive(buf)
}
