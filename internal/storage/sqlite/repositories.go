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

const timestampFormat = time.RFC3339Nano

type executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
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
}

var (
	_ storage.StateStore         = (*stateRepository)(nil)
	_ storage.WorkspaceMetaStore = (*workspaceMetaRepository)(nil)
	_ artifacts.Index            = (*artifactRepository)(nil)
	_ audit.Recorder             = (*auditRepository)(nil)
)

func newRepositories(target executor, timeout time.Duration, now func() time.Time) Repositories {
	return Repositories{
		State:         &stateRepository{executor: target, timeout: timeout, now: now},
		WorkspaceMeta: &workspaceMetaRepository{executor: target, timeout: timeout, now: now},
		Artifacts:     &artifactRepository{executor: target, timeout: timeout, now: now},
		Audit:         &auditRepository{executor: target, timeout: timeout, now: now},
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
	err := repository.executor.QueryRowContext(operationContext,
		"SELECT ownership FROM artifact_index WHERE path = ?",
		path,
	).Scan(&artifact.Ownership)
	if errors.Is(err, sql.ErrNoRows) {
		return artifacts.Artifact{}, false, nil
	}
	if err != nil {
		return artifacts.Artifact{}, false, fmt.Errorf("read artifact %s: %w", path, err)
	}
	if !artifact.Ownership.Valid() {
		return artifacts.Artifact{}, false, fmt.Errorf("artifact %s has invalid ownership %q", path, artifact.Ownership)
	}
	return artifact, true, nil
}

func (repository *artifactRepository) Put(ctx context.Context, artifact artifacts.Artifact) error {
	if err := requireName("artifact path", artifact.Path); err != nil {
		return err
	}
	if !artifact.Ownership.Valid() {
		return fmt.Errorf("artifact ownership %q is invalid", artifact.Ownership)
	}

	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `
INSERT INTO artifact_index (path, ownership, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
    ownership = excluded.ownership,
    updated_at = excluded.updated_at`,
		artifact.Path,
		artifact.Ownership,
		repository.now().UTC().Format(timestampFormat),
	)
	if err != nil {
		return fmt.Errorf("write artifact %s: %w", artifact.Path, err)
	}
	return nil
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
	if err := requireName("audit action", event.Action); err != nil {
		return err
	}
	if err := requireName("audit subject", event.Subject); err != nil {
		return err
	}

	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode audit metadata: %w", err)
	}

	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	if _, err := repository.executor.ExecContext(operationContext, `
INSERT INTO audit_events (occurred_at, action, subject, metadata_json)
VALUES (?, ?, ?, ?)`,
		repository.now().UTC().Format(timestampFormat),
		event.Action,
		event.Subject,
		string(encoded),
	); err != nil {
		return fmt.Errorf("record audit event %s: %w", event.Action, err)
	}
	return nil
}

func requireName(kind, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be empty", kind)
	}
	return nil
}
