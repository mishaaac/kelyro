package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

type researchFinalizationRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchFinalizationRepository) Finalize(ctx context.Context, finalization application.ResearchFinalization) (application.ResearchFinalizationResult, error) {
	const operation = "finalize SQLite research run and queue"
	if err := finalization.Validate(); err != nil {
		return application.ResearchFinalizationResult{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return application.ResearchFinalizationResult{}, err
	}
	defer cancel()
	target := repository.executor
	var transaction *sql.Tx
	if starter, ok := repository.executor.(researchTransactionStarter); ok {
		transaction, err = starter.BeginTx(opCtx, nil)
		if err != nil {
			return application.ResearchFinalizationResult{}, researchPersistence(operation, err)
		}
		target = transaction
	}
	rollback := func(cause error) (application.ResearchFinalizationResult, error) {
		if transaction != nil {
			if rollbackErr := transaction.Rollback(); rollbackErr != nil {
				cause = errors.Join(cause, researchPersistence(operation, fmt.Errorf("rollback finalization: %w", rollbackErr)))
			}
		}
		return application.ResearchFinalizationResult{}, cause
	}
	run, err := scanResearchRun(target.QueryRowContext(opCtx, `SELECT id,request_id,status,started_at,completed_at FROM research_runs WHERE id=?`, finalization.RunID.String()), operation)
	if err != nil {
		return rollback(err)
	}
	var queueRequestID, queueStatus, executionRunID, executionStatus string
	var attempts int
	err = target.QueryRowContext(opCtx, `SELECT request_id,status,execution_run_id,execution_status,execution_attempts FROM research_trigger_queue WHERE id=?`, finalization.QueueItemID.String()).Scan(
		&queueRequestID, &queueStatus, &executionRunID, &executionStatus, &attempts,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return rollback(researchNotFound(operation))
	}
	if err != nil {
		return rollback(researchPersistence(operation, err))
	}
	if run.RequestID.String() != queueRequestID || queueStatus != string(research.ResearchQueueQueued) ||
		executionRunID != run.ID.String() || executionStatus != string(application.ResearchQueueExecutionClaimed) || attempts != finalization.Attempts {
		return rollback(researchConflict(operation))
	}
	if finalization.BundleID != nil {
		var bundleRunID string
		err = target.QueryRowContext(opCtx, `SELECT run_id FROM source_bundles WHERE id=?`, finalization.BundleID.String()).Scan(&bundleRunID)
		if errors.Is(err, sql.ErrNoRows) {
			return rollback(researchNotFound(operation))
		}
		if err != nil {
			return rollback(researchPersistence(operation, err))
		}
		if bundleRunID != run.ID.String() {
			return rollback(researchInvalid(operation, errors.New("research finalization bundle belongs to another run")))
		}
	}
	terminal, err := research.TransitionResearchRunV1(run, finalization.RunStatus, finalization.FinalizedAt)
	if err != nil {
		return rollback(researchInvalid(operation, err))
	}
	result, err := target.ExecContext(opCtx, `UPDATE research_runs SET status=?,completed_at=? WHERE id=? AND status=?`,
		string(terminal.Status), timestampText(*terminal.CompletedAt), terminal.ID.String(), string(run.Status))
	if err != nil {
		return rollback(researchPersistence(operation, err))
	}
	if err := requireAffected(result); err != nil {
		return rollback(researchConflict(operation))
	}
	settledQueueStatus := research.ResearchQueueDispatched
	var statusChanged any = timestampText(finalization.FinalizedAt)
	if finalization.ExecutionStatus == application.ResearchQueueExecutionRetry {
		settledQueueStatus, statusChanged = research.ResearchQueueQueued, nil
	} else if finalization.ExecutionStatus == application.ResearchQueueExecutionCancelled {
		settledQueueStatus = research.ResearchQueueCancelled
	}
	var failureKind, bundleID any
	if finalization.FailureKind != "" {
		failureKind = string(finalization.FailureKind)
	}
	if finalization.BundleID != nil {
		bundleID = finalization.BundleID.String()
	}
	result, err = target.ExecContext(opCtx, `UPDATE research_trigger_queue
SET status=?,status_changed_at=?,execution_status=?,execution_changed_at=?,execution_failure_kind=?,execution_bundle_id=?
WHERE id=? AND status='queued' AND execution_status='claimed' AND execution_run_id=? AND execution_attempts=?`,
		string(settledQueueStatus), statusChanged, string(finalization.ExecutionStatus), timestampText(finalization.FinalizedAt), failureKind, bundleID,
		finalization.QueueItemID.String(), finalization.RunID.String(), finalization.Attempts)
	if err != nil {
		return rollback(researchPersistence(operation, err))
	}
	if err := requireAffected(result); err != nil {
		return rollback(researchConflict(operation))
	}
	if finalization.Audit != nil {
		if err := appendTerminalAuditSQLite(opCtx, target, terminal, *finalization.Audit); err != nil {
			return rollback(err)
		}
	}
	if transaction != nil {
		if err := transaction.Commit(); err != nil {
			return application.ResearchFinalizationResult{}, researchPersistence(operation, err)
		}
	}
	execution := application.ResearchQueueExecution{
		QueueItemID: finalization.QueueItemID, RunID: finalization.RunID, Status: finalization.ExecutionStatus,
		Attempts: finalization.Attempts, ChangedAt: finalization.FinalizedAt, FailureKind: finalization.FailureKind,
		BundleID: cloneSQLiteID(finalization.BundleID), AlgorithmVersion: application.ResearchQueueWorkerV1,
	}
	if finalization.Cost != nil {
		cost := *finalization.Cost
		terminal.Cost = &cost
	}
	resultValue := application.ResearchFinalizationResult{Run: terminal, Execution: execution}
	if finalization.Audit != nil {
		audit := *finalization.Audit
		resultValue.Audit = &audit
	}
	if err := resultValue.Validate(); err != nil {
		return application.ResearchFinalizationResult{}, researchPersistence(operation, err)
	}
	return resultValue, nil
}

func appendTerminalAuditSQLite(ctx context.Context, target executor, terminal research.ResearchRun, audit research.ResearchRunAudit) error {
	const operation = "append terminal SQLite research audit"
	if audit.RunID != terminal.ID || audit.Outcome != terminal.Status || !audit.StartedAt.Time().Equal(terminal.StartedAt.Time()) ||
		!equalSQLiteAuditTimestamp(audit.CompletedAt, terminal.CompletedAt) {
		return researchInvalid(operation, errors.New("terminal audit lifecycle does not match Research Run"))
	}
	var technology string
	var targetVersion sql.NullString
	if err := target.QueryRowContext(ctx, `SELECT technology,target_version FROM research_topics WHERE request_id=?`, terminal.RequestID.String()).Scan(&technology, &targetVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return researchNotFound(operation)
		}
		return researchPersistence(operation, err)
	}
	version, err := scanOptionalVersion(targetVersion)
	if err != nil {
		return researchPersistence(operation, err)
	}
	if technology != audit.TargetTechnology || !equalSQLiteAuditVersion(version, audit.TargetVersion) {
		return researchInvalid(operation, errors.New("terminal audit target does not match Research Request"))
	}
	for _, item := range audit.Sources {
		var locator, contentHash string
		err := target.QueryRowContext(ctx, `SELECT locator,content_hash FROM source_snapshots WHERE id=? AND source_id=?`, item.SnapshotID.String(), item.SourceID.String()).Scan(&locator, &contentHash)
		if errors.Is(err, sql.ErrNoRows) {
			return researchNotFound(operation)
		}
		if err != nil {
			return researchPersistence(operation, err)
		}
		if locator != item.Locator.String() || contentHash != item.SnapshotHash {
			return researchInvalid(operation, errors.New("terminal audit snapshot does not match durable data"))
		}
	}
	payload, err := audit.ExportJSON()
	if err != nil {
		return researchInvalid(operation, err)
	}
	_, err = target.ExecContext(ctx, `INSERT INTO research_run_audit (id,run_id,recorded_at,outcome,content_hash,metadata_json,algorithm_version) VALUES (?,?,?,?,?,?,?)`,
		audit.ID.String(), audit.RunID.String(), timestampText(audit.RecordedAt), string(audit.Outcome), audit.ContentHash, string(payload), audit.AlgorithmVersion)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return researchConflict(operation)
		}
		return researchPersistence(operation, err)
	}
	return nil
}

func cloneSQLiteID(id *research.ID) *research.ID {
	if id == nil {
		return nil
	}
	copy := *id
	return &copy
}
