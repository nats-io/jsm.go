# Backends and credential resolvers

This document describes how the `natscontext` package composes storage and credential resolution, and how to add a new implementation of either.

## Overview

The package has two orthogonal abstractions:

- A `Backend` owns the storage of context payloads as opaque bytes.
- A `CredentialResolver` owns the resolution of secret material referenced by URI.

A `Registry` composes one `Backend`, optionally a `Selector`, and one or more `CredentialResolver`s. Package-level helpers (`KnownContexts`, `SelectContext`, `New`, `Connect`, and so on) delegate to a lazily constructed default `Registry` backed by `NewDefaultFileBackend` and the stock resolvers.

A `Backend` trades raw payload bytes. A `CredentialResolver` is invoked later, at `Context.NATSOptions` time, to materialize secret material referenced by the context fields. The two concerns stay independent so a backend does not need to know what a credential looks like and a resolver does not need to know where the context was loaded from.

## Shipped implementations

| Name                     | Kind     | Description                                                                |
|--------------------------|----------|----------------------------------------------------------------------------|
| `FileBackend`            | Backend  | JSON files under `<root>/nats/context/<name>.json`, tracks selection       |
| `SingleFileBackend`      | Backend  | One context at a caller-supplied file path, used by `NewFromFile`          |
| `MemoryBackend`          | Backend  | Mutex-protected map, primarily for tests and embeds                        |
| `FileSelector`           | Selector | Tracks active/previous context on disk; composes with any Backend          |
| `file://` (and bare path)| Resolver | Reads bytes from a filesystem path                                         |
| `op://`                  | Resolver | Shells `op read` against the 1Password CLI                                 |
| `nsc://`                 | Resolver | Shells `nsc generate profile`, returns the bytes of the generated creds    |
| `env://NAME`             | Resolver | Returns the value of environment variable `NAME`                           |
| `data:;base64,<payload>` | Resolver | Base64-decodes an RFC 2397 inline payload                                  |

## Adding a backend

### The `Backend` interface

```go
type Backend interface {
    Load(ctx context.Context, name string) ([]byte, error)
    Save(ctx context.Context, name string, data []byte) error
    Delete(ctx context.Context, name string) error
    List(ctx context.Context) ([]string, error)
}
```

A backend that tracks the active context also implements `Selector`:

```go
type Selector interface {
    Selected(ctx context.Context) (string, error)
    SetSelected(ctx context.Context, name string) (previous string, err error)
}
```

Every method accepts a `context.Context` so remote and network-backed
implementations can honor caller deadlines and cancellation. Purely
local backends (file, memory) are free to ignore the context.

`Registry` opts into selection tracking via a runtime type assertion, so a backend that cannot persist selection state (read-only HTTP, sealed container volume) simply omits the interface.

### Responsibilities

A correct `Backend` implementation:

- Validates names with `ValidateName` before touching storage, returning `ErrInvalidName` for rejections.
- Returns `ErrNotFound` (wrapped with context is fine) when `Load` is called for a missing name.
- Treats `Delete` of a missing name as a no-op.
- Makes `Save` and (if implemented) `SetSelected` atomic with respect to concurrent readers: a concurrent `Load` or `Selected` call must observe either the previous state or the new one, never a partial write.
- Returns payload bytes byte-for-byte on `Load`; the backend does not inspect or mutate the JSON.

A backend does not perform credential resolution, does not parse the payload, and does not know about individual context fields.

### Error sentinels

| Sentinel           | Meaning                                                          |
|--------------------|------------------------------------------------------------------|
| `ErrNotFound`      | The requested name is not stored                                 |
| `ErrAlreadyExists` | A name-collision was rejected                                    |
| `ErrInvalidName`   | The name fails `ValidateName` or backend-specific rules          |
| `ErrActiveContext` | An operation was refused because it targeted the active context  |
| `ErrNoneSelected`  | `Selected` was called but no context is selected                 |
| `ErrReadOnly`      | A write was attempted against a read-only backend                |
| `ErrConflict`      | A concurrent modification was detected during an atomic swap     |

Wrap, do not replace: `fmt.Errorf("%w: %q", ErrNotFound, name)` remains `errors.Is`-compatible while adding context for operators.

### Name validation

`ValidateName` enforces the historical portable rule, kept deliberately loose so context names users already have on disk keep working after upgrade:

