package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestServiceAssemblesDeterministicEvidenceStageWithWorkspaceRepository(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	memoryStore := memory.New()
	repositories := memoryStore.Repositories()
	source := appSource(t)
	fetchedAt, err := research.NewTimestamp(time.Date(2026, 8, 31, 15, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	snapshotID, err := research.NewID("snapshot.app-evidence")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := research.SourceSnapshot{
		ID: snapshotID, SourceID: source.ID, Locator: source.Locator, FetchedAt: fetchedAt,
		Fetch: research.FetchMetadata{
			StatusCode: 200, ContentType: "text/plain", ContentHash: "sha256:" + strings.Repeat("e", 64),
			ContentLength: 40, FetchVersion: "fixture-v1",
		},
	}
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Snapshots.Append(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	store := &fakeSourceRegistryStore{
		evidence: repositories.Evidence, claims: repositories.Claims, citations: repositories.Citations,
		trustRepository: repositories.TrustRegistry, freshness: researchapp.NewFreshnessService(repositories.Freshness),
		releases: repositories.Releases, deprecations: repositories.Deprecations,
		registry: researchapp.NewSourceRegistryService(repositories.SourceRegistry), close: func() {},
	}
	service := NewService(nil, nil).WithResearchClock(func() time.Time { return fetchedAt.Time().Add(time.Hour) })
	stage, err := service.researchEvidenceForRun(store)
	if err != nil {
		t.Fatal(err)
	}
	topic, err := research.NewResearchTopic("Go modules", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	request := research.ResearchRequest{
		ID: mustAppResearchID(t, "request.app-evidence"), Topic: topic,
		Purpose: research.PurposeCurrentUsage, RequestedAt: fetchedAt,
	}
	artifacts, err := stage.Execute(ctx, researchapp.LiveResearchStageInput{
		Request: request,
		Artifacts: researchapp.LiveResearchArtifacts{
			Sources: []research.Source{source}, Snapshots: []research.SourceSnapshot{snapshot},
			NormalizedSources: []researchapp.NormalizedSource{{
				SourceID: source.ID, Locator: source.Locator, ContentType: "text/plain",
				TextSegments: []string{"Go modules are organized so a Go module is a dependency management system."}, NormalizationVersion: "source-normalization-v1",
			}},
		},
	})
	if err != nil || len(artifacts.Evidence) == 0 || artifacts.Evidence[0].ExtractorVersion != researchapp.EvidenceExtractorV2 ||
		len(artifacts.Claims) != 1 || len(artifacts.Citations) != 1 || len(artifacts.TrustDecisions) != 1 ||
		len(artifacts.FreshnessRecords) != 1 || len(artifacts.TemporalObservations) != 1 {
		t.Fatalf("evidence stage artifacts=%+v error=%v", artifacts, err)
	}
}

func mustAppResearchID(t *testing.T, value string) research.ID {
	t.Helper()
	id, err := research.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
