package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

type researchProvenanceRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchProvenanceRepository) Append(ctx context.Context, graph research.ProvenanceGraph) error {
	const operation = "append SQLite provenance graph"
	if err := graph.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	encoded, err := graph.ExportJSON()
	if err != nil {
		return researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	exists, err := recordExists(opCtx, repository.executor, "provenance_graphs", "id", graph.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if exists {
		return researchConflict(operation)
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO provenance_graphs (id,claim_id,graph_json,recorded_at,algorithm_version) VALUES (?,?,?,?,?)`,
		graph.ID.String(), graph.ClaimID.String(), string(encoded), timestampText(graph.RecordedAt), graph.AlgorithmVersion)
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}

func (repository *researchProvenanceRepository) LatestByClaim(ctx context.Context, claimID research.ClaimID) (research.ProvenanceGraph, error) {
	const operation = "get latest SQLite provenance graph"
	if err := claimID.Validate(); err != nil {
		return research.ProvenanceGraph{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.ProvenanceGraph{}, err
	}
	defer cancel()
	var idValue, claimValue, encoded, recordedValue, algorithm string
	err = repository.executor.QueryRowContext(opCtx, `SELECT id,claim_id,graph_json,recorded_at,algorithm_version FROM provenance_graphs WHERE claim_id=? ORDER BY recorded_at DESC,id DESC LIMIT 1`, claimID.String()).Scan(
		&idValue, &claimValue, &encoded, &recordedValue, &algorithm,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return research.ProvenanceGraph{}, researchNotFound(operation)
	}
	if err != nil {
		return research.ProvenanceGraph{}, researchPersistence(operation, err)
	}
	graph, err := research.ParseProvenanceGraphJSON([]byte(encoded))
	if err != nil {
		return research.ProvenanceGraph{}, researchPersistence(operation, fmt.Errorf("invalid stored provenance graph: %w", err))
	}
	recordedAt, err := scanTimestamp(recordedValue)
	if err != nil {
		return research.ProvenanceGraph{}, researchPersistence(operation, err)
	}
	if graph.ID.String() != idValue || graph.ClaimID.String() != claimValue || graph.AlgorithmVersion != algorithm || !graph.RecordedAt.Time().Equal(recordedAt.Time()) {
		return research.ProvenanceGraph{}, researchPersistence(operation, errors.New("stored provenance metadata does not match graph export"))
	}
	return graph, nil
}
