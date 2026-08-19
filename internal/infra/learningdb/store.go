// Package learningdb binds Student Core application services to a
// workspace-local SQLite database.
package learningdb

import (
	"context"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
)

// Factory opens one profile store per workspace operation.
type Factory struct {
	appVersion string
	backup     sqlite.BackupFunc
	now        func() time.Time
}

func NewFactory(appVersion ...string) *Factory {
	version := "unknown"
	if len(appVersion) > 0 {
		version = appVersion[0]
	}
	return &Factory{appVersion: version, now: time.Now}
}

func (factory *Factory) WithMigrationBackup(create sqlite.BackupFunc) *Factory {
	factory.backup = create
	return factory
}

func (factory *Factory) Open(ctx context.Context, workspaceRoot string) (application.ProfileStore, error) {
	database, err := sqlite.Open(ctx, workspaceRoot, sqlite.WithAppVersion(factory.appVersion), sqlite.WithDestructiveMigrationBackup(factory.backup))
	if err != nil {
		return nil, err
	}
	now := factory.now
	if now == nil {
		now = time.Now
	}
	students := application.NewStudentService(database.LearningRepositories().Students)
	return &store{database: database, profiles: application.NewProfileService(students, application.WithProfileClock(now))}, nil
}

type store struct {
	database *sqlite.Database
	profiles application.ProfileService
}

func (store *store) Profiles() application.ProfileService { return store.profiles }

func (store *store) Close() error {
	if err := store.database.Close(); err != nil {
		return fmt.Errorf("close learner profile database: %w", err)
	}
	return nil
}

var _ application.ProfileStoreFactory = (*Factory)(nil)
var _ application.ProfileStore = (*store)(nil)
