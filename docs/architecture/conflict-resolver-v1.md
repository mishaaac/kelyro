# Conflict Resolver v1

Step 23 implements `conflict-resolver-v1` as a pure, deterministic policy in
`internal/research/conflict` plus an application service and append-only
repository. It classifies two incompatible Claims and produces a durable,
explainable outcome. It does not hide the losing Claim, change Evidence, or
produce the multi-source verification result; `multi-source-verification-v1`
consumes its latest visible pairwise outcomes separately.

## Input boundary

V1 is pairwise. A caller supplies exactly two distinct Claim/source
observations and one bounded semantic relation: direct contradiction or
recommendation disagreement. The relation means that a human or an upstream
structured extractor has already identified incompatibility. V1 treats Claim
statements and all external content as opaque data; it does not use keywords,
LLM inference, or arbitrary prose comparison to invent a contradiction.

`ConflictResolutionService` resolves each reference through the stored Claim
and Source repositories. The selected source must belong to that Claim, both
Claims must address the same `ResearchTopic`, and the latest TrustDecision for
each source must be `accepted` or `accepted_as_supplement`. Rejected or merely
pending sources cannot influence the resolver. The injected clock must not
precede any Claim, Source, or TrustDecision input.

## Detection precedence

The policy assigns exactly one of the existing conflict types in this order:

1. `temporal_mismatch` when exactly one source is current;
2. `version_mismatch` when Claim version scopes differ, including specific
   versus unscoped;
3. `scope_mismatch` when applicability or release-status scope differs;
4. `recommendation_disagreement` for the explicit relation or a recommendation
   Claim;
5. `authority_mismatch` when reviewed tiers differ or a normative source rule
   can apply;
6. `direct_contradiction` otherwise.

This precedence explains the current-official versus old-tutorial case as a
temporal conflict even when their authority also differs. Version and scope
separation happen before authority, preventing a globally stronger source from
silently erasing guidance that is valid for another version or context.

## Resolution rules

Temporal conflicts select the current source only for current guidance and
retain the non-current Claim as historical or version-bound context. Version
and scope mismatches are resolved by keeping the Claims in separate declared
scopes; neither becomes a global winner.

For normative definition, requirement, behavior, compatibility, or security
Claims, a specification or standard may beat a non-normative source only when
its reviewed authority tier is at least as strong. Otherwise, a source must be
at least two tiers stronger for v1 to select it. This is an explicit contextual
rule, not numeric score maximization, and preserves the principle that an
official label is not automatically correct.

Comparable recommendations remain unresolved. A direct contradiction without
a temporal, version, scope, or clear authority rule also remains unresolved.
In particular, two accepted official documents at the same tier and version
are escalated instead of being broken by stable ID, source order, freshness,
or an arbitrary score.

## Durable output

Every v1 `Conflict` records:

- the conflict type and both Claim IDs;
- `resolution` when resolved and an explicit `unresolved` flag otherwise;
- confidence and a human-readable reason;
- winning Claim, source, and scope only when a contextual rule has a winner;
- detection time and immutable algorithm ID `conflict-resolver-v1`.

Resolved version/scope conflicts legitimately have no winner. An unresolved
record cannot carry winner metadata or a resolution. Confidence describes the
policy's confidence in its classification/resolution; it is not Claim truth,
Trust Policy output, freshness, or Step 24 verification confidence.

## Persistence and compatibility

Migration v33 extends `source_conflicts` with confidence, reason, optional
winner metadata, algorithm version, and deterministic detection ordering. Rows
created before Step 23 remain readable as `conflict-unversioned-legacy`, with
no invented winner. New writes require a valid v1 or legacy domain record,
existing Claim identities, and—when a winner exists—a `claim_sources`
relationship between the winning Claim and source.

Both memory and SQLite adapters are append-only and return chronological,
stable-ID ordered history for a Claim. Reassessment appends another record; it
does not rewrite the prior decision that a research run observed.

## Explicitly deferred

V1 does not discover candidate Claims, parse web prose, calculate freshness,
count independent organizations, verify a Claim from multiple sources, modify
Source Bundles, compile curriculum, migrate learner content, or change Student
Core state.

## I-03 closure status

Step 49 reconfirmed `conflict-resolver-v1` as the shipped append-only conflict
contract. Contradictions, version/scope mismatches and unresolved outcomes stay
visible; the resolver cannot invent a winner or mutate bundles, curriculum, or
Student Core. Hosted CI for the current source commit remains the final formal
closure gate.
