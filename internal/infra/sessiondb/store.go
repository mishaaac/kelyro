// Package sessiondb persists workspace session state in the Foundation SQLite
// database.
package sessiondb

import (
	"context"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/session"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
)

// Factory opens workspace-local session stores.
type Factory struct {
	now        func() time.Time
	appVersion string
	backup     sqlite.BackupFunc
}

// WithMigrationBackup installs the mandatory preflight for future destructive
// SQLite migrations.
func (factory *Factory) WithMigrationBackup(create sqlite.BackupFunc) *Factory {
	factory.backup = create
	return factory
}

// NewFactory creates a SQLite-backed session-store factory.
func NewFactory(appVersion ...string) *Factory {
	version := "unknown"
	if len(appVersion) > 0 {
		version = appVersion[0]
	}
	return &Factory{now: time.Now, appVersion: version}
}

// Open opens the existing Foundation database and binds session operations to
// its transaction runner.
func (factory *Factory) Open(ctx context.Context, workspaceRoot string) (session.Store, error) {
	database, err := sqlite.Open(ctx, workspaceRoot, sqlite.WithAppVersion(factory.appVersion), sqlite.WithDestructiveMigrationBackup(factory.backup))
	if err != nil {
		return nil, err
	}
	now := factory.now
	if now == nil {
		now = time.Now
	}
	return &store{database: database, now: now}, nil
}

type store struct {
	database *sqlite.Database
	now      func() time.Time
}

func (store *store) Resume(ctx context.Context) (result session.Resume, err error) {
	err = store.database.WithTransaction(ctx, func(repositories sqlite.Repositories) error {
		result, err = session.NewManager(repositories.State, repositories.Audit, store.now).Resume(ctx)
		return err
	})
	return result, err
}

func (store *store) Checkpoint(ctx context.Context, state session.State) error {
	return store.database.WithTransaction(ctx, func(repositories sqlite.Repositories) error {
		return session.NewManager(repositories.State, repositories.Audit, store.now).Checkpoint(ctx, state)
	})
}

func (store *store) Complete(ctx context.Context, state session.State) error {
	return store.database.WithTransaction(ctx, func(repositories sqlite.Repositories) error {
		return session.NewManager(repositories.State, repositories.Audit, store.now).Complete(ctx, state)
	})
}

func (store *store) Close() error {
	if err := store.database.Close(); err != nil {
		return fmt.Errorf("close session database: %w", err)
	}
	return nil
}

var _ session.WorkspaceStoreFactory = (*Factory)(nil)
var _ session.Store = (*store)(nil)
