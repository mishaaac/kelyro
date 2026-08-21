# Warm-up Selector v1

Step 20 adds a contextual, read-only selector for small reviews before a new
lesson. `warm-up-selector-v1` chooses concepts and explains each choice. It
does not generate questions, execute exercises, record Evidence, or alter the
review backlog.

## Inputs and scope

The pure policy receives:

- the candidate lesson identity, curriculum version, and direct prerequisite
  concept IDs;
- concepts from that curriculum;
- pending reviews already due at the injected UTC instant;
- durable mistake projections;
- remaining session minutes; and
- optional recent warm-up concept IDs, newest first.

The application service loads due reviews and mistakes from their existing
repositories and filters them to the candidate curriculum. This keeps the
selector contextual when a learner has durable state from more than one
curriculum. The caller supplies the lesson context; choosing today's lesson is
reserved for the adaptive daily-plan policy in Step 24.

## Eligibility and priority

A concept is eligible when it has either a pending due review or at least two
occurrences of an unresolved mistake pattern. A prerequisite without either
signal is not added merely to fill time, so a valid plan may be empty.

Candidates use this stable priority tuple:

```text
1. candidate-lesson prerequisite with a due review
2. candidate-lesson prerequisite with a repeated unresolved mistake
3. other due review in the candidate curriculum
4. other repeated unresolved mistake in the candidate curriculum
```

Within one level, concepts absent from recent warm-ups come first. If all were
recent, the least recently selected comes first. Earlier due time, higher
mistake occurrence count, newer mistake observation, and finally stable
concept ID provide deterministic tie-breaking. A concept carrying both review
and mistake signals appears only once; its explanation mentions the secondary
signal.

## Time policy

Each selected concept receives a five-minute contextual recall estimate. The
warm-up budget is:

```text
floor_to_5(min(15 minutes, available_minutes / 3))
```

This leaves time for the candidate lesson, caps long-session warm-ups at 15
minutes, and returns an empty plan when fewer than 15 minutes are available.
The application service additionally caps caller-supplied remaining time at
the learner's configured daily availability.

The fixed estimate is scheduling metadata only. I-05 owns the concrete review
exercise and its actual outcome.

## State and boundaries

Warm-up selection is a projection over existing durable facts. Step 20 adds no
migration, table, cache, command, or TUI state. Recent selections are explicit
ephemeral caller input; a later daily-plan or session consumer may persist its
own auditable plan without changing this policy.

The clock is injected at the application boundary and normalized to UTC. The
domain result carries the lesson, selected concepts, reasons, priorities,
estimated minutes, budget, generation instant, and `warm-up-selector-v1`
version so future policies can coexist without reinterpretation.
