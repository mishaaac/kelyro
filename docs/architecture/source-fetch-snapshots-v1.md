# Source fetch and immutable snapshots v1

Step 09 connects the hardened HTTP transport to immutable Research snapshot
history. `internal/infra/researchfetch` implements the application-owned
`SourceFetcher` port; `SnapshotCaptureService` owns conditional revalidation,
snapshot construction, append-only persistence, and raw-body disposition.

The live call still passes through the Step 07 `FetchService`. The adapter does
not authorize itself and cannot bypass `privacy.allow_network` or the selected
`offline`, `online`, or `auto` mode.

## Fetch adapter contract

`source-fetch-v1` maps a validated `FetchRequest` to one GET through
`researchhttp.Client`:

- `ETag` becomes `If-None-Match`;
- `LastModified` becomes `If-Modified-Since`;
- `MaximumBytes` narrows the HTTP client's configured decoded-body limit;
- the final redirected HTTP(S) locator is preserved in the result;
- application output records whether retrieval was `live` or `cache`, with the
  privacy-gated service owning that classification;
- `ETag`, `Last-Modified`, status, media type, decoded length, and fetch time
  are converted to transport-neutral Research values;
- body-bearing success responses receive a canonical content hash.

Canonical hash algorithm `sha256` hashes the exact bounded bytes returned after
safe HTTP content decoding and before any Step 10 normalization. Its serialized
form is lowercase `sha256:<64 hex characters>`. A changed byte sequence changes
the hash even when a provider reuses an ETag; an ETag is a conditional validator,
not content identity.

The adapter relies on `researchhttp` for content-type validation, SSRF defense,
redirect limits, timeouts, retry policy, compression, and global response
limits. The request-specific limit can only lower the configured global limit.

## Capture and conditional history

`SnapshotCaptureService` first resolves the stable source and its latest
snapshot. When prior `ETag` or `Last-Modified` metadata exists, it supplies the
validators to the privacy-gated fetch service.

The stable `SourceID` must match the request, while a different final locator is
valid after a hardened redirect. Only an origin marked `live` is a new fetch
observation and may append history. An offline/cache result is reusable data,
not a new snapshot, and capture rejects it rather than falsifying `fetched_at`.

Every successful observation appends a new `SourceSnapshot`; no repository
operation updates an existing snapshot. For a body-bearing `2xx` response, the
service independently verifies that decoded length and `sha256` metadata match
the returned body before appending it.

A `304 Not Modified` is valid only when a prior snapshot exists, has a
conditional validator, has a canonical content hash, and does not postdate the
revalidation. The new snapshot records status `304` and the current fetch
version/time while carrying forward the prior content type, length, hash, and
missing validator values. `SnapshotCapture.RevalidatedSnapshotID` identifies
that prior snapshot to the caller. Persisted history can reconstruct the same
reference as the nearest preceding snapshot for the source with that hash.

This design records the revalidation event without duplicating content or
overwriting historical metadata. A `200` response creates a new snapshot even
when its ETag or content hash is unchanged.

## Body policy

Raw web bodies never enter `SourceSnapshot` or the snapshot table. Each capture
declares one disposition:

| Policy | Step 09 behavior |
| --- | --- |
| `metadata_only` | Discard the body after hash/length verification. |
| `normalized_excerpt` | Return a defensive transient `FetchedSource` as `NormalizationInput` for the Step 10 pipeline; snapshot capture does not parse or persist it. |
| `bounded_cached_body` | Return a defensive `CacheCandidate`, limited to 1 MiB, for the future Step 32 cache; Step 09 does not write cache entries. |

The metadata-only path is the default storage shape, not an unbounded archive.
Evidence excerpts remain separately limited by persistence, and cache
encoding, expiry, stale behavior, and eviction remain Step 32 responsibilities.

## Failure and trust boundary

Invalid modes, policies, source relationships, orphan `304` responses,
non-canonical hashes, and inconsistent lengths are `invalid_state`. Repository
failures retain the application persistence taxonomy; network/client failures
retain the Step 07 external/unavailable/network-blocked classifications. An ID
generation failure is `unavailable`.

Fetched content remains untrusted data. Step 10 now consumes the transient
normalization input through the separate adapter documented in
[source-normalization-v1.md](source-normalization-v1.md). Snapshot capture still
performs no parsing, evidence extraction, claim generation, trust decision,
release ingestion, or curriculum compilation.
