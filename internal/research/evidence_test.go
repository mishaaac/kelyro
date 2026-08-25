package research

import (
	"strings"
	"testing"
)

func TestEvidenceClaimProvenanceAndCitationFormTraceableChain(t *testing.T) {
	t.Parallel()

	requestID := mustID(t, "request.interfaces")
	runID := mustID(t, "run.interfaces")
	discoveryID := mustID(t, "discovery.spec")
	sourceID := mustSourceID(t, "spec")
	snapshotID := mustID(t, "snapshot.spec.001")
	evidenceID := mustID(t, "evidence.spec.types")
	claimID := mustClaimID(t, "interfaces.definition")
	topic, _ := NewResearchTopic("Interfaces", "software", "Go")
	request := ResearchRequest{
		ID: requestID, Topic: topic, Purpose: PurposeConceptDefinition,
		RequestedAt: mustTimestamp(t, 8),
	}
	completedAt := mustTimestamp(t, 12)
	run := ResearchRun{
		ID: runID, RequestID: requestID, Status: ResearchRunCompleted,
		StartedAt: mustTimestamp(t, 9), CompletedAt: &completedAt,
	}
	source := Source{
		ID: sourceID, Kind: SourceSpecification, Locator: mustLocator(t, "spec"),
		Metadata: SourceMetadata{Title: "Language specification"}, CreatedAt: mustTimestamp(t, 8),
	}
	snapshot := SourceSnapshot{
		ID: snapshotID, SourceID: sourceID, Locator: source.Locator, FetchedAt: mustTimestamp(t, 10),
		Fetch: FetchMetadata{
			StatusCode: 200, ContentType: "text/html", ContentHash: "sha256:abc",
			ContentLength: 100, FetchVersion: domainFixtureVersion,
		},
	}

	evidence := Evidence{
		ID: evidenceID, SourceID: sourceID, SnapshotID: snapshotID,
		Location: "§Interface types", Excerpt: "A bounded fixture excerpt.",
		ExcerptHash:      CanonicalEvidenceExcerptHashV1("A bounded fixture excerpt."),
		ContextBefore:    "The specification introduces interface types.",
		ContextAfter:     "Implementations satisfy the type set.",
		ExtractedAt:      mustTimestamp(t, 11),
		ExtractorVersion: domainFixtureVersion,
	}
	if err := evidence.Validate(); err != nil {
		t.Fatalf("Evidence.Validate() error = %v", err)
	}

	claim := Claim{
		ID: claimID, Topic: topic, Statement: "An interface defines a type set.",
		Type: ClaimDefinition, Scope: "language semantics", StatusScope: ClaimStatusStable,
		Confidence: mustConfidence(t, 0.95),
		SourceIDs:  []SourceID{sourceID}, EvidenceIDs: []ID{evidenceID},
		CreatedAt: mustTimestamp(t, 12),
	}
	if err := claim.Validate(); err != nil {
		t.Fatalf("Claim.Validate() error = %v", err)
	}

	provenance := Provenance{
		RequestID: requestID, RunID: runID, DiscoveryID: &discoveryID,
		SourceID: sourceID, SnapshotID: snapshotID, EvidenceID: evidenceID,
		ClaimID: claimID, RecordedAt: mustTimestamp(t, 12), ToolVersion: domainFixtureVersion,
	}
	if err := provenance.Validate(); err != nil {
		t.Fatalf("Provenance.Validate() error = %v", err)
	}
	if err := ValidateProvenanceRelationships(provenance, request, run, source, snapshot, evidence, claim); err != nil {
		t.Fatalf("ValidateProvenanceRelationships() error = %v", err)
	}

	deepLink := DeepLink{Locator: mustLocator(t, "spec#interface-types"), Label: "Interface types"}
	citation := Citation{
		ID: mustID(t, "citation.interfaces"), SourceID: sourceID,
		SnapshotID: snapshotID, EvidenceID: evidenceID, Title: "Language specification",
		Locator: mustLocator(t, "spec"), DeepLink: &deepLink,
		LinkStrategy: CitationSpecification, Section: "§Interface types",
		SnapshotDate: mustTimestamp(t, 10), LastVerified: mustTimestamp(t, 12),
		AlgorithmVersion: CitationAlgorithmV1,
	}
	if err := citation.Validate(); err != nil {
		t.Fatalf("Citation.Validate() error = %v", err)
	}
	if err := ValidateCitationRelationships(citation, source, snapshot, evidence); err != nil {
		t.Fatalf("ValidateCitationRelationships() error = %v", err)
	}

	mismatched := evidence
	mismatched.SnapshotID = mustID(t, "snapshot.other")
	if err := ValidateProvenanceRelationships(provenance, request, run, source, snapshot, mismatched, claim); err == nil {
		t.Fatal("ValidateProvenanceRelationships() accepted mismatched snapshot")
	}
	if err := ValidateCitationRelationships(citation, source, snapshot, mismatched); err == nil {
		t.Fatal("ValidateCitationRelationships() accepted mismatched snapshot")
	}
}

