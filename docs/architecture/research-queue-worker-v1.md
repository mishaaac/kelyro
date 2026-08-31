# Research Queue Worker v1

`research-queue-worker-v1` consumes the durable queue created by
`research-trigger-v1`. It is an application boundary around the existing
`LiveResearchOrchestrator`; it does not define pipeline stages, run lifecycle,
network policy, or a second queue.

## Durable state

The original queue lifecycle remains `queued -> dispatched|cancelled`.
Forward-only migration 44 adds execution metadata to the same
`research_trigger_queue` row:

```text
pending -> claimed -> completed
                   -> retry -> claimed
                   -> failed
                   -> cancelled
```

`dispatched` is the existing durable acknowledgement. Execution `completed`
and `failed` both acknowledge the queue as dispatched; the execution status
distinguishes their outcome. Retry keeps the queue `queued`, while cancellation
sets both lifecycles to cancelled. An active claim is excluded from the
claimable ordered list but retains the queued dedupe key.

Each claim records the run ID, monotonic attempt count, transition timestamp,
safe failure classification, and algorithm version. It never stores exception
messages, external response bodies, authorization headers, or credentials.

## Consumer semantics

The consumer validates queue → request → run identity before acquiring the
claim, then calls the orchestrator once. A repeated call cannot execute an
already claimed item. Replaying a completed execution returns its durable
outcome without invoking the orchestrator again.

Transient application classifications are `conflict`, `unavailable`,
`persistence_failure`, and `external_failure`. They return the same logical
queue item to `retry`; no automatic loop or hidden network request occurs. A
later attempt uses a new run for the same immutable request, as required by the
query-to-bundle acceptance contract. Other classified errors fail permanently.
Context cancellation/deadline settles the execution and queue as cancelled.

Success is acknowledged only after the orchestrator returns a matching
completed run. The orchestrator remains the sole owner of stage order and
`ResearchRun` transitions.

## Boundaries

The worker starts no goroutine, daemon, scheduler, recursive crawl, or network
operation by itself. Search/fetch remain behind their adapters, privacy gate,
and cost controls. Concrete query-to-bundle stage assembly belongs to later
I-03C steps.
