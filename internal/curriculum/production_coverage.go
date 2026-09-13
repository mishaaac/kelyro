package curriculum

import "fmt"

const (
	ProductionCoverageVersionV1       = "production-coverage-v1"
	ProductionEvidencePolicyVersionV1 = "production-evidence-v1"
)

type ProductionCategory string

const (
	ProductionFailureModes        ProductionCategory = "failure_modes"
	ProductionPerformance         ProductionCategory = "performance"
	ProductionObservability       ProductionCategory = "observability"
	ProductionDeployment          ProductionCategory = "deployment"
	ProductionConfiguration       ProductionCategory = "configuration"
	ProductionMaintenance         ProductionCategory = "maintenance"
	ProductionDebugging           ProductionCategory = "debugging"
	ProductionReliability         ProductionCategory = "reliability"
	ProductionTradeoffs           ProductionCategory = "tradeoffs"
	ProductionOperationalConcerns ProductionCategory = "operational_concerns"
)

func (category ProductionCategory) Validate() error {
	switch category {
	case ProductionFailureModes, ProductionPerformance, ProductionObservability,
		ProductionDeployment, ProductionConfiguration, ProductionMaintenance,
		ProductionDebugging, ProductionReliability, ProductionTradeoffs,
		ProductionOperationalConcerns:
		return nil
	default:
		return fmt.Errorf("invalid production category %q", category)
	}
}

type ProductionRequirement struct {
	ID           ID
	Category     ProductionCategory
	TargetKind   CoverageTargetKind
	TargetID     ID
	Description  string
	EvidenceRefs []EvidenceRef
}

func (requirement ProductionRequirement) Validate() error {
	if err := requirement.ID.Validate(); err != nil {
		return fmt.Errorf("production requirement: %w", err)
	}
	if err := requirement.Category.Validate(); err != nil {
		return err
	}
	if err := requirement.TargetKind.Validate(); err != nil {
		return err
	}
	if err := requirement.TargetID.Validate(); err != nil {
		return fmt.Errorf("production requirement target: %w", err)
	}
	if err := requireText("production requirement description", requirement.Description); err != nil {
		return err
	}
	if err := validateEvidenceRefs("production requirement evidence", requirement.EvidenceRefs); err != nil {
		return err
	}
	if len(requirement.EvidenceRefs) == 0 {
		return fmt.Errorf("production requirement %q has no evidence", requirement.ID)
	}
	return nil
}

type ProductionSupport struct {
	ID            ID
	RequirementID ID
	ConceptIDs    []ConceptID
	EvidenceRefs  []EvidenceRef
	Reason        string
}

func (support ProductionSupport) Validate() error {
	if err := support.ID.Validate(); err != nil {
		return fmt.Errorf("production support: %w", err)
	}
	if err := support.RequirementID.Validate(); err != nil {
		return fmt.Errorf("production support requirement: %w", err)
	}
	if err := validateConceptIDs("production support concepts", support.ConceptIDs); err != nil {
		return err
	}
	if len(support.ConceptIDs) == 0 {
		return fmt.Errorf("production support %q has no concepts", support.ID)
	}
	if err := validateEvidenceRefs("production support evidence", support.EvidenceRefs); err != nil {
		return err
	}
	if len(support.EvidenceRefs) == 0 {
		return fmt.Errorf("production support %q has no evidence", support.ID)
	}
	return requireText("production support reason", support.Reason)
}

type ProductionCoverageResult struct {
	RequirementID      ID
	Category           ProductionCategory
	Status             CoverageStatus
	AdequateSupportIDs []ID
	RejectedSupportIDs []ID
	Reasons            []string
}

func (result ProductionCoverageResult) Validate() error {
	if err := result.RequirementID.Validate(); err != nil {
		return fmt.Errorf("production coverage requirement: %w", err)
	}
	if err := result.Category.Validate(); err != nil {
		return err
	}
	if result.Status != CoverageMissing && result.Status != CoverageCovered {
		return fmt.Errorf("production requirement must be missing or covered")
	}
	if err := validateIDs("adequate production supports", result.AdequateSupportIDs); err != nil {
		return err
	}
	if err := validateIDs("rejected production supports", result.RejectedSupportIDs); err != nil {
		return err
	}
	if err := validateTexts("production coverage reasons", result.Reasons); err != nil {
		return err
	}
	if len(result.Reasons) == 0 {
		return fmt.Errorf("production coverage result has no reasons")
	}
	if result.Status == CoverageCovered && len(result.AdequateSupportIDs) == 0 {
		return fmt.Errorf("covered production requirement has no adequate support")
	}
	if result.Status == CoverageMissing && len(result.AdequateSupportIDs) != 0 {
		return fmt.Errorf("missing production requirement has adequate support")
	}
	return nil
}

type ProductionCoverageReport struct {
	Results               []ProductionCoverageResult
	CoverageRequirements  []CoverageRequirement
	CoverageSupports      []CoverageSupport
	EvidencePolicyVersion string
	AlgorithmVersion      string
}

func (report ProductionCoverageReport) Validate() error {
	if len(report.Results) == 0 {
		return fmt.Errorf("production coverage report has no results")
	}
	requirements := make(map[ID]struct{}, len(report.CoverageRequirements))
	for _, requirement := range report.CoverageRequirements {
		if err := requirement.Validate(); err != nil {
			return err
		}
		if requirement.Dimension != CoverageProduction {
			return fmt.Errorf("production report contains non-production requirement %q", requirement.ID)
		}
		if _, exists := requirements[requirement.ID]; exists {
			return fmt.Errorf("duplicate production coverage requirement %q", requirement.ID)
		}
		requirements[requirement.ID] = struct{}{}
	}
	seenResults := make(map[ID]struct{}, len(report.Results))
	for _, result := range report.Results {
		if err := result.Validate(); err != nil {
			return err
		}
		if _, exists := requirements[result.RequirementID]; !exists {
			return fmt.Errorf("production result references missing requirement %q", result.RequirementID)
		}
		if _, exists := seenResults[result.RequirementID]; exists {
			return fmt.Errorf("duplicate production result %q", result.RequirementID)
		}
		seenResults[result.RequirementID] = struct{}{}
	}
	if len(seenResults) != len(requirements) {
		return fmt.Errorf("production results do not cover every requirement")
	}
	seenSupports := make(map[ID]struct{}, len(report.CoverageSupports))
	for _, support := range report.CoverageSupports {
		if err := support.Validate(); err != nil {
			return err
		}
		if _, exists := requirements[support.RequirementID]; !exists {
			return fmt.Errorf("production support %q references missing requirement", support.ID)
		}
		if _, exists := seenSupports[support.ID]; exists {
			return fmt.Errorf("duplicate production coverage support %q", support.ID)
		}
		seenSupports[support.ID] = struct{}{}
	}
	if report.EvidencePolicyVersion != ProductionEvidencePolicyVersionV1 {
		return fmt.Errorf("unsupported production evidence policy %q", report.EvidencePolicyVersion)
	}
	if report.AlgorithmVersion != ProductionCoverageVersionV1 {
		return fmt.Errorf("unsupported production coverage version %q", report.AlgorithmVersion)
	}
	return nil
}
