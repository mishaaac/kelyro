# Research-to-curriculum update contract v1

## Purpose and ownership

This Step 40 contract defines the selective, student-safe hand-off from I-03
Research & Source Intelligence to the future I-04 Curriculum Compiler. It is a
design contract, not a curriculum migration implementation.

```text
I-03 observes and classifies knowledge change
  └─ DriftReport + ImpactReport + old/new SourceBundle + ChangeClassification
                                      |
                                      v
I-04 decides and compiles curriculum change
  └─ selective compile plan + versioned immutable Curriculum definition
                                      |
                                      v
future learner-migration owner applies an explicit student-safe plan
  └─ old instance retained + per-concept transfer decisions + audit
```

I-03 MUST NOT read or write curriculum definitions, learner curriculum
instances, concept state, mastery, retention, reviews, mistakes, diagnostics,
sessions, history, streaks, or schedules. A recommended action in an
`ImpactReport` is advisory metadata, never a command.

The contract identifier is `research-to-curriculum-update/v1`. Consumers MUST
reject unsupported contract versions rather than reinterpret them.

## I-03 update envelope

The future transport-neutral hand-off has this logical shape:

```text
ResearchUpdateEnvelope
  contract_version
  generated_at
  old_bundle_ref
  new_bundle_ref optional
  drift_report_ids[]
  impact_report_ids[]
  change_classification
  affected_claim_ids[]
  affected_evidence_ids[]
  affected_concept_refs[]
  affected_lesson_refs[]
  technology_version_refs[]
  suggested_migration_class
  rationale[]
  unresolved[]
```

The envelope carries identities and bounded explanations, not raw fetched
pages or generated curriculum content. Every collection is duplicate-free and
deterministically ordered. Every `ImpactReport` MUST refer to one included
`DriftReport`; every affected Claim/Evidence/bundle reference MUST be traceable
through those reports.

### Source Bundle version identity

`SourceBundle` is immutable and has no mutable version field. Its versioned
identity for this hand-off is the tuple:

```text
(bundle_id, content_hash, algorithm_version, verified_at)
```

The old and new refs MUST preserve that complete tuple. A consumer MUST load
the exact bundles and confirm the hashes rather than treating a topic or run ID
as the bundle version. Changing evidence produces a new bundle ID/hash; an
existing bundle is never rewritten.

If no new bundle exists, I-03 may still report source disappearance or
unresolved impact, but I-04 MUST NOT claim that replacement curriculum is ready
to compile. A `ready_with_caveats`, `incomplete`, or `conflicted` new bundle
must retain its warnings; I-04 decides whether its compile gate permits use.

## Change classification

`ChangeClassification` answers whether the verified meaning consumed by
curriculum changed. It is distinct from `DriftType`, severity, and recommended
action.

| Classification | Meaning | Minimum I-04 response |
| --- | --- | --- |
| `no_knowledge_change` | Evidence representation changed but reviewed meaning did not | retain content; update provenance only if a new immutable version is emitted |
| `non_breaking` | Meaning was clarified or extended without invalidating prior learner knowledge | selectively recompile affected nodes; preserve stable concept continuity |
| `breaking` | Prior guidance, scope, prerequisite meaning, recommendation, version applicability, or safety is no longer valid | emit a new curriculum version and an explicit per-concept migration proposal |
| `unknown` | Available evidence cannot prove one of the above | manual decision; no automatic compile or learner migration |

The classification MUST contain a non-empty rationale, the supporting
DriftReport/ImpactReport IDs, a bounded confidence, and a reviewer or versioned
deterministic policy identity. Search candidates, changed hashes alone, missing
current evidence, popularity, or an `ImpactReport` recommendation are
insufficient to assert `non_breaking` or `breaking`.

Conservative defaults apply:

- `source_changed` alone remains `unknown` until Claims are reviewed;
- `claim_invalidated` and `deprecation_introduced` normally require `breaking`
  review but do not bypass that review;
- `version_superseded`, `recommendation_changed`, and `scope_changed` require a
  version/scope-aware decision and MUST NOT be silently called non-breaking;
- unresolved evidence always keeps the classification `unknown`.

Step 40 defines this vocabulary only. It does not add a classifier or persist
classification records.

## Stable curriculum identities

I-04 MUST obey the existing `curriculum-consumption/v1` identity rules:

- curriculum and node titles/descriptions are display data, never identity;
- a changed immutable definition receives a new curriculum version;
- the same curriculum ID/version MUST NOT be reused with a different
  fingerprint;
- node versions change when their compiled semantic or pedagogical content
  changes;
- stable concept IDs represent continuity of the same independently tracked
  knowledge unit across curriculum versions.

A concept ID may be retained only when the independently tracked knowledge and
assessment meaning remain continuous. Editorial wording, citation, or Further
Reading changes do not by themselves require a new concept ID. A semantic
replacement, split, or merge MUST use explicit old/new mappings and MUST NOT be
represented as a silent rename.

The hand-off may reference future concept and lesson IDs through
`ClaimImpactReference`; those IDs are opaque to I-03. I-04 owns validation
against the old definition and the proposed new definition.

## Selective migration classes

`suggested_migration_class` is advisory and closed:

| Class | Intended I-04 planning behavior |
| --- | --- |
| `no_migration` | no curriculum content change; provenance-only handling |
| `reverify_only` | gather or review evidence before compilation |
| `selective_recompile` | recompile only referenced concepts/lessons and structurally dependent nodes |
| `additive_update` | add new nodes without invalidating existing concept continuity |
| `deprecate_and_replace` | mark old nodes deprecated and introduce explicit replacements |
| `new_curriculum_version` | compile a new immutable definition because the knowledge contract breaks |
| `manual_decision_required` | unresolved, conflicting, ambiguous, split/merge, or otherwise unsafe to automate |

