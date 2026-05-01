# natscontext svcbackend Protocol v1

This document is the contract for server implementers. A conformant server MUST satisfy every requirement marked MUST; SHOULD requirements are strongly recommended but not mandatory. See [wire.go](wire.go) for the Go struct definitions.

## 1. Scope

This document specifies the wire protocol, crypto envelope, and behavioral requirements that any conforming `natscontext` service-backend server MUST implement. The reference client lives in this repository; any server implementation in any language that satisfies this contract is conforming.

## 2. Terminology

Key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in [RFC 2119](https://datatracker.ietf.org/doc/html/rfc2119).

- **Envelope**: the outer JSON document carried as a request payload or reply body on a given subject. Envelope types are defined in [schema.v1.json](schema.v1.json).
- **Sealed payload**: the plaintext JSON document (`LoadSealed`, `SaveSealed`) that the sender serializes, seals with nkeys xkeys, and transports inside the `sealed` field of its envelope.
- **Ephemeral xkey**: a freshly generated curve keypair used for a single request. Clients generate an ephemeral xkey per `ctx.load` and per `ctx.save`; the public half travels in the envelope.
- **Long-term xkey**: the server's persistent curve keypair. Its public half is advertised by `natscontext.v1.sys.xkey` and cached by clients for the lifetime of their `Client`.
- **Conforming server**: a server that satisfies every MUST in this document.

## 3. Threat model

The protocol defends against a **passive observer** with broker-side read access — an adversary who can `nats sub '>'` against the account carrying service traffic, read broker logs, or scrape JetStream archives and packet captures.

The protocol does **NOT** defend against:

- Active broker MITM (envelope substitution, ciphertext replay, subject rewriting).
- Compromise of the server's long-term xkey private key; past `ctx.save` ciphertexts archived by a broker become decryptable.
- Traffic analysis: subject patterns, request counts, timing, payload size.
- Context **name** confidentiality: names appear in subjects and `list` responses and are visible to anyone with subscribe permission.
- Authorization bypass at the NATS layer.

Operators who need the "does-not-defend" properties rely on NATS account isolation, subject-level ACLs, and deployment choices outside this protocol's scope.

## 4. Subjects

A conforming server MUST expose the following subjects. The subject prefix `natscontext.v1` is the default; an operator MAY configure a different prefix, but `v1` MUST remain in the subject tree so a future `v2` is a parallel tree.

| Subject                            | Request                | Response                | Notes                         |
|------------------------------------|------------------------|-------------------------|-------------------------------|
| `natscontext.v1.sys.xkey`          | `SysXKeyRequest`       | `SysXKeyResponse`       | Long-term xkey discovery.     |
| `natscontext.v1.ctx.load.<name>`   | `LoadRequest`          | `LoadResponse`          | `<name>` is the context name. |
| `natscontext.v1.ctx.save.<name>`   | `SaveRequest`          | `SaveResponse`          | Sealed request.               |
| `natscontext.v1.ctx.delete.<name>` | `DeleteRequest`        | `DeleteResponse`        |                               |
| `natscontext.v1.ctx.list`          | `ListRequest`          | `ListResponse`          |                               |

A conforming server SHOULD register every endpoint under a single queue group so replicas load-balance uniformly. The default queue group name is `natscontext`. Operators MAY override the queue group.

Selection (which context is "active") is deliberately **not** part of this protocol. A service backend typically carries a catalog shared by many operators; the active-context choice is personal and per-machine, and storing it centrally either leaks it between operators or requires per-user partitioning the protocol does not specify. A conforming client pairs this backend with a local `Selector` at `Registry` composition time (for example `natscontext.WithLocalSelector`) rather than looking for `sel.*` subjects.

## 5. Envelopes

Every envelope is a JSON object. [schema.v1.json](schema.v1.json) is the normative source for field presence, types, and required-field sets. Go struct bindings live in [wire.go](wire.go). The examples below are illustrative.

### 5.1 `sys.xkey`

Request:

```json
{}
```

Response (success):

```json
{ "xkey_pub": "XAAAA..." }
```

Response (error):

```json
{ "error": { "code": "internal", "message": "..." } }
```

- The server MUST return an `xkey_pub` that encodes a curve public key (nkeys `PrefixByteCurve`).
- The returned key MUST be usable for `Open` against ciphertexts produced by a client that sealed to it, until rotation.

### 5.2 `ctx.load`

Request:

```json
{ "reply_pub": "XAAAA...", "req_id": "0f1e2d3c4b5a69788796a5b4c3d2e1f0" }
```

- `reply_pub` is the client's per-request ephemeral xkey public key. The server seals `LoadSealed` to it.
- `req_id` is a 16-byte random token the client echoes back inside `LoadSealed` for response correlation.

Response (success):

```json
{ "sealed": "<base64 xkv1 ciphertext of LoadSealed>" }
```

Response (error):

```json
{ "error": { "code": "not_found", "message": "..." } }
```

Sealed plaintext (`LoadSealed`):

```json
{ "data": "<base64 of raw context bytes>", "req_id": "<echoed>" }
```

- The server MUST seal `LoadSealed` with its long-term xkey private as sender and `reply_pub` as recipient.
- The server MUST include the request `req_id` verbatim in the sealed `req_id` field.
- The server MUST NOT include context payload bytes in any plaintext envelope or `error.message`.

### 5.3 `ctx.save`

Request:

```json
{ "sender_pub": "XAAAA...", "sealed": "<base64 xkv1 ciphertext of SaveSealed>" }
```

Response (success):

```json
{}
```

Response (error):

```json
{ "error": { "code": "invalid_name", "message": "..." } }
```

Sealed plaintext (`SaveSealed`):

```json
{ "data": "<base64 of raw context bytes>", "req_id": "0f1e..." }
```

- The server MUST open `sealed` with its long-term xkey private as recipient and `sender_pub` as sender.
- The server MAY track `req_id` in a replay window (§9.3).
- The server MUST NOT log or echo the plaintext `data`, ever.

### 5.4 `ctx.delete`

Request:

```json
{ "req_id": "0f1e..." }
```

Response:

```json
{}
```

- Deleting a name that does not exist MUST succeed with an empty response (no error).
- Invalid names MUST be rejected with `invalid_name` before any storage touch.

### 5.5 `ctx.list`

Request:

```json
{}
```

Response:

```json
{ "names": ["admin", "dev", "prod"] }
```

- `names` MUST be sorted ascending (lexicographic byte order).
- A server with zero stored contexts MUST return a response with `names` absent or set to an empty array.

## 6. Error codes

Every response type carries an optional `error` field of the `Error` shape:

```json
{ "code": "<code>", "message": "<human-readable>" }
```

| Wire code        | `natscontext` sentinel                     |
|------------------|--------------------------------------------|
| `not_found`      | `natscontext.ErrNotFound`                  |
| `already_exists` | `natscontext.ErrAlreadyExists`             |
| `invalid_name`   | `natscontext.ErrInvalidName`               |
| `active_context` | `natscontext.ErrActiveContext`             |
| `read_only`      | `natscontext.ErrReadOnly`                  |
| `conflict`       | `natscontext.ErrConflict`                  |
| `stale_key`      | no sentinel — client-internal retry signal |
| `internal`       | generic unwrapped error                    |

- `Error.message` MUST NOT include payload bytes.
- `Error.message` SHOULD be short and human-readable; clients may surface it verbatim.
- Successful responses MUST omit the `error` field (or set it to JSON null).

## 7. Name validation

The core `natscontext` layer enforces a deliberately loose rule (historical backward-compat): a name is valid iff it is non-empty, does not contain the `..` substring, and does not contain `/` or `\`. Whitespace, control characters, `.`, `*`, and `>` all pass at that layer.

The svcbackend wire protocol embeds the context name as the final token of a NATS subject (`natscontext.v1.ctx.<verb>.<name>`), which is strictly tighter than that. A conforming **client** therefore MUST reject, with `invalid_name`, before publishing, any name that would break subject encoding:

- The name MUST NOT be empty.
- The name MUST NOT contain the `..` substring.
- The name MUST NOT contain `/` or `\`.
- The name MUST NOT contain whitespace (including tab, CR, LF) or control characters.
- The name MUST NOT contain `.` (token separator), `*`, or `>` (subject wildcards).

The server does NOT need to enforce the extra svcbackend-tier rules. A dotted or wildcarded name produces a subject the server's single-token wildcard subscription never matches, so the request is simply not delivered. A conforming server MUST still validate names it does receive against the core rule (non-empty, no `..`, no `/` or `\`) before any storage touch, credential decryption, or long-running work, and MUST return `invalid_name` on failure.

A server MAY reject additional names for storage-specific reasons (e.g. path-traversal hazards on disk backends); such rejections MUST also use `invalid_name`.

## 8. Behavioral requirements

### 8.1 `ctx.load`

- MUST return `not_found` for missing names.
- MUST NOT leak payload bytes in `error.message`.

### 8.2 `ctx.save`

- MUST be atomic with respect to concurrent `ctx.load` against the same name: a concurrent reader MUST see either the prior payload or the newly-written payload, never a partial write.
- MUST overwrite existing values for the same name (no `already_exists` on plain save).

### 8.3 `ctx.delete`

- MUST treat deleting a missing name as a no-op with a success response.

### 8.4 `ctx.list`

- MUST return names in ascending lexicographic byte order.
- MAY return an empty `names` field when the store is empty.

### 8.5 Read-only servers

- Servers that do not allow mutation MUST return `read_only` on `ctx.save` and `ctx.delete`.

## 9. Crypto

The wire crypto uses NaCl box via the `nkeys` xkeys API: Curve25519 key agreement + XSalsa20-Poly1305 AEAD with a 24-byte random nonce. The primitive and its wire layout are defined in [nkeys/xkeys.go](https://github.com/nats-io/nkeys/blob/main/xkeys.go).

### 9.1 Long-term xkey

A conforming server MUST generate a curve keypair via `nkeys.CreateCurveKeys()` (or equivalent) and keep the private half in-process. The public half is advertised by `sys.xkey`.

### 9.2 Sealing and opening

- `ctx.load` (sealed response): the server MUST seal `LoadSealed` using its long-term xkey private key as sender and `LoadRequest.reply_pub` as recipient. The client opens with its ephemeral private key and the cached server public.
- `ctx.save` (sealed request): the server MUST open `SaveRequest.sealed` using its long-term xkey private key as recipient and `SaveRequest.sender_pub` as sender.
- The `sealed` field is the base64 (standard, padded) encoding of the raw `xkv1` ciphertext blob produced by `nkeys.KeyPair.Seal`.

### 9.3 `req_id` dedupe (optional)

A server MAY maintain a sliding window of recently-seen `req_id` values and reject replays with `conflict`. If implemented, the window SHOULD be at least 5 minutes.

### 9.4 Rotation

- A server MAY rotate its long-term xkey at any time.
- A server SHOULD accept its previous long-term xkey for a brief overlap window (a few minutes) to reduce client-visible retries.
- Clients that fail `Open` MUST invalidate their cached server key, refetch via `sys.xkey` once, and retry the original call once. Rotation therefore requires no operator coordination.
- A server MUST return `stale_key` when it fails to `Open` a sealed request (e.g. `ctx.save` sealed to a prior long-term xkey). The `stale_key` code MUST NOT be used for non-rotation failures such as malformed base64 or an unparsable plaintext; those MUST return `internal`.
- Clients MUST treat `stale_key` the same as a local `Open` failure: invalidate the cached server key, refetch once, and retry once with a freshly generated ephemeral keypair and `req_id`. A second `stale_key` reply MUST surface as an error rather than loop.
- Servers predating this revision return `internal` on server-side `Open` failure. A conforming client cannot distinguish that from a genuine internal error, so transparent rotation is only guaranteed against conforming servers.

## 10. Logging and error hygiene

- A server MUST NOT log payload bytes (`LoadSealed.Data`, `SaveSealed.Data`) or context names derived from sealed plaintext at levels above DEBUG, and MUST NOT log them at any level to destinations outside the operator's trust boundary.
- A server MUST NOT echo payload bytes in `error.message`.
- A server SHOULD log `req_id` (when present) to aid correlation across client, server, and broker logs.

## 11. Versioning

- The protocol version is encoded in the subject tree (`natscontext.v1.*`).
- A future v2 is a parallel subject tree with its own schema document.
- A server MAY register both v1 and v2 concurrently.

## 12. Conformance checklist

A conforming server MUST satisfy every item below. The
[`svcbackend-conformance`](cmd/svcbackend-conformance/) command in this
repository exercises the wire-testable portions of this checklist against
a running server; implementers can use it as an automated regression gate.
Items the tester cannot verify from a client position (log hygiene, storage
pre-validation ordering) are flagged in its output as `SKIP` or `WARN`.

### Subjects

- [ ] `natscontext.v1.sys.xkey` is registered.
- [ ] `natscontext.v1.ctx.load.<name>` is registered.
- [ ] `natscontext.v1.ctx.save.<name>` is registered.
- [ ] `natscontext.v1.ctx.delete.<name>` is registered.
- [ ] `natscontext.v1.ctx.list` is registered.

### Envelopes

- [ ] Every response sets `error` to nil/absent on success.
- [ ] Every `error.message` is free of payload bytes.
- [ ] `ctx.list` names are sorted ascending.

### Name validation

- [ ] Names are validated before any storage touch.
- [ ] Empty names are rejected with `invalid_name`.
- [ ] Names containing `..`, `/`, or `\` are rejected with `invalid_name`.
- [ ] (Client-side) names containing whitespace, control characters, `.`, `*`, or `>` are rejected with `invalid_name` before publishing.

### Behavior

- [ ] `ctx.load` of a missing name returns `not_found`.
- [ ] `ctx.delete` of a missing name returns success with no error.
- [ ] `ctx.save` is atomic vs concurrent `ctx.load`.
- [ ] Immutable servers return `read_only` from `ctx.save` and `ctx.delete`.

### Crypto

- [ ] Server generates and persists a long-term xkey.
- [ ] `sys.xkey` advertises the current long-term xkey public key.
- [ ] `ctx.load` responses are sealed with `(server_priv, reply_pub)`.
- [ ] `ctx.save` requests are opened with `(server_priv, sender_pub)`.
- [ ] The `sealed` field is base64-encoded `xkv1` ciphertext.
- [ ] `ctx.save` with a sealed request sealed to a prior long-term xkey returns `stale_key`, not `internal`.

### Logging

- [ ] Payload bytes never appear in logs above DEBUG.
- [ ] Payload bytes never appear in `error.message`.
- [ ] `req_id` is logged when present (recommended).
