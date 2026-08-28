# Research Cache and Offline Mode v1

Step 32 turns the offline ports introduced in Step 07 into a bounded,
workspace-local cache. `research-cache-v1` is disposable acceleration data; it
is never the source of truth for SourceSnapshots, Evidence, Claims, citations,
or Source Bundles persisted as historical Research output.

## Location and layers

The filesystem adapter uses Foundation's cross-platform path contract:

```text
<workspace>/.kelyro/cache/research/
├── discovery/
├── fetch_metadata/
├── bounded_source/
├── normalized_source/
└── source_bundle/
```

Keys are SHA-256-derived filenames, so queries, locators, and arbitrary caller
keys do not become paths. Directories and files use restrictive permissions
where supported, and writes use same-directory staging plus atomic replacement.

The immutable v1 policy is:

| Layer | TTL | Per-entry payload limit |
| --- | ---: | ---: |
| discovery | 24 hours | 256 KiB |
| fetch metadata | 24 hours | 32 KiB |
| bounded source | 7 days | 1 MiB |
| normalized source | 7 days | 1 MiB |
| source bundle | 30 days | 256 KiB |

The cache is additionally capped at 512 records and 64 MiB of decoded payload.
After every put, expired records are removed first and remaining excess is
evicted by oldest `stored_at`, then layer/key as stable tie breakers.

## Hit, stale, and corruption semantics

`ResearchCacheService.Get` always returns an explicit `CacheLookup.Hit`. A miss
is not an error at this boundary. `fresh_only` treats an expired record as an
explicit stale miss. `offline_allow_stale` returns the payload with
`Stale=true` and the closed warning `stale_cache_used_offline`.

Every envelope stores schema/algorithm versions, layer, opaque key, payload,
canonical SHA-256 content hash, `stored_at`, and policy-derived `expires_at`.
JSON decoding is strict and bounded. Size, unknown fields, trailing data,
identity/filename mismatch, TTL mismatch, and content-hash mismatch are detected
as corruption. `status` reports corrupt record/byte counts without interpreting
their payload as valid data; a direct corrupt read is `persistence_failure`.

## Offline adapters

`researchcachefs.OfflineAdapter` provides concrete, network-free implementations
of the existing `SearchCache`, `SourceFetchCache`, and `ReleaseLookupCache`
ports. Discovery results use the discovery layer. A fetched source is split
between fetch metadata and bounded body layers. Release candidates reuse the
bounded metadata layer. Adapter codecs are strict, domain-general JSON and
revalidate all reconstructed application/domain objects.

The adapter also exposes normalized-source bytes and canonical Source Bundle
JSON through their dedicated layers. Source Bundles are parsed and their stable
identity must match the requested cache key.

In `ResearchModeOffline`, the application privacy boundary never calls the
Foundation network gate or live provider. Integration tests exercise stale
discovery, fetched source, release lookup, and normalized cache data after eight
days and assert zero live-provider calls. Cached SearchResult and FetchedSource
values carry explicit hit/stale/warning metadata.

## Status and clear

The CLI surface is:

```bash
kelyro research cache status
kelyro research cache clear
```

Status reports every layer plus total entries/bytes, stale records, and corrupt
files. Clear removes only `.json` records and cache staging files under the five
known Research layer directories. It does not recursively target `.kelyro`, the
workspace root, another component's cache, or SQLite.

Tests create a real SQLite Source → SourceSnapshot → Evidence chain, clear the
Research cache, and read both durable records afterward. Eviction uses the same
filesystem-only delete boundary. No confirmation is required because the
operation cannot delete evidence; the CLI states this explicitly.

## Compatibility and boundaries

The generic `research_cache_entries` table created by migration v23 remains
readable for compatibility, but `research-cache-v1` uses the Foundation cache
directory and does not create a new migration or a second historical evidence
store. Existing SQLite repositories remain unchanged.

Step 32 adds no paid-provider accounting, request budgets, trigger policy,
automatic research command, network provider, Curriculum Compiler behavior, or
Student Core/mastery mutation. Research Cost Control belongs to Step 33.
