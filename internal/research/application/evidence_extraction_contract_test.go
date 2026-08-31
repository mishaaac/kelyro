package application_test

import (
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestEvidenceCandidateContractIsBoundedVersionedAndSnapshotScoped(t *testing.T) {
	t.Parallel()
	source := testSource(t, "evidence-candidate")
	candidate := application.EvidenceCandidate{
		SourceID: source.ID, SnapshotID: testID(t, "snapshot.evidence-candidate"),
		Kind: application.EvidenceCandidatePassage, Location: "text[0001]",
		Excerpt: "Version 1.2.0 is available.", ContextBefore: "Release notes.",
		Score: 75, Signals: []application.EvidenceSignal{
			application.EvidenceSignalTargetVersion, application.EvidenceSignalVersionFact,
		},
		ExtractorVersion: application.EvidenceExtractorV1,
	}
	candidate.ExcerptHash = research.CanonicalEvidenceExcerptHashV1(candidate.Excerpt)
	if err := candidate.Validate(); err != nil {
		t.Fatalf("EvidenceCandidate.Validate() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*application.EvidenceCandidate)
	}{
		{name: "unadmitted score", mutate: func(item *application.EvidenceCandidate) { item.Score = application.MinimumEvidenceCandidateScore - 1 }},
		{name: "hash mismatch", mutate: func(item *application.EvidenceCandidate) { item.Excerpt = "changed" }},
		{name: "duplicate signal", mutate: func(item *application.EvidenceCandidate) { item.Signals = append(item.Signals, item.Signals[0]) }},
		{name: "oversize excerpt", mutate: func(item *application.EvidenceCandidate) {
			item.Excerpt = strings.Repeat("x", application.MaximumEvidenceCandidateExcerptBytes+1)
			item.ExcerptHash = research.CanonicalEvidenceExcerptHashV1(item.Excerpt)
		}},
		{name: "wrong version", mutate: func(item *application.EvidenceCandidate) { item.ExtractorVersion = "future" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := candidate
			item.Signals = append([]application.EvidenceSignal(nil), candidate.Signals...)
			test.mutate(&item)
			if err := item.Validate(); err == nil {
				t.Fatal("EvidenceCandidate.Validate() accepted invalid candidate")
			}
		})
	}
}

func TestEvidenceExtractionRequestRequiresExactNormalizedSnapshotIdentity(t *testing.T) {
	t.Parallel()
	source := testSource(t, "evidence-request")
	snapshot := testSnapshot(t, source, strings.Repeat("a", 64), 11)
	topic, err := research.NewResearchTopic("modules", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	request := application.EvidenceExtractionRequest{
		Topic: topic, Purpose: research.PurposeCurrentUsage, Snapshot: snapshot,
		Source: application.NormalizedSource{
			SourceID: source.ID, Locator: source.Locator, ContentType: "text/plain",
			TextSegments: []string{"Modules manage dependencies."}, NormalizationVersion: "source-normalization-v1",
		},
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("EvidenceExtractionRequest.Validate() error = %v", err)
	}
	request.Source.Locator = testSource(t, "other-locator").Locator
	if err := request.Validate(); err == nil {
		t.Fatal("request accepted a normalized source from another snapshot locator")
	}
}
