package application_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

const fixtureVersion = "i03-step02-v1"

func TestResearchAndSourceServicesUseNarrowMemoryRepositories(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	researchService := application.NewResearchService(repositories.Runs)
	sourceService := application.NewSourceService(repositories.Sources, repositories.Snapshots)

	request, run := testRequestRun(t)
	if err := researchService.Start(ctx, request, run); err != nil {
		t.Fatalf("ResearchService.Start() error = %v", err)
	}
	loadedRun, err := researchService.Run(ctx, run.ID)
	if err != nil || loadedRun.ID != run.ID {
		t.Fatalf("ResearchService.Run() = (%+v, %v)", loadedRun, err)
	}
	completedAt := testTimestamp(t, 12)
	run.Status = research.ResearchRunCompleted
	run.CompletedAt = &completedAt
	if err := researchService.UpdateRun(ctx, run); err != nil {
		t.Fatalf("ResearchService.UpdateRun() error = %v", err)
	}
	retry := research.ResearchRun{
		ID: testID(t, "run.interfaces.retry"), RequestID: request.ID,
		Status: research.ResearchRunRunning, StartedAt: testTimestamp(t, 13),
	}
	if err := researchService.Start(ctx, request, retry); err != nil {
		t.Fatalf("ResearchService.Start() retry error = %v", err)
	}

	source := testSource(t, "spec")
	if err := sourceService.Register(ctx, source); err != nil {
		t.Fatalf("SourceService.Register() error = %v", err)
	}
	snapshot := testSnapshot(t, source, "001", 10)
	if err := sourceService.RecordSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("SourceService.RecordSnapshot() error = %v", err)
	}
	latest, err := sourceService.LatestSnapshot(ctx, source.ID)
	if err != nil || latest.ID != snapshot.ID {
		t.Fatalf("SourceService.LatestSnapshot() = (%+v, %v)", latest, err)
	}
	listed, err := sourceService.List(ctx)
	if err != nil || len(listed) != 1 || listed[0].ID != source.ID {
		t.Fatalf("SourceService.List() = (%+v, %v)", listed, err)
	}
	if err := sourceService.ClassifyTemporalScope(ctx, source.ID, research.SourceTemporalArchived); err != nil {
		t.Fatalf("ClassifyTemporalScope(archived) error = %v", err)
	}
	classified, err := sourceService.Get(ctx, source.ID)
	if err != nil || classified.TemporalScope != research.SourceTemporalArchived {
		t.Fatalf("classified source = (%+v, %v)", classified, err)
	}
	if err := sourceService.ClassifyTemporalScope(ctx, source.ID, research.SourceTemporalVersionBound); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("version-bound source without version error = %v, want invalid_state", err)
	}
}

func TestResearchServiceRecordsImmutableReproducibleAuditTrail(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	service := application.NewResearchService(repositories.Runs)
	request, run := testRequestRun(t)
	if err := service.Start(ctx, request, run); err != nil {
		t.Fatal(err)
	}
	first, err := research.SealResearchRunAuditV1(research.ResearchRunAudit{
		ID: testID(t, "audit.interfaces.running"), RunID: run.ID,
		RecordedAt: testTimestamp(t, 11), StartedAt: run.StartedAt, Outcome: run.Status,
		QueryPlannerVersion: "query-planner-v1", TrustPolicyVersion: "trust-policy-v1",
		FreshnessVersion: research.FreshnessAlgorithmV1, ConflictResolverVersion: research.ConflictResolverAlgorithmV1,
		NetworkMode: research.ResearchAuditNetworkAuto, Queries: []string{"Go interfaces official documentation"},
		TargetTechnology: request.Topic.Technology,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RecordAudit(ctx, first); err != nil {
		t.Fatalf("RecordAudit(running) error = %v", err)
	}
	completedAt := testTimestamp(t, 12)
	run.Status, run.CompletedAt = research.ResearchRunCompleted, &completedAt
	if err := service.UpdateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	final := first
	final.ID = testID(t, "audit.interfaces.completed")
	final.RecordedAt = testTimestamp(t, 13)
	final.Outcome, final.CompletedAt, final.ContentHash = run.Status, run.CompletedAt, ""
	final, err = research.SealResearchRunAuditV1(final)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RecordAudit(ctx, final); err != nil {
		t.Fatalf("RecordAudit(completed) error = %v", err)
	}
	trail, err := service.AuditTrail(ctx, run.ID)
	if err != nil || len(trail) != 2 || trail[0].Outcome != research.ResearchRunRunning || trail[1].Outcome != research.ResearchRunCompleted {
		t.Fatalf("AuditTrail() = (%+v, %v)", trail, err)
	}
	trail[0].Queries[0] = "mutated"
	again, err := service.AuditTrail(ctx, run.ID)
	if err != nil || again[0].Queries[0] != "Go interfaces official documentation" {
		t.Fatalf("AuditTrail() defensive copy = (%+v, %v)", again, err)
	}
	if err := service.RecordAudit(ctx, final); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate audit error = %v, want conflict", err)
	}
	mismatch := final
	mismatch.ID = testID(t, "audit.interfaces.mismatch")
	mismatch.Outcome = research.ResearchRunFailed
	mismatch.ContentHash = ""
	mismatch, err = research.SealResearchRunAuditV1(mismatch)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RecordAudit(ctx, mismatch); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("mismatched lifecycle error = %v, want invalid state", err)
	}
}