I-04 MAY choose a more conservative class and MUST record why. It MUST NOT
choose a less conservative class without an explicit reviewed override.
`review_curriculum` and `recompile_future` in `ImpactReport` are inputs to this
decision; they do not select a migration class automatically.

Selective recompilation is dependency-aware. Updating one concept may require
new versions of its containing topic/lesson/module, prerequisite edges,
assessment expectations, or other nodes that embed its compiled representation.
Unreferenced nodes remain byte/semantically unchanged where the I-04 format
allows it, but the enclosing curriculum definition still receives a new
version whenever its fingerprint changes.

## Concept continuity map

Before any learner migration, I-04 MUST emit an explicit continuity map:

```text
ConceptContinuity
  old_concept_ids[]
  new_concept_ids[]
  relationship
  knowledge_change
  supporting_claim_ids[]
  supporting_impact_report_ids[]
  student_state_directive
  rationale
```

Relationships are `unchanged`, `revised`, `added`, `deprecated`, `replaced`,
`split`, `merged`, or `removed`. Empty old IDs are valid only for `added`;
empty new IDs only for `removed`. Split/merge relationships are explicit and
never inferred from titles or positions.

The student-state directive is one of:

- `preserve`: same stable concept and no breaking knowledge change;
- `preserve_and_reverify`: retain historical facts, but require fresh evidence
  before derived mastery is asserted for changed meaning;
- `do_not_transfer`: no safe one-to-one continuity, including removed,
  replaced, split, or merge cases unless a future reviewed policy proves a
  narrower mapping;
- `not_applicable`: newly added concept has no old state.

These directives describe a future plan. I-03 never emits or applies learner
state writes, and Step 40 does not implement the future executor.

## Student-safe update invariants

Any future migration implementation MUST preserve these invariants:

1. The old immutable curriculum definition and learner instance remain
   readable and auditable.
2. An active instance is never silently rebound in place to a different
   curriculum version.
3. Append-only learning facts—Evidence, mistakes, sessions, study history,
   diagnostic attempts, and review outcomes—are never rewritten, deleted, or
   reassigned to manufacture continuity.
4. Derived state is scoped to `(curriculum_instance_id, concept_id)`. Transfer
   creates state in the target instance only through an explicit, versioned,
   idempotent migration operation.
5. Stable-ID continuity plus `no_knowledge_change` or reviewed `non_breaking`
   classification is required for `preserve`; matching titles, order, or text
   similarity is insufficient.
6. Breaking, unknown, split, merge, replacement, and removal cases never
   duplicate mastery automatically. Historical achievement remains visible in
   the old instance even when the new instance requires re-verification.
7. Added concepts start without invented exposure or mastery.
8. Retention/review schedules are not blindly copied when meaning, version
   scope, or prerequisites changed; a future owner must recompute them from
   preserved facts under explicit algorithm versions.
9. A migration plan names old/new curriculum refs, source-bundle refs, policy
   version, creation time, reviewer/actor, and a deterministic idempotency key.
10. Preview/dry-run and explicit acceptance precede writes; partial application
    is atomic or recoverable and audit-visible.

The existing I-02 model already isolates different curriculum versions in
different learner instances. This contract preserves that boundary and does
not reinterpret legacy or sparse state.

## Decision gates for I-04

I-04 MUST stop and return an explainable non-ready result when any of these
holds:

- the update contract version is unsupported;
- a referenced DriftReport, ImpactReport, Claim, Evidence, or Source Bundle is
  absent or inconsistent;
- a bundle hash/algorithm tuple does not match durable data;
- the new bundle required for compilation is missing, incomplete, or rejected
  by I-04's future compile gate;
- change classification is `unknown` or contains unresolved evidence;
- affected curriculum refs do not exist in the old definition;
- stable IDs are duplicated or a split/merge/replacement lacks an explicit
  continuity map;
- a proposed definition reuses an existing curriculum ID/version with a new
  fingerprint;
- a student-state directive would overwrite historical facts or silently
  downgrade/delete old progress.

`Not found` remains distinct from `does not exist`; a failed lookup cannot be
converted into a removal classification.

## Audit and reproducibility

A future I-04 compile or migration decision must retain:

- the exact update envelope and contract version;
- old/new Source Bundle identities and content hashes;
- all DriftReport and ImpactReport IDs;
- ChangeClassification, migration class, rationale, and policy/reviewer;
- old/new curriculum refs and definition fingerprints;
- the concept continuity map and per-concept state directives;
- preview result, acceptance actor/time, final status, and failures.

Re-running the same accepted plan against the same target state must be
idempotent. A changed bundle, classification, curriculum fingerprint, or policy
version produces a different plan identity and requires new review.

## Explicitly deferred

Step 40 does not implement:

- a Curriculum Compiler or Learning Pack format;
- ChangeClassification algorithms or repositories;
- curriculum diffing, dependency expansion, or compilation;
- learner-instance lifecycle transitions;
- copying/recalculating mastery, retention, reviews, mistakes, or schedules;
- automatic, background, destructive, or user-visible migration commands.

Step 41 separately defines the source-driven compiler request/response API.
I-04 must specify and implement compilation and migration behavior before any
of these future writes are authorized.
