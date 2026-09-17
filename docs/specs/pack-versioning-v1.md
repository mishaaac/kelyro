# Learning Pack Versioning Policy v1

## Purpose

Learning Packs use their own version line. A pack version is independent of
the Kelyro application version, curriculum definition version, schema version,
dependency constraints, and Concept IDs. Updating Kelyro never implicitly
updates a pack, and publishing a pack never moves an application release.

The policy algorithm is `pack-versioning-policy-v1`. Pack version identities
use strict SemVer 2.0 syntax without a leading `v`.

The policy, change classifier and safe-upgrade integration are complete in
Kelyro `v0.3.0-alpha.1`.

## Immutable publication rule

A published `(pack_id, version)` is immutable. Corrections always produce a
new version with greater SemVer precedence. Maintainers must never overwrite a
published archive, move its release tag, or use build metadata alone to replace
its content. Checksums and catalog metadata must continue to identify the
original artifact.

## Change impact

Every release classifies all `CurriculumChange` records before selecting a
version. The highest impact wins.

| Impact | Changes |
| --- | --- |
| PATCH | Source refreshes, typo/metadata fixes, and non-structural clarifications that are migration-safe. |
| MINOR | Added Concepts, compatible new lessons/hierarchy, expanded coverage, status/environment changes, or changes requiring recompilation without learner review. |
| MAJOR | Removed, split, or merged Concept identities; incompatible curriculum contracts; restructuring that requires learner review; or any explicitly breaking migration. |

Migration classification is authoritative when it is stricter than the change
kind: `requires_recompile` raises an otherwise PATCH change to MINOR, while
`requires_student_review` and `breaking` require MAJOR impact.

The policy requires the next exact core component and resets lower components:

```text
PATCH  1.2.3 -> 1.2.4
MINOR  1.2.3 -> 1.3.0
MAJOR  1.2.3 -> 2.0.0
```

Skipping components or over-versioning is rejected so the version communicates
the classified compatibility contract. A release containing mixed changes uses
the highest classification rather than separate bumps.

## `0.x` semantics

Pack APIs and Concept contracts may still evolve while the major version is
zero, but breaking changes remain explicitly classified as MAJOR impact. While
remaining on `0.x`, that impact opens the next minor line:

```text
breaking: 0.4.2 -> 0.5.0
```

PATCH and compatible MINOR behavior remains conventional:

```text
patch: 0.4.2 -> 0.4.3
minor: 0.4.2 -> 0.5.0
```

The decision artifact preserves `ChangeImpact=major` and
`RequiredTransition=minor`, so consumers do not mistake the zero-major syntax
for compatibility. A maintainer may instead declare the stable breaking line
with the next major (`0.4.2 -> 1.0.0`); stabilization is never automatic.

## Prerelease semantics

A prerelease suffix marks an unstable iteration of an already selected core
release line, for example `0.5.0-alpha.1`. Once the line has been selected from
the classified changes, later artifacts may advance SemVer prerelease
precedence or remove the suffix:

```text
0.5.0-alpha.1 -> 0.5.0-alpha.2 -> 0.5.0-beta.1 -> 0.5.0
```

Each iteration is a distinct immutable publication. Equal or decreasing
precedence is rejected. A core-line change still has to satisfy PATCH, MINOR,
MAJOR, or the `0.x` rule; adding a prerelease suffix does not weaken that rule.

## Deterministic decision artifact

`PackVersioningService` accepts current/candidate pack versions and already
classified `CurriculumChange` values. It returns:

- one canonical classification per change;
- the aggregate change impact;
- required and actual version transitions;
- an allowed/rejected decision with reasons;
- the algorithm version.

Input ordering does not change the decision. The service has no catalog,
filesystem, Git tag, installer, migration, network, or student-state authority.
Change detection, migration planning, upgrade execution, and catalog publishing
remain separate I-04 services and never become implicit powers of this policy.
