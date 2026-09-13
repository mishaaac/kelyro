package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type GuidanceClassifierV1 struct{}

func NewGuidanceClassifierV1() GuidanceClassifierV1 {
	return GuidanceClassifierV1{}
}

func (GuidanceClassifierV1) Classify(ctx context.Context, request GuidanceClassificationRequest) (curriculum.GuidanceClassificationResult, error) {
	const operation = "classify curriculum guidance"
	if err := ctx.Err(); err != nil {
		return curriculum.GuidanceClassificationResult{}, ExternalError(operation, err)
	}
	if err := request.TemporalResult.Validate(); err != nil {
		return curriculum.GuidanceClassificationResult{}, Invalid(operation, err)
	}
	knownEvidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.GuidanceClassificationResult{}, Invalid(operation, err)
	}
	claims := make(map[evidenceKey]curriculum.CurriculumEvidenceClaim)
	for _, set := range request.EvidenceSets {
		for _, claim := range set.Claims {
			claims[evidenceKey{bundle: set.Bundle.ID.String(), claim: claim.ID.String()}] = claim
		}
	}
	result := curriculum.GuidanceClassificationResult{AlgorithmVersion: curriculum.GuidanceClassifierVersionV1}
	values := append([]curriculum.TemporalClassification(nil), request.TemporalResult.Classifications...)
	sort.Slice(values, func(i, j int) bool {
		if values[i].TargetKind != values[j].TargetKind {
			return values[i].TargetKind < values[j].TargetKind
		}
		return values[i].TargetID.String() < values[j].TargetID.String()
	})
	for _, temporal := range values {
		if err := ctx.Err(); err != nil {
			return curriculum.GuidanceClassificationResult{}, ExternalError(operation, err)
		}
		if err := requireKnownEvidence(temporal.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.GuidanceClassificationResult{}, Invalid(operation, fmt.Errorf("guidance target %q: %w", temporal.TargetID, err))
		}
		hasRecommendation := false
		for _, reference := range temporal.EvidenceRefs {
			claim, exists := claims[evidenceKey{bundle: reference.BundleID.String(), claim: reference.ClaimID.String()}]
			if !exists {
				return curriculum.GuidanceClassificationResult{}, Invalid(operation, fmt.Errorf("guidance claim unavailable for %s/%s", reference.BundleID, reference.ClaimID))
			}
			hasRecommendation = hasRecommendation || claim.Kind == curriculum.EvidenceClaimRecommendation
		}
		guidance, reason := guidanceForTemporalStatus(temporal.Status, hasRecommendation)
		classification := curriculum.GuidanceClassification{
			TargetKind: temporal.TargetKind, TargetID: temporal.TargetID, Status: temporal.Status,
			Guidance: guidance, EvidenceRefs: uniqueSortedEvidence(temporal.EvidenceRefs), Reason: reason,
		}
		result.Classifications = append(result.Classifications, classification)
		if temporal.Status == curriculum.ConceptDeprecated ||
			((temporal.Status == curriculum.ConceptLegacy || temporal.Status == curriculum.ConceptHistorical) && !temporal.ContextualUse) {
			result.CurrentGuidanceFindings = append(result.CurrentGuidanceFindings, curriculum.CurrentGuidanceFinding{
				TargetID:     temporal.TargetID,
				Reason:       "target has " + string(guidance) + " guidance without a declared current alternative",
				EvidenceRefs: uniqueSortedEvidence(temporal.EvidenceRefs),
			})
		}
	}
	sort.Slice(result.CurrentGuidanceFindings, func(i, j int) bool {
		return result.CurrentGuidanceFindings[i].TargetID.String() < result.CurrentGuidanceFindings[j].TargetID.String()
	})
	if err := result.Validate(); err != nil {
		return curriculum.GuidanceClassificationResult{}, Invalid(operation, err)
	}
	return result, nil
}

func guidanceForTemporalStatus(status curriculum.ConceptStatus, hasRecommendation bool) (curriculum.GuidanceType, string) {
	switch status {
	case curriculum.ConceptCurrent:
		if hasRecommendation {
			return curriculum.GuidanceRecommendedCurrent, "current guidance has an explicit recommendation Claim"
		}
		return curriculum.GuidanceAcceptableCurrent, "current evidence supports acceptable behavior without an explicit recommendation Claim"
	case curriculum.ConceptPreview, curriculum.ConceptExperimental:
		return curriculum.GuidanceExperimental, "preview or experimental guidance must remain separated from current guidance"
	case curriculum.ConceptLegacy:
		return curriculum.GuidanceLegacyMaintenance, "legacy guidance is limited to maintenance context"
	case curriculum.ConceptHistorical:
		return curriculum.GuidanceHistoricalContext, "historical guidance is limited to historical context"
	case curriculum.ConceptDeprecated:
		return curriculum.GuidanceAvoid, "deprecated guidance must not be recommended"
	default:
		panic("validated temporal status is unsupported")
	}
}
