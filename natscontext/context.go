// Copyright 2020-2023 The NATS Authors
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

// Package natscontext provides a way for sets of configuration options to be stored
// in named files and later retrieved either by name or if no name is supplied by access
// a chosen default context.
//
// Files are stored in ~/.config/nats or in the directory set by XDG_CONFIG_HOME environment
//
//	.config/nats
//	.config/nats/context
//	.config/nats/context/ngs.js.json
//	.config/nats/context/ngs.stats.json
//	.config/nats/context.txt
//
// Here the context.txt holds simply the string matching a context name like 'ngs.js'
package natscontext

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats-server/v2/server/certstore"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

type Option func(c *settings)

type settings struct {
	Name                string `json:"name,omitempty"`
	Description         string `json:"description"`
	URL                 string `json:"url"`
	nscUrl              string
	SocksProxy          string `json:"socks_proxy"`
	Token               string `json:"token"`
	User                string `json:"user"`
	Password            string `json:"password"`
	Creds               string `json:"creds"`
	nscCreds            string
	NKey                string   `json:"nkey"`
	Cert                string   `json:"cert"`
	Key                 string   `json:"key"`
	CA                  string   `json:"ca"`
	NSCLookup           string   `json:"nsc"`
	JSDomain            string   `json:"jetstream_domain"`
	JSAPIPrefix         string   `json:"jetstream_api_prefix"`
	JSEventPrefix       string   `json:"jetstream_event_prefix"`
	InboxPrefix         string   `json:"inbox_prefix"`
	UserJwt             string   `json:"user_jwt"`
	ColorScheme         string   `json:"color_scheme"`
	TLSFirst            bool     `json:"tls_first"`
	WinCertStoreType    string   `json:"windows_cert_store"`
	WinCertStoreMatchBy string   `json:"windows_cert_match_by"`
	WinCertStoreMatch   string   `json:"windows_cert_match"`
	WinCertStoreCaMatch []string `json:"windows_ca_certs_match"`
}

type Context struct {
	Name   string `json:"-"`
	config *settings
	path   string
}

const (
	selectedCtxFile string = "context.txt"
	previousCtxFile string = "previous-context.txt"
)

// New loads a new configuration context. If name is empty the current active
// one will be loaded.  If load is false no loading of existing data is done
// this is mainly useful to create new empty contexts.
//
// When opts is supplied those settings will override what was loaded or supply
// values for an empty context
func New(name string, load bool, opts ...Option) (*Context, error) {
	c := &Context{
		Name:   name,
		config: &settings{},
	}

	if load {
		err := c.loadActiveContext()
		if err != nil {
			return nil, err
		}
	}

	c.configureNewContext(opts...)

	return c, nil
}

// NewFromFile loads a new configuration context from the given filename.
//
// When opts is supplied those settings will override what was loaded or supply
// values for an empty context
func NewFromFile(filename string, opts ...Option) (*Context, error) {
	c := &Context{
		Name:   strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)),
		config: &settings{},
		path:   filename,
	}

	err := c.loadActiveContext()
	if err != nil {
		return nil, err
	}

	c.configureNewContext(opts...)

	return c, nil
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

func parentDir() (string, error) {
	parent := os.Getenv("XDG_CONFIG_HOME")
	if parent != "" {
		return parent, nil
	}

	u, err := user.Current()
	if err != nil {
		return "", err
	}

	if u.HomeDir == "" {
		return "", fmt.Errorf("cannot determine home directory")
	}

	return filepath.Join(u.HomeDir, parent, ".config"), nil
}

// DeleteContext deletes a context with a given name, the active context
// can not be deleted unless it's the only context
func DeleteContext(name string) error {
	if !validName(name) {
		return fmt.Errorf("invalid context name %q", name)
	}

	known := KnownContexts()
	selected := SelectedContext() == name

	if selected && len(known) > 1 {
		return fmt.Errorf("cannot remove the current active context")
	}

	parent, err := parentDir()
	if err != nil {
		return err
	}

	cfile := filepath.Join(parent, "nats", "context", name+".json")
	_, err = os.Stat(cfile)
	if os.IsNotExist(err) {
		return nil
	}

	err = os.Remove(cfile)
	if err != nil {
		return err
	}

	if selected {
		return os.Remove(filepath.Join(parent, "nats", selectedCtxFile))
	}

	return nil
}

// IsKnown determines if a context is known
func IsKnown(name string) bool {
	if !validName(name) {
		return false
	}

	parent, err := parentDir()
	if err != nil {
		return false
	}

	return knownContext(parent, name)
}

