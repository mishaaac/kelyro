# Live Research Bounds v1

Status: active for I-03C Step 45  
Algorithm/policy versions: `query-planner-v1`,
`research-processing-limits-v1`, `evidence-extractor-v1`, and
`claim-extractor-v1`.

Live query-to-bundle work is synchronous, cancellable, and bounded at every
fan-out boundary. Configuration may reduce a limit but may not raise these
hard ceilings.

| Boundary | Production default | Hard ceiling | Enforcement |
| --- | ---: | ---: | --- |
| Queries per run | 4 | 8 | Search configuration and `ResearchQueryPlan.Validate`; the CLI truncates the deterministic plan to the configured value. |
| Results per query | 8 | 100 | Search configuration, `SearchOptions`, cost-controlled discovery, and provider response validation. |
| Live Sources per run | 32 by defaults | 200 | The search stage stops before exceeding `MaximumLiveResearchSourcesPerRun`, aligned with the live fetch ceiling. Generic non-live candidate tooling retains its independent 500-item hard ceiling. |
| Concurrent source fetches | 8 | 32 | `LiveSourceFetchService` uses a fixed worker pool from validated `research-processing-limits-v1`. |
| Bytes per Source | 2 MiB | allocated run share | Each request receives `min(2 MiB, 64 MiB / source_count)` and the fetch adapter enforces declared and actual decoded size. |
| Fetched bytes per run | at most 64 MiB | 64 MiB | Allocation occurs before requests; fetched bodies and result summaries are revalidated after the workers finish. |
| Evidence candidates per Source | 24 | 24 | Deterministic Evidence extractor output contract and live extraction validation. |
| Claim candidates per Evidence | 8 | 8 | Deterministic Claim extractor output contract. |
| Claim candidates per Source | 192 | 192 | Live Claim extraction rejects more than 24 Evidence for one Source before any Claim/citation write; `24 × 8 = 192`. |
| Evidence/Claim candidates per run | 1000 | 1000 | Live extraction accumulators reject overflow before appending the overflowing result. |

The default discovery fan-out is `4 × 8 = 32` Sources. If users raise both
search settings, the production stage stops at 200 candidates so every
registered Source remains processable by the following fetch stage. It does
not issue the remaining planned queries after the cap is reached.

Provider API calls can exceed logical queries only through bounded Brave
pagination (20 results per page). Every physical call requires its own durable
cost authorization; pagination stops immediately when authorization fails.

These bounds control memory, goroutines, remote calls, persisted fan-out, and
worst-case extraction work. They do not change trust, verification, ranking,
privacy, retry, or Source Bundle policy.
