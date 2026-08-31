package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestMapSearchResultsToSourceCandidatesPreservesUntrustedDiscoveryMetadata(t *testing.T) {
	t.Parallel()
	published := testTimestamp(t, 8)
	discovered := testTimestamp(t, 10)
	query := application.SearchQuery{RequestID: testID(t, "request.candidate-map"), Text: "  Go\n interfaces docs "}
	results := []application.SearchResult{
		{
			Title: "  Interface\n documentation ", Locator: discoveryLocator(t, "HTTPS://DOCS.EXAMPLE.TEST/interfaces#methods"),
			Snippet: " Defines\tinterface types. ", Provider: " brave\nsearch ", Rank: 3, PublishedHint: &published,
		},
		{
			Title: "Duplicate URL", Locator: discoveryLocator(t, "https://docs.example.test/interfaces#examples"),
			Provider: "brave search", Rank: 7,
		},
	}

	candidates, err := application.MapSearchResultsToSourceCandidatesV1(context.Background(), application.SourceCandidateMappingInput{
		Query: query, Results: results, DiscoveredAt: discovered,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates = %+v, want one per result before Step 18", candidates)
	}
	first := candidates[0]
	if first.Locator.String() != "https://docs.example.test/interfaces" || first.AlgorithmVersion != application.SourceCandidateMapperV1 || len(first.Discoveries) != 1 {
		t.Fatalf("mapped candidate = %+v", first)
	}
	discovery := first.Discoveries[0]
	if discovery.Query.RequestID != query.RequestID || discovery.Query.Text != "Go interfaces docs" || discovery.Title != "Interface documentation" ||
		discovery.Snippet != "Defines interface types." || discovery.Provider != "brave search" || discovery.Rank != 3 ||
		discovery.DiscoveredAt.Time() != discovered.Time() || discovery.PublishedHint == nil || discovery.PublishedHint.Time() != published.Time() {
		t.Fatalf("mapped discovery = %+v", discovery)
	}
	if discovery.PublishedHint == results[0].PublishedHint {
		t.Fatal("candidate retained provider-owned timestamp pointer")
	}
	results[0].PublishedHint = nil
	if candidates[0].Discoveries[0].PublishedHint == nil {
		t.Fatal("candidate changed with provider result")
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("candidate validation = %v", err)
	}
}

func TestSourceCandidateDeduplicationMergesCanonicalURLsAndPreservesEveryQuery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	existing := testSource(t, "candidate-existing")
	existing.Locator = discoveryLocator(t, "https://known.example.test/docs#legacy")
	if err := repositories.Sources.Create(ctx, existing); err != nil {
		t.Fatal(err)
	}
	service, err := application.NewSourceCandidateDeduplicationService(repositories.Sources)
	if err != nil {
		t.Fatal(err)
	}
	first := mappedCandidate(t, "request.dedupe", "first query", "HTTPS://EXAMPLE.TEST:443#one", 1)
	second := mappedCandidate(t, "request.dedupe", "second query", "https://example.test/#two", 2)
	known := mappedCandidate(t, "request.dedupe", "known query", "https://known.example.test/docs#current", 3)

	result, err := service.Deduplicate(ctx, []application.SourceCandidate{first, second, first, known})
	if err != nil {
		t.Fatal(err)
	}
	if result.InputCount != 4 || result.DuplicateCount != 2 || result.ExistingCount != 1 || result.DiscoveryCount != 3 ||
		result.AlgorithmVersion != application.SourceCandidateDeduplicationV1 || len(result.Candidates) != 2 {
		t.Fatalf("deduplication result = %+v", result)
	}
	canonical := result.Candidates[0]
	if canonical.Candidate.Locator.String() != "https://example.test/" || canonical.ExistingSourceID != nil || len(canonical.Candidate.Discoveries) != 2 ||
		canonical.Candidate.Discoveries[0].Query.Text != "first query" || canonical.Candidate.Discoveries[1].Query.Text != "second query" {
		t.Fatalf("canonical candidate = %+v", canonical)
	}
	matched := result.Candidates[1]
	if matched.Candidate.Locator.String() != "https://known.example.test/docs" || matched.ExistingSourceID == nil || *matched.ExistingSourceID != existing.ID {
		t.Fatalf("existing candidate = %+v", matched)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("deduplication result validation = %v", err)
	}
	canonical.Candidate.Discoveries[0].Title = "mutated"
	again, err := service.Deduplicate(ctx, []application.SourceCandidate{first, second, known})
	if err != nil || again.Candidates[0].Candidate.Discoveries[0].Title == "mutated" {
		t.Fatalf("deduplication leaked result ownership: (%+v,%v)", again, err)
	}
}