// ContextPath is the path on disk to store the context
func ContextPath(name string) (string, error) {
	if !validName(name) {
		return "", fmt.Errorf("invalid context name %q", name)
	}

	parent, err := parentDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(ctxDir(parent), name+".json"), nil
}

// KnownContexts is a list of known context
func KnownContexts() []string {
	configs := []string{}

	parent, err := parentDir()
	if err != nil {
		return configs
	}

	files, err := os.ReadDir(filepath.Join(parent, "nats", "context"))
	if err != nil {
		return configs
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		nfo, err := f.Info()
		if err != nil {
			continue
		}
		if nfo.Size() == 0 {
			continue
		}

		ext := filepath.Ext(f.Name())
		if ext != ".json" {
			continue
		}

		configs = append(configs, strings.TrimSuffix(f.Name(), ext))
	}

	sort.Strings(configs)

	return configs
}

// SelectedContext returns the name of the current selected context, empty when non is selected
func SelectedContext() string {
	return readCtxFromFile(selectedCtxFile)
}

// PreviousContext returns the name of the previous selected context, empty if it hasn't been selected before
func PreviousContext() string {
	return readCtxFromFile(previousCtxFile)
}

func readCtxFromFile(file string) string {
	parent, err := parentDir()
	if err != nil {
		return ""
	}

	currentFile := filepath.Join(parent, "nats", file)

	_, err = os.Stat(currentFile)
	if os.IsNotExist(err) {
		return ""
	}

	fc, err := os.ReadFile(currentFile)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(fc))
}

func knownContext(parent string, name string) bool {
	if !validName(name) {
		return false
	}

	_, err := os.Stat(filepath.Join(ctxDir(parent), name+".json"))
	return !os.IsNotExist(err)
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

// NATSOptions creates NATS client configuration based on the contents of the context
func (c *Context) NATSOptions(opts ...nats.Option) ([]nats.Option, error) {
	var nopts []nats.Option

	switch {
	case c.User() != "":
		nopts = append(nopts, nats.UserInfo(c.User(), c.Password()))
	case c.Creds() != "":
		if strings.HasPrefix(c.Creds(), "op://") {
			cmd := exec.Command("op", "read", c.Creds())
			out, err := cmd.Output()
			if err != nil {
				return nil, err
			}
			jwt, err := nkeys.ParseDecoratedJWT(out)
			if err != nil {
				return nil, err
			}
			kp, err := nkeys.ParseDecoratedNKey(out)
			if err != nil {
				return nil, err
			}
			wipeSlice(out)

			userCB := func() (string, error) {
				return jwt, nil
			}
			sigCB := func(nonce []byte) ([]byte, error) {
				return kp.Sign(nonce)
			}
			nopts = append(nopts, nats.UserJWT(userCB, sigCB))
		} else {
			nopts = append(nopts, nats.UserCredentials(expandHomedir(c.Creds())))
		}

	case c.NKey() != "":
		nko, err := nats.NkeyOptionFromSeed(expandHomedir(c.NKey()))
		if err != nil {
			return nil, err
		}

		nopts = append(nopts, nko)
	}

	if c.Token() != "" {
		nopts = append(nopts, nats.Token(expandHomedir(c.Token())))
	}

	if c.Certificate() != "" && c.Key() != "" {
		nopts = append(nopts, nats.ClientCert(expandHomedir(c.Certificate()), expandHomedir(c.Key())))
	}

	if c.CA() != "" {
		nopts = append(nopts, nats.RootCAs(expandHomedir(c.CA())))
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

func (c *Context) parseWinCertStoreType(t string) (certstore.StoreType, error) {
	storeTypeString := c.config.WinCertStoreType
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
		rootCAs, _ := x509.SystemCertPool()
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}

		certs, err := os.ReadFile(c.config.CA)
		if err != nil {
			return nil, err
		}

		if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
			return nil, fmt.Errorf("failed to append CA certificates from %s", c.config.CA)
		}

		tlsc.RootCAs = rootCAs
	}

	return []nats.Option{nats.Secure(tlsc)}, nil
}

