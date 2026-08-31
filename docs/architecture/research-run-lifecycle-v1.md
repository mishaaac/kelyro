# Research Run lifecycle v1

`research-run-lifecycle-v1` is the single durable state machine used by live
research orchestration. It retains the I-03 vocabulary and schema:

```text
planned ──→ running ──→ completed
   │            │
   ├────────────┼──────→ failed
   └────────────┴──────→ cancelled
```

Detailed orchestrator work (`search`, candidate registration, fetch, snapshot,
normalization, extraction, verification, bundling, and finalization) runs under
the durable `running` state. Those stages are ordered execution checkpoints,
not a second persisted lifecycle.

## Transition contract

- A newly queued run is `planned` and has no completion timestamp.
- Execution transitions `planned → running` before the first pipeline stage.
- `running → completed` is legal only after the bundle stage returns the
  durable bundle identity for the same run.
- A failure transitions `planned` or `running` to `failed`.
- Context cancellation transitions `planned` or `running` to `cancelled` and
  stops new stage work.
- Terminal states never reopen or change into another terminal state.
- Repeating the current state is idempotent and preserves the original
  completion timestamp.
- Every transition timestamp is valid UTC and cannot precede `started_at`.

`TransitionResearchRunV1` is the pure domain policy.
`ResearchService.TransitionRun` loads, applies, and persists that policy.
`UpdateRun` remains available for compatibility, but validates proposed status
changes against the same policy and rejects changes to run identity or start
time.

## Bundle ordering

The live acceptance contract requires:

```text
running run → append durable bundle → completed run
```

The bundle assembler and repositories therefore accept a valid `running` run
for assembly. They continue accepting already-completed runs for compatibility
with existing I-03 fixtures and imports. A planned, failed, or cancelled run
cannot own a newly appended bundle.

Bundle creation does not complete the run by itself. The orchestrator verifies
the returned bundle ID and `RunID`, executes finalization, then calls the single
lifecycle service. Queue claiming, acknowledgement, and retry remain outside
this contract and belong to the existing queue consumer.
