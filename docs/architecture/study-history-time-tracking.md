# Study History and time tracking

Step 17 adds a learner-facing timeline and calendar-aware summaries of
intentional study time. Both remain local to the workspace and are separate
from the Foundation Audit Trail.

## Learner events

`StudyEvent` is an immutable fact under the versioned contract
`study-history-v1`. The supported vocabulary is:

```text
onboarding.completed
diagnostic.completed
concept.introduced
evidence.recorded
concept.mastered
review.completed
session.completed
achievement.unlocked
```

Every event has an opaque event ID, student, semantic source ID, UTC occurrence
time, and optional goal, Curriculum Instance, and concept scope. The semantic
key `(student, event type, source)` makes retries idempotent while rejecting a
different event that attempts to reuse the same origin.

This is not a technical audit log. It contains educational facts a learner can
recognize, and does not copy command execution, migration, privacy-denial, or
system diagnostic records from I-01. Evidence/progression, diagnostic
completion, and explicit session completion record their events in the same
transaction as the originating change. Onboarding completion is retry-safe
through the semantic key because its existing service boundary spans profile,
goal, and onboarding repositories.

The forward-only migration v15 creates `study_history_events` and backfills
facts that already have authoritative timestamps. It does not invent missing
goal, instance, or concept ownership. Review and achievement facts already in
the published Student Core schema are projected during upgrade; their future
live lifecycle services can use the same event contract without implementing
those later policies in Step 17.

## Ordering and calendar filtering

Storage is always RFC3339Nano UTC. Timeline queries use an inclusive start,
exclusive end and return newest first, with ID as the deterministic tie-breaker.
`history --today` constructs the learner's local midnight boundaries from the
IANA timezone saved in the profile, then converts those instants to UTC for the
query. CLI timestamps are converted back to that timezone for display.

Calendar windows use:

- today: local midnight through the next local midnight;
- week: Monday local midnight through the next Monday;
- month: first local day through the first day of the next month.

They use calendar arithmetic rather than fixed 24-hour durations. A DST spring
day can therefore be 23 hours and a fall day 25 hours without changing stored
UTC facts.

## Time policy v1

`time-tracking-v1` consumes `StudySession.ActiveDuration` from
`study-session-v1`; it never reconstructs raw wall time or idle time. A
terminal session is assigned to the local calendar period containing its end
timestamp. An active session uses its last meaningful activity as the current
anchor. Future-dated anchors are excluded.

The summaries report today, current week, current month, and all-time duration
and session count. The legacy v4 session history remains visible in the
timeline after migration, but is not included in time totals because it lacks
the bounded `active_duration` and Curriculum Instance ownership required by
this policy.

Concept/module attribution is deliberately conservative. The policy examines
concept-scoped educational events in the same Curriculum Instance between the
session start and anchor:

- exactly one unique concept receives the whole active duration;
- exactly one unique ancestor module across the observed concepts receives the
  whole active duration;
- ambiguous or absent evidence produces no breakdown instead of an invented
  split.

Module lookup follows the installed generic curriculum hierarchy
`concept → topic → lesson → module`; it does not assume a programming language
or subject.

## CLI and privacy

The presentation-neutral service backs:

```text
kelyro history
kelyro history --today
kelyro time
```

The timeline and totals never capture keystrokes, mouse movement, app
presence, external-editor activity, exercise payloads, or telemetry. No data
leaves the workspace. `streak-v1` now consumes these durable facts without
changing them; retention, review scheduling, achievement policy, analytics,
daily planning, and the full Exercise Engine remain separate policies.
