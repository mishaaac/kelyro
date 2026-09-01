package application_test

import (
	"context"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	"github.com/mishaaac/kelyro/internal/research/citation"
	freshnesspolicy "github.com/mishaaac/kelyro/internal/research/freshness"
	temporalpolicy "github.com/mishaaac/kelyro/internal/research/temporal"
)

func TestLiveTemporalEvaluationCapturesNormalizedMetadataAndPersistsFreshness(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	topic, _ := research.NewResearchTopic("Go module", "software", "Go")
	target := testVersion(t, "1.0")
	fixture := liveTemporalFixture(t, "temporal-current", topic, research.ClaimRequirement, "A Go module must declare its path.", 12)
	fixture.source.Version = &target
	fixture.source.TemporalScope = research.SourceTemporalVersionBound
	fixture.normalized.VersionHints = []string{"1.0"}
	published := testTimestamp(t, 8)
	updated := testTimestamp(t, 11)
	fixture.normalized.PublishedAt = &published
	fixture.normalized.UpdatedAt = &updated
	fixture.citation = liveTemporalCitation(t, fixture.source, fixture.snapshot, fixture.evidence, 12)
	persistLiveTemporalFixture(t, ctx, repositories, fixture)

	service := liveTemporalService(t, repositories, 16)
	result, err := service.EvaluateTemporal(ctx, application.LiveTemporalEvaluationRequest{
		Topic: topic, Purpose: research.PurposeVersionBehavior, TargetVersion: &target,
		Sources: []research.Source{fixture.source}, NormalizedSources: []application.NormalizedSource{fixture.normalized},
		Claims: []research.Claim{fixture.claim}, Citations: []research.Citation{fixture.citation},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.TemporalObservations) != 1 || result.TemporalObservations[0].Temporal.Role != temporalpolicy.RoleVersionAuthority ||
		result.TemporalObservations[0].PublishedAt == nil || result.TemporalObservations[0].UpdatedAt == nil ||
		len(result.TemporalObservations[0].VersionHints) != 1 {
		t.Fatalf("temporal observation = %+v", result.TemporalObservations)
	}
	if len(result.FreshnessRecords) != 1 || result.FreshnessRecords[0].State != research.FreshnessFresh ||
		len(result.TrustDecisions) != 1 || result.TrustDecisions[0].State != research.TrustAccepted {
		t.Fatalf("freshness/trust = (%+v, %+v)", result.FreshnessRecords, result.TrustDecisions)
	}
	subjectID, _ := research.NewID(fixture.claim.ID.String())
	stored, err := repositories.Freshness.Get(ctx, subjectID)
	if err != nil || stored.State != research.FreshnessFresh || stored.AlgorithmVersion != research.FreshnessAlgorithmV1 {
		t.Fatalf("persisted freshness = (%+v, %v)", stored, err)
	}
}

func TestLiveTemporalEvaluationMarksSourceUpdateAndKnownCurrentReleaseStale(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	topic, _ := research.NewResearchTopic("Go module", "software", "Go")
	baseline := testVersion(t, "1.0")
	fixture := liveTemporalFixture(t, "temporal-stale", topic, research.ClaimVersionChange, "Go module version 1.0 was released as stable.", 12)
	fixture.claim.VersionScope = &baseline
	updated := testTimestamp(t, 13)
	fixture.normalized.UpdatedAt = &updated
	persistLiveTemporalFixture(t, ctx, repositories, fixture)
	current, _ := research.NewVersionIdentifier("2.0")
	release := research.ReleaseRecord{
		ID: testID(t, "release.temporal-current"), TechnologyID: testID(t, "technology.go"), Version: current,
		Channel: research.ReleaseStable, Status: research.ReleaseCurrent, SourceIDs: []research.SourceID{fixture.source.ID},
		VerifiedAt: testTimestamp(t, 14),
	}
	if err := repositories.Releases.Create(ctx, release); err != nil {
		t.Fatal(err)
	}

	service := liveTemporalService(t, repositories, 16)
	result, err := service.EvaluateTemporal(ctx, application.LiveTemporalEvaluationRequest{
		Topic: topic, Purpose: research.PurposeReleaseStatus, TargetVersion: &baseline,
		Sources: []research.Source{fixture.source}, NormalizedSources: []application.NormalizedSource{fixture.normalized},
		Claims: []research.Claim{fixture.claim}, Citations: []research.Citation{fixture.citation},
	})
	if err != nil {
		t.Fatal(err)
	}
	assessment := result.FreshnessAssessments[0]
	if assessment.Assessment.State != research.FreshnessStale || !assessment.KnownNewRelease ||
		!hasFreshnessReason(assessment.Assessment, freshnesspolicy.ReasonKnownNewRelease) ||
		!hasFreshnessReason(assessment.Assessment, freshnesspolicy.ReasonSourceUpdated) {
		t.Fatalf("stale assessment = %+v", assessment)
	}
	if result.TrustDecisions[0].State != research.TrustRequiresVerification || !hasTrustReasonCode(result.TrustDecisions[0], "freshness.stale") {
		t.Fatalf("stale trust decision = %+v", result.TrustDecisions[0])
	}
}

func TestLiveTemporalEvaluationReusesExactDeprecationVerification(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	topic, _ := research.NewResearchTopic("Go module", "software", "Go")
	fixture := liveTemporalFixture(t, "temporal-deprecation", topic, research.ClaimDeprecation, "Go module legacy mode is deprecated.", 12)
	fixture.claim.StatusScope = research.ClaimStatusLegacy
	persistLiveTemporalFixture(t, ctx, repositories, fixture)
	record := research.DeprecationRecord{
		ID: testID(t, "deprecation.temporal"), Subject: topic.Subject, Status: research.DeprecationDeprecated,
		Determination: research.DeprecationExplicitEvidence, SourceIDs: []research.SourceID{fixture.source.ID},
		EvidenceIDs: []research.ID{fixture.evidence.ID}, VerifiedAt: testTimestamp(t, 14),
		AlgorithmVersion: research.DeprecationIntelligenceAlgorithmV1,
	}
	if err := repositories.Deprecations.Append(ctx, record); err != nil {
		t.Fatal(err)
	}
	service := liveTemporalService(t, repositories, 16)
	result, err := service.EvaluateTemporal(ctx, application.LiveTemporalEvaluationRequest{
		Topic: topic, Purpose: research.PurposeDeprecationCheck, Sources: []research.Source{fixture.source},
		NormalizedSources: []application.NormalizedSource{fixture.normalized}, Claims: []research.Claim{fixture.claim},
		Citations: []research.Citation{fixture.citation},
	})
	if err != nil {
		t.Fatal(err)
	}
	assessment := result.FreshnessAssessments[0]
	if !assessment.DeprecationVerified || assessment.Assessment.LastVerifiedAt == nil ||
		!assessment.Assessment.LastVerifiedAt.Time().Equal(testTimestamp(t, 14).Time()) {
		t.Fatalf("deprecation freshness = %+v", assessment)
	}
}

type liveTemporalTestFixture struct {
	source     research.Source
	snapshot   research.SourceSnapshot
	evidence   research.Evidence
	claim      research.Claim
	citation   research.Citation
	normalized application.NormalizedSource
}

func liveTemporalFixture(t *testing.T, suffix string, topic research.ResearchTopic, claimType research.ClaimType, statement string, verifiedHour int) liveTemporalTestFixture {
	t.Helper()
	source := testSource(t, suffix)
	source.Metadata.Publisher = "Example Foundation"
	metadataUpdated := testTimestamp(t, 11)
	source.Metadata.UpdatedAt = &metadataUpdated
	snapshot := testSnapshot(t, source, suffix, 10)
	evidence := claimEvidenceFixtureForChain(t, suffix, statement, source, snapshot, 11)
	confidence, _ := research.NewClaimConfidence(.9)
	claim := research.Claim{
		ID: testClaimID(t, suffix), Topic: topic, Statement: statement, Type: claimType, Scope: topic.Subject,
		StatusScope: research.ClaimStatusStable, Confidence: confidence, SourceIDs: []research.SourceID{source.ID},
		EvidenceIDs: []research.ID{evidence.ID}, CreatedAt: testTimestamp(t, verifiedHour),
	}
	item := liveTemporalCitation(t, source, snapshot, evidence, verifiedHour)
	return liveTemporalTestFixture{
		source: source, snapshot: snapshot, evidence: evidence, claim: claim, citation: item,
		normalized: application.NormalizedSource{
			SourceID: source.ID, Locator: source.Locator, ContentType: "text/plain", TextSegments: []string{statement},
			NormalizationVersion: "source-normalization-v1",
		},
	}
}

func liveTemporalCitation(t *testing.T, source research.Source, snapshot research.SourceSnapshot, evidence research.Evidence, verifiedHour int) research.Citation {
	t.Helper()
	item, err := citation.GenerateV1(citation.Request{
		ID: testID(t, "citation."+evidence.ID.String()), Source: source, Snapshot: snapshot, Evidence: evidence,
		LastVerified: testTimestamp(t, verifiedHour), Target: citation.Target{Section: evidence.Location},
	})
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func persistLiveTemporalFixture(t *testing.T, ctx context.Context, repositories application.Repositories, fixture liveTemporalTestFixture) {
	t.Helper()
	if err := repositories.Sources.Create(ctx, fixture.source); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Snapshots.Append(ctx, fixture.snapshot); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Evidence.Append(ctx, fixture.evidence); err != nil {
		t.Fatal(err)
	}
}

func liveTemporalService(t *testing.T, repositories application.Repositories, nowHour int) application.LiveTemporalEvaluationService {
	t.Helper()
	clock := fixedClock{now: testTimestamp(t, nowHour)}
	trust, err := application.NewLiveTrustEvaluationService(
		repositories.TrustRegistry, application.NewSourceRegistryService(repositories.SourceRegistry), clock,
	)
	if err != nil {
		t.Fatal(err)
	}
	service, err := application.NewLiveTemporalEvaluationService(
		application.NewFreshnessService(repositories.Freshness), trust, repositories.TrustRegistry,
		repositories.Releases, repositories.Deprecations, clock,
	)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func hasFreshnessReason(assessment freshnesspolicy.Assessment, code freshnesspolicy.ReasonCode) bool {
	for _, reason := range assessment.Reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}
