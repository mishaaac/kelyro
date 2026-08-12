// Package auditsqlite binds the neutral audit trail to workspace SQLite.
package auditsqlite

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
)

// Factory opens durable workspace audit trails.
type Factory struct {
	appVersion string
	backup     sqlite.BackupFunc
}

// WithMigrationBackup installs the mandatory preflight for future destructive
// SQLite migrations.
func (factory *Factory) WithMigrationBackup(create sqlite.BackupFunc) *Factory {
	factory.backup = create
	return factory
}

// NewFactory creates an audit factory that stamps the running app version.
func NewFactory(appVersion string) *Factory {
	return &Factory{appVersion: appVersion}
}

// Open owns one database handle until the returned store is closed.
func (factory *Factory) Open(ctx context.Context, workspaceRoot string) (audit.Store, error) {
	database, err := sqlite.Open(ctx, workspaceRoot, sqlite.WithAppVersion(factory.appVersion), sqlite.WithDestructiveMigrationBackup(factory.backup))
	if err != nil {
		return nil, err
	}
	return &store{Trail: database.Repositories().Audit, database: database}, nil
}

type store struct {
	audit.Trail
	database *sqlite.Database
}

func (store *store) Close() error {
	if err := store.database.Close(); err != nil {
		return fmt.Errorf("close audit database: %w", err)
	}
	return nil
}

var _ audit.WorkspaceStoreFactory = (*Factory)(nil)
var _ audit.Store = (*store)(nil)
