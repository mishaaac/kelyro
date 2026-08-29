package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/research/application"
)

type researchReleaseIngestionRepository struct {
	executor executor
	timeout  time.Duration
}

type researchTransactionStarter interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

func (repository *researchReleaseIngestionRepository) Commit(ctx context.Context, batch application.ReleaseIngestionBatch) error {
	const operation = "commit SQLite release ingestion"
	if err := validateReleaseIngestionBatch(batch); err != nil {
		return researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	starter, startsTransaction := repository.executor.(researchTransactionStarter)
	if !startsTransaction {
		// The executor is already a caller-owned transaction. Returning any error
		// lets the outer Database.WithTransaction boundary roll the batch back.
		return commitReleaseIngestion(opCtx, repository.executor, repository.timeout, batch)
	}
	transaction, err := starter.BeginTx(opCtx, nil)
	if err != nil {
		return researchPersistence(operation, err)
	}
	if err := commitReleaseIngestion(opCtx, transaction, repository.timeout, batch); err != nil {
		if rollbackErr := transaction.Rollback(); rollbackErr != nil {
			return errors.Join(err, researchPersistence(operation, fmt.Errorf("rollback release ingestion: %w", rollbackErr)))
		}
		return err
	}
	if err := transaction.Commit(); err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}

func validateReleaseIngestionBatch(batch application.ReleaseIngestionBatch) error {
	if err := batch.ValidateBounds(); err != nil {
		return err
	}
	for _, item := range batch.Evidence {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	for _, item := range batch.Claims {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	for _, item := range batch.Releases {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	for _, item := range batch.StatusUpdates {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func commitReleaseIngestion(ctx context.Context, target executor, timeout time.Duration, batch application.ReleaseIngestionBatch) error {
	prepared := newPreparedBatchExecutor(target)
	target = prepared
	evidence := &researchEvidenceRepository{target, timeout}
	claims := &researchClaimRepository{target, timeout}
	releases := &researchReleaseRepository{target, timeout}
	var writeErr error
	for _, item := range batch.Evidence {
		if err := evidence.Append(ctx, item); err != nil {
			writeErr = err
			break
		}
	}
	if writeErr == nil {
		for _, item := range batch.Claims {
			if err := claims.Append(ctx, item); err != nil {
				writeErr = err
				break
			}
		}
	}
	if writeErr == nil {
		for _, item := range batch.Releases {
			if err := releases.Create(ctx, item); err != nil {
				writeErr = err
				break
			}
		}
	}
	if writeErr == nil {
		for _, item := range batch.StatusUpdates {
			if err := releases.Update(ctx, item); err != nil {
				writeErr = err
				break
			}
		}
	}
	closeErr := prepared.close()
	if writeErr != nil {
		if closeErr != nil {
			return errors.Join(writeErr, researchPersistence("close SQLite release ingestion batch", closeErr))
		}
		return writeErr
	}
	if closeErr != nil {
		return researchPersistence("close SQLite release ingestion batch", closeErr)
	}
	return nil
}

type statementPreparer interface {
	executor
	PrepareContext(context.Context, string) (*sql.Stmt, error)
}

// preparedBatchExecutor prepares only repeated writes. Reads keep their normal
// repository paths so relationship and conflict semantics remain unchanged.
type preparedBatchExecutor struct {
	target     executor
	preparer   statementPreparer
	statements map[string]*sql.Stmt
}

func newPreparedBatchExecutor(target executor) *preparedBatchExecutor {
	preparer, ok := target.(statementPreparer)
	if !ok {
		return &preparedBatchExecutor{target: target}
	}
	return &preparedBatchExecutor{target: target, preparer: preparer, statements: make(map[string]*sql.Stmt)}
}

func (executor *preparedBatchExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if executor.preparer == nil {
		return executor.target.ExecContext(ctx, query, args...)
	}
	statement := executor.statements[query]
	if statement == nil {
		var err error
		statement, err = executor.preparer.PrepareContext(ctx, query)
		if err != nil {
			return nil, err
		}
		executor.statements[query] = statement
	}
	return statement.ExecContext(ctx, args...)
}

func (executor *preparedBatchExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return executor.target.QueryContext(ctx, query, args...)
}

func (executor *preparedBatchExecutor) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return executor.target.QueryRowContext(ctx, query, args...)
}

func (executor *preparedBatchExecutor) close() error {
	var result error
	for _, statement := range executor.statements {
		if err := statement.Close(); err != nil {
			result = errors.Join(result, err)
		}
	}
	executor.statements = nil
	return result
}
