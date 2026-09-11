# Atomic concept criteria v1

`atomic-concept-criteria-v1` formalizes whether a knowledge unit is small and
complete enough to become an independently learner-trackable Concept. It is a
pure domain policy: it has no network, storage, UI, AI or domain-vocabulary
dependency.

## Criteria

Each candidate is assessed with explicit `satisfied`, `unsatisfied` or
`unknown` signals for whether it:

- is named;
- is defined;
- has an explicit prerequisite boundary, including an explicit foundational
  boundary;
- can be explained independently;
- can be practiced independently;
- can be assessed independently;
- has evidence;
- has meaningful standalone learning value.

The input also lists any independently assessable parts. Domain profiles or
later compiler inputs provide these signals; the core does not infer them from
titles such as "Go Programming", "Backend Development" or "Databases".

## Result and precedence

The policy emits exactly one closed `Atomicity` result and stable reasons:

1. More than one independently assessable part is `needs_split`.
2. An explicitly non-standalone, non-explainable, non-practicable or
   non-assessable unit is `too_fragmented`.
3. Any other incomplete, unknown or unsatisfied criterion is `unknown`.
4. All criteria satisfied, with no evidence of multiple parts, is `atomic`.

This precedence prevents incomplete metadata from silently certifying a broad
candidate and prevents a tiny fragment from being accepted merely because it
has a name. Assessments include `atomic-concept-criteria-v1` and at least one
machine-stable reason. The policy does not split candidates; that belongs to
Atomizer v1.
