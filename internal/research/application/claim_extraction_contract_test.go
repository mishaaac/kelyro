package application_test

import (
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestClaimCandidateContractRequiresLiteralEvidenceIdentityAndClosedFamily(t *testing.T) {
	t.Parallel()
	confidence, err := research.NewClaimConfidence(0.9)
	if err != nil {
		t.Fatal(err)
	}
	version := testVersion(t, "2026")
	statement := "Version 2026 was released as stable."
	candidate := application.ClaimCandidate{
		SourceID: testSourceID(t, "claim-candidate"), SnapshotID: testID(t, "snapshot.claim-candidate"),
		EvidenceID: testID(t, "evidence.claim-candidate"), Family: application.ClaimFamilyVersionReleaseFact,
		Statement: statement, StatementHash: application.CanonicalClaimCandidateStatementHashV1(statement),
		Scope: "release lifecycle", VersionScope: &version, StatusScope: research.ClaimStatusStable,
		Confidence: confidence, ExplicitMarker: "released", SentenceIndex: 0,
		ExtractorVersion: application.ClaimExtractorV1,
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("ClaimCandidate.Validate() error = %v", err)
	}
	if candidate.Family.ClaimType() != research.ClaimVersionChange {
		t.Fatalf("ClaimType() = %q", candidate.Family.ClaimType())
	}

	tests := []struct {
		name   string
		mutate func(*application.ClaimCandidate)
	}{
		{name: "hash mismatch", mutate: func(item *application.ClaimCandidate) { item.Statement = "changed" }},
		{name: "unknown family", mutate: func(item *application.ClaimCandidate) { item.Family = "future" }},
		{name: "missing evidence", mutate: func(item *application.ClaimCandidate) { item.EvidenceID = research.ID{} }},
		{name: "oversize statement", mutate: func(item *application.ClaimCandidate) {
			item.Statement = strings.Repeat("x", application.MaximumClaimCandidateStatementBytes+1)
			item.StatementHash = application.CanonicalClaimCandidateStatementHashV1(item.Statement)
		}},
		{name: "missing marker", mutate: func(item *application.ClaimCandidate) { item.ExplicitMarker = "" }},
		{name: "negative sentence", mutate: func(item *application.ClaimCandidate) { item.SentenceIndex = -1 }},
		{name: "wrong version", mutate: func(item *application.ClaimCandidate) { item.ExtractorVersion = "future" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := candidate
			test.mutate(&item)
			if err := item.Validate(); err == nil {
				t.Fatal("ClaimCandidate.Validate() accepted invalid candidate")
			}
		})
	}
}

func TestClaimFamilyMapsOnlyTheSixV1FamiliesToPublishedClaimTypes(t *testing.T) {
	t.Parallel()
	want := map[application.ClaimFamily]research.ClaimType{
		application.ClaimFamilyExplicitDefinition:     research.ClaimDefinition,
		application.ClaimFamilyVersionReleaseFact:     research.ClaimVersionChange,
		application.ClaimFamilyDeprecationStatement:   research.ClaimDeprecation,
		application.ClaimFamilyAvailabilitySupport:    research.ClaimCompatibility,
		application.ClaimFamilyExplicitRequirement:    research.ClaimRequirement,
		application.ClaimFamilyExplicitRecommendation: research.ClaimRecommendation,
	}
	for family, claimType := range want {
		if err := family.Validate(); err != nil || family.ClaimType() != claimType {
			t.Fatalf("family %q = (%q, %v), want %q", family, family.ClaimType(), err, claimType)
		}
	}
	if got := application.ClaimFamily("future").ClaimType(); got != "" {
		t.Fatalf("unknown family maps to %q", got)
	}
}

func TestClaimExtractionRequestAcceptsOnlyPersistableEvidence(t *testing.T) {
	t.Parallel()
	topic, err := research.NewResearchTopic("Go modules", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	excerpt := "Go modules manage dependency versions."
	evidence := research.Evidence{
		ID: testID(t, "evidence.claim-request"), SourceID: testSourceID(t, "claim-request"),
		SnapshotID: testID(t, "snapshot.claim-request"), Location: "text[0000]",
		Excerpt: excerpt, ExcerptHash: research.CanonicalEvidenceExcerptHashV1(excerpt),
		ExtractedAt: testTimestamp(t, 12), ExtractorVersion: application.EvidenceExtractorV1,
	}
	request := application.ClaimExtractionRequest{
		Topic: topic, Purpose: research.PurposeCurrentUsage, Evidence: evidence,
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("ClaimExtractionRequest.Validate() error = %v", err)
	}
	request.Evidence.Excerpt = "changed without matching hash"
	if err := request.Validate(); err == nil {
		t.Fatal("claim extraction request accepted invalid Evidence")
	}
}
