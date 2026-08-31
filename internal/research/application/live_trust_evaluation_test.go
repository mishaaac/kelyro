package application_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	trustpolicy "github.com/mishaaac/kelyro/internal/research/trust"
)

func TestLiveTrustEvaluationAppliesExistingPolicyWithoutProviderRankAuthority(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	topic, _ := research.NewResearchTopic("Go module", "software", "Go")
	official := testSource(t, "trust-official")
	official.Metadata.Publisher = "Example Foundation"
	updated := testTimestamp(t, 9)
	official.Metadata.UpdatedAt = &updated
	unclassified := testSource(t, "trust-other")
	unclassified.Kind = research.SourceOther
	unclassified.Metadata.Publisher = "Unknown Publisher"
	unclassified.Metadata.UpdatedAt = &updated
	for _, source := range []research.Source{official, unclassified} {
		if err := repositories.Sources.Create(ctx, source); err != nil {
			t.Fatal(err)
		}
	}
	claims := []research.Claim{
		trustClaimFixture(t, "trust-official", topic, official.ID),
		trustClaimFixture(t, "trust-other", topic, unclassified.ID),
	}
	service, err := application.NewLiveTrustEvaluationService(
		repositories.TrustRegistry, application.NewSourceRegistryService(repositories.SourceRegistry), fixedClock{now: testTimestamp(t, 12)},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := application.LiveTrustEvaluationRequest{Topic: topic, Purpose: research.PurposeCurrentUsage, Sources: []research.Source{official, unclassified}, Claims: claims}
	first, err := service.EvaluateTrust(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Decisions) != 2 {
		t.Fatalf("decisions = %+v", first.Decisions)
	}
	bySource := make(map[research.SourceID]research.TrustDecision)
	for _, decision := range first.Decisions {
		bySource[decision.SourceID] = decision
	}
	if got := bySource[official.ID]; got.Tier != research.AuthorityTierB || got.State != research.TrustRequiresVerification || got.Policy != trustpolicy.PolicyVersionV1 {
		t.Fatalf("official decision = %+v", got)
	}
	if got := bySource[unclassified.ID]; got.Tier != research.AuthorityTierE || got.State != research.TrustRejected {
		t.Fatalf("unclassified decision = %+v", got)
	}

	input := application.LiveResearchStageInput{
		Request: research.ResearchRequest{ID: testID(t, "request.trust-rank"), Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: testTimestamp(t, 8)},
		Artifacts: application.LiveResearchArtifacts{Sources: []research.Source{official, unclassified}, Claims: claims,
			SearchResults: []application.SearchResult{{Title: "ranked first", Locator: official.Locator, Provider: "fixture", Rank: 1}}},
	}
	withRank, err := service.Execute(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input.Artifacts.SearchResults[0].Rank = 99
	withoutRankAuthority, err := service.Execute(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(withRank.TrustDecisions, withoutRankAuthority.TrustDecisions) {
		t.Fatalf("provider rank changed trust: first=%+v second=%+v", withRank.TrustDecisions, withoutRankAuthority.TrustDecisions)
	}
}

func TestLiveTrustEvaluationUsesApplicableBlockedRegistryEntry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	topic, _ := research.NewResearchTopic("Go module", "software", "Go")
	source := testSource(t, "trust-blocked")
	source.Metadata.Publisher = "Blocked Publisher"
	updated := testTimestamp(t, 9)
	source.Metadata.UpdatedAt = &updated
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	entry := testRegistryEntry(t, "registry.trust-blocked", "example.test", research.RegistryBlocked)
	if err := repositories.SourceRegistry.Save(ctx, entry); err != nil {
		t.Fatal(err)
	}
	service, err := application.NewLiveTrustEvaluationService(
		repositories.TrustRegistry, application.NewSourceRegistryService(repositories.SourceRegistry), fixedClock{now: testTimestamp(t, 12)},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.EvaluateTrust(ctx, application.LiveTrustEvaluationRequest{
		Topic: topic, Purpose: research.PurposeCurrentUsage, Sources: []research.Source{source}, Claims: []research.Claim{trustClaimFixture(t, "blocked", topic, source.ID)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Decisions) != 1 || result.Decisions[0].State != research.TrustRejected || !hasTrustReasonCode(result.Decisions[0], "decision.rejected_registry_blocked") {
		t.Fatalf("blocked registry decision = %+v", result.Decisions)
	}
	latest, err := repositories.TrustRegistry.LatestDecision(ctx, source.ID)
	if err != nil || latest.State != research.TrustRejected {
		t.Fatalf("persisted decision = (%+v, %v)", latest, err)
	}
}

func TestLiveTrustEvaluationDoesNotInventIndependentCorroboration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	topic, _ := research.NewResearchTopic("Go module", "software", "Go")
	left := testSource(t, "trust-left")
	right := testSource(t, "trust-right")
	updated := testTimestamp(t, 9)
	for _, source := range []*research.Source{&left, &right} {
		source.Metadata.Publisher = "Example Foundation"
		source.Metadata.UpdatedAt = &updated
		if err := repositories.Sources.Create(ctx, *source); err != nil {
			t.Fatal(err)
		}
	}
	claim := trustClaimFixture(t, "multi", topic, left.ID)
	claim.SourceIDs = []research.SourceID{left.ID, right.ID}
	claim.EvidenceIDs = []research.ID{testID(t, "evidence.trust-left"), testID(t, "evidence.trust-right")}
	service, err := application.NewLiveTrustEvaluationService(
		repositories.TrustRegistry, application.NewSourceRegistryService(repositories.SourceRegistry), fixedClock{now: testTimestamp(t, 12)},
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.EvaluateTrust(ctx, application.LiveTrustEvaluationRequest{
		Topic: topic, Purpose: research.PurposeCurrentUsage, Sources: []research.Source{left, right}, Claims: []research.Claim{claim},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, decision := range result.Decisions {
		if !hasTrustReasonCode(decision, "corroboration.unknown") || decision.State != research.TrustRequiresVerification {
			t.Fatalf("multi-source decision invented independence: %+v", decision)
		}
	}
}

func trustClaimFixture(t *testing.T, suffix string, topic research.ResearchTopic, sourceID research.SourceID) research.Claim {
	t.Helper()
	confidence, _ := research.NewClaimConfidence(.9)
	return research.Claim{
		ID: testClaimID(t, suffix), Topic: topic, Statement: "A Go module must declare its path.", Type: research.ClaimRequirement,
		Scope: topic.Subject, StatusScope: research.ClaimStatusStable, Confidence: confidence,
		SourceIDs: []research.SourceID{sourceID}, EvidenceIDs: []research.ID{testID(t, "evidence."+suffix)}, CreatedAt: testTimestamp(t, 11),
	}
}

func hasTrustReasonCode(decision research.TrustDecision, code string) bool {
	for _, reason := range decision.Reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}
