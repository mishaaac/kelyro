package live_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/infra/researchfetch"
	"github.com/mishaaac/kelyro/internal/infra/researchhttp"
	"github.com/mishaaac/kelyro/internal/infra/researchnormalize"
	"github.com/mishaaac/kelyro/internal/infra/researchsearch"
	"github.com/mishaaac/kelyro/internal/infra/secretstore"
	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

const liveResearchSearchEnvironment = "KELYRO_LIVE_RESEARCH_SEARCH_TESTS"

func TestLiveResearchSearchProvider(t *testing.T) {
	if os.Getenv(liveResearchSearchEnvironment) != "1" {
		t.Skipf("set %s=1 to run the external search provider smoke", liveResearchSearchEnvironment)
	}
	discovery := newLiveSearchDiscovery(t)

	requestID, err := research.NewID("request.live.search-provider")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	results, err := discovery.Search(ctx, application.ResearchModeOnline, application.SearchQuery{
		RequestID: requestID,
		Text:      "Go programming language official documentation",
	}, application.SearchOptions{Limit: 3})
	if err != nil {
		t.Fatalf("live search provider query failed: %v", err)
	}
	if len(results) == 0 || len(results) > 3 {
		t.Fatalf("live search provider returned %d results, want 1..3", len(results))
	}

	seen := make(map[string]struct{}, len(results))
	for index, result := range results {
		if err := result.Validate(); err != nil {
			t.Fatalf("live search result %d: %v", index, err)
		}
		if result.Provider != researchsearch.ProviderID || result.Rank < 0 ||
			result.Locator.String() == researchsearch.EndpointURL {
			t.Fatalf("live search result %d has invalid provider metadata", index)
		}
		locator := result.Locator.String()
		if _, duplicate := seen[locator]; duplicate {
			t.Fatalf("live search provider repeated result URL at position %d", index)
		}
		seen[locator] = struct{}{}
	}
}