func TestEvidenceAndClaimRejectMissingRelationships(t *testing.T) {
	t.Parallel()

	sourceID := mustSourceID(t, "spec")
	evidence := Evidence{
		ID: mustID(t, "evidence.spec"), SourceID: sourceID,
		Location: "section", Excerpt: "fixture", ExcerptHash: CanonicalEvidenceExcerptHashV1("fixture"),
		ExtractedAt: mustTimestamp(t, 10), ExtractorVersion: domainFixtureVersion,
	}
	if err := evidence.Validate(); err == nil {
		t.Fatal("Evidence.Validate() accepted missing snapshot relationship")
	}

	topic, _ := NewResearchTopic("Interfaces", "software", "Go")
	claim := Claim{
		ID: mustClaimID(t, "interfaces"), Topic: topic, Statement: "A statement",
		Type: ClaimBehavior, Scope: "language semantics", StatusScope: ClaimStatusAll,
		Confidence: mustConfidence(t, 0.8), CreatedAt: mustTimestamp(t, 11),
	}
	if err := claim.Validate(); err == nil {
		t.Fatal("Claim.Validate() accepted no source or evidence")
	}
	claim.SourceIDs = []SourceID{sourceID}
	if err := claim.Validate(); err == nil {
		t.Fatal("Claim.Validate() accepted no evidence")
	}
	claim.EvidenceIDs = []ID{mustID(t, "evidence.spec")}
	claim.SourceIDs = []SourceID{sourceID, sourceID}
	if err := claim.Validate(); err == nil {
		t.Fatal("Claim.Validate() accepted duplicate source relationship")
	}
}

