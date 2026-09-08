package application_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestDeterministicEvidenceExtractorV1SelectsRelevantLiteralContentAcrossSourceStyles(t *testing.T) {
	t.Parallel()
	topic, err := research.NewResearchTopic("Go modules", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		kind       research.SourceKind
		title      string
		headings   []application.NormalizedHeading
		segments   []string
		versions   []string
		purpose    research.ResearchPurpose
		wantCount  bool
		wantSignal application.EvidenceSignal
	}{
		{
			name: "official documentation", kind: research.SourceOfficialDocumentation,
			title: "Go Modules Reference", segments: []string{"Go modules manage dependency versions for a module."},
			purpose: research.PurposeCurrentUsage, wantCount: true, wantSignal: application.EvidenceSignalTopicExact,
		},
		{
			name: "release notes", kind: research.SourceReleaseNotes,
			title: "Go Modules Releases", segments: []string{"Go modules version 2026 was released as stable."}, versions: []string{"2026"},
			purpose: research.PurposeReleaseStatus, wantCount: true, wantSignal: application.EvidenceSignalReleaseFact,
		},
		{
			name: "specification", kind: research.SourceSpecification,
			title: "Language Specification", headings: []application.NormalizedHeading{{Level: 2, Text: "Go modules", Path: []string{"Go modules"}}},
			segments: []string{"A Go modules implementation must preserve the selected module versions."},
			purpose:  research.PurposeConceptDefinition, wantCount: true, wantSignal: application.EvidenceSignalTopicExact,
		},
		{
			name: "community", kind: research.SourceCommunityArticle,
			title: "A practical guide", segments: []string{"Go modules make dependency upgrades reproducible."},
			purpose: research.PurposeProductionPractice, wantCount: true, wantSignal: application.EvidenceSignalTopicExact,
		},
		{
			name: "irrelevant markers", kind: research.SourceCommunityArticle,
			title: "Legacy database mode", segments: []string{"Legacy database mode is deprecated in version 1.0.0."}, versions: []string{"1.0.0"},
			purpose: research.PurposeDeprecationCheck, wantCount: false,
		},
	}
	extractor := application.NewDeterministicEvidenceExtractorV1()
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := testSource(t, "extractor-style-"+string(rune('a'+index)))
			source.Kind = test.kind
			snapshot := testSnapshot(t, source, strings.Repeat(string(rune('a'+index)), 64), 11)
			result, extractErr := extractor.Extract(context.Background(), application.EvidenceExtractionRequest{
				Topic: topic, Purpose: test.purpose, Snapshot: snapshot,
				Source: application.NormalizedSource{
					SourceID: source.ID, Locator: source.Locator, ContentType: "text/plain", Title: test.title,
					Headings: test.headings, TextSegments: test.segments, VersionHints: test.versions,
					NormalizationVersion: "source-normalization-v1",
				},
			})
			if extractErr != nil {
				t.Fatalf("Extract() error = %v", extractErr)
			}
			if test.wantCount != (len(result.Candidates) > 0) {
				t.Fatalf("candidate count = %d, want non-empty %v: %+v", len(result.Candidates), test.wantCount, result.Candidates)
			}
			if test.wantSignal != "" && !evidenceCandidatesContainSignal(result.Candidates, test.wantSignal) {
				t.Fatalf("candidates do not contain signal %q: %+v", test.wantSignal, result.Candidates)
			}
			for _, candidate := range result.Candidates {
				if candidate.ExcerptHash != research.CanonicalEvidenceExcerptHashV1(candidate.Excerpt) || candidate.Score < application.MinimumEvidenceCandidateScore {
					t.Fatalf("invalid admitted candidate: %+v", candidate)
				}
			}
		})
	}
}

