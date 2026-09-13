package curriculum

import "fmt"

const (
	SecurityCoverageVersionV1       = "security-coverage-v1"
	SecurityEvidencePolicyVersionV1 = "security-evidence-v1"
)

type SecurityCategory string

const (
	SecurityInputValidation    SecurityCategory = "input_validation"
	SecurityAuthentication     SecurityCategory = "authentication"
	SecurityAuthorization      SecurityCategory = "authorization"
	SecuritySecrets            SecurityCategory = "secrets"
	SecurityDependencySecurity SecurityCategory = "dependency_security"
	SecurityDataProtection     SecurityCategory = "data_protection"
	SecuritySecureDefaults     SecurityCategory = "secure_defaults"
	SecurityThreatAwareness    SecurityCategory = "threat_awareness"
	SecuritySupplyChain        SecurityCategory = "supply_chain"
)

func (category SecurityCategory) Validate() error {
	switch category {
	case SecurityInputValidation, SecurityAuthentication, SecurityAuthorization,
		SecuritySecrets, SecurityDependencySecurity, SecurityDataProtection,
		SecuritySecureDefaults, SecurityThreatAwareness, SecuritySupplyChain:
		return nil
	default:
		return fmt.Errorf("invalid security category %q", category)
	}
}

type SecurityRequirement struct {
	ID           ID
	Category     SecurityCategory
	TargetKind   CoverageTargetKind
	TargetID     ID
	Description  string
	EvidenceRefs []EvidenceRef
}

func (requirement SecurityRequirement) Validate() error {
	if err := requirement.ID.Validate(); err != nil {
		return fmt.Errorf("security requirement: %w", err)
	}
	if err := requirement.Category.Validate(); err != nil {
		return err
	}
	if err := requirement.TargetKind.Validate(); err != nil {
		return err
	}
	if err := requirement.TargetID.Validate(); err != nil {
		return fmt.Errorf("security requirement target: %w", err)
	}
	if err := requireText("security requirement description", requirement.Description); err != nil {
		return err
	}
	if err := validateEvidenceRefs("security requirement evidence", requirement.EvidenceRefs); err != nil {
		return err
	}
	if len(requirement.EvidenceRefs) == 0 {
		return fmt.Errorf("security requirement %q has no evidence", requirement.ID)
	}
	return nil
}

type SecuritySupport struct {
	ID            ID
	RequirementID ID
	ConceptIDs    []ConceptID
	EvidenceRefs  []EvidenceRef
	Reason        string
}

func (support SecuritySupport) Validate() error {
	if err := support.ID.Validate(); err != nil {
		return fmt.Errorf("security support: %w", err)
	}
	if err := support.RequirementID.Validate(); err != nil {
		return fmt.Errorf("security support requirement: %w", err)
	}
	if err := validateConceptIDs("security support concepts", support.ConceptIDs); err != nil {
		return err
	}
	if len(support.ConceptIDs) == 0 {
		return fmt.Errorf("security support %q has no concepts", support.ID)
	}
	if err := validateEvidenceRefs("security support evidence", support.EvidenceRefs); err != nil {
		return err
	}
	if len(support.EvidenceRefs) == 0 {
		return fmt.Errorf("security support %q has no evidence", support.ID)
	}
	return requireText("security support reason", support.Reason)
}

type SecurityCoverageResult struct {
	RequirementID      ID
	Category           SecurityCategory
	Status             CoverageStatus
	VerifiedSupportIDs []ID
	RejectedSupportIDs []ID
	Reasons            []string
}

func (result SecurityCoverageResult) Validate() error {
	if err := result.RequirementID.Validate(); err != nil {
		return fmt.Errorf("security coverage requirement: %w", err)
	}
	if err := result.Category.Validate(); err != nil {
		return err
	}
	if result.Status != CoverageMissing && result.Status != CoverageCovered {
		return fmt.Errorf("security requirement must be missing or covered")
	}
	if err := validateIDs("verified security supports", result.VerifiedSupportIDs); err != nil {
		return err
	}
	if err := validateIDs("rejected security supports", result.RejectedSupportIDs); err != nil {
		return err
	}
	if err := validateTexts("security coverage reasons", result.Reasons); err != nil {
		return err
	}
	if len(result.Reasons) == 0 {
		return fmt.Errorf("security coverage result has no reasons")
	}
	if result.Status == CoverageCovered && len(result.VerifiedSupportIDs) == 0 {
		return fmt.Errorf("covered security requirement has no verified support")
	}
	if result.Status == CoverageMissing && len(result.VerifiedSupportIDs) != 0 {
		return fmt.Errorf("missing security requirement has verified support")
	}
	return nil
}

type SecurityCoverageReport struct {
	Results               []SecurityCoverageResult
	CoverageRequirements  []CoverageRequirement
	CoverageSupports      []CoverageSupport
	EvidencePolicyVersion string
	AlgorithmVersion      string
}

func (report SecurityCoverageReport) Validate() error {
	if len(report.Results) == 0 {
		return fmt.Errorf("security coverage report has no results")
	}
	requirements := make(map[ID]struct{}, len(report.CoverageRequirements))
	for _, requirement := range report.CoverageRequirements {
		if err := requirement.Validate(); err != nil {
			return err
		}
		if requirement.Dimension != CoverageSecurity {
			return fmt.Errorf("security report contains non-security requirement %q", requirement.ID)
		}
		if _, exists := requirements[requirement.ID]; exists {
			return fmt.Errorf("duplicate security coverage requirement %q", requirement.ID)
		}
		requirements[requirement.ID] = struct{}{}
	}
	seenResults := make(map[ID]struct{}, len(report.Results))
	for _, result := range report.Results {
		if err := result.Validate(); err != nil {
			return err
		}
		if _, exists := requirements[result.RequirementID]; !exists {
			return fmt.Errorf("security result references missing requirement %q", result.RequirementID)
		}
		if _, exists := seenResults[result.RequirementID]; exists {
			return fmt.Errorf("duplicate security result %q", result.RequirementID)
		}
		seenResults[result.RequirementID] = struct{}{}
	}
	if len(seenResults) != len(requirements) {
		return fmt.Errorf("security results do not cover every requirement")
	}
	seenSupports := make(map[ID]struct{}, len(report.CoverageSupports))
	for _, support := range report.CoverageSupports {
		if err := support.Validate(); err != nil {
			return err
		}
		if _, exists := requirements[support.RequirementID]; !exists {
			return fmt.Errorf("security support %q references missing requirement", support.ID)
		}
		if _, exists := seenSupports[support.ID]; exists {
			return fmt.Errorf("duplicate security coverage support %q", support.ID)
		}
		seenSupports[support.ID] = struct{}{}
	}
	if report.EvidencePolicyVersion != SecurityEvidencePolicyVersionV1 {
		return fmt.Errorf("unsupported security evidence policy %q", report.EvidencePolicyVersion)
	}
	if report.AlgorithmVersion != SecurityCoverageVersionV1 {
		return fmt.Errorf("unsupported security coverage version %q", report.AlgorithmVersion)
	}
	return nil
}
