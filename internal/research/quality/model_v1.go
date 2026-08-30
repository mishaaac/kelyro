package quality

import (
	"fmt"
	"math"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
)

const AlgorithmVersionV1 = "resource-quality-v1"

const (
	accuracyWeight        = 0.25
	clarityWeight         = 0.15
	specificityWeight     = 0.15
	depthWeight           = 0.10
	maintainabilityWeight = 0.10
	examplesWeight        = 0.10
	accessibilityWeight   = 0.10
	signalWeight          = 0.05
)

// Dimensions contains reviewed facts about a resource. Noise is the only
// inverse dimension: zero means no noise and one means unusably noisy.
type Dimensions struct {
	AccuracyConfidence research.QualityScore
	Clarity            research.QualityScore
	Specificity        research.QualityScore
	Depth              research.QualityScore
	Maintainability    research.QualityScore
	Examples           research.QualityScore
	Accessibility      research.QualityScore
	Noise              research.QualityScore
}

func (dimensions Dimensions) Validate() error {
	checks := []struct {
		name  string
		score research.QualityScore
	}{
		{"accuracy confidence", dimensions.AccuracyConfidence},
		{"clarity", dimensions.Clarity},
		{"specificity", dimensions.Specificity},
		{"depth", dimensions.Depth},
		{"maintainability", dimensions.Maintainability},
		{"examples", dimensions.Examples},
		{"accessibility", dimensions.Accessibility},
		{"noise", dimensions.Noise},
	}
	for _, check := range checks {
		if err := check.score.Validate(); err != nil {
			return fmt.Errorf("quality %s: %w", check.name, err)
		}
	}
	return nil
}

type RecommendedUse string

const (
	UseEvidence       RecommendedUse = "evidence"
	UseFurtherReading RecommendedUse = "further_reading"
	UseExample        RecommendedUse = "example"
	UseSupplementary  RecommendedUse = "supplementary"
	UseReject         RecommendedUse = "reject"
)

func (use RecommendedUse) Validate() error {
	switch use {
	case UseEvidence, UseFurtherReading, UseExample, UseSupplementary, UseReject:
		return nil
	default:
		return fmt.Errorf("invalid resource quality recommended use %q", use)
	}
}

type ReasonCode string

const (
	ReasonAccuracyConfidence ReasonCode = "dimension.accuracy_confidence"
	ReasonClarity            ReasonCode = "dimension.clarity"
	ReasonSpecificity        ReasonCode = "dimension.specificity"
	ReasonDepth              ReasonCode = "dimension.depth"
	ReasonMaintainability    ReasonCode = "dimension.maintainability"
	ReasonExamples           ReasonCode = "dimension.examples"
	ReasonAccessibility      ReasonCode = "dimension.accessibility"
	ReasonNoise              ReasonCode = "dimension.noise"
	ReasonUseEvidence        ReasonCode = "recommendation.evidence"
	ReasonUseFurtherReading  ReasonCode = "recommendation.further_reading"
	ReasonUseExample         ReasonCode = "recommendation.example"
	ReasonUseSupplementary   ReasonCode = "recommendation.supplementary"
	ReasonUseReject          ReasonCode = "recommendation.reject"
)

func (code ReasonCode) Validate() error {
	switch code {
	case ReasonAccuracyConfidence, ReasonClarity, ReasonSpecificity, ReasonDepth,
		ReasonMaintainability, ReasonExamples, ReasonAccessibility, ReasonNoise,
		ReasonUseEvidence, ReasonUseFurtherReading, ReasonUseExample,
		ReasonUseSupplementary, ReasonUseReject:
		return nil
	default:
		return fmt.Errorf("invalid resource quality reason code %q", code)
	}
}

type Reason struct {
	Code   ReasonCode
	Detail string
}