- non-empty,
- does not contain the `..` substring,
- does not contain `/` or `\`.

Whitespace, control characters, a single `.`, and the NATS subject wildcards `*` and `>` all pass at this layer. That was the behavior shipped before the `Backend` refactor; tightening the rule at this layer would strand existing on-disk contexts (e.g. `ngs.js`) and break unknown user setups in-the-wild. `FileBackend` uses this rule as-is — the filesystem handles the permitted characters fine.

Backends whose storage model cannot carry some of those characters layer their own rejection on top. The shipped `svcbackend` client rejects whitespace, control characters, and `.`, `*`, `>` before publishing because each of those would break the single-token NATS subject the wire protocol embeds the name in. Each such additional rejection MUST surface `ErrInvalidName` so callers can still use `errors.Is`.

Round-tripping a name that passes `ValidateName` between two backends never produces a surprise rejection unless one of them imposes extra backend-specific rules; see each backend's docs for any such extras.

### Atomicity

Atomic writes are a correctness requirement, not an optimization. A concurrent reader that observes a half-written file or a partially populated KV record corrupts downstream consumers. `FileBackend` uses write-temp-then-rename with a parent-directory `fsync`. A KV-backed backend should use revision-based compare-and-swap, retrying on `ErrConflict` up to a bounded number of times before surfacing the error.

### Skeleton

```go
type MyBackend struct {
    // whatever storage handle the backend needs
}

func (b *MyBackend) Load(ctx context.Context, name string) ([]byte, error) {
    err := ValidateName(name)
    if err != nil {
        return nil, err
    }
    // fetch bytes from storage, honoring ctx for cancellation
    // return nil, fmt.Errorf("%w: %q", ErrNotFound, name) on miss
}

func (b *MyBackend) Save(ctx context.Context, name string, data []byte) error {
    err := ValidateName(name)
    if err != nil {
        return err
    }
    // atomic write
}

func (b *MyBackend) Delete(ctx context.Context, name string) error {
    err := ValidateName(name)
    if err != nil {
        return err
    }
    // no-op on missing
}

func (b *MyBackend) List(ctx context.Context) ([]string, error) {
    // returns every stored name, sorted
}
```

### Load and save flow

```
Registry.Load(ctx, name) ->  Backend.Load(ctx, name)       bytes
                         ->  Context.unmarshalAndExpand    fields + NSCLookup
                         ->  backend.Path(name) if present set c.path
                         ->  configureNewContext(opts...)  apply caller options

Registry.Save(ctx, c)    ->  c.Validate()
                         ->  migrateNSCLookup(c.config)    rewrite legacy NSC field
                         ->  json.MarshalIndent            bytes
                         ->  Backend.Save(ctx, name, bytes)
```

A backend that also wants `Context.path` populated on load can expose an optional method:

```go
func (b *MyBackend) Path(name string) string
```

`Registry.Load` and `Registry.Save` probe for this via type assertion and record the returned path on the `Context`. `ContextPath`, the package-level helper, is intentionally filesystem-specific and is not part of `Registry`.

### Optional probes

`Backend` is deliberately small. Capabilities that only some backends have are exposed as **optional probes** — interfaces that `Registry` discovers via a type assertion on the backend. The rule for adding a probe:

- **Prefer a stdlib interface.** If the semantic fits an existing stdlib type, use it. A remote backend that manages a connection implements `io.Closer`; `Registry.Close` type-asserts and delegates. Callers already know what `io.Closer` means, so there is nothing new to document.
- **Only invent a custom probe when no stdlib idiom fits.** `Path(name string) string` is custom because "give me the on-disk path for this name" has no stdlib equivalent and is meaningless for non-filesystem backends.
- **Keep probes narrow.** One method, one purpose. Do not bundle unrelated capabilities behind a single probe interface.

Current probes:

| Probe                         | Purpose                                                 | Implemented by              |
|-------------------------------|---------------------------------------------------------|-----------------------------|
| `interface{ Path(string) string }` | Reveals the on-disk location of a stored context   | `FileBackend`, `SingleFileBackend` |
| `io.Closer`                   | Releases a backend's long-lived resources (conns, watchers) | Future remote backends      |

A third custom probe should come with an explicit note in this section and a justification for why a stdlib interface would not work.

### Composing backends

Because `Backend` trades opaque bytes, cross-cutting concerns — encryption at rest, caching, access logging — can be expressed as a backend that wraps another backend rather than as a concern every new backend reimplements.

An **encrypting wrapper backend** is the canonical example: it seals payloads on `Save`, opens them on `Load`, and passes `Delete` / `List` through unchanged.

```go
type encryptedBackend struct {
    inner Backend
    // key material, envelope format, etc.
}

