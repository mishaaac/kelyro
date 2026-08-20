// Package developmentfixture provides the deterministic demo content used to
// exercise Student Core before real Learning Packs exist. It is not a
// curriculum compiler or a production content selector.
package developmentfixture

import (
	"fmt"
	"strings"

	"github.com/mishaaac/kelyro/internal/infra/curriculumyaml"
	"github.com/mishaaac/kelyro/internal/infra/diagnosticjson"
	"github.com/mishaaac/kelyro/internal/learning"
)

func FoundationDemo() (learning.Curriculum, learning.Diagnostic, error) {
	curriculum, err := curriculumyaml.Load(strings.NewReader(foundationCurriculumYAML))
	if err != nil {
		return learning.Curriculum{}, learning.Diagnostic{}, fmt.Errorf("load embedded development curriculum: %w", err)
	}
	diagnostic, err := diagnosticjson.Load(strings.NewReader(foundationDiagnosticJSON))
	if err != nil {
		return learning.Curriculum{}, learning.Diagnostic{}, fmt.Errorf("load embedded development diagnostic: %w", err)
	}
	if diagnostic.Curriculum != curriculum.Reference {
		return learning.Curriculum{}, learning.Diagnostic{}, fmt.Errorf("embedded development diagnostic does not match curriculum")
	}
	return curriculum, diagnostic, nil
}

const foundationCurriculumYAML = `contract_version: curriculum-consumption/v1
id: foundation-demo
version: 1.0.0
title: Quantitative Reasoning Foundations
description: A deterministic development curriculum for exercising Student Core contracts.
nodes:
  - id: phase.foundations
    type: phase
    title: Foundations
    description: Establish the vocabulary and reasoning habits used throughout the fixture.
    order: 0
    display:
      short_title: Foundations
    status:
      state: active
    version: 1.0.0

  - id: module.quantities
    type: module
    parent: phase.foundations
    title: Quantities and comparisons
    description: Represent quantities and compare them using ratios.
    order: 0
    display:
      short_title: Quantities
    status:
      state: active
    version: 1.0.0

  - id: lesson.proportional-reasoning
    type: lesson
    parent: module.quantities
    title: Proportional reasoning
    description: Build comparisons from quantities before applying formulas.
    order: 0
    display:
      short_title: Proportions
    status:
      state: active
    version: 1.0.0

  - id: topic.ratios
    type: topic
    parent: lesson.proportional-reasoning
    title: Ratios
    description: Express and interpret relationships between two quantities.
    order: 0
    display:
      short_title: Ratios
    status:
      state: active
    version: 1.0.0

  - id: concept.ratio-meaning
    type: concept
    parent: topic.ratios
    title: Meaning of a ratio
    description: Interpret a ratio as a multiplicative comparison between quantities.
    order: 0
    display:
      short_title: Ratio meaning
    status:
      state: active
    version: 1.0.0
    concept:
      objectives:
        - Explain a ratio in words using its two referenced quantities.
        - Distinguish multiplicative comparison from additive difference.
      difficulty: 1
      estimated_effort_minutes: 20
      theory_required: true
      assessment_expectations:
        - Correctly interpret ratios in at least two neutral real-world contexts.

  - id: concept.equivalent-ratios
    type: concept
    parent: topic.ratios
    title: Equivalent ratios
    description: Recognize and construct ratios that express the same relationship.
    order: 1
    display:
      short_title: Equivalence
    status:
      state: active
    version: 1.0.0
    concept:
      objectives:
        - Generate an equivalent ratio by scaling both quantities.
        - Explain why scaling only one quantity changes the relationship.
      prerequisites:
        - concept_id: concept.ratio-meaning
          requirement: mastered
      difficulty: 2
      estimated_effort_minutes: 30
      theory_required: true
      assessment_expectations:
        - Construct and justify equivalent ratios without relying on a single notation.
`

const foundationDiagnosticJSON = `{
  "contract_version": "diagnostic/v1",
  "scoring_version": "diagnostic-scoring/v1",
  "id": "foundation-demo-initial",
  "version": "1.0.0",
  "curriculum_id": "foundation-demo",
  "curriculum_version": "1.0.0",
  "title": "Quantitative reasoning initial diagnostic",
  "sections": [
    {
      "id": "section.ratios",
      "title": "Ratios",
      "items": [
        {
          "id": "item.ratio-comparison",
          "concept_id": "concept.ratio-meaning",
          "kind": "single_choice",
          "prompt": "A ratio compares two quantities in which way?",
          "options": [
            {"value": "multiplicative", "label": "Multiplicatively"},
            {"value": "additive", "label": "By their additive difference"}
          ],
          "accepted_answers": ["multiplicative"]
        },
        {
          "id": "item.ratio-words",
          "concept_id": "concept.ratio-meaning",
          "kind": "short_answer",
          "prompt": "Write the ratio 3:2 in words.",
          "accepted_answers": ["three to two"],
          "requirements": [
            {"item_id": "item.ratio-comparison", "minimum_score": 1.0}
          ]
        },
        {
          "id": "item.equivalent-selection",
          "concept_id": "concept.equivalent-ratios",
          "kind": "multiple_choice",
          "prompt": "Select every ratio equivalent to 2:3.",
          "options": [
            {"value": "4:6", "label": "4:6"},
            {"value": "6:9", "label": "6:9"},
            {"value": "4:5", "label": "4:5"}
          ],
          "accepted_answers": ["4:6", "6:9"],
          "requirements": [
            {"item_id": "item.ratio-comparison", "minimum_score": 1.0}
          ]
        },
        {
          "id": "item.equivalent-confidence",
          "concept_id": "concept.equivalent-ratios",
          "kind": "self_report",
          "prompt": "How comfortable are you explaining why two ratios are equivalent?",
          "options": [
            {"value": "new", "label": "This is new to me", "score": 0.0},
            {"value": "somewhat", "label": "Somewhat comfortable", "score": 0.5},
            {"value": "confident", "label": "Confident", "score": 1.0}
          ]
        }
      ]
    }
  ]
}
`
