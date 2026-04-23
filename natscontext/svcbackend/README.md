# natscontext/svcbackend

Reference client and wire protocol for a NATS request/reply service that
stores `natscontext` payloads remotely. A `Client` constructed here is a
drop-in `natscontext.Backend` + `natscontext.Selector`, so a `Registry`
can't tell the difference between a remote service and the shipped
`FileBackend` or `MemoryBackend`.

Server implementations are operator-owned. This package ships the client,
the wire types, the JSON schema, and a conformance test binary; it does
**not** ship a production server.

## Documents

| File                                                         | Audience                                                   |
|--------------------------------------------------------------|------------------------------------------------------------|
| [`PROTOCOL.md`](PROTOCOL.md)                                 | Server implementers. Normative contract in RFC 2119 terms. |
| [`schema.v1.json`](schema.v1.json)                           | Everyone. JSON Schema for every envelope.                  |
| [`cmd/svcbackend-conformance/`](cmd/svcbackend-conformance/) | Implementers. Checklist runner for a live server.          |

## Client quickstart

```go
import (
    "context"

    "github.com/nats-io/nats.go"

    "github.com/nats-io/jsm.go/natscontext"
    "github.com/nats-io/jsm.go/natscontext/svcbackend"
)

nc, err := nats.Connect("nats://svcbackend.example.net:4222")
// ... handle err, defer nc.Close()

client, err := svcbackend.NewClient(nc)
// ... handle err, defer client.Close()

// Use directly:
payload, err := client.Load(context.Background(), "prod")

// Or compose with the natscontext Registry so every caller picks up
// the remote store transparently:
reg, err := natscontext.NewRegistry(client)
ctx, err := reg.Load(context.Background(), "prod")
```

`NewClient` accepts the usual option pattern:

- `WithSubjectPrefix(p)` when the server is deployed under a non-default
  subject tree. The default is `natscontext.v1`.
- `WithTimeout(d)` for the per-request deadline applied when the caller's
  `context.Context` has none. Default is 5s.
- `WithServerKey(pub)` pre-seeds the cached long-term xkey, skipping the
  initial `sys.xkey` round-trip. Intended for tests and air-gapped
  deployments; production code should let the client discover it.

## Wire protocol at a glance

| Subject                                 | Purpose                          |
|-----------------------------------------|----------------------------------|
| `natscontext.v1.sys.xkey`               | Long-term xkey discovery         |
| `natscontext.v1.ctx.load.<name>`        | Fetch a context (sealed reply)   |
| `natscontext.v1.ctx.save.<name>`        | Store a context (sealed request) |
| `natscontext.v1.ctx.delete.<name>`      | Remove a context                 |
| `natscontext.v1.ctx.list`               | List stored names                |
| `natscontext.v1.sel.get`                | Read current selection           |
| `natscontext.v1.sel.set.<name>`         | Set selection                    |
| `natscontext.v1.sel.clear`              | Clear selection                  |

Payload encryption uses NaCl box via `nkeys.CreateCurveKeys`: a fresh
client ephemeral per request and a persistent server long-term xkey.
`PROTOCOL.md §9` is the authoritative description.

## Threat model

The protocol defends against a **passive observer** with broker-side
read access. It does **not** defend against an active broker, against
compromise of the server's long-term xkey, or against traffic analysis.
Context names appear in subjects and list responses and are visible to
anyone with subscribe permission. See `PROTOCOL.md` for the full
statement.

## Conformance testing

`cmd/svcbackend-conformance/` is a standalone Go module that audits a
running server against the checklist in `PROTOCOL.md`:

```
go install github.com/nats-io/jsm.go/natscontext/svcbackend/cmd/svcbackend-conformance@latest

svcbackend-conformance --context prod
svcbackend-conformance --context prod --mode=ro
svcbackend-conformance --context prod --mode=no-sel
svcbackend-conformance --context prod --log-file /var/log/svcbackend.log
```

Exit code is non-zero on any FAIL; WARN and SKIP are informational.

## Package layout

```
svcbackend/
  PROTOCOL.md, schema.v1.json   — normative contract + schema
  wire.go, schemas.go            — request/response types, embedded schema
  crypto.go                      — seal/open helpers over nkeys xkeys
  errors.go                      — wire-code <-> sentinel mapping
  client.go                      — Client: Backend + Selector + io.Closer
  cmd/svcbackend-conformance/    — standalone conformance tester
  internal/testserver/           — reference server used by the contract
                                   tests; not public API
```
