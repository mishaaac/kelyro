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
	databaseURL := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	if filepath.VolumeName(path) != "" && !strings.HasPrefix(databaseURL.Path, "/") {
		databaseURL.Path = "/" + databaseURL.Path
	}
	query := databaseURL.Query()
	query.Set("mode", "ro")
	query.Add("_pragma", "query_only(1)")
	databaseURL.RawQuery = query.Encode()

	handle, err := sql.Open("sqlite", databaseURL.String())
	if err != nil {
		return 0, fmt.Errorf("open SQLite snapshot: %w", err)
	}
	defer handle.Close()
	handle.SetMaxOpenConns(1)
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

var _ backup.Validator = SnapshotValidator{}
