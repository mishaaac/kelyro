package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
	"github.com/mishaaac/kelyro/internal/research"
)

type SourceBundleIngester struct {
	provider ResearchBundleProvider
}

func NewSourceBundleIngester(provider ResearchBundleProvider) *SourceBundleIngester {
	return &SourceBundleIngester{provider: provider}
}

func (service *SourceBundleIngester) Ingest(ctx context.Context, request EvidenceIngestionRequest) (EvidenceIngestionResult, error) {
	const operation = "ingest source bundle"
	if err := ctx.Err(); err != nil {
		return EvidenceIngestionResult{}, ExternalError(operation, err)
	}
	if err := request.BundleID.Validate(); err != nil {
		return EvidenceIngestionResult{}, Invalid(operation, err)
	}
	if service == nil || service.provider == nil {
		return EvidenceIngestionResult{}, RequireDependency(operation, "research bundle provider", nil)
	}
	critical := make(map[research.ClaimID]struct{}, len(request.CriticalClaimIDs))
	for _, claimID := range request.CriticalClaimIDs {
		if err := claimID.Validate(); err != nil {
			return EvidenceIngestionResult{}, Invalid(operation, err)
		}
		if _, exists := critical[claimID]; exists {
			return EvidenceIngestionResult{}, Invalid(operation, fmt.Errorf("duplicate critical claim %q", claimID))
		}
		critical[claimID] = struct{}{}
	}
	bundle, err := service.provider.GetBundle(ctx, request.BundleID)
	if err != nil {
		return EvidenceIngestionResult{}, ExternalError(operation+": get bundle", err)
	}
	if err := bundle.Validate(); err != nil {
		return EvidenceIngestionResult{}, Invalid(operation, fmt.Errorf("invalid durable bundle: %w", err))
	}
	claimSet := make(map[research.ClaimID]struct{}, len(bundle.ClaimIDs))
	for _, claimID := range bundle.ClaimIDs {
		claimSet[claimID] = struct{}{}
	}
	for claimID := range critical {
		if _, exists := claimSet[claimID]; !exists {
			return EvidenceIngestionResult{}, Invalid(operation, fmt.Errorf("critical claim %q is outside bundle", claimID))
		}
	}

	evidence, err := newEvidenceSet(bundle)
	if err != nil {
		return EvidenceIngestionResult{}, Invalid(operation, err)
	}
	for _, claimID := range bundle.ClaimIDs {
		claim, err := service.provider.GetClaim(ctx, claimID)
		if err != nil {
			return EvidenceIngestionResult{}, ExternalError(operation+": get claim "+claimID.String(), err)
		}
		if err := claim.Validate(); err != nil {
			return EvidenceIngestionResult{}, Invalid(operation, fmt.Errorf("invalid durable claim %q: %w", claimID, err))
		}
		if claim.ID != claimID || claim.Topic != bundle.Topic {
			return EvidenceIngestionResult{}, Invalid(operation, fmt.Errorf("claim %q identity or topic does not match bundle", claimID))
		}
		converted, err := convertClaim(claim)
		if err != nil {
			return EvidenceIngestionResult{}, Invalid(operation, err)
		}
		evidence.Claims = append(evidence.Claims, converted)
		if converted.VersionScope != "" {
			evidence.VersionScopes = append(evidence.VersionScopes, converted.VersionScope)
		}
	}
	for _, conflictID := range bundle.ConflictIDs {
		conflict, err := service.provider.GetConflict(ctx, conflictID)
		if err != nil {
			return EvidenceIngestionResult{}, ExternalError(operation+": get conflict "+conflictID.String(), err)
		}
		if err := conflict.Validate(); err != nil {
			return EvidenceIngestionResult{}, Invalid(operation, fmt.Errorf("invalid durable conflict %q: %w", conflictID, err))
		}
		if conflict.ID != conflictID {
			return EvidenceIngestionResult{}, Invalid(operation, fmt.Errorf("conflict identity does not match bundle reference"))
		}
		converted, err := convertConflict(conflict, claimSet)
		if err != nil {
			return EvidenceIngestionResult{}, Invalid(operation, err)
		}
		evidence.Conflicts = append(evidence.Conflicts, converted)
	}
	evidence.VersionScopes = uniqueSorted(evidence.VersionScopes)
	reasons := eligibilityReasons(bundle, evidence.Conflicts, critical)
	accepted := evidence.Eligibility != curriculum.EvidenceNotReady && len(reasons) == 0
	if !accepted {
		evidence.Eligibility = curriculum.EvidenceNotReady
	}
	if err := evidence.Validate(); err != nil {
		return EvidenceIngestionResult{}, Invalid(operation, err)
	}
	return EvidenceIngestionResult{Evidence: evidence, Accepted: accepted, Reasons: reasons}, nil
}