func TestMemorySourceRepositoryDefensivelyCopiesSpecializedMetadata(t *testing.T) {
	t.Parallel()
	repositories := memory.New().Repositories()
	service := application.NewSourceService(repositories.Sources, repositories.Snapshots)
	version := testVersion(t, "runtime-2")
	shareable := testLocator(t, "playground/share/abc")
	source := research.Source{
		ID: testSourceID(t, "playground"), Kind: research.SourcePlayground,
		Locator: testLocator(t, "playground"), Version: &version,
		TemporalScope: research.SourceTemporalVersionBound,
		Metadata:      research.SourceMetadata{Title: "Interactive runtime"},
		Specialization: &research.SourceSpecialization{
			Kind: research.SourcePlayground, AlgorithmVersion: research.SpecializedSourceMetadataV1,
			Playground: &research.PlaygroundDetails{
				Interactive: true, LanguageRuntime: "Portable runtime", Version: &version,
				Affiliation: research.SourceAffiliationCommunity, ShareableLocator: shareable,
			},
		},
		CreatedAt: testTimestamp(t, 9),
	}
	if err := service.Register(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	source.Specialization.Playground.LanguageRuntime = "mutated caller input"
	loaded, err := service.Get(context.Background(), source.ID)
	if err != nil || loaded.Specialization.Playground.LanguageRuntime != "Portable runtime" {
		t.Fatalf("first specialized source read = (%+v, %v)", loaded, err)
	}
	loaded.Specialization.Playground.LanguageRuntime = "mutated returned value"
	again, err := service.Get(context.Background(), source.ID)
	if err != nil || again.Specialization.Playground.LanguageRuntime != "Portable runtime" {
		t.Fatalf("second specialized source read = (%+v, %v)", again, err)
	}
}

func TestMemorySourceRepositoryDefensivelyCopiesVideoMetadata(t *testing.T) {
	t.Parallel()
	repositories := memory.New().Repositories()
	service := application.NewSourceService(repositories.Sources, repositories.Snapshots)
	published := testTimestamp(t, 8)
	locator := testLocator(t, "video")
	deepLink := testLocator(t, "video?at=60")
	source := research.Source{
		ID: testSourceID(t, "video"), Kind: research.SourceVideo, Locator: locator,
		TemporalScope: research.SourceTemporalCurrent,
		Metadata:      research.SourceMetadata{Title: "Portable video", Publisher: "Conference", PublishedAt: &published},
		Video: &research.VideoSupplementMetadata{
			VideoLocator: locator, Channel: "Conference Sessions", DurationSeconds: 600,
			Affiliation: research.SourceAffiliationOfficial, TranscriptAvailability: research.TranscriptAvailable,
			DeepLinks:        []research.VideoDeepLink{{OffsetSeconds: 60, Locator: deepLink}},
			AlgorithmVersion: research.VideoSupplementMetadataV1,
		},
		CreatedAt: testTimestamp(t, 9),
	}
	if err := service.Register(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	source.Video.DeepLinks[0].OffsetSeconds = 90
	loaded, err := service.Get(context.Background(), source.ID)
	if err != nil || loaded.Video.DeepLinks[0].OffsetSeconds != 60 {
		t.Fatalf("first video source read = (%+v, %v)", loaded, err)
	}
	loaded.Video.Channel = "mutated"
	again, err := service.Get(context.Background(), source.ID)
	if err != nil || again.Video.Channel != "Conference Sessions" {
		t.Fatalf("second video source read = (%+v, %v)", again, err)
	}
}

func TestSourceRegistryServicePreservesEntriesAndRejectsDuplicateDomains(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	service := application.NewSourceRegistryService(repositories.SourceRegistry)
	trusted := testRegistryEntry(t, "registry.alpha", "Docs.Example.Test.", research.RegistryTrusted)
	if err := service.Save(ctx, trusted); err != nil {
		t.Fatal(err)
	}
	historical := testRegistryEntry(t, "registry.history", "archive.example.test", research.RegistryHistorical)
	if err := service.Save(ctx, historical); err != nil {
		t.Fatal(err)
	}
	entries, err := service.List(ctx)
	if err != nil || len(entries) != 2 || entries[0].ID != trusted.ID || entries[1].Status != research.RegistryHistorical {
		t.Fatalf("List() = (%+v, %v)", entries, err)
	}
	entries[0].ResearchDomains[0] = "changed"
	loaded, err := service.Get(ctx, trusted.ID)
	if err != nil || loaded.ResearchDomains[0] != "software" {
		t.Fatalf("Get() defensive copy = (%+v, %v)", loaded, err)
	}
	duplicate := testRegistryEntry(t, "registry.duplicate", "docs.example.test", research.RegistryConditional)
	if err := service.Save(ctx, duplicate); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("Save(duplicate domain) error = %v, want conflict", err)
	}
}

func TestServicesClassifyValidationRepositoryAndContextErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	service := application.NewSourceService(store.Repositories().Sources, store.Repositories().Snapshots)
	source := testSource(t, "spec")

	invalidSource := source
	invalidSource.Kind = research.SourceKind("invented")
	if err := service.Register(ctx, invalidSource); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("invalid Register() error = %v, want invalid_state", err)
	}
	if err := service.Register(ctx, source); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if err := service.Register(ctx, source); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate Register() error = %v, want conflict", err)
	}
	if _, err := service.Get(ctx, testSourceID(t, "missing")); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("missing Get() error = %v, want not_found", err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.Get(canceled, source.ID); !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("canceled Get() error = %v, want unavailable", err)
	}

	wantCause := errors.New("driver exploded")
	failing := application.NewSourceService(failingSourceRepository{err: wantCause}, nil)
	_, err := failing.Get(ctx, source.ID)
	if !errors.Is(err, application.ErrPersistenceFailure) || !errors.Is(err, wantCause) {
		t.Fatalf("unclassified repository error = %v", err)
	}
	if kind, ok := application.KindOf(err); !ok || kind != application.ErrorPersistenceFailure {
		t.Fatalf("KindOf() = (%q, %v)", kind, ok)
	}

	missingDependency := application.NewSourceService(nil, nil)
	if _, err := missingDependency.Get(ctx, source.ID); !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("missing repository error = %v, want unavailable", err)
	}
}

