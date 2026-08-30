package citation_test

import (
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/citation"
)

func TestGenerateV1SelectsStableAnchorStrategyBySourceKind(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		kind     research.SourceKind
		strategy research.CitationLinkStrategy
	}{
		{"documentation anchor", research.SourceOfficialDocumentation, research.CitationURLAnchor},
		{"package symbol", research.SourcePackageReference, research.CitationPackageSymbol},
		{"specification section", research.SourceSpecification, research.CitationSpecification},
		{"standard section", research.SourceStandard, research.CitationSpecification},
		{"release heading", research.SourceReleaseNotes, research.CitationReleaseHeading},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := fixtureRequest(t, test.kind, "https://docs.example.test/reference")
			request.Target = citation.Target{Anchor: "stable-section", Section: "Reference > Stable section"}
			got, err := citation.GenerateV1(request)
			if err != nil {
				t.Fatalf("GenerateV1() error = %v", err)
			}
			if got.LinkStrategy != test.strategy || got.DeepLink == nil ||
				got.DeepLink.Locator.String() != "https://docs.example.test/reference#stable-section" {
				t.Fatalf("citation = %+v, want strategy %q with anchor", got, test.strategy)
			}
			if got.Locator != request.Source.Locator || got.Section != request.Target.Section ||
				got.AlgorithmVersion != research.CitationAlgorithmV1 ||
				got.TemporalScope != research.SourceTemporalCurrent || got.TemporalWarning != "" ||
				got.TemporalAlgorithmVersion != research.SourceTemporalPolicyV1 {
				t.Fatalf("citation metadata = %+v", got)
			}
		})
	}
}

func TestGenerateV1AnnotatesArchivedAndVersionBoundSources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		kind  research.SourceKind
		scope research.SourceTemporalScope
	}{
		{"archived docs", research.SourceOfficialDocumentation, research.SourceTemporalArchived},
		{"old release notes", research.SourceReleaseNotes, research.SourceTemporalVersionBound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := fixtureRequest(t, test.kind, "https://archive.example.test/reference")
			version := mustVersion(t, "1.0")
			request.Source.Version = &version
			request.Source.TemporalScope = test.scope
			got, err := citation.GenerateV1(request)
			if err != nil {
				t.Fatal(err)
			}
			if got.TemporalScope != test.scope || got.TemporalWarning == "" ||
				got.VersionScope == nil || got.VersionScope.String() != "1.0" {
				t.Fatalf("temporal citation = %+v", got)
			}
		})
	}
}

func TestGenerateV1FallsBackToCanonicalLocatorAndLocationHint(t *testing.T) {
	t.Parallel()
	request := fixtureRequest(t, research.SourceOfficialTutorial, "https://example.test/tutorial")
	got, err := citation.GenerateV1(request)
	if err != nil {
		t.Fatalf("GenerateV1() error = %v", err)
	}
	if got.LinkStrategy != research.CitationCanonicalFallback || got.DeepLink != nil {
		t.Fatalf("fallback = %+v", got)
	}
	if got.Locator.String() != "https://example.test/tutorial" || got.Section != request.Evidence.Location {
		t.Fatalf("fallback locator/hint = (%q, %q)", got.Locator.String(), got.Section)
	}
}

func TestGenerateV1AcceptsCommitPinnedSourceCodePermalink(t *testing.T) {
	t.Parallel()
	request := fixtureRequest(t, research.SourceCode, "https://github.com/example/project")
	version := mustVersion(t, "v1.4.0")
	request.Source.Version = &version
	request.Evidence.SourceCode.VersionScope = version
	request.Target = citation.Target{Section: "internal/client/client.go:42-47"}
	got, err := citation.GenerateV1(request)
	if err != nil {
		t.Fatalf("GenerateV1() error = %v", err)
	}
	if got.LinkStrategy != research.CitationSourcePermalink || got.DeepLink == nil ||
		!strings.Contains(got.DeepLink.Locator.String(), "0123456789abcdef") {
		t.Fatalf("source permalink citation = %+v", got)
	}
	if got.VersionScope == nil || got.VersionScope.String() != "v1.4.0" {
		t.Fatalf("version scope = %+v", got.VersionScope)
	}
}

