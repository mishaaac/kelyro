package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type PrerequisiteExtractorV1 struct{}

func NewPrerequisiteExtractorV1() PrerequisiteExtractorV1 {
	return PrerequisiteExtractorV1{}
}

func (PrerequisiteExtractorV1) Extract(ctx context.Context, request PrerequisiteExtractionRequest) (curriculum.PrerequisiteExtraction, error) {
	const operation = "extract concept prerequisites"
	if err := ctx.Err(); err != nil {
		return curriculum.PrerequisiteExtraction{}, ExternalError(operation, err)
	}
	concepts := make(map[curriculum.ConceptID]struct{}, len(request.Concepts))
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return curriculum.PrerequisiteExtraction{}, Invalid(operation, err)
		}
		if concept.Atomicity != curriculum.AtomicityAtomic {
			return curriculum.PrerequisiteExtraction{}, Invalid(operation, fmt.Errorf("concept %q is not atomic", concept.ID))
		}
		if _, exists := concepts[concept.ID]; exists {
			return curriculum.PrerequisiteExtraction{}, Invalid(operation, fmt.Errorf("duplicate concept %q", concept.ID))
		}
		concepts[concept.ID] = struct{}{}
	}
	if len(concepts) == 0 {
		return curriculum.PrerequisiteExtraction{}, Invalid(operation, fmt.Errorf("prerequisite extraction has no concepts"))
	}
	knownEvidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.PrerequisiteExtraction{}, Invalid(operation, err)
	}

	semantics := cloneAndSortPrerequisiteSemantics(request.Semantics)
	result := curriculum.PrerequisiteExtraction{AlgorithmVersion: curriculum.PrerequisiteExtractorVersionV1}
	type edgeKey struct {
		concept  curriculum.ConceptID
		required curriculum.ConceptID
		kind     curriculum.PrerequisiteKind
	}
	seen := make(map[edgeKey]struct{}, len(semantics))
	for _, semantic := range semantics {
		if err := ctx.Err(); err != nil {
			return curriculum.PrerequisiteExtraction{}, ExternalError(operation, err)
		}
		if err := semantic.Validate(); err != nil {
			return curriculum.PrerequisiteExtraction{}, Invalid(operation, err)
		}
		if _, exists := concepts[semantic.ConceptID]; !exists {
			return curriculum.PrerequisiteExtraction{}, Invalid(operation, fmt.Errorf("prerequisite semantic references missing concept %q", semantic.ConceptID))
		}
		if _, exists := concepts[semantic.RequiredConceptID]; !exists {
			return curriculum.PrerequisiteExtraction{}, Invalid(operation, fmt.Errorf("prerequisite semantic references missing required concept %q", semantic.RequiredConceptID))
		}
		if err := requireKnownEvidence(semantic.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.PrerequisiteExtraction{}, Invalid(operation, err)
		}
		key := edgeKey{concept: semantic.ConceptID, required: semantic.RequiredConceptID, kind: semantic.Kind}
		if _, exists := seen[key]; exists {
			return curriculum.PrerequisiteExtraction{}, Invalid(operation, fmt.Errorf("duplicate prerequisite semantic from %q to %q", semantic.ConceptID, semantic.RequiredConceptID))
		}
		seen[key] = struct{}{}
		result.Derivations = append(result.Derivations, curriculum.PrerequisiteDerivation{
			Prerequisite: curriculum.Prerequisite{
				ConceptID: semantic.ConceptID, RequiredConceptID: semantic.RequiredConceptID,
				Kind: semantic.Kind, EvidenceRefs: append([]curriculum.EvidenceRef(nil), semantic.EvidenceRefs...),
			},
			Reason: semantic.Reason,
		})
	}
	if err := result.Validate(); err != nil {
		return curriculum.PrerequisiteExtraction{}, Invalid(operation, err)
	}
	return result, nil
}

func cloneAndSortPrerequisiteSemantics(values []curriculum.ConceptPrerequisiteSemantic) []curriculum.ConceptPrerequisiteSemantic {
	result := append([]curriculum.ConceptPrerequisiteSemantic(nil), values...)
	for index := range result {
		result[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), result[index].EvidenceRefs...)
		sortEvidenceRefs(result[index].EvidenceRefs)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i], result[j]
		if left.ConceptID != right.ConceptID {
			return left.ConceptID.String() < right.ConceptID.String()
		}
		if left.RequiredConceptID != right.RequiredConceptID {
			return left.RequiredConceptID.String() < right.RequiredConceptID.String()
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		return left.Reason < right.Reason
	})
	return result
}

var _ PrerequisiteExtractorService = PrerequisiteExtractorV1{}
