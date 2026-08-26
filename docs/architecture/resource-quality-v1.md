# Resource Quality Model v1

`internal/research/quality.ModelV1` is Kelyro's deterministic rubric for the
technical and pedagogical usefulness of one reviewed resource. Its immutable
algorithm identifier is `resource-quality-v1`.

The model is pure domain policy. It performs no discovery, network access,
fetching, parsing, persistence, trust decision, freshness calculation,
verification, curriculum compilation, or student-state mutation. The zero
value of `ModelV1` is ready for use and produces the same result for the same
input.

## Reviewed input

Callers provide eight normalized scores in `[0,1]`:

| Dimension | Meaning | Direction |
| --- | --- | --- |
| accuracy confidence | confidence after review that the resource describes its subject correctly | higher is better |
| clarity | how understandable and unambiguous the explanation is | higher is better |
| specificity | how directly and concretely it addresses the intended subject | higher is better |
| depth | how completely it develops the relevant material | higher is better |
| maintainability | how likely links, structure, examples, and presentation are to remain usable | higher is better |
| examples | quality and usefulness of concrete examples | higher is better |
| accessibility | practical readability and availability to the intended learner | higher is better |
| noise | irrelevant, distracting, promotional, or low-signal material | lower is better |

`accuracy confidence` is not an `AuthorityTier`, `TrustDecision`, or
`ClaimConfidence`. It records a content-review judgment supplied to this
rubric. The model does not derive any dimension from a domain name, source
kind, publisher, popularity signal, discovery rank, or an `official` label.
Missing review must not be replaced with invented scores.

## Score

The overall score is a weighted sum:

```text
score = 0.25 * accuracy_confidence
      + 0.15 * clarity
      + 0.15 * specificity
      + 0.10 * depth
      + 0.10 * maintainability
      + 0.10 * examples
      + 0.10 * accessibility
      + 0.05 * (1 - noise)
```

The result is returned as a finite `research.QualityScore` in `[0,1]`.
Accuracy has the largest weight, but the score is not used alone for the
recommended use: explicit safety and specialized-use gates prevent strong
unrelated dimensions from hiding low accuracy or extreme noise.

## Recommended-use precedence

Rules are evaluated in this exact order. Comparisons on a stated boundary are
inclusive unless the rule says `below` or `above`.

1. `reject` when accuracy confidence is below `0.50`, the total score is below
   `0.40`, or noise is above `0.85`.
2. `example` when examples are at least `0.85`, specificity at least `0.75`,
   clarity at least `0.60`, accuracy confidence at least `0.70`, and noise at
   most `0.35`.
3. `further_reading` when the score is at least `0.70`, clarity at least
   `0.70`, accessibility at least `0.65`, maintainability at least `0.60`, and
   noise at most `0.40`.
4. `evidence` when accuracy confidence is at least `0.80`, specificity at
   least `0.75`, depth at least `0.65`, and noise at most `0.70`.
5. Every other non-rejected resource is `supplementary`.

Specialized pedagogical uses precede `evidence` deliberately. A clear official
tutorial can therefore be recommended as further reading, while a dense
technical specification with weak clarity, accessibility, and examples can
still be recommended for evidence. This recommendation says only that its
reviewed technical shape is suitable for that role. Creating Evidence still
requires a source snapshot and bounded extraction, and sustaining a Claim
still requires the independent trust, provenance, and verification contracts.

## Explainability and validation

Every assessment retains the eight input dimensions, the computed score, the
recommended use, the algorithm version, eight ordered dimension reasons, and
one terminal recommendation reason. Dimension reasons classify values below
`0.40` as low, values from `0.40` through below `0.70` as moderate, and values
from `0.70` as high. Noise reasons explicitly state that lower is better.

`Assessment.Validate` recomputes the score and recommendation, requires the
exact algorithm version, rejects unknown or duplicate reasons, and requires
one reason for every dimension plus the matching terminal reason. Returned
reason slices are caller-owned.

## Boundaries

Step 18 adds no repository, migration, network path, CLI/TUI command, or
automatic evaluator. A future adapter may help reviewers produce dimension
inputs, but it must preserve their provenance and may not promote discovery
metadata into evidence. Resource quality remains independent from authority,
freshness, refresh priority, release intelligence, and the future I-04
Curriculum Compiler.
