package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type TemporalClassificationV1 struct{}

func NewTemporalClassificationV1() TemporalClassificationV1 {
	return TemporalClassificationV1{}
}

type temporalEvidenceSignal struct {
	status curriculum.ConceptStatus
}

func (TemporalClassificationV1) Classify(ctx context.Context, request TemporalClassificationRequest) (curriculum.TemporalClassificationResult, error) {
	const operation = "classify curriculum temporal status"
	if err := ctx.Err(); err != nil {
		return curriculum.TemporalClassificationResult{}, ExternalError(operation, err)
	}
	knownEvidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.TemporalClassificationResult{}, Invalid(operation, err)
	}
	signals := indexTemporalEvidenceSignals(request.EvidenceSets)
	contextualConcepts := make(map[curriculum.ConceptID]struct{}, len(request.ContextualConceptIDs))
	for _, conceptID := range request.ContextualConceptIDs {
		if err := conceptID.Validate(); err != nil {
			return curriculum.TemporalClassificationResult{}, Invalid(operation, err)
		}
		if _, exists := contextualConcepts[conceptID]; exists {
			return curriculum.TemporalClassificationResult{}, Invalid(operation, fmt.Errorf("duplicate contextual concept %q", conceptID))
		}
		contextualConcepts[conceptID] = struct{}{}
	}
	concepts := make(map[curriculum.ConceptID]curriculum.Concept, len(request.Concepts))
	conceptClassifications := make(map[curriculum.ConceptID]curriculum.TemporalClassification, len(request.Concepts))
	result := curriculum.TemporalClassificationResult{AlgorithmVersion: curriculum.TemporalClassificationVersionV1}
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return curriculum.TemporalClassificationResult{}, Invalid(operation, err)
		}
		if _, exists := concepts[concept.ID]; exists {
			return curriculum.TemporalClassificationResult{}, Invalid(operation, fmt.Errorf("duplicate temporal concept %q", concept.ID))
		}
		if len(concept.EvidenceRefs) == 0 {
			return curriculum.TemporalClassificationResult{}, Invalid(operation, fmt.Errorf("temporal concept %q has no evidence", concept.ID))
		}
		if err := requireKnownEvidence(concept.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.TemporalClassificationResult{}, Invalid(operation, fmt.Errorf("temporal concept %q: %w", concept.ID, err))
		}
		status, mixed, err := classifyTemporalRefs(concept.EvidenceRefs, signals)
		if err != nil {
			return curriculum.TemporalClassificationResult{}, Invalid(operation, err)
		}
		targetID, _ := curriculum.NewID(concept.ID.String())
		_, contextual := contextualConcepts[concept.ID]
		classification := temporalClassification(targetID, curriculum.TemporalTargetConcept, concept.Status, status, contextual, concept.EvidenceRefs, mixed)
		concepts[concept.ID] = concept
		conceptClassifications[concept.ID] = classification
		result.Classifications = append(result.Classifications, classification)
	}
	for conceptID := range contextualConcepts {
		if _, exists := concepts[conceptID]; !exists {
			return curriculum.TemporalClassificationResult{}, Invalid(operation, fmt.Errorf("contextual concept %q is absent", conceptID))
		}
	}
	seenLessons := make(map[curriculum.ID]struct{}, len(request.Lessons))
	for _, lesson := range request.Lessons {
		if err := lesson.Validate(); err != nil {
			return curriculum.TemporalClassificationResult{}, Invalid(operation, err)
		}
		if _, exists := seenLessons[lesson.Lesson.ID]; exists {
			return curriculum.TemporalClassificationResult{}, Invalid(operation, fmt.Errorf("duplicate temporal lesson %q", lesson.Lesson.ID))
		}
		seenLessons[lesson.Lesson.ID] = struct{}{}
		refs := append([]curriculum.EvidenceRef(nil), lesson.EvidenceRefs...)
		statuses := make([]curriculum.ConceptStatus, 0, len(lesson.ConceptIDs)+len(lesson.EvidenceRefs))
		for _, conceptID := range lesson.ConceptIDs {
			classification, exists := conceptClassifications[conceptID]
			if !exists {
				return curriculum.TemporalClassificationResult{}, Invalid(operation, fmt.Errorf("lesson %q references missing temporal concept %q", lesson.Lesson.ID, conceptID))
			}
			statuses = append(statuses, classification.Status)
			refs = append(refs, classification.EvidenceRefs...)
		}
		if err := requireKnownEvidence(lesson.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.TemporalClassificationResult{}, Invalid(operation, fmt.Errorf("temporal lesson %q: %w", lesson.Lesson.ID, err))
		}
		for _, reference := range lesson.EvidenceRefs {
			signal, exists := signals[evidenceKey{bundle: reference.BundleID.String(), claim: reference.ClaimID.String()}]
			if !exists {
				return curriculum.TemporalClassificationResult{}, Invalid(operation, fmt.Errorf("temporal evidence signal unavailable"))
			}
			statuses = append(statuses, signal.status)
		}
		refs = uniqueSortedEvidence(refs)
		status, mixed := dominantTemporalStatus(statuses)
		result.Classifications = append(result.Classifications, temporalClassification(
			lesson.Lesson.ID, curriculum.TemporalTargetLesson, lesson.DeclaredStatus,
			status, lesson.ContextualUse, refs, mixed,
		))
	}
	sort.Slice(result.Classifications, func(i, j int) bool {
		left, right := result.Classifications[i], result.Classifications[j]
		if left.TargetKind != right.TargetKind {
			return left.TargetKind < right.TargetKind
		}
		return left.TargetID.String() < right.TargetID.String()
	})
	if err := result.Validate(); err != nil {
		return curriculum.TemporalClassificationResult{}, Invalid(operation, err)
	}
	return result, nil
}

