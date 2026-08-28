# Research Cost Control v1

Step 33 adds provider-neutral, bounded cost control to Research. The algorithm
is identified by `research-cost-control-v1`; it records units, not currency or
vendor prices.

## Dimensions and default budget

Every cost record uses five explicit dimensions:

```text
search requests
fetch requests
bytes
provider API calls
future model calls
```

The default v1 budget allows 6 searches, 12 fetches, 8 MiB, 18 provider calls,
and no model calls per run. The corresponding per-topic limits are 12, 24,
16 MiB, 36, and zero. A caller may add a global UTC-day budget. Per-run limits
must fit inside per-topic limits; all dimensions remain generic integers.

`ResearchRun.Cost` carries the chosen budget, accumulated use, valid-cache
savings, and an explicit budget stop scope/reason. Runs created before v1 have
no cost metadata and remain readable.

## Decision order

Before a caller performs network work, `ResearchCostService.Evaluate` applies
this order:

1. use a valid cache result and record the avoided units;
2. stop if verification requirements are already satisfied;
3. stop if the requested number of primary sources is already present;
4. atomically reserve the proposed generic units;
5. if a run, topic, or optional daily budget would be crossed, deny the
   network operation with a user-visible explanation.

The controller returns decisions; it does not perform network I/O. Discovery,
fetch, release, and future model adapters remain responsible for presenting
their proposed units before invoking a provider. Fetch callers must keep their
bounded byte request within the remaining authorization they reserve.

## Persistence and concurrency

Forward-only migration v39 adds `research_cost_controls` and the append-only
`research_cost_events` ledger. SQLite triggers evaluate run, logical topic, and
optional UTC-day totals in the same statement that appends an event. A rejected
insert writes no partial usage. The adapter then records the closed stop scope
and stable human explanation on the control row.

Logical topic accounting compares subject, domain, and technology across
otherwise independent ResearchRequests. A new run cannot inject prior usage,
cache savings, or a predeclared stop. Normal status changes do not overwrite
repository-owned cost metadata.

The in-memory adapter implements the same semantics with a mutex and is used by
application tests. SQLite tests prove that a second run cannot exceed a shared
topic budget and that the rejected units do not appear in stats.

## Inspection and boundaries

`kelyro research stats` reports all-time use, current UTC-day use, runs stopped
by budget, and units saved by valid cache. It does not estimate money.

The policy does not add a paid provider, scheduler, model integration, network
permission bypass, Curriculum Compiler behavior, or Student Core mutation.
Research Trigger Policies belong to Step 34.

