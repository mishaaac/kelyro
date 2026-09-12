package curriculum

import "fmt"

const GranularityGuardVersionV1 = "granularity-guard-v1"

type ConceptMergeProposal struct {
	ID             ID
	ConceptIDs     []ConceptID
	MergedCriteria AtomicConceptCriteria
}

func (proposal ConceptMergeProposal) Validate() error {
	if err := proposal.ID.Validate(); err != nil {
		return fmt.Errorf("concept merge proposal: %w", err)
	}
	if err := validateConceptIDs("concept merge proposal sources", proposal.ConceptIDs); err != nil {
		return err
	}
	if len(proposal.ConceptIDs) < 2 {
		return fmt.Errorf("concept merge proposal %q requires at least two concepts", proposal.ID)
	}
	return proposal.MergedCriteria.Validate()
}

// VisualConceptGroup groups stable Concept IDs under a LessonSpec without
// combining or replacing their learner-trackable identities.
type VisualConceptGroup struct {
	LessonID   ID
	ConceptIDs []ConceptID
}

func (group VisualConceptGroup) Validate() error {
	if err := group.LessonID.Validate(); err != nil {
		return fmt.Errorf("visual concept group lesson: %w", err)
	}
	if err := validateConceptIDs("visual concept group concepts", group.ConceptIDs); err != nil {
		return err
	}
	if len(group.ConceptIDs) < 2 {
		return fmt.Errorf("visual concept group %q requires at least two concepts", group.LessonID)
	}
	return nil
}

type GranularityWarning struct {
	Code       string
	ConceptIDs []ConceptID
	Message    string
}

func (warning GranularityWarning) Validate() error {
	if err := requireText("granularity warning code", warning.Code); err != nil {
		return err
	}
	if err := validateConceptIDs("granularity warning concepts", warning.ConceptIDs); err != nil {
		return err
	}
	return requireText("granularity warning message", warning.Message)
}

type ConceptMergeDecision struct {
	ProposalID ID
	Allowed    bool
	Reasons    []string
}

func (decision ConceptMergeDecision) Validate() error {
	if err := decision.ProposalID.Validate(); err != nil {
		return fmt.Errorf("concept merge decision: %w", err)
	}
	if err := validateUniqueTexts("concept merge decision reasons", decision.Reasons); err != nil {
		return err
	}
	if len(decision.Reasons) == 0 {
		return fmt.Errorf("concept merge decision %q has no reasons", decision.ProposalID)
	}
	return nil
}

type GranularityResult struct {
	Warnings           []GranularityWarning
	ForcedSplit        []ConceptID
	SafeVisualGrouping []VisualConceptGroup
	MergeDecisions     []ConceptMergeDecision
	AlgorithmVersion   string
}

func (result GranularityResult) Validate() error {
	for _, warning := range result.Warnings {
		if err := warning.Validate(); err != nil {
			return err
		}
	}
	if err := validateConceptIDs("granularity forced splits", result.ForcedSplit); err != nil {
		return err
	}
	seenLessons := make(map[ID]struct{}, len(result.SafeVisualGrouping))
	seenVisualConcepts := make(map[ConceptID]struct{})
	for _, group := range result.SafeVisualGrouping {
		if err := group.Validate(); err != nil {
			return err
		}
		if _, exists := seenLessons[group.LessonID]; exists {
			return fmt.Errorf("duplicate visual concept group for lesson %q", group.LessonID)
		}
		seenLessons[group.LessonID] = struct{}{}
		for _, conceptID := range group.ConceptIDs {
			if _, exists := seenVisualConcepts[conceptID]; exists {
				return fmt.Errorf("concept %q appears in multiple visual groups", conceptID)
			}
			seenVisualConcepts[conceptID] = struct{}{}
		}
	}
	seenProposals := make(map[ID]struct{}, len(result.MergeDecisions))
	for _, decision := range result.MergeDecisions {
		if err := decision.Validate(); err != nil {
			return err
		}
		if _, exists := seenProposals[decision.ProposalID]; exists {
			return fmt.Errorf("duplicate concept merge decision %q", decision.ProposalID)
		}
		seenProposals[decision.ProposalID] = struct{}{}
	}
	if result.AlgorithmVersion != GranularityGuardVersionV1 {
		return fmt.Errorf("unsupported granularity guard version %q", result.AlgorithmVersion)
	}
	return nil
}
