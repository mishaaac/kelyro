# Integrated learner setup

Step 12 joins the existing profile, goal, mastery-policy, onboarding,
diagnostic, curriculum-instance, and instance-state services without moving
their rules into the CLI or TUI. `LearnerSetup` is the durable orchestration
checkpoint; it does not replace any of those aggregates.

## Lifecycle and completion invariant

The persisted states are:

```text
awaiting_onboarding
  ├─ opt in  → awaiting_diagnostic → initializing → completed
  └─ opt out → initializing → completed
```

`setup_completed_at` is present only in `completed`, and it equals the final
`updated_at`. A completed onboarding interview or a created curriculum instance
is therefore insufficient to report a ready learning path.

`Show` resumes the current child flow. It also retries `initializing`, which is
the recoverable checkpoint left when a previous initialization attempt failed.
Diagnostic attempts retain their item-level checkpoints and are resumed by ID.

## Initialization policy

I-02 uses the versioned `foundation-demo@1.0.0` curriculum and matching
diagnostic as development/demo content. This is an explicit temporary fixture
provider, not the future curriculum-selection UX and not a Curriculum Compiler.
I-04 will replace this bridge with a researched or imported learning path.

After the optional diagnostic becomes terminal, every concept in the selected
curriculum is materialized in the learner's exact Curriculum Instance with:

```text
exposure = not_seen
mastery  = 0
first_seen_at = null
last_seen_at  = null
```

Diagnostic evidence is preserved separately. Estimated diagnostic mastery is
not promoted to confirmed mastery; that remains the responsibility of the
future versioned Mastery Engine.

Concept-state creation and the final setup transition run in one unit of work.
If any write fails, neither partial concept states nor `setup_completed_at` are
committed. The durable pre-transaction checkpoint remains recoverable: either
`initializing`, or `awaiting_diagnostic` with a terminal attempt. Retrying
`Show` safely repeats the operation and preserves any already valid state.

## Development reset

`kelyro setup reset` is gated to development/demo builds and requires explicit
confirmation (or `--yes`). It deletes only the setup checkpoint, onboarding
interview, its fixture curriculum instance, and diagnostic attempt/evidence for
that instance. The learner profile, goal history, Foundation state, and all
unrelated educational data remain intact.

## Presentation boundary

The TUI automatically opens setup when no completed checkpoint exists, renders
the onboarding summary before confirmation, and can run or skip the optional
diagnostic. CLI `setup status` exposes the durable state; `setup reset` is the
safe development recovery surface. Both adapters call the same application
service and contain no educational calculations.
