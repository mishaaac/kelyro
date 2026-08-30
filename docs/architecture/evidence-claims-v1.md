# Structured Evidence and Claims v1

Step 13 completes the domain model that distinguishes a fetched resource from
what that resource actually supports:

```text
Source   = stable external resource identity
Snapshot = immutable observation of one fetch
Evidence = minimal bounded excerpt or observation from that snapshot
Claim    = structured assertion backed by one or more evidence records
```

Neither a discovery result nor its provider snippet is evidence.

## Evidence contract

Every `Evidence` identifies its source and immutable snapshot, a meaningful
location, the exact excerpt, its canonical hash, optional bounded context on
both sides, extraction time, and extractor version.

The domain enforces byte limits rather than rune counts so persistence and
in-memory validation agree for all UTF-8 text:

| Field | Requirement | Maximum |
| --- | --- | ---: |
| `excerpt` | required, valid UTF-8 | 8 KiB |
| `context_before` | optional, valid UTF-8 | 2 KiB |
| `context_after` | optional, valid UTF-8 | 2 KiB |

`CanonicalEvidenceExcerptHashV1` is canonical SHA-256 over the exact UTF-8
bytes of `excerpt`. Validation rejects malformed hashes and hashes that belong
to different text. Context is deliberately excluded: the hash identifies the
minimal quoted evidence, while context only helps a later reviewer interpret
or relocate it.

These limits are ceilings, not extraction targets. An extractor must retain
only the minimum excerpt necessary to support a claim and add context only when
needed to avoid ambiguity. Step 13 does not authorize mirroring complete pages,
articles, or documents.

## Claim contract

Every `Claim` contains:

```text
stable ClaimID
ResearchTopic
non-empty statement
closed ClaimType
bounded applicability scope
optional opaque version scope
explicit status scope
confidence in [0,1]
one or more distinct SourceIDs
one or more distinct evidence IDs
creation timestamp
```

The eleven claim types are `definition`, `requirement`, `behavior`,
`version_change`, `deprecation`, `recommendation`, `warning`, `example`,
`compatibility`, `security`, and `historical`.

`Scope` is a required, domain-general applicability description capped at
1 KiB, such as `language semantics`, `linux/amd64`, or `clinical screening`.
It does not assume that research concerns software. `VersionScope` remains
opaque because ecosystems use SemVer, dates, editions, revisions, and other
version systems.

`StatusScope` is explicit and closed:

```text
all
stable
preview
experimental
legacy
```

It scopes the assertion; it does not itself prove a release's channel or
lifecycle. Release intelligence remains responsible for evidence-backed status
facts.

A claim may cite multiple evidence records and multiple sources. Collection
validation rejects missing or duplicate identities. Step 14 will validate the
full provenance graph and prove that each named evidence record belongs to the
named source/snapshot chain; Step 13 does not pre-implement that graph.

## Persistence

Forward-only migration v26 adds `context_before` and `context_after` to
`evidence`, plus `scope` and `status_scope` to `claims`. Existing I-03 rows
receive compatible defaults: empty optional context, `general` scope, and
`all` status scope. SQLite repeats the byte/status constraints, and the
production Evidence repository round-trips both context fields without
retaining raw source bodies.

## Boundaries

Step 13 adds no evidence extractor, Claim repository/application service,
Provenance graph, Citation or Deep Link behavior, verification algorithm,
conflict resolution, AI inference, network access, cache policy, Curriculum
Compiler behavior, or Student Core mutation.
