# Current, Experimental, and Legacy Classification v1

`temporal-classification-v1` transfers temporal intelligence frozen in
I-03-derived Claims and source authority into UI-ready Concept and lesson
metadata.

## Status derivation

The closed statuses are `current`, `preview`, `experimental`, `legacy`,
`historical`, and `deprecated`. A deprecation Claim yields `deprecated`; a
historical Claim or evidence supported only by historical/archived sources
yields `historical`; otherwise the preserved Claim status applies.

When evidence has mixed statuses, v1 uses safety precedence:

`deprecated > historical > legacy > experimental > preview > current`.

The result records mixed evidence and any override of the author-declared
status as explicit reasons. Consequently historical or deprecated evidence can
never remain silently labeled current.

## Concept and lesson metadata

Concept classification reads the Concept's exact evidence refs. Lesson input
declares its Concepts plus optional direct evidence; lesson status aggregates
both. This does not add `Status` to the immutable `LessonSpec` before the
hierarchy builder. Instead, a separate classification artifact is ready for UI
grouping and later compiler assembly.

- current material is eligible as the primary recommendation;
- preview and experimental material is separated;
- legacy and historical material is context-only;
- deprecated material is neither primary, separated experimentation, nor
  context-enabled guidance.

A contextual-use flag records whether a legacy/historical Concept or lesson was
actually declared for maintenance/history. Context-only material without that
declaration remains visible as a diagnostic.

## Determinism and boundaries

Evidence refs are deduplicated and sorted; targets sort by kind and stable ID.
The classifier performs no research, hierarchy construction, lesson content
generation, UI rendering, mutation of published curricula, or student-state
mutation.
