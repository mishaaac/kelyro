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
type Probe struct{}

func New() *Probe { return &Probe{} }

func (*Probe) Check(ctx context.Context, workspaceRoot string) (health doctor.StorageHealth) {
	database, err := sqlite.Open(ctx, workspaceRoot)
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