func newEvidenceSet(bundle research.SourceBundle) (curriculum.CurriculumEvidenceSet, error) {
	id, err := curriculum.NewID(bundle.ID.String())
	if err != nil {
		return curriculum.CurriculumEvidenceSet{}, err
	}
	verifiedAt, err := curriculum.NewTimestamp(bundle.VerifiedAt.Time())
	if err != nil {
		return curriculum.CurriculumEvidenceSet{}, err
	}
	lastVerified := (*curriculum.Timestamp)(nil)
	if bundle.Freshness.LastVerifiedAt != nil {
		converted, err := curriculum.NewTimestamp(bundle.Freshness.LastVerifiedAt.Time())
		if err != nil {
			return curriculum.CurriculumEvidenceSet{}, err
		}
		lastVerified = &converted
	}
	eligibility := curriculum.EvidenceNotReady
	switch bundle.State {
	case research.BundleReady:
		eligibility = curriculum.EvidenceReadyForCompile
	case research.BundleReadyWithCaveats:
		eligibility = curriculum.EvidenceReadyWithCaveats
	}
	result := curriculum.CurriculumEvidenceSet{
		Bundle:           curriculum.SourceBundleRef{ID: id, ContentHash: bundle.ContentHash, AlgorithmVersion: bundle.AlgorithmVersion, VerifiedAt: verifiedAt},
		Eligibility:      eligibility,
		Freshness:        curriculum.EvidenceFreshness{State: string(bundle.Freshness.State), Score: bundle.Freshness.Score.Value(), LastVerifiedAt: lastVerified, Algorithm: bundle.Freshness.AlgorithmVersion},
		AlgorithmVersion: curriculum.EvidenceIngestionAlgorithmV1,
	}
	if bundle.TargetVersion != nil {
		result.VersionScopes = append(result.VersionScopes, bundle.TargetVersion.String())
	}
	for _, issue := range bundle.Issues {
		result.Caveats = append(result.Caveats, string(issue))
	}
	for _, source := range bundle.Sources {
		sourceID, err := curriculum.NewID(source.SourceID.String())
		if err != nil {
			return curriculum.CurriculumEvidenceSet{}, err
		}
		versionScope := ""
		if source.VersionScope != nil {
			versionScope = source.VersionScope.String()
			result.VersionScopes = append(result.VersionScopes, versionScope)
		}
		result.SourceAuthority = append(result.SourceAuthority, curriculum.EvidenceSourceAuthority{SourceID: sourceID, Role: string(source.Role), TemporalScope: string(source.TemporalScope), VersionScope: versionScope})
	}
	return result, nil
}

func convertClaim(claim research.Claim) (curriculum.CurriculumEvidenceClaim, error) {
	id, err := curriculum.NewID(claim.ID.String())
	if err != nil {
		return curriculum.CurriculumEvidenceClaim{}, err
	}
	sources := make([]curriculum.ID, 0, len(claim.SourceIDs))
	for _, raw := range claim.SourceIDs {
		sourceID, err := curriculum.NewID(raw.String())
		if err != nil {
			return curriculum.CurriculumEvidenceClaim{}, err
		}
		sources = append(sources, sourceID)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].String() < sources[j].String() })
	versionScope := ""
	if claim.VersionScope != nil {
		versionScope = claim.VersionScope.String()
	}
	status := curriculum.ConceptCurrent
	if claim.Type == research.ClaimHistorical {
		status = curriculum.ConceptHistorical
	}
	switch claim.StatusScope {
	case research.ClaimStatusPreview:
		status = curriculum.ConceptPreview
	case research.ClaimStatusExperimental:
		status = curriculum.ConceptExperimental
	case research.ClaimStatusLegacy:
		status = curriculum.ConceptLegacy
	}
	return curriculum.CurriculumEvidenceClaim{ID: id, Statement: claim.Statement, Scope: claim.Scope, VersionScope: versionScope, Status: status, Confidence: claim.Confidence.Value(), SourceIDs: sources}, nil
}

func convertConflict(conflict research.Conflict, bundleClaims map[research.ClaimID]struct{}) (curriculum.CurriculumEvidenceConflict, error) {
	id, err := curriculum.NewID(conflict.ID.String())
	if err != nil {
		return curriculum.CurriculumEvidenceConflict{}, err
	}
	claims := make([]curriculum.ID, 0, len(conflict.ClaimIDs))
	for _, raw := range conflict.ClaimIDs {
		if _, exists := bundleClaims[raw]; !exists {
			return curriculum.CurriculumEvidenceConflict{}, fmt.Errorf("conflict %q references claim outside bundle", conflict.ID)
		}
		claimID, err := curriculum.NewID(raw.String())
		if err != nil {
			return curriculum.CurriculumEvidenceConflict{}, err
		}
		claims = append(claims, claimID)
	}
	sort.Slice(claims, func(i, j int) bool { return claims[i].String() < claims[j].String() })
	return curriculum.CurriculumEvidenceConflict{ID: id, ClaimIDs: claims, Unresolved: conflict.Unresolved, Reason: conflict.Reason, AlgorithmVersion: conflict.AlgorithmVersion}, nil
}

func eligibilityReasons(bundle research.SourceBundle, conflicts []curriculum.CurriculumEvidenceConflict, critical map[research.ClaimID]struct{}) []string {
	reasons := make([]string, 0)
	if bundle.State == research.BundleIncomplete {
		reasons = append(reasons, "bundle_not_ready: incomplete")
	}
	if bundle.State == research.BundleConflicted {
		reasons = append(reasons, "bundle_not_ready: conflicted")
	}
	criticalFreshness := len(critical) > 0 && (bundle.Freshness.State == research.FreshnessStale || bundle.Freshness.State == research.FreshnessUnknown)
	if criticalFreshness {
		reasons = append(reasons, "critical_claim_freshness: "+string(bundle.Freshness.State))
	}
	for _, conflict := range conflicts {
		if !conflict.Unresolved {
			continue
		}
		for _, claimID := range conflict.ClaimIDs {
			researchID, _ := research.NewClaimID(claimID.String())
			if _, exists := critical[researchID]; exists {
				reasons = append(reasons, "critical_claim_conflict: "+claimID.String())
				break
			}
		}
	}
	sort.Strings(reasons)
	return uniqueSorted(reasons)
}

func uniqueSorted(values []string) []string {
	sort.Strings(values)
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

var _ SourceBundleIngestionService = (*SourceBundleIngester)(nil)