func TestDiscoveryServiceMapsProviderFailuresAndRejectsInvalidResults(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	query := application.SearchQuery{
		RequestID: testID(t, "request.search"), Text: "official interface specification",
	}
	options := application.SearchOptions{Limit: 5}
	wantCause := errors.New("provider unavailable")
	access := application.NetworkResearchAccess{Gate: privacy.NewNetworkGate(privacy.Policy{AllowNetwork: true}, nil)}
	service := application.NewDiscoveryService(searchProviderStub{err: wantCause}, nil, access)
	_, err := service.Search(ctx, application.ResearchModeOnline, query, options)
	if !errors.Is(err, application.ErrExternalFailure) || !errors.Is(err, wantCause) {
		t.Fatalf("provider error = %v, want external_failure preserving cause", err)
	}

	service = application.NewDiscoveryService(searchProviderStub{results: []application.SearchResult{{
		Title: "Specification", Locator: testLocator(t, "spec"), Provider: "fixture", Rank: 0,
	}}}, nil, access)
	results, err := service.Search(ctx, application.ResearchModeOnline, query, options)
	if err != nil || len(results) != 1 {
		t.Fatalf("DiscoveryService.Search() = (%+v, %v)", results, err)
	}

	service = application.NewDiscoveryService(searchProviderStub{results: []application.SearchResult{{
		Title: "", Locator: testLocator(t, "bad"), Provider: "fixture", Rank: 0,
	}}}, nil, access)
	if _, err := service.Search(ctx, application.ResearchModeOnline, query, options); !errors.Is(err, application.ErrExternalFailure) {
		t.Fatalf("invalid provider result error = %v, want external_failure", err)
	}

	options.Limit = 0
	if _, err := service.Search(ctx, application.ResearchModeOnline, query, options); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("invalid query error = %v, want invalid_state", err)
	}
}

