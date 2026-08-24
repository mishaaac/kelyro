package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/artifacts"
	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/storage"
)

// timestampFormat always emits all nine fractional digits. Several schema
// invariants and indexes compare UTC timestamps as TEXT, so the persisted
// representation must preserve chronological order lexicographically.
const timestampFormat = "2006-01-02T15:04:05.000000000Z07:00"

// parseTimestamp remains permissive for databases written before timestamps
// used fixed-width fractional seconds.
func parseTimestamp(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

type executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type stateRepository struct {
	executor executor
	timeout  time.Duration
	now      func() time.Time
}

type workspaceMetaRepository struct {
	executor executor
	timeout  time.Duration
	now      func() time.Time
}

type artifactRepository struct {
	executor executor
	timeout  time.Duration
	now      func() time.Time
}

type auditRepository struct {
	executor executor
	timeout  time.Duration
	now      func() time.Time
	version  string
}

var (
	_ storage.StateStore         = (*stateRepository)(nil)
	_ storage.WorkspaceMetaStore = (*workspaceMetaRepository)(nil)
	_ artifacts.Index            = (*artifactRepository)(nil)
	_ audit.Trail                = (*auditRepository)(nil)
)

func newRepositories(target executor, timeout time.Duration, now func() time.Time, version string) Repositories {
	return Repositories{
		State:         &stateRepository{executor: target, timeout: timeout, now: now},
		WorkspaceMeta: &workspaceMetaRepository{executor: target, timeout: timeout, now: now},
		Artifacts:     &artifactRepository{executor: target, timeout: timeout, now: now},
		Audit:         &auditRepository{executor: target, timeout: timeout, now: now, version: version},
	}
}

func (repository *stateRepository) Get(ctx context.Context, namespace, key string) ([]byte, bool, error) {
	if err := requireName("state namespace", namespace); err != nil {
		return nil, false, err
	}
	if err := requireName("state key", key); err != nil {
		return nil, false, err
	}

	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()

	var value []byte
	err := repository.executor.QueryRowContext(operationContext,
		"SELECT value FROM app_state WHERE namespace = ? AND key = ?",
		namespace,
		key,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read application state %s/%s: %w", namespace, key, err)
	}
	return append([]byte(nil), value...), true, nil
}

func (repository *stateRepository) Set(ctx context.Context, namespace, key string, value []byte) error {
	if err := requireName("state namespace", namespace); err != nil {
		return err
	}
	if err := requireName("state key", key); err != nil {
		return err
	}

	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()

	if value == nil {
		value = []byte{}
	}
	_, err := repository.executor.ExecContext(operationContext, `
INSERT INTO app_state (namespace, key, value, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(namespace, key) DO UPDATE SET
    value = excluded.value,
    updated_at = excluded.updated_at`,
		namespace,
		key,
		value,
		repository.now().UTC().Format(timestampFormat),
	)
	if err != nil {
		return fmt.Errorf("write application state %s/%s: %w", namespace, key, err)
	}
	return nil
}

func (repository *stateRepository) Delete(ctx context.Context, namespace, key string) error {
	if err := requireName("state namespace", namespace); err != nil {
		return err
	}
	if err := requireName("state key", key); err != nil {
		return err
	}

	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	if _, err := repository.executor.ExecContext(operationContext,
		"DELETE FROM app_state WHERE namespace = ? AND key = ?",
		namespace,
		key,
	); err != nil {
		return fmt.Errorf("delete application state %s/%s: %w", namespace, key, err)
	}
	return nil
}

func (repository *workspaceMetaRepository) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if err := requireName("workspace metadata key", key); err != nil {
		return nil, false, err
	}

	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()

	var value []byte
	err := repository.executor.QueryRowContext(operationContext,
		"SELECT value FROM workspace_meta WHERE key = ?",
		key,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read workspace metadata %s: %w", key, err)
	}
	return append([]byte(nil), value...), true, nil
}

func (repository *workspaceMetaRepository) Set(ctx context.Context, key string, value []byte) error {
	if err := requireName("workspace metadata key", key); err != nil {
		return err
	}

	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	if value == nil {
		value = []byte{}
	}
	_, err := repository.executor.ExecContext(operationContext, `
INSERT INTO workspace_meta (key, value, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
    value = excluded.value,
    updated_at = excluded.updated_at`,
		key,
		value,
		repository.now().UTC().Format(timestampFormat),
	)
	if err != nil {
		return fmt.Errorf("write workspace metadata %s: %w", key, err)
	}
	return nil
}

func (repository *workspaceMetaRepository) Delete(ctx context.Context, key string) error {
	if err := requireName("workspace metadata key", key); err != nil {
		return err
	}

	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	if _, err := repository.executor.ExecContext(operationContext,
		"DELETE FROM workspace_meta WHERE key = ?",
		key,
	); err != nil {
		return fmt.Errorf("delete workspace metadata %s: %w", key, err)
	}
	return nil
}

