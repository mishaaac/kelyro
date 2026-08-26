# Last Verified and Refresh Scheduling v1

Step 17 adds the pure `refresh-scheduling-v1` policy beside `freshness-v1` in
`internal/research/freshness`. It determines when an already verified source or
claim must be checked again. It does not perform verification, network access,
release discovery, conflict resolution, or background execution.

## Scheduling contract

A persisted scheduled record keeps these dates and decisions distinct:

- `last_verified_at`: the exact verification timestamp consumed by
  `freshness-v1`;
- `next_verify_at`: the deadline at or before which re-verification is due;
- `verification_reason`: the primary deterministic reason for that deadline;
- `priority`: `normal`, `high`, or `critical` queue priority; and
- `scheduling_algorithm_version`: immutable `refresh-scheduling-v1`.

Scheduling requires a validated, known `freshness-v1` assessment. An `unknown`
assessment has no real `last_verified_at`, so the scheduler rejects it instead
of inventing a date. `FreshnessRecordFromSchedule` verifies that the assessment
and schedule use the same last-verification timestamp before combining them.

## TTL and event triggers

Without an event trigger, the next deadline is exactly:

```text
last_verified_at + effective freshness TTL
```

The stored reason is `ttl_expired`, including while that deadline is still in
the future; the record becomes due only when `next_verify_at <= as_of`.

Event triggers are immediately due at the assessment time. Their deterministic
precedence and priority are:

| Precedence | Trigger | Stored reason | Priority |
| ---: | --- | --- | --- |
| 1 | manual request | `manual_request` | critical |
| 2 | security-sensitive review | `security_sensitive` | critical |
| 3 | unresolved conflict | `conflict_unresolved` | high |
| 4 | source changed after verification | `source_changed` | high |
| 5 | relevant new release detected | `new_release_detected` | high |
| 6 | TTL deadline | `ttl_expired` | normal |

Source-change and release triggers are derived from validated
`freshness-v1` reason codes. Conflict, security, and manual signals are explicit
scheduler inputs. When multiple signals exist, v1 stores the highest-precedence
primary reason rather than silently depending on input order.

## Persistence and due queries

Forward-only SQLite migration v30 adds one bounded, checked scheduling metadata
object to `freshness_state`. It contains the closed reason and priority plus the
immutable algorithm version. Previously scheduled rows are conservatively
classified as normal TTL deadlines; unscheduled Step 16 rows remain valid and
the metadata object is inactive until `next_verify_at` exists.

Both memory and SQLite adapters return only records due at the caller-provided
UTC timestamp. Results are ordered by critical/high/normal priority, then due
time, then stable subject ID. This ordering is deterministic and works offline.

## CLI and execution boundary

`kelyro sources stale` opens the workspace-local database, evaluates due state
with an injected clock, and renders source or claim subject IDs with priority,
reason, deadline, and last-verification date. An empty queue is a successful,
explicit result.

The command is read-only and never uses the network. Step 17 creates scheduling
state and inspection only: it starts no daemon, timer, goroutine, automation,
fetch, verification run, or curriculum migration.
