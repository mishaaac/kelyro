// Package doctorsqlite adapts workspace SQLite health to Doctor's neutral
// storage diagnostic contract.
package doctorsqlite

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/doctor"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
)

// Probe opens and independently inspects one workspace database.
type Probe struct{ backup sqlite.BackupFunc }

func New() *Probe { return &Probe{} }

// WithMigrationBackup installs the mandatory preflight for future destructive
// SQLite migrations discovered by Doctor.
func (probe *Probe) WithMigrationBackup(create sqlite.BackupFunc) *Probe {
	probe.backup = create
	return probe
}

func (probe *Probe) Check(ctx context.Context, workspaceRoot string) (health doctor.StorageHealth) {
	database, err := sqlite.Open(ctx, workspaceRoot, sqlite.WithDestructiveMigrationBackup(probe.backup))
	if err != nil {
		health.DatabaseError = err
		health.MigrationError = err
		health.ArtifactIndexError = err
		return health
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil && health.DatabaseError == nil {
			health.DatabaseError = fmt.Errorf("close workspace database: %w", closeErr)
		}
	}()

	version, err := database.SchemaVersion(ctx)
	switch {
	case err != nil:
		health.MigrationError = err
	case version != sqlite.LatestSchemaVersion():
		health.MigrationError = fmt.Errorf("schema version is %d, expected %d", version, sqlite.LatestSchemaVersion())
	}
	health.ArtifactIndexError = database.CheckArtifactIndex(ctx)
	return health
}

var _ doctor.StorageProbe = (*Probe)(nil)
