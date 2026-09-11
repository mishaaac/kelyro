package curriculum

import "fmt"

const AtomizerVersionV1 = "atomizer-v1"

// ConceptAtomizationHint is pack/domain-authored decomposition metadata. The
// atomizer validates it against candidate Claims; it never invents a split.
type ConceptAtomizationHint struct {
	CandidateID  ID
	ConceptID    ConceptID
	Title        string
	Definition   string
	Version      string
	Difficulty   Difficulty
	Foundational bool
	ClaimRefs    []EvidenceRef
	Criteria     AtomicConceptCriteria
}

func (hint ConceptAtomizationHint) Validate() error {
	if err := hint.CandidateID.Validate(); err != nil {
		return fmt.Errorf("atomization hint candidate: %w", err)
	}
	if err := hint.ConceptID.Validate(); err != nil {
		return fmt.Errorf("atomization hint concept: %w", err)
	}
	if err := requireText("atomization hint title", hint.Title); err != nil {
		return err
	}
	if err := requireText("atomization hint definition", hint.Definition); err != nil {
		return err
	}
	if err := requireText("atomization hint version", hint.Version); err != nil {
		return err
	}
	if err := hint.Difficulty.Validate(); err != nil {
		return err
	}
	if err := validateEvidenceRefs("atomization hint claims", hint.ClaimRefs); err != nil {
		return err
	}
	if len(hint.ClaimRefs) == 0 {
		return fmt.Errorf("atomization hint %q has no claims", hint.ConceptID)
	}
	return hint.Criteria.Validate()
}

type ConceptClaimMapping struct {
	ClaimRef   EvidenceRef
	ConceptIDs []ConceptID
}

func (mapping ConceptClaimMapping) Validate() error {
	if err := mapping.ClaimRef.Validate(); err != nil {
		return fmt.Errorf("atomization claim mapping: %w", err)
	}
	if err := validateConceptIDs("atomization claim mapping concepts", mapping.ConceptIDs); err != nil {
		return err
	}
	if len(mapping.ConceptIDs) == 0 {
		return fmt.Errorf("atomization claim mapping has no concepts")
	}
	return nil
}

type AtomicConceptSet struct {
	CandidateID      ID
	Concepts         []Concept
	SplitReasons     []string
	ClaimMapping     []ConceptClaimMapping
	PolicyVersion    string
	AlgorithmVersion string
}

func (set AtomicConceptSet) Validate() error {
	if err := set.CandidateID.Validate(); err != nil {
		return fmt.Errorf("atomic concept set candidate: %w", err)
	}
	if len(set.Concepts) == 0 {
		return fmt.Errorf("atomic concept set has no concepts")
	}
	concepts := make(map[ConceptID]struct{}, len(set.Concepts))
	conceptEvidence := make(map[ConceptID]map[EvidenceRef]struct{}, len(set.Concepts))
	for _, concept := range set.Concepts {
		if err := concept.Validate(); err != nil {
			return err
		}
		if concept.Atomicity != AtomicityAtomic {
			return fmt.Errorf("atomized concept %q is not atomic", concept.ID)
		}
		if _, exists := concepts[concept.ID]; exists {
			return fmt.Errorf("atomic concept set contains duplicate concept %q", concept.ID)
		}
		concepts[concept.ID] = struct{}{}
		refs := make(map[EvidenceRef]struct{}, len(concept.EvidenceRefs))
		for _, reference := range concept.EvidenceRefs {
			refs[reference] = struct{}{}
		}
		conceptEvidence[concept.ID] = refs
	}
	if err := validateUniqueTexts("atomization split reasons", set.SplitReasons); err != nil {
		return err
	}
	if len(set.Concepts) > 1 && len(set.SplitReasons) == 0 {
		return fmt.Errorf("split atomic concept set has no split reasons")
	}
	seenClaims := make(map[EvidenceRef]struct{}, len(set.ClaimMapping))
	mappedEvidence := make(map[ConceptID]map[EvidenceRef]struct{}, len(set.Concepts))
	for _, mapping := range set.ClaimMapping {
		if err := mapping.Validate(); err != nil {
			return err
		}
		if _, exists := seenClaims[mapping.ClaimRef]; exists {
			return fmt.Errorf("atomic concept set contains duplicate claim mapping")
		}
		seenClaims[mapping.ClaimRef] = struct{}{}
		for _, conceptID := range mapping.ConceptIDs {
			if _, exists := concepts[conceptID]; !exists {
				return fmt.Errorf("claim mapping references missing concept %q", conceptID)
			}
			if _, exists := conceptEvidence[conceptID][mapping.ClaimRef]; !exists {
				return fmt.Errorf("claim mapping for concept %q is absent from concept evidence", conceptID)
			}
			if mappedEvidence[conceptID] == nil {
				mappedEvidence[conceptID] = make(map[EvidenceRef]struct{})
			}
			mappedEvidence[conceptID][mapping.ClaimRef] = struct{}{}
		}
	}
	if len(set.ClaimMapping) == 0 {
		return fmt.Errorf("atomic concept set has no claim mapping")
	}
	for conceptID, references := range conceptEvidence {
		if len(references) != len(mappedEvidence[conceptID]) {
			return fmt.Errorf("atomized concept %q has unmapped evidence", conceptID)
		}
	}
	if set.PolicyVersion != AtomicConceptCriteriaVersionV1 {
		return fmt.Errorf("unsupported atomization policy version %q", set.PolicyVersion)
	}
	if set.AlgorithmVersion != AtomizerVersionV1 {
		return fmt.Errorf("unsupported atomizer version %q", set.AlgorithmVersion)
	}
	return nil
}
