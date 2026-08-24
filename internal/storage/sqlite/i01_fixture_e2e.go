//go:build e2e

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mishaaac/kelyro/internal/platform"
)

const i01FoundationSchemaVersion = 3

// CreateI01FoundationFixture creates the final I-01 database schema for E2E
// migration tests. It deliberately reuses the immutable published migrations
// instead of maintaining a second SQL representation of the legacy schema.
func CreateI01FoundationFixture(ctx context.Context, workspaceRoot string, now time.Time) (string, error) {
	internal, err := platform.WorkspaceInternalDir(workspaceRoot)
	if err != nil {
		return "", fmt.Errorf("resolve I-01 fixture directory: %w", err)
	}
	if err := os.MkdirAll(internal, 0o700); err != nil {
		return "", fmt.Errorf("create I-01 fixture directory: %w", err)
	}
	path, err := platform.WorkspaceDBPath(workspaceRoot)
	if err != nil {
		return "", fmt.Errorf("resolve I-01 fixture database: %w", err)
	}
	if _, err := os.Lstat(path); err == nil {
		return "", fmt.Errorf("create I-01 fixture: database already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect I-01 fixture database: %w", err)
	}

	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		return "", fmt.Errorf("open I-01 fixture database: %w", err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{
		sql: handle, path: filepath.Clean(path), timeout: defaultOperationTimeout,
		now: func() time.Time { return now }, version: "v0.1.0-alpha.2-e2e",
	}
	if err := database.migrate(ctx, foundationMigrations[:i01FoundationSchemaVersion]); err != nil {
		_ = database.Close()
		return "", fmt.Errorf("apply I-01 fixture migrations: %w", err)
	}
	if err := database.Repositories().State.Set(ctx, "foundation", "i01-preserved", []byte("preserved")); err != nil {
		_ = database.Close()
		return "", fmt.Errorf("seed I-01 fixture state: %w", err)
	}
	if err := database.Close(); err != nil {
		return "", fmt.Errorf("close I-01 fixture database: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("restrict I-01 fixture database: %w", err)
	}
	return path, nil
}
