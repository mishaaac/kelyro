package curriculum

import "fmt"

const CurriculumReviewerVersionV1 = "curriculum-reviewer-v1"

type CurriculumReviewDecision string

const (
	ReviewApproved             CurriculumReviewDecision = "approved"
	ReviewApprovedWithWarnings CurriculumReviewDecision = "approved_with_warnings"
	ReviewRejected             CurriculumReviewDecision = "rejected"
)

func (decision CurriculumReviewDecision) Validate() error {
	switch decision {
	case ReviewApproved, ReviewApprovedWithWarnings, ReviewRejected:
		return nil
	default:
		return fmt.Errorf("invalid curriculum review decision %q", decision)
	}
}

type CurriculumReviewDimension string

const (
	ReviewCoverage            CurriculumReviewDimension = "coverage"
	ReviewGranularity         CurriculumReviewDimension = "granularity"
	ReviewPrerequisites       CurriculumReviewDimension = "prerequisites"
	ReviewDefinitionBeforeUse CurriculumReviewDimension = "definition_before_use"
	ReviewZeroAssumption      CurriculumReviewDimension = "zero_assumption"
	ReviewSourceReadiness     CurriculumReviewDimension = "source_readiness"
	ReviewFreshness           CurriculumReviewDimension = "freshness"
	ReviewSecurity            CurriculumReviewDimension = "security"
	ReviewProduction          CurriculumReviewDimension = "production"
	ReviewToolchain           CurriculumReviewDimension = "toolchain"
	ReviewTemporalStatus      CurriculumReviewDimension = "temporal_status"
)

func AllCurriculumReviewDimensions() []CurriculumReviewDimension {
	return []CurriculumReviewDimension{
		ReviewCoverage, ReviewGranularity, ReviewPrerequisites,
		ReviewDefinitionBeforeUse, ReviewZeroAssumption, ReviewSourceReadiness,
		ReviewFreshness, ReviewSecurity, ReviewProduction, ReviewToolchain,
		ReviewTemporalStatus,
	}
}

func (dimension CurriculumReviewDimension) Validate() error {
	for _, candidate := range AllCurriculumReviewDimensions() {
		if dimension == candidate {
			return nil
		}
	}
	return fmt.Errorf("invalid curriculum review dimension %q", dimension)
}

type CurriculumReviewSeverity string

const (
	ReviewWarning CurriculumReviewSeverity = "warning"
	ReviewError   CurriculumReviewSeverity = "error"
)

func (severity CurriculumReviewSeverity) Validate() error {
	switch severity {
	case ReviewWarning, ReviewError:
		return nil
	default:
		return fmt.Errorf("invalid curriculum review severity %q", severity)
	}
}

type CurriculumReviewFinding struct {
	Severity CurriculumReviewSeverity
	Code     string
	Target   string
	Reason   string
}

func (finding CurriculumReviewFinding) Validate() error {
	if err := finding.Severity.Validate(); err != nil {
		return err
	}
	for _, field := range []struct{ name, value string }{
		{"curriculum review finding code", finding.Code},
		{"curriculum review finding target", finding.Target},
		{"curriculum review finding reason", finding.Reason},
	} {
		if err := requireText(field.name, field.value); err != nil {
			return err
		}
	}
	return nil
}

type CurriculumReviewDimensionResult struct {
	Dimension CurriculumReviewDimension
	Passed    bool
	Findings  []CurriculumReviewFinding
}

func (result CurriculumReviewDimensionResult) Validate() error {
	if err := result.Dimension.Validate(); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(result.Findings))
	hasError := false
	for _, finding := range result.Findings {
		if err := finding.Validate(); err != nil {
			return err
		}
		key := string(finding.Severity) + "\x00" + finding.Code + "\x00" + finding.Target
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate curriculum review finding %q", finding.Code)
		}
		seen[key] = struct{}{}
		hasError = hasError || finding.Severity == ReviewError
	}
	if result.Passed == hasError {
		return fmt.Errorf("review dimension %q pass state does not match findings", result.Dimension)
	}
	return nil
}

type CurriculumReviewResult struct {
	Decision         CurriculumReviewDecision
	Dimensions       []CurriculumReviewDimensionResult
	AdvisorNotes     []string
	AlgorithmVersion string
}

func (result CurriculumReviewResult) Validate() error {
	if err := result.Decision.Validate(); err != nil {
		return err
	}
	if len(result.Dimensions) != len(AllCurriculumReviewDimensions()) {
		return fmt.Errorf("curriculum review must contain every dimension")
	}
	seen := make(map[CurriculumReviewDimension]struct{}, len(result.Dimensions))
	hasError, hasWarning := false, false
	for _, dimension := range result.Dimensions {
		if err := dimension.Validate(); err != nil {
			return err
		}
		if _, exists := seen[dimension.Dimension]; exists {
			return fmt.Errorf("duplicate curriculum review dimension %q", dimension.Dimension)
		}
		seen[dimension.Dimension] = struct{}{}
		for _, finding := range dimension.Findings {
			hasError = hasError || finding.Severity == ReviewError
			hasWarning = hasWarning || finding.Severity == ReviewWarning
		}
	}
	expected := ReviewApproved
	if hasError {
		expected = ReviewRejected
	} else if hasWarning {
		expected = ReviewApprovedWithWarnings
	}
	if result.Decision != expected {
		return fmt.Errorf("curriculum review decision %q does not match findings", result.Decision)
	}
	if err := validateTexts("curriculum review advisor notes", result.AdvisorNotes); err != nil {
		return err
	}
	if result.AlgorithmVersion != CurriculumReviewerVersionV1 {
		return fmt.Errorf("unsupported curriculum reviewer version %q", result.AlgorithmVersion)
	}
	return nil
}