func (e *encryptedBackend) Load(ctx context.Context, name string) ([]byte, error) {
    ct, err := e.inner.Load(ctx, name)
    if err != nil {
        return nil, err
    }
    return e.open(ct)
}

func (e *encryptedBackend) Save(ctx context.Context, name string, data []byte) error {
    ct, err := e.seal(data)
    if err != nil {
        return err
    }
    return e.inner.Save(ctx, name, ct)
}

// Delete / List pass through; implement Selector only if inner does,
// and delegate those calls as-is.
```

Paired with a remote `Backend` that only knows how to move bytes (S3, HTTP, Redis, a different KV) this gives secret-at-rest coverage without the remote backend growing its own envelope format. A new remote backend should reach for the wrapper before reimplementing encryption. The shipped `natscontextkv` backend (see `spec/KV.md`) is the first to follow this pattern; later remote backends are expected to compose with the same wrapper rather than invent a second envelope.

### Separating selection from storage

A `Backend` stores contexts; a `Selector` tracks which one is active. By default `Registry` picks up a `Selector` via a type assertion on the `Backend`, which is the right behavior when storage is single-user (for example `FileBackend` on a developer's laptop).

When the `Backend` is shared across users or machines — the `svcbackend` client talking to a service multiple operators connect to, a KV bucket replicated across a team — the "active context" is still a personal, per-machine choice. Storing it centrally leaks one operator's preference to every other operator (or requires per-user partitioning on the server). The composition for that case is "shared Backend, local Selector":

```go
sel, err := natscontext.NewDefaultFileSelector()
if err != nil {
    // fail loudly — see the XDG caveat below
}
reg := natscontext.NewRegistry(
    svcClient,
    natscontext.WithSelector(sel),
)
```

`natscontext.WithLocalSelector()` is sugar for the same composition when the default `$XDG_CONFIG_HOME/nats/` (falling back to `$HOME/.config/nats/`) location is acceptable. It panics if neither is resolvable; on CI or container environments where that may happen, construct a `FileSelector` explicitly with `NewFileSelectorAt(dir)` and a known-writable directory.

`WithoutSelection()` explicitly disables selection tracking even when the `Backend` implements `Selector`, for read-only pipelines or tools that always load by explicit name.

Caveats the composition inherits and the Registry does not fully hide:

- **Stale local selection.** Machine A deletes a context that machine B's local selector still names. When B next calls `Registry.Load("")`, the stale-selection guard clears the local selector and returns an empty `Context` — no error, but the next command that relied on a selection now starts from scratch. `Registry.Delete` cannot protect the remote catalog across machines; active-context refusal only covers the local selector.
- **Local `previous-context.txt`.** The `previous` pointer is also local. `nats context previous` can reference a name the shared backend no longer has; surface those as `ErrNotFound` to the user.
- **Inverse composition is undefined.** "Local `FileBackend` + remote `Selector`" (selection persisted to a shared store, contexts kept on one machine) is nonsense and not supported. A remote `Selector` naming a context the local backend does not have produces an `ErrNotFound` that no guard can meaningfully recover from.

### Contract tests

A shared contract-conformance suite lives in `natscontext/backendtest`. New backends should call it from a `_test.go` file:

```go
import (
    "testing"

    "github.com/nats-io/jsm.go/natscontext"
    "github.com/nats-io/jsm.go/natscontext/backendtest"
)