func TestIntelligenceServicesPersistOnlyValidatedPolicyOutputs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	source := testSource(t, "release-notes")
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}

	freshnessService := application.NewFreshnessService(repositories.Freshness)
	dueAt := testTimestamp(t, 12)
	freshness := application.FreshnessRecord{
		SubjectID: testID(t, "claim.release"), State: research.FreshnessAging,
		Score: testFreshnessScore(t, 0.6), LastVerifiedAt: testTimestamp(t, 10),
		NextVerifyAt: &dueAt, VerificationReason: research.VerificationTTLExpired,
		Priority: research.VerificationPriorityNormal, AlgorithmVersion: research.FreshnessAlgorithmV1,
		SchedulingAlgorithmVersion: research.RefreshSchedulingAlgorithmV1,
	}
	if err := freshnessService.Save(ctx, freshness); err != nil {
		t.Fatalf("FreshnessService.Save() error = %v", err)
	}
	critical := freshness
	critical.SubjectID = testID(t, "claim.manual")
	critical.VerificationReason = research.VerificationManualRequest
	critical.Priority = research.VerificationPriorityCritical
	if err := freshnessService.Save(ctx, critical); err != nil {
		t.Fatalf("FreshnessService.Save(critical) error = %v", err)
	}
	futureAt := testTimestamp(t, 13)
	future := freshness
	future.SubjectID = testID(t, "claim.future")
	future.NextVerifyAt = &futureAt
	if err := freshnessService.Save(ctx, future); err != nil {
		t.Fatalf("FreshnessService.Save(future) error = %v", err)
	}
	due, err := freshnessService.Due(ctx, dueAt)
	if err != nil || len(due) != 2 || due[0].SubjectID != critical.SubjectID || due[1].SubjectID != freshness.SubjectID {
		t.Fatalf("FreshnessService.Due() = (%+v, %v)", due, err)
	}

	releaseService := application.NewReleaseIntelligenceService(repositories.Releases)
	release := testRelease(t, source.ID)
	if err := releaseService.Record(ctx, release); err != nil {
		t.Fatalf("ReleaseIntelligenceService.Record() error = %v", err)
	}
	if releases, err := releaseService.List(ctx, release.TechnologyID); err != nil || len(releases) != 1 {
		t.Fatalf("ReleaseIntelligenceService.List() = (%+v, %v)", releases, err)
	} else if releases[0].Version.Scheme() != research.VersionDateBased {
		t.Fatalf("release version scheme = %q, want date_based", releases[0].Version.Scheme())
	}

	driftService := application.NewDriftService(repositories.Drift)
	drift := testDrift(t)
	if err := driftService.Record(ctx, drift); err != nil {
		t.Fatalf("DriftService.Record() error = %v", err)
	}
	impactService := application.NewImpactService(repositories.Drift, repositories.Impact)
	impact := testImpact(t, drift)
	if err := impactService.Record(ctx, impact); err != nil {
		t.Fatalf("ImpactService.Record() error = %v", err)
	}
	if loaded, err := impactService.Get(ctx, impact.ID); err != nil || loaded.DriftReportID != drift.ID {
		t.Fatalf("ImpactService.Get() = (%+v, %v)", loaded, err)
	}
}

