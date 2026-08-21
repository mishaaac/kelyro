# Non-punitive Study Streak v1

Step 21 represents study consistency as optional learner information.
`streak-v1` is a deterministic projection over local Study History; it never
changes mastery, prerequisites, progression, review eligibility, achievements,
or access to content.

## Active-day policy

The default, configurable policy counts one local calendar date when either:

- completed/active Study Sessions assigned to that date accumulate at least
  10 minutes of intentional `study-session-v1` active time; or
- Study History contains a completed educational fact: diagnostic completion,
  concept introduction, evidence recording, concept mastery, or review
  completion.

The threshold is whole minutes in `StreakPolicy` and the default policy is
identified by `streak-v1`. Multiple short sessions on the same date are added
together. Multiple events on one date still produce one active day.

Opening Kelyro or the TUI, completing onboarding, unlocking an achievement, or
creating/stopping an empty session does not count. A `session.completed` event
does not bypass the time threshold; the bounded `ActiveDuration` of its source
session remains authoritative.

Study-session state does not preserve a full interval-by-interval allocation
across midnight. Consistent with `time-tracking-v1`, each session is assigned
to the local date of its terminal end, or its last meaningful activity while
active. The session's already idle-bounded active duration is never inferred
from wall time.

## Calculation

The policy sorts unique active local dates and calculates maximal consecutive
calendar runs. Calendar adjacency uses `AddDate`, not a fixed 24-hour duration,
so 23-hour spring days and 25-hour fall days remain consecutive.

The materialized result contains:

```text
current_days
longest_days
last_active_local_date
total_active_days
last_study_at (UTC audit instant)
timezone
minimum_active_minutes
policy_version
```

`current_days` is the final run when the last active date is today or yesterday
in the learner timezone. Yesterday remains current throughout today, giving
the learner the whole local day to continue. If the last active date is older,
current becomes zero while longest and total history remain intact.

## Timezone changes and recalculation

Every read through `StreakService.Show` reloads all durable Study History and
Study Sessions, recalculates with the profile's current IANA timezone, and
atomically replaces the materialized row. This repairs stale, legacy, or
otherwise inconsistent projections without an incremental counter.

Changing timezone may merge or split UTC observations near local midnight;
that is the truthful calendar projection for the newly selected timezone.
It cannot permanently add duplicate days: events are deduplicated by local
date on every full recalculation, and changing back restores the prior
projection from the same immutable facts. The captured timezone makes the
meaning of the stored local date explicit.

## Persistence and compatibility

Forward-only migration v18 extends the existing `streak_state` table with the
local date, total active days, timezone, threshold, and policy version. Rows
from the published v4 schema remain readable as `legacy-streak/v0`; the next
service read replaces them with a complete `streak-v1` projection.

SQLite checks and triggers mirror aggregate consistency, while date grouping,
activity qualification, DST handling, and consecutive-run calculation remain
pure domain logic. The in-memory adapter applies the same domain validation.

## Presentation

`kelyro streak` and the TUI Study consistency view use neutral wording such as
`Streak: 6 days`, show longest/total context, and state that the number does not
change mastery or block learning. There are no warnings, loss messages,
recovery purchases, notifications, or guilt language.
