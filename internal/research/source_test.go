package research

import "testing"

func TestAllInitialSourceKindsAreValid(t *testing.T) {
	t.Parallel()

	kinds := []SourceKind{
		SourceOfficialDocumentation, SourceSpecification, SourceStandard,
		SourceReleaseNotes, SourceOfficialBlog, SourcePackageReference,
		SourceOfficialTutorial, SourceCode, SourceIssueTracker,
		SourceCommunityArticle, SourceCommunityForum, SourceVideo,
		SourcePaper, SourceBookReference, SourceOther,
	}
	seen := make(map[SourceKind]struct{}, len(kinds))
	for _, kind := range kinds {
		if err := kind.Validate(); err != nil {
			t.Errorf("%q.Validate() error = %v", kind, err)
		}
		if _, exists := seen[kind]; exists {
			t.Errorf("duplicate source kind %q", kind)
		}
		seen[kind] = struct{}{}
	}
	if err := SourceKind("social_post").Validate(); err == nil {
		t.Fatal("SourceKind.Validate() accepted unknown kind")
	}
}

func TestSourceAndImmutableSnapshotRequireTraceableMetadata(t *testing.T) {
	t.Parallel()

	publishedAt := mustTimestamp(t, 9)
	updatedAt := mustTimestamp(t, 10)
	version := mustVersion(t, "go1.25")
	source := Source{
		ID:            mustSourceID(t, "spec"),
		Kind:          SourceSpecification,
		Locator:       mustLocator(t, "spec"),
		Version:       &version,
		TemporalScope: SourceTemporalCurrent,
		Metadata: SourceMetadata{
			Title: "Language specification", Publisher: "Example standards body",
			Language: "en", PublishedAt: &publishedAt, UpdatedAt: &updatedAt,
		},
		CreatedAt: mustTimestamp(t, 11),
	}
	if err := source.Validate(); err != nil {
		t.Fatalf("Source.Validate() error = %v", err)
	}

	snapshot := SourceSnapshot{
		ID: mustID(t, "snapshot.spec.001"), SourceID: source.ID,
		Locator: source.Locator, FetchedAt: mustTimestamp(t, 12),
		Fetch: FetchMetadata{
			StatusCode: 200, ContentType: "text/html", ContentHash: "sha256:abc",
			ContentLength: 1234, FetchVersion: domainFixtureVersion,
		},
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("SourceSnapshot.Validate() error = %v", err)
	}

	snapshot.FetchedAt = Timestamp{}
	if err := snapshot.Validate(); err == nil {
		t.Fatal("SourceSnapshot.Validate() accepted missing fetched_at")
	}
	snapshot.FetchedAt = mustTimestamp(t, 12)
	snapshot.SourceID = SourceID{}
	if err := snapshot.Validate(); err == nil {
		t.Fatal("SourceSnapshot.Validate() accepted missing source identity")
	}
}

func TestSourceTemporalScopesRequireExplicitVersionForVersionBound(t *testing.T) {
	t.Parallel()
	for _, scope := range []SourceTemporalScope{
		SourceTemporalCurrent, SourceTemporalHistorical,
		SourceTemporalVersionBound, SourceTemporalArchived,
	} {
		if err := scope.Validate(); err != nil {
			t.Errorf("%q.Validate() error = %v", scope, err)
		}
	}
	if err := SourceTemporalScope("oldish").Validate(); err == nil {
		t.Fatal("SourceTemporalScope.Validate() accepted unknown scope")
	}
	source := Source{
		ID: mustSourceID(t, "version-bound"), Kind: SourceOfficialDocumentation,
		Locator: mustLocator(t, "version-bound"), TemporalScope: SourceTemporalVersionBound,
		Metadata: SourceMetadata{Title: "Old reference"}, CreatedAt: mustTimestamp(t, 10),
	}
	if err := source.Validate(); err == nil {
		t.Fatal("Source.Validate() accepted a version-bound source without version")
	}
	version := mustVersion(t, "1.0")
	source.Version = &version
	if err := source.Validate(); err != nil {
		t.Fatalf("version-bound Source.Validate() error = %v", err)
	}
	if warning, err := source.TemporalScope.Warning(source.Version); err != nil || warning == "" {
		t.Fatalf("version-bound warning = (%q, %v)", warning, err)
	}
}

func TestSourceMetadataRejectsReversedPublicationTimeline(t *testing.T) {
	t.Parallel()

	publishedAt := mustTimestamp(t, 11)
	updatedAt := mustTimestamp(t, 10)
	metadata := SourceMetadata{Title: "Reference", PublishedAt: &publishedAt, UpdatedAt: &updatedAt}
	if err := metadata.Validate(); err == nil {
		t.Fatal("SourceMetadata.Validate() accepted update before publication")
	}
}

func TestResearchRunAuthorityTrustAndDiscoveryValidateEnumsAndTime(t *testing.T) {
	t.Parallel()

	topic, _ := NewResearchTopic("Interfaces", "software", "Go")
	version := mustVersion(t, "go1.25")
	request := ResearchRequest{
		ID: mustID(t, "request.interfaces"), Topic: topic,
		Purpose: PurposeCurrentUsage, TargetVersion: &version, RequestedAt: mustTimestamp(t, 9),
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("ResearchRequest.Validate() error = %v", err)
	}
	completedAt := mustTimestamp(t, 11)
	run := ResearchRun{
		ID: mustID(t, "run.interfaces"), RequestID: request.ID,
		Status: ResearchRunCompleted, StartedAt: mustTimestamp(t, 10), CompletedAt: &completedAt,
	}
	if err := run.Validate(); err != nil {
		t.Fatalf("ResearchRun.Validate() error = %v", err)
	}
	run.CompletedAt = nil
	if err := run.Validate(); err == nil {
		t.Fatal("ResearchRun.Validate() accepted terminal run without completion")
	}

	profile := AuthorityProfile{
		ID: mustID(t, "authority.software"), Version: domainFixtureVersion,
		Domain: "software", PreferredKinds: []SourceKind{SourceSpecification, SourceOfficialDocumentation},
		MinimumCorroboration: 1, MinimumTier: AuthorityTierB, CreatedAt: mustTimestamp(t, 9),
	}
	if err := profile.Validate(); err != nil {
		t.Fatalf("AuthorityProfile.Validate() error = %v", err)
	}
	decision := TrustDecision{
		SourceID: mustSourceID(t, "spec"), State: TrustAccepted, Tier: AuthorityTierA,
		Reasons: []TrustReason{{Code: "normative_primary"}}, Policy: domainFixtureVersion,
		EvaluatedAt: mustTimestamp(t, 11),
	}
	if err := decision.Validate(); err != nil {
		t.Fatalf("TrustDecision.Validate() error = %v", err)
	}
	discovered := DiscoveredSource{
		ID: mustID(t, "discovery.spec"), RequestID: request.ID, Locator: mustLocator(t, "spec"),
		Title: "Specification", Provider: "fixture-search", Rank: 0, DiscoveredAt: mustTimestamp(t, 10),
	}
	if err := discovered.Validate(); err != nil {
		t.Fatalf("DiscoveredSource.Validate() error = %v", err)
	}
	discovered.Provider = ""
	if err := discovered.Validate(); err == nil {
		t.Fatal("DiscoveredSource.Validate() accepted missing provider")
	}
}
