//go:build e2e

package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/infra/researchcachefs"
	"github.com/mishaaac/kelyro/internal/infra/researchfetch"
	"github.com/mishaaac/kelyro/internal/infra/researchhttp"
	"github.com/mishaaac/kelyro/internal/infra/researchnormalize"
	"github.com/mishaaac/kelyro/internal/infra/researchrelease"
	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	conflictpolicy "github.com/mishaaac/kelyro/internal/research/conflict"
	driftpolicy "github.com/mishaaac/kelyro/internal/research/drift"
	"github.com/mishaaac/kelyro/internal/research/queryplanner"
	"github.com/mishaaac/kelyro/internal/research/trust"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
)

func TestResearchEngineEvidencePipelineEndToEnd(t *testing.T) {
	root := moduleRoot(t)
	binary := buildBinary(t, root)
	scenario := newScenario(t, binary)
	scenario.mustRun("init")

	ctx := context.Background()
	fixture := newResearchHTTPFixture(t)
	clock := &researchE2EClock{current: time.Date(2026, 8, 29, 15, 0, 0, 0, time.UTC)}
	database, err := sqlite.Open(ctx, scenario.workspace, sqlite.WithAppVersion("research-e2e"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	repositories := database.Repositories().Research

	config := researchhttp.DefaultConfig()
	config.UserAgent = "Kelyro/research-e2e"
	config.RequestTimeout = 3 * time.Second
	config.DialTimeout = time.Second
	config.InitialBackoff = time.Millisecond
	config.MaxBackoff = 5 * time.Millisecond
	config.MinimumIntervalPerHost = time.Millisecond
	httpClient, err := researchhttp.NewLoopbackFixtureClient(config, fixture.hosts(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(httpClient.CloseIdleConnections)
	fetcher := researchfetch.New(httpClient, researchfetch.WithClock(clock.Time))
	allow := application.NetworkResearchAccess{Gate: privacy.NewNetworkGate(privacy.Policy{AllowNetwork: true}, nil)}
	fetchService := application.NewFetchService(fetcher, nil, allow)
	snapshotSequence := 0
	captureService := application.NewSnapshotCaptureService(repositories.Sources, repositories.Snapshots, fetchService,
		application.WithSnapshotIDGenerator(func() (research.ID, error) {
			snapshotSequence++
			return research.NewID(fmt.Sprintf("snapshot.e2e.%03d", snapshotSequence))
		}))

	topic := mustTopic(t, "Evidence pipeline", "software", "Fixture Runtime")
	profile := research.AuthorityProfile{
		ID: mustID(t, "authority.e2e"), Version: "authority-profile-e2e-v1", Domain: "software",
		TopicPattern: "*", PreferredKinds: []research.SourceKind{research.SourceOfficialDocumentation, research.SourceReleaseNotes},
		PreferredDomains: []string{"official.fixture.test"}, PreferredOrganizations: []string{"Fixture Foundation"},
		MinimumCorroboration: 2, AllowedSupplementaryKinds: []research.SourceKind{research.SourceCommunityArticle},
		MinimumTier: research.AuthorityTierB, CreatedAt: clock.Now(),
	}
	request := research.ResearchRequest{
		ID: mustID(t, "request.e2e.pipeline"), Topic: topic, Purpose: research.PurposeCurrentUsage,
		RequestedAt: clock.Now(),
	}
	run := research.ResearchRun{
		ID: mustID(t, "run.e2e.pipeline"), RequestID: request.ID,
		Status: research.ResearchRunRunning, StartedAt: clock.Now(),
	}
	if err := application.NewResearchService(repositories.Runs).Start(ctx, request, run); err != nil {
		t.Fatal(err)
	}
	plan, err := (queryplanner.PlannerV1{}).Plan(queryplanner.Input{
		Topic: topic, Purpose: request.Purpose, AuthorityProfile: profile,
	})
	if err != nil || plan.AlgorithmVersion != queryplanner.AlgorithmVersion || len(plan.Queries) < 2 {
		t.Fatalf("query plan = (%+v, %v)", plan, err)
	}

	discoveryProvider := &fixtureSearchProvider{results: fixture.searchResults(t)}
	discovery := application.NewDiscoveryService(discoveryProvider, nil, allow)
	candidates, err := discovery.Search(ctx, application.ResearchModeOnline,
		application.SearchQuery{RequestID: request.ID, Text: plan.Queries[0].Query},
		application.SearchOptions{DesiredKind: &plan.Queries[0].DesiredSourceKind, Limit: 20})
	if err != nil || len(candidates) != len(fixture.pages) {
		t.Fatalf("discovery candidates = (%+v, %v)", candidates, err)
	}
	if discoveryProvider.calls != 1 {
		t.Fatalf("live discovery calls = %d, want 1", discoveryProvider.calls)
	}

	sources := make(map[string]research.Source, len(fixture.pages))
	entries := make(map[string]research.SourceRegistryEntry, len(fixture.pages))
	for _, page := range fixture.pages {
		source := fixture.source(t, page, clock.Now())
		if err := repositories.Sources.Create(ctx, source); err != nil {
			t.Fatalf("register %s: %v", page.name, err)
		}
		entry := fixture.registryEntry(t, page, source, clock.Now())
		if err := repositories.SourceRegistry.Save(ctx, entry); err != nil {
			t.Fatalf("save registry %s: %v", page.name, err)
		}
		decision, err := (trust.PolicyV1{}).Evaluate(trust.Input{
			Source: source, Topic: topic, Purpose: request.Purpose, UseCase: trust.UseCaseGeneral,
			Freshness: research.FreshnessFresh, Relevance: trust.RelevanceExact,
			Directness: trust.DirectnessPrimary, Stability: trust.StabilityStable,
			Corroboration: trust.CorroborationIndependent, Registry: &entry, EvaluatedAt: clock.Now(),
		})
		if err != nil {
			t.Fatalf("classify %s: %v", page.name, err)
		}
		if decision.State != research.TrustAccepted && decision.State != research.TrustAcceptedSupplement {
			t.Fatalf("trust decision %s = %+v", page.name, decision)
		}
		if err := repositories.TrustRegistry.SaveDecision(ctx, decision); err != nil {
			t.Fatal(err)
		}
		sources[page.name], entries[page.name] = source, entry
	}

	normalizer := researchnormalize.New()
	evidence := make(map[string]research.Evidence, len(fixture.pages))
	snapshots := make(map[string]research.SourceSnapshot, len(fixture.pages))
	for _, page := range fixture.pages {
		if page.name == "release" {
			continue
		}
		capture, err := captureService.Capture(ctx, application.ResearchModeOnline, application.SnapshotCaptureRequest{
			SourceID: sources[page.name].ID, MaximumBytes: 128 << 10, BodyPolicy: application.SnapshotNormalizedExcerpt,
		})
		if err != nil || capture.NormalizationInput == nil {
			t.Fatalf("capture %s = (%+v, %v)", page.name, capture, err)
		}
		normalized, err := normalizer.Normalize(ctx, *capture.NormalizationInput)
		if err != nil || len(normalized.TextSegments) == 0 {
			t.Fatalf("normalize %s = (%+v, %v)", page.name, normalized, err)
		}
		excerpt := normalized.TextSegments[len(normalized.TextSegments)-1]
		item := research.Evidence{
			ID: mustID(t, "evidence.e2e."+page.name), SourceID: sources[page.name].ID,
			SnapshotID: capture.Snapshot.ID, Location: "normalized.text[last]", Excerpt: excerpt,
			ExcerptHash: research.CanonicalEvidenceExcerptHashV1(excerpt), ExtractedAt: clock.Now(),
			ExtractorVersion: researchnormalize.Version,
		}
		if err := repositories.Evidence.Append(ctx, item); err != nil {
			t.Fatal(err)
		}
		evidence[page.name], snapshots[page.name] = item, capture.Snapshot
	}
	if fixture.callCount("rate-limit") != 2 {
		t.Fatalf("rate-limit retry calls = %d, want 2", fixture.callCount("rate-limit"))
	}

	primary := appendResearchE2EClaim(t, ctx, repositories, clock, topic, "primary", research.ClaimDefinition,
		"Fixture Runtime uses bounded evidence.", []research.Source{sources["official"]}, []research.Evidence{evidence["official"]}, nil)
	recommendation := appendResearchE2EClaim(t, ctx, repositories, clock, topic, "corroboration", research.ClaimRecommendation,
		"Use the fixture transport in production.", []research.Source{sources["official"]}, []research.Evidence{evidence["official"]}, nil)
	changedClaim := appendResearchE2EClaim(t, ctx, repositories, clock, topic, "changed", research.ClaimBehavior,
		"The changed endpoint currently returns generation one.", []research.Source{sources["changed"]}, []research.Evidence{evidence["changed"]}, nil)
	historicalVersion := mustSourceVersion(t, "1.0.0")
	historical := appendResearchE2EClaim(t, ctx, repositories, clock, topic, "historical", research.ClaimHistorical,
		"Version 1 used the historical behavior.", []research.Source{sources["historical"]}, []research.Evidence{evidence["historical"]}, &historicalVersion)

	verificationService := application.NewVerificationService(repositories.Verification, repositories.Claims,
		repositories.Sources, repositories.TrustRegistry, repositories.SourceRegistry, repositories.Conflicts, clock)
	for _, item := range []struct {
		claim research.Claim
		want  research.VerificationStatus
	}{
		{primary, research.VerificationVerified},
		{recommendation, research.VerificationVerifiedCaveat},
		{changedClaim, research.VerificationVerified},
		{historical, research.VerificationVerified},
	} {
		result, err := verificationService.Verify(ctx, item.claim.ID)
		if err != nil || result.Status != item.want {
			t.Fatalf("verify %s = (%+v, %v), want %s", item.claim.ID, result, err, item.want)
		}
	}

	conflictLeft := appendResearchE2EClaim(t, ctx, repositories, clock, topic, "conflict-left", research.ClaimRequirement,
		"Fixture mode must stay enabled.", []research.Source{sources["official"]}, []research.Evidence{evidence["official"]}, nil)
	conflictRight := appendResearchE2EClaim(t, ctx, repositories, clock, topic, "conflict-right", research.ClaimRequirement,
		"Fixture mode must stay disabled.", []research.Source{sources["conflict"]}, []research.Evidence{evidence["conflict"]}, nil)
	conflictService := application.NewConflictResolutionService(repositories.Conflicts, repositories.Claims,
		repositories.Sources, repositories.TrustRegistry, clock)
	conflict, err := conflictService.Assess(ctx, application.ConflictAssessmentRequest{
		Relation: conflictpolicy.RelationContradiction,
		Observations: []application.ConflictObservationRef{
			{ClaimID: conflictLeft.ID, SourceID: sources["official"].ID},
			{ClaimID: conflictRight.ID, SourceID: sources["conflict"].ID},
		},
	})
	if err != nil || len(conflict.ClaimIDs) != 2 {
		t.Fatalf("conflict = (%+v, %v)", conflict, err)
	}

	for _, claim := range []research.Claim{primary, changedClaim, historical} {
		score, _ := research.NewFreshnessScore(.95)
		if err := repositories.Freshness.Save(ctx, application.FreshnessRecord{
			SubjectID: mustID(t, claim.ID.String()), State: research.FreshnessFresh, Score: score,
			LastVerifiedAt: clock.Now(), AlgorithmVersion: research.FreshnessAlgorithmV1,
		}); err != nil {
			t.Fatal(err)
		}
	}
	completedAt := clock.Now()
	run.Status, run.CompletedAt = research.ResearchRunCompleted, &completedAt
	if err := application.NewResearchService(repositories.Runs).UpdateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	bundleService := application.NewSourceBundleService(repositories.Bundles, repositories.Runs, repositories.Claims,
		repositories.Sources, repositories.Evidence, repositories.TrustRegistry, repositories.Verification,
		repositories.Conflicts, repositories.Freshness, clock)
	bundle, err := bundleService.Assemble(ctx, application.AssembleSourceBundleRequest{
		RunID: run.ID, ClaimIDs: []research.ClaimID{primary.ID, changedClaim.ID, historical.ID},
	})
	if err != nil || (bundle.State != research.BundleReady && bundle.State != research.BundleReadyWithCaveats) {
		t.Fatalf("source bundle = (%+v, %v)", bundle, err)
	}
	if !bundleHasTemporalWarning(bundle, sources["historical"].ID) {
		t.Fatalf("historical source warning missing from bundle: %+v", bundle.Sources)
	}

	statusOutput := scenario.mustRun("research", "status", run.ID.String())
	if !strings.Contains(statusOutput, "Evidence pipeline") || !strings.Contains(statusOutput, "Status: ready with caveats") {
		t.Fatalf("research status did not inspect persisted bundle:\n%s", statusOutput)
	}
	sourceOutput := scenario.mustRun("sources", "show", sources["official"].ID.String())
	if !strings.Contains(sourceOutput, sources["official"].ID.String()) || !strings.Contains(sourceOutput, snapshots["official"].Fetch.ContentHash) {
		t.Fatalf("sources show did not inspect persisted snapshot:\n%s", sourceOutput)
	}

	provider, err := researchrelease.NewJSONProvider("official-json")
	if err != nil {
		t.Fatal(err)
	}
	releaseService := application.NewReleaseDiscoveryService(repositories.Sources, repositories.TrustRegistry,
		captureService, repositories.Releases, repositories.ReleaseIngestion, provider)
	releaseResult, err := releaseService.Discover(ctx, application.ResearchModeOnline, application.ReleaseDiscoveryRequest{
		TechnologyID: mustID(t, "technology.e2e.fixture-runtime"), Topic: topic, Profile: profile,
		Sources:      []application.ReleaseDiscoverySource{{SourceID: sources["release"].ID, Provider: "official-json"}},
		MaximumBytes: application.MaximumReleaseFeedBytes,
	})
	if err != nil || releaseResult.CurrentStable == nil || releaseResult.CurrentStable.Version.String() != "2.0.0" || len(releaseResult.Claims) == 0 {
		t.Fatalf("new release = (%+v, %v)", releaseResult, err)
	}

	deprecationClaim := appendResearchE2EClaim(t, ctx, repositories, clock, topic, "deprecation", research.ClaimDeprecation,
		"Legacy transport is deprecated; use current transport.", []research.Source{sources["deprecation"]}, []research.Evidence{evidence["deprecation"]}, nil)
	deprecatedIn := mustSourceVersion(t, "2.0.0")
	deprecationResult, err := application.NewDeprecationIntelligenceService(repositories.Deprecations, repositories.Claims,
		repositories.Evidence, clock).Assess(ctx, application.DeprecationAssessmentRequest{
		Subject: "Legacy transport", Signals: []application.DeprecationSignal{{
			Kind: application.DeprecationSignalExplicitStatement, ClaimID: deprecationClaim.ID,
			EvidenceID: evidence["deprecation"].ID, SourceID: sources["deprecation"].ID,
			Status: research.DeprecationDeprecated, DeprecatedIn: &deprecatedIn, Replacement: "Current transport",
		}},
	})
	if err != nil || deprecationResult.Record.Determination != research.DeprecationExplicitEvidence {
		t.Fatalf("deprecation = (%+v, %v)", deprecationResult, err)
	}

	rateFetched, err := fetchService.Fetch(ctx, application.ResearchModeOnline, application.FetchRequest{
		SourceID: sources["rate-limit"].ID, Locator: sources["rate-limit"].Locator, MaximumBytes: 64 << 10,
	})
	if err != nil || rateFetched.Metadata.StatusCode != http.StatusOK || fixture.callCount("rate-limit") != 3 {
		t.Fatalf("rate-limited fetch = (%+v, %v), calls %d", rateFetched, err, fixture.callCount("rate-limit"))
	}

	rateCalls := fixture.callCount("rate-limit")
	assertResearchE2EOfflineAndPrivacy(t, ctx, scenario.workspace, clock, request, candidates, rateFetched, discoveryProvider, fetcher)
	if fixture.callCount("rate-limit") != rateCalls {
		t.Fatalf("offline/privacy scenarios invoked live fetch: calls %d -> %d", rateCalls, fixture.callCount("rate-limit"))
	}

	clock.Advance(time.Minute)
	revalidated, err := captureService.Capture(ctx, application.ResearchModeOnline, application.SnapshotCaptureRequest{
		SourceID: sources["changed"].ID, MaximumBytes: 128 << 10, BodyPolicy: application.SnapshotNormalizedExcerpt,
	})
	if err != nil || revalidated.RevalidatedSnapshotID == nil || revalidated.NormalizationInput != nil ||
		revalidated.Snapshot.Fetch.ContentHash != snapshots["changed"].Fetch.ContentHash {
		t.Fatalf("ETag revalidation = (%+v, %v)", revalidated, err)
	}
	fixture.changePage()
	clock.Advance(time.Minute)
	changedCapture, err := captureService.Capture(ctx, application.ResearchModeOnline, application.SnapshotCaptureRequest{
		SourceID: sources["changed"].ID, MaximumBytes: 128 << 10, BodyPolicy: application.SnapshotNormalizedExcerpt,
	})
	if err != nil || changedCapture.NormalizationInput == nil ||
		changedCapture.Snapshot.Fetch.ContentHash == snapshots["changed"].Fetch.ContentHash {
		t.Fatalf("changed snapshot = (%+v, %v)", changedCapture, err)
	}
	driftService := application.NewDriftService(repositories.Drift)
	driftResult, err := driftService.Detect(ctx, application.DriftDetectionRequest{
		OldBundle: bundle, OldClaims: []research.Claim{primary, changedClaim, historical},
		NewClaims: []research.Claim{primary, changedClaim, historical}, DetectedAt: clock.Now(),
		SnapshotObservations: []driftpolicy.SnapshotObservation{{
			SourceID: sources["changed"].ID, OldSnapshot: snapshots["changed"],
			NewSnapshot: &changedCapture.Snapshot, AffectedClaims: []research.ClaimID{changedClaim.ID},
		}},
	})
	if err != nil || len(driftResult.Reports) != 1 || driftResult.Reports[0].Type != research.DriftSourceChanged {
		t.Fatalf("source drift = (%+v, %v)", driftResult, err)
	}
	if err := driftService.Record(ctx, driftResult.Reports[0]); err != nil {
		t.Fatal(err)
	}
	if stored, err := driftService.Get(ctx, driftResult.Reports[0].ID); err != nil || stored.Type != research.DriftSourceChanged {
		t.Fatalf("stored drift = (%+v, %v)", stored, err)
	}

	for _, name := range []string{"official", "release", "historical", "community", "conflict", "changed", "deprecation", "rate-limit"} {
		if fixture.callCount(name) == 0 {
			t.Errorf("controlled endpoint %q was not exercised", name)
		}
	}
}

func assertResearchE2EOfflineAndPrivacy(t *testing.T, ctx context.Context, workspace string, clock *researchE2EClock,
	request research.ResearchRequest,
	candidates []application.SearchResult, fetched application.FetchedSource, liveSearch *fixtureSearchProvider,
	liveFetch application.SourceFetcher,
) {
	t.Helper()
	query := application.SearchQuery{RequestID: request.ID, Text: "fixture cached research"}
	options := application.SearchOptions{Limit: 20}
	fetchRequest := application.FetchRequest{SourceID: fetched.SourceID, Locator: fetched.Locator, MaximumBytes: 64 << 10}
	cache, err := researchcachefs.NewFactory().WithClock(clock).Open(ctx, workspace)
	if err != nil {
		t.Fatal(err)
	}
	cacheAdapter := researchcachefs.NewOfflineAdapter(cache)
	if err := cacheAdapter.CacheSearch(ctx, query, options, candidates); err != nil {
		t.Fatal(err)
	}
	if err := cacheAdapter.CacheFetched(ctx, fetchRequest, fetched); err != nil {
		t.Fatal(err)
	}
	offlineDiscovery := application.NewDiscoveryService(liveSearch, cacheAdapter, application.NetworkResearchAccess{})
	offlineFetch := application.NewFetchService(liveFetch, cacheAdapter, application.NetworkResearchAccess{})
	beforeSearch := liveSearch.calls
	results, err := offlineDiscovery.Search(ctx, application.ResearchModeOffline, query, options)
	if err != nil || len(results) == 0 || !results[0].CacheHit || liveSearch.calls != beforeSearch {
		t.Fatalf("offline discovery = (%+v, %v), live calls %d", results, err, liveSearch.calls-beforeSearch)
	}
	result, err := offlineFetch.Fetch(ctx, application.ResearchModeOffline, fetchRequest)
	if err != nil || !result.CacheHit || result.Origin != application.FetchOriginCache {
		t.Fatalf("offline fetch = (%+v, %v)", result, err)
	}

	clock.Advance(8 * 24 * time.Hour)
	results, err = offlineDiscovery.Search(ctx, application.ResearchModeOffline, query, options)
	if err != nil || !results[0].CacheStale || results[0].CacheWarning != application.CacheWarningStaleOffline {
		t.Fatalf("stale discovery cache = (%+v, %v)", results, err)
	}
	result, err = offlineFetch.Fetch(ctx, application.ResearchModeOffline, fetchRequest)
	if err != nil || !result.CacheStale || result.CacheWarning != application.CacheWarningStaleOffline {
		t.Fatalf("stale fetch cache = (%+v, %v)", result, err)
	}

	deny := application.NetworkResearchAccess{Gate: privacy.NewNetworkGate(privacy.Policy{}, nil)}
	blockedDiscovery := application.NewDiscoveryService(liveSearch, nil, deny)
	blockedSearchCalls := liveSearch.calls
	if _, err := blockedDiscovery.Search(ctx, application.ResearchModeOnline, query, options); !errors.Is(err, application.ErrNetworkResearchBlocked) || liveSearch.calls != blockedSearchCalls {
		t.Fatalf("privacy discovery error = %v", err)
	}
	blockedFetch := application.NewFetchService(liveFetch, nil, deny)
	if _, err := blockedFetch.Fetch(ctx, application.ResearchModeOnline, fetchRequest); !errors.Is(err, application.ErrNetworkResearchBlocked) {
		t.Fatalf("privacy fetch error = %v", err)
	}
}

type researchFixturePage struct {
	name, host, path, title, body string
	kind                          research.SourceKind
	temporal                      research.SourceTemporalScope
	version                       string
}

type researchHTTPFixture struct {
	server *httptest.Server
	port   string
	pages  []researchFixturePage
	mu     sync.Mutex
	calls  map[string]int
	body   string
	etag   string
}

func newResearchHTTPFixture(t *testing.T) *researchHTTPFixture {
	t.Helper()
	fixture := &researchHTTPFixture{
		calls: make(map[string]int), body: "The changed endpoint currently returns generation one.", etag: `"generation-1"`,
		pages: []researchFixturePage{
			{name: "official", host: "official.fixture.test", path: "/official", title: "Official documentation", kind: research.SourceOfficialDocumentation, temporal: research.SourceTemporalCurrent, body: "Fixture Runtime uses bounded evidence."},
			{name: "release", host: "release.fixture.test", path: "/release", title: "Release notes", kind: research.SourceReleaseNotes, temporal: research.SourceTemporalCurrent},
			{name: "historical", host: "historical.fixture.test", path: "/historical", title: "Historical documentation", kind: research.SourceOfficialDocumentation, temporal: research.SourceTemporalVersionBound, version: "1.0.0", body: "Version 1 used the historical behavior."},
			{name: "community", host: "community.fixture.test", path: "/community", title: "Community article", kind: research.SourceCommunityArticle, temporal: research.SourceTemporalCurrent, body: "A community maintainer independently supports bounded evidence."},
			{name: "conflict", host: "conflict.fixture.test", path: "/conflict", title: "Conflicting page", kind: research.SourceOfficialDocumentation, temporal: research.SourceTemporalCurrent, body: "Fixture mode must stay disabled."},
			{name: "changed", host: "changed.fixture.test", path: "/changed", title: "Changed page", kind: research.SourceOfficialDocumentation, temporal: research.SourceTemporalCurrent},
			{name: "deprecation", host: "deprecation.fixture.test", path: "/deprecation", title: "Deprecation notice", kind: research.SourceOfficialDocumentation, temporal: research.SourceTemporalCurrent, body: "Legacy transport is deprecated; use current transport."},
			{name: "rate-limit", host: "rate.fixture.test", path: "/rate-limit", title: "Rate limited page", kind: research.SourceOfficialDocumentation, temporal: research.SourceTemporalCurrent, body: "Rate limiting recovered deterministically."},
		},
	}
	fixture.server = httptest.NewServer(http.HandlerFunc(fixture.serveHTTP))
	parsed, err := url.Parse(fixture.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	fixture.port = parsed.Port()
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (fixture *researchHTTPFixture) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	name := strings.TrimPrefix(request.URL.Path, "/")
	fixture.mu.Lock()
	fixture.calls[name]++
	calls, changedBody, changedETag := fixture.calls[name], fixture.body, fixture.etag
	fixture.mu.Unlock()
	if name == "rate-limit" && calls == 1 {
		writer.Header().Set("Retry-After", "0")
		http.Error(writer, "retry", http.StatusTooManyRequests)
		return
	}
	if name == "changed" {
		writer.Header().Set("ETag", changedETag)
		if request.Header.Get("If-None-Match") == changedETag {
			writer.WriteHeader(http.StatusNotModified)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(writer, "<html><head><title>Changed page</title></head><body><p>%s</p></body></html>", changedBody)
		return
	}
	if name == "release" {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"releases":[{"version":"2.0.0","channel":"stable","released_at":"2026-08-29T14:00:00Z","notes":["New bounded transport API."]}]}`))
		return
	}
	for _, page := range fixture.pages {
		if page.name == name {
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(writer, "<html><head><title>%s</title></head><body><h1>%s</h1><p>%s</p></body></html>", page.title, page.title, page.body)
			return
		}
	}
	http.NotFound(writer, request)
}

func (fixture *researchHTTPFixture) hosts() []string {
	hosts := make([]string, 0, len(fixture.pages))
	for _, page := range fixture.pages {
		hosts = append(hosts, page.host)
	}
	return hosts
}

func (fixture *researchHTTPFixture) locator(t *testing.T, page researchFixturePage) research.SourceLocator {
	t.Helper()
	locator, err := research.NewSourceLocator("http://" + page.host + ":" + fixture.port + page.path)
	if err != nil {
		t.Fatal(err)
	}
	return locator
}

func (fixture *researchHTTPFixture) searchResults(t *testing.T) []application.SearchResult {
	results := make([]application.SearchResult, 0, len(fixture.pages))
	for index, page := range fixture.pages {
		results = append(results, application.SearchResult{
			Title: page.title, Locator: fixture.locator(t, page), Snippet: "Controlled E2E candidate",
			Provider: "fixture-search-v1", Rank: index + 1,
		})
	}
	return results
}

func (fixture *researchHTTPFixture) source(t *testing.T, page researchFixturePage, now research.Timestamp) research.Source {
	t.Helper()
	updated := now
	source := research.Source{
		ID: mustSourceID(t, "source.e2e."+page.name), Kind: page.kind, Locator: fixture.locator(t, page),
		TemporalScope: page.temporal, Metadata: research.SourceMetadata{
			Title: page.title, Publisher: "Fixture " + page.name, Language: "en", UpdatedAt: &updated,
		}, CreatedAt: now,
	}
	if page.version != "" {
		version := mustSourceVersion(t, page.version)
		source.Version = &version
	}
	return source
}

func (fixture *researchHTTPFixture) registryEntry(t *testing.T, page researchFixturePage, source research.Source, now research.Timestamp) research.SourceRegistryEntry {
	t.Helper()
	domain, err := research.NewCanonicalDomain(page.host)
	if err != nil {
		t.Fatal(err)
	}
	return research.SourceRegistryEntry{
		ID: mustID(t, "registry.e2e."+page.name), Organization: "Fixture " + page.name,
		CanonicalDomains: []research.CanonicalDomain{domain}, SourceKinds: []research.SourceKind{source.Kind},
		AuthorityHints:  []research.RegistryAuthorityHint{{SourceKind: source.Kind, Tier: fixtureTier(source.Kind), Reason: "Controlled E2E authority."}},
		ResearchDomains: []string{"software"}, TopicPatterns: []string{"*"}, Status: research.RegistryTrusted,
		AddedAt: now, LastReviewedAt: now,
	}
}

func fixtureTier(kind research.SourceKind) research.AuthorityTier {
	if kind == research.SourceCommunityArticle {
		return research.AuthorityTierD
	}
	return research.AuthorityTierB
}

func (fixture *researchHTTPFixture) callCount(name string) int {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return fixture.calls[name]
}

func (fixture *researchHTTPFixture) changePage() {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	fixture.body = "The changed endpoint now returns generation two."
	fixture.etag = `"generation-2"`
}

type fixtureSearchProvider struct {
	results []application.SearchResult
	calls   int
}

func (provider *fixtureSearchProvider) Search(context.Context, application.SearchQuery, application.SearchOptions) ([]application.SearchResult, error) {
	provider.calls++
	return append([]application.SearchResult(nil), provider.results...), nil
}

type researchE2EClock struct{ current time.Time }

func (clock *researchE2EClock) Now() research.Timestamp {
	value, err := research.NewTimestamp(clock.current)
	if err != nil {
		panic(err)
	}
	return value
}

func (clock *researchE2EClock) Time() time.Time { return clock.current }
func (clock *researchE2EClock) Advance(duration time.Duration) {
	clock.current = clock.current.Add(duration)
}

func appendResearchE2EClaim(t *testing.T, ctx context.Context, repositories application.Repositories,
	clock *researchE2EClock, topic research.ResearchTopic, suffix string, claimType research.ClaimType,
	statement string, sources []research.Source, evidence []research.Evidence, version *research.SourceVersion,
) research.Claim {
	t.Helper()
	confidence, err := research.NewClaimConfidence(.9)
	if err != nil {
		t.Fatal(err)
	}
	claim := research.Claim{
		ID: mustClaimID(t, "claim.e2e."+suffix), Topic: topic, Statement: statement, Type: claimType,
		Scope: "Fixture Runtime E2E", VersionScope: version, StatusScope: research.ClaimStatusAll,
		Confidence: confidence, CreatedAt: clock.Now(),
	}
	for _, source := range sources {
		claim.SourceIDs = append(claim.SourceIDs, source.ID)
	}
	for _, item := range evidence {
		claim.EvidenceIDs = append(claim.EvidenceIDs, item.ID)
	}
	if err := repositories.Claims.Append(ctx, claim); err != nil {
		t.Fatal(err)
	}
	return claim
}

func bundleHasTemporalWarning(bundle research.SourceBundle, sourceID research.SourceID) bool {
	for _, source := range bundle.Sources {
		if source.SourceID == sourceID && source.Warning != "" {
			return true
		}
	}
	return false
}

func mustID(t *testing.T, value string) research.ID {
	t.Helper()
	id, err := research.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustSourceID(t *testing.T, value string) research.SourceID {
	t.Helper()
	id, err := research.NewSourceID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustClaimID(t *testing.T, value string) research.ClaimID {
	t.Helper()
	id, err := research.NewClaimID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustTopic(t *testing.T, subject, domain, technology string) research.ResearchTopic {
	t.Helper()
	topic, err := research.NewResearchTopic(subject, domain, technology)
	if err != nil {
		t.Fatal(err)
	}
	return topic
}

func mustSourceVersion(t *testing.T, value string) research.SourceVersion {
	t.Helper()
	version, err := research.NewSourceVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}