func TestDeterministicEvidenceExtractorV1IsStableBoundedAndKeepsFocus(t *testing.T) {
	t.Parallel()
	source := testSource(t, "extractor-bounds")
	snapshot := testSnapshot(t, source, strings.Repeat("b", 64), 11)
	topic, err := research.NewResearchTopic("Go modules", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	request := application.EvidenceExtractionRequest{
		Topic: topic, Purpose: research.PurposeCurrentUsage, Snapshot: snapshot,
		Source: application.NormalizedSource{
			SourceID: source.ID, Locator: source.Locator, ContentType: "text/plain",
			TextSegments:         []string{strings.TrimSpace(strings.Repeat("prefacio ", 350) + "Go modules manage dependencies deterministically. " + strings.Repeat("apéndice ", 350))},
			NormalizationVersion: "source-normalization-v1",
		},
	}
	extractor := application.NewDeterministicEvidenceExtractorV1()
	first, err := extractor.Extract(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := extractor.Extract(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || len(first.Candidates) != 1 {
		t.Fatalf("unstable extraction first=%+v second=%+v", first, second)
	}
	candidate := first.Candidates[0]
	if len(candidate.Excerpt) > application.MaximumEvidenceCandidateExcerptBytes ||
		len(candidate.ContextBefore) > application.MaximumEvidenceCandidateContextBytes ||
		len(candidate.ContextAfter) > application.MaximumEvidenceCandidateContextBytes ||
		!strings.Contains(candidate.Excerpt, "Go modules") {
		t.Fatalf("bounded focused candidate = %+v", candidate)
	}
}

func TestDeterministicEvidenceExtractorV2AdmitsNaturalCLITopicOverlapWithinMatchingDocument(t *testing.T) {
	t.Parallel()
	source := testSource(t, "extractor-v2-natural-topic")
	snapshot := testSnapshot(t, source, strings.Repeat("e", 64), 11)
	topic, err := research.NewResearchTopic("Go context cancellation", "", "")
	if err != nil {
		t.Fatal(err)
	}
	request := application.EvidenceExtractionRequest{
		Topic: topic, Purpose: research.PurposeConceptDefinition, Snapshot: snapshot,
		Source: application.NormalizedSource{
			SourceID: source.ID, Locator: source.Locator, ContentType: "text/html", Title: "Package context",
			TextSegments:         []string{"Package context defines the Context type, which carries deadlines and cancellation signals across API boundaries."},
			NormalizationVersion: "source-normalization-v1",
		},
	}
	result, err := application.NewDeterministicEvidenceExtractorV2().Extract(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) == 0 || result.AlgorithmVersion != application.EvidenceExtractorV2 {
		t.Fatalf("v2 extraction = %+v", result)
	}
	for _, candidate := range result.Candidates {
		if candidate.ExtractorVersion != application.EvidenceExtractorV2 {
			t.Fatalf("candidate version = %q", candidate.ExtractorVersion)
		}
	}

	unrelated := request
	unrelated.Source.TextSegments = []string{"Legacy database mode is deprecated in version 1.0.0."}
	unrelated.Source.Title = "Go context cancellation"
	result, err = application.NewDeterministicEvidenceExtractorV2().Extract(context.Background(), unrelated)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range result.Candidates {
		if candidate.Kind == application.EvidenceCandidatePassage {
			t.Fatalf("unrelated passage admitted: %+v", candidate)
		}
	}
}

func TestLiveEvidenceExtractionPersistsCandidatesIdempotentlyAndPopulatesArtifacts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	source := testSource(t, "live-evidence")
	snapshot := testSnapshot(t, source, strings.Repeat("c", 64), 11)
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Snapshots.Append(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	service, err := application.NewLiveEvidenceExtractionService(
		application.NewDeterministicEvidenceExtractorV1(), repositories.Evidence, fixedClock{now: testTimestamp(t, 12)},
	)
	if err != nil {
		t.Fatal(err)
	}
	topic, err := research.NewResearchTopic("Go modules", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	normalized := application.NormalizedSource{
		SourceID: source.ID, Locator: source.Locator, ContentType: "text/plain", Title: "Go Modules",
		TextSegments: []string{"Go modules manage dependency versions."}, NormalizationVersion: "source-normalization-v1",
	}
	request := application.LiveEvidenceExtractionRequest{
		Topic: topic, Purpose: research.PurposeCurrentUsage, Sources: []research.Source{source},
		Snapshots: []research.SourceSnapshot{snapshot}, NormalizedSources: []application.NormalizedSource{normalized},
	}
	first, err := service.ExtractEvidence(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.ExtractEvidence(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := repositories.Evidence.ListBySnapshot(ctx, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || len(stored) != len(first.Evidence) || len(first.Evidence) == 0 {
		t.Fatalf("idempotent extraction first=%+v second=%+v stored=%+v", first, second, stored)
	}
	requestRecord := research.ResearchRequest{
		ID: testID(t, "request.live-evidence"), Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: testTimestamp(t, 9),
	}
	artifacts, err := service.Execute(ctx, application.LiveResearchStageInput{
		Request: requestRecord,
		Artifacts: application.LiveResearchArtifacts{
			Sources: []research.Source{source}, Snapshots: []research.SourceSnapshot{snapshot}, NormalizedSources: []application.NormalizedSource{normalized},
		},
	})
	if err != nil || len(artifacts.EvidenceCandidates) != len(first.Candidates) || len(artifacts.Evidence) != len(first.Evidence) {
		t.Fatalf("Execute() artifacts=%+v error=%v", artifacts, err)
	}
}

func TestLiveEvidenceExtractionRejectsIrrelevantAndCancelledInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	source := testSource(t, "live-evidence-irrelevant")
	snapshot := testSnapshot(t, source, strings.Repeat("d", 64), 11)
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Snapshots.Append(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	service, err := application.NewLiveEvidenceExtractionService(
		application.NewDeterministicEvidenceExtractorV1(), repositories.Evidence, fixedClock{now: testTimestamp(t, 12)},
	)
	if err != nil {
		t.Fatal(err)
	}
	topic, err := research.NewResearchTopic("Go modules", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	request := application.LiveEvidenceExtractionRequest{
		Topic: topic, Purpose: research.PurposeDeprecationCheck, Sources: []research.Source{source}, Snapshots: []research.SourceSnapshot{snapshot},
		NormalizedSources: []application.NormalizedSource{{
			SourceID: source.ID, Locator: source.Locator, ContentType: "text/plain",
			TextSegments: []string{"Legacy database mode is deprecated in version 1.0.0."}, NormalizationVersion: "source-normalization-v1",
		}},
	}
	if _, err := service.ExtractEvidence(ctx, request); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("irrelevant extraction error = %v", err)
	}
	stored, err := repositories.Evidence.ListBySnapshot(ctx, snapshot.ID)
	if err != nil || len(stored) != 0 {
		t.Fatalf("irrelevant Evidence = (%+v, %v)", stored, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.ExtractEvidence(cancelled, request); !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("cancelled extraction error = %v", err)
	}
}

func evidenceCandidatesContainSignal(candidates []application.EvidenceCandidate, signal application.EvidenceSignal) bool {
	for _, candidate := range candidates {
		for _, value := range candidate.Signals {
			if value == signal {
				return true
			}
		}
	}
	return false
}
