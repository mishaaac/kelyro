package curriculum

import "fmt"

const PackVersioningPolicyVersionV1 = "pack-versioning-policy-v1"

type PackChangeImpact string

const (
	PackImpactPatch PackChangeImpact = "patch"
	PackImpactMinor PackChangeImpact = "minor"
	PackImpactMajor PackChangeImpact = "major"
)

func (impact PackChangeImpact) Validate() error {
	switch impact {
	case PackImpactPatch, PackImpactMinor, PackImpactMajor:
		return nil
	default:
		return fmt.Errorf("invalid pack change impact %q", impact)
	}
}

type PackVersionTransition string

const (
	PackTransitionPatch      PackVersionTransition = "patch"
	PackTransitionMinor      PackVersionTransition = "minor"
	PackTransitionMajor      PackVersionTransition = "major"
	PackTransitionPrerelease PackVersionTransition = "prerelease_iteration"
	PackTransitionInvalid    PackVersionTransition = "invalid"
)

func (transition PackVersionTransition) Validate() error {
	switch transition {
	case PackTransitionPatch, PackTransitionMinor, PackTransitionMajor,
		PackTransitionPrerelease, PackTransitionInvalid:
		return nil
	default:
		return fmt.Errorf("invalid pack version transition %q", transition)
	}
}

type PackChangeClassification struct {
	ChangeID ID
	Kind     CurriculumChangeKind
	Impact   PackChangeImpact
	Reason   string
}

func (classification PackChangeClassification) Validate() error {
	if err := classification.ChangeID.Validate(); err != nil {
		return fmt.Errorf("pack change classification: %w", err)
	}
	if err := classification.Kind.Validate(); err != nil {
		return err
	}
	if err := classification.Impact.Validate(); err != nil {
		return err
	}
	return requireText("pack change classification reason", classification.Reason)
}

type PackVersioningDecision struct {
	CurrentVersion     PackVersion
	CandidateVersion   PackVersion
	ChangeImpact       PackChangeImpact
	RequiredTransition PackVersionTransition
	ActualTransition   PackVersionTransition
	Classifications    []PackChangeClassification
	Allowed            bool
	Reasons            []string
	AlgorithmVersion   string
}

func (decision PackVersioningDecision) Validate() error {
	if err := decision.CurrentVersion.Validate(); err != nil {
		return err
	}
	if err := decision.CandidateVersion.Validate(); err != nil {
		return err
	}
	if err := decision.ChangeImpact.Validate(); err != nil {
		return err
	}
	if err := decision.RequiredTransition.Validate(); err != nil {
		return err
	}
	if decision.RequiredTransition == PackTransitionInvalid || decision.RequiredTransition == PackTransitionPrerelease {
		return fmt.Errorf("required pack transition must be patch, minor, or major")
	}
	if err := decision.ActualTransition.Validate(); err != nil {
		return err
	}
	if len(decision.Classifications) == 0 {
		return fmt.Errorf("pack versioning decision has no classified changes")
	}
	seen := make(map[ID]struct{}, len(decision.Classifications))
	for _, classification := range decision.Classifications {
		if err := classification.Validate(); err != nil {
			return err
		}
		if _, exists := seen[classification.ChangeID]; exists {
			return fmt.Errorf("duplicate pack change classification %q", classification.ChangeID)
		}
		seen[classification.ChangeID] = struct{}{}
	}
	if err := validateTexts("pack versioning reasons", decision.Reasons); err != nil {
		return err
	}
	if len(decision.Reasons) == 0 {
		return fmt.Errorf("pack versioning decision has no reasons")
	}
	if decision.Allowed && decision.ActualTransition == PackTransitionInvalid {
		return fmt.Errorf("allowed pack versioning decision has invalid transition")
	}
	if decision.AlgorithmVersion != PackVersioningPolicyVersionV1 {
		return fmt.Errorf("unsupported pack versioning policy %q", decision.AlgorithmVersion)
	}
	return nil
}
