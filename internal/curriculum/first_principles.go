package curriculum

import "fmt"

const FirstPrinciplesExpansionVersionV1 = "first-principles-expansion-v1"

// FirstPrinciplesCandidate is a pack-authored root candidate. The application
// layer accepts it only when both the Concept and the proposed edge are backed
// by usable evidence from the compilation input.
type FirstPrinciplesCandidate struct {
	RequirementID            ID
	Concept                  Concept
	PrerequisiteKind         PrerequisiteKind
	PrerequisiteEvidenceRefs []EvidenceRef
	Reason                   string
}

func (candidate FirstPrinciplesCandidate) Validate() error {
	if err := candidate.RequirementID.Validate(); err != nil {
		return fmt.Errorf("first-principles candidate requirement: %w", err)
	}
	if err := candidate.Concept.Validate(); err != nil {
		return err
	}
	if candidate.Concept.Atomicity != AtomicityAtomic {
		return fmt.Errorf("first-principles candidate %q is not atomic", candidate.Concept.ID)
	}
	if !candidate.Concept.Foundational {
		return fmt.Errorf("first-principles candidate %q is not foundational", candidate.Concept.ID)
	}
	if err := candidate.PrerequisiteKind.Validate(); err != nil {
		return err
	}
	if err := validateEvidenceRefs("first-principles prerequisite evidence", candidate.PrerequisiteEvidenceRefs); err != nil {
		return err
	}
	return requireText("first-principles candidate reason", candidate.Reason)
}

type FirstPrinciplesResearchNeedCode string

const (
	FirstPrinciplesCandidateUnavailable FirstPrinciplesResearchNeedCode = "candidate_unavailable"
	FirstPrinciplesEvidenceInsufficient FirstPrinciplesResearchNeedCode = "evidence_insufficient"
	FirstPrinciplesImmutableConflict    FirstPrinciplesResearchNeedCode = "immutable_concept_conflict"
	FirstPrinciplesCyclePrevented       FirstPrinciplesResearchNeedCode = "cycle_prevented"
)

func (code FirstPrinciplesResearchNeedCode) Validate() error {
	switch code {
	case FirstPrinciplesCandidateUnavailable, FirstPrinciplesEvidenceInsufficient,
		FirstPrinciplesImmutableConflict, FirstPrinciplesCyclePrevented:
		return nil
	default:
		return fmt.Errorf("invalid first-principles research need code %q", code)
	}
}

type FirstPrinciplesResearchNeed struct {
	RequirementID       ID
	FoundationConceptID ConceptID
	TargetConceptIDs    []ConceptID
	Code                FirstPrinciplesResearchNeedCode
	Reason              string
}

func (need FirstPrinciplesResearchNeed) Validate() error {
	if err := need.RequirementID.Validate(); err != nil {
		return fmt.Errorf("first-principles research requirement: %w", err)
	}
	if err := need.FoundationConceptID.Validate(); err != nil {
		return fmt.Errorf("first-principles research foundation: %w", err)
	}
	if err := validateConceptIDs("first-principles research targets", need.TargetConceptIDs); err != nil {
		return err
	}
	if len(need.TargetConceptIDs) == 0 {
		return fmt.Errorf("first-principles research need has no target concepts")
	}
	if err := need.Code.Validate(); err != nil {
		return err
	}
	return requireText("first-principles research reason", need.Reason)
}

type FirstPrinciplesExpansionResult struct {
	ExpandedRootConcepts    []Concept
	ExpandedPrerequisites   []Prerequisite
	UnresolvedResearchNeeds []FirstPrinciplesResearchNeed
	AlgorithmVersion        string
}

func (result FirstPrinciplesExpansionResult) Validate() error {
	roots := make(map[ConceptID]struct{}, len(result.ExpandedRootConcepts))
	for _, concept := range result.ExpandedRootConcepts {
		if err := concept.Validate(); err != nil {
			return err
		}
		if concept.Atomicity != AtomicityAtomic || !concept.Foundational {
			return fmt.Errorf("expanded first-principles concept %q is not an atomic foundation", concept.ID)
		}
		if _, exists := roots[concept.ID]; exists {
			return fmt.Errorf("duplicate expanded first-principles concept %q", concept.ID)
		}
		roots[concept.ID] = struct{}{}
	}
	edges := make(map[[2]ConceptID]struct{}, len(result.ExpandedPrerequisites))
	for _, prerequisite := range result.ExpandedPrerequisites {
		if err := prerequisite.Validate(); err != nil {
			return err
		}
		key := [2]ConceptID{prerequisite.ConceptID, prerequisite.RequiredConceptID}
		if _, exists := edges[key]; exists {
			return fmt.Errorf("duplicate first-principles prerequisite from %q to %q", prerequisite.ConceptID, prerequisite.RequiredConceptID)
		}
		edges[key] = struct{}{}
	}
	needs := make(map[string]struct{}, len(result.UnresolvedResearchNeeds))
	for _, need := range result.UnresolvedResearchNeeds {
		if err := need.Validate(); err != nil {
			return err
		}
		key := need.RequirementID.String() + "\x00" + string(need.Code)
		if _, exists := needs[key]; exists {
			return fmt.Errorf("duplicate first-principles research need for requirement %q", need.RequirementID)
		}
		needs[key] = struct{}{}
	}
	if result.AlgorithmVersion != FirstPrinciplesExpansionVersionV1 {
		return fmt.Errorf("unsupported first-principles expansion version %q", result.AlgorithmVersion)
	}
	return nil
}
