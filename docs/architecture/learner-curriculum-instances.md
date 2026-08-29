# Learner curriculum instances

## Boundary and identities

Step 10 separates three identities that must never be collapsed:

```text
immutable Curriculum Definition (curriculum_id, version)
                         |
                         v
learner Curriculum Instance (instance_id, student_id, goal_id)
                         |
                         v
Instance Concept State (instance_id, concept_id)
```

A definition describes pedagogy and the knowledge graph. An instance records
that one student is following exactly one referenced definition version for a
goal. Concept state records only that learner's progress inside that instance.
Changing progress therefore cannot mutate the definition or another instance.

The domain and application boundaries remain independent of SQLite, YAML,
Bubble Tea, CLI handlers, AI providers, packs, and the operating system. The
YAML fixture is decoded by the existing adapter and passed as a validated
`Curriculum` to `CurriculumInstanceService.Create`.

## Instance lifecycle and source

An instance persists an opaque instance ID, student, goal, exact curriculum
ID/version, source kind, status, and UTC creation/update timestamps. Source
kinds are `fixture`, `import`, and `pack`; the latter two are forward contracts,
not import or Learning Pack implementations. Status values are `active`,
`paused`, `completed`, and `archived`; Step 10 creates `active` instances but
does not add lifecycle commands before a use case requires them.

Creation requires the current student's active goal and runs atomically:

1. validate the complete curriculum;
2. install its immutable definition identity;
3. create the learner instance referencing that exact version.

The logical tuple `(student, goal, curriculum_id, curriculum_version)` is
unique. A second request for that tuple is a conflict even if it proposes a
different instance ID.

Definition installation computes a canonical `sha256` fingerprint over the
complete consumption contract, including hierarchy and pedagogical metadata.
Reinstalling identical content is idempotent; reusing the same curriculum
ID/version with changed content is a conflict. Node order does not affect the
fingerprint because validated nodes and prerequisites are canonicalized first.

## Lazy concept-state policy

Concept states are initialized lazily. Creating an instance writes no progress
rows. The first `State(instance, concept)` read verifies concept membership and
materializes a valid `not_seen` row with zero mastery. This policy:

- avoids an eager write proportional to curriculum size;
- keeps untouched concepts distinguishable from stored learning activity;
- permits large definitions without a creation-time write burst;
- retains deterministic behavior because missing state always materializes the
  same domain default at the service clock.

`States(instance)` lists only materialized rows. Callers needing every concept
combine the immutable definition with these sparse states; they must not infer
progress by modifying definition nodes.

Each state is keyed by `(curriculum_instance_id, concept_id)` and also carries
the student owner for database integrity. It stores exposure, evidence-derived
mastery, first/last seen, mastered/review-due timestamps, an update timestamp,
and sorted opaque manual flags. Manual flags reserve durable metadata only;
they do not override prerequisites or implement unlock behavior in Step 10.

Learning evidence remains in the existing append-only evidence aggregate. It
is not copied into instance state. A later mastery policy may project evidence
into the state while preserving evidence as the auditable source.

## Persistence and compatibility

Migration v9 is forward-only and adds:

- `curriculum_definition_fingerprints`;
- `learner_curriculum_instances`;
- `learner_curriculum_concept_states`.

The published v4 table named `curriculum_instances` remains unchanged. Despite
its historical name, it is the versioned definition catalog referenced by
`curriculum_nodes` and `curriculum_edges`. Reinterpreting or renaming that table
would modify a published migration, so v9 gives actual learner instances a new,
unambiguous table.

The old `student_concept_states` table also remains intact. Upgrade to v9 does
not infer learner instances or copy those rows because legacy rows contain no
goal, curriculum version, or instance provenance. Guessing would silently
assign progress to the wrong identity. Tests verify that legacy state survives
and both new learner tables start empty.

Foreign keys protect student/goal ownership and exact definition versions.
Checks protect enums, mastery bounds, UTC timestamps, and temporal state
invariants. Indexes support listing a student's instances and filtering sparse
states by instance/exposure.

## Reopen and future curriculum-version migration

Reopening a workspace reconstructs the same instance reference and sparse
concept state from SQLite. Tests install the deterministic foundation fixture,
persist state, close the store, and verify the exact curriculum version and
state after reopening.

Different versions of the same curriculum may coexist as separate instances,
and their states remain isolated even for stable concept IDs. This is the
required preparation for a future explicit migration policy. That future
policy can compare old/new immutable definitions, map stable concepts, decide
which progress is transferable, and transition instance statuses. Step 10 does
not auto-copy state, merge evidence, or guess mappings.

I-03 Step 40 now defines the design hand-off for that future owner, including
explicit continuity mappings and non-destructive state directives, in
[research-to-curriculum-update-contract.md](research-to-curriculum-update-contract.md).

The prerequisite application service now requires an instance ID and performs
one `ListByInstance` read. It verifies instance ownership, projects only that
instance's states into the existing pure graph snapshot, and preserves the
Step 9 traversal, threshold, and explanation policies unchanged.
