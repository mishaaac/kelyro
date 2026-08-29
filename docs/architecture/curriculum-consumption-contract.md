# Curriculum consumption contract

## Boundary

I-02 consumes a deterministic curriculum definition. It does not discover,
research, generate, or compile curriculum content.

```text
I-03 Research Engine       investigates and verifies sources
I-04 Curriculum Compiler  compiles and versions real Learning Packs
I-02 Student Core         validates and consumes the resulting contract
```

The `foundation-demo` data under `testdata/curricula` is a development fixture,
not a researched pack and not a claim about an official learning path.

The consumption contract itself does not persist a learner-specific curriculum
instance, calculate mastery, traverse prerequisites, or decide unlocks. The
completed I-02 implementation supplies those responsibilities through the
separate Curriculum Instance, mastery, graph, and progression boundaries, so
the reusable curriculum definition remains learner-neutral.

## Contract version and identity

The first supported representation is `curriculum-consumption/v1`. A definition
has a stable curriculum ID and an explicit curriculum version. Every node also
has a stable ID and its own version. Titles and descriptions are display data and
must never be used as identity.

An unsupported contract version is rejected rather than guessed. Loading a
fixture therefore cannot silently reinterpret data after this contract evolves.

## Visible hierarchy and knowledge graph

The visible hierarchy is strict:

```text
Curriculum
└── Phase
    └── Module
        └── Lesson
            └── Topic
                └── Concept
```

The serialized model is flat: each non-phase node names its parent. This keeps
identity and references explicit while allowing consumers to render the nested
shape. `order` is a zero-based display hint that must be unique among siblings;
it is not a prerequisite and never unlocks knowledge.

The real knowledge graph is declared by concept prerequisites. Each edge names
another concept and records one of two requirements:

- `introduced`: exposure is sufficient;
- `mastered`: the configured mastery policy must be satisfied.

This contract validates these declarations only. The versioned behavior is
implemented separately by the prerequisite engine described in
`knowledge-graph-prerequisite-engine.md`.

## Node contract

Every node contains:

- stable `id` and one of `phase`, `module`, `lesson`, `topic`, or `concept`;
- non-empty `title` and `description`;
- non-negative sibling `order` plus optional `display.short_title` and
  `display.hidden` hints;
- status metadata with `draft`, `active`, or `deprecated` state and an optional
  explanatory note;
- explicit node `version`.

A concept additionally declares:

- one or more objectives;
- zero or more typed prerequisites;
- difficulty on the documented general scale 1–5: introductory, foundational,
  intermediate, advanced, and expert;
- positive estimated effort in minutes;
- whether theory is required before practice;
- one or more assessment expectations.

Assessment expectations describe future evidence needs. They do not contain
generated questions, evaluators, or Exercise Engine behavior.

## Validation

Construction rejects:

- duplicate stable IDs or duplicate prerequisites;
- unknown node types, invalid statuses, and incomplete concept metadata;
- missing parents, invalid parent types, hierarchy cycles, and self-parenting;
- negative order values or duplicate sibling order values;
- prerequisites that point to absent or non-concept nodes;
- self-prerequisites and prerequisite cycles;
- unknown prerequisite requirement modes.

There is no maximum number of phases, modules, lessons, topics, concepts, or
edges. The deterministic large-graph fixture test validates a 1,500-concept
chain to guard against accidental product limits.

## Deterministic loading

`internal/infra/curriculumyaml.Load` accepts an `io.Reader`, strictly decodes one
YAML document, and then constructs the domain contract. It rejects unknown YAML
fields, duplicate mapping keys, empty input, and additional YAML documents.

The constructor copies input slices and canonicalizes nodes by hierarchy type,
parent, display order, and stable ID. Prerequisites are canonicalized by stable
ID. Equivalent source order therefore yields the same in-memory definition;
objectives and assessment expectations retain their authored order because that
order is meaningful to readers.

## YAML dependency

The adapter uses the pinned pure-Go dependency `go.yaml.in/yaml/v3 v3.0.5`.
The standard library has no YAML decoder, and treating a `.yaml` fixture as JSON
would not implement the declared contract. Version 3 was selected because it is
the current stable line and provides strict known-field decoding; version 4 was
still release-candidate software when this contract was implemented.

The dependency is isolated from `internal/learning`: the educational domain
continues to depend only on the Go standard library, and future fixture, import,
or pack adapters can feed the same validated domain constructor.

The future research-driven update boundary must preserve these immutable
identity and fingerprint rules. Its selective migration and student-state
safety requirements are defined in
[research-to-curriculum-update-contract.md](research-to-curriculum-update-contract.md).
The research inputs and fail-closed eligibility gate used by the future
compiler are defined in
[source-driven-compiler-contract.md](source-driven-compiler-contract.md).
