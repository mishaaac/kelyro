# Video Learning Resources v1

`research.VideoSupplementMetadata` is Kelyro's host-neutral metadata contract
for a reviewed video supplement. Its immutable identifier is
`video-supplement-metadata-v1`. Video remains learning/supporting material by
default; transcript availability or an official publisher does not turn it
into primary evidence automatically.

## Normalized metadata

Common fields stay in the existing `Source` contract instead of being copied
into a second, contradictory record:

| Required metadata | Representation |
| --- | --- |
| video URL | `Source.Locator` and matching `VideoLocator` |
| title | `Source.Metadata.Title` |
| publisher | `Source.Metadata.Publisher` |
| published_at | `Source.Metadata.PublishedAt` |
| channel | `VideoSupplementMetadata.Channel` |
| duration | positive `DurationSeconds`, capped at seven days |
| description | optional UTF-8 text capped at 4096 bytes |
| official/community | closed `SourceAffiliation` |
| transcript availability | `available`, `partial`, `unavailable`, or `unknown` |

A Source with video metadata must have kind `video`, a matching canonical
locator, publisher, and `published_at`. Existing legacy video Sources without
the optional v1 record remain readable; absence is not filled with invented
channel, duration, affiliation, or transcript state.

## Transcript retention

The domain contains no transcript text field. Only the closed availability
state is serialized. A future explicitly authorized extraction path may retain
a bounded excerpt as Evidence under the normal copyright and provenance
contracts, but neither this Source record nor SQLite can store a full
transcript accidentally.

## Timestamp deep links

Up to 32 deep links may pair a strictly increasing positive offset in seconds
with a validated absolute locator. Each offset must fall before the declared
duration, and offsets and locators are unique.

URL timestamp syntax is provider-specific, so adapters supply the exact deep
link. `DeepLinkAt` returns a link only for a stored offset and never constructs
one. The domain therefore has no dependency on YouTube or any other host's
query/fragment convention.

## Encoding and persistence

The strict canonical JSON uses lower-case closed vocabularies, rejects unknown
fields and trailing data, and is capped at 16 KiB. Source title, publisher, and
publication time remain in their normalized columns. Migration v37 adds only
`sources.video_metadata_json`; existing rows receive an empty value, and a
constraint permits non-empty object JSON only for the physical `video` kind.
The adapter validates canonical metadata and its Source relationship on every
read and returns persistence failure for corrupt payloads.

Memory and SQLite repositories clone the deep-link slice defensively. No raw
video content or transcript is stored.

## Trust, freshness, community, and reading selection

`trust-policy-v1` assigns explicit official video tier B and community/legacy
video tier D, but all videos remain accepted supplements at most. It emits
`authority.video_official` or `authority.video_community`; stale, unknown,
uncorroborated, or otherwise weak inputs still require verification.

`freshness-v1` retains the existing 60-day default for video. Further Reading
requires the reviewed community marker to match the metadata affiliation and
keeps the student-visible community label. Community Resource Policy accepts a
conference talk only when explicit video metadata, if present, says community;
official conference videos are evaluated directly as video supplements.

## Boundaries

Step 29 adds no media provider, network call, transcript fetch/storage,
platform-specific parser, playback UI, popularity ranking, Source Diversity
policy, Source Bundle authority rewrite, Curriculum Compiler, or Student
Core/mastery mutation.
