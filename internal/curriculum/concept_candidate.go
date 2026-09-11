package curriculum

import "fmt"

const ConceptCandidateExtractorVersionV1 = "claim-to-concept-candidates-v1"

// ConceptCandidate groups evidence about one semantic subject. It is an input
// to atomicity assessment, not yet a learner-trackable Concept.
type ConceptCandidate struct {
	ID           ID
	Title        string
	Scope        string
	ClaimRefs    []EvidenceRef
	VersionScope string
	Status       ConceptStatus
}

func (candidate ConceptCandidate) Validate() error {
	if err := candidate.ID.Validate(); err != nil {
		return fmt.Errorf("concept candidate: %w", err)
	}
	if err := requireText("concept candidate title", candidate.Title); err != nil {
		return err
	}
	if err := requireText("concept candidate scope", candidate.Scope); err != nil {
		return err
	}
	if err := candidate.Status.Validate(); err != nil {
		return err
	}
	if err := validateEvidenceRefs("concept candidate claims", candidate.ClaimRefs); err != nil {
		return err
	}
	if len(candidate.ClaimRefs) == 0 {
		return fmt.Errorf("concept candidate %q has no claims", candidate.ID)
	}
	return nil
}

type ConceptCandidateSet struct {
	Candidates       []ConceptCandidate
	AlgorithmVersion string
}

func (set ConceptCandidateSet) Validate() error {
	if len(set.Candidates) == 0 {
		return fmt.Errorf("concept candidate set is empty")
	}
	seen := make(map[ID]struct{}, len(set.Candidates))
	for _, candidate := range set.Candidates {
		if err := candidate.Validate(); err != nil {
			return err
		}
		if _, exists := seen[candidate.ID]; exists {
			return fmt.Errorf("concept candidate set contains duplicate candidate %q", candidate.ID)
		}
		seen[candidate.ID] = struct{}{}
	}
	if set.AlgorithmVersion != ConceptCandidateExtractorVersionV1 {
		return fmt.Errorf("unsupported concept candidate extractor version %q", set.AlgorithmVersion)
	}
	return nil
}
