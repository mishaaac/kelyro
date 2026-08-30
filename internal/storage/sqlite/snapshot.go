package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/mishaaac/kelyro/internal/backup"
)

// SnapshotValidator verifies a copied SQLite database in read-only mode. It
// never creates a database or applies migrations.
type SnapshotValidator struct{}

func (SnapshotValidator) Validate(ctx context.Context, path string) (int, error) {
	handle, err := openSnapshot(path, true)
	if err != nil {
		return 0, err
	}
	defer handle.Close()
	if err := handle.PingContext(ctx); err != nil {
		return 0, fmt.Errorf("connect to SQLite snapshot: %w", err)
	}

	rows, err := handle.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrIntegrity, err)
	}
	checked := false
	for rows.Next() {
		checked = true
		var result string
		if err := rows.Scan(&result); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("%w: %v", ErrIntegrity, err)
		}
		if result != "ok" {
			_ = rows.Close()
			return 0, fmt.Errorf("%w: %s", ErrIntegrity, result)
		}
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrIntegrity, err)
	}
	if !checked {
		return 0, fmt.Errorf("%w: no result", ErrIntegrity)
	}

	applied, err := loadAppliedMigrations(ctx, handle)
	if err != nil {
		return 0, err
	}
	if err := validateAppliedMigrations(applied, foundationMigrations); err != nil {
		return 0, err
	}
	version := 0
	for appliedVersion := range applied {
		if appliedVersion > version {
			version = appliedVersion
		}
	}
	if err := checkRelationalIntegrity(ctx, handle); err != nil {
		return 0, err
	}
	if err := checkStudentCoreIntegrity(ctx, handle, version); err != nil {
		return 0, err
	}
	return version, nil
}

type artifactIndexRecord struct {
	path            string
	ownership       string
	createdBy       string
	contentHash     string
	createdAt       string
	lastGeneratedAt string
	expectedVersion string
	updatedAt       string
}

// ReconcileUnbackedArtifacts keeps the current integrity records for visible
// generated artifacts because those files are deliberately excluded from a
// machine-state backup and remain in place during restore.
func (SnapshotValidator) ReconcileUnbackedArtifacts(ctx context.Context, currentPath, restoredPath string) error {
	current, err := openSnapshot(currentPath, true)
	if err != nil {
		return fmt.Errorf("open current SQLite database: %w", err)
	}
	defer current.Close()
	rows, err := current.QueryContext(ctx, `
SELECT path, ownership, created_by, content_hash, created_at,
       last_generated_at, expected_version, updated_at
FROM artifact_index
WHERE ownership = 'system-generated-human-readable'
ORDER BY path`)
	if err != nil {
		return fmt.Errorf("read current visible artifact index: %w", err)
	}
	var records []artifactIndexRecord
	for rows.Next() {
		var record artifactIndexRecord
		if err := rows.Scan(
			&record.path, &record.ownership, &record.createdBy, &record.contentHash,
			&record.createdAt, &record.lastGeneratedAt, &record.expectedVersion, &record.updatedAt,
		); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan current visible artifact index: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate current visible artifact index: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close current visible artifact index: %w", err)
	}

	restored, err := openSnapshot(restoredPath, false)
	if err != nil {
		return fmt.Errorf("open restored SQLite database: %w", err)
	}
	defer restored.Close()
	transaction, err := restored.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin artifact index reconciliation: %w", err)
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx,
		"DELETE FROM artifact_index WHERE ownership = 'system-generated-human-readable'",
	); err != nil {
		return fmt.Errorf("clear restored visible artifact index: %w", err)
	}
	for _, record := range records {
		if _, err := transaction.ExecContext(ctx, `
INSERT INTO artifact_index (
    path, ownership, created_by, content_hash, created_at,
    last_generated_at, expected_version, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			record.path, record.ownership, record.createdBy, record.contentHash,
			record.createdAt, record.lastGeneratedAt, record.expectedVersion, record.updatedAt,
		); err != nil {
			return fmt.Errorf("preserve visible artifact index for %s: %w", record.path, err)
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit artifact index reconciliation: %w", err)
	}
	return nil
}

func openSnapshot(path string, readOnly bool) (*sql.DB, error) {
	databaseURL := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	if filepath.VolumeName(path) != "" && !strings.HasPrefix(databaseURL.Path, "/") {
		databaseURL.Path = "/" + databaseURL.Path
	}
	query := databaseURL.Query()
	if readOnly {
		query.Set("mode", "ro")
		query.Add("_pragma", "query_only(1)")
	} else {
		query.Set("mode", "rw")
	}
	databaseURL.RawQuery = query.Encode()

	handle, err := sql.Open("sqlite", databaseURL.String())
	if err != nil {
		return nil, fmt.Errorf("open SQLite snapshot: %w", err)
	}
	handle.SetMaxOpenConns(1)
	return handle, nil
}

var _ backup.Validator = SnapshotValidator{}
var _ backup.RestoreReconciler = SnapshotValidator{}
