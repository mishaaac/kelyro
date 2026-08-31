# Research and Sources CLI v1

Step 35 exposes Research & Source Intelligence as human-first, read-oriented
CLI views. The orchestration contract is identified by
`research-cli-workflow-v1`; query construction remains independently versioned
as `query-planner-v1`.

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
inputs as the initial audit checkpoint. The generic CLI authority
profile is a planning preference only: it requires corroboration and tier C or
better, but cannot make any candidate trusted.

The same operation evaluates `research-trigger-v1` with an explicit manual
signal and no recorded evidence. When a `ResearchTopicExecutor` is assembled,
the command passes that durable queue item and run to `research-queue-worker-v1`
in the same call stack with a fixed two-minute deadline. The result is returned
to the CLI view; no daemon, scheduler, detached goroutine, or hidden retry is
started.

The execution seam is optional until the concrete query-to-bundle stages are
assembled. Without it, the CLI does not invent discovery results or pretend
that a completed bundle exists: the run remains `planned`, discovery is
pending, and no network operation is attempted. With it, every live stage
continues to pass through privacy and cost policy. Stored sources, snapshots,
evidence, bundles, and cache remain inspectable while network access is off.

## Boundaries

- The domain and application services do not perform direct network calls.
- CLI output contains bounded metadata, identifiers, counts, and hashes only.
- Unresolved conflicts are ordered deterministically by detection time and ID.
- The CLI does not compile curriculum, mutate mastery, launch a scheduler, or
  implement live discovery in the absence of a configured provider.
