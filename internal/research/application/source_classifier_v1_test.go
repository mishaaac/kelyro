package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestDeterministicSourceClassifierV1UsesFetchedLocatorAndContentType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		locator     string
		contentType string
		want        research.SourceKind
	}{
		{name: "Go specification", locator: "https://go.dev/ref/spec", contentType: "text/html", want: research.SourceSpecification},
		{name: "Go release notes", locator: "https://go.dev/doc/devel/release", contentType: "text/html", want: research.SourceReleaseNotes},
		{name: "Go release", locator: "https://go.dev/doc/go1.27", contentType: "text/html", want: research.SourceReleaseNotes},
		{name: "Go blog", locator: "https://go.dev/blog/go1.27", contentType: "text/html", want: research.SourceOfficialBlog},
		{name: "Go documentation", locator: "https://go.dev/doc/database/cancel-operations", contentType: "text/html", want: research.SourceOfficialDocumentation},
		{name: "package reference", locator: "https://pkg.go.dev/context", contentType: "text/html", want: research.SourcePackageReference},
		{name: "issue tracker", locator: "https://github.com/golang/go/issues/123", contentType: "text/html", want: research.SourceIssueTracker},
		{name: "source code", locator: "https://github.com/golang/go/blob/master/src/context/context.go", contentType: "text/html", want: research.SourceCode},
		{name: "community article", locator: "https://example.test/go-context", contentType: "text/html; charset=utf-8", want: research.SourceCommunityArticle},
		{name: "unsupported binary", locator: "https://example.test/archive.zip", contentType: "application/zip", want: research.SourceOther},
	}
	classifier := application.NewDeterministicSourceClassifierV1()
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := testSource(t, fmt.Sprintf("classifier-%d", index))
			source.Kind = research.SourceOther
			locator, err := research.NewSourceLocator(test.locator)
			if err != nil {
				t.Fatal(err)
			}
			source.Locator = locator
			result, err := classifier.Classify(application.SourceClassificationRequest{
				Source: source,
				Normalized: application.NormalizedSource{
					SourceID: source.ID, Locator: locator, ContentType: test.contentType,
					TextSegments: []string{"Fetched and normalized content."}, NormalizationVersion: "source-normalization-v1",
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Kind != test.want || result.AlgorithmVersion != application.SourceClassifierV1 {
				t.Fatalf("classification = %+v, want kind %q", result, test.want)
			}
		})
	}
}

func TestLiveSourceClassificationStagePersistsKindAndUpdatesArtifacts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	source := testSource(t, "classification-stage")
	source.Kind = research.SourceOther
	locator, err := research.NewSourceLocator("https://pkg.go.dev/context")
	if err != nil {
		t.Fatal(err)
	}
	source.Locator = locator
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	normalized := application.NormalizedSource{
		SourceID: source.ID, Locator: locator, ContentType: "text/html",
		TextSegments: []string{"Package context defines the Context type."}, NormalizationVersion: "source-normalization-v1",
	}
	normalization := staticLiveResearchStage{artifacts: application.LiveResearchArtifacts{
		Sources: []research.Source{source}, NormalizedSources: []application.NormalizedSource{normalized},
	}}
	stage, err := application.NewLiveSourceClassificationStage(
		normalization, application.NewDeterministicSourceClassifierV1(), application.NewSourceService(repositories.Sources, repositories.Snapshots),
	)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := stage.Execute(ctx, application.LiveResearchStageInput{})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := repositories.Sources.Get(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Kind != research.SourcePackageReference || artifacts.Sources[0].Kind != research.SourcePackageReference ||
		len(artifacts.SourceClassifications) != 1 || artifacts.SourceClassifications[0].Kind != research.SourcePackageReference {
		t.Fatalf("stored/artifact classification = %+v / %+v", stored, artifacts)
	}
}

func TestDeterministicSourceClassifierV1PreservesReviewedKind(t *testing.T) {
	t.Parallel()
	source := testSource(t, "classifier-reviewed")
	locator, err := research.NewSourceLocator("https://example.test/article")
	if err != nil {
		t.Fatal(err)
	}
	source.Locator = locator
	result, err := application.NewDeterministicSourceClassifierV1().Classify(application.SourceClassificationRequest{
		Source: source,
		Normalized: application.NormalizedSource{
			SourceID: source.ID, Locator: locator, ContentType: "text/html",
			TextSegments: []string{"Fetched content."}, NormalizationVersion: "source-normalization-v1",
		},
	})
	if err != nil || result.Kind != research.SourceOfficialDocumentation {
		t.Fatalf("reviewed classification = (%+v, %v)", result, err)
	}
}

func TestLiveSourceClassificationStageRejectsRedirectIdentityAsPartialFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	source := testSource(t, "classification-redirect")
	source.Kind = research.SourceOther
	original, _ := research.NewSourceLocator("https://medium.com/example")
	redirected, _ := research.NewSourceLocator("https://example.test/article")
	source.Locator = original
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	normalization := staticLiveResearchStage{artifacts: application.LiveResearchArtifacts{
		Sources: []research.Source{source},
		NormalizedSources: []application.NormalizedSource{{
			SourceID: source.ID, Locator: redirected, ContentType: "text/html",
			TextSegments: []string{"Redirected content."}, NormalizationVersion: "source-normalization-v1",
		}},
	}}
	stage, err := application.NewLiveSourceClassificationStage(
		normalization, application.NewDeterministicSourceClassifierV1(), application.NewSourceService(repositories.Sources, repositories.Snapshots),
	)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := stage.Execute(ctx, application.LiveResearchStageInput{})
	if !errors.Is(err, application.ErrInvalidState) || len(artifacts.NormalizedSources) != 0 ||
		len(artifacts.NormalizationFailures) != 1 || len(artifacts.SourceClassifications) != 0 {
		t.Fatalf("redirect classification = (%+v, %v)", artifacts, err)
	}
}

type staticLiveResearchStage struct {
	artifacts application.LiveResearchArtifacts
}

func (stage staticLiveResearchStage) Execute(context.Context, application.LiveResearchStageInput) (application.LiveResearchArtifacts, error) {
	return stage.artifacts, nil
}
