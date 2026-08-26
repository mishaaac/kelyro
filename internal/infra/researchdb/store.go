// Package researchdb binds Research application services to a workspace-local
// SQLite database without exposing storage details to presentation packages.
package researchdb

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
)

type Factory struct {
	appVersion string
	backup     sqlite.BackupFunc
}

func NewFactory(appVersion string) *Factory { return &Factory{appVersion: appVersion} }

func (factory *Factory) WithMigrationBackup(create sqlite.BackupFunc) *Factory {
	factory.backup = create
	return factory
}

func (factory *Factory) Open(ctx context.Context, workspaceRoot string) (application.SourceRegistryStore, error) {
	database, err := sqlite.Open(ctx, workspaceRoot, sqlite.WithAppVersion(factory.appVersion), sqlite.WithDestructiveMigrationBackup(factory.backup))
	if err != nil {
		return nil, err
	}
	registry := application.NewSourceRegistryService(database.Repositories().Research.SourceRegistry)
	provenance := application.NewProvenanceService(database.Repositories().Research.Provenance)
	freshness := application.NewFreshnessService(database.Repositories().Research.Freshness)
	return &store{database: database, registry: registry, provenance: provenance, freshness: freshness}, nil
}

type store struct {
	database   *sqlite.Database
	registry   application.SourceRegistryService
	provenance application.ProvenanceService
	freshness  application.FreshnessService
}

func (store *store) Registry() application.SourceRegistryService { return store.registry }
func (store *store) Provenance() application.ProvenanceService   { return store.provenance }
func (store *store) Freshness() application.FreshnessService     { return store.freshness }

func (store *store) Close() error {
	if err := store.database.Close(); err != nil {
		return fmt.Errorf("close research database: %w", err)
	}
	return nil
}

var _ application.SourceRegistryStoreFactory = (*Factory)(nil)
var _ application.SourceRegistryStore = (*store)(nil)
