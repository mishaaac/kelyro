package curriculum

import "fmt"

const PrerequisiteExtractorVersionV1 = "prerequisite-extractor-v1"

// ConceptPrerequisiteSemantic is an evidence-backed, domain-authored semantic
// dependency. It is independent of all visual hierarchy ordering.
type ConceptPrerequisiteSemantic struct {
	ConceptID         ConceptID
	RequiredConceptID ConceptID
	Kind              PrerequisiteKind
	EvidenceRefs      []EvidenceRef
	Reason            string
}

func (semantic ConceptPrerequisiteSemantic) Validate() error {
	edge := Prerequisite{
		ConceptID: semantic.ConceptID, RequiredConceptID: semantic.RequiredConceptID,
		Kind: semantic.Kind, EvidenceRefs: semantic.EvidenceRefs,
	}
	if err := edge.Validate(); err != nil {
		return err
	}
	if len(semantic.EvidenceRefs) == 0 {
		return fmt.Errorf("prerequisite semantic from %q to %q has no evidence", semantic.ConceptID, semantic.RequiredConceptID)
	}
	return requireText("prerequisite semantic reason", semantic.Reason)
}

type PrerequisiteDerivation struct {
	Prerequisite Prerequisite
	Reason       string
}

func (derivation PrerequisiteDerivation) Validate() error {
	if err := derivation.Prerequisite.Validate(); err != nil {
		return err
	}
	if len(derivation.Prerequisite.EvidenceRefs) == 0 {
		return fmt.Errorf("derived prerequisite has no evidence")
	}
	return requireText("prerequisite derivation reason", derivation.Reason)
}

type PrerequisiteExtraction struct {
	Derivations      []PrerequisiteDerivation
	AlgorithmVersion string
}

func (extraction PrerequisiteExtraction) Validate() error {
	type edgeKey struct {
		concept  ConceptID
		required ConceptID
		kind     PrerequisiteKind
	}
	seen := make(map[edgeKey]struct{}, len(extraction.Derivations))
	for _, derivation := range extraction.Derivations {
		if err := derivation.Validate(); err != nil {
			return err
		}
		edge := derivation.Prerequisite
		key := edgeKey{concept: edge.ConceptID, required: edge.RequiredConceptID, kind: edge.Kind}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate derived prerequisite from %q to %q", edge.ConceptID, edge.RequiredConceptID)
		}
		seen[key] = struct{}{}
	}
	if extraction.AlgorithmVersion != PrerequisiteExtractorVersionV1 {
		return fmt.Errorf("unsupported prerequisite extractor version %q", extraction.AlgorithmVersion)
	}
	return nil
}

func (extraction PrerequisiteExtraction) Edges() []Prerequisite {
	result := make([]Prerequisite, len(extraction.Derivations))
	for index, derivation := range extraction.Derivations {
		result[index] = derivation.Prerequisite
		result[index].EvidenceRefs = append([]EvidenceRef(nil), derivation.Prerequisite.EvidenceRefs...)
	}
	return result
}
