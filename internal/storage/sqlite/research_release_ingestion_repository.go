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
	evidence := &researchEvidenceRepository{target, timeout}
	claims := &researchClaimRepository{target, timeout}
	releases := &researchReleaseRepository{target, timeout}
	for _, item := range batch.Evidence {
		if err := evidence.Append(ctx, item); err != nil {
			return err
		}
	}
	for _, item := range batch.Claims {
		if err := claims.Append(ctx, item); err != nil {
			return err
		}
	}
	for _, item := range batch.Releases {
		if err := releases.Create(ctx, item); err != nil {
			return err
		}
	}
	for _, item := range batch.StatusUpdates {
		if err := releases.Update(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