func TestGenerateV1RejectsInvalidURLsAndUnstablePermalinks(t *testing.T) {
	t.Parallel()
	invalidSource := fixtureRequest(t, research.SourceSpecification, "https://example.test/spec")
	invalidSource.Source.Locator = research.SourceLocator{}
	if _, err := citation.GenerateV1(invalidSource); err == nil {
		t.Fatal("GenerateV1() accepted invalid canonical URL")
	}

	request := fixtureRequest(t, research.SourceCode, "https://github.com/example/project")
	request.Evidence.SourceCode.Commit = "main"
	request.Evidence.SourceCode.Permalink = mustLocator(t, "https://github.com/example/project/blob/main/client.go#L9")
	if _, err := citation.GenerateV1(request); err == nil || !strings.Contains(err.Error(), "hexadecimal revision") {
		t.Fatalf("mutable permalink error = %v", err)
	}

	request = fixtureRequest(t, research.SourceCode, "https://github.com/example/project")
	request.Evidence.SourceCode.Permalink = mustLocator(t, "https://evil.example/project/blob/0123456/client.go#L9")
	if _, err := citation.GenerateV1(request); err == nil || !strings.Contains(err.Error(), "host") {
		t.Fatalf("cross-host permalink error = %v", err)
	}

	anchored := fixtureRequest(t, research.SourceOfficialDocumentation, "https://example.test/docs")
	anchored.Target = citation.Target{Anchor: "section", Label: strings.Repeat("x", research.MaximumCitationLabelBytes+1)}
	if _, err := citation.GenerateV1(anchored); err == nil || !strings.Contains(err.Error(), "label exceeds") {
		t.Fatalf("oversize label error = %v", err)
	}
}

func fixtureRequest(t *testing.T, kind research.SourceKind, locatorValue string) citation.Request {
	t.Helper()
	created := mustTimestamp(t, time.Date(2026, time.August, 25, 10, 0, 0, 0, time.UTC))
	fetched := mustTimestamp(t, created.Time().Add(time.Hour))
	extracted := mustTimestamp(t, fetched.Time().Add(time.Hour))
	verified := mustTimestamp(t, extracted.Time().Add(time.Hour))
	sourceID, _ := research.NewSourceID("source.fixture")
	snapshotID, _ := research.NewID("snapshot.fixture")
	evidenceID, _ := research.NewID("evidence.fixture")
	citationID, _ := research.NewID("citation.fixture")
	locator := mustLocator(t, locatorValue)
	excerpt := "A bounded fixture excerpt."
	request := citation.Request{
		ID: citationID,
		Source: research.Source{
			ID: sourceID, Kind: kind, Locator: locator,
			TemporalScope: research.SourceTemporalCurrent,
			Metadata:      research.SourceMetadata{Title: "Fixture reference"}, CreatedAt: created,
		},
		Snapshot: research.SourceSnapshot{
			ID: snapshotID, SourceID: sourceID, Locator: locator, FetchedAt: fetched,
			Fetch: research.FetchMetadata{StatusCode: 200, ContentType: "text/html", ContentHash: "sha256:fixture", ContentLength: 100, FetchVersion: "fetch/v1"},
		},
		Evidence: research.Evidence{
			ID: evidenceID, SourceID: sourceID, SnapshotID: snapshotID,
			Location: "Reference > Fixture", Excerpt: excerpt,
			ExcerptHash: research.CanonicalEvidenceExcerptHashV1(excerpt),
			ExtractedAt: extracted, ExtractorVersion: "extract/v1",
		},
		LastVerified: verified,
	}
	if kind == research.SourceCode {
		version := mustVersion(t, "commit-0123456789abcdef")
		request.Evidence.SourceCode = &research.SourceCodeLocator{
			Repository: locator,
			Permalink:  mustLocator(t, locatorValue+"/blob/0123456789abcdef/internal/client/client.go#L42-L47"),
			Commit:     "0123456789abcdef", Path: "internal/client/client.go", StartLine: 42, EndLine: 47,
			Symbol: "Client.Do", VersionScope: version, AlgorithmVersion: research.SourceCodeEvidenceV1,
		}
	}
	return request
}

func mustLocator(t *testing.T, value string) research.SourceLocator {
	t.Helper()
	locator, err := research.NewSourceLocator(value)
	if err != nil {
		t.Fatal(err)
	}
	return locator
}

func mustTimestamp(t *testing.T, value time.Time) research.Timestamp {
	t.Helper()
	timestamp, err := research.NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}

func mustVersion(t *testing.T, value string) research.SourceVersion {
	t.Helper()
	version, err := research.NewSourceVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}
