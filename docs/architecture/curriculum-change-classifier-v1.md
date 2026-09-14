# Curriculum Change Classifier v1

## Contract

`curriculum-change-classifier-v1` compares two valid immutable definitions of
the same curriculum with different versions. Optional old/new Environment
Packs and explicit I-03 Drift/Impact reports enrich the comparison. The output
is a deterministic `CurriculumChangeClassification` containing validated,
stably ordered `CurriculumChange` records and deterministic change IDs.

The classifier is read-only. It does not select a pack version, apply a
migration, activate a pack, or read/write Student Core state. Its output is the
input to `pack-versioning-policy-v1` and the future Step 42 migration planner.

## Change detection

The v1 classifier emits the closed kinds required by I-04:

| Kind | Detection | Default migration class |
|---|---|---|
| `metadata_only` | display metadata, goal/competency/vocabulary/coverage metadata, or same-ID Concept specification | `safe` for display-only; otherwise `requires_recompile` |
| `source_refresh` | Source Bundle identity, evidence references, or validated Drift/Impact signals | `safe`, elevated by I-03 signals |
| `concept_added` | ID present only in new definition and not mapped | `safe` |
| `concept_removed` | ID present only in old definition and not mapped | `requires_student_review` |
| `concept_split` | explicit one-old-to-many-new identity mapping | `breaking` |
| `concept_merged` | explicit many-old-to-one-new identity mapping | `breaking` |
| `prerequisite_changed` | prerequisite endpoint/kind set changed | `requires_recompile` |
| `hierarchy_changed` | phase/module/lesson/topic projection changed | `requires_recompile` |
| `status_changed` | same-ID Concept temporal status changed | `requires_student_review` |
| `environment_changed` | Environment Pack presence or content changed | `requires_recompile` |

Evidence-only changes on a prerequisite do not masquerade as a graph change;
they are a source refresh. Hierarchy remains separate from prerequisites and a
hierarchy move never implies lost mastery.

## Concept identity changes

Split and merge cannot be inferred safely from similar titles, definitions, or
evidence. They require a validated `ConceptIdentityMapping` plus a rationale:

```text
one removed old ID -> multiple added new IDs = split
multiple removed old IDs -> one added new ID = merge
```

Mapped IDs must exist on their declared side, must actually be removed/added,
and may occur in only one mapping. Without a mapping, the exact same old/new
shapes are classified as independent removals and additions. The classifier
therefore never invents mastery continuity.

## I-03 Drift/Impact integration

Every supplied report must validate under its own I-03 algorithm version. A
Drift report must reference an old Source Bundle in the old definition and its
optional new bundle must occur in the new definition. Every Impact report must
reference one supplied Drift report.

Impact `FutureConceptRefs` restrict affected concepts to IDs present in either
definition. Important drift or `recompile_future` elevates the source refresh
to `requires_recompile`; critical drift, `review_curriculum`, or
`manual_review` elevates it to `requires_student_review`. Signals never create
claims or rewrite curriculum content.

## Determinism

Affected Concept IDs and change kinds are sorted. Mappings and report inputs
are accumulated as sets with conservative maximum migration severity. Change
IDs hash from/to versions, kind, migration class, and sorted Concept IDs. Input
reordering therefore does not change the result.
