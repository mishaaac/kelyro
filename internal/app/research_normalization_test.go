package app

import (
	"context"
	"testing"

	"github.com/mishaaac/kelyro/internal/infra/researchnormalize"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestServiceAssemblesExistingResearchNormalizerStage(t *testing.T) {
	t.Parallel()
	service := NewService(nil, nil).WithResearchNormalizer(researchnormalize.New())
	repositories := memory.New().Repositories()
	sources := researchapp.NewSourceService(repositories.Sources, repositories.Snapshots)
	stage, err := service.researchNormalizationForRun(&fakeSourceRegistryStore{sources: sources})
	if err != nil {
		t.Fatal(err)
	}
	source := appResearchFetchSource(t, "source.app-normalization")
	source.Kind = research.SourceOther
	if err := sources.Register(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	body := []byte("normalized through production composition")
	fetched := researchapp.FetchedSource{
		SourceID: source.ID, Locator: source.Locator, FetchedAt: source.CreatedAt, Body: body, Origin: researchapp.FetchOriginLive,
		Metadata: research.FetchMetadata{StatusCode: 200, ContentType: "text/plain", ContentHash: research.CanonicalContentHashV1(body),
			ContentLength: int64(len(body)), FetchVersion: "fixture-fetch-v1"},
	}
	snapshotID, _ := research.NewID("snapshot.app-normalization")
	snapshot := research.SourceSnapshot{
		ID: snapshotID, SourceID: source.ID, Locator: source.Locator, FetchedAt: fetched.FetchedAt, Fetch: fetched.Metadata,
	}
	artifacts, err := stage.Execute(context.Background(), researchapp.LiveResearchStageInput{Artifacts: researchapp.LiveResearchArtifacts{
		Sources: []research.Source{source}, Snapshots: []research.SourceSnapshot{snapshot}, NormalizationInputs: []researchapp.FetchedSource{fetched},
	}})
	if err != nil || len(artifacts.NormalizedSources) != 1 ||
		artifacts.NormalizedSources[0].NormalizationVersion != researchnormalize.Version || len(artifacts.NormalizationFailures) != 0 ||
		len(artifacts.SourceClassifications) != 1 || artifacts.Sources[0].Kind != research.SourceCommunityArticle {
		t.Fatalf("production normalization stage = (%+v,%v)", artifacts, err)
	}
}
