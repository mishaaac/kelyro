# Curriculum Evidence Report v1

`curriculum-evidence-report/v1` is the deterministic human/machine-readable
summary of the frozen evidence behind one compiled curriculum. It is generated
only from a validated `CompilationResult` and its exact I-03-derived
`CurriculumEvidenceSet` inputs; it has no discovery or fetch capability.

## Machine-readable report

The report records:

- goal identity/title/domain and ordered competency summaries;
- concept and Source Bundle counts plus exact bundle/Claim references;
- referenced Claims supported by at least one bundle-authoritative primary
  source, expressed as counts and a ratio under
  `primary-source-coverage-v1`;
- per-bundle freshness state, score, verification time, and algorithm;
- absolute source citation URLs plus optional SHA-256-bound excerpts limited to
  512 UTF-8 bytes;
- resolved/unresolved conflicts and bundle caveats;
- legacy/historical/deprecated and preview/experimental Concept IDs;
- compiler gaps and the compiler/pass version list.

All collections with no semantic input order are sorted by stable IDs. The
report contains no Claim statements, Evidence excerpts, cached web bodies, or
full source documents.

## Human-readable report

The same value renders deterministically as `EVIDENCE.md`. It shows counts,
primary-source coverage, freshness, conflicts/caveats, temporal content, gaps,
and compiler pass versions. Source and Claim identities remain citations to the
durable evidence graph rather than copies of external prose.

The Learning Pack loader strictly decodes the JSON form, rejects unknown and
duplicate fields, confirms every curriculum evidence reference is reported,
and cross-checks goal, counts, Source Bundles, and compiler versions against
the curriculum and `curriculum-build-info/v1` entry.
