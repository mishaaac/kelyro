package application

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/citation"
)

type citationService struct {
	sources   SourceRepository
	snapshots SnapshotRepository
	evidence  EvidenceRepository
	citations CitationRepository
}

func NewCitationService(sources SourceRepository, snapshots SnapshotRepository, evidence EvidenceRepository, citations CitationRepository) CitationService {
	return &citationService{sources: sources, snapshots: snapshots, evidence: evidence, citations: citations}
}

func (service *citationService) Generate(ctx context.Context, request GenerateCitationRequest) (research.Citation, error) {
	const operation = "generate source citation"
	if err := request.ID.Validate(); err != nil {
		return research.Citation{}, invalid(operation, fmt.Errorf("citation id: %w", err))
	}
	if err := request.SourceID.Validate(); err != nil {
		return research.Citation{}, invalid(operation, err)
	}
	if err := request.SnapshotID.Validate(); err != nil {
		return research.Citation{}, invalid(operation, fmt.Errorf("snapshot id: %w", err))
	}
	if err := request.EvidenceID.Validate(); err != nil {
		return research.Citation{}, invalid(operation, fmt.Errorf("evidence id: %w", err))
	}
	if err := request.LastVerified.Validate(); err != nil {
		return research.Citation{}, invalid(operation, fmt.Errorf("last verified: %w", err))
	}
	for _, item := range []struct {
		name       string
		dependency any
	}{
		{"source repository", service.sources},
		{"snapshot repository", service.snapshots},
		{"evidence repository", service.evidence},
		{"citation repository", service.citations},
	} {
		if err := requireDependency(operation, item.name, item.dependency); err != nil {
			return research.Citation{}, err
		}
	}
	source, err := service.sources.Get(ctx, request.SourceID)
	if err != nil {
		return research.Citation{}, repositoryError(operation, err)
	}
	snapshot, err := service.snapshots.Get(ctx, request.SnapshotID)
	if err != nil {
		return research.Citation{}, repositoryError(operation, err)
	}
	evidence, err := service.evidence.Get(ctx, request.EvidenceID)
	if err != nil {
		return research.Citation{}, repositoryError(operation, err)
	}
	generated, err := citation.GenerateV1(citation.Request{
		ID: request.ID, Source: source, Snapshot: snapshot, Evidence: evidence,
		LastVerified: request.LastVerified, Target: request.Target,
	})
	if err != nil {
		return research.Citation{}, invalid(operation, err)
	}
	if err := service.citations.Append(ctx, generated); err != nil {
		return research.Citation{}, repositoryError(operation, err)
	}
	return generated, nil
}

func (service *citationService) Get(ctx context.Context, id research.ID) (research.Citation, error) {
	const operation = "get source citation"
	if err := id.Validate(); err != nil {
		return research.Citation{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "citation repository", service.citations); err != nil {
		return research.Citation{}, err
	}
	result, err := service.citations.Get(ctx, id)
	return result, repositoryError(operation, err)
}

func (service *citationService) ListForEvidence(ctx context.Context, evidenceID research.ID) ([]research.Citation, error) {
	const operation = "list source citations for evidence"
	if err := evidenceID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	if err := requireDependency(operation, "citation repository", service.citations); err != nil {
		return nil, err
	}
	result, err := service.citations.ListByEvidence(ctx, evidenceID)
	return result, repositoryError(operation, err)
}

var _ CitationService = (*citationService)(nil)
