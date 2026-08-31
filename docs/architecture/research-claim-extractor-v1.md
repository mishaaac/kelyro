# Conservative Claim Extractor v1

I-03C Step 25 defines `claim-extractor-v1`, the deterministic boundary from
persisted Evidence to conservative Claim candidates. The extractor consumes
one validated `Evidence` record plus its Research topic/purpose and optional
target version. It performs no network or repository I/O, uses no LLM, and
does not apply authority, trust, freshness, corroboration, or conflict policy.

Every `ClaimCandidate` references exactly one Source, snapshot, and persisted
Evidence identity. Its statement is an exact bounded sentence from
`Evidence.Excerpt`; context before/after can help a reviewer but cannot supply
words missing from the statement. Search candidates and normalized/raw content
are no longer valid inputs at this boundary.

## Initial families

V1 recognizes only six families and maps them to existing domain Claim types:

| Claim family | Required explicit shape | Domain Claim type |
| --- | --- | --- |
| `explicit_definition` | subject + direct `is/means/refers to` definition | `definition` |
| `version_release_fact` | explicit version/release marker and literal version | `version_change` |
| `deprecation_statement` | direct deprecated/obsolete/removed/not-supported marker | `deprecation` |
| `availability_support_statement` | direct available/supported/unsupported/compatible statement | `compatibility` |
| `explicit_requirement` | direct must/shall/required/debe/deberá marker | `requirement` |
| `explicit_recommendation` | direct should/recommended/prefer/se recomienda marker | `recommendation` |

The implementation may use a closed English/Spanish v1 marker lexicon. A
keyword is necessary but never sufficient: the selected sentence must also be
topic-anchored and satisfy the complete family shape.

## Candidate contract

```text
source_id + snapshot_id + evidence_id
family
literal statement + canonical hash
bounded applicability scope
optional opaque version scope
explicit status scope
fixed family confidence
exact marker + sentence index
extractor_version
```

Statements are capped at 2 KiB, explicit markers at 256 bytes, and scope uses
the existing 1 KiB Claim ceiling. Sentence indexes are zero-based within the
Evidence excerpt. V1 returns at most eight candidates per Evidence and the live
stage may retain at most 1,000 per run.

`CanonicalClaimCandidateStatementHashV1` is SHA-256 over the exact statement
bytes. It does not lowercase or paraphrase content. The statement therefore
remains auditable against the Evidence excerpt.

## Ambiguity gate

V1 emits no candidate when any of these conditions holds:

- a sentence matches zero or more than one family;
- the explicit subject is absent and only surrounding context mentions it;
- uncertainty/hedging changes the assertion (`may`, `might`, `could`,
  `possibly`, `appears`, `quizá`, `podría`, and equivalent closed markers);
- negation makes the family unclear rather than forming an explicit
  unsupported/not-supported statement;
- more than one incompatible version or lifecycle status appears;
- a sentence is truncated, grammatically incomplete, or exceeds the bound;
- a marker occurs only in code, a URL, a heading label, or quoted attribution
  without a direct assertion by the Evidence passage;
- extracting qualifiers would require inference from another Evidence record.

Multiple markers for the same family are permitted only when they express one
unambiguous assertion. For example, `deprecated and removed in version 3`
remains one deprecation family; `deprecated, but should be preferred` matches
deprecation and recommendation and is rejected.

## Qualifiers and confidence

`Scope` is the bounded Research subject/application scope, never an invented
platform or audience. `VersionScope` is copied only from an explicit literal
qualifier or from the requested target when that exact opaque value occurs in
the sentence. V1 does not impose SemVer. `StatusScope` is `all` unless exactly
one explicit `stable`, `preview`, `experimental`, or `legacy` qualifier occurs.

Confidence measures extraction directness, not source truth or authority:

| Family | Confidence |
| --- | ---: |
| explicit definition | 0.85 |
| version/release fact | 0.90 |
| deprecation statement | 0.90 |
| availability/support | 0.85 |
| explicit requirement | 0.90 |
| explicit recommendation | 0.80 |

These constants are part of `claim-extractor-v1`. Trust and multi-source
verification may later accept, supplement, conflict, or reject the persisted
Claim without rewriting this extraction confidence.

## Dedupe and persistence

Candidates are ordered by Evidence identity and sentence index. Exact
duplicates use family + statement hash + scope + version/status scope as their
semantic key. Step 26 aggregates byte-identical candidates from distinct
Sources into one Claim with multiple SourceIDs/EvidenceIDs. Differently worded
statements are never declared equivalent without a future explicit semantic
policy; they remain separate Claims and may be insufficiently corroborated.

The live stage persists Claims with stable semantic IDs and idempotent replay only
after loading each referenced Evidence and validating Source/snapshot
ownership. A run with Evidence but no unambiguous candidate fails rather than
inventing a Claim.

Every supporting Evidence also receives a durable `citation-v1` record tied to
the same Source and snapshot. Claim output validation requires a citation for
every Evidence ID; citations are deduplicated by Evidence when one excerpt
supports multiple Claims.

The implementation does not apply trust/freshness, verification, conflicts,
bundle assembly, I-04, or AI review.
