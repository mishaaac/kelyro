# Curriculum Compiler persistence

## Migration boundary

I-04 persistence begins with forward-only SQLite migration 48,
`curriculum compiler and learning packs`, applied after the completed I-03
schema at migration 47. No earlier migration is edited.

The migration is additive and preserves Foundation, I-02 Student Core, and I-03
Research rows. It does not create or modify learner concept/mastery state.

## Reused I-02 structures

I-02 already published these tables as the deterministic curriculum-consumption
contract:

```text
curriculum_instances
curriculum_nodes
curriculum_edges
concept_registry
```

Despite its early name, `curriculum_instances` identifies an immutable
curriculum ID/version for the reusable I-02 definition; learner-specific
instances live in `learner_curriculum_instances`. I-04 therefore references
and reuses the published definition/node/edge tables instead of creating
conflicting replacements. The richer compiler representation is retained in
the new version record and projected into the I-02 tables by later compiler and
installation steps.

## I-04 tables

Stable definitions and their immutable versions:

```text
curriculum_definitions
curriculum_versions
curriculum_sources
competencies
competency_concepts
```

`curriculum_sources` references durable I-03 `source_bundles(id)` and freezes
the bundle content hash, algorithm version, verification time, and deterministic
position used for the curriculum version.

Pack availability, dependencies, and workspace-local installation state:

```text
learning_packs
learning_pack_versions
pack_dependencies
pack_installations
```

Available pack versions are distinct from installed versions. Installation
stores the validated content hash and time. A partial unique index permits at
most one active pack in the workspace database. Pack-version rows are immutable
through update/delete triggers.

Compilation diagnostics and reproducibility projections:

```text
curriculum_compilations
compilation_passes
coverage_results
curriculum_gaps
curriculum_audit_results
```

Compilation records and curriculum versions are append-only. Pass position,
coverage requirement, gap severity, and audit result keys make each projection
deterministic and queryable without reducing coverage to one score.

Environment declarations are stored separately:

```text
environment_packs
environment_tool_requirements
```

Environment-pack versions are immutable. Tool rows contain declarative purpose,
requirement level, concept introduction reference, platforms, and evidence only;
there are no secret or executable-script columns.

## Integrity and bounds

- Exact `(id, version)` primary keys prevent in-place version replacement.
- Foreign keys connect versions, nodes, competencies, bundles, packs, and
  environment tools.
- Self pack dependencies are rejected.
- Deterministic position columns are unique within their owners.
- UTC timestamps are stored with the existing `*Z` contract.
- Closed status/level fields reject unknown values.
- JSON objects/arrays use SQLite JSON checks and explicit byte limits: 1 MiB
  for manifests/config, 8 MiB for inputs/environment metadata, and 64 MiB for
  large curriculum/results.
- Source bodies, cached pages, credentials, and secrets are never retained.

## Deferred behavior

Migration 48 establishes the durable schema. Portable Learning Pack encoding,
safe loaders, Source Bundle ingestion, compilation projections, install and
activation workflows, catalog synchronization, and upgrades remain assigned to
their later I-04 steps. Those workflows must use the Step 2 repository/service
boundaries and preserve the immutability constraints established here.
