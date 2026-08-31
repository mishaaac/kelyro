# Research search configuration v1

I-03C adds provider-neutral configuration for bounded live source discovery.
This configuration selects no vendor and performs no network request by itself.

## TOML contract

```toml
[research.search]
provider = ""
max_results_per_query = 8
max_queries_per_run = 4
```

| Dotted key | Type | Default | Valid values |
|---|---|---:|---|
| `research.search.provider` | string | `""` | empty, or a lowercase provider identifier of at most 64 ASCII characters using letters, digits, internal `-` or `_` |
| `research.search.max_results_per_query` | integer | `8` | 1–100 |
| `research.search.max_queries_per_run` | integer | `4` | 1–8 |

The empty provider is the safe disabled default. Both global and project files
may set these keys through the existing precedence rules; project values and
explicit CLI overrides win over global values.

The limits align with the frozen provider boundary and `query-planner-v1` hard
ceilings. Configuration can reduce or select work within those bounds but
cannot raise the application hard caps.

## Readiness states

`config.ResearchSearchConfig.State` combines resolved configuration with safe
capability booleans supplied by future infrastructure wiring:

| State | Meaning |
|---|---|
| `disabled` | `provider` is empty; no live search adapter should be invoked. |
| `unavailable` | A provider identifier is selected but no matching adapter is available. |
| `missing_credentials` | The adapter exists and requires a credential, but Foundation Secrets reports none available. |
| `configured` | The adapter exists and either needs no credential or its credential is available. |

The readiness input contains booleans only. It never contains, returns, logs or
persists a secret value.

## Secrets and privacy boundaries

Provider credentials are not configuration. The Step 8 Foundation Secrets
integration uses exactly this reference:

```text
research.search.<provider>.api_key
```

The production secret store resolves the matching environment reference first:

```text
KELYRO_SECRET_RESEARCH_SEARCH_BRAVE_API_KEY
```

and otherwise uses the native OS keychain. The credential can be populated via
`kelyro secrets set research.search.brave.api_key`; the CLI reads its value
without terminal echo. The strict configuration schema continues to reject
`research.search.api_key` and any other credential-shaped unknown key.

`researchsearch.NewBraveFromSecrets` requests only the exact reference, passes
the value directly into the in-memory adapter, and returns no value-bearing DTO.
It does not call `Status`, persist the response, or include backend details in
errors. Missing, unavailable and invalid credentials remain distinct safe
states.

The only permitted readiness rendering is state-only, for example:

```text
Provider: configured
Credential: available
```

The provider value, errors, TOML, SQLite, logs, audit, cache, Doctor and
diagnostic formatting never contain the API key.

Selecting a provider does not grant network access. Every live search must
still pass through `DiscoveryService` and the Foundation
`privacy.allow_network` gate. A disabled privacy policy wins over a configured
provider.

## Production assembly

I-03C Step 11 wires the boundary explicitly for each durable run:

```text
resolved workspace config
→ Foundation SecretStore
→ resolved privacy.NetworkGate
→ hardened Brave transport and adapter
→ cost-controlled DiscoveryService
```

`researchsearch.Factory` recognizes only the empty disabled selection and the
documented `brave` provider. Empty or unknown selections return explicit
disabled/unavailable errors, perform no secret read and never substitute
`StaticSearchProvider`. Construction performs no network request. The fixed
transport is created in `cmd/kelyro/main.go`, uses the running Kelyro version in
its bounded User-Agent, and is injected into the application composition root.

Actual per-run assembly remains workspace-aware: application code resolves the
effective global/project/CLI configuration, passes the existing SecretStore,
privacy gate, cost service, clock and run identity to the factory, and receives
the production provider plus its guarded `DiscoveryService`. The configured
`max_queries_per_run` truncates the planned query list and becomes the durable
per-run `SearchRequests` budget; `max_results_per_query` becomes
`live-search-cost-policy-v1` input. Assembly does not execute discovery.

## Ownership boundary

The schema and readiness model live in `internal/config`, and strict TOML
roundtrips live in `internal/infra/configfs`. Vendor-specific identifiers may be
interpreted only by future infra/config/Doctor wiring. Research domain and the
`SearchProvider` application port remain vendor-neutral.

Provider selection, adapter implementation, transport hardening, credential
resolution, privacy/cost gating and production assembly are now implemented by
I-03C Steps 5–11. Doctor checks and live orchestration remain separate later
steps.
