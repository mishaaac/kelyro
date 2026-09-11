package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type ConceptAtomizerV1 struct {
	policy AtomicityPolicy
}

func NewConceptAtomizerV1(policy AtomicityPolicy) *ConceptAtomizerV1 {
	return &ConceptAtomizerV1{policy: policy}
}

func (service *ConceptAtomizerV1) Atomize(ctx context.Context, request ConceptAtomizationRequest) (curriculum.AtomicConceptSet, error) {
	const operation = "atomize concept candidate"
	if err := ctx.Err(); err != nil {
		return curriculum.AtomicConceptSet{}, ExternalError(operation, err)
	}
	if service == nil || service.policy == nil {
		return curriculum.AtomicConceptSet{}, RequireDependency(operation, "atomicity policy", nil)
	}
	if service.policy.Version() != curriculum.AtomicConceptCriteriaVersionV1 {
		return curriculum.AtomicConceptSet{}, Invalid(operation, fmt.Errorf("unsupported atomicity policy %q", service.policy.Version()))
	}
	if err := request.Candidate.Validate(); err != nil {
		return curriculum.AtomicConceptSet{}, Invalid(operation, err)
	}
	known, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.AtomicConceptSet{}, Invalid(operation, err)
	}
	if err := requireKnownEvidence(request.Candidate.ClaimRefs, known); err != nil {
		return curriculum.AtomicConceptSet{}, Invalid(operation, fmt.Errorf("candidate %q: %w", request.Candidate.ID, err))
	}
	assessment, err := service.policy.Assess(request.CandidateCriteria)
	if err != nil {
		return curriculum.AtomicConceptSet{}, Invalid(operation, err)
	}
	if assessment.Result == curriculum.AtomicityUnknown || assessment.Result == curriculum.AtomicityTooFragmented {
		return curriculum.AtomicConceptSet{}, Invalid(operation, fmt.Errorf("candidate %q cannot be atomized: %s", request.Candidate.ID, assessment.Result))
	}
	if len(request.DomainHints) == 0 {
		return curriculum.AtomicConceptSet{}, Invalid(operation, fmt.Errorf("candidate %q has no evidence-backed atomization hints", request.Candidate.ID))
	}
	if assessment.Result == curriculum.AtomicityAtomic && len(request.DomainHints) != 1 {
		return curriculum.AtomicConceptSet{}, Invalid(operation, fmt.Errorf("atomic candidate %q requires exactly one concept hint", request.Candidate.ID))
	}
	if assessment.Result == curriculum.AtomicityNeedsSplit && len(request.DomainHints) < 2 {
		return curriculum.AtomicConceptSet{}, Invalid(operation, fmt.Errorf("candidate %q needs a split but has insufficient evidence-backed hints", request.Candidate.ID))
	}

	hints := cloneAndSortAtomizationHints(request.DomainHints)
	candidateRefs := make(map[curriculum.EvidenceRef]struct{}, len(request.Candidate.ClaimRefs))
	for _, reference := range request.Candidate.ClaimRefs {
		candidateRefs[reference] = struct{}{}
	}
	coveredRefs := make(map[curriculum.EvidenceRef]struct{}, len(candidateRefs))
	seenConcepts := make(map[curriculum.ConceptID]struct{}, len(hints))
	result := curriculum.AtomicConceptSet{
		CandidateID: request.Candidate.ID, PolicyVersion: service.policy.Version(), AlgorithmVersion: curriculum.AtomizerVersionV1,
	}
	if assessment.Result == curriculum.AtomicityNeedsSplit {
		result.SplitReasons = append([]string(nil), assessment.Reasons...)
	}
	claimConcepts := make(map[curriculum.EvidenceRef][]curriculum.ConceptID, len(candidateRefs))
	for _, hint := range hints {
		if err := ctx.Err(); err != nil {
			return curriculum.AtomicConceptSet{}, ExternalError(operation, err)
		}
		if err := hint.Validate(); err != nil {
			return curriculum.AtomicConceptSet{}, Invalid(operation, err)
		}
		if hint.CandidateID != request.Candidate.ID {
			return curriculum.AtomicConceptSet{}, Invalid(operation, fmt.Errorf("atomization hint %q belongs to another candidate", hint.ConceptID))
		}
		if _, exists := seenConcepts[hint.ConceptID]; exists {
			return curriculum.AtomicConceptSet{}, Invalid(operation, fmt.Errorf("duplicate atomization concept %q", hint.ConceptID))
		}
		seenConcepts[hint.ConceptID] = struct{}{}
		if request.Candidate.VersionScope != "" && hint.Version != request.Candidate.VersionScope {
			return curriculum.AtomicConceptSet{}, Invalid(operation, fmt.Errorf("atomization hint %q version does not match candidate", hint.ConceptID))
		}
		childAssessment, err := service.policy.Assess(hint.Criteria)
		if err != nil {
			return curriculum.AtomicConceptSet{}, Invalid(operation, fmt.Errorf("atomization hint %q: %w", hint.ConceptID, err))
		}
		if childAssessment.Result != curriculum.AtomicityAtomic {
			return curriculum.AtomicConceptSet{}, Invalid(operation, fmt.Errorf("atomization hint %q is %s, not atomic", hint.ConceptID, childAssessment.Result))
		}
		for _, reference := range hint.ClaimRefs {
			if _, exists := candidateRefs[reference]; !exists {
				return curriculum.AtomicConceptSet{}, Invalid(operation, fmt.Errorf("atomization hint %q references claim outside candidate", hint.ConceptID))
			}
			coveredRefs[reference] = struct{}{}
			claimConcepts[reference] = append(claimConcepts[reference], hint.ConceptID)
		}
		result.Concepts = append(result.Concepts, curriculum.Concept{
			ID: hint.ConceptID, Title: hint.Title, Definition: hint.Definition, Version: hint.Version,
			Atomicity: curriculum.AtomicityAtomic, Difficulty: hint.Difficulty, Status: request.Candidate.Status,
			Foundational: hint.Foundational, EvidenceRefs: append([]curriculum.EvidenceRef(nil), hint.ClaimRefs...),
		})
	}
	if len(coveredRefs) != len(candidateRefs) {
		return curriculum.AtomicConceptSet{}, Invalid(operation, fmt.Errorf("atomization hints do not map every candidate claim"))
	}

	refs := append([]curriculum.EvidenceRef(nil), request.Candidate.ClaimRefs...)
	sortEvidenceRefs(refs)
	for _, reference := range refs {
		ids := claimConcepts[reference]
		sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
		result.ClaimMapping = append(result.ClaimMapping, curriculum.ConceptClaimMapping{ClaimRef: reference, ConceptIDs: append([]curriculum.ConceptID(nil), ids...)})
	}
	if err := result.Validate(); err != nil {
		return curriculum.AtomicConceptSet{}, Invalid(operation, err)
	}
	return result, nil
}

func cloneAndSortAtomizationHints(values []curriculum.ConceptAtomizationHint) []curriculum.ConceptAtomizationHint {
	result := append([]curriculum.ConceptAtomizationHint(nil), values...)
	for index := range result {
		result[index].ClaimRefs = append([]curriculum.EvidenceRef(nil), result[index].ClaimRefs...)
		result[index].Criteria.IndependentlyAssessableParts = append([]string(nil), result[index].Criteria.IndependentlyAssessableParts...)
		sortEvidenceRefs(result[index].ClaimRefs)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ConceptID.String() < result[j].ConceptID.String() })
	return result
}

func sortEvidenceRefs(values []curriculum.EvidenceRef) {
	sort.Slice(values, func(i, j int) bool {
		left := values[i].BundleID.String() + "\x00" + values[i].ClaimID.String()
		right := values[j].BundleID.String() + "\x00" + values[j].ClaimID.String()
		return left < right
	})
}

var _ ConceptAtomizerService = (*ConceptAtomizerV1)(nil)
