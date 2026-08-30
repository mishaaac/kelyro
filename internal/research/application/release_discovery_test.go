package application_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/mishaaac/kelyro/internal/infra/researchrelease"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestReleaseDiscoveryIngestsNewStableSnapshotEvidenceAndVersionClaim(t *testing.T) {
	fixture := newReleaseDiscoveryFixture(t)
	fixture.fetch.results = []application.FetchedSource{
		releaseFetchedFixture(t, fixture.source, 12, `{"releases":[{"version":"2.0.0","channel":"stable","released_at":"2026-08-24T11:00:00Z","notes":["New transport API."]}]}`),
	}

	result, err := fixture.service.Discover(context.Background(), application.ResearchModeOnline, fixture.request())
	if err != nil {
		t.Fatal(err)
	}
	if result.AlgorithmVersion != application.ReleaseDiscoveryAlgorithmV1 || len(result.Releases) != 1 || result.CurrentStable == nil {
		t.Fatalf("release discovery result = %+v", result)
	}
	if result.CurrentStable.Version.String() != "2.0.0" || result.CurrentStable.Status != research.ReleaseCurrent || len(result.PreviewReleases) != 0 {
		t.Fatalf("current stable = %+v, previews = %+v", result.CurrentStable, result.PreviewReleases)
	}
	if len(result.Evidence) != 1 || len(result.Claims) != 1 {
		t.Fatalf("release notes output = evidence %+v claims %+v", result.Evidence, result.Claims)
	}
	claim := result.Claims[0]
	if claim.VersionScope == nil || claim.VersionScope.String() != "2.0.0" || claim.StatusScope != research.ClaimStatusStable || claim.Type != research.ClaimVersionChange {
		t.Fatalf("version-scoped release claim = %+v", claim)
	}
	if result.Evidence[0].ExtractorVersion != application.ReleaseNotesIngestionAlgorithmV1 || result.Evidence[0].Excerpt != "New transport API." {
		t.Fatalf("release evidence = %+v", result.Evidence[0])
	}
	if _, err := fixture.repositories.Claims.Get(context.Background(), claim.ID); err != nil {
		t.Fatalf("stored claim: %v", err)
	}
	if snapshots, err := fixture.repositories.Snapshots.ListBySource(context.Background(), fixture.source.ID); err != nil || len(snapshots) != 1 {
		t.Fatalf("stored release snapshots = (%+v, %v)", snapshots, err)
	}
}

func TestReleaseDiscoveryKeepsPreviewSeparateAndSupersedesPriorStable(t *testing.T) {
	fixture := newReleaseDiscoveryFixture(t)
	fixture.fetch.results = []application.FetchedSource{
		releaseFetchedFixture(t, fixture.source, 12, `{"releases":[{"version":"1.0.0","channel":"stable","released_at":"2026-08-23T11:00:00Z"}]}`),
		releaseFetchedFixture(t, fixture.source, 14, `{"releases":[{"version":"2.0.0","channel":"stable","released_at":"2026-08-24T13:00:00Z"},{"version":"3.0.0-rc.1","channel":"rc","released_at":"2026-08-24T13:30:00Z","notes":"Preview only."}]}`),
	}
	request := fixture.request()
	if _, err := fixture.service.Discover(context.Background(), application.ResearchModeOnline, request); err != nil {
		t.Fatal(err)
	}
	second, err := fixture.service.Discover(context.Background(), application.ResearchModeOnline, request)
	if err != nil {
		t.Fatal(err)
	}
	if second.CurrentStable == nil || second.CurrentStable.Version.String() != "2.0.0" {
		t.Fatalf("current stable = %+v", second.CurrentStable)
	}
	if len(second.PreviewReleases) != 1 || second.PreviewReleases[0].Version.String() != "3.0.0-rc.1" || second.PreviewReleases[0].Channel != research.ReleaseRC {
		t.Fatalf("preview releases = %+v", second.PreviewReleases)
	}
	records, err := fixture.repositories.Releases.ListByTechnology(context.Background(), request.TechnologyID)
	if err != nil {
		t.Fatal(err)
	}
	statuses := make(map[string]research.ReleaseStatus)
	for _, record := range records {
		statuses[record.Version.String()] = record.Status
	}
	if statuses["1.0.0"] != research.ReleaseSuperseded || statuses["2.0.0"] != research.ReleaseCurrent {
		t.Fatalf("stable statuses = %+v", statuses)
	}
}

