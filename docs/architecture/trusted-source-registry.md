# Trusted Source Registry

The Trusted Source Registry records reviewed knowledge about source families
and publishing organizations. It is contextual metadata, not evidence and not
an unconditional list of truth.

## Domain model

`SourceRegistryEntry` contains:

- a stable ID and organization name;
- one or more normalized canonical DNS domain rules;
- supported source kinds and reasoned authority hints;
- applicable research domains and topic patterns;
- notes, status, addition time, and last review time.

Registry status is closed and explicit:

```text
trusted  conditional  historical  deprecated  blocked
```

`trusted` does not automatically accept a source. `conditional`, `deprecated`,
and out-of-context `historical` entries require verification when supplied to
Trust Policy v1. `blocked` rejects the matched source family explicitly. An
authority hint may make a baseline tier more conservative, but can never
promote it above the tier derived from source kind and use case.

## Canonical domains and matching

`NewCanonicalDomain` lowercases DNS names and removes one terminal root dot.
Rules are either exact (`go.dev`) or explicitly subdomain-only
(`*.example.org`). A wildcard does not match its apex; an entry that owns both
must declare both rules.

The pure `internal/research/registry` catalog:

- rejects duplicate entry IDs and normalized domain rules;
- extracts the host from a validated `SourceLocator`;
- gives exact-host matches precedence over wildcard matches;
- gives a more specific wildcard host precedence over a broader one;
- returns blocked and historical entries without hiding their status;
- verifies source kind, research domain, and the same
  `<technology>/<subject>` topic-key convention used by Authority Profiles.

Matching only identifies applicable metadata. It does not fetch a URL, resolve
DNS, create evidence, or persist a `TrustDecision`.

## Trust Policy integration

`trust.Input.Registry` is optional. When present, validation proves that the
entry matches the source locator, source kind, research domain, and topic. The
decision then records ordered `registry.<status>` and
`registry.authority_hint` reasons. All prior freshness, relevance, directness,
stability, corroboration, metadata, and use-case rules continue to apply.

This keeps the policy equation explicit:

```text
registry context + topic + source kind + freshness + corroboration
                         -> trust-policy-v1 decision
```

The registry entry itself never becomes a citation or evidence record.

## Persistence and application boundary

Forward-only SQLite migration v25 creates `source_registry_entries`. Ordered
collections are stored as validated JSON arrays. Insert/update triggers reject
a canonical domain already owned by another entry, including concurrent writes
that bypass application prechecks. Status and organization have a deterministic
listing index.

`SourceRegistryRepository` is a separate narrow port with `Save`, `Get`, and
`List`. The memory and SQLite adapters preserve stable ID ordering and defensive
slice ownership. `SourceRegistryService` maps validation and persistence errors,
while `SourceRegistryStoreFactory` scopes the SQLite lifetime to one workspace
without exposing the database to CLI or application coordination.

## Initial CLI

The initial administrative surface is read-only:

```text
kelyro sources registry list
kelyro sources registry show <id>
```

List output includes identity, status, organization, and canonical domains.
Show output includes every reviewed field plus an explicit reminder that
registry metadata is not evidence or an automatic trust decision. An empty
registry is a valid offline state.

## Deferred behavior

This step does not add registry mutation commands, built-in network discovery,
privacy gates, HTTP clients, URL probing, automatic source enrollment,
freshness calculation, or curriculum compilation. Those remain separate
authorized steps.
