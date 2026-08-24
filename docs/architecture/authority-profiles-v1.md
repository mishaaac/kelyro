# Authority Profiles v1

Authority Profiles make source preferences contextual to a research domain and
topic. They are declarative inputs to later source classification and to
`trust-policy-v1`; matching a profile does not make a candidate trustworthy and
does not create evidence.

## Contract and package boundaries

The versioned YAML contract is `authority-profiles/v1`.

```text
assets/research/authority-profiles/*.yaml
                    |
                    v
internal/infra/authorityyaml  strict YAML decoding
                    |
                    v
internal/research.AuthorityProfile
                    |
                    v
internal/research/authority   validation, matching, precedence
```

The domain and matcher use only the Go standard library. YAML remains an
infrastructure concern and reuses the already-pinned `go.yaml.in/yaml/v3`
dependency. The loader enables known-field checking, rejects empty and
multi-document input, validates the contract version, and returns a complete
immutable catalog.

## Profile fields

Each profile declares:

- a stable ID and explicit profile version;
- an exact research domain or the global `*` fallback;
- a topic pattern using only `*` wildcards;
- ordered preferred source kinds;
- optional preferred DNS host patterns and publisher/organization names;
- a minimum independent corroboration count of at least one;
- source kinds permitted only as supplements;
- the minimum contextual authority tier compatible with Trust Policy v1;
- a UTC creation timestamp.

Preferred DNS patterns are lowercase exact hosts such as `go.dev` or a single
leading-wildcard host such as `*.example.org`. They are data for later source
classification; this step does not resolve DNS, inspect URLs, fetch sources, or
consult a registry. A source kind cannot be both preferred and supplementary.

## Topic key and matching

The matcher canonicalizes case and repeated whitespace. A topic with a
technology uses this key:

```text
<technology>/<subject>
```

Without a technology, the key is only `<subject>`. Thus the data pattern
`go/*` matches any Go topic without putting Go-specific logic in the core.

Matching precedence is deterministic:

1. an exact research domain outranks the global `*` domain;
2. within the same domain specificity, the matching topic pattern with more
   literal characters outranks a broader pattern;
3. an equal-rank tie is resolved by stable profile ID order.

Catalog construction rejects duplicate profile IDs and duplicate normalized
domain/topic selectors, so two rules cannot silently claim the same selector.
If no selector matches, the matcher reports no profile; it does not invent a
default. Callers that need fallback must provide an explicit `*` profile.

## Reference fixture

`assets/research/authority-profiles/technology-software.yaml` contains a
general Software fallback and a more specific Go profile. The Go domains and
organizations are fixture data, not hardcoded matcher branches. A future pack
or installation can add a new domain such as medicine or law using the same
contract without changing the core.

## Persistence compatibility

SQLite migration v24 extends the v23 `authority_profiles` table with validated
JSON arrays for preferred domains, organizations, and supplementary kinds, plus
`minimum_corroboration`. Existing v23 records migrate with empty preference
arrays and corroboration `1`. The repository and memory fake preserve all
slices defensively.

## Deferred work

Authority Profiles v1 does not implement the Trusted Source Registry, source
discovery, URL-to-profile classification, network access, trust decisions,
freshness, verification, or curriculum compilation. Those remain separate
I-03/I-04 steps.
