# Pluggable source discovery

Step 11 completes the application-owned source-discovery boundary without
binding Kelyro to a search vendor. Discovery returns unverified candidates; it
does not fetch content, classify trust, create snapshots, extract evidence, or
verify claims.

## Contracts

`SearchProvider` receives separate, transport-neutral values:

```text
Search(ctx, SearchQuery, SearchOptions) []SearchResult
```

`SearchQuery` carries the stable research request identity and query text.
`SearchOptions` carries the optional desired source kind and target version,
plus a required positive limit capped at 100 results. Keeping options separate
allows Step 12 query plans to vary search intent without introducing provider
request types into application or domain code.

Every `SearchResult` contains:

```text
title
HTTP(S) locator
optional snippet
provider identity
provider rank
optional published timestamp hint
```

The published timestamp is only provider metadata. It is not evidence, a
verified source publication date, or a freshness input until a later stage
fetches and verifies the source.

## Normalization and duplicate policy

`DiscoveryService` normalizes query text before crossing the provider/cache
boundary by collapsing Unicode whitespace. It applies the same deterministic
output policy to live and cached candidates:

1. reject more than 100 provider results;
2. collapse whitespace in titles, snippets, and provider identities;
3. validate every HTTP(S) locator and remove its fragment for document-level
   candidate identity;
4. validate ranks and optional published hints;
5. keep the first occurrence of each normalized locator;
6. preserve provider order and each surviving provider rank exactly;
7. return at most the requested number of unique candidates.

Paths and query strings remain significant. Discovery does not guess which
query parameters are trackers or merge different resources on the same host.
All returned candidates are defensive values, including timestamp pointers.

Malformed live output is `external_failure`; malformed cached output is
`persistence_failure`. Provider cancellation and deadlines retain the shared
`unavailable` classification. A provider error preserves its cause.

## Provider and privacy boundaries

The interface is pluggable and contains no API-key field or vendor-specific
configuration. `application/memory.StaticSearchProvider` is the initial
deterministic, network-free provider for tests and development. A later live
adapter can replace it without changing the application contract, but every
live call must continue through `DiscoveryService` and Foundation's
`privacy.allow_network` gate.

Offline fallback remains a separate `SearchCache` port. A cache implementation
cannot be wired accidentally as a live provider, and live and cached candidates
receive identical normalization, deduplication, rank, and limit handling.

## Candidate-only lifecycle

The mandatory lifecycle remains:

```text
discover → classify → fetch → verify
```

A search result is neither a registered `Source` nor `Evidence`. It cannot be
used to support a claim merely because a provider ranked it highly or supplied
a snippet or publication hint.

## Deferred work

Step 11 adds no live search adapter, API credentials, query planner, source
classification policy, evidence extraction, cache persistence/expiry, release
discovery, CLI command, Curriculum Compiler behavior, or Student Core changes.
Those remain in their separately authorized steps.