func TestExternalAdapterDTOsValidateBoundedTransportNeutralData(t *testing.T) {
	t.Parallel()

	source := testSource(t, "docs")
	fetchRequest := application.FetchRequest{
		SourceID: source.ID, Locator: source.Locator, MaximumBytes: 1024,
	}
	if err := fetchRequest.Validate(); err != nil {
		t.Fatalf("FetchRequest.Validate() error = %v", err)
	}
	fetched := application.FetchedSource{
		SourceID: source.ID, Locator: source.Locator, FetchedAt: testTimestamp(t, 10),
		Metadata: research.FetchMetadata{
			StatusCode: 200, ContentType: "text/html", ContentHash: "sha256:abc",
			ContentLength: 10, FetchVersion: fixtureVersion,
		},
		Body: []byte("fixture"), Origin: application.FetchOriginLive,
	}
	if err := fetched.Validate(); err != nil {
		t.Fatalf("FetchedSource.Validate() error = %v", err)
	}
	normalized := application.NormalizedSource{
		SourceID: source.ID, Locator: source.Locator, Title: "Documentation",
		ContentType: "text/html", Language: "en",
		TextSegments: []string{"A bounded normalized segment."}, NormalizationVersion: fixtureVersion,
	}
	if err := normalized.Validate(); err != nil {
		t.Fatalf("NormalizedSource.Validate() error = %v", err)
	}
	fetchRequest.MaximumBytes = 0
	if err := fetchRequest.Validate(); err == nil {
		t.Fatal("FetchRequest.Validate() accepted unbounded request")
	}
	fetched.Body = nil
	if err := fetched.Validate(); err == nil {
		t.Fatal("FetchedSource.Validate() accepted empty successful body")
	}
}