func (repository *artifactRepository) Get(ctx context.Context, path string) (artifacts.Artifact, bool, error) {
	if err := requireName("artifact path", path); err != nil {
		return artifacts.Artifact{}, false, err
	}

	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()

	artifact := artifacts.Artifact{Path: path}
	var createdAt, lastGeneratedAt string
	err := repository.executor.QueryRowContext(operationContext,
		`SELECT ownership, created_by, content_hash, created_at, last_generated_at, expected_version
FROM artifact_index WHERE path = ?`,
		path,
	).Scan(
		&artifact.Ownership,
		&artifact.CreatedBy,
		&artifact.ContentHash,
		&createdAt,
		&lastGeneratedAt,
		&artifact.ExpectedVersion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return artifacts.Artifact{}, false, nil
	}
	if err != nil {
		return artifacts.Artifact{}, false, fmt.Errorf("read artifact %s: %w", path, err)
	}
	if !artifact.Ownership.Valid() {
		return artifacts.Artifact{}, false, fmt.Errorf("artifact %s has invalid ownership %q", path, artifact.Ownership)
	}
	artifact.CreatedAt, err = parseArtifactTime(path, "creation", createdAt)
	if err != nil {
		return artifacts.Artifact{}, false, err
	}
	artifact.LastGeneratedAt, err = parseArtifactTime(path, "generation", lastGeneratedAt)
	if err != nil {
		return artifacts.Artifact{}, false, err
	}
	return artifact, true, nil
}

func (repository *artifactRepository) Put(ctx context.Context, artifact artifacts.Artifact) error {
	if err := artifact.Validate(); err != nil {
		return err
	}

	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `
INSERT INTO artifact_index (
    path, ownership, created_by, content_hash, created_at,
    last_generated_at, expected_version, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
    ownership = excluded.ownership,
	created_by = excluded.created_by,
	content_hash = excluded.content_hash,
	created_at = excluded.created_at,
	last_generated_at = excluded.last_generated_at,
	expected_version = excluded.expected_version,
    updated_at = excluded.updated_at`,
		artifact.Path,
		artifact.Ownership,
		artifact.CreatedBy,
		artifact.ContentHash,
		artifact.CreatedAt.UTC().Format(timestampFormat),
		artifact.LastGeneratedAt.UTC().Format(timestampFormat),
		artifact.ExpectedVersion,
		repository.now().UTC().Format(timestampFormat),
	)
	if err != nil {
		return fmt.Errorf("write artifact %s: %w", artifact.Path, err)
	}
	return nil
}

func parseArtifactTime(path, field, encoded string) (time.Time, error) {
	if encoded == "" {
		return time.Time{}, nil // Records created before schema version 2 are legacy metadata.
	}
	parsed, err := parseTimestamp(encoded)
	if err != nil {
		return time.Time{}, fmt.Errorf("artifact %s has invalid %s time: %w", path, field, err)
	}
	return parsed, nil
}

func (repository *artifactRepository) Delete(ctx context.Context, path string) error {
	if err := requireName("artifact path", path); err != nil {
		return err
	}

	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	if _, err := repository.executor.ExecContext(operationContext,
		"DELETE FROM artifact_index WHERE path = ?",
		path,
	); err != nil {
		return fmt.Errorf("delete artifact %s: %w", path, err)
	}
	return nil
}

func (repository *auditRepository) Record(ctx context.Context, event audit.Event) error {
	if err := event.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(event.AppVersion) == "" {
		event.AppVersion = repository.version
	}
	if strings.TrimSpace(event.AppVersion) == "" {
		event.AppVersion = "unknown"
	}

	metadata := audit.SafeMetadata(event.Metadata)
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode audit metadata: %w", err)
	}

	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	if _, err := repository.executor.ExecContext(operationContext, `
INSERT INTO audit_events (occurred_at, action, actor, subject, metadata_json, app_version)
VALUES (?, ?, ?, ?, ?, ?)`,
		repository.now().UTC().Format(timestampFormat),
		event.Name,
		event.Actor,
		event.Subject,
		string(encoded),
		event.AppVersion,
	); err != nil {
		return fmt.Errorf("record audit event %s: %w", event.Name, err)
	}
	return nil
}

func (repository *auditRepository) List(ctx context.Context) ([]audit.Entry, error) {
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, `
SELECT occurred_at, action, actor, subject, metadata_json, app_version
FROM audit_events
ORDER BY occurred_at, id`)
	if err != nil {
		return nil, fmt.Errorf("read audit trail: %w", err)
	}
	defer rows.Close()

	var entries []audit.Entry
	for rows.Next() {
		var encodedTimestamp, encodedMetadata string
		var entry audit.Entry
		if err := rows.Scan(
			&encodedTimestamp,
			&entry.Event,
			&entry.Actor,
			&entry.Subject,
			&encodedMetadata,
			&entry.AppVersion,
		); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		entry.Timestamp, err = parseTimestamp(encodedTimestamp)
		if err != nil {
			return nil, fmt.Errorf("parse audit event timestamp: %w", err)
		}
		if !entry.Actor.Valid() {
			return nil, fmt.Errorf("audit event %s has invalid actor %q", entry.Event, entry.Actor)
		}
		if err := json.Unmarshal([]byte(encodedMetadata), &entry.Metadata); err != nil {
			return nil, fmt.Errorf("decode audit event %s metadata: %w", entry.Event, err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit trail: %w", err)
	}
	return entries, nil
}

func requireName(kind, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be empty", kind)
	}
	return nil
}