func indexTemporalEvidenceSignals(sets []curriculum.CurriculumEvidenceSet) map[evidenceKey]temporalEvidenceSignal {
	result := make(map[evidenceKey]temporalEvidenceSignal)
	for _, set := range sets {
		authority := make(map[curriculum.ID]curriculum.EvidenceSourceAuthority, len(set.SourceAuthority))
		for _, source := range set.SourceAuthority {
			authority[source.SourceID] = source
		}
		for _, claim := range set.Claims {
			status := claim.Status
			if claim.Kind == curriculum.EvidenceClaimDeprecation {
				status = curriculum.ConceptDeprecated
			} else if claim.Kind == curriculum.EvidenceClaimHistorical || allClaimSourcesHistorical(claim.SourceIDs, authority) {
				status = curriculum.ConceptHistorical
			}
			result[evidenceKey{bundle: set.Bundle.ID.String(), claim: claim.ID.String()}] = temporalEvidenceSignal{status: status}
		}
	}
	return result
}

func allClaimSourcesHistorical(sourceIDs []curriculum.ID, authority map[curriculum.ID]curriculum.EvidenceSourceAuthority) bool {
	if len(sourceIDs) == 0 {
		return false
	}
	for _, sourceID := range sourceIDs {
		source := authority[sourceID]
		if source.TemporalScope != "historical" && source.TemporalScope != "archived" {
			return false
		}
	}
	return true
}

func classifyTemporalRefs(references []curriculum.EvidenceRef, signals map[evidenceKey]temporalEvidenceSignal) (curriculum.ConceptStatus, bool, error) {
	statuses := make([]curriculum.ConceptStatus, 0, len(references))
	for _, reference := range references {
		signal, exists := signals[evidenceKey{bundle: reference.BundleID.String(), claim: reference.ClaimID.String()}]
		if !exists {
			return "", false, fmt.Errorf("temporal evidence signal unavailable for %s/%s", reference.BundleID, reference.ClaimID)
		}
		statuses = append(statuses, signal.status)
	}
	status, mixed := dominantTemporalStatus(statuses)
	return status, mixed, nil
}

func dominantTemporalStatus(statuses []curriculum.ConceptStatus) (curriculum.ConceptStatus, bool) {
	dominant := curriculum.ConceptCurrent
	seen := make(map[curriculum.ConceptStatus]struct{}, len(statuses))
	for _, status := range statuses {
		seen[status] = struct{}{}
		if temporalStatusRank(status) > temporalStatusRank(dominant) {
			dominant = status
		}
	}
	return dominant, len(seen) > 1
}

func temporalStatusRank(status curriculum.ConceptStatus) int {
	switch status {
	case curriculum.ConceptCurrent:
		return 0
	case curriculum.ConceptPreview:
		return 1
	case curriculum.ConceptExperimental:
		return 2
	case curriculum.ConceptLegacy:
		return 3
	case curriculum.ConceptHistorical:
		return 4
	case curriculum.ConceptDeprecated:
		return 5
	default:
		return -1
	}
}

func temporalClassification(targetID curriculum.ID, targetKind curriculum.TemporalTargetKind, declared, status curriculum.ConceptStatus, contextual bool, refs []curriculum.EvidenceRef, mixed bool) curriculum.TemporalClassification {
	classification := curriculum.TemporalClassification{
		TargetKind: targetKind, TargetID: targetID, DeclaredStatus: declared, Status: status,
		ContextualUse: contextual, EvidenceRefs: uniqueSortedEvidence(refs),
	}
	switch status {
	case curriculum.ConceptCurrent:
		classification.PrimaryRecommendation = true
	case curriculum.ConceptPreview, curriculum.ConceptExperimental:
		classification.Separated = true
	case curriculum.ConceptLegacy, curriculum.ConceptHistorical:
		classification.ContextOnly = true
	}
	classification.Reasons = []string{"status_from_verified_claims:" + string(status)}
	if declared != status {
		classification.Reasons = append(classification.Reasons, "declared_status_overridden:"+string(declared)+"->"+string(status))
	}
	if mixed {
		classification.Reasons = append(classification.Reasons, "mixed_temporal_evidence:safety_precedence_applied")
	}
	if classification.ContextOnly && !contextual {
		classification.Reasons = append(classification.Reasons, "context_only_without_contextual_use_declaration")
	}
	sort.Strings(classification.Reasons)
	return classification
}

func uniqueSortedEvidence(values []curriculum.EvidenceRef) []curriculum.EvidenceRef {
	seen := make(map[curriculum.EvidenceRef]struct{}, len(values))
	result := make([]curriculum.EvidenceRef, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sortEvidenceRefs(result)
	return result
}
