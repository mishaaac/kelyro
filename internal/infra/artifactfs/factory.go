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
type Factory struct {
	appVersion string
}

// NewFactory creates a workspace artifact-store factory.
func NewFactory(appVersion ...string) *Factory {
	version := "unknown"
	if len(appVersion) > 0 {
		version = appVersion[0]
	}
	return &Factory{appVersion: version}
}

// Open creates the workspace database when needed and binds its artifact index
// to an ownership-aware filesystem store.
func (factory *Factory) Open(ctx context.Context, workspaceRoot string) (artifacts.WorkspaceStore, error) {
	database, err := sqlite.Open(ctx, workspaceRoot, sqlite.WithAppVersion(factory.appVersion))
	if err != nil {
		return nil, err
	}
	repositories := database.Repositories()
	store, err := New(workspaceRoot, repositories.Artifacts, repositories.Audit)
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
