# Further Reading Selection v1

`internal/research/furtherreading.SelectV1` produces a small, explainable set
of student-facing resources from already reviewed Research records. Its
immutable algorithm identifier is `further-reading-selection-v1`.

The selector is separate from Source Bundle evidence assembly. A primary
source can be excellent evidence but poor reading material, while an accepted
community explanation can be pedagogically useful without becoming primary
evidence. Selection never creates Evidence, changes a Trust Decision, or
reclassifies a Source Bundle.

## Categories and reviewed input

Every candidate has one explicit category:

```text
official_deep_dive
tutorial
interactive_resource
reference
community_explanation
video_supplement
source_code
```

The caller also supplies the persisted Source and its reviewed
`resource-quality-v1`, Trust, and `freshness-v1` assessments, plus explicit
reading level, access conditions, community status, optional reviewed
organization, and optional duplicate key. The selector does not parse source
content or infer paywalls, reading level, organizational ownership, or semantic
duplication.

Reading levels are `introductory`, `intermediate`, and `advanced`. Access is
closed as `open`, `registration_required`, `paywalled`, or `unknown` so a
consumer cannot represent a known paywall as open access. Community article
and forum kinds, and the `community_explanation` category, must carry the
explicit community marker. Video and other kinds can also be marked community
when that status was reviewed.

Step 27 requires `interactive_resource` to reference a specialized Playground
source. Its reviewed official/community affiliation must agree with the
candidate community marker, preserving the mandatory community label. The
selector still does not initiate discovery or execute the Playground.

## Eligibility

Rules run before ranking:

1. the latest Trust Decision must be `accepted` or
   `accepted_as_supplement`;
2. Resource Quality must recommend `further_reading` or `example`;
3. `source-temporal-policy-v1` must not classify the source as
   `not_applicable` for the requested version;
4. duplicate Source identities are invalid input.

This intentionally excludes a dense source assessed only as `evidence` even
when it is tier A. Authority remains an input to ranking, not a substitute for
pedagogical quality.

Every valid but unselected candidate receives one closed exclusion reason:
`trust_not_accepted`, `quality_not_suitable`, `version_not_applicable`,
`duplicate`, or `limit_reached`. “Not selected” therefore does not silently
become “bad source” or “does not exist.”

## Score and ordering

Eligible candidates receive this base rank:

```text
base = 0.43 * resource_quality_score
     + 0.18 * reading_level_fit
     + 0.14 * freshness_score
     + 0.08 * authority_value
     + 0.05 * access_value
     + 0.05 * temporal_value
```

Reading-level fit is `1.00` for an exact match, `0.65` for an adjacent level,
and `0.30` across two levels. Authority maps A/B/C/D/E to
`1.00/0.80/0.60/0.35/0.10`. Access maps open/registration/paywalled/unknown to
`1.00/0.65/0.35/0.25`. Current guidance and exact version authority receive
`1.00`; historical context receives `0.35`. Non-applicable content has already
been excluded.

An optional reviewed duplicate key groups mirrors, republications, or
substantially equivalent coverage without comparing arbitrary source text.
Only the strongest base-ranked candidate in each group survives; ties use
quality, authority, then stable Source ID.

Selection is greedy and deterministic. At each position, an unseen category
adds `0.04` and a known, unseen normalized organization adds `0.03`; the final
rank is capped at `1.00`. Unknown organizations receive no invented diversity
credit. Stable Source ID resolves the final tie, so input order does not affect
the result.

The input limit is required and cannot exceed seven items. Candidate input is
bounded at 128 records.

## Student-visible disclosure

Selected items retain title, canonical locator, category, reading level,
access, community flag, reviewed organization, quality score, authority tier,
freshness state, rank score, and the exact quality/trust/freshness algorithm
identifiers used.

Labels make community, registration, paywall, unknown access, stale, and
historical status directly renderable. Warnings make the following conditions
explicit:

- registration, paywall, or unknown access;
- stale tutorial (distinct from another stale resource) or unknown freshness;
- historical/non-current applicability;
- reading level above the requested level;
- unknown publishing organization and therefore unconfirmed organizational
  diversity.

A stale tutorial is allowed only with its dedicated warning. Paywalled
resources remain eligible when otherwise useful, but both their access field
and paywall label/warning remain present. Community resources always retain the
community flag and label; the selector never promotes that label into evidence
authority.

## Boundaries

The policy is pure and offline. Step 26 adds no repository, SQLite migration,
network request, discovery provider, CLI/TUI surface, Source Kind, curriculum
compiler behavior, or learner-state mutation. Persistence or presentation of a
future reading list can consume this result later without changing the v1
selection rules.
