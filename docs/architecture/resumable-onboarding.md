# Resumable onboarding

Step 6 adds a deterministic learner interview that can be interrupted without
coupling educational rules to Bubble Tea, SQLite, AI, or a particular subject.

## Versioned flow

`OnboardingFlow` is an ordered, validated definition with a stable ID and
version. Questions have stable IDs, a section, a prompt, a kind, optional
choices, and required-answer metadata. The core flow `core.onboarding@1`
covers:

1. identity and optional display name;
2. goal title, open domain, and target outcome;
3. general background;
4. prior subject experience;
5. daily and weekly availability;
6. study preference;
7. required mastery strictness;
8. diagnostic opt-in;
9. summary;
10. confirmation.

The application service accepts another flow through `WithOnboardingFlow`.
Future Learning Packs may insert pack-owned questions while retaining the
stable common question IDs needed to apply profile and goal data. Answers are
stored by question ID, so rendering order is not persisted as UI state.

## State machine and navigation

The durable aggregate uses four states:

```text
not_started ---> in_progress ---> completed
                      |
                      +----------> cancelled

cancelled ------restart---------> in_progress
```

Only `in_progress` has a current question. Each submitted answer is validated,
normalized, saved, and then advances the current question. Back navigation
preserves prior answers so they can be edited. Restarting a cancelled interview
creates a clean draft. A completed interview cannot be restarted accidentally.

The TUI persists only submitted answers; the text currently being typed is
presentation state until Enter is pressed. Escape leaves the wizard without
cancelling it. Ctrl+C follows the normal session shutdown path, while the last
submitted onboarding checkpoint remains independently durable. Ctrl+B goes
back and Ctrl+X explicitly cancels.

## Confirmation and recovery

Before confirmation, onboarding changes neither the learner profile nor the
goal history. Confirmation derives validated inputs and reuses `ProfileService`
and `GoalLifecycleService` to update the stable `student.primary` profile and
activate the goal. The goal-specific experience remains distinct from general
profile experience.

The confirmation sequence is retry-safe. If the process stops after the goal
was activated but before the final onboarding checkpoint, resume still lands
on confirmation. A retry recognizes the matching active goal rather than
creating another one, then completes the interview. This recovery path is
covered by a simulated final-write failure test.

Mastery strictness is captured as the numeric threshold selected for the goal.
Step 7 remains responsible for global defaults, named policy presets, custom
ranges, and override precedence. Diagnostic opt-in is retained as an answer
only; Step 6 does not run or implement a diagnostic.

## Persistence

Forward-only migration v7 adds one `onboarding_interviews` row per student.
It stores flow identity/version, lifecycle, current question, timestamps, and
a JSON object of answers. JSON is appropriate for this evolving draft boundary:
the domain validates every configured question and future pack-owned answer,
while relational profile and goal data remain normalized in their existing
tables. Foreign keys and lifecycle checks prevent orphaned or contradictory
terminal states.

The application contract is presentation-neutral (`Show`, `Start`, `Submit`,
`Back`, `Cancel`, and `Confirm`). The current TUI consumes that contract; a
future non-interactive CLI or pack adapter can use the same operations without
depending on Bubble Tea.
