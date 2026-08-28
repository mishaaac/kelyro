# Evidence Freshness v1

Step 16 adds the deterministic `freshness-v1` policy in
`internal/research/freshness`. It measures how recently evidence was verified;
it does not measure source authority, publication recency, or claim confidence.
The model is pure apart from an injected clock and performs no network,
database, release lookup, or scheduling operation.

## Temporal inputs

The model keeps these values distinct:

- `last_verified_at`: when the evidence/claim relationship was last checked;
- `source_updated_at`: when the source reports that its content changed;
- snapshot `fetched_at`: when bytes were observed, not a freshness input by
  itself;
- source `published_at`: original publication time, not verification time;
- evaluation time: the injected clock value used for this assessment; and
- freshness score: derived policy output, never a timestamp substitute.

`Input` requires a valid claim type and source kind. Last verification and
source update are optional UTC timestamps; release cadence is zero when unknown
or an integer from 1 through 3,650 days. Future verification/update timestamps
are rejected rather than silently clamped.

## Effective TTL

An Authority Profile may carry at most 64 `FreshnessTTLHint` entries. Each hint
selects an exact claim/source pair, a claim type, a source kind, or the whole
profile, with a TTL from 1 through 3,650 days. Matching precedence is:

1. exact claim type and source kind;
2. claim type;
3. source kind;
4. profile-wide default.

Duplicate selectors are invalid. When no hint matches, v1 takes the smaller of
the claim and source defaults:

| Claim type | Default days |
| --- | ---: |
| security | 14 |
| version change, deprecation, compatibility | 30 |
| warning | 45 |
| behavior, recommendation | 90 |
| example | 120 |
| definition, requirement | 180 |
| historical | 365 |

| Source kind | Default days |
| --- | ---: |
| release notes, issue tracker, community forum, playground | 30 |
| official blog, community article, video | 60 |
| package reference, source code, other | 90 |
| official documentation, official tutorial | 120 |
| specification, standard, paper | 180 |
| book reference | 365 |

Step 29 keeps video at 60 days regardless of official/community affiliation.
Affiliation affects contextual trust, not the evidence-age formula; an
Authority Profile TTL hint may still provide an explicit narrower policy.

A known release cadence then caps, but never lengthens, the selected TTL. An
explicit Authority Profile hint therefore configures the base TTL while the
technology cadence remains a safety ceiling.

## State and score

Without `last_verified_at`, v1 returns `unknown`, score `0`, and effective TTL
`0`. It does not invent a timestamp merely to create a persisted record.

When verification exists, a known relevant new release or a source update
strictly later than verification immediately returns `stale` with score `0`.
Otherwise, for non-negative verification age `a` and effective TTL `t`:

```text
score = max(0, 1 - a / (2 * t))
```

The state boundaries are explicit:

```text
a <= t/2       fresh
t/2 < a <= t   aging
a > t          stale
```

Thus a newly verified item scores `1`, the fresh boundary scores `0.75`, the
aging/stale TTL boundary scores `0.5`, and age at or beyond twice the TTL scores
`0`. State uses the TTL boundary directly; score remains a continuous signal.

Every known `Assessment` carries the exact last verification used. Every
assessment records `freshness-v1`, evaluation time, effective TTL, and ordered
reason codes identifying defaults/profile hints, cadence caps, temporal
triggers, missing verification, and the selected age band.

## Persistence and configuration

The existing `FreshnessRecord`/repository stores versioned known-verification
outputs. `unknown` can be evaluated in memory without fabricating the required
`last_verified_at` persistence field. Step 17 adds the separate
`refresh-scheduling-v1` policy and may combine a known assessment with its
versioned schedule; `freshness-v1` itself still does not populate or interpret
`next_verify_at`.

Forward-only SQLite migration v29 adds bounded
`freshness_ttl_hints_json` to Authority Profiles. Existing profiles receive an
empty list. The strict YAML loader accepts optional `freshness_ttl_hints` and
the checked-in Go profile demonstrates claim- and source-specific hints.

## Boundaries

`freshness-v1` consumes `known_new_release` as an already established signal;
it does not discover releases or compare opaque versions. It does not fetch
sources, verify claims, resolve conflicts, schedule refreshes, scan for drift,
compile curriculum, mutate Student Core, or infer that an undocumented fact is
absent.

Refresh scheduling is specified separately in
[refresh-scheduling-v1.md](refresh-scheduling-v1.md).
