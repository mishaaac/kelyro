# Source Diversity v1

`internal/research/diversity.PolicyV1` evaluates whether a Claim's apparently
corroborating sources have demonstrably independent origins and useful reviewed
dimension coverage. Its immutable identifier is `source-diversity-v1`.

The policy is pure, deterministic, and offline. It does not discover sources,
fetch pages, infer ownership from prose, inspect popularity, modify
`multi-source-verification-v1` or `source-bundle-v1`, persist output, compile
curriculum, or mutate Student Core state.

## Reviewed input

Every Source declared by the Claim must have exactly one review containing:

- the validated Source and optional matching Trust Decision;
- optional reviewed organization from the Trusted Source Registry;
- optional reviewed upstream dependency group, used to identify mirrors,
  syndication, or sources that ultimately depend on the same origin;
- one closed perspective: normative, maintainer, vendor, practitioner,
  academic, independent review, historical, or unknown;
- one closed technical role: reference, implementation, combined,
  explanation, observation, or unknown.

Only `accepted` and `accepted_supplement` Trust Decisions are eligible support.
Missing or non-accepted trust is not converted into an authority tier.
Organization, dependency, perspective, and technical role are reviewed facts;
the policy never guesses them from the locator, title, votes, views, or text.

## Proven independence

Eligible Sources form a graph. Two Sources are joined when either:

1. their normalized non-empty organizations match; or
2. their normalized non-empty upstream dependency groups match.

The independent-source count is the number of connected components with known
organization or dependency metadata. This is deliberately transitive: two
different organizations that mirror the same upstream work remain one origin,
and two documents from one organization remain one origin even if their URLs
or kinds differ. When a multi-source review lacks both organization and
dependency metadata, that Source is not assumed independent. A literal
single-source assessment still reports one source without claiming
corroboration.

## State precedence

The assessment state follows this order:

1. `normative_source_sufficient` when a definition or requirement has exactly
   one eligible tier-A accepted Specification or Standard;
2. `sufficient` when at least two independent origins are demonstrated;
3. `concentrated` when accepted support exists but fewer than two independent
   origins are demonstrated;
4. `unknown` when no Source has accepted trust.

The normative exception is intentional. A unique controlling standard does
not need a second, weaker source merely to make a diversity metric look better.
It receives no artificial single-kind, single-perspective, or missing-
implementation warning. Diversity never overrules conflict, scope,
applicability, freshness, or trust decisions.

## Dimension output

Every Assessment reports:

```text
state
total and eligible source counts
independent source count
organization count
source-kind count
known perspective count
reference present
implementation present
warnings
deferred dimensions
algorithm version
```

Source kind comes from the domain Source. `combined` provides both reference
and implementation coverage. For recommendation, security, and example Claims,
missing reference or implementation coverage is warned; it does not falsify an
otherwise genuine independent-origin count.

V1 explicitly returns geography and language as ordered deferred dimensions.
It neither invents those annotations nor folds locale into independence. A
later version may activate them only with reviewed metadata and documented
policy.

## Warnings

Stable warning families cover:

```text
support.no_accepted_sources
independence.single_source
independence.organization_concentrated
independence.shared_dependency
independence.metadata_unknown
dimension.source_kind_concentrated
dimension.perspective_concentrated
dimension.perspective_unknown
dimension.technical_role_unknown
dimension.reference_absent
dimension.implementation_absent
```

Warnings carry sorted Source IDs and bounded human-readable detail. Output is
stable under input reordering and returned slices are caller-owned.

## Application composition

`application.SourceDiversityService` loads one persisted Claim, every declared
Source, its latest Trust Decision, and an applicable Trusted Source Registry
organization. The caller supplies only reviewed dependency, perspective, and
technical-role annotations. Coverage must be exact; missing or extra Source
annotations are `invalid_state`.

The service makes the policy usable before or alongside Source Bundle assembly
without changing the canonical v1 bundle JSON/hash or the persisted v1
Verification Result. Consumers should keep the assessment adjacent to the
Claim/bundle decision until a future explicitly authorized persistence version
defines durable storage.

## Boundaries

Step 30 adds no migration, repository, network path, scoring formula,
geography/language heuristic, automatic author classification, bundle rewrite,
CLI/TUI surface, Source Code Evidence, Curriculum Compiler, or Student
Core/mastery mutation. Step 31 remains separately authorized work.
