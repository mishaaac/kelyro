package conflict_test

import (
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	conflictpolicy "github.com/mishaaac/kelyro/internal/research/conflict"
)

func TestResolverV1FavorsNormativeSpecificationOverCommunityClaim(t *testing.T) {
	left := observation(t, "spec", research.SourceSpecification, research.SourceTemporalCurrent, research.AuthorityTierA)
	right := observation(t, "blog", research.SourceCommunityArticle, research.SourceTemporalCurrent, research.AuthorityTierC)
	left.Claim.Type, right.Claim.Type = research.ClaimRequirement, research.ClaimRequirement
	result := resolve(t, conflictpolicy.RelationContradiction, left, right)
	if result.Type != research.ConflictAuthorityMismatch || result.Unresolved ||
		result.WinningSourceID == nil || *result.WinningSourceID != left.Source.ID ||
		result.Confidence.Value() != 0.95 || result.Reason == "" {
		t.Fatalf("normative result = %+v", result)
	}
}

func TestResolverV1DoesNotPreferNormativeLabelOverStrongerReviewedAuthority(t *testing.T) {
	spec := observation(t, "weak-spec", research.SourceSpecification, research.SourceTemporalCurrent, research.AuthorityTierD)
	community := observation(t, "reviewed-community", research.SourceCommunityArticle, research.SourceTemporalCurrent, research.AuthorityTierA)
	spec.Claim.Type, community.Claim.Type = research.ClaimRequirement, research.ClaimRequirement
	result := resolve(t, conflictpolicy.RelationContradiction, spec, community)
	if result.WinningSourceID == nil || *result.WinningSourceID != community.Source.ID ||
		result.Confidence.Value() != 0.85 {
		t.Fatalf("reviewed authority result = %+v", result)
	}
}

func TestResolverV1ClassifiesCurrentVersusHistoricalAsTemporal(t *testing.T) {
	current := observation(t, "current", research.SourceOfficialDocumentation, research.SourceTemporalCurrent, research.AuthorityTierA)
	historical := observation(t, "old-tutorial", research.SourceOfficialTutorial, research.SourceTemporalHistorical, research.AuthorityTierB)
	result := resolve(t, conflictpolicy.RelationContradiction, historical, current)
	if result.Type != research.ConflictTemporalMismatch || result.Unresolved ||
		result.WinningSourceID == nil || *result.WinningSourceID != current.Source.ID ||
		result.WinningScope != string(research.SourceTemporalCurrent) {
		t.Fatalf("temporal result = %+v", result)
	}
}

func TestResolverV1SeparatesVersionBoundClaimsWithoutInventingWinner(t *testing.T) {
	left := observation(t, "v1", research.SourceOfficialDocumentation, research.SourceTemporalCurrent, research.AuthorityTierA)
	right := observation(t, "v2", research.SourceOfficialDocumentation, research.SourceTemporalCurrent, research.AuthorityTierA)
	v1, _ := research.NewSourceVersion("1.0")
	v2, _ := research.NewSourceVersion("2.0")
	left.Claim.VersionScope, right.Claim.VersionScope = &v1, &v2
	result := resolve(t, conflictpolicy.RelationContradiction, left, right)
	if result.Type != research.ConflictVersionMismatch || result.Unresolved ||
		result.WinningClaimID != nil || result.Resolution == "" || result.Confidence.Value() != 0.95 {
		t.Fatalf("version result = %+v", result)
	}
}

func TestResolverV1EscalatesSameVersionOfficialContradiction(t *testing.T) {
	left := observation(t, "official-one", research.SourceOfficialDocumentation, research.SourceTemporalCurrent, research.AuthorityTierA)
	right := observation(t, "official-two", research.SourceOfficialDocumentation, research.SourceTemporalCurrent, research.AuthorityTierA)
	version, _ := research.NewSourceVersion("3.0")
	left.Claim.VersionScope, right.Claim.VersionScope = &version, &version
	result := resolve(t, conflictpolicy.RelationContradiction, left, right)
	if result.Type != research.ConflictDirectContradiction || !result.Unresolved ||
		result.WinningSourceID != nil || result.Resolution != "" || result.Reason == "" ||
		result.AlgorithmVersion != research.ConflictResolverAlgorithmV1 {
		t.Fatalf("official conflict result = %+v", result)
	}
}

func resolve(t *testing.T, relation conflictpolicy.Relation, left, right conflictpolicy.Observation) research.Conflict {
	t.Helper()
	id, _ := research.NewID("conflict.fixture")
	at, _ := research.NewTimestamp(time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC))
	result, err := conflictpolicy.Resolve(conflictpolicy.Input{
		ID: id, Relation: relation, Observations: []conflictpolicy.Observation{left, right}, DetectedAt: at,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func observation(
	t *testing.T,
	suffix string,
	kind research.SourceKind,
	temporalScope research.SourceTemporalScope,
	tier research.AuthorityTier,
) conflictpolicy.Observation {
	t.Helper()
	topic, _ := research.NewResearchTopic("conflict fixture", "software", "fixture")
	sourceID, _ := research.NewSourceID("source." + suffix)
	locator, _ := research.NewSourceLocator("https://example.test/" + suffix)
	createdAt, _ := research.NewTimestamp(time.Date(2026, time.August, 26, 10, 0, 0, 0, time.UTC))
	claimID, _ := research.NewClaimID("claim." + suffix)
	evidenceID, _ := research.NewID("evidence." + suffix)
	confidence, _ := research.NewClaimConfidence(0.9)
	return conflictpolicy.Observation{
		Source: research.Source{
			ID: sourceID, Kind: kind, Locator: locator, TemporalScope: temporalScope,
			Metadata: research.SourceMetadata{Title: "Source " + suffix}, CreatedAt: createdAt,
		},
		Claim: research.Claim{
			ID: claimID, Topic: topic, Statement: "Statement " + suffix,
			Type: research.ClaimBehavior, Scope: "general", StatusScope: research.ClaimStatusStable,
			Confidence: confidence, SourceIDs: []research.SourceID{sourceID},
			EvidenceIDs: []research.ID{evidenceID}, CreatedAt: createdAt,
		},
		AuthorityTier: tier,
	}
}