func TestLiveResearchQueryToBundle(t *testing.T) {
	if os.Getenv(liveResearchSearchEnvironment) != "1" {
		t.Skipf("set %s=1 to run the live query-to-bundle smoke", liveResearchSearchEnvironment)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	discovery := newLiveSearchDiscovery(t)
	requestID := mustLiveID(t, "request.live.query-to-bundle")
	results, err := discovery.Search(ctx, application.ResearchModeOnline, application.SearchQuery{
		RequestID: requestID,
		Text:      `site:go.dev/doc/code "A module is a collection of related Go packages"`,
	}, application.SearchOptions{Limit: 5})
	if err != nil {
		t.Fatalf("live query-to-bundle search: %v", err)
	}
	selected, found := selectOfficialGoResult(results)
	if !found {
		t.Fatalf("live search returned no HTTPS go.dev documentation result among %d candidates", len(results))
	}

	clock := liveClock{}
	now := clock.Now()
	topic, err := research.NewResearchTopic("Go module", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	source := research.Source{
		ID:   liveSourceID(t, selected.Locator.String()),
		Kind: research.SourceOfficialDocumentation, Locator: selected.Locator,
		TemporalScope: research.SourceTemporalCurrent,
		Metadata: research.SourceMetadata{
			Title: selected.Title, Publisher: "The Go Authors", Language: "en",
		},
		CreatedAt: now,
	}
	domain, err := research.NewCanonicalDomain("go.dev")
	if err != nil {
		t.Fatal(err)
	}
	registryEntry := research.SourceRegistryEntry{
		ID:               mustLiveID(t, "registry.live.query-to-bundle.go-dev"),
		Organization:     "The Go Authors",
		CanonicalDomains: []research.CanonicalDomain{domain},
		SourceKinds:      []research.SourceKind{research.SourceOfficialDocumentation},
		AuthorityHints: []research.RegistryAuthorityHint{{
			SourceKind: research.SourceOfficialDocumentation,
			Tier:       research.AuthorityTierB,
			Reason:     "Official Go documentation selected dynamically by the live smoke.",
		}},
		ResearchDomains: []string{"software"}, TopicPatterns: []string{"*"},
		Status: research.RegistryTrusted, AddedAt: now, LastReviewedAt: now,
	}

	store := memory.New()
	repositories := store.Repositories()
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	if err := repositories.SourceRegistry.Save(ctx, registryEntry); err != nil {
		t.Fatal(err)
	}
	researchRequest := research.ResearchRequest{
		ID: requestID, Topic: topic, Purpose: research.PurposeConceptDefinition, RequestedAt: now,
	}
	run := research.ResearchRun{
		ID: mustLiveID(t, "run.live.query-to-bundle"), RequestID: requestID,
		Status: research.ResearchRunRunning, StartedAt: now,
	}
	if err := repositories.Runs.Create(ctx, researchRequest, run); err != nil {
		t.Fatal(err)
	}

	fetchConfig := researchhttp.DefaultConfig()
	fetchConfig.UserAgent = "Kelyro/live-query-to-bundle-test"
	fetchConfig.RequestTimeout = 15 * time.Second
	fetchConfig.DialTimeout = 5 * time.Second
	fetchConfig.TLSHandshakeTimeout = 5 * time.Second
	fetchConfig.ResponseHeaderTimeout = 10 * time.Second
	fetchConfig.MaxResponseBytes = 2 << 20
	fetchConfig.MaxAttempts = 1
	fetchConfig.MaxRedirects = 3
	fetchConfig.MinimumIntervalPerHost = 10 * time.Millisecond
	fetchClient, err := researchhttp.New(fetchConfig, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(fetchClient.CloseIdleConnections)
	access := application.NetworkResearchAccess{
		Gate: privacy.NewNetworkGate(privacy.Policy{AllowNetwork: true}, nil),
	}
	captureService := application.NewSnapshotCaptureService(
		repositories.Sources,
		repositories.Snapshots,
		application.NewFetchService(researchfetch.New(fetchClient), nil, access),
	)
	capture, err := captureService.Capture(ctx, application.ResearchModeOnline, application.SnapshotCaptureRequest{
		SourceID: source.ID, MaximumBytes: 2 << 20, BodyPolicy: application.SnapshotNormalizedExcerpt,
	})
	if err != nil {
		t.Fatalf("fetch discovered live result: %v", err)
	}
	if capture.NormalizationInput == nil || capture.Snapshot.Locator != selected.Locator {
		t.Fatalf("live snapshot does not preserve discovered result identity")
	}
	normalized, err := researchnormalize.New().Normalize(ctx, *capture.NormalizationInput)
	if err != nil {
		t.Fatalf("normalize discovered live result: %v", err)
	}

	artifacts := application.LiveResearchArtifacts{
		SearchResults:       []application.SearchResult{selected},
		Sources:             []research.Source{source},
		FetchedSources:      []application.FetchedSource{*capture.NormalizationInput},
		Snapshots:           []research.SourceSnapshot{capture.Snapshot},
		NormalizationInputs: []application.FetchedSource{*capture.NormalizationInput},
		NormalizedSources:   []application.NormalizedSource{normalized},
	}
	input := application.LiveResearchStageInput{Request: researchRequest, Run: run, Artifacts: artifacts}
	evidenceStage, err := application.NewLiveEvidenceExtractionService(
		application.NewDeterministicEvidenceExtractorV1(), repositories.Evidence, clock,
	)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err = evidenceStage.Execute(ctx, input)
	if err != nil {
		t.Fatalf("extract live Evidence: %v", err)
	}
	input.Artifacts = artifacts
	claimStage, err := application.NewLiveClaimExtractionService(
		application.NewDeterministicClaimExtractorV1(), repositories.Evidence, repositories.Claims,
		repositories.Citations, clock,
	)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err = claimStage.Execute(ctx, input)
	if err != nil {
		t.Fatalf("extract live Claims: %v", err)
	}

	registry := application.NewSourceRegistryService(repositories.SourceRegistry)
	trustStage, err := application.NewLiveTrustEvaluationService(repositories.TrustRegistry, registry, clock)
	if err != nil {
		t.Fatal(err)
	}
	temporalStage, err := application.NewLiveTemporalEvaluationService(
		application.NewFreshnessService(repositories.Freshness), trustStage, repositories.TrustRegistry,
		repositories.Releases, repositories.Deprecations, clock,
	)
	if err != nil {
		t.Fatal(err)
	}
	input.Artifacts = artifacts
	artifacts, err = temporalStage.Execute(ctx, input)
	if err != nil {
		t.Fatalf("evaluate live trust and freshness: %v", err)
	}

	verification := application.NewVerificationService(
		repositories.Verification, repositories.Claims, repositories.Sources, repositories.TrustRegistry,
		repositories.SourceRegistry, repositories.Conflicts, clock,
	)
	diversity := application.NewSourceDiversityService(
		repositories.Claims, repositories.Sources, repositories.TrustRegistry, repositories.SourceRegistry,
	)
	verificationStage, err := application.NewLiveMultiSourceVerificationService(verification, diversity)
	if err != nil {
		t.Fatal(err)
	}
	input.Artifacts = artifacts
	artifacts, err = verificationStage.Execute(ctx, input)
	if err != nil {
		t.Fatalf("verify live Claims: %v", err)
	}

	bundles := application.NewSourceBundleService(
		repositories.Bundles, repositories.Runs, repositories.Claims, repositories.Sources,
		repositories.Evidence, repositories.TrustRegistry, repositories.Verification,
		repositories.Conflicts, repositories.Freshness, clock,
	)
	bundleStage, err := application.NewLiveSourceBundleService(bundles)
	if err != nil {
		t.Fatal(err)
	}
	input.Artifacts = artifacts
	artifacts, err = bundleStage.Execute(ctx, input)
	if err != nil {
		t.Fatalf("assemble live Source Bundle: %v", err)
	}

	if len(artifacts.Evidence) == 0 || len(artifacts.Claims) == 0 || len(artifacts.TrustDecisions) == 0 ||
		len(artifacts.Verifications) != len(artifacts.Claims) || artifacts.Bundle == nil {
		t.Fatalf(
			"live query-to-bundle artifacts are incomplete: evidence=%d claims=%d trust=%d verification=%d bundle=%v",
			len(artifacts.Evidence), len(artifacts.Claims), len(artifacts.TrustDecisions), len(artifacts.Verifications), artifacts.Bundle != nil,
		)
	}
	for _, result := range artifacts.Verifications {
		if result.Status != research.VerificationVerified && result.Status != research.VerificationVerifiedCaveat {
			t.Fatalf("live verification %q status = %s", result.ClaimID, result.Status)
		}
	}
	if artifacts.Bundle.RunID != run.ID || artifacts.Bundle.ContentHash == "" ||
		(artifacts.Bundle.State != research.BundleReady && artifacts.Bundle.State != research.BundleReadyWithCaveats) {
		t.Fatalf("live Source Bundle = %+v", artifacts.Bundle)
	}
	stored, err := bundles.Get(ctx, artifacts.Bundle.ID)
	if err != nil || stored.ContentHash != artifacts.Bundle.ContentHash {
		t.Fatalf("durable live Source Bundle = (%+v, %v)", stored, err)
	}
}

func newLiveSearchDiscovery(t *testing.T) application.DiscoveryService {
	t.Helper()
	transportConfig := researchsearch.DefaultTransportConfig()
	transportConfig.UserAgent = "Kelyro/live-research-search-test"
	transportConfig.RequestTimeout = 15 * time.Second
	transportConfig.DialTimeout = 5 * time.Second
	transportConfig.TLSHandshakeTimeout = 5 * time.Second
	transportConfig.ResponseHeaderTimeout = 10 * time.Second
	client, err := researchsearch.NewBraveHTTPClient(transportConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.CloseIdleConnections)

	provider, readiness, err := researchsearch.NewBraveFromSecrets(client, secretstore.New())
	if err != nil {
		t.Fatalf("build live search provider (%s): %v", readiness, err)
	}
	return application.NewDiscoveryService(
		provider,
		nil,
		application.NetworkResearchAccess{
			Gate: privacy.NewNetworkGate(privacy.Policy{AllowNetwork: true}, nil),
		},
	)
}

func selectOfficialGoResult(results []application.SearchResult) (application.SearchResult, bool) {
	for _, result := range results {
		parsed, err := url.Parse(result.Locator.String())
		if err == nil && parsed.Scheme == "https" && parsed.Hostname() == "go.dev" &&
			strings.HasPrefix(parsed.EscapedPath(), "/doc/") {
			return result, true
		}
	}
	return application.SearchResult{}, false
}

func mustLiveID(t *testing.T, value string) research.ID {
	t.Helper()
	id, err := research.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func liveSourceID(t *testing.T, locator string) research.SourceID {
	t.Helper()
	digest := sha256.Sum256([]byte(locator))
	id, err := research.NewSourceID("source.live.search." + hex.EncodeToString(digest[:12]))
	if err != nil {
		t.Fatal(fmt.Errorf("build live Source ID: %w", err))
	}
	return id
}
