# Research Trigger Policy v1

Step 34 defines when Kelyro should research or re-research a topic. The
deterministic policy is identified by `research-trigger-v1`. It produces a
decision and durable queue metadata only; it does not start a scheduler,
goroutine, timer, worker, or network operation.

## Inputs and triggers

The policy accepts an immutable `ResearchRequest`, an evaluation timestamp,
and bounded signals. It recognizes exactly eight triggers:

```text
manual
missing_evidence
freshness_expired
new_technology_release
deprecation_detected
conflict_unresolved
curriculum_compile_request
security_sensitive_refresh
```

Missing evidence means an explicit evidence count of zero. Freshness expires
when the state is stale or `as_of >= next_verify_at`; this makes the deadline
boundary deterministic. Release, deprecation, conflict, future compile, and
security refresh signals must be supplied explicitly by their owning use case.
The policy never infers them from absent data.

All matching triggers are retained in stable order instead of hiding secondary
reasons. Manual and security refresh are critical; deprecation, unresolved
conflict, and new release are high; freshness, missing evidence, and future
compile are normal unless combined with a stronger trigger. If no trigger
matches, no queue record is created.

## Queue metadata and deduplication

A queued item preserves the request, all triggers, priority, queue timestamp,
algorithm version, and a deterministic dedupe key derived from normalized
topic subject/domain/technology, purpose, and target version. Request and queue
IDs and timestamps do not change that logical key.

Only one `queued` record may exist for a dedupe key. A repeated evaluation
returns the existing item, including its original reasons and priority. Once it
is explicitly dispatched or cancelled, a later trigger may create a new item.
Queue order is critical, high, normal, then `queued_at` and stable ID.

`queued -> dispatched|cancelled` are the only transitions in v1. They require
an explicit timestamp and cannot rewrite request, triggers, priority, dedupe
key, or algorithm metadata.

## Persistence and boundaries

Forward-only migration v40 adds `research_trigger_queue`, a partial unique
index for active dedupe keys, and a deterministic queue-order index. The
SQLite and memory adapters implement the same idempotency and transition
semantics. Concurrent duplicate inserts converge on the existing active row.

This queue is not an external scheduler and does not execute work. It performs
no discovery, fetch, release lookup, model call, curriculum compile, or Student
Core mutation. Step 35 may explicitly dispatch manual CLI work through the
same application boundary while still honoring privacy and cost controls.