func (reason Reason) Validate() error {
	if err := reason.Code.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(reason.Detail) == "" || reason.Detail != strings.TrimSpace(reason.Detail) {
		return fmt.Errorf("resource quality reason detail is invalid")
	}
	return nil
}

type Input struct {
	Dimensions Dimensions
}

func (input Input) Validate() error {
	return input.Dimensions.Validate()
}

type Assessment struct {
	Score            research.QualityScore
	Dimensions       Dimensions
	Reasons          []Reason
	RecommendedUse   RecommendedUse
	AlgorithmVersion string
}

func (assessment Assessment) Validate() error {
	if err := assessment.Score.Validate(); err != nil {
		return fmt.Errorf("resource quality score: %w", err)
	}
	if err := assessment.Dimensions.Validate(); err != nil {
		return err
	}
	if err := assessment.RecommendedUse.Validate(); err != nil {
		return err
	}
	if assessment.AlgorithmVersion != AlgorithmVersionV1 {
		return fmt.Errorf("resource quality algorithm version must be %q", AlgorithmVersionV1)
	}

	expectedScore := weightedScore(assessment.Dimensions)
	if math.Abs(assessment.Score.Value()-expectedScore) > 1e-12 {
		return fmt.Errorf("resource quality score does not match %s dimensions", AlgorithmVersionV1)
	}
	expectedUse := recommend(assessment.Dimensions, expectedScore)
	if assessment.RecommendedUse != expectedUse {
		return fmt.Errorf("resource quality recommended use does not match %s rules", AlgorithmVersionV1)
	}

	required := map[ReasonCode]struct{}{
		ReasonAccuracyConfidence: {}, ReasonClarity: {}, ReasonSpecificity: {},
		ReasonDepth: {}, ReasonMaintainability: {}, ReasonExamples: {},
		ReasonAccessibility: {}, ReasonNoise: {}, recommendationReason(expectedUse): {},
	}
	if len(assessment.Reasons) != len(required) {
		return fmt.Errorf("resource quality assessment must contain eight dimension reasons and one recommendation reason")
	}
	seen := make(map[ReasonCode]struct{}, len(assessment.Reasons))
	for _, reason := range assessment.Reasons {
		if err := reason.Validate(); err != nil {
			return err
		}
		if _, exists := required[reason.Code]; !exists {
			return fmt.Errorf("resource quality assessment contains unexpected reason %q", reason.Code)
		}
		if _, exists := seen[reason.Code]; exists {
			return fmt.Errorf("resource quality assessment contains duplicate or missing reasons")
		}
		seen[reason.Code] = struct{}{}
	}
	return nil
}

// ModelV1 is stateless. Callers provide reviewed dimension scores; the model
// does not infer them from authority, source kind, freshness, or popularity.
type ModelV1 struct{}

func (ModelV1) Assess(input Input) (Assessment, error) {
	if err := input.Validate(); err != nil {
		return Assessment{}, fmt.Errorf("assess %s: %w", AlgorithmVersionV1, err)
	}
	scoreValue := weightedScore(input.Dimensions)
	score, err := research.NewQualityScore(scoreValue)
	if err != nil {
		return Assessment{}, fmt.Errorf("assess %s score: %w", AlgorithmVersionV1, err)
	}
	use := recommend(input.Dimensions, scoreValue)
	assessment := Assessment{
		Score: score, Dimensions: input.Dimensions, RecommendedUse: use,
		AlgorithmVersion: AlgorithmVersionV1,
		Reasons:          dimensionReasons(input.Dimensions),
	}
	assessment.Reasons = append(assessment.Reasons, useReason(use))
	if err := assessment.Validate(); err != nil {
		return Assessment{}, fmt.Errorf("assess %s output: %w", AlgorithmVersionV1, err)
	}
	assessment.Reasons = append([]Reason(nil), assessment.Reasons...)
	return assessment, nil
}

