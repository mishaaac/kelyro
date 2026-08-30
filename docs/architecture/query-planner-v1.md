# Deterministic Research Query Planner v1

Step 12 adds the pure, provider-neutral `internal/research/queryplanner`
algorithm. It converts one validated research topic and purpose into ordered
discovery intentions without using AI, network access, persistence, or search
provider behavior.

## Contract

The immutable algorithm identifier is:

```text
query-planner-v1
```

`PlannerV1.Plan` consumes:

```text
ResearchTopic
optional target SourceVersion
ResearchPurpose
already-selected AuthorityProfile
```

The topic carries subject, domain, and optional technology. Authority-profile
matching remains owned by `internal/research/authority`; the planner validates
the supplied profile but does not silently select or invent one.

The returned `ResearchQueryPlan` contains at most eight ordered
`ResearchQuery` values. Every item records:

```text
normalized query text
desired SourceKind
minimum required AuthorityTier
positive sequential priority
```

The required tier is copied from the profile. It is a later classification
threshold, not a trust decision about discovery results. The plan and each
item have public validation so invalid or differently-versioned values cannot
cross the boundary silently.

## Deterministic query construction

The base query is assembled without empty placeholders:

1. use technology when present, otherwise use the generic domain when present;
2. add the topic subject;
3. add the opaque target version when present;
4. add the purpose/source qualifier;
5. collapse all Unicode whitespace to single spaces.

This means a software topic may begin with `Go HTTP caching`, while a generic
topic can begin with `statistics Bayesian inference`. The planner does not add
API, source-code, or technology language merely because the broader Research
system also supports software.

Purpose v1 contributes these default discovery variants:

| Purpose | Default source intentions |
| --- | --- |
| `concept_definition` | official documentation, specification, tutorial |
| `current_usage` | official documentation, API reference, tutorial, source code |
| `version_behavior` | official documentation, API reference, release notes, source code |
| `release_status` | release notes, official release status, official announcement |
| `deprecation_check` | official documentation, release notes, specification, source code |
| `prerequisite_research` | tutorial, official documentation, specification |
| `production_practice` | official documentation, API reference, source examples, tutorial |
| `security_guidance` | official security guidance, security standard, security release notes, source code |

Profile preferences that match a purpose default are promoted in their
declared order. Remaining purpose defaults follow in their stable order so a
large profile cannot crowd out the purpose's core discovery coverage. Other
preferred kinds are appended with an explicit combined purpose/source
qualifier while capacity remains. Source kinds and query strings are
deduplicated, and priority is always the resulting one-based position.

## Discovery integration

The plan remains independent from provider and application DTOs. An
orchestrator maps each item as follows:

```text
ResearchQuery.Query             -> SearchQuery.Text
ResearchQuery.DesiredSourceKind -> SearchOptions.DesiredKind
Input.TargetVersion             -> SearchOptions.TargetVersion
ResearchQuery.Priority          -> execution order
```

The orchestrator supplies the research request ID and its bounded result limit.
`DiscoveryService` still owns online/offline mode, the Foundation privacy gate,
provider invocation, normalization, duplicate handling, and result bounds.

## Boundaries

Planner output is search intent, not a source, snapshot, evidence, claim, trust
decision, freshness score, or release fact. Step 12 does not add a live search
adapter, cache policy, query execution orchestration, evidence extraction,
Curriculum Compiler behavior, or Student Core mutation.
