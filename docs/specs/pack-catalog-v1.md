# Learning Pack Catalog v1

## Purpose and trust boundary

`pack-catalog/v1` is discovery metadata. It is not a Learning Pack, plugin
marketplace, signature, trust grant, or installation authorization. Catalog
list/search never downloads, installs, activates, or executes an artifact.

A snapshot contains a UTC generation time and entries sorted by pack ID. Each
entry contains:

```text
pack ID
name and description
domain and target
maintainer
one or more available versions
source identity, locator, and trust status
```

Each available version declares its pack status, minimum Kelyro version,
compatibility result, and inert artifact locator. Versions and entries are
unique and deterministically ordered.

## Closed metadata

Source trust is one of:

```text
official
verified
community
unverified
```

Compatibility is one of:

```text
compatible
incompatible
unknown
```

Compatibility is recomputed locally from the running Kelyro SemVer and the
version's `minimum_kelyro_version`; catalog-provided compatibility is never
trusted. Development/unknown builds report `unknown`.

Pack temporal status uses the existing current, preview, experimental, legacy,
historical, and deprecated vocabulary. Trust status and temporal status answer
different questions and are never collapsed into one flag.

## Source and offline cache

The application consumes a `PackCatalogSource` abstraction. V1 includes a
strict bounded JSON document source; it does not prescribe a community
publication ecosystem or a network protocol. Unknown fields, trailing JSON,
invalid values, duplicate identities, and non-canonical ordering after
normalization are rejected.

The last valid normalized snapshot is written atomically to Foundation's global
cache as `pack-catalog-v1.json`. When the source is absent, unavailable, or
invalid, list/search uses that cache and reports offline mode plus a safe source
warning. A missing cache is a valid empty offline catalog.

## Search v1

`pack-catalog-v1` search is case-insensitive and whitespace-normalized. Every
query token must occur in the combined pack ID, name, description, domain,
target, maintainer, or source name. Results rank exact ID/name first, then
prefix matches, then ID/name substring matches, then other metadata matches;
ties use pack ID. The same snapshot and query always produce the same order.

The CLI surface is:

```text
kelyro packs catalog
kelyro packs search <query>
```

Users must still obtain and explicitly pass a pack path to `packs install`;
catalog artifact locators remain display metadata in I-04.
