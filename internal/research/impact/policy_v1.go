package impact

import (
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
)

type Input struct {
	Drift      research.DriftReport
	References []research.ClaimImpactReference
	AssessedAt research.Timestamp
}

type Assessment struct {
	AffectedEvidenceIDs   []research.ID
	AffectedBundleIDs     []research.ID
	AffectedClaimIDs      []research.ClaimID
	FutureConceptRefs     []research.ID
	FutureLessonRefs      []research.ID
	TechnologyVersionRefs []research.TechnologyVersionReference
	Severity              research.Severity
	RecommendedAction     research.RecommendedAction
}

func AnalyzeV1(input Input) (Assessment, error) {
	if err := input.Drift.Validate(); err != nil {
		return Assessment{}, fmt.Errorf("impact drift: %w", err)
	}
	if input.Drift.AlgorithmVersion != research.DriftAlgorithmV1 {
		return Assessment{}, fmt.Errorf("impact analysis requires %s drift", research.DriftAlgorithmV1)
	}
	if err := input.AssessedAt.Validate(); err != nil {
		return Assessment{}, fmt.Errorf("impact assessed at: %w", err)
	}
	if input.AssessedAt.Before(input.Drift.DetectedAt) {
		return Assessment{}, fmt.Errorf("impact assessment precedes drift detection")
	}

	affectedClaims := make(map[research.ClaimID]struct{}, len(input.Drift.AffectedClaims))
	for _, claimID := range input.Drift.AffectedClaims {
		affectedClaims[claimID] = struct{}{}
	}
	seenReferences := make(map[research.ClaimID]struct{}, len(input.References))
	concepts := make(map[research.ID]struct{})
	lessons := make(map[research.ID]struct{})
	versions := make(map[string]research.TechnologyVersionReference)
	for _, reference := range input.References {
		if err := reference.Validate(); err != nil {
			return Assessment{}, err
		}
		if _, exists := affectedClaims[reference.ClaimID]; !exists {
			return Assessment{}, fmt.Errorf("impact reference claim %q is not affected by drift", reference.ClaimID)
		}
		if _, duplicate := seenReferences[reference.ClaimID]; duplicate {
			return Assessment{}, fmt.Errorf("duplicate impact reference for claim %q", reference.ClaimID)
		}
		seenReferences[reference.ClaimID] = struct{}{}
		for _, id := range reference.FutureConceptRefs {
			concepts[id] = struct{}{}
		}
		for _, id := range reference.FutureLessonRefs {
			lessons[id] = struct{}{}
		}
		for _, version := range reference.TechnologyVersionRefs {
			versions[technologyVersionKey(version)] = version
		}
	}

	bundles := []research.ID{input.Drift.OldBundleID}
	if input.Drift.NewBundleID != nil && *input.Drift.NewBundleID != input.Drift.OldBundleID {
		bundles = append(bundles, *input.Drift.NewBundleID)
	}
	evidence := uniqueSortedIDs(append(append([]research.ID(nil), input.Drift.OldEvidence...), input.Drift.NewEvidence...))
	claims := append([]research.ClaimID(nil), input.Drift.AffectedClaims...)
	sort.Slice(claims, func(i, j int) bool { return claims[i].String() < claims[j].String() })
	return Assessment{
		AffectedEvidenceIDs: evidence, AffectedBundleIDs: uniqueSortedIDs(bundles),
		AffectedClaimIDs: claims, FutureConceptRefs: sortedIDSet(concepts),
		FutureLessonRefs: sortedIDSet(lessons), TechnologyVersionRefs: sortedVersionSet(versions),
		Severity: input.Drift.Severity, RecommendedAction: recommendedAction(input.Drift),
	}, nil
}

func recommendedAction(drift research.DriftReport) research.RecommendedAction {
	if drift.Severity == research.SeverityInformational {
		return research.ActionNoAction
	}
	if drift.Severity == research.SeverityCritical {
		return research.ActionManualReview
	}
	switch drift.Type {
	case research.DriftSourceChanged:
		return research.ActionReverify
	case research.DriftVersionSuperseded:
		return research.ActionRecompileFuture
	case research.DriftClaimInvalidated, research.DriftRecommendationChanged,
		research.DriftDeprecationIntroduced, research.DriftScopeChanged:
		return research.ActionReviewCurriculum
	default:
		return research.ActionManualReview
	}
}

func uniqueSortedIDs(values []research.ID) []research.ID {
	set := make(map[research.ID]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return sortedIDSet(set)
}

func sortedIDSet(set map[research.ID]struct{}) []research.ID {
	if len(set) == 0 {
		return nil
	}
	values := make([]research.ID, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].String() < values[j].String() })
	return values
}

func sortedVersionSet(set map[string]research.TechnologyVersionReference) []research.TechnologyVersionReference {
	if len(set) == 0 {
		return nil
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]research.TechnologyVersionReference, 0, len(keys))
	for _, key := range keys {
		values = append(values, set[key])
	}
	return values
}

func technologyVersionKey(reference research.TechnologyVersionReference) string {
	return reference.TechnologyID.String() + "\x00" + reference.Version.String()
}
