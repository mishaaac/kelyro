# Student Core command-line interface

Step 27 exposes the Student & Learning Core without entering the full-screen
terminal experience. The CLI remains a presentation adapter: it discovers the
workspace through the application service, requests Student Core services or
the shared `progress-dashboard/v1` read model, and renders plain text. It does
not open SQLite or reproduce mastery, progression, prerequisite, retention,
analytics, or daily-plan policy.

## Command surface

The minimum read surface is:

| Command | Application source | Default result |
| --- | --- | --- |
| `profile` | `ProfileService` | persistent learner profile |
| `goal` | `GoalLifecycleService` | active and historical goals |
| `status` | `ProgressDashboardService` | goal, current location, effective mastery requirement, and concept counts |
| `progress` | `ProgressDashboardService` | completion, known mastery, reviews, study time, streak, and weak concepts |
| `progress export` | `ProgressDashboardService` plus the ownership-aware artifact store | explicitly regenerated human-readable learning documents |
| `roadmap` | `ProgressDashboardService` | resolved hierarchy, concept status, mastery when known, and lock reasons |
| `history` | `StudyHistoryService` | learner-facing durable study events |
| `time` | `StudyHistoryService` | intentional active study time |
| `reviews` | `ReviewSchedulerService` | scheduled review queue |
| `mistakes` | `MistakeMemoryService` | persistent mistake memory |
| `today` | `ProgressDashboardService` | persisted `daily-plan-v1` selection and explanations |
| `mastery` | `MasteryPolicyService` | effective progression threshold |
| `maintenance recalculate` | `MaintenanceRecalculationService` | advanced dry-run or backup-protected derived-state recalculation |

`profile`, `goal`, and `mastery` use their read operation when no nested
operation is supplied. Their existing explicit forms (`profile show`, `goal
show`, and `mastery threshold`) remain compatible. Write operations such as
profile edit, goal lifecycle changes, and mastery threshold changes retain
their explicit syntax.

`roadmap` now means the resolved Student Core roadmap. The Foundation artifact
is still available through `kelyro open roadmap`, so the dynamic learning view
and the human-owned/editor workflow are unambiguous.

## Shared read model and rendering

`status`, `progress`, `roadmap`, and `today` dispatch distinct application
actions, but all four are coordinated by `executeDashboard` and receive one
coherent `ProgressDashboard`. This keeps their facts aligned with the TUI while
allowing each command to render only the relevant subset.

Output is human-readable plain text. Unknown average mastery is written as
`unknown`; it is never presented as a measured `0%`. Roadmap status uses text
labels (`mastered`, `current`, `available`, `locked`, `review due`) and prints
application-supplied lock reasons. Today replaces stable concept IDs in plan
explanations with curriculum titles when the dashboard supplies them. No JSON
contract is introduced.

## Empty states and process behavior

An initialized workspace with no active learning goal is a successful read and
returns setup guidance. A missing active curriculum or daily plan is likewise
rendered explicitly instead of fabricating progress. Workspace discovery,
storage, or read-model failures go to standard error and return exit code `1`;
invalid command syntax returns `2`; successful reads, including incomplete
setup states, return `0`.

Every explicit subcommand bypasses the interactive adapter. `--workspace`
continues to flow through `app.Command` and therefore uses the same Foundation
workspace discovery semantics as existing commands. Global and command-scoped
`--help` return before application dispatch.

## Boundaries

Step 27 added no JSON primary interface, schema migration, algorithm
recalculation command, Exercise Engine, Research Engine, Curriculum Compiler,
AI provider, plugin, generated learning content, or network activity. Step 28
later added only the explicit Markdown progress export documented in
`student-learning-markdown-artifacts.md`; the remaining boundaries are
unchanged. Step 29 adds the advanced local maintenance command documented in
`versioned-learning-state-recalculation.md`; it is not part of the ordinary
learner workflow.
