# Progress Dashboard read model

`progress-dashboard/v1` is the single application query used by future CLI and
TUI progress surfaces. It assembles a presentation-neutral view; presentation
code does not open SQLite, traverse curriculum tables, recalculate metrics, or
coordinate independent Student Core services.

## Scope

The read model contains the active goal and curriculum instance, overall
curriculum progress, the current phase/module/lesson/topic/concept path,
mastery summary, due-review count, today's adaptive plan, study time, streak,
latest goal milestone, and titled weak concepts. It records its generation
instant, profile timezone, analytics policy, and read-model version.

Curriculum progress, mastery, reviews, location, and weak concepts are scoped to
the active curriculum instance. Study time and streak remain learner-wide facts
because historical effort and consistency do not disappear when a goal is
paused or replaced. A goal milestone is scoped to the active goal.

## Empty and unavailable data

Optional aggregates use pointers rather than fabricated placeholder entities:

- no active goal means `Goal`, `Curriculum`, `Current`, `TodayPlan`, and
  `RecentMilestone` are absent;
- an active goal without an active curriculum keeps the goal but omits the
  curriculum-dependent fields;
- a curriculum with every concept mastered has no current frontier;
- no known concepts produces an absent average mastery and an empty weak list;
- zero reviews, study time, or streak days remain meaningful numeric zeros.

The view never invents a curriculum title because the published persistence
contract stores the stable curriculum reference and hierarchy titles, not the
definition-level display title.

## Consistency and refresh

`ProgressDashboardService.Show` first invokes the existing
`AdaptiveDailyPlanService.Today` boundary. That service reuses today's snapshot
when its versioned source fingerprint is unchanged and regenerates it after
relevant evidence, state, review, mistake, history, or policy changes. Absence
of an active goal or curriculum is a valid empty plan state.

The dashboard then reads the active learning context, curriculum outline,
instance concept states, retention, reviews, study sessions, history, and
milestones inside one unit of work. `learning-analytics-v1` calculates the
metrics at one captured instant from those primary facts. The returned plan is
accepted only when its student, goal, and curriculum instance match that active
context. Calling `Show` again after a progression or session update therefore
refreshes both the calculated metrics and the invalidated daily plan.

The dashboard does not read the legacy analytics snapshot as truth and does
not persist another dashboard cache.

## Current location and ordering

The current location is the first concept in canonical hierarchy order whose
instance state is neither `mastered` nor `review_due`. Missing state means the
concept is not yet seen. Hierarchy order compares phase, module, lesson, topic,
and concept sibling positions, using stable node identity as the tie-breaker.
If all concepts are mastered, the location is absent rather than pointing back
to completed content.

This location is a navigation frontier, not a replacement for prerequisite or
Daily Plan policy. The adaptive planner remains authoritative for what should
be studied today.

## Performance

`CurriculumStateRepository.Outline` returns only stable node identity, type,
parent, title, and sibling order. SQLite loads it in one query. The application
indexes nodes and states in maps, validates every concept path once, and sorts
concept paths in `O(n log n)` time. It performs no query per concept. The memory
adapter uses the same projection and pre-indexes curriculum nodes when building
large deterministic fixtures.

The scale test exercises 5,000 concepts together with daily-plan refresh and
the complete dashboard assembly.

## Boundaries

This step adds no CLI command, TUI screen, Markdown progress export, schema
migration, Exercise Engine behavior, Research Engine, Curriculum Compiler, AI
provider, or plugin. Step 26 may render this application read model without
gaining direct persistence access.
