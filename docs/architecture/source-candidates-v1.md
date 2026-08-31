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
keeps duplicate locators.

## Deduplication

`source-candidate-deduplication-v1` merges mapped candidates across queries in
stable first-seen order. Canonical identity applies only these conservative
rules:

- lowercase scheme/host through `SourceLocator`;
- remove fragments;
- remove default `:80`/`:443` ports;
- represent an empty root path as `/`.

All other paths and query strings remain significant. The policy does not
strip trackers, reorder parameters, follow redirects, or inspect HTML canonical
links.

Every distinct discovery observation is retained, so one URL records every
query/provider/rank/time path that found it. Exactly repeated observations are
stored once. Inputs and total discoveries are capped at 500 per run.

The service reads the existing stable Source repository and attaches a matching
Source ID to its result. This link prevents later duplicate registration; it
does not make the candidate trusted. Two existing Sources that collapse to the
same canonical URL are rejected as ambiguous rather than resolved silently.

## Boundaries

Candidates contain no stable Source ID, kind classification, registry
authority, trust tier, Evidence, Claim, or verification result. Mapping does
not fetch content, inspect HTML canonical links, strip query parameters, or
infer that a provider-ranked result is trustworthy.

The deduplication result may reference an already durable Source ID, but the
candidate itself remains transient and untrusted. Step 18 performs no write;
new Source registration belongs to Step 19.

## Registration

`source-candidate-registration-v1` reuses the stable Source repository. A
candidate already linked during deduplication keeps that Source unchanged. A
new canonical locator creates one `Source` with kind `other`, current temporal
scope, and the first observed title; provider rank and publication hints never
become classification, authority, or trusted Source metadata.

Every distinct observation is persisted as an immutable `DiscoveredSource`
linked to its request and Source. The record keeps the bounded query, title,
snippet, provider, rank, discovery time, optional publication hint, and cache
origin flags. Stable content-derived IDs make retries idempotent. Registration
can recover from a concurrent Source insert by resolving the canonical locator,
but it rejects an existing identity whose locator changed after deduplication.

The append-only discovery store is separate from `trust_registry`. Registration
does not match an authority profile, write a trust decision, fetch content, or
promote provider metadata to Evidence. Those remain later explicit stages.
