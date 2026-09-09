# Research and Sources CLI v1

Step 35 introduced Research & Source Intelligence as human-first CLI views.
I-03C completes the manual live path behind the same
`research-cli-workflow-v1` contract; query construction remains independently
versioned as `query-planner-v1`.

## Command surface

`kelyro sources` and `kelyro sources list` are equivalent. They list durable
source identities and bounded metadata, never fetched page bodies. `sources
show <source-id>` adds the latest immutable snapshot identity, fetch timestamp,
and content hash when available. Registry, provenance trace, stale scheduling,
and unresolved conflict commands reuse their existing application services.

`kelyro research status <run-id>` reads the durable request and run. When a
Source Bundle exists, it presents its state, primary/supporting counts,
conflicts, and verification timestamp. A search result or query plan is never
counted as a source, claim, or evidence.

`kelyro research show <run-id>` reads the append-only `research-audit-v1`
checkpoints for the run. It exposes exact queries, algorithm versions,
providers used, network mode/privacy result, cache/fetch counters, and durable
locator/snapshot/hash refs. It always distinguishes reproducible stored inputs
from the external Internet, which may return different content in the future.

`kelyro research update-scan` inventories stored releases, tracked sources,
freshness schedules, deprecations, and unresolved conflicts. It reports an
explicit incomplete reason when privacy blocks current lookup or no live
provider is configured, and never modifies curriculum or student state.

## Manual topic workflow

`kelyro research topic <topic>` normalizes and validates the topic, creates a
durable planned request/run with the default `research-cost-control-v1` budget,
builds a bounded provider-neutral query plan, and appends the exact planned
inputs as the initial audit checkpoint. The generic CLI authority profile is a
planning preference only: it requires corroboration and tier C or better, but
cannot make any candidate trusted.

The same operation evaluates `research-trigger-v1` with an explicit manual
signal and no recorded evidence, then passes the durable queue item and run to
`research-queue-worker-v1` in the same call stack with a fixed two-minute
deadline. The production binary assembles the provider factory, Secrets,
privacy gate, search/fetch caches, hardened HTTP fetcher, normalizer, SQLite
services, live orchestrator, terminal audit, and transactional finalizer. The
internal `ResearchTopicExecutor` seam remains replaceable for deterministic
tests and alternate embedding, but it is no longer a missing production stage.

The synchronous production flow is:

```text
topic / plan / initial audit
→ queue claim
→ Brave discovery through privacy + cost + cache
→ candidate deduplication and durable Source registration
→ bounded HTTP fetch and immutable snapshots
→ normalization and post-fetch Source classification
→ literal Evidence and Evidence-backed Claim extraction
→ trust, freshness, multi-source verification, and diversity
→ supported-Claim Source Bundle and provenance graphs
→ atomic completed/failed/cancelled run, queue, audit, and cost finalization
```

Success returns a durable `ready` or `ready_with_caveats` bundle and a
`completed` run. Provider absence, blocked network without sufficient cache,
no usable results, insufficient verification, cancellation, or another stage
failure returns a terminal, auditable non-success instead of leaving the run
`planned` or presenting a false bundle. Individual fetch/normalization failures
may remain warnings when other Sources provide enough support.

Provider configuration and network permission are independent. Brave is used
only when `research.search.provider="brave"`, its secret is available through
Foundation Secrets, and privacy permits live search. Every live search and
fetch passes through cost control and bounded adapters. Stored Sources,
snapshots, Evidence, Claims, bundles, audit, and usable cache remain inspectable
while network access is off. No daemon, scheduler, detached goroutine, or
unbounded retry is started.

## Boundaries

- The domain and application services do not perform direct network calls.
- CLI output contains bounded metadata, identifiers, counts, and hashes only.
- Unresolved conflicts are ordered deterministically by detection time and ID.
- The CLI does not compile curriculum, mutate mastery, launch a scheduler, or
  bypass provider configuration, Secrets, privacy, cache, or cost controls.