func weightedScore(dimensions Dimensions) float64 {
	value := dimensions.AccuracyConfidence.Value()*accuracyWeight +
		dimensions.Clarity.Value()*clarityWeight +
		dimensions.Specificity.Value()*specificityWeight +
		dimensions.Depth.Value()*depthWeight +
		dimensions.Maintainability.Value()*maintainabilityWeight +
		dimensions.Examples.Value()*examplesWeight +
		dimensions.Accessibility.Value()*accessibilityWeight +
		(1-dimensions.Noise.Value())*signalWeight
	return math.Max(0, math.Min(1, value))
}

func recommend(dimensions Dimensions, score float64) RecommendedUse {
	accuracy := dimensions.AccuracyConfidence.Value()
	noise := dimensions.Noise.Value()
	if accuracy < 0.50 || score < 0.40 || noise > 0.85 {
		return UseReject
	}
	if dimensions.Examples.Value() >= 0.85 && dimensions.Specificity.Value() >= 0.75 &&
		dimensions.Clarity.Value() >= 0.60 && accuracy >= 0.70 && noise <= 0.35 {
		return UseExample
	}
	if score >= 0.70 && dimensions.Clarity.Value() >= 0.70 &&
		dimensions.Accessibility.Value() >= 0.65 && dimensions.Maintainability.Value() >= 0.60 && noise <= 0.40 {
		return UseFurtherReading
	}
	if accuracy >= 0.80 && dimensions.Specificity.Value() >= 0.75 &&
		dimensions.Depth.Value() >= 0.65 && noise <= 0.70 {
		return UseEvidence
	}
	return UseSupplementary
}

func dimensionReasons(dimensions Dimensions) []Reason {
	return []Reason{
		dimensionReason(ReasonAccuracyConfidence, "Accuracy confidence", dimensions.AccuracyConfidence.Value(), false),
		dimensionReason(ReasonClarity, "Clarity", dimensions.Clarity.Value(), false),
		dimensionReason(ReasonSpecificity, "Specificity", dimensions.Specificity.Value(), false),
		dimensionReason(ReasonDepth, "Depth", dimensions.Depth.Value(), false),
		dimensionReason(ReasonMaintainability, "Maintainability", dimensions.Maintainability.Value(), false),
		dimensionReason(ReasonExamples, "Examples", dimensions.Examples.Value(), false),
		dimensionReason(ReasonAccessibility, "Accessibility", dimensions.Accessibility.Value(), false),
		dimensionReason(ReasonNoise, "Noise", dimensions.Noise.Value(), true),
	}
}

func dimensionReason(code ReasonCode, label string, value float64, inverse bool) Reason {
	band := "low"
	if value >= 0.70 {
		band = "high"
	} else if value >= 0.40 {
		band = "moderate"
	}
	detail := fmt.Sprintf("%s is %s (%.2f).", label, band, value)
	if inverse {
		detail = fmt.Sprintf("%s is %s (%.2f); lower is better.", label, band, value)
	}
	return Reason{Code: code, Detail: detail}
}

func recommendationReason(use RecommendedUse) ReasonCode {
	switch use {
	case UseEvidence:
		return ReasonUseEvidence
	case UseFurtherReading:
		return ReasonUseFurtherReading
	case UseExample:
		return ReasonUseExample
	case UseSupplementary:
		return ReasonUseSupplementary
	default:
		return ReasonUseReject
	}
}

func useReason(use RecommendedUse) Reason {
	details := map[RecommendedUse]string{
		UseEvidence:       "Technical strength supports evidence use; authority and claim verification remain separate requirements.",
		UseFurtherReading: "Clarity, accessibility, maintainability, and low noise support further reading.",
		UseExample:        "Concrete, specific examples support example use.",
		UseSupplementary:  "The resource is usable but does not meet a stronger specialized-use gate.",
		UseReject:         "Accuracy confidence, overall quality, or noise crosses a rejection gate.",
	}
	return Reason{Code: recommendationReason(use), Detail: details[use]}
}
