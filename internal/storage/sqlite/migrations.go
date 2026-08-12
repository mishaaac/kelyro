package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const migrationTableSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY CHECK (version > 0),
    name TEXT NOT NULL CHECK (length(name) > 0),
    checksum TEXT NOT NULL CHECK (length(checksum) = 64),
    applied_at TEXT NOT NULL
)`

type migration struct {
	version     int
	name        string
	statements  []string
	destructive bool
}

var foundationMigrations = []migration{
	{
		version: 1,
		name:    "foundation repositories",
		statements: []string{
			`CREATE TABLE workspace_meta (
    key TEXT PRIMARY KEY CHECK (length(key) > 0),
    value BLOB NOT NULL,
    updated_at TEXT NOT NULL
)`,
			`CREATE TABLE app_state (
    namespace TEXT NOT NULL CHECK (length(namespace) > 0),
    key TEXT NOT NULL CHECK (length(key) > 0),
    value BLOB NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (namespace, key)
)`,
			`CREATE TABLE artifact_index (
    path TEXT PRIMARY KEY CHECK (length(path) > 0),
    ownership TEXT NOT NULL CHECK (
        ownership IN ('machine-owned', 'system-generated-human-readable', 'student-owned')
    ),
    updated_at TEXT NOT NULL
)`,
			`CREATE TABLE audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at TEXT NOT NULL,
    action TEXT NOT NULL CHECK (length(action) > 0),
    subject TEXT NOT NULL CHECK (length(subject) > 0),
    metadata_json TEXT NOT NULL
)`,
			`CREATE INDEX audit_events_occurred_at_idx ON audit_events (occurred_at, id)`,
		},
	},
}

// LatestSchemaVersion returns the newest migration version embedded in this
// build.
func LatestSchemaVersion() int {
	if len(foundationMigrations) == 0 {
		return 0
	}
	return foundationMigrations[len(foundationMigrations)-1].version
}

// Migrate validates the applied history and transactionally applies every
// pending migration.
func (database *Database) Migrate(ctx context.Context) error {
	return database.migrate(ctx, foundationMigrations)
}

// SchemaVersion returns the greatest successfully applied migration version.
func (database *Database) SchemaVersion(ctx context.Context) (int, error) {
	operationContext, cancel := database.operationContext(ctx)
	defer cancel()

	var version int
	if err := database.sql.QueryRowContext(operationContext,
		"SELECT COALESCE(MAX(version), 0) FROM schema_migrations",
	).Scan(&version); err != nil {
		return 0, fmt.Errorf("read SQLite schema version: %w", err)
	}
	return version, nil
}

func (database *Database) migrate(ctx context.Context, migrations []migration) error {
	if err := validateMigrations(migrations); err != nil {
		return err
	}

	operationContext, cancel := database.operationContext(ctx)
	defer cancel()

	if _, err := database.sql.ExecContext(operationContext, migrationTableSQL); err != nil {
		return fmt.Errorf("initialize SQLite migration history: %w", err)
	}

	applied, err := loadAppliedMigrations(operationContext, database.sql)
	if err != nil {
		return err
	}
	if err := validateAppliedMigrations(applied, migrations); err != nil {
		return err
	}

	for _, next := range migrations {
		if _, exists := applied[next.version]; exists {
			continue
		}
		if next.destructive {
			if database.backup == nil {
				return fmt.Errorf("%w: migration %d (%s)", ErrBackupRequired, next.version, next.name)
			}
			if err := database.backup(operationContext, database.path, MigrationInfo{
				Version: next.version,
				Name:    next.name,
			}); err != nil {
				return fmt.Errorf("backup before migration %d (%s): %w", next.version, next.name, err)
			}
		}
		if err := database.applyMigration(operationContext, next); err != nil {
			return err
		}
	}

	return nil
}

func (database *Database) applyMigration(ctx context.Context, next migration) error {
	transaction, err := database.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d (%s): %w", next.version, next.name, err)
	}
	defer transaction.Rollback()

	for statementIndex, statement := range next.statements {
		if _, err := transaction.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migration %d (%s), statement %d: %w", next.version, next.name, statementIndex+1, err)
		}
	}
	if _, err := transaction.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, ?)",
		next.version,
		next.name,
		migrationChecksum(next),
		database.now().UTC().Format(timestampFormat),
	); err != nil {
		return fmt.Errorf("record migration %d (%s): %w", next.version, next.name, err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit migration %d (%s): %w", next.version, next.name, err)
	}
	return nil
}

type appliedMigration struct {
	name     string
	checksum string
}

func loadAppliedMigrations(ctx context.Context, database *sql.DB) (map[int]appliedMigration, error) {
	rows, err := database.QueryContext(ctx,
		"SELECT version, name, checksum FROM schema_migrations ORDER BY version",
	)
	if err != nil {
		return nil, fmt.Errorf("read SQLite migration history: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]appliedMigration)
	for rows.Next() {
		var version int
		var record appliedMigration
		if err := rows.Scan(&version, &record.name, &record.checksum); err != nil {
			return nil, fmt.Errorf("scan SQLite migration history: %w", err)
		}
		applied[version] = record
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SQLite migration history: %w", err)
	}
	return applied, nil
}

func validateMigrations(migrations []migration) error {
	for index, candidate := range migrations {
		expectedVersion := index + 1
		switch {
		case candidate.version != expectedVersion:
			return fmt.Errorf("migration definitions must be consecutive from 1: expected %d, got %d", expectedVersion, candidate.version)
		case strings.TrimSpace(candidate.name) == "":
			return fmt.Errorf("migration %d has an empty name", candidate.version)
		case len(candidate.statements) == 0:
			return fmt.Errorf("migration %d (%s) has no statements", candidate.version, candidate.name)
		}
	}
	return nil
}

func validateAppliedMigrations(applied map[int]appliedMigration, migrations []migration) error {
	versions := make([]int, 0, len(applied))
	for version := range applied {
		versions = append(versions, version)
	}
	sort.Ints(versions)

	for index, version := range versions {
		if version != index+1 || version > len(migrations) {
			return fmt.Errorf("%w: unexpected applied version %d", ErrMigrationHistory, version)
		}
		expected := migrations[version-1]
		record := applied[version]
		if record.name != expected.name || record.checksum != migrationChecksum(expected) {
			return fmt.Errorf("%w: migration %d (%s) was modified", ErrMigrationHistory, version, record.name)
		}
	}
	return nil
}

func migrationChecksum(candidate migration) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(strconv.Itoa(candidate.version)))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(candidate.name))
	if candidate.destructive {
		_, _ = digest.Write([]byte{1})
	} else {
		_, _ = digest.Write([]byte{0})
	}
	for _, statement := range candidate.statements {
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(statement))
	}
	return hex.EncodeToString(digest.Sum(nil))
}