func TestSourceCandidateDeduplicationKeepsQueryStringsDistinctAndClassifiesFailures(t *testing.T) {
	t.Parallel()
	store := memory.New()
	service, err := application.NewSourceCandidateDeduplicationService(store.Repositories().Sources)
	if err != nil {
		t.Fatal(err)
	}
	one := mappedCandidate(t, "request.dedupe-query", "query one", "https://example.test/docs?version=1", 1)
	two := mappedCandidate(t, "request.dedupe-query", "query two", "https://example.test/docs?version=2", 2)
	result, err := service.Deduplicate(context.Background(), []application.SourceCandidate{one, two})
	if err != nil || len(result.Candidates) != 2 || result.DuplicateCount != 0 {
		t.Fatalf("query-sensitive deduplication = (%+v,%v)", result, err)
	}

	wantCause := errors.New("registry unavailable")
	failing, err := application.NewSourceCandidateDeduplicationService(failingSourceRepository{err: wantCause})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := failing.Deduplicate(context.Background(), []application.SourceCandidate{one}); !errors.Is(err, application.ErrPersistenceFailure) || !errors.Is(err, wantCause) {
		t.Fatalf("repository failure = %v", err)
	}
	empty, err := failing.Deduplicate(context.Background(), nil)
	if err != nil || len(empty.Candidates) != 0 || empty.AlgorithmVersion != application.SourceCandidateDeduplicationV1 {
		t.Fatalf("empty deduplication = (%+v,%v)", empty, err)
	}
	overflow := one
	overflow.Discoveries = make([]application.SourceCandidateDiscovery, application.MaximumDiscoveryCandidatesPerRun)
	for index := range overflow.Discoveries {
		overflow.Discoveries[index] = one.Discoveries[0]
	}
	if _, err := service.Deduplicate(context.Background(), []application.SourceCandidate{overflow, two}); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("unbounded discovery error = %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Deduplicate(cancelled, []application.SourceCandidate{one}); !errors.Is(err, application.ErrUnavailable) || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled deduplication = %v", err)
	}
	if _, err := application.NewSourceCandidateDeduplicationService(nil); !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("missing repository error = %v", err)
	}

	collisionStore := memory.New()
	left := testSource(t, "candidate-collision-left")
	right := testSource(t, "candidate-collision-right")
	left.Locator = discoveryLocator(t, "https://collision.example.test")
	right.Locator = discoveryLocator(t, "https://collision.example.test/")
	if err := collisionStore.Repositories().Sources.Create(context.Background(), left); err != nil {
		t.Fatal(err)
	}
	if err := collisionStore.Repositories().Sources.Create(context.Background(), right); err != nil {
		t.Fatal(err)
	}
	collisionService, err := application.NewSourceCandidateDeduplicationService(collisionStore.Repositories().Sources)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collisionService.Deduplicate(context.Background(), []application.SourceCandidate{
		mappedCandidate(t, "request.dedupe-collision", "collision query", "https://collision.example.test", 1),
	}); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("ambiguous existing canonical locator error = %v", err)
	}
}

func mappedCandidate(t *testing.T, requestID, queryText, locator string, rank int) application.SourceCandidate {
	t.Helper()
	results, err := application.MapSearchResultsToSourceCandidatesV1(context.Background(), application.SourceCandidateMappingInput{
		Query: application.SearchQuery{RequestID: testID(t, requestID), Text: queryText},
		Results: []application.SearchResult{{
			Title: "Result for " + queryText, Locator: discoveryLocator(t, locator), Provider: "fixture", Rank: rank,
		}},
		DiscoveredAt: testTimestamp(t, 10+rank),
	})
	if err != nil {
		t.Fatal(err)
	}
	return results[0]
}

func TestMapSearchResultsToSourceCandidatesRejectsInvalidOrUnboundedInput(t *testing.T) {
	t.Parallel()
	query := application.SearchQuery{RequestID: testID(t, "request.candidate-invalid"), Text: "docs"}
	valid := application.SearchResult{Title: "Docs", Locator: discoveryLocator(t, "https://example.test/docs"), Provider: "fixture", Rank: 0}
	oversized := make([]application.SearchResult, application.MaximumSearchResults+1)
	for index := range oversized {
		oversized[index] = valid
	}
	if _, err := application.MapSearchResultsToSourceCandidatesV1(context.Background(), application.SourceCandidateMappingInput{
		Query: query, Results: oversized, DiscoveredAt: testTimestamp(t, 10),
	}); err == nil {
		t.Fatal("mapper accepted unbounded results")
	}
	invalid := valid
	invalid.Rank = -1
	if _, err := application.MapSearchResultsToSourceCandidatesV1(context.Background(), application.SourceCandidateMappingInput{
		Query: query, Results: []application.SearchResult{invalid}, DiscoveredAt: testTimestamp(t, 10),
	}); err == nil {
		t.Fatal("mapper accepted invalid provider metadata")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := application.MapSearchResultsToSourceCandidatesV1(cancelled, application.SourceCandidateMappingInput{
		Query: query, Results: []application.SearchResult{valid}, DiscoveredAt: testTimestamp(t, 10),
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled mapping error = %v", err)
	}
}
