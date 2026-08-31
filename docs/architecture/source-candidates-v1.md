# Source Candidates v1

`source-candidate-mapper-v1` is the transient boundary between provider search
results and durable Sources. A candidate is neither a Source nor Evidence and
cannot support a Claim or inherit authority from provider rank.

## Mapping

Each normalized `SearchResult` becomes one `SourceCandidate` in provider order.
The candidate owns its fragmentless HTTP(S) locator and one bounded discovery
observation containing:

```text
request/query
title
snippet
provider
rank
discovered_at
optional published hint
cache origin metadata
```

Whitespace and locator normalization reuse the discovery boundary. Timestamp
pointers and artifact slices are defensively copied. Provider title, snippet,
rank, and publication hints remain untrusted metadata even after mapping.

The mapper accepts at most 100 results per query, observes context
cancellation, performs no network or persistence operation, and deliberately
keeps duplicate locators. Cross-query deduplication and existing-Source lookup
belong to I-03C Step 18.

## Boundaries

Candidates contain no stable Source ID, kind classification, registry
authority, trust tier, Evidence, Claim, or verification result. Mapping does
not fetch content, inspect HTML canonical links, strip query parameters, or
infer that a provider-ranked result is trustworthy.
