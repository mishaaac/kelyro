package curriculum

import "fmt"

const ExpertCoverageReviewVersionV1 = "expert-coverage-review-v1"

type ExpertCoverageFindingKind string

const (
	ExpertMissingAdvancedCompetency   ExpertCoverageFindingKind = "missing_advanced_competency"
	ExpertInsufficientDepth           ExpertCoverageFindingKind = "insufficient_depth"
	ExpertMissingProductionCapability ExpertCoverageFindingKind = "missing_production_capability"
)

func (kind ExpertCoverageFindingKind) Validate() error {
	switch kind {
	case ExpertMissingAdvancedCompetency, ExpertInsufficientDepth, ExpertMissingProductionCapability:
		return nil
	default:
		return fmt.Errorf("invalid expert coverage finding kind %q", kind)
	}
}

type ExpertCoverageFinding struct {
	Kind      ExpertCoverageFindingKind
	TargetID  ID
	Dimension CoverageDimension
	Reason    string
}

func (finding ExpertCoverageFinding) Validate() error {
	if err := finding.Kind.Validate(); err != nil {
		return err
	}
	if err := finding.TargetID.Validate(); err != nil {
		return fmt.Errorf("expert coverage finding target: %w", err)
	}
	if finding.Dimension != "" {
		if err := finding.Dimension.Validate(); err != nil {
			return err
		}
	}
	if finding.Kind == ExpertMissingProductionCapability {
		switch finding.Dimension {
		case CoverageProduction, CoverageSecurity, CoverageToolchain:
		default:
			return fmt.Errorf("missing production capability must identify production, security, or toolchain")
		}
	} else if finding.Dimension != "" {
		return fmt.Errorf("expert finding %q cannot identify a coverage dimension", finding.Kind)
	}
	return requireText("expert coverage finding reason", finding.Reason)
}

type ExpertCoverageReviewResult struct {
	Passed                bool
	Professional          bool
	ReviewedOutcomeIDs    []ID
	ReviewedCompetencyIDs []ID
	Findings              []ExpertCoverageFinding
	AdvisorNotes          []string
	AlgorithmVersion      string
}

func (result ExpertCoverageReviewResult) Validate() error {
	if result.AlgorithmVersion != ExpertCoverageReviewVersionV1 {
		return fmt.Errorf("unsupported expert coverage review version %q", result.AlgorithmVersion)
	}
	if result.Passed != (len(result.Findings) == 0) {
		return fmt.Errorf("expert coverage pass state does not match findings")
	}
	if err := validateIDs("expert reviewed outcomes", result.ReviewedOutcomeIDs); err != nil {
		return err
	}
	if err := validateIDs("expert reviewed competencies", result.ReviewedCompetencyIDs); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(result.Findings))
	for _, finding := range result.Findings {
		if err := finding.Validate(); err != nil {
			return err
		}
		key := string(finding.Kind) + "\x00" + finding.TargetID.String() + "\x00" + string(finding.Dimension)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate expert coverage finding %q", finding.Kind)
		}
		seen[key] = struct{}{}
	}
	return validateTexts("expert coverage advisor notes", result.AdvisorNotes)
}
