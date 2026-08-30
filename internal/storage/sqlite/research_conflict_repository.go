package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

type researchConflictRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchConflictRepository) Append(ctx context.Context, result research.Conflict) error {
	const operation = "append SQLite source conflict"
	if err := result.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	claimIDs, err := encodeJSON(operation, claimIDStrings(result.ClaimIDs))
	if err != nil {
		return err
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	exists, err := recordExists(opCtx, repository.executor, "source_conflicts", "id", result.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if exists {
		return researchConflict(operation)
	}
	for _, claimID := range result.ClaimIDs {
		exists, err = recordExists(opCtx, repository.executor, "claims", "id", claimID.String())
		if err != nil {
			return researchPersistence(operation, err)
		}
		if !exists {
			return researchNotFound(operation)
		}
	}
	var winningClaim, winningSource any
	if result.WinningClaimID != nil {
		winningClaim = result.WinningClaimID.String()
		winningSource = result.WinningSourceID.String()
		var membership int
		err = repository.executor.QueryRowContext(opCtx,
			`SELECT COUNT(*) FROM claim_sources WHERE claim_id=? AND source_id=?`, winningClaim, winningSource,
		).Scan(&membership)
		if err != nil {
			return researchPersistence(operation, err)
		}
		if membership != 1 {
			return researchInvalid(operation, errors.New("winning source is not declared by winning claim"))
		}
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO source_conflicts
(id,conflict_type,claim_ids_json,resolution,unresolved,detected_at,confidence,reason,winning_claim_id,winning_source_id,winning_scope,algorithm_version)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		result.ID.String(), string(result.Type), claimIDs, result.Resolution, result.Unresolved,
		timestampText(result.DetectedAt), result.Confidence.Value(), result.Reason,
		winningClaim, winningSource, result.WinningScope, result.AlgorithmVersion)
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}

func (repository *researchConflictRepository) Get(ctx context.Context, id research.ID) (research.Conflict, error) {
	const operation = "get SQLite source conflict"
	if err := id.Validate(); err != nil {
		return research.Conflict{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.Conflict{}, err
	}
	defer cancel()
	return scanConflict(repository.executor.QueryRowContext(opCtx, conflictSelect+` WHERE id=?`, id.String()), operation)
}

func (repository *researchConflictRepository) ListByClaim(ctx context.Context, claimID research.ClaimID) ([]research.Conflict, error) {
	const operation = "list SQLite source conflicts by claim"
	if err := claimID.Validate(); err != nil {
		return nil, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	rows, err := repository.executor.QueryContext(opCtx, conflictSelect+
		` WHERE EXISTS (SELECT 1 FROM json_each(source_conflicts.claim_ids_json) WHERE value=?) ORDER BY detected_at,id`,
		claimID.String())
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	results := make([]research.Conflict, 0)
	for rows.Next() {
		result, scanErr := scanConflict(rows, operation)
		if scanErr != nil {
			return nil, scanErr
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, researchPersistence(operation, err)
	}
	return results, nil
}

func (repository *researchConflictRepository) ListUnresolved(ctx context.Context) ([]research.Conflict, error) {
	const operation = "list unresolved SQLite source conflicts"
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	rows, err := repository.executor.QueryContext(opCtx, conflictSelect+` WHERE unresolved=1 ORDER BY detected_at,id`)
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	results := make([]research.Conflict, 0)
	for rows.Next() {
		result, scanErr := scanConflict(rows, operation)
		if scanErr != nil {
			return nil, scanErr
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, researchPersistence(operation, err)
	}
	return results, nil
}

const conflictSelect = `SELECT id,conflict_type,claim_ids_json,resolution,unresolved,detected_at,
confidence,reason,winning_claim_id,winning_source_id,winning_scope,algorithm_version FROM source_conflicts`

func scanConflict(row rowScanner, operation string) (research.Conflict, error) {
	var idValue, conflictType, claimIDsJSON, resolution, detectedAtValue, reason, winningScope, algorithm string
	var unresolved bool
	var score float64
	var winningClaimValue, winningSourceValue sql.NullString
	if err := row.Scan(&idValue, &conflictType, &claimIDsJSON, &resolution, &unresolved, &detectedAtValue,
		&score, &reason, &winningClaimValue, &winningSourceValue, &winningScope, &algorithm); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.Conflict{}, researchNotFound(operation)
		}
		return research.Conflict{}, researchPersistence(operation, err)
	}
	id, err := research.NewID(idValue)
	if err != nil {
		return research.Conflict{}, researchPersistence(operation, err)
	}
	var claimValues []string
	if err := decodeJSON(claimIDsJSON, &claimValues); err != nil {
		return research.Conflict{}, researchPersistence(operation, err)
	}
	claimIDs, err := parseClaimIDs(claimValues)
	if err != nil {
		return research.Conflict{}, researchPersistence(operation, err)
	}
	confidence, err := research.NewClaimConfidence(score)
	if err != nil {
		return research.Conflict{}, researchPersistence(operation, err)
	}
	detectedAt, err := scanTimestamp(detectedAtValue)
	if err != nil {
		return research.Conflict{}, researchPersistence(operation, err)
	}
	var winningClaimID *research.ClaimID
	if winningClaimValue.Valid {
		parsed, parseErr := research.NewClaimID(winningClaimValue.String)
		if parseErr != nil {
			return research.Conflict{}, researchPersistence(operation, parseErr)
		}
		winningClaimID = &parsed
	}
	var winningSourceID *research.SourceID
	if winningSourceValue.Valid {
		parsed, parseErr := research.NewSourceID(winningSourceValue.String)
		if parseErr != nil {
			return research.Conflict{}, researchPersistence(operation, parseErr)
		}
		winningSourceID = &parsed
	}
	result := research.Conflict{
		ID: id, Type: research.ConflictType(conflictType), ClaimIDs: claimIDs,
		Resolution: resolution, Confidence: confidence, Reason: reason,
		WinningClaimID: winningClaimID, WinningSourceID: winningSourceID, WinningScope: winningScope,
		Unresolved: unresolved, DetectedAt: detectedAt, AlgorithmVersion: algorithm,
	}
	if err := result.Validate(); err != nil {
		return research.Conflict{}, researchPersistence(operation, err)
	}
	return result, nil
}
