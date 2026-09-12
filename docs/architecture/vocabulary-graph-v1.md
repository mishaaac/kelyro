# Vocabulary Graph v1

`vocabulary-graph-v1` builds a canonical vocabulary dependency artifact from
explicit term definitions, observed uses, Concepts, and an optional domain
baseline. It does not extract words from prose and has no global list of terms
that learners are presumed to know.

## Declarations and resolution

Each declared term identifies its canonical Concept, the Concept that
introduces it, aliases, and scope. Every observed use identifies the spelling
seen and the Concept where it occurs. Resolution is case-insensitive and
collapses internal whitespace, while the declared spelling remains in output.

Acronyms use the same explicit alias mechanism as long names. For example,
`API` can resolve to `application programming interface` only when that mapping
is present in the input. Canonical terms and aliases occupy one namespace;
collisions are rejected rather than guessed from scope or context.

The resulting `VocabularyGraph` contains canonical terms and stable, sorted
`UsedBy` Concept IDs. `ResolvedVocabularyUse` retains both the observed and
canonical spelling so later audits can explain alias/acronym use. Repeated
spellings that resolve to the same term at the same Concept are coalesced.

## Explicit domain baseline

`DomainVocabularyBaselineTerm` is the only exception for assumed common terms.
Every entry requires a term, scope, and reason and may declare aliases. A use
that has neither a vocabulary declaration nor a matching baseline entry is an
invalid input. Baseline entries are carried into the compiled artifact and
cannot collide with declared terms or each other.

This pass verifies that canonical, introduction, and use Concept IDs exist. It
does not decide whether the introduction precedes each use; that is the
Definition-before-use Audit in Step 18.

## Determinism

Definitions, aliases, baseline entries, uses, and `UsedBy` IDs are
canonicalized by normalized term and stable Concept ID. Input order therefore
does not affect `VocabularyGraphCompilation`. The algorithm identity is
`vocabulary-graph-v1`.
