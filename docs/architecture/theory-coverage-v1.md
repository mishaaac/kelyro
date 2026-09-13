# Theory Coverage v1

`theory-coverage-v1` verifies that every explicitly important competency has an
evidence-backed conceptual knowledge contract.

## Contract

Importance is declared by competency ID; the pass does not infer it from a
name, prose, or a hardcoded competency level. Each important competency must
have at most one `TheoryContract`. A contract selects one or more independent
facets:

- `definition`
- `mental_model`
- `mechanism`
- `tradeoffs`
- `failure_modes`

The selection is evidence-backed and domain-specific. Requiring all five is
allowed but is not a universal default. A `TheoryFacetSupport` links exactly one
required facet to one or more Concepts already mapped to that competency and
to accepted I-03-derived evidence.

These structures state what conceptual understanding must be supported. They
do not generate or render lesson prose; that remains an I-05 responsibility.

## Results and Coverage Engine bridge

Each required facet is atomic and reports `covered` or `missing`. Competency
status is `covered` when every facet has support, `missing` when none has
support, and `partial` otherwise. Missing contracts are reported explicitly as
missing rather than being treated as an invalid or optional dimension.

The report also emits `CoverageRequirement` and `CoverageSupport` records for
the generic Coverage Engine. Each declared facet becomes its own theory
requirement, so one covered facet cannot hide a missing definition or mental
model. A missing contract emits one unsupported competency-targeted theory
requirement, allowing the Gap Scanner to produce `missing_theory`.

## Evidence, determinism, and boundaries

Contracts, supports, competency mappings, and Concepts must resolve to the
provided matrix and accepted evidence sets. Requirement IDs are derived from
stable contract/competency identity plus facet. Outputs are sorted by stable
IDs and canonical facet order.

The pass performs no network access, content generation, practice/assessment
definition, hierarchy mutation, persistence, or student mastery mutation.
