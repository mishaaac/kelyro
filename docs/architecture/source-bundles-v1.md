# Source Bundles v1

`source-bundle-v1` is the immutable I-03 hand-off that lets I-04 consume a
bounded set of researched Claims without repeating discovery and fetch work.
It does not compile a concept or lesson and does not modify learner state.

## Bounded representation

Each bundle records:

- stable bundle and completed research-run identities;
- the exact research topic, purpose, and optional target version;
- ordered Claim identities;
- primary, supporting, and historical source identities with frozen temporal
  scope, optional source version, and warning;
- the latest visible conflict identity for each canonical Claim pair;
- conservative aggregate Claim freshness;
- closed issue codes, derived lifecycle state, a human-readable summary,
  verification time, algorithm version, and canonical content hash.

The machine representation is canonical JSON capped at 256 KiB. It contains no
raw fetched page, full source document, evidence excerpt, transcript, or other
unbounded external content. Claims and Evidence remain independently persisted
and traceable by stable identity.

## Source roles

A source is `historical` when it is archived/historical, or when a
version-bound source does not match the bundle target. A current or exact-target
source is `primary` only when its latest Trust Decision is `accepted`, its tier
is A/B, and its kind is specification, standard, official documentation,
package reference, release notes, or source code. Other declared Claim sources are
`supporting`.

These roles are immutable assembly annotations. Later source or trust
reclassification creates a new bundle; it never rewrites an old one. Legacy
rows use `unclassified` only under `source-bundle-unversioned-legacy`.

## Freshness aggregation

`source-bundle-freshness-v1` evaluates the selected Claims, not an arbitrary
publication date. Its score is the minimum stored score, its timestamp is the
oldest stored `last_verified_at`, and its state is the worst known state in the
order `fresh < aging < stale`. Any Claim without stored freshness makes the
aggregate `unknown` and records that Claim ID explicitly. Underlying freshness
algorithm identifiers are preserved and sorted.

## State policy

State is derived from closed issue codes with this precedence:

1. `incomplete` when required Evidence or verification is missing, verification
   is insufficient/rejected/legacy, or Claim freshness is missing;
2. `conflicted` when a selected Claim has an unresolved latest conflict or a
   conflicted verification result;
3. `ready_with_caveats` for verified-with-caveat results, resolved conflicts,
   non-current sources, or aging/stale freshness;
4. `ready` only when none of those conditions is present.

Missing records are never invented and never silently omitted from the state.
Incomplete takes precedence because I-04 cannot treat a conflicted but
under-specified record as sufficient evidence.

## Determinism and hash

Claims, sources, conflicts, issues, missing-freshness Claims, and underlying
algorithm identifiers are canonically sorted. The SHA-256 content hash covers
the entire canonical representation except the `content_hash` field itself,
including bundle/run identities, timestamps, summary, state, and algorithm
versions. Reordering equivalent input therefore produces identical JSON and
hashes; changing any hashed field invalidates validation.

`ExportJSON` emits the human and machine-readable representation together.
`ParseSourceBundleJSON` rejects unknown fields, trailing data, oversize payloads,
invalid vocabulary, and hash mismatches.

## Application and persistence

`SourceBundleService.Assemble` requires a completed Research Run and selected
Claim IDs. It loads Claims, every declared Evidence identity, latest
multi-source verification, the union of Sources and latest Trust Decisions,
latest conflict per canonical pair, and Claim freshness. The pure assembler
derives the output and the append-only repository stores it atomically.

SQLite migration v35 activates the tables reserved in v23. New records store
the bounded canonical JSON and hash alongside indexed bundle metadata and
ordered Claim/source relationship rows. Source rows also freeze role, temporal
scope, version, and warning. Existing rows remain readable as
`source-bundle-unversioned-legacy` with unknown freshness, no invented hash, and
unclassified sources.

The memory and SQLite adapters provide exact lookup plus deterministic
run-history listing and defensive copies. Neither adapter performs networking,
re-verification, conflict resolution, freshness calculation, curriculum
compilation, or learner-state mutation.
