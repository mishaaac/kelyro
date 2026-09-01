package application_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestDeterministicClaimExtractorV1RecognizesOnlyExplicitSingleFamilyStatements(t *testing.T) {
	t.Parallel()
	topic, err := research.NewResearchTopic("Go module", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		statement string
		family    application.ClaimFamily
		version   string
		status    research.ClaimStatusScope
		admitted  bool
	}{
		{name: "definition", statement: "A Go module is a collection of packages.", family: application.ClaimFamilyExplicitDefinition, status: research.ClaimStatusAll, admitted: true},
		{name: "release", statement: "Go module version 2026 was released as stable.", family: application.ClaimFamilyVersionReleaseFact, version: "2026", status: research.ClaimStatusStable, admitted: true},
		{name: "deprecation", statement: "Go module legacy mode is deprecated.", family: application.ClaimFamilyDeprecationStatement, status: research.ClaimStatusLegacy, admitted: true},
		{name: "availability", statement: "Go module workspaces are supported.", family: application.ClaimFamilyAvailabilitySupport, status: research.ClaimStatusAll, admitted: true},
		{name: "requirement", statement: "A Go module must declare its path.", family: application.ClaimFamilyExplicitRequirement, status: research.ClaimStatusAll, admitted: true},
		{name: "recommendation", statement: "A Go module should pin tool dependencies.", family: application.ClaimFamilyExplicitRecommendation, status: research.ClaimStatusAll, admitted: true},
		{name: "multiple families", statement: "A Go module is deprecated and should be preferred.", admitted: false},
		{name: "hedged", statement: "A Go module could be supported.", admitted: false},
		{name: "unanchored", statement: "This feature is deprecated.", admitted: false},
		{name: "multiple statuses", statement: "Go module workspaces are supported in preview and stable status.", admitted: false},
		{name: "trailing fragment", statement: "A Go module must declare its path", admitted: false},
		{name: "url", statement: "Go module guidance is available at https://example.test.", admitted: false},
	}
	extractor := application.NewDeterministicClaimExtractorV1()
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence := claimEvidenceFixture(t, "family."+string(rune('a'+index)), test.statement, 11)
			result, extractErr := extractor.Extract(context.Background(), application.ClaimExtractionRequest{
				Topic: topic, Purpose: research.PurposeCurrentUsage, Evidence: evidence,
			})
			if extractErr != nil {
				t.Fatal(extractErr)
			}
			if !test.admitted {
				if len(result.Candidates) != 0 {
					t.Fatalf("ambiguous statement admitted: %+v", result.Candidates)
				}
				return
			}
			if len(result.Candidates) != 1 {
				t.Fatalf("candidates = %+v", result.Candidates)
			}
			candidate := result.Candidates[0]
			if candidate.Family != test.family || candidate.Statement != test.statement || candidate.StatusScope != test.status {
				t.Fatalf("candidate = %+v", candidate)
			}
			if test.version == "" && candidate.VersionScope != nil || test.version != "" && (candidate.VersionScope == nil || candidate.VersionScope.String() != test.version) {
				t.Fatalf("version scope = %+v, want %q", candidate.VersionScope, test.version)
			}
		})
	}
}

