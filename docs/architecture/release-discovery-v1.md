# Release discovery and release-notes ingestion v1

Step 20 implements the bounded `release-discovery-v1` orchestration and the
literal `release-notes-ingestion-v1` extractor. It discovers release facts from
already registered, contextually authorized sources, snapshots the exact live
feed observation, and persists version-scoped change claims without compiling
or upgrading curriculum.

## Pipeline and adapter boundary

```text
Authority Profile + accepted TrustDecision
                    |
                    v
       preferred registered sources
                    |
                    v
 privacy-gated SnapshotCaptureService
                    |
                    v
 bounded FetchedSource + immutable snapshot
                    |
                    v
 network-free ReleaseNotesProvider adapter
                    |
                    v
 candidates -> dedupe -> precedence -> atomic ingestion
```

`ReleaseNotesProvider` is application-owned and receives only a validated,
bounded `FetchedSource`. Providers cannot open the network. Live bytes always
pass through the existing `FetchService` and `SnapshotCaptureService`, so
`offline`, `online`, `auto`, and Foundation `privacy.allow_network` retain their
Step 07 semantics. A valid fetch observation is append-only even when parsing
later proves that the external feed is malformed.

`internal/infra/researchrelease` supplies two replaceable adapters:

- `JSONProvider` accepts common official repository/registry shapes: a top-level
  array or `releases`/`items`/`results`, version aliases, explicit channel or
  prerelease metadata, RFC3339 dates, and bounded notes/body/description fields;
- `AtomProvider` accepts standard Atom entries with title version, category
  channel, published/updated time, and content/summary notes.

Provider IDs bind a registered source to its parser. This supports official
release pages, changelogs, repository/tag feeds, and registries without making
GitHub—or any other vendor—the release model.

## Authorization and source priority

The supplied Authority Profile must match the research topic. Every input
source must:

1. already exist in the Source repository;
2. have a kind in `AuthorityProfile.PreferredKinds`;
3. have a latest `TrustDecision` of `accepted`;
4. meet or exceed the profile's minimum authority tier.

Sources are processed by the profile's preferred-kind order and then stable
`SourceID`. Supplementary, requires-verification, rejected, and below-threshold
sources cannot silently establish a release fact. Multi-source corroboration
remains the separately versioned Step 24 responsibility.

## Candidate and duplicate policy

A candidate carries exact version identity, explicit channel, optional release
date, and zero or more bounded literal note changes. Limits are 16 source feeds,
1 MiB per fetched feed, 256 releases per provider response, 256 changes per
release, and 8 KiB per evidence excerpt. Exceeding a limit fails explicitly;
raw bodies never enter SQLite.

Candidates are deduplicated by exact `(version, channel)`. Duplicate source
identities are invalid input. Equal candidates merge distinct source IDs and
note observations; conflicting known release dates are malformed provider
output. A previously persisted `(technology, version, channel)` is idempotently
reported as a duplicate and is not ingested again.

Stable release, Evidence, and Claim IDs are SHA-256-derived from their semantic
inputs. The serialized IDs expose no source content.

## Current stable and preview separation

Stable and non-stable channels form separate families. A preview, beta, RC,
experimental, or nightly release never replaces `CurrentStable`.

Within a family, v1 compares:

1. SemVer 2.0.0 precedence when both versions are semantic;
2. validated calendar order when both versions are date-based;
3. known release dates for opaque or mixed schemes.

Build metadata does not change SemVer precedence. Equal-precedence distinct
identities require distinct known release dates; otherwise current selection
fails rather than guessing. The newest stable is `current` and prior ordinary
stable records become `superseded`; legacy/EOL states are preserved. Preview
records are returned separately newest-first with the same lifecycle rule.

## Release-notes evidence and claims

Each bounded note observation becomes Evidence tied to the newly persisted
feed snapshot. The excerpt hash is canonical over the exact normalized note
text, and the extractor version is `release-notes-ingestion-v1`.

Claims use `version_change`, `scope=release notes`, the candidate's exact
`VersionScope`, and a channel-derived status scope:

| Channel | Claim status scope |
| --- | --- |
| stable | `stable` |
| preview, beta, RC | `preview` |
| experimental, nightly | `experimental` |
| unknown | `all` |

The claim statement remains the literal bounded note observation. The fixed
v1 confidence is `0.8`: extraction is exact but multi-source verification has
not occurred. Identical statements for one release combine evidence/source
references; no release-note text is executed or treated as instructions.

## Persistence and failure behavior

Step 20 adds `ClaimRepository` over the existing v23/v26 tables and a
`ReleaseIngestionRepository`. SQLite commits Evidence, Claims, new releases,
and lifecycle status changes in one transaction; the memory adapter applies the
same batch atomically after validating relationships. No migration is needed.

Malformed/oversized provider output is `external_failure`; invalid policy,
source, precedence, or relationship state is `invalid_state`; repository
failures retain their existing classification. Snapshot history intentionally
survives a later parsing failure, while no partial release intelligence batch
does.

There is no background scan, auto-upgrade, Curriculum Compiler call, Learning
Pack mutation, or Student Core mutation in this step.
