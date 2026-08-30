package quality

import (
	"math"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestModelV1ComputesDocumentedWeightedScore(t *testing.T) {
	t.Parallel()
	dimensions := qualityDimensions(t, .9, .8, .7, .6, .5, .4, .3, .2)
	got, err := (ModelV1{}).Assess(Input{Dimensions: dimensions})
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	want := .9*.25 + .8*.15 + .7*.15 + .6*.10 + .5*.10 + .4*.10 + .3*.10 + (1-.2)*.05
	if math.Abs(got.Score.Value()-want) > 1e-12 {
		t.Fatalf("score = %.12f, want %.12f", got.Score.Value(), want)
	}
	if got.AlgorithmVersion != AlgorithmVersionV1 || len(got.Reasons) != 9 {
		t.Fatalf("assessment metadata = %+v", got)
	}
	if got.Reasons[7].Code != ReasonNoise || !strings.Contains(got.Reasons[7].Detail, "lower is better") {
		t.Fatalf("noise reason = %+v", got.Reasons[7])
	}
}

func TestModelV1RecommendsSpecializedUsesWithoutAuthorityInput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		dimensions Dimensions
		want       RecommendedUse
	}{
		{
			name:       "dense technical resource remains evidence despite low pedagogy",
			dimensions: qualityDimensions(t, .95, .20, .90, .90, .60, 0, .20, .20),
			want:       UseEvidence,
		},
		{
			name:       "clear accessible resource is further reading",
			dimensions: qualityDimensions(t, .75, .90, .70, .65, .80, .65, .90, .15),
			want:       UseFurtherReading,
		},
		{
			name:       "concrete focused resource is an example",
			dimensions: qualityDimensions(t, .75, .75, .90, .50, .60, .95, .80, .15),
			want:       UseExample,
		},
		{
			name:       "usable resource without specialized strength is supplementary",
			dimensions: qualityDimensions(t, .60, .60, .60, .60, .60, .60, .60, .40),
			want:       UseSupplementary,
		},
		{
			name:       "low accuracy rejects even with high aggregate dimensions",
			dimensions: qualityDimensions(t, .30, 1, 1, 1, 1, 1, 1, 0),
			want:       UseReject,
		},
		{
			name:       "excessive noise rejects",
			dimensions: qualityDimensions(t, 1, 1, 1, 1, 1, 1, 1, .86),
			want:       UseReject,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := (ModelV1{}).Assess(Input{Dimensions: test.dimensions})
			if err != nil {
				t.Fatalf("Assess() error = %v", err)
			}
			if got.RecommendedUse != test.want {
				t.Fatalf("recommended use = %q, want %q (score %.3f)", got.RecommendedUse, test.want, got.Score.Value())
			}
			if got.Reasons[len(got.Reasons)-1].Code != recommendationReason(test.want) {
				t.Fatalf("recommendation reason = %+v", got.Reasons[len(got.Reasons)-1])
			}
		})
	}
}

func TestModelV1UsesInclusiveRecommendationBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		dimensions Dimensions
		want       RecommendedUse
	}{
		{"evidence boundary", qualityDimensions(t, .80, .20, .75, .65, .50, 0, .20, .70), UseEvidence},
		{"example boundary", qualityDimensions(t, .70, .60, .75, .50, .50, .85, .50, .35), UseExample},
		{"accuracy below acceptance", qualityDimensions(t, math.Nextafter(.50, 0), 1, 1, 1, 1, 1, 1, 0), UseReject},
		{"noise above rejection", qualityDimensions(t, 1, 1, 1, 1, 1, 1, 1, math.Nextafter(.85, 1)), UseReject},
	}
	for _, test := range tests {
		got, err := (ModelV1{}).Assess(Input{Dimensions: test.dimensions})
		if err != nil || got.RecommendedUse != test.want {
			t.Fatalf("Assess(%s) = (%+v, %v), want %q", test.name, got, err, test.want)
		}
	}
}

func TestAssessmentValidationRejectsDivergentOutput(t *testing.T) {
	t.Parallel()
	valid, err := (ModelV1{}).Assess(Input{Dimensions: qualityDimensions(t, .6, .6, .6, .6, .6, .6, .6, .4)})
	if err != nil {
		t.Fatal(err)
	}
	wrongScore, _ := research.NewQualityScore(.1)
	tests := []struct {
		name   string
		mutate func(Assessment) Assessment
		needle string
	}{
		{"algorithm", func(value Assessment) Assessment { value.AlgorithmVersion = "resource-quality-v2"; return value }, "algorithm version"},
		{"score", func(value Assessment) Assessment { value.Score = wrongScore; return value }, "score does not match"},
		{"use", func(value Assessment) Assessment { value.RecommendedUse = UseEvidence; return value }, "recommended use"},
		{"missing reason", func(value Assessment) Assessment { value.Reasons = value.Reasons[:8]; return value }, "eight dimension reasons"},
		{"duplicate reason", func(value Assessment) Assessment { value.Reasons[0] = value.Reasons[1]; return value }, "duplicate or missing"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Reasons = append([]Reason(nil), valid.Reasons...)
			candidate = test.mutate(candidate)
			if err := candidate.Validate(); err == nil || !strings.Contains(err.Error(), test.needle) {
				t.Fatalf("Validate() error = %v, want %q", err, test.needle)
			}
		})
	}
}

func TestQualityScoreRejectsNonFiniteAndOutOfRangeValues(t *testing.T) {
	t.Parallel()
	for _, value := range []float64{-0.01, 1.01, math.NaN(), math.Inf(1)} {
		if _, err := research.NewQualityScore(value); err == nil {
			t.Fatalf("NewQualityScore(%v) succeeded", value)
		}
	}
}

func TestModelV1ReturnsIndependentReasonSlices(t *testing.T) {
	t.Parallel()
	input := Input{Dimensions: qualityDimensions(t, .6, .6, .6, .6, .6, .6, .6, .4)}
	first, err := (ModelV1{}).Assess(input)
	if err != nil {
		t.Fatal(err)
	}
	first.Reasons[0].Detail = "mutated"
	second, err := (ModelV1{}).Assess(input)
	if err != nil {
		t.Fatal(err)
	}
	if second.Reasons[0].Detail == "mutated" {
		t.Fatal("Assess() reused mutable reason storage")
	}
}

func qualityDimensions(t *testing.T, values ...float64) Dimensions {
	t.Helper()
	if len(values) != 8 {
		t.Fatalf("qualityDimensions received %d values", len(values))
	}
	scores := make([]research.QualityScore, len(values))
	for index, value := range values {
		score, err := research.NewQualityScore(value)
		if err != nil {
			t.Fatalf("quality score %d: %v", index, err)
		}
		scores[index] = score
	}
	return Dimensions{
		AccuracyConfidence: scores[0], Clarity: scores[1], Specificity: scores[2],
		Depth: scores[3], Maintainability: scores[4], Examples: scores[5],
		Accessibility: scores[6], Noise: scores[7],
	}
}
