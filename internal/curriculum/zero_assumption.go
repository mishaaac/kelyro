package curriculum

import "fmt"

const ZeroAssumptionAuditVersionV1 = "zero-assumption-v1"

// LearnerExperienceProfile declares what a curriculum may assume about its
// intended learner. It is compiler input, not mutable student state.
type LearnerExperienceProfile string

const (
	LearnerProfileZero              LearnerExperienceProfile = "zero"
	LearnerProfileSomeExperience    LearnerExperienceProfile = "some_experience"
	LearnerProfileDomainExperienced LearnerExperienceProfile = "domain_experienced"
)

func (profile LearnerExperienceProfile) Validate() error {
	switch profile {
	case LearnerProfileZero, LearnerProfileSomeExperience, LearnerProfileDomainExperienced:
		return nil
	default:
		return fmt.Errorf("invalid learner experience profile %q", profile)
	}
}

// FoundationRequirement is a domain-profile-authored, evidence-backed
// foundation relevant to specific competencies. The audit never invents a
// universal list of terminal, filesystem, network, or tool concepts.
type FoundationRequirement struct {
	ID                  ID
	ConceptID           ConceptID
	TargetCompetencyIDs []ID
	Reason              string
	EvidenceRefs        []EvidenceRef
}

func (requirement FoundationRequirement) Validate() error {
	if err := requirement.ID.Validate(); err != nil {
		return fmt.Errorf("foundation requirement: %w", err)
	}
	if err := requirement.ConceptID.Validate(); err != nil {
		return fmt.Errorf("foundation requirement concept: %w", err)
	}
	if err := validateIDs("foundation target competencies", requirement.TargetCompetencyIDs); err != nil {
		return err
	}
	if len(requirement.TargetCompetencyIDs) == 0 {
		return fmt.Errorf("foundation requirement %q has no target competencies", requirement.ID)
	}
	if err := requireText("foundation requirement reason", requirement.Reason); err != nil {
		return err
	}
	if err := validateEvidenceRefs("foundation requirement evidence", requirement.EvidenceRefs); err != nil {
		return err
	}
	if len(requirement.EvidenceRefs) == 0 {
		return fmt.Errorf("foundation requirement %q has no evidence", requirement.ID)
	}
	return nil
}

// AssumptionBaseline freezes the domain profile and learner profile used by an
// audit. Non-zero profiles may explicitly name assumed concepts; zero may not.
type AssumptionBaseline struct {
	ID                   ID
	Version              string
	DomainProfileID      ID
	DomainProfileVersion string
	LearnerProfile       LearnerExperienceProfile
	AssumedConceptIDs    []ConceptID
	Requirements         []FoundationRequirement
}

func (baseline AssumptionBaseline) Validate() error {
	if err := baseline.ID.Validate(); err != nil {
		return fmt.Errorf("assumption baseline: %w", err)
	}
	if err := requireText("assumption baseline version", baseline.Version); err != nil {
		return err
	}
	if err := baseline.DomainProfileID.Validate(); err != nil {
		return fmt.Errorf("assumption baseline domain profile: %w", err)
	}
	if err := requireText("assumption baseline domain profile version", baseline.DomainProfileVersion); err != nil {
		return err
	}
	if err := baseline.LearnerProfile.Validate(); err != nil {
		return err
	}
	if err := validateConceptIDs("assumption baseline concepts", baseline.AssumedConceptIDs); err != nil {
		return err
	}
	if baseline.LearnerProfile == LearnerProfileZero && len(baseline.AssumedConceptIDs) != 0 {
		return fmt.Errorf("zero learner baseline cannot declare assumed concepts")
	}
	seen := make(map[ID]struct{}, len(baseline.Requirements))
	for _, requirement := range baseline.Requirements {
		if err := requirement.Validate(); err != nil {
			return err
		}
		if _, exists := seen[requirement.ID]; exists {
			return fmt.Errorf("assumption baseline contains duplicate requirement %q", requirement.ID)
		}
		seen[requirement.ID] = struct{}{}
	}
	if len(baseline.Requirements) == 0 {
		return fmt.Errorf("assumption baseline has no foundation requirements")
	}
	return nil
}