func (c *Context) loadActiveContext() error {
	if c.path == "" {
		parent, err := parentDir()
		if err != nil {
			return err
		}

		// none given, lets try to find it via the fs
		if c.Name == "" {
			c.Name = SelectedContext()
			if c.Name == "" {
				return nil
			}
		}

		if !validName(c.Name) {
			return fmt.Errorf("invalid context name %s", c.Name)
		}

		if !knownContext(parent, c.Name) {
			return fmt.Errorf("unknown context %q", c.Name)
		}

		c.path = filepath.Join(parent, "nats", "context", c.Name+".json")
	}

	ctxContent, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}

	err = json.Unmarshal(ctxContent, c.config)
	if err != nil {
		return err
	}

	// performing environment variable expansion for the path of the cerds.
	c.config.Creds = os.ExpandEnv(c.config.Creds)

	if c.config.NSCLookup != "" {
		err := c.resolveNscLookup()
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Context) resolveNscLookup() error {
	if c.config.NSCLookup == "" {
		return nil
	}

	path, err := exec.LookPath("nsc")
	if err != nil {
		return fmt.Errorf("cannot find 'nsc' in user path")
	}

	cmd := exec.Command(path, "generate", "profile", c.config.NSCLookup)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nsc invoke failed: %s", string(out))
	}

	type nscCreds struct {
		UserCreds string `json:"user_creds"`
		Operator  struct {
			Service []string `json:"service"`
		} `json:"operator"`
	}

	var result nscCreds
	err = json.Unmarshal(out, &result)
	if err != nil {
		return fmt.Errorf("could not parse nsc output: %s", err)
	}

	if result.UserCreds != "" {
		c.config.nscCreds = result.UserCreds
	}

	if len(result.Operator.Service) > 0 {
		c.config.nscUrl = strings.Join(result.Operator.Service, ",")
	}

	return nil
}

func expandHomedir(path string) string {
	if path[0] != '~' {
		return path
	}

	usr, err := user.Current()
	if err != nil {
		return path
	}

	return strings.Replace(path, "~", usr.HomeDir, 1)
}

func validName(name string) bool {
	return name != "" && !strings.Contains(name, "..") && !strings.Contains(name, string(os.PathSeparator))
}

func numCreds(c *Context) int {
	i := 0
	creds := []string{
		c.config.User,
		c.config.Creds,
		c.config.NKey,
		c.config.NSCLookup,
	}

	for _, c := range creds {
		if c != "" {
			i++
		}
	}

	return i
}

func createTree(parent string) error {
	return os.MkdirAll(ctxDir(parent), 0700)
}

func ctxDir(parent string) string {
	return filepath.Join(parent, "nats", "context")
}

func UnSelectContext() error {
	currentCtx := SelectedContext()
	if currentCtx == "" {
		return nil
	}

	parent, err := parentDir()
	if err != nil {
		return err
	}

	err = setPreviousContext(parent, currentCtx)
	if err != nil {
		return err
	}

	return os.Remove(filepath.Join(parent, "nats", selectedCtxFile))
}

// SelectContext sets the given context to be the default, error if it does not exist
func SelectContext(name string) error {
	if !validName(name) {
		return fmt.Errorf("invalid context name %q", name)
	}

	if !IsKnown(name) {
		return fmt.Errorf("unknown context")
	}

	parent, err := parentDir()
	if err != nil {
		return err
	}

	err = createTree(parent)
	if err != nil {
		return err
	}

	currentCtx := SelectedContext()
	err = setPreviousContext(parent, currentCtx)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(parent, "nats", selectedCtxFile), []byte(name), 0600)
}

func setPreviousContext(parent string, name string) error {
	if name == "" {
		return nil
	}

	return os.WriteFile(filepath.Join(parent, "nats", previousCtxFile), []byte(name), 0600)
}

func (c *Context) MarshalJSON() ([]byte, error) {
	c.config.Name = c.Name
	return json.MarshalIndent(c.config, "", "  ")
}

func (c *Context) Validate() error {
	if !validName(c.Name) {
		return fmt.Errorf("invalid context name %q", c.Name)
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

// Save saves the current context to name
func (c *Context) Save(name string) error {
	if name != "" {
		c.Name = name
	}

	if err := c.Validate(); err != nil {
		return err
	}

	parent, err := parentDir()
	if err != nil {
		return err
	}

	ctxDir := filepath.Join(parent, "nats", "context")
	err = createTree(parent)
	if err != nil {
		return err
	}

	// make sure no disk representations change while supporting showing a name for external serializers
	c.config.Name = ""

	j, err := json.MarshalIndent(c.config, "", "  ")
	if err != nil {
		return err
	}

	c.path = filepath.Join(ctxDir, c.Name+".json")
	return os.WriteFile(c.path, j, 0600)
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

// NscURL is the url used to resolve credentials in nsc
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

// UserJWT retrieves the configured user jwt, empty if not set
func (c *Context) UserJWT() string {
	return c.config.UserJwt
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
}
