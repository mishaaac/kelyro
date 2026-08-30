# Community Resource Policy v1

`internal/research/community.PolicyV1` is Kelyro's deterministic classifier for
reviewed community resources. Its immutable identifier is
`community-resource-policy-v1`. It permits useful community material without
treating it as equivalent to normative or official documentation.

The policy is pure and offline. It performs no discovery, fetching, transcript
processing, persistence, automatic reputation lookup, evidence extraction,
curriculum compilation, or student-state mutation.

## Resource and contribution types

The closed resource vocabulary maps to existing source kinds:

| Community resource | Required source kind |
| --- | --- |
| `blog`, `community_tutorial` | `community_article` |
| `forum`, `question_answer` | `community_forum` |
| `conference_talk` | `video` |
| `repository_example` | `source_code` |

The separate contribution kind is `resource` or `comment`. This distinction is
important for a forum or Q&A: the reviewed original resource may be useful as a
supplement, while an individual comment is always `context_only`, tier D, and
requires verification. Comments never become strong evidence merely because
they are attached to a high-quality discussion.

## Input and attribution

Callers provide a validated `Source`, topic, resource and contribution types,
independently reviewed `FreshnessState`, optional author and organization, an
optional already-selected `AuthorityProfile`, and optional engagement counts.
The output always preserves explicit attribution:

```text
title
author (optional)
organization (optional)
publisher (optional)
canonical source locator
```

Author and organization are descriptive reviewed facts. Their absence is not
filled with an invented identity. The source title and locator remain required,
so a selected resource can always be attributed clearly.

## Roles and authority elevation

Every non-comment community resource starts as `supplementary`, tier D. It may
become `recognized_supplementary`, tier C, only when all of these facts hold:

1. the supplied Authority Profile validates and matches the research topic;
2. the resource's source kind is explicitly in `PreferredKinds`; and
3. either the reviewed organization/publisher matches a preferred organization
   or the source host matches a preferred domain.

Elevation never yields tier A/B, a primary role, or a normative status. A
profile that merely lists a kind as allowed supplementary does not elevate it.
Trust Policy consumes the assessment's conservative tier and role, while
registry context may still lower authority.

## Freshness and popularity

Freshness is an explicit input. `stale` and `unknown` resources require
verification; `aging` remains visible without being treated as stale. The
policy does not derive freshness from publication date or engagement.

Votes and views are accepted only as optional engagement metadata. Their exact
values are deliberately absent from every authority, role, and verification
rule. When present, the policy emits `popularity.ignored`; equal reviewed input
with zero or millions of interactions produces the same classification.
Popularity is never evidence of truth.

## Explainability and Trust Policy integration

The assessment retains source identity, resource/contribution type,
attribution, freshness, role, tier, elevation and verification flags, ordered
reasons, and the algorithm version. Required reasons explain resource type,
role, freshness, and attribution; the optional popularity reason records that
engagement was ignored.

`trust-policy-v1` may consume the assessment. It validates source identity and
freshness equality, uses the community tier instead of the generic source-kind
baseline, and forces context-only or otherwise flagged community material to
`requires_verification`. Recognized resources remain accepted supplements at
most; a community repository example cannot inherit the generic tier-B source
code baseline.

## Boundaries

Step 28 itself adds no source table columns, migration, network adapter,
CLI/TUI path, transcript storage, Source Bundle rewrite, Community popularity
ranking, Curriculum Compiler, or Student Core/mastery mutation. Step 29's
separate optional video contract now makes conference-talk affiliation
explicit: this community policy rejects an official video presented as a
community talk and still never reads or stores transcript text.
