# Curriculum Compiler domain

## Purpose and boundary

`internal/curriculum` is Kelyro's persistence- and presentation-neutral
language for describing source-backed curriculum definitions, compiler inputs
and outputs, and Learning Pack metadata. It is one cohesive package split into
files by area. This avoids premature package cycles while future application
services reveal useful boundaries.

The package depends only on the Go standard library. It does not import Bubble
Tea, SQLite, YAML, Research adapters, operating-system APIs, AI providers, or
Student Core. It validates domain data but does not fetch evidence, compile a
graph, persist a pack, mutate learner mastery, or generate lesson/exercise
content.

```text
CLI / TUI / pack and storage adapters
                 |
                 v
future curriculum application services
                 |
                 v
       internal/curriculum
          /             \
I-03 Source Bundle refs  compiled I-02 hand-off
```

I-03 remains the authority for evidence and Source Bundle readiness. I-04 owns
curriculum structure and pack identity. I-02 remains the authority for learner
instances and progress.

## Identity, time, and versions

- `ID` is a stable opaque identity for domain records.
- `CurriculumID` is stable across immutable curriculum versions.
- `ConceptID` identifies the same independently assessable knowledge unit
  across compatible versions. Titles and hierarchy positions are not identity.
- `Timestamp` is non-zero and UTC. Constructors normalize input to UTC; entity
  validation rejects raw non-UTC values.
- `CurriculumVersion` is opaque because the compiled definition policy is not
  required to be SemVer.
- `PackVersion` accepts strict SemVer 2.0 syntax without a leading `v`. This
  only validates version identity; Step 35 owns compatibility classification
  and prerelease policy.

Zero-value identities, timestamps, and versions are invalid.

## Source evidence boundary

The domain never imports or embeds I-03 records. `SourceBundleRef` freezes the
complete version identity defined by the I-03 hand-off contract:

```text
(bundle ID, sha256 content hash, algorithm version, verified-at UTC instant)
```

`EvidenceRef` points to a Claim inside one declared bundle. Validation rejects
evidence that names an undeclared bundle. Production definitions use the
`required` source-reference policy. The explicit `optional_for_fixture` policy
exists only so deterministic development fixtures remain possible; it does not
make unevidenced production compilation valid.

This boundary carries structured identities, never raw web bodies or network
authority.

## Main model

```text
LearningGoalSpec
  └── GoalOutcome
        ^
        |
CompetencyMatrix ── Competency
                         |
                         v
CurriculumDefinition ── Concept ── Prerequisite ── Concept
        |                  ^                         |
        |                  └──── VocabularyGraph ───┘
        |
        ├── Phase → Module → LessonSpec → TopicSpec → ConceptID
        ├── CoverageRequirement
        └── SourceBundleRef → EvidenceRef

CompilationInput + CompilationConfig
        ↓ future versioned passes
CompilationResult + CompilationPass + CoverageResult + Gap

LearningPack
  ├── PackManifest + PackVersion + PackDependency
  ├── CurriculumDefinition
  └── optional EnvironmentPack + ToolRequirement
```

`LearningGoalSpec` is learner-neutral compiler input and must not be confused
with I-02's learner-owned `LearningGoal` lifecycle. Similarly, a
`CurriculumDefinition` is not a personalized path or a `CurriculumInstance`.
Every `GoalOutcome` declares one professional coverage category. Goals with a
`ProfessionalRole` additionally cover the five explicit capabilities explain,
build, debug, operate and maintain; see
[professional-outcomes-v1.md](professional-outcomes-v1.md).

## Closed vocabularies

Concept and pack status is one of:

```text
current | experimental | preview | legacy | historical | deprecated
```

Atomicity is `atomic`, `needs_split`, `too_fragmented`, or `unknown`.
Difficulty uses the general 1–5 introductory-to-expert scale. Competency levels
are `awareness`, `understand`, `apply`, `analyze`, `design`, `operate`, and
`teach_explain`.

Competencies bind a stable area ID, one outcome and an expected level. Optional
pack-defined dimensions carry their own expected level, and optional parent IDs
form an acyclic hierarchy within one area. The production builder and coverage
rules are defined in [competency-matrix-v1.md](competency-matrix-v1.md).

Outcome categories are `knowledge`, `application`, `debugging`, `design`,
`tool_usage`, `production`, `security`, `communication_documentation`, and
`maintenance`. Professional capabilities are `explain`, `build`, `debug`,
`operate`, and `maintain`.

Prerequisite kinds are `hard`, `recommended`, `exposure_only`,
`tool_dependency`, and `vocabulary`. These describe graph edges; visual order
does not imply a prerequisite.

Coverage, gap, tool-requirement, change, and migration types are also closed.
Unknown serialized values must be rejected instead of guessed.

## Structural invariants

`CurriculumDefinition.Validate` enforces:

- valid stable IDs and no duplicate concept or hierarchy-node IDs;
- a matrix bound to the same goal, competencies bound to known outcomes, and
  concept references bound to known concepts;
- no self-prerequisite, duplicate prerequisite edge, or graph edge to a
  missing concept;
- vocabulary canonical/introduction/use references to known concepts;
- a strict `Phase → Module → LessonSpec → TopicSpec → Concept` hierarchy with
  valid parents, unique sibling order, unique concept placement, and no
  omitted or dangling concept;
- coverage targets that exist in the definition;
- valid closed statuses and other enums;
- required or explicitly fixture-optional Source Bundle refs, and no evidence
  ref to an undeclared bundle;
- non-zero UTC timestamps and syntactically valid pack versions.

There is no maximum number of phases, modules, lessons, topics, concepts, or
edges. Hierarchy is the display structure; prerequisite and vocabulary graphs
remain separate pedagogical structures.

Cycle detection, topological ordering, atomization decisions, coverage
algorithms, and compiler-pass hashing are deliberately not implemented here;
their versioned behavior belongs to later I-04 steps.

## Pack and change boundaries

`LearningPack`, `PackManifest`, `PackDependency`, `EnvironmentPack`, and
`ToolRequirement` are domain shapes represented by the portable v1 format.
Dependency resolution belongs to Step 36.

An environment pack contains declarative tool requirements only. The base
model has no scripts, installers, or secrets, and tool evidence/introduction
references must resolve against the containing Learning Pack.

`CurriculumChange` records old/new definition versions, a closed change kind,
affected concept IDs, rationale, and a migration class. It does not apply a
migration or write Student Core state. Change classification and student-safe
migration planning remain later, separately versioned policies.

## Explicitly deferred

Step 1 does not implement repositories, application services, SQLite
migrations, YAML/JSON adapters, pack archives, loaders, dependency resolution,
compiler passes, audits, CLI/TUI handlers, I-02 conversion, learner migration,
I-05 lessons/practice/assessments, or AI behavior.
