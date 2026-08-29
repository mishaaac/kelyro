package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func (repository *researchRunRepository) AppendAudit(ctx context.Context, audit research.ResearchRunAudit) error {
	const operation = "append SQLite research run audit"
	if err := audit.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	payload, err := audit.ExportJSON()
	if err != nil {
		return researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	var status, started, technology string
	var completed, targetVersion sql.NullString
	err = repository.executor.QueryRowContext(opCtx, `SELECT r.status,r.started_at,r.completed_at,t.technology,t.target_version FROM research_runs r JOIN research_topics t ON t.request_id=r.request_id WHERE r.id=?`, audit.RunID.String()).Scan(&status, &started, &completed, &technology, &targetVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return researchNotFound(operation)
	}
	if err != nil {
		return researchPersistence(operation, err)
	}
	startedAt, err := scanTimestamp(started)
	if err != nil {
		return researchPersistence(operation, err)
	}
	completedAt, err := scanOptionalTimestamp(completed)
	if err != nil {
		return researchPersistence(operation, err)
	}
	target, err := scanOptionalVersion(targetVersion)
	if err != nil {
		return researchPersistence(operation, err)
	}
	if status != string(audit.Outcome) || !startedAt.Time().Equal(audit.StartedAt.Time()) || !equalSQLiteAuditTimestamp(completedAt, audit.CompletedAt) {
		return researchInvalid(operation, errors.New("audit lifecycle does not match durable research run"))
	}
	if technology != audit.TargetTechnology || !equalSQLiteAuditVersion(target, audit.TargetVersion) {
		return researchInvalid(operation, errors.New("audit target does not match durable research request"))
	}
	for _, item := range audit.Sources {
		var locator, contentHash string
		err := repository.executor.QueryRowContext(opCtx, `SELECT locator,content_hash FROM source_snapshots WHERE id=? AND source_id=?`, item.SnapshotID.String(), item.SourceID.String()).Scan(&locator, &contentHash)
		if errors.Is(err, sql.ErrNoRows) {
			return researchNotFound(operation)
		}
		if err != nil {
			return researchPersistence(operation, err)
		}
		if locator != item.Locator.String() || contentHash != item.SnapshotHash {
			return researchInvalid(operation, errors.New("audit source snapshot metadata does not match durable research data"))
		}
	}
	idExists, err := recordExists(opCtx, repository.executor, "research_run_audit", "id", audit.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if idExists {
		return researchConflict(operation)
	}
	var timestampExists int
	if err := repository.executor.QueryRowContext(opCtx, `SELECT COUNT(*) FROM research_run_audit WHERE run_id=? AND recorded_at=?`, audit.RunID.String(), timestampText(audit.RecordedAt)).Scan(&timestampExists); err != nil {
		return researchPersistence(operation, err)
	}
	if timestampExists != 0 {
		return researchConflict(operation)
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO research_run_audit (id,run_id,recorded_at,outcome,content_hash,metadata_json,algorithm_version) VALUES (?,?,?,?,?,?,?)`,
		audit.ID.String(), audit.RunID.String(), timestampText(audit.RecordedAt), string(audit.Outcome), audit.ContentHash, string(payload), audit.AlgorithmVersion)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return researchConflict(operation)
		}
		return researchPersistence(operation, err)
	}
	return nil
}

func equalSQLiteAuditTimestamp(left, right *research.Timestamp) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Time().Equal(right.Time())
}

func equalSQLiteAuditVersion(left, right *research.SourceVersion) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func (repository *researchRunRepository) ListAudit(ctx context.Context, runID research.ID) ([]research.ResearchRunAudit, error) {
	const operation = "list SQLite research run audit"
	if err := runID.Validate(); err != nil {
		return nil, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	exists, err := recordExists(opCtx, repository.executor, "research_runs", "id", runID.String())
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	if !exists {
		return nil, researchNotFound(operation)
	}
	rows, err := repository.executor.QueryContext(opCtx, `SELECT metadata_json FROM research_run_audit WHERE run_id=? ORDER BY recorded_at,id`, runID.String())
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	result := make([]research.ResearchRunAudit, 0)
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, researchPersistence(operation, err)
		}
		audit, err := research.ParseResearchRunAuditJSON([]byte(payload))
		if err != nil {
			return nil, researchPersistence(operation, err)
		}
		if audit.RunID != runID {
			return nil, researchPersistence(operation, errors.New("research audit run identity does not match index"))
		}
		result = append(result, audit)
	}
	if err := rows.Err(); err != nil {
		return nil, researchPersistence(operation, err)
	}
	return result, nil
}

var _ application.ResearchRunRepository = (*researchRunRepository)(nil)