func TestMyBackendContract(t *testing.T) {
    backendtest.RunBackendContract(t, func(t *testing.T) natscontext.Backend {
        return newMyBackendForTest(t)
    })
}
```

The factory returns a fresh backend per subtest so state does not leak between assertions. `Selector` subtests run automatically when the returned backend also satisfies the `Selector` interface.

Backends with a narrower surface than the shared contract (for example `SingleFileBackend`, which accepts only one name) should provide a targeted test rather than force-fit the shared suite.

> [!info] Note
> Third-party backends in other repositories are encouraged to vendor the contract suite into their tests the same way. The suite is the canonical specification of backend behavior.

## Adding a credential resolver

### The `CredentialResolver` interface

```go
type CredentialResolver interface {
    Schemes() []string
    Resolve(ctx context.Context, ref string) ([]byte, error)
}
```

A resolver returns the raw bytes referenced by `ref`. The scheme dispatch happens inside `Context.NATSOptions` based on the URI scheme of the context field value. A single resolver may claim multiple schemes; returning `""` in `Schemes` marks the resolver as the fallback for bare, unschemed values.

### Responsibilities

A correct resolver:

- Returns a freshly-allocated slice on every call. The caller wipes resolver output once the derived `nats.Option` is constructed.
- Never logs the resolved bytes.
- Never includes resolved bytes in returned errors. The `nsc` resolver is the cautionary tale: its previous implementation echoed `cmd.CombinedOutput()` on failure, which could carry decorated JWTs.
- Accepts cancellation via the supplied `context.Context`. Resolvers that shell out use `exec.CommandContext`.

### Field semantics

Context fields that go through resolvers fall into two shapes:

| Field                                   | Shape        | Bare value treated as |
|-----------------------------------------|--------------|-----------------------|
| `Creds`, `NKey`                         | URI          | A `file://` path      |
| `UserJwt`, `UserSeed`, `Token`, `Password` | URI or literal | The literal value itself |

`Cert`, `Key`, and `CA` are currently file-path only. Non-`file:` schemes for these fields are rejected with a clear error until nats.go exposes in-memory TLS equivalents.

Scheme detection is `net/url`-based with a two-character minimum on the scheme, so Windows drive letters like `C:\foo` do not masquerade as URIs. Schemes are matched case-insensitively per RFC 3986, so `FILE:///tmp/x`, `ENV://NAME`, and similar variants reach the same resolvers as their lowercase forms.

### Registration

Construct a `Registry` with a resolver:

```go
reg := natscontext.NewRegistry(
    natscontext.NewFileBackendAt(root),
    natscontext.WithDefaultResolvers(),
    natscontext.WithCredentialResolver(myResolver),
)
```

`NewRegistry` applies the stock resolvers automatically when no resolver-related option is supplied. Passing any resolver option disables the implicit defaults; combine with `WithDefaultResolvers` to layer a custom resolver on top of the stock set.

### Secret hygiene

Three patterns keep secret material scoped:

- **Resolver output is wiped by the caller.** `buildCredsOption`, `buildNkeyOption`, and `resolveLiteral` all invoke `wipeSlice` on the returned bytes once the option is built. A resolver that caches bytes internally must hand out copies so the caller's wipe does not corrupt the cache.
- **Error messages never echo resolved payload.** The `nsc` resolver now captures stderr separately and returns only the exit code and stderr hint; stdout, which may carry a decorated JWT, is never quoted.
- **Context formatting is redacted.** `*Context` implements `String` and `GoString`, and `*settings` has a `<redacted>` `String` as defense-in-depth against reflection-based dumpers. `%v`, `%+v`, `%s`, and `%#v` on a `Context` all route through the redactor.

> [!info] Warning
> A `Backend` that stores context payloads containing inline credentials inherits the secret-at-rest protection of the backend's storage. Storing URI references with a separate encrypted credential store is strictly safer and is the recommended pattern for KV-backed deployments.

### Skeleton

```go
type myResolver struct{}

func (r *myResolver) Schemes() []string {
    return []string{"myscheme"}
}

func (r *myResolver) Resolve(ctx context.Context, ref string) ([]byte, error) {
    // fetch bytes referenced by ref
    // return fresh slice the caller may clobber
}
```

Registering the resolver with a `Registry` is enough to make every field that routes through scheme dispatch accept `myscheme://...` references.

### Resolution flow

```
Context.NATSOptions()
  -> Creds/NKey field     -> parseScheme       -> resolver.Resolve -> bytes -> nats.Option
  -> UserJwt/Seed/Token   -> parseScheme       -> resolver.Resolve -> string -> nats.Option
                          -> no known scheme   -> value used as literal
```

`Creds` with `file://` or a bare path does not invoke a resolver: the path is handed directly to `nats.UserCredentials` so the file is read lazily at dial time. Non-`file:` schemes read the bytes eagerly and configure `nats.UserJWT` with callbacks, since nats.go has no path-based equivalent for those cases.