# Audit Archive Format Specification

## Overview

An audit archive is a ZIP file that captures a point-in-time snapshot of a NATS deployment.
It stores monitoring endpoint responses, stream state, server profiles, and operational metadata
gathered from one or more servers.

The format is designed to be:

- **Single-file** — easy to share, attach, and store.
- **Compressed** — ZIP with Deflate; JSON has significant redundancy.
- **Indexed** — a manifest maps every file in the archive to its tags, enabling tag-based queries
  without parsing file paths.
- **Human-readable** — all artifact content is pretty-printed JSON (except raw binary profiles).

The `archive` package provides `Writer` and `Reader` types that enforce this format.
Consumers **must** use those types; they **must not** rely on the path structure for anything
other than manual inspection.

---

## Container Format

The archive is a standard ZIP file (PKWARE ZIP, as implemented by Go's `archive/zip`).

- All entries use **Deflate** compression (method 8).
- All entry timestamps are set to the **capture time** supplied to `Writer.SetTime`.
  If no time is set, the current wall clock time (UTC) is used. Timestamps are always stored in UTC.
- The file extension is conventionally `.zip`.

---

## Directory Layout

All paths inside the ZIP are rooted under `capture/`. Nothing is written at the top level, so
extracting the archive does not pollute the containing directory.

```
capture/
  clusters/
    <cluster>/
      <server>/
        <artifact_type>/
          0001.json
          0002.json
          ...
  accounts/
    <account>/
      servers/
        <cluster>__<server>/
          <artifact_type>/
            0001.json
            ...
      streams/
        <stream>/
          replicas/
            <cluster>__<server>/
              <artifact_type>/
                0001.json
                ...
  profiles/
    <cluster>/
      <server>__<profile_name>.<ext>
  misc/
    manifest.json
    <name>.<ext>
    ...
```

Where `<cluster>__<server>` is the cluster name and server name joined by a double-underscore
(`__`) separator.

If a server has no cluster, the literal string `unclustered` is used in place of the cluster name.

### Path Component Constraints

Every value placed into a path (server name, cluster name, account name, stream name, artifact
type, profile name, file extension, and special file name) **must** satisfy all of the following:

1. Must pass `filepath.IsLocal` — i.e., must not be empty, must not be absolute, and must not
   contain or resolve to a `..` traversal.
2. Must not contain path separators (`/` or `\`).

The `Writer` enforces these constraints when an artifact is added; an error is returned if any
value violates them.

---

## Artifact Storage: Paged vs. Non-Paged

### Paged artifacts (default)

Most artifacts are stored in **paged** format.  Each call to `Writer.Add` or `Writer.AddRaw` for
a given tag combination appends a new page under a shared directory.

Pages are numbered from `0001` and formatted as four zero-padded decimal digits:

```
<artifact_dir>/0001.json
<artifact_dir>/0002.json
...
<artifact_dir>/9999.json
```

Each page file contains exactly one JSON object — the serialized form of one artifact value.
Pages for the same tag combination are ordered by page number.

`ForEachTaggedArtifact` reads pages in ascending name order (i.e., `0001.json` before `0002.json`)
and will only process files whose base name is exactly 9 characters long and ends in `.json`
(matching the pattern `NNNN.json` with `N` being a decimal digit).  This means a maximum of
**9 999 pages** per tag combination is supported.

### Non-paged artifacts

Two categories of artifact bypass paging and are written as a single flat file:

| Category | Condition | Example path |
|---|---|---|
| **Profile** | `artifact_type` tag value is `profile` | `capture/profiles/C1/s1__heap.prof` |
| **Special** | Tagged with the `special` tag | `capture/misc/manifest.json` |

Non-paged files appear in the manifest with their full path (including extension) as the key,
rather than a page-numbered path.

---

## Tags

Every artifact is associated with a set of **tags** at write time.  Tags are key-value pairs.
The key is called the *label*; the value is a plain string.

Tags serve two purposes:

1. **Path construction** — dimension tags are combined to derive the file path in the ZIP.
2. **Indexing** — the manifest records all tags for each file, allowing the `Reader` to answer
   queries such as "all stream info artifacts for account A in cluster C".

### Dimension Tags

Dimension tags directly determine file placement.  At most one value per label is allowed per
artifact.

| Label | Constant | Description |
|---|---|---|
| `server` | `serverTagLabel` | Name of the server the artifact was captured from |
| `cluster` | `clusterTagLabel` | Name of the cluster the server belongs to |
| `account` | `accountTagLabel` | NATS account identifier |
| `stream` | `streamTagLabel` | JetStream stream name |
| `artifact_type` | `typeTagLabel` | Artifact type name (see below) |
| `profile_name` | `profileNameTagLabel` | Identifies which profile (e.g., `heap`, `cpu`) |

Every non-special artifact **must** supply `server`, `cluster`, and `artifact_type` tags.
The `cluster` tag may have the value `unclustered` (via `TagNoCluster()`) if the server is
not part of a cluster.

### The Special Tag

The `special` tag is mutually exclusive with all dimension tags.  An artifact tagged only with
`special` is written to `capture/misc/<value>.<ext>` as a single non-paged file.  It is an
error to combine `special` with any other tag.

### Required Tag Combinations by Artifact Family

| Family | Required tags | Optional tags |
|---|---|---|
| Server artifact | `server`, `cluster`, `artifact_type` | — |
| Server profile | `server`, `cluster`, `artifact_type` (= `profile`), `profile_name` | — |
| Account artifact | `server`, `cluster`, `account`, `artifact_type` | — |
| Stream artifact | `server`, `cluster`, `account`, `stream`, `artifact_type` | — |
| Special file | `special` | — |

---

## Artifact Types

### Server artifacts

Captured per server.  Path: `capture/clusters/<cluster>/<server>/<type>/NNNN.json`.

| Type value | Source endpoint | Tag constructor |
|---|---|---|
| `health` | `HEALTHZ` | `TagServerHealth()` |
| `variables` | `VARZ` | `TagServerVars()` |
| `exp_variables` | `EXPVARZ` | `TagServerExpVars()` |
| `connections` | `CONNZ` | `TagServerConnections()` |
| `routes` | `ROUTEZ` | `TagServerRoutes()` |
| `gateways` | `GATEWAYZ` | `TagServerGateways()` |
| `leafs` | `LEAFZ` | `TagServerLeafs()` |
| `subs` | `SUBSZ` | `TagServerSubs()` |
| `jetstream_info` | `JSZ` | `TagServerJetStream()` |
| `accounts` | `ACCOUNTZ` | `TagServerAccounts()` |

### Account artifacts

Captured per account per server.  Path:
`capture/accounts/<account>/servers/<cluster>__<server>/<type>/NNNN.json`.

| Type value | Source endpoint | Tag constructor |
|---|---|---|
| `account_connections` | `CONNZ` (account) | `TagAccountConnections()` |
| `account_leafs` | `LEAFZ` (account) | `TagAccountLeafs()` |
| `account_subs` | `SUBSZ` (account) | `TagAccountSubs()` |
| `account_jetstream_info` | `JSZ` (account) | `TagAccountJetStream()` |
| `account_info` | `INFO` (account) | `TagAccountInfo()` |
| `account_raftz` | `RAFTZ` | `TagAccountRaftz()` |
| `account_ipqueues` | `IPQUEUESZ` | `TagAccountIpqueuesz()` |

### Stream artifacts

Captured per stream replica.  Path:
`capture/accounts/<account>/streams/<stream>/replicas/<cluster>__<server>/<type>/NNNN.json`.

| Type value | Description | Tag constructor |
|---|---|---|
| `stream_info` | Full stream configuration and state | `TagStreamInfo()` |

### Server profile artifacts (non-paged)

Captured per server per profile type.
Path: `capture/profiles/<cluster>/<server>__<profile_name>.<ext>`.

The `artifact_type` tag value is always `profile`.  The `profile_name` tag distinguishes
individual profiles.  The file extension is typically `prof` (pprof binary format).

| Profile name | Description |
|---|---|
| `goroutine` | Goroutine stack traces (captured at debug levels 1 and 2) |
| `goroutine_1` | Goroutine traces with aggregated tags |
| `goroutine_2` | Full per-goroutine stacks |
| `heap` | Heap memory profile |
| `allocs` | Memory allocation profile |
| `mutex` | Mutex contention profile |
| `threadcreate` | Thread creation profile |
| `block` | Blocking profile |
| `cpu` | CPU profile |

### Special files (non-paged)

Stored under `capture/misc/`.

| Name | Path | Description |
|---|---|---|
| `manifest` | `capture/misc/manifest.json` | Archive index (written by `Writer.Close`) |
| `audit_gather_metadata` | `capture/misc/audit_gather_metadata.json` | Capture run metadata |
| `audit_gather_log` | `capture/misc/audit_gather_log.log` | Log output from the capture run |

---

## Manifest

`capture/misc/manifest.json` is the archive index.  It is written automatically by
`Writer.Close` as the last entry in the ZIP; it **must not** be added manually.

### Structure

The manifest is a JSON object whose keys are ZIP entry paths (relative to the archive root) and
whose values are arrays of tag objects.

```json
{
  "capture/clusters/C1/s1/variables/0001.json": [
    {"Name": "cluster",       "Value": "C1"},
    {"Name": "server",        "Value": "s1"},
    {"Name": "artifact_type", "Value": "variables"}
  ],
  "capture/misc/audit_gather_metadata.json": [
    {"Name": "special", "Value": "audit_gather_metadata"}
  ]
}
```

The manifest itself does **not** have an entry in the manifest.

### Integrity Checks on Open

When `NewReader` opens an archive it performs two checks:

1. **Manifest → ZIP**: every path in the manifest must exist as a ZIP entry.
   A missing file causes `NewReader` to return an error.
2. **ZIP → manifest**: every ZIP entry (except the manifest itself) should have a manifest entry.
   A ZIP entry without a manifest entry is non-fatal; it is recorded in `Reader.Warnings`.

---

## Capture Metadata

`capture/misc/audit_gather_metadata.json` contains a JSON object with the following fields:

| JSON key | Type | Description |
|---|---|---|
| `capture_timestamp` | RFC 3339 timestamp | When the capture was started (UTC) |
| `connected_server_name` | string | Name of the NATS server the capture tool connected to |
| `connected_server_version` | string | Version of that server |
| `connect_url` | string | Connection URL with credentials redacted |
| `user_name` | string | OS username of the operator who ran the capture |
| `version` | string | VCS revision of the capture tool |

---

## Reader Index

`NewReader` builds three in-memory indices from the manifest at open time:

| Index | Key | Value |
|---|---|---|
| Inverted index | `Tag` | sorted list of ZIP paths that carry that tag |
| Cluster → servers | cluster name | sorted list of server names in that cluster |
| Account → streams | account name | sorted list of stream names in that account |
| Account/stream → servers | `account/stream` | sorted list of server names holding replicas |

All lists are sorted alphabetically.  The indices are used by `Load`, `ForEachTaggedArtifact`,
and `EachClusterServerArtifact` to answer queries without scanning the ZIP directory.

### Querying

- **`Load(v, tags...)`** — resolves to exactly one file. Returns `ErrNoMatches` or
  `ErrMultipleMatches` if the tag intersection contains zero or more than one file respectively.
- **`ForEachTaggedArtifact[T](r, tags, cb)`** — iterates all paged files matching the given tags,
  decoding each page into a `T` and calling `cb`. Files are visited in ascending page-number order.
  Returns `ErrNoMatches` if no pages exist for the tag combination.
- **`EachClusterServerArtifact[T](r, artifactTag, cb)`** — iterates every (cluster, server) pair
  known to the archive and calls `ForEachTaggedArtifact` for each, passing the combined tags.
  If a server has no matching artifact, `cb` is called once with a nil artifact and
  `ErrNoMatches` so the caller can record the gap.