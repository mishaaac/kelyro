# Research processing limits v1

Step 45 defines `research-processing-limits-v1`, the bounded execution policy
for a prepared Research run. It coordinates existing discovery and fetch
application services; it does not select candidates, turn candidates into
Evidence, or bypass the network privacy gate.

## Application worker pools

`ResearchProcessingService` runs two sequential phases with independent fixed
worker pools. Discovery defaults to four workers and fetch defaults to eight.
Configuration may lower or raise those values only within immutable v1 hard
ceilings of 16 and 32 respectively. The implementation creates one goroutine
per worker, never one per query or URL.

Jobs enter each pool in input order. Completion order may differ, but results
are written back to their original indexes. If multiple items fail, the
service reports the lowest failing input index deterministically after all
workers have exited. Context cancellation reaches every dependency call,
prevents queued work from starting, and the service waits for its fixed workers
before returning.

## Global run budget

The v1 hard ceilings for one call are:

| Resource | Maximum |
| --- | ---: |
| discovery queries | 100 |
| requested/returned candidates | 500 |
| fetches | 200 |
| claims represented by the run | 5,000 |
| requested/returned fetched bytes | 64 MiB |

Requested candidate and fetch-byte totals are checked before work starts.
Returned totals are checked again so a faulty dependency cannot silently
exceed the run budget. These are structural resource limits; the durable
provider/cost reservation policy remains independently enforced by
`research-cost-control-v1`.

## Shared HTTP limits

The hardened Research HTTP client applies mandatory limits to every attempt,
including retries:

| Control | Default | Hard ceiling |
| --- | ---: | ---: |
| active attempts across the client | 16 | 256 |
| active attempts per normalized hostname | 4 | 32 |
| minimum start interval per hostname | 100 ms | 1 minute |

Per-host concurrency keys use normalized hostnames and intentionally ignore
paths, queries, fragments, and credentials. A waiting cancellation releases
both its host reference and any acquired slot. Host entries disappear after
their last holder/waiter exits, while expired rate schedules are pruned. Rate
state is capped at 1,024 hostnames; at capacity, a new hostname waits
context-aware for the earliest schedule to expire instead of growing the table
or bypassing throttling. Go's transport also sets `MaxConnsPerHost` to the same
active per-host limit, which preserves the cap for physical connections and
redirected destinations.

An injected `RateLimiter` remains an additive provider-specific hook; omitting
it no longer means requests are unthrottled. The mandatory interval limiter
runs first. Foundation's `privacy.allow_network` authorization still occurs in
the application service before this infrastructure boundary.

## SQLite ingestion batches

`ReleaseIngestionRepository` accepts at most 5,000 Evidence records, 5,000
Claims, 256 new releases, and 256 lifecycle updates in one batch. Memory and
SQLite adapters reject larger batches before record validation or mutation.

SQLite validates the complete batch, opens one transaction, reuses prepared
write statements within it, and commits only after all Evidence, Claims,
releases, and status updates succeed. Any validation, relationship, write, or
context error rolls the whole batch back. No migration is needed because the
stored model is unchanged.

## Offline performance fixture

`BenchmarkResearchProcessingStep45Fixture` executes the plan magnitudes without
network access: 100 discovery queries return 500 candidates, 200 sources are
fetched, and the request accounts for 5,000 Claims. Unit and race tests cover
pool ceilings, stable ordering, deterministic error selection, cancellation,
slot cleanup, rate scheduling, hard-limit validation, and transactional
rollback.

Step 45 does not add the controlled full-pipeline HTTP fixture assigned to Step
46 and does not compile curriculum or modify Student Core.
