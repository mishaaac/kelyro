package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
	"github.com/mishaaac/kelyro/internal/research"
)

func TestSourceBundleIngesterAcceptsReadyAndCaveatedEvidence(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name        string
		freshness   research.FreshnessState
		issues      []research.SourceBundleIssue
		eligibility curriculum.EvidenceEligibility
	}{
		{"ready", research.FreshnessFresh, nil, curriculum.EvidenceReadyForCompile},
		{"caveat", research.FreshnessAging, []research.SourceBundleIssue{research.BundleIssueAgingFreshness}, curriculum.EvidenceReadyWithCaveats},
		{"stale noncritical", research.FreshnessStale, []research.SourceBundleIssue{research.BundleIssueStaleFreshness}, curriculum.EvidenceReadyWithCaveats},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			provider, bundle, claimIDs := evidenceFixture(t, test.freshness, test.issues, false)
			result, err := NewSourceBundleIngester(provider).Ingest(context.Background(), EvidenceIngestionRequest{BundleID: bundle.ID})
			if err != nil || !result.Accepted || result.Evidence.Eligibility != test.eligibility || len(result.Evidence.Claims) != len(claimIDs) {
				t.Fatalf("Ingest() result=%+v error=%v", result, err)
			}
			if result.Evidence.Bundle.ContentHash != bundle.ContentHash || result.Evidence.AlgorithmVersion != curriculum.EvidenceIngestionAlgorithmV1 || len(result.Evidence.SourceAuthority) != 1 {
				t.Fatalf("evidence = %+v", result.Evidence)
			}
			repeated, err := NewSourceBundleIngester(provider).Ingest(context.Background(), EvidenceIngestionRequest{BundleID: bundle.ID})
			if err != nil || !reflect.DeepEqual(result, repeated) {
				t.Fatalf("repeated ingestion differs: first=%+v second=%+v error=%v", result, repeated, err)
			}
		})
	}
}

func TestSourceBundleIngesterRejectsNotReadyAndCriticalStale(t *testing.T) {
	t.Parallel()
	provider, bundle, _ := evidenceFixture(t, research.FreshnessFresh, []research.SourceBundleIssue{research.BundleIssueMissingEvidence}, false)
	result, err := NewSourceBundleIngester(provider).Ingest(context.Background(), EvidenceIngestionRequest{BundleID: bundle.ID})
	if err != nil || result.Accepted || result.Evidence.Eligibility != curriculum.EvidenceNotReady || !containsReason(result.Reasons, "bundle_not_ready: incomplete") {
		t.Fatalf("incomplete result=%+v error=%v", result, err)
	}

	provider, bundle, claimIDs := evidenceFixture(t, research.FreshnessStale, []research.SourceBundleIssue{research.BundleIssueStaleFreshness}, false)
	result, err = NewSourceBundleIngester(provider).Ingest(context.Background(), EvidenceIngestionRequest{BundleID: bundle.ID, CriticalClaimIDs: []research.ClaimID{claimIDs[0]}})
	if err != nil || result.Accepted || !containsReason(result.Reasons, "critical_claim_freshness: stale") {
		t.Fatalf("critical stale result=%+v error=%v", result, err)
	}
}

func TestSourceBundleIngesterRejectsConflictedCriticalClaim(t *testing.T) {
	t.Parallel()
	provider, bundle, claimIDs := evidenceFixture(t, research.FreshnessFresh, []research.SourceBundleIssue{research.BundleIssueUnresolvedConflict}, true)
	result, err := NewSourceBundleIngester(provider).Ingest(context.Background(), EvidenceIngestionRequest{BundleID: bundle.ID, CriticalClaimIDs: []research.ClaimID{claimIDs[0]}})
	if err != nil || result.Accepted || result.Evidence.Eligibility != curriculum.EvidenceNotReady || !containsReason(result.Reasons, "critical_claim_conflict: "+claimIDs[0].String()) {
		t.Fatalf("conflicted result=%+v error=%v", result, err)
	}
}

func TestSourceBundleIngesterPreservesVersionAndHistoricalFlags(t *testing.T) {
	t.Parallel()
	provider, bundle, claimIDs := evidenceFixture(t, research.FreshnessFresh, []research.SourceBundleIssue{research.BundleIssueNonCurrentSource}, false)
	claim := provider.claims[claimIDs[0].String()]
	version, _ := research.NewSourceVersion("1.22")
	claim.VersionScope = &version
	claim.StatusScope = research.ClaimStatusLegacy
	provider.claims[claimIDs[0].String()] = claim
	result, err := NewSourceBundleIngester(provider).Ingest(context.Background(), EvidenceIngestionRequest{BundleID: bundle.ID})
	if err != nil || !result.Accepted || result.Evidence.Claims[0].Status != curriculum.ConceptLegacy || len(result.Evidence.VersionScopes) != 1 || result.Evidence.VersionScopes[0] != "1.22" {
		t.Fatalf("historical result=%+v error=%v", result, err)
	}
}

