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

## Manual topic workflow

`kelyro research topic <topic>` normalizes and validates the topic, creates a
durable planned request/run with the default `research-cost-control-v1` budget,
and builds a bounded provider-neutral query plan. The generic CLI authority
profile is a planning preference only: it requires corroboration and tier C or
better, but cannot make any candidate trusted.

The same operation evaluates `research-trigger-v1` with an explicit manual
signal and no recorded evidence. Its deduplicated queue item remains `queued`
because no worker or live adapter was dispatched.

Kelyro currently has no production live-search adapter. Consequently the CLI
does not invent discovery results or pretend that a completed bundle exists.
It leaves the run in `planned`, reports discovery as pending, and states whether
`privacy.allow_network` would permit a future live adapter. No network gate is
invoked because no network operation is attempted. Stored sources, snapshots,
evidence, bundles, and cache remain inspectable while network access is off.

## Boundaries

- The domain and application services do not perform direct network calls.
- CLI output contains bounded metadata, identifiers, counts, and hashes only.
- Unresolved conflicts are ordered deterministically by detection time and ID.
- The CLI does not compile curriculum, mutate mastery, launch a scheduler, or
  implement live discovery in the absence of a configured provider.
