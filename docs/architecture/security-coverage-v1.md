# Security Coverage v1

`security-coverage-v1` makes security an explicit, independently measured part
of a curriculum rather than an optional appendix.

## Domain-declared categories

The closed vocabulary is `input_validation`, `authentication`,
`authorization`, `secrets`, `dependency_security`, `data_protection`,
`secure_defaults`, `threat_awareness`, and `supply_chain`. A domain declares
the categories relevant to its goal as atomic requirements; core does not force
authentication onto a domain that has none or infer security needs from prose.

Requirements may target the curriculum, goal, a competency, or a Concept.
Supports must reference existing Concepts and remain inside competency/Concept
targets when those targets exist.

## Security-sensitive evidence policy v1

`security-evidence-v1` counts a Claim only when all of these hold:

- its evidence set is `ready_for_compile` without caveats;
- freshness is `fresh`;
- kind is `security` and status is `current`;
- confidence is at least `0.8`;
- no unresolved conflict contains the Claim;
- at least two current or version-bound authoritative sources support it;
- at least one of those sources has the `primary` role.

This consumes verification metadata frozen by I-03; it does not re-run
research. Known evidence that misses the policy remains a rejected diagnostic
and cannot produce security coverage. Unknown references remain invalid input.

## Coverage bridge and boundaries

Every domain-declared category produces a `security` Coverage requirement.
Only verified supports cross into the generic Coverage Engine, so a weak Claim
cannot accidentally satisfy the blocking security dimension.

Requirements and supports are sorted by stable IDs. The pass performs no live
network access, secret storage, vulnerability scanning, host inspection,
lesson generation, or student-state mutation.
