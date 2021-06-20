# Proposed design for JetStream based KV services

This is a proposal to design KV services ontop of JetStream, this requires a KV client to be written, the basic
features are:

 * Multiple named buckets full of keys with n historical values kept per value
 * Put and Get `string(k)=string(v)` values
 * Deleting a key completely
 * Per key TTL
 * Compacting a key down to n remaining values
 * Watching a specific key or the entire bucket for live updates
 * Encoders and Decoders that transforms both keys and values
 * A read-cache that builds an in-memory cache for fast reads
 * Ability to read from regional read replicas maintained by JS Mirroring (planned)
 * read-after-write safety unless the read cache of read replicas were used
 * Valid keys are `\A[-/_a-zA-Z0-9]+\z` after encoding
 * Valid buckets are `^[a-zA-Z0-9_-]+$`
 * Custom Stream Names and Stream ingest subjects to cater for different domains, mirrors and imports 
 * Key starting with `_kv` is reserved for internal use

This is an implementation of this concept focussed on the CLI, nats.go and others will have to build language
specific interfaces focussed on performance and end user.

## Design

### Storage

Given a bucket `CONFIGURATION` we will have:

 * A stream called `KV_CONFIGURATION` with subjects `kv.CONFIGURATION.*`
 * The stream has Max Messages Per Subject limit set to history with optional placement, R and max age for TTL
 * Getting a value is an ephemeral consumer with filter subject for the key (`kv.CONFIGURATION.foo`), last received in `Direct` mode
 * A state of `NumPending+Delivered.Consumer` being 0 means the key does not exist
 * We store headers as per the table below

### Headers

|Header|Description|
|------|-----------|
|KV-Origin-Server|The server name where the client was connected to that created the value|
|KV-Origin-Cluster|The cluster where the client was connected to that created the value|
|KV-Origin|If the client opts into this, the IP that created the value|

### Watchers

Watchers can either be per key or per the entire bucket.

For watching a key we simply send key updates over the watch, starting with the latest value or nothing. We will only 
send the last result for a subject - `NumPending==0`.

For watching the bucket we will send a nil if the bucket is empty, else every result even historical ones.

### Read Replicas

A read replicas a mirror stream from the primary stream.  The KV client is configured to do its reads against the 
named bucket but all writes go to the main bucket for the KV based on above naming.

This will inevitably result in breaking the read-after-write promises and should be made clear to clients.

Local read caches to be build from the primary bucket not the replica. 

### Local Cache

A local cache wraps the JetStream storage with one that passes most calls to the JetStream one but will store results
from `Get()` in a local cache to serve later.

It will also start a Bucket Watch that pro-actively builds and maintains a cache of the entire bucket locally in memory.
The cache will only be used when `ready` - meaning the watch has delivered the last message in the watched bucket to 
avoid serving stale data.
