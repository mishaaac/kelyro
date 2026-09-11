# Claim-to-concept candidates v1

`claim-to-concept-candidates-v1` is a deterministic application pass that
groups accepted I-03 Claims into concept candidates. A candidate is not yet a
learner-trackable Concept and one Claim is not assumed to be one Concept.

The curriculum evidence hand-off preserves each Claim family as a closed,
Research-independent `EvidenceClaimKind`. The extractor accepts only validated
`ready_for_compile` or `ready_with_caveats` evidence sets. It makes no network
calls and rejects repeated bundle/Claim references.

## Semantic subject key

V1 treats the structured Claim `scope` as its semantic subject. For grouping,
it collapses whitespace and applies Unicode lowercase. Version scope and
curriculum status remain part of the key, so current, historical and
version-bound guidance cannot be merged accidentally. Definition, behavior and
other Claim families for the same key remain together as supporting evidence.

The lexicographically first original scope spelling supplies the candidate
title and scope. Claim references are sorted by bundle ID and Claim ID.

## Stable identity and output

Candidate IDs are `candidate.` followed by the full lowercase SHA-256 digest
of the algorithm version, normalized semantic subject, version scope and
status. Candidates are ordered by the same tuple. The same evidence produces
the same IDs and ordering regardless of input Claim order.

The output records candidate ID, title, scope, exact Claim references, version
scope, status and the algorithm version. It does not infer prerequisites,
atomicity, lesson prose, practice or assessments.