func TestReleaseDiscoveryHandlesNoReleasesMalformedFeedAndDuplicates(t *testing.T) {
	t.Run("no releases", func(t *testing.T) {
		fixture := newReleaseDiscoveryFixture(t)
		fixture.fetch.results = []application.FetchedSource{releaseFetchedFixture(t, fixture.source, 12, `{"releases":[]}`)}
		result, err := fixture.service.Discover(context.Background(), application.ResearchModeOnline, fixture.request())
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Releases) != 0 || result.CurrentStable != nil {
			t.Fatalf("empty feed result = %+v", result)
		}
	})

	t.Run("retired release is not current", func(t *testing.T) {
		fixture := newReleaseDiscoveryFixture(t)
		version, err := research.NewVersionIdentifier("1.0.0")
		if err != nil {
			t.Fatal(err)
		}
		releasedAt := testTimestamp(t, 8)
		retired := research.TechnologyRelease{
			ID: testID(t, "release.retired"), TechnologyID: fixture.request().TechnologyID,
			Version: version, Channel: research.ReleaseStable, Status: research.ReleaseEOL,
			SourceIDs: []research.SourceID{fixture.source.ID}, ReleasedAt: &releasedAt, VerifiedAt: testTimestamp(t, 10),
		}
		if err := fixture.repositories.Releases.Create(context.Background(), retired); err != nil {
			t.Fatal(err)
		}
		fixture.fetch.results = []application.FetchedSource{releaseFetchedFixture(t, fixture.source, 12, `{"releases":[]}`)}
		result, err := fixture.service.Discover(context.Background(), application.ResearchModeOnline, fixture.request())
		if err != nil {
			t.Fatal(err)
		}
		if result.CurrentStable != nil {
			t.Fatalf("retired release selected as current = %+v", result.CurrentStable)
		}
	})

	t.Run("malformed feed", func(t *testing.T) {
		fixture := newReleaseDiscoveryFixture(t)
		fixture.fetch.results = []application.FetchedSource{releaseFetchedFixture(t, fixture.source, 12, `{"releases":`)}
		_, err := fixture.service.Discover(context.Background(), application.ResearchModeOnline, fixture.request())
		if !errors.Is(err, application.ErrExternalFailure) {
			t.Fatalf("malformed feed error = %v", err)
		}
		snapshots, listErr := fixture.repositories.Snapshots.ListBySource(context.Background(), fixture.source.ID)
		if listErr != nil || len(snapshots) != 1 {
			t.Fatalf("malformed feed snapshot = (%+v, %v)", snapshots, listErr)
		}
	})

	t.Run("duplicate release", func(t *testing.T) {
		fixture := newReleaseDiscoveryFixture(t)
		fixture.fetch.results = []application.FetchedSource{releaseFetchedFixture(t, fixture.source, 12,
			`{"releases":[{"version":"2.0.0","channel":"stable","released_at":"2026-08-24T11:00:00Z","notes":"First change."},{"version":"2.0.0","channel":"stable","released_at":"2026-08-24T11:00:00Z","notes":"Second change."}]}`)}
		result, err := fixture.service.Discover(context.Background(), application.ResearchModeOnline, fixture.request())
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Releases) != 1 || len(result.Claims) != 2 || result.DuplicateCount != 1 {
			t.Fatalf("deduplicated result = %+v", result)
		}
	})
}

