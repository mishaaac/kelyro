# Knowledge graph and prerequisite engine

## Boundary

Step 9 turns the prerequisite declarations in a validated `Curriculum` into a
read-only in-memory `KnowledgeGraph`. It answers graph and introduction-policy
questions; it does not calculate mastery, write concept state, instantiate a
curriculum, or implement exercises.

The graph has no repository, SQLite, YAML, CLI, TUI, or operating-system
dependency. `application.PrerequisiteService` is the orchestration boundary: it
resolves the effective threshold through `MasteryPolicyService`, loads all
concept states for the current student once, builds a validated snapshot, and
then delegates every traversal and decision to the domain graph.

## Graph direction and operations

A declaration that concept `B` requires concept `A` creates the knowledge edge
`A → B`. The visible curriculum hierarchy and its display order never create
knowledge edges.

`KnowledgeGraph` provides:

- `GetPrerequisites(concept)`: direct prerequisite declarations and their
  `introduced`/`mastered` requirements;
- `GetDependents(concept)`: concepts directly enabled by the concept;
- `Ancestors(concept)`: all transitive prerequisites, foundations first;
- `CanIntroduce(concept, snapshot, policy)`: the boolean policy result;
- `MissingPrerequisites(concept, snapshot, policy)`: failed direct checks with
  their inputs and reason;
- `TopologicalOrder()`: every concept in prerequisite-safe order.

All returned collections are defensive copies. Unknown concepts produce the
stable `ErrUnknownCurriculumConcept` cause.

## Versioned introduction policy

Policy `prerequisite-v1` evaluates every direct prerequisite with logical AND:

```text
concept can be introduced = every direct prerequisite is satisfied
```

A concept with no prerequisites can be introduced. Missing student state blocks
either requirement and is reported explicitly.

The two requirement modes remain independent:

```text
introduced satisfied = exposure_state != not_seen
mastered satisfied   = calculated_mastery >= resolved_threshold
```

An `introduced` edge ignores the numeric mastery score. A `mastered` edge uses
the calculated score and ignores the exposure label. This preserves the domain
invariant that exposure lifecycle and evidence-derived mastery are separate
dimensions; for example, deterministic diagnostic evidence may satisfy mastery
without pretending the learner has already traversed lesson presentation.

Mastery comparison delegates to `threshold-v1`, including its inclusive
boundary. Step 9 never calculates mastery from evidence and never duplicates
threshold presets or precedence. The decision retains both
`prerequisite-v1` and the complete resolved mastery policy/source for audit.

## Explainability

`EvaluateIntroduction` returns an `IntroductionDecision` with one ordered
`PrerequisiteCheck` per direct edge. Each check records:

- stable concept ID and display title;
- declared requirement;
- whether student state was present;
- observed exposure and mastery;
- required mastery where applicable;
- satisfied result and stable reason code.

Stable reasons distinguish satisfied exposure, satisfied mastery, missing
state, not introduced, and mastery below threshold. `Explanation()` renders a
human-readable summary such as:

```text
Memory model is locked.

Required:
✓ Variables — 91% (requires 85%)
✓ Functions — introduced (learning)
✗ Addresses — 63% (requires 85%)
```

Presentation adapters may render the structured checks differently without
reimplementing the educational decision.

## Determinism and complexity

Construction indexes concepts, direct prerequisites, and reverse dependents.
Dependents and direct prerequisites are sorted by stable ID. Kahn's algorithm
uses a stable-ID priority queue, so multiple valid topological orders always
resolve to the same result independent of map iteration, YAML order, titles, or
display hints.

For `V` concepts and `E` prerequisite edges:

- construction and topological ordering: `O(V + E log V)` time, `O(V + E)`
  memory;
- direct prerequisite/dependent lookup: `O(1)` index lookup plus output size;
- `Ancestors`: `O(V + E)` worst case;
- introduction evaluation: `O(d)`, where `d` is the number of direct
  prerequisites.

Cycles are rejected both by curriculum validation and by topological graph
construction. Tests cover chains, diamonds, multiple requirements, missing
state, exact threshold boundaries, cycles, deterministic ordering, and a
3,000-concept chain.

## Persistence rule

Graph traversal never queries a repository. The application service performs
one `ListByStudent` call per evaluation and passes the complete validated
`StudentStateSnapshot` to the graph. This avoids N+1 reads and keeps algorithms
testable without SQLite.

Step 10 remains responsible for durable curriculum instances, their goal/source
ownership, and learner-state isolation. Step 9 does not add migrations or infer
that lifecycle.