func TestMemoryFakesPreserveRelationshipsOrderingAndOwnership(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	source := testSource(t, "spec")
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	first := testSnapshot(t, source, "001", 10)
	second := testSnapshot(t, source, "002", 11)
	if err := repositories.Snapshots.Append(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Snapshots.Append(ctx, first); err != nil {
		t.Fatal(err)
	}
	snapshots, err := repositories.Snapshots.ListBySource(ctx, source.ID)
	if err != nil || len(snapshots) != 2 || snapshots[0].ID != first.ID || snapshots[1].ID != second.ID {
		t.Fatalf("Snapshots.ListBySource() = (%+v, %v)", snapshots, err)
	}

	evidence := research.Evidence{
		ID: testID(t, "evidence.spec"), SourceID: source.ID, SnapshotID: second.ID,
		Location: "section", Excerpt: "bounded fixture",
		ExcerptHash: research.CanonicalEvidenceExcerptHashV1("bounded fixture"),
		ExtractedAt: testTimestamp(t, 12), ExtractorVersion: fixtureVersion,
	}
	if err := repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	if items, err := repositories.Evidence.ListBySnapshot(ctx, second.ID); err != nil || len(items) != 1 {
		t.Fatalf("Evidence.ListBySnapshot() = (%+v, %v)", items, err)
	}

	securityClaim := research.ClaimSecurity
	profile := research.AuthorityProfile{
		ID: testID(t, "authority.software"), Version: fixtureVersion, Domain: "software",
		PreferredKinds: []research.SourceKind{research.SourceSpecification}, PreferredDomains: []string{"example.com"},
		PreferredOrganizations: []string{"Example"}, MinimumCorroboration: 1,
		AllowedSupplementaryKinds: []research.SourceKind{research.SourceCommunityArticle},
		FreshnessTTLHints:         []research.FreshnessTTLHint{{ClaimType: &securityClaim, TTLDays: 14}},
		MinimumTier:               research.AuthorityTierB, CreatedAt: testTimestamp(t, 9),
	}
	if err := repositories.TrustRegistry.SaveProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	loadedProfile, err := repositories.TrustRegistry.GetProfile(ctx, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	loadedProfile.PreferredKinds[0] = research.SourceVideo
	loadedProfile.PreferredDomains[0] = "changed.example"
	loadedProfile.PreferredOrganizations[0] = "Changed"
	loadedProfile.AllowedSupplementaryKinds[0] = research.SourceVideo
	*loadedProfile.FreshnessTTLHints[0].ClaimType = research.ClaimHistorical
	loadedProfile.FreshnessTTLHints[0].TTLDays = 300
	reloadedProfile, err := repositories.TrustRegistry.GetProfile(ctx, profile.ID)
	if err != nil || reloadedProfile.PreferredKinds[0] != research.SourceSpecification ||
		reloadedProfile.PreferredDomains[0] != "example.com" || reloadedProfile.PreferredOrganizations[0] != "Example" ||
		reloadedProfile.AllowedSupplementaryKinds[0] != research.SourceCommunityArticle ||
		*reloadedProfile.FreshnessTTLHints[0].ClaimType != research.ClaimSecurity || reloadedProfile.FreshnessTTLHints[0].TTLDays != 14 {
		t.Fatalf("authority profile fake leaked mutable slice: (%+v, %v)", reloadedProfile, err)
	}

	expiresAt := testTimestamp(t, 14)
	entry := application.CacheEntry{
		Key: "discovery:interfaces", Payload: []byte("fixture"), ContentHash: "sha256:cache",
		StoredAt: testTimestamp(t, 13), ExpiresAt: &expiresAt,
	}
	if err := repositories.Cache.Put(ctx, entry); err != nil {
		t.Fatal(err)
	}
	loadedEntry, err := repositories.Cache.Get(ctx, entry.Key)
	if err != nil {
		t.Fatal(err)
	}
	loadedEntry.Payload[0] = 'X'
	reloadedEntry, err := repositories.Cache.Get(ctx, entry.Key)
	if err != nil || string(reloadedEntry.Payload) != "fixture" {
		t.Fatalf("cache fake leaked mutable payload: (%q, %v)", reloadedEntry.Payload, err)
	}
}

func TestProvenanceServiceRecordsTracesAndExportsLatestGraph(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repository := memory.New().Repositories().Provenance
	service := application.NewProvenanceService(repository)
	first := testProvenanceGraph(t, "001", 15)
	second := testProvenanceGraph(t, "002", 16)
	if err := service.Record(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := service.Record(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := service.Record(ctx, second); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate Record() error = %v", err)
	}
	loaded, err := service.Trace(ctx, second.ClaimID)
	if err != nil || loaded.ID != second.ID {
		t.Fatalf("Trace() = (%+v, %v)", loaded, err)
	}
	loaded.Nodes[0].Label = "changed"
	reloaded, err := service.Trace(ctx, second.ClaimID)
	if err != nil || reloaded.Nodes[0].Label == "changed" {
		t.Fatalf("Trace() leaked graph ownership: (%+v, %v)", reloaded, err)
	}
	exported, err := service.Export(ctx, second.ClaimID)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := research.ParseProvenanceGraphJSON(exported)
	if err != nil || parsed.ID != second.ID {
		t.Fatalf("Export() parsed = (%+v, %v)", parsed, err)
	}
}

func testProvenanceGraph(t *testing.T, suffix string, recordedHour int) research.ProvenanceGraph {
	t.Helper()
	claimID := testClaimID(t, "trace")
	claimNodeID, err := research.NewID(claimID.String())
	if err != nil {
		t.Fatal(err)
	}
	requestID := testID(t, "request.trace")
	runID := testID(t, "run.trace")
	sourceID := testID(t, "source.trace")
	snapshotID := testID(t, "snapshot.trace")
	evidenceID := testID(t, "evidence.trace")
	return research.ProvenanceGraph{
		ID: testID(t, "graph.trace."+suffix), ClaimID: claimID,
		RecordedAt: testTimestamp(t, recordedHour), AlgorithmVersion: research.ProvenanceGraphAlgorithmV1,
		Nodes: []research.ProvenanceNode{
			{ID: requestID, Kind: research.ProvenanceRequest, Label: "trace request", OccurredAt: testTimestamp(t, 9)},
			{ID: runID, Kind: research.ProvenanceRun, Label: "trace run", OccurredAt: testTimestamp(t, 10)},
			{ID: sourceID, Kind: research.ProvenanceSource, Label: "manual source", OccurredAt: testTimestamp(t, 8)},
			{ID: snapshotID, Kind: research.ProvenanceSnapshot, Label: "historical snapshot", OccurredAt: testTimestamp(t, 11), ToolVersion: "fetch/v1"},
			{ID: evidenceID, Kind: research.ProvenanceEvidence, Label: "section 1", OccurredAt: testTimestamp(t, 12), ToolVersion: "extract/v1"},
			{ID: claimNodeID, Kind: research.ProvenanceClaim, Label: "traceable claim", OccurredAt: testTimestamp(t, 13)},
		},
		Edges: []research.ProvenanceEdge{
			{From: requestID, To: runID}, {From: runID, To: sourceID},
			{From: sourceID, To: snapshotID}, {From: snapshotID, To: evidenceID},
			{From: evidenceID, To: claimNodeID},
		},
	}
}

type failingSourceRepository struct{ err error }

func (repository failingSourceRepository) Create(context.Context, research.Source) error {
	return repository.err
}
func (repository failingSourceRepository) Get(context.Context, research.SourceID) (research.Source, error) {
	return research.Source{}, repository.err
}
func (repository failingSourceRepository) FindByLocator(context.Context, research.SourceLocator) (research.Source, error) {
	return research.Source{}, repository.err
}
func (repository failingSourceRepository) List(context.Context) ([]research.Source, error) {
	return nil, repository.err
}
func (repository failingSourceRepository) SetTemporalScope(context.Context, research.SourceID, research.SourceTemporalScope) error {
	return repository.err
}

type searchProviderStub struct {
	results []application.SearchResult
	err     error
}

type fixedClock struct{ now research.Timestamp }

func (clock fixedClock) Now() research.Timestamp { return clock.now }

var _ application.Clock = fixedClock{}

func (provider searchProviderStub) Search(context.Context, application.SearchQuery, application.SearchOptions) ([]application.SearchResult, error) {
	return provider.results, provider.err
}

func testRequestRun(t *testing.T) (research.ResearchRequest, research.ResearchRun) {
	t.Helper()
	topic, err := research.NewResearchTopic("Interfaces", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	request := research.ResearchRequest{
		ID: testID(t, "request.interfaces"), Topic: topic,
		Purpose: research.PurposeConceptDefinition, RequestedAt: testTimestamp(t, 9),
	}
	run := research.ResearchRun{
		ID: testID(t, "run.interfaces"), RequestID: request.ID,
		Status: research.ResearchRunRunning, StartedAt: testTimestamp(t, 10),
	}
	return request, run
}

func testSource(t *testing.T, suffix string) research.Source {
	t.Helper()
	return research.Source{
		ID: testSourceID(t, suffix), Kind: research.SourceOfficialDocumentation,
		Locator: testLocator(t, suffix), TemporalScope: research.SourceTemporalCurrent,
		Metadata:  research.SourceMetadata{Title: "Fixture " + suffix},
		CreatedAt: testTimestamp(t, 9),
	}
}

func testSnapshot(t *testing.T, source research.Source, suffix string, hour int) research.SourceSnapshot {
	t.Helper()
	return research.SourceSnapshot{
		ID: testID(t, "snapshot."+suffix), SourceID: source.ID, Locator: source.Locator,
		FetchedAt: testTimestamp(t, hour),
		Fetch: research.FetchMetadata{
			StatusCode: 200, ContentType: "text/html", ContentHash: "sha256:" + suffix,
			ContentLength: 100, FetchVersion: fixtureVersion,
		},
	}
}

func testRelease(t *testing.T, sourceID research.SourceID) research.ReleaseRecord {
	t.Helper()
	return research.ReleaseRecord{
		ID: testID(t, "release.current"), TechnologyID: testID(t, "technology.fixture"),
		Version: testVersion(t, "2026.08"), Channel: research.ReleaseStable,
		Status: research.ReleaseCurrent, SourceIDs: []research.SourceID{sourceID},
		VerifiedAt: testTimestamp(t, 11),
	}
}

func testDrift(t *testing.T) research.DriftReport {
	t.Helper()
	newBundle := testID(t, "bundle.new")
	return research.DriftReport{
		ID: testID(t, "drift.current"), OldBundleID: testID(t, "bundle.old"),
		NewBundleID: &newBundle, Type: research.DriftSourceChanged, Severity: research.SeverityImportant,
		AffectedClaims: []research.ClaimID{testClaimID(t, "release.current")},
		OldEvidence:    []research.ID{testID(t, "evidence.old")},
		NewEvidence:    []research.ID{testID(t, "evidence.new")}, Confidence: testConfidence(t, .8),
		DetectedAt: testTimestamp(t, 12), AlgorithmVersion: research.DriftAlgorithmV1,
	}
}

func testImpact(t *testing.T, drift research.DriftReport) research.ImpactReport {
	t.Helper()
	return research.ImpactReport{
		ID: testID(t, "impact.current"), DriftReportID: drift.ID,
		AffectedEvidenceIDs: append(append([]research.ID(nil), drift.OldEvidence...), drift.NewEvidence...),
		AffectedBundleIDs:   []research.ID{drift.OldBundleID},
		AffectedClaimIDs:    drift.AffectedClaims, Severity: research.SeverityImportant,
		RecommendedAction: research.ActionReviewCurriculum, AssessedAt: testTimestamp(t, 13),
		AlgorithmVersion: research.ImpactAnalysisAlgorithmV1,
	}
}

func testRegistryEntry(t *testing.T, idSuffix, domainValue string, status research.RegistryStatus) research.SourceRegistryEntry {
	t.Helper()
	domain, err := research.NewCanonicalDomain(domainValue)
	if err != nil {
		t.Fatal(err)
	}
	return research.SourceRegistryEntry{
		ID: testID(t, idSuffix), Organization: "Example", CanonicalDomains: []research.CanonicalDomain{domain},
		SourceKinds:     []research.SourceKind{research.SourceOfficialDocumentation},
		AuthorityHints:  []research.RegistryAuthorityHint{{SourceKind: research.SourceOfficialDocumentation, Tier: research.AuthorityTierB, Reason: "Fixture authority hint."}},
		ResearchDomains: []string{"software"}, TopicPatterns: []string{"*"}, Status: status,
		AddedAt: testTimestamp(t, 8), LastReviewedAt: testTimestamp(t, 9),
	}
}

func testID(t *testing.T, suffix string) research.ID {
	t.Helper()
	id, err := research.NewID(fixtureVersion + "." + suffix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func testSourceID(t *testing.T, suffix string) research.SourceID {
	t.Helper()
	id, err := research.NewSourceID(fixtureVersion + ".source." + suffix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func testClaimID(t *testing.T, suffix string) research.ClaimID {
	t.Helper()
	id, err := research.NewClaimID(fixtureVersion + ".claim." + suffix)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func testTimestamp(t *testing.T, hour int) research.Timestamp {
	t.Helper()
	timestamp, err := research.NewTimestamp(time.Date(2026, 8, 24, hour, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}

func testLocator(t *testing.T, suffix string) research.SourceLocator {
	t.Helper()
	locator, err := research.NewSourceLocator("https://example.test/" + suffix)
	if err != nil {
		t.Fatal(err)
	}
	return locator
}

func testVersion(t *testing.T, value string) research.SourceVersion {
	t.Helper()
	version, err := research.NewSourceVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func testConfidence(t *testing.T, value float64) research.ClaimConfidence {
	t.Helper()
	confidence, err := research.NewClaimConfidence(value)
	if err != nil {
		t.Fatal(err)
	}
	return confidence
}

func testFreshnessScore(t *testing.T, value float64) research.FreshnessScore {
	t.Helper()
	if math.IsNaN(value) {
		t.Fatal("test score is NaN")
	}
	score, err := research.NewFreshnessScore(value)
	if err != nil {
		t.Fatal(err)
	}
	return score
}