func TestReleaseDiscoveryUsesAuthorityPriorityAcrossAuthorizedProviders(t *testing.T) {
	fixture := newReleaseDiscoveryFixture(t)
	ctx := context.Background()
	docs := testSource(t, "official-release-page")
	docs.Kind = research.SourceOfficialDocumentation
	if err := fixture.repositories.Sources.Create(ctx, docs); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repositories.TrustRegistry.SaveDecision(ctx, research.TrustDecision{
		SourceID: docs.ID, State: research.TrustAccepted, Tier: research.AuthorityTierA,
		Reasons: []research.TrustReason{{Code: "fixture.accepted"}}, Policy: "trust-policy-v1", EvaluatedAt: testTimestamp(t, 10),
	}); err != nil {
		t.Fatal(err)
	}
	feed := `{"releases":[{"version":"2.0.0","channel":"stable","released_at":"2026-08-24T11:00:00Z","notes":"Shared change."}]}`
	fixture.fetch.results = []application.FetchedSource{
		releaseFetchedFixture(t, fixture.source, 12, feed),
		releaseFetchedFixture(t, docs, 13, feed),
	}
	request := fixture.request()
	request.Sources = []application.ReleaseDiscoverySource{
		{SourceID: docs.ID, Provider: "official-json"},
		{SourceID: fixture.source.ID, Provider: "official-json"},
	}
	result, err := fixture.service.Discover(ctx, application.ResearchModeOnline, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixture.fetch.requests) != 2 || fixture.fetch.requests[0].SourceID != fixture.source.ID || fixture.fetch.requests[1].SourceID != docs.ID {
		t.Fatalf("authority source order = %+v", fixture.fetch.requests)
	}
	if len(result.Releases) != 1 || len(result.Releases[0].SourceIDs) != 2 || len(result.Claims) != 1 || len(result.Claims[0].EvidenceIDs) != 2 {
		t.Fatalf("multi-source release ingestion = %+v", result)
	}
}

type releaseDiscoveryFixture struct {
	source       research.Source
	repositories application.Repositories
	fetch        *recordingCaptureFetchService
	service      application.ReleaseDiscoveryService
	profile      research.AuthorityProfile
}

func newReleaseDiscoveryFixture(t *testing.T) releaseDiscoveryFixture {
	t.Helper()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	source := testSource(t, "release-feed")
	source.Kind = research.SourceReleaseNotes
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	decision := research.TrustDecision{
		SourceID: source.ID, State: research.TrustAccepted, Tier: research.AuthorityTierA,
		Reasons: []research.TrustReason{{Code: "fixture.accepted", Detail: "Official release feed."}},
		Policy:  "trust-policy-v1", EvaluatedAt: testTimestamp(t, 10),
	}
	if err := repositories.TrustRegistry.SaveDecision(ctx, decision); err != nil {
		t.Fatal(err)
	}
	profile := research.AuthorityProfile{
		ID: testID(t, "profile.release"), Version: "authority-profile-v1", Domain: "software", TopicPattern: "*",
		PreferredKinds:       []research.SourceKind{research.SourceReleaseNotes, research.SourceOfficialDocumentation, research.SourceCode, research.SourcePackageReference},
		MinimumCorroboration: 1, MinimumTier: research.AuthorityTierB, CreatedAt: testTimestamp(t, 9),
	}
	fetch := &recordingCaptureFetchService{}
	sequence := 0
	capture := application.NewSnapshotCaptureService(repositories.Sources, repositories.Snapshots, fetch,
		application.WithSnapshotIDGenerator(func() (research.ID, error) {
			sequence++
			return research.NewID(fmt.Sprintf("snapshot.release.%d", sequence))
		}))
	provider, err := researchrelease.NewJSONProvider("official-json")
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewReleaseDiscoveryService(repositories.Sources, repositories.TrustRegistry, capture,
		repositories.Releases, repositories.ReleaseIngestion, provider)
	return releaseDiscoveryFixture{source: source, repositories: repositories, fetch: fetch, service: service, profile: profile}
}

func (fixture releaseDiscoveryFixture) request() application.ReleaseDiscoveryRequest {
	topic, _ := research.NewResearchTopic("Runtime releases", "software", "Fixture Runtime")
	return application.ReleaseDiscoveryRequest{
		TechnologyID: testIDNoFail("technology.release-fixture"), Topic: topic, Profile: fixture.profile,
		Sources:      []application.ReleaseDiscoverySource{{SourceID: fixture.source.ID, Provider: "official-json"}},
		MaximumBytes: application.MaximumReleaseFeedBytes,
	}
}

func releaseFetchedFixture(t *testing.T, source research.Source, hour int, body string) application.FetchedSource {
	t.Helper()
	bytes := []byte(body)
	return application.FetchedSource{
		SourceID: source.ID, Locator: source.Locator, FetchedAt: testTimestamp(t, hour), Origin: application.FetchOriginLive,
		Metadata: research.FetchMetadata{StatusCode: http.StatusOK, ContentType: "application/json", ContentLength: int64(len(bytes)), ContentHash: research.CanonicalContentHashV1(bytes), FetchVersion: "source-fetch-v1"},
		Body:     bytes,
	}
}

func testIDNoFail(value string) research.ID {
	id, err := research.NewID(value)
	if err != nil {
		panic(err)
	}
	return id
}