func TestSourceBundleIngesterReportsMissingBundle(t *testing.T) {
	t.Parallel()
	id, _ := research.NewID("bundle.missing")
	_, err := NewSourceBundleIngester(&fakeResearchBundleProvider{}).Ingest(context.Background(), EvidenceIngestionRequest{BundleID: id})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Ingest() error=%v, want not found", err)
	}
}

type fakeResearchBundleProvider struct {
	bundles   map[string]research.SourceBundle
	claims    map[string]research.Claim
	conflicts map[string]research.Conflict
}

func (provider *fakeResearchBundleProvider) GetBundle(_ context.Context, id research.ID) (research.SourceBundle, error) {
	value, exists := provider.bundles[id.String()]
	if !exists {
		return research.SourceBundle{}, ErrNotFound
	}
	return value, nil
}

func (provider *fakeResearchBundleProvider) GetClaim(_ context.Context, id research.ClaimID) (research.Claim, error) {
	value, exists := provider.claims[id.String()]
	if !exists {
		return research.Claim{}, ErrNotFound
	}
	return value, nil
}

func (provider *fakeResearchBundleProvider) GetConflict(_ context.Context, id research.ID) (research.Conflict, error) {
	value, exists := provider.conflicts[id.String()]
	if !exists {
		return research.Conflict{}, ErrNotFound
	}
	return value, nil
}

func evidenceFixture(t *testing.T, freshness research.FreshnessState, issues []research.SourceBundleIssue, withConflict bool) (*fakeResearchBundleProvider, research.SourceBundle, []research.ClaimID) {
	t.Helper()
	topic, err := research.NewResearchTopic("Go packages", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	claimIDs := []research.ClaimID{researchClaimID(t, "claim.packages")}
	if withConflict {
		claimIDs = append(claimIDs, researchClaimID(t, "claim.packages.opposing"))
	}
	sourceID, _ := research.NewSourceID("source.go-spec")
	scoreValue := 1.0
	if freshness == research.FreshnessAging {
		scoreValue = .6
	}
	if freshness == research.FreshnessStale {
		scoreValue = .2
	}
	score, _ := research.NewFreshnessScore(scoreValue)
	verified := researchTimestamp(t, 12)
	bundleInput := research.SourceBundle{
		ID: researchID(t, "bundle.go-packages"), RunID: researchID(t, "run.go-packages"), Topic: topic,
		Purpose: research.PurposeConceptDefinition, ClaimIDs: claimIDs,
		Sources:   []research.SourceBundleSource{{SourceID: sourceID, Role: research.BundleSourcePrimary, TemporalScope: research.SourceTemporalCurrent}},
		Freshness: research.SourceBundleFreshness{State: freshness, Score: score, LastVerifiedAt: &verified, SourceAlgorithms: []string{research.FreshnessAlgorithmV1}, AlgorithmVersion: research.SourceBundleFreshnessV1},
		Issues:    issues, VerifiedAt: researchTimestamp(t, 13),
	}
	provider := &fakeResearchBundleProvider{bundles: map[string]research.SourceBundle{}, claims: map[string]research.Claim{}, conflicts: map[string]research.Conflict{}}
	confidence, _ := research.NewClaimConfidence(.95)
	for index, claimID := range claimIDs {
		provider.claims[claimID.String()] = research.Claim{ID: claimID, Topic: topic, Statement: "Go package claim " + claimID.String(), Type: research.ClaimDefinition, Scope: "Go packages", StatusScope: research.ClaimStatusStable, Confidence: confidence, SourceIDs: []research.SourceID{sourceID}, EvidenceIDs: []research.ID{researchID(t, "evidence."+string(rune('a'+index)))}, CreatedAt: researchTimestamp(t, 10)}
	}
	if withConflict {
		conflictID := researchID(t, "conflict.packages")
		bundleInput.ConflictIDs = []research.ID{conflictID}
		provider.conflicts[conflictID.String()] = research.Conflict{ID: conflictID, Type: research.ConflictDirectContradiction, ClaimIDs: claimIDs, Confidence: confidence, Reason: "The claims disagree.", Unresolved: true, DetectedAt: researchTimestamp(t, 11), AlgorithmVersion: research.ConflictResolverAlgorithmV1}
	}
	bundle, err := research.SealSourceBundleV1(bundleInput)
	if err != nil {
		t.Fatal(err)
	}
	provider.bundles[bundle.ID.String()] = bundle
	return provider, bundle, claimIDs
}

func researchID(t *testing.T, value string) research.ID {
	t.Helper()
	id, err := research.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func researchClaimID(t *testing.T, value string) research.ClaimID {
	t.Helper()
	id, err := research.NewClaimID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func researchTimestamp(t *testing.T, hour int) research.Timestamp {
	t.Helper()
	value, err := research.NewTimestamp(time.Date(2026, 9, 11, hour, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func containsReason(values []string, target string) bool {
	for _, value := range values {
		if value == target || strings.Contains(value, target) {
			return true
		}
	}
	return false
}
