package artifactfs

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/artifacts"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
)

// Factory opens filesystem artifact stores with a workspace-local SQLite
// integrity index.
type Factory struct{}

// NewFactory creates a workspace artifact-store factory.
func NewFactory() *Factory {
	return &Factory{}
}

// Open creates the workspace database when needed and binds its artifact index
// to an ownership-aware filesystem store.
func (*Factory) Open(ctx context.Context, workspaceRoot string) (artifacts.WorkspaceStore, error) {
	database, err := sqlite.Open(ctx, workspaceRoot)
	if err != nil {
		return nil, err
	}
	store, err := New(workspaceRoot, database.Repositories().Artifacts)
	if err != nil {
		if closeErr := database.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close workspace database: %w", closeErr))
		}
		return nil, err
	}
	return &persistentStore{Store: store, database: database}, nil
}

type persistentStore struct {
	*Store
	database *sqlite.Database
}

func (store *persistentStore) Close() error {
	if err := store.database.Close(); err != nil {
		return fmt.Errorf("close workspace database: %w", err)
	}
	return nil
}

var _ artifacts.WorkspaceStoreFactory = (*Factory)(nil)
var _ artifacts.WorkspaceStore = (*persistentStore)(nil)
