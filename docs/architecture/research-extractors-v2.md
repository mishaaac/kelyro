# Live Research Extractors v2

Dogfooding showed that the v1 exact-topic contract did not interoperate with
ordinary CLI subjects such as `Go 1.27 release notes`: source prose can contain
a complete literal fact without repeating every qualifier in the same order.
The production composition therefore uses `evidence-extractor-v2` and
`claim-extractor-v2`; both v1 implementations remain available for durable
history and compatibility tests.

## Evidence v2

V2 preserves the v1 bounds, candidate kinds, literal excerpts, hashes, signal
vocabulary, sorting, deduplication, cancellation, and no-I/O behavior. Exact
subject phrases still score 55. Distinct subject-term overlap scores 18 per
term, capped at three terms. A title or heading that establishes document
topic contributes 20 points, but never becomes the local topical anchor for an
otherwise unrelated passage. The admission floor remains 50.

This admits a passage containing `context` and `cancellation` inside a matching
Go context document while still rejecting an unrelated deprecation paragraph
whose page title alone mentions the requested topic.

## Claim v2

V2 still selects only complete, terminally punctuated sentences copied byte for
byte from persisted Evidence. Ambiguity, URLs, quotations, multiple families,
conflicting status/version qualifiers, bounds, hashes, and maximum counts keep
their v1 behavior.

A natural subject anchor requires at least half of its distinct terms, capped
at three, and at least one matched term longer than two characters or numeric.
The closed definition shapes add explicit plural/definitional forms such as
`are named`, `are the`, `defines`, and `provides a way`; a subject term must
precede the definition marker. This prevents a later incidental topic mention
from relabeling an unrelated sentence as a definition.

For release facts, a dotted version appearing in the topic may supply
`VersionScope` only when that same delimited token occurs literally in the
statement. The unqualified word `release` uses this path only as the verb phrase
`to release`; noun phrases such as `read the release notes` are not release
facts. No statement text, authority, stability, or truth is inferred.
