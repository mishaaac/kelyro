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