type ZeroAssumptionViolationCode string

const (
	ZeroAssumptionMissingFoundation   ZeroAssumptionViolationCode = "missing_foundation_concept"
	ZeroAssumptionNotFoundational     ZeroAssumptionViolationCode = "foundation_not_root"
	ZeroAssumptionMissingPrerequisite ZeroAssumptionViolationCode = "missing_foundation_prerequisite"
)

func (code ZeroAssumptionViolationCode) Validate() error {
	switch code {
	case ZeroAssumptionMissingFoundation, ZeroAssumptionNotFoundational, ZeroAssumptionMissingPrerequisite:
		return nil
	default:
		return fmt.Errorf("invalid zero-assumption violation code %q", code)
	}
}

type ZeroAssumptionViolation struct {
	Code                ZeroAssumptionViolationCode
	RequirementID       ID
	FoundationConceptID ConceptID
	TargetCompetencyID  ID
	TargetConceptID     ConceptID
	Severity            CurriculumAuditSeverity
	Reason              string
	EvidenceRefs        []EvidenceRef
}

func (violation ZeroAssumptionViolation) Validate() error {
	if err := violation.Code.Validate(); err != nil {
		return err
	}
	if err := violation.RequirementID.Validate(); err != nil {
		return fmt.Errorf("zero-assumption requirement: %w", err)
	}
	if err := violation.FoundationConceptID.Validate(); err != nil {
		return fmt.Errorf("zero-assumption foundation: %w", err)
	}
	if err := violation.TargetCompetencyID.Validate(); err != nil {
		return fmt.Errorf("zero-assumption target competency: %w", err)
	}
	if err := violation.TargetConceptID.Validate(); err != nil {
		return fmt.Errorf("zero-assumption target concept: %w", err)
	}
	if err := violation.Severity.Validate(); err != nil {
		return err
	}
	if err := requireText("zero-assumption violation reason", violation.Reason); err != nil {
		return err
	}
	return validateEvidenceRefs("zero-assumption violation evidence", violation.EvidenceRefs)
}

type ZeroAssumptionAuditResult struct {
	BaselineID          ID
	BaselineVersion     string
	LearnerProfile      LearnerExperienceProfile
	AuditedRequirements int
	AssumedRequirements int
	Passed              bool
	Violations          []ZeroAssumptionViolation
	AlgorithmVersion    string
}

func (result ZeroAssumptionAuditResult) Validate() error {
	if err := result.BaselineID.Validate(); err != nil {
		return fmt.Errorf("zero-assumption result baseline: %w", err)
	}
	if err := requireText("zero-assumption result baseline version", result.BaselineVersion); err != nil {
		return err
	}
	if err := result.LearnerProfile.Validate(); err != nil {
		return err
	}
	if result.AuditedRequirements < 0 || result.AssumedRequirements < 0 || result.AssumedRequirements > result.AuditedRequirements {
		return fmt.Errorf("invalid zero-assumption audit counts")
	}
	seen := make(map[string]struct{}, len(result.Violations))
	for _, violation := range result.Violations {
		if err := violation.Validate(); err != nil {
			return err
		}
		key := string(violation.Code) + "\x00" + violation.RequirementID.String() + "\x00" + violation.TargetCompetencyID.String() + "\x00" + violation.TargetConceptID.String()
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate zero-assumption violation for requirement %q", violation.RequirementID)
		}
		seen[key] = struct{}{}
	}
	if result.Passed != (len(result.Violations) == 0) {
		return fmt.Errorf("zero-assumption pass state does not match violations")
	}
	if result.AlgorithmVersion != ZeroAssumptionAuditVersionV1 {
		return fmt.Errorf("unsupported zero-assumption audit version %q", result.AlgorithmVersion)
	}
	return nil
}