func TestEvidenceEnforcesCopyrightAwareBoundsAndExcerptHash(t *testing.T) {
	t.Parallel()

	excerpt := strings.Repeat("x", MaximumEvidenceExcerptBytes)
	evidence := Evidence{
		ID: mustID(t, "evidence.bounds"), SourceID: mustSourceID(t, "source.bounds"),
		SnapshotID: mustID(t, "snapshot.bounds"), Location: "bounded section",
		Excerpt: excerpt, ExcerptHash: CanonicalEvidenceExcerptHashV1(excerpt),
		ContextBefore: strings.Repeat("b", MaximumEvidenceContextBytes),
		ContextAfter:  strings.Repeat("a", MaximumEvidenceContextBytes),
		ExtractedAt:   mustTimestamp(t, 10), ExtractorVersion: domainFixtureVersion,
	}
	if err := evidence.Validate(); err != nil {
		t.Fatalf("Evidence.Validate() at bounds error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Evidence)
		want   string
	}{
		{name: "excerpt", mutate: func(item *Evidence) {
			item.Excerpt += "x"
			item.ExcerptHash = CanonicalEvidenceExcerptHashV1(item.Excerpt)
		}, want: "evidence excerpt exceeds"},
		{name: "context before", mutate: func(item *Evidence) { item.ContextBefore += "x" }, want: "evidence context before exceeds"},
		{name: "context after", mutate: func(item *Evidence) { item.ContextAfter += "x" }, want: "evidence context after exceeds"},
		{name: "hash format", mutate: func(item *Evidence) { item.ExcerptHash = "sha256:not-a-digest" }, want: "not canonical"},
		{name: "hash mismatch", mutate: func(item *Evidence) {
			item.ExcerptHash = CanonicalEvidenceExcerptHashV1("different excerpt")
		}, want: "does not match"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			candidate := evidence
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestClaimSupportsMultipleEvidenceAndExplicitVersionScope(t *testing.T) {
	t.Parallel()

	topic, _ := NewResearchTopic("HTTP behavior", "software", "Go")
	version := mustVersion(t, "1.24")
	claim := Claim{
		ID: mustClaimID(t, "http.behavior.1.24"), Topic: topic,
		Statement: "The behavior applies to the target version.", Type: ClaimVersionChange,
		Scope: "standard library HTTP client", VersionScope: &version, StatusScope: ClaimStatusStable,
		Confidence: mustConfidence(t, 0.9), SourceIDs: []SourceID{
			mustSourceID(t, "source.docs"), mustSourceID(t, "source.release"),
		}, EvidenceIDs: []ID{
			mustID(t, "evidence.docs"), mustID(t, "evidence.release"),
		}, CreatedAt: mustTimestamp(t, 12),
	}
	if err := claim.Validate(); err != nil {
		t.Fatalf("Claim.Validate() error = %v", err)
	}
	if claim.VersionScope == nil || claim.VersionScope.String() != "1.24" || len(claim.EvidenceIDs) != 2 {
		t.Fatalf("versioned multi-evidence claim = %+v", claim)
	}

	claim.EvidenceIDs[1] = claim.EvidenceIDs[0]
	if err := claim.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate evidence error = %v", err)
	}
}

func TestClaimRejectsInvalidScopeAndStatusScope(t *testing.T) {
	t.Parallel()

	topic, _ := NewResearchTopic("Interfaces", "software", "Go")
	claim := Claim{
		ID: mustClaimID(t, "interfaces.scope"), Topic: topic, Statement: "A statement.",
		Type: ClaimBehavior, Scope: "language semantics", StatusScope: ClaimStatusAll,
		Confidence: mustConfidence(t, 0.8), SourceIDs: []SourceID{mustSourceID(t, "source.spec")},
		EvidenceIDs: []ID{mustID(t, "evidence.spec")}, CreatedAt: mustTimestamp(t, 11),
	}
	claim.Scope = strings.Repeat("s", MaximumClaimScopeBytes+1)
	if err := claim.Validate(); err == nil || !strings.Contains(err.Error(), "claim scope exceeds") {
		t.Fatalf("oversize scope error = %v", err)
	}
	claim.Scope = "language semantics"
	claim.StatusScope = ClaimStatusScope("sometimes")
	if err := claim.Validate(); err == nil || !strings.Contains(err.Error(), "invalid claim status scope") {
		t.Fatalf("invalid status scope error = %v", err)
	}
}

func TestCitationRequiresSourceSnapshotAndEvidenceRelationships(t *testing.T) {
	t.Parallel()

	citation := Citation{
		ID: mustID(t, "citation.invalid"), SourceID: mustSourceID(t, "spec"),
		SnapshotID: mustID(t, "snapshot.spec"), Title: "Specification",
		Locator: mustLocator(t, "spec"), SnapshotDate: mustTimestamp(t, 10),
		LastVerified: mustTimestamp(t, 11),
	}
	if err := citation.Validate(); err == nil {
		t.Fatal("Citation.Validate() accepted missing evidence relationship")
	}
	citation.EvidenceID = mustID(t, "evidence.spec")
	citation.LastVerified = mustTimestamp(t, 9)
	if err := citation.Validate(); err == nil {
		t.Fatal("Citation.Validate() accepted verification before snapshot")
	}
}

func TestSourceBundleRequiresClaimsSourcesAndValidState(t *testing.T) {
	t.Parallel()

	topic, _ := NewResearchTopic("Interfaces", "software", "Go")
	bundle := SourceBundle{
		ID: mustID(t, "bundle.interfaces"), RunID: mustID(t, "run.interfaces"),
		Topic: topic, Purpose: PurposeConceptDefinition,
		ClaimIDs:  []ClaimID{mustClaimID(t, "interfaces.definition")},
		SourceIDs: []SourceID{mustSourceID(t, "spec")}, State: BundleReady,
		VerifiedAt: mustTimestamp(t, 12),
	}
	if err := bundle.Validate(); err != nil {
		t.Fatalf("SourceBundle.Validate() error = %v", err)
	}
	bundle.State = SourceBundleState("pretty")
	if err := bundle.Validate(); err == nil {
		t.Fatal("SourceBundle.Validate() accepted invalid state")
	}
}
