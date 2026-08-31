package app

import (
	"context"
	"testing"

	"github.com/mishaaac/kelyro/internal/infra/researchnormalize"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func TestServiceAssemblesExistingResearchNormalizerStage(t *testing.T) {
	t.Parallel()
	service := NewService(nil, nil).WithResearchNormalizer(researchnormalize.New())
	stage, err := service.researchNormalizationForRun()
	if err != nil {
		t.Fatal(err)
	}
	source := appResearchFetchSource(t, "source.app-normalization")
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
		Snapshots: []research.SourceSnapshot{snapshot}, NormalizationInputs: []researchapp.FetchedSource{fetched},
	}})
	if err != nil || len(artifacts.NormalizedSources) != 1 ||
		artifacts.NormalizedSources[0].NormalizationVersion != researchnormalize.Version || len(artifacts.NormalizationFailures) != 0 {
		t.Fatalf("production normalization stage = (%+v,%v)", artifacts, err)
	}
}