func TestDeterministicClaimExtractorV1IsStableBoundedAndCancellationAware(t *testing.T) {
	t.Parallel()
	topic, _ := research.NewResearchTopic("Go module", "software", "Go")
	evidence := claimEvidenceFixture(t, "deterministic", "A Go module must declare its path. A Go module should declare its Go version.", 11)
	request := application.ClaimExtractionRequest{Topic: topic, Purpose: research.PurposeCurrentUsage, Evidence: evidence}
	extractor := application.NewDeterministicClaimExtractorV1()
	first, err := extractor.Extract(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := extractor.Extract(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || len(first.Candidates) != 2 || len(first.Candidates) > application.MaximumClaimCandidatesPerEvidence {
		t.Fatalf("unstable or unbounded result first=%+v second=%+v", first, second)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := extractor.Extract(cancelled, request); !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("cancelled extraction error = %v", err)
	}
}

func TestLiveClaimExtractionAggregatesExactStatementsPersistsCitationsAndReplaysIdempotently(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	topic, _ := research.NewResearchTopic("Go module", "software", "Go")
	statement := "A Go module must declare its path."
	var sources []research.Source
	var snapshots []research.SourceSnapshot
	var evidence []research.Evidence
	for index, suffix := range []string{"claim-one", "claim-two"} {
		source := testSource(t, suffix)
		snapshot := testSnapshot(t, source, suffix, 10)
		item := claimEvidenceFixtureForChain(t, suffix, statement, source, snapshot, 11+index-index)
		if err := repositories.Sources.Create(ctx, source); err != nil {
			t.Fatal(err)
		}
		if err := repositories.Snapshots.Append(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
		if err := repositories.Evidence.Append(ctx, item); err != nil {
			t.Fatal(err)
		}
		sources = append(sources, source)
		snapshots = append(snapshots, snapshot)
		evidence = append(evidence, item)
	}
	service, err := application.NewLiveClaimExtractionService(
		application.NewDeterministicClaimExtractorV1(), repositories.Evidence, repositories.Claims, repositories.Citations,
		fixedClock{now: testTimestamp(t, 12)},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := application.LiveClaimExtractionRequest{
		Topic: topic, Purpose: research.PurposeCurrentUsage, Sources: sources, Snapshots: snapshots, Evidence: evidence,
	}
	first, err := service.ExtractClaims(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.ExtractClaims(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || len(first.Candidates) != 2 || len(first.Claims) != 1 || len(first.Citations) != 2 {
		t.Fatalf("live extraction first=%+v second=%+v", first, second)
	}
	claim := first.Claims[0]
	if len(claim.SourceIDs) != 2 || len(claim.EvidenceIDs) != 2 || claim.Statement != statement || claim.Type != research.ClaimRequirement {
		t.Fatalf("aggregated Claim = %+v", claim)
	}
	for _, item := range evidence {
		citations, listErr := repositories.Citations.ListByEvidence(ctx, item.ID)
		if listErr != nil || len(citations) != 1 {
			t.Fatalf("citations for %q = (%+v, %v)", item.ID, citations, listErr)
		}
	}
}

func TestLiveClaimExtractionRejectsUnpersistedOrAmbiguousEvidence(t *testing.T) {
	t.Parallel()
	repositories := memory.New().Repositories()
	topic, _ := research.NewResearchTopic("Go module", "software", "Go")
	source := testSource(t, "claim-invalid")
	snapshot := testSnapshot(t, source, "claim-invalid", 10)
	evidence := claimEvidenceFixtureForChain(t, "claim-invalid", "A Go module might be supported.", source, snapshot, 11)
	service, err := application.NewLiveClaimExtractionService(
		application.NewDeterministicClaimExtractorV1(), repositories.Evidence, repositories.Claims, repositories.Citations,
		fixedClock{now: testTimestamp(t, 12)},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := application.LiveClaimExtractionRequest{Topic: topic, Purpose: research.PurposeCurrentUsage, Sources: []research.Source{source}, Snapshots: []research.SourceSnapshot{snapshot}, Evidence: []research.Evidence{evidence}}
	if _, err := service.ExtractClaims(context.Background(), request); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("unpersisted Evidence error = %v", err)
	}
	if err := repositories.Sources.Create(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Snapshots.Append(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Evidence.Append(context.Background(), evidence); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExtractClaims(context.Background(), request); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("ambiguous Evidence error = %v", err)
	}
}

func TestLiveClaimExtractionRejectsEvidenceAbovePerSourceClaimBoundBeforeWrites(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	topic, _ := research.NewResearchTopic("Go module", "software", "Go")
	source := testSource(t, "claim-source-bound")
	snapshot := testSnapshot(t, source, "claim-source-bound", 10)
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Snapshots.Append(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	evidence := make([]research.Evidence, application.MaximumEvidenceCandidatesPerSource+1)
	for index := range evidence {
		statement := fmt.Sprintf("Go module feature %d is supported.", index)
		item := claimEvidenceFixtureForChain(t, fmt.Sprintf("claim-source-bound.%03d", index), statement, source, snapshot, 11)
		item.Location = fmt.Sprintf("text[%04d]", index)
		if err := repositories.Evidence.Append(ctx, item); err != nil {
			t.Fatal(err)
		}
		evidence[index] = item
	}
	extractor := &recordingClaimExtractor{}
	service, err := application.NewLiveClaimExtractionService(
		extractor, repositories.Evidence, repositories.Claims, repositories.Citations,
		fixedClock{now: testTimestamp(t, 12)},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ExtractClaims(ctx, application.LiveClaimExtractionRequest{
		Topic: topic, Purpose: research.PurposeCurrentUsage, Sources: []research.Source{source},
		Snapshots: []research.SourceSnapshot{snapshot}, Evidence: evidence,
	})
	if !errors.Is(err, application.ErrInvalidState) ||
		!strings.Contains(err.Error(), fmt.Sprintf("%d Claim candidate bound", application.MaximumClaimCandidatesPerSource)) {
		t.Fatalf("per-source Claim bound error = %v", err)
	}
	if extractor.calls != 0 {
		t.Fatalf("Claim extractor ran %d times before source bound rejection", extractor.calls)
	}
}

type recordingClaimExtractor struct{ calls int }

func (extractor *recordingClaimExtractor) Extract(context.Context, application.ClaimExtractionRequest) (application.ClaimExtractionResult, error) {
	extractor.calls++
	return application.ClaimExtractionResult{}, nil
}

func claimEvidenceFixture(t *testing.T, suffix, statement string, hour int) research.Evidence {
	t.Helper()
	return claimEvidenceFixtureForChain(t, suffix, statement, testSource(t, suffix), testSnapshot(t, testSource(t, suffix), suffix, 10), hour)
}

func claimEvidenceFixtureForChain(t *testing.T, suffix, statement string, source research.Source, snapshot research.SourceSnapshot, hour int) research.Evidence {
	t.Helper()
	return research.Evidence{
		ID: testID(t, "evidence."+suffix), SourceID: source.ID, SnapshotID: snapshot.ID,
		Location: "text[0000]", Excerpt: statement, ExcerptHash: research.CanonicalEvidenceExcerptHashV1(statement),
		ExtractedAt: testTimestamp(t, hour), ExtractorVersion: application.EvidenceExtractorV1,
	}
}
