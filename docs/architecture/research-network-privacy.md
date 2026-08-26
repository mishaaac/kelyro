# Research network privacy boundary

Step 07 makes Foundation's deny-by-default network policy mandatory at every
Research Engine use case that can invoke a live adapter. The boundary lives in
`internal/research/application`; the pure `internal/research` domain still has
no dependency on configuration, networking, or Foundation services.

## Protected operations

Three stable authorization operation IDs cover the live boundaries introduced
by the Research application contracts:

| Use case | Authorization operation | Live port |
| --- | --- | --- |
| source discovery | `research.discovery` | `SearchProvider` |
| source fetch | `research.fetch` | `SourceFetcher` |
| release lookup | `research.release_lookup` | `ReleaseLookupProvider` |

Every request uses Foundation purpose `external-resource`. Authorization
metadata never includes URLs, search text, source content, technology names,
workspace paths, or credentials. The application receives an already resolved
`privacy.NetworkGate`; it does not read configuration itself.

## Research modes

`ResearchMode` is a closed application value with the following behavior:

| Mode | Behavior |
| --- | --- |
| `offline` | Never consults the gate or a live provider; uses only the corresponding offline cache. |
| `online` | Consults the gate and uses live access only when Foundation authorizes it; a denial is returned even when cached data exists. |
| `auto` | Consults the gate; uses live access when authorized and the corresponding offline cache when policy denies access. |

No mode overrides `privacy.allow_network`. In particular, `online` expresses
the caller's requested behavior, not permission. A missing privacy gate is an
`unavailable` dependency and never causes a live call.

## Offline cache separation

The offline read contracts are intentionally distinct from live ports:

- `SearchCache.SearchCached`, with the same query/options as the live provider;
- `SourceFetchCache.FetchCached`;
- `ReleaseLookupCache.LookupCachedReleases`.

This makes the fallback path explicit and prevents an adapter labeled as a
cache from silently delegating to a network provider. Cached outputs pass the
same normalization, URL deduplication, rank preservation, limits, and
structural validation as live outputs. Step 07 does not define cache encoding,
cache writes, expiry, or eviction; the existing bounded
`ResearchCacheRepository` remains available for future adapters.

When live access is impossible and the offline cache is absent or reports a
miss, the service returns the stable application classification
`network_research_blocked` through `ErrNetworkResearchBlocked`. A Foundation
privacy denial remains discoverable with
`errors.Is(err, privacy.ErrNetworkBlocked)`. Invalid mode/input, missing
dependencies, cache failures, and provider failures retain their separate
application categories.

## Local operations remain available

The privacy gate is not applied to repository-only reads. Stored sources,
snapshots, evidence, registry entries, freshness records, verification output,
drift/impact records, and opaque cache data remain available while network
access is disabled. The source registry CLI therefore remains fully offline.

Step 20 release discovery reuses `SnapshotCaptureService`; its JSON/Atom
release adapters parse only the bounded bytes returned after the
`research.fetch` authorization. They have no transport and cannot bypass the
gate. Stored releases, Evidence, and Claims remain ordinary offline repository
reads.

## Deferred work

Step 11 adds a network-free static search provider and Step 20 adds network-free
release-feed parsers. No production search provider, cache format, background
activity, or public research command exists.
Student Core state and Curriculum Compiler behavior are unchanged.
