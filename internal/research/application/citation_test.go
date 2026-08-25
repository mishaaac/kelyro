package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	"github.com/mishaaac/kelyro/internal/research/citation"
)

func TestCitationServiceGeneratesAndPersistsTraceableCitation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	source := testSource(t, "citation")
	source.Kind = research.SourceSpecification
	snapshot := testSnapshot(t, source, "citation", 10)
	excerpt := "A bounded citation excerpt."
	evidence := research.Evidence{
		ID: testID(t, "evidence.citation"), SourceID: source.ID, SnapshotID: snapshot.ID,
		Location: "Specification > Interfaces", Excerpt: excerpt,
		ExcerptHash: research.CanonicalEvidenceExcerptHashV1(excerpt),
		ExtractedAt: testTimestamp(t, 11), ExtractorVersion: fixtureVersion,
	}
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Snapshots.Append(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	service := application.NewCitationService(repositories.Sources, repositories.Snapshots, repositories.Evidence, repositories.Citations)
	request := application.GenerateCitationRequest{
		ID: testID(t, "citation.spec"), SourceID: source.ID, SnapshotID: snapshot.ID,
		EvidenceID: evidence.ID, LastVerified: testTimestamp(t, 12),
		Target: citation.Target{Anchor: "interfaces", Section: evidence.Location},
	}
	generated, err := service.Generate(ctx, request)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if generated.LinkStrategy != research.CitationSpecification || generated.DeepLink == nil {
		t.Fatalf("generated citation = %+v", generated)
	}
	loaded, err := service.Get(ctx, generated.ID)
	if err != nil || loaded.DeepLink == nil || loaded.DeepLink.Locator != generated.DeepLink.Locator {
		t.Fatalf("Get() = (%+v, %v)", loaded, err)
	}
	listed, err := service.ListForEvidence(ctx, evidence.ID)
	if err != nil || len(listed) != 1 || listed[0].ID != generated.ID {
		t.Fatalf("ListForEvidence() = (%+v, %v)", listed, err)
	}
	if _, err := service.Generate(ctx, request); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate Generate() error = %v, want conflict", err)
	}
}

func TestCitationServiceClassifiesMissingRelationshipsAndDependencies(t *testing.T) {
	t.Parallel()
	repositories := memory.New().Repositories()
	service := application.NewCitationService(repositories.Sources, repositories.Snapshots, repositories.Evidence, repositories.Citations)
	request := application.GenerateCitationRequest{
		ID: testID(t, "citation.missing"), SourceID: testSourceID(t, "missing"),
		SnapshotID: testID(t, "snapshot.missing"), EvidenceID: testID(t, "evidence.missing"),
		LastVerified: testTimestamp(t, 12),
	}
	if _, err := service.Generate(context.Background(), request); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("missing source error = %v, want not_found", err)
	}
	missing := application.NewCitationService(nil, nil, nil, nil)
	if _, err := missing.Generate(context.Background(), request); !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("missing dependency error = %v, want unavailable", err)
	}
}
