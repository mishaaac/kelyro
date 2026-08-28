package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/backup"
	"github.com/mishaaac/kelyro/internal/cli"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/doctor"
	"github.com/mishaaac/kelyro/internal/infra/artifactfs"
	"github.com/mishaaac/kelyro/internal/infra/auditsqlite"
	"github.com/mishaaac/kelyro/internal/infra/backupfs"
	"github.com/mishaaac/kelyro/internal/infra/configfs"
	"github.com/mishaaac/kelyro/internal/infra/doctoros"
	"github.com/mishaaac/kelyro/internal/infra/doctorsqlite"
	"github.com/mishaaac/kelyro/internal/infra/editoros"
	"github.com/mishaaac/kelyro/internal/infra/learningdb"
	"github.com/mishaaac/kelyro/internal/infra/logfs"
	"github.com/mishaaac/kelyro/internal/infra/platformos"
	"github.com/mishaaac/kelyro/internal/infra/portabilityfs"
	"github.com/mishaaac/kelyro/internal/infra/researchcachefs"
	"github.com/mishaaac/kelyro/internal/infra/researchdb"
	"github.com/mishaaac/kelyro/internal/infra/sessiondb"
	"github.com/mishaaac/kelyro/internal/infra/updatecache"
	"github.com/mishaaac/kelyro/internal/infra/workspacefs"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
	"github.com/mishaaac/kelyro/internal/tui"
	"github.com/mishaaac/kelyro/internal/update"
	"github.com/mishaaac/kelyro/internal/version"
)

func main() {
	workspaces := workspacefs.New(version.Version)
	configs := configfs.New()
	backups := backupfs.New(version.Version, sqlite.SnapshotValidator{})
	portable := portabilityfs.New(version.Version, sqlite.SnapshotValidator{})
	updates := update.New(version.Version, newReleaseProvider(), updatecache.New())
	migrationBackup := func(ctx context.Context, databasePath string, migration sqlite.MigrationInfo) error {
		root := filepath.Dir(filepath.Dir(databasePath))
		global, err := configs.LoadGlobal()
		if err != nil {
			return err
		}
		project, err := configs.LoadProject(root)
		if err != nil {
			return err
		}
		settings, err := config.Resolve(global, project)
		if err != nil {
			return err
		}
		retention, ok := settings[config.KeyBackupRetention].NumberField()
		if !ok {
			return fmt.Errorf("backup retention configuration is invalid")
		}
		_, err = backups.Create(ctx, root, backup.CreateOptions{
			Reason:    fmt.Sprintf("migration-%d-%s", migration.Version, migration.Name),
			Retention: int(retention),
		})
		return err
	}
	service := app.NewService(workspaces, os.Getwd).
		WithConfig(configs).
		WithSecrets(newSecretStore()).
		WithArtifactStores(artifactfs.NewFactory(version.Version).WithMigrationBackup(migrationBackup)).
		WithSessionStores(sessiondb.NewFactory(version.Version).WithMigrationBackup(migrationBackup)).
		WithEditor(editoros.New()).
		WithDoctor(doctor.New(doctoros.New(), doctorsqlite.New().WithMigrationBackup(migrationBackup), doctor.DefaultRegistry())).
		WithLogging(logfs.New()).
		WithAudit(auditsqlite.NewFactory(version.Version).WithMigrationBackup(migrationBackup)).
		WithBackups(backups).
		WithPortability(portable).
		WithUpdates(updates).
		WithResearchStores(researchdb.NewFactory(version.Version).WithMigrationBackup(migrationBackup)).
		WithResearchCaches(researchcachefs.NewFactory()).
		WithProfiles(learningdb.NewFactory(version.Version).WithMigrationBackup(migrationBackup))
	runner := cli.NewRunner(service, os.Stdout, os.Stderr).
		WithSecretReader(cli.NewTerminalSecretReader(os.Stdin, os.Stderr)).
		WithConfirmer(cli.NewTextConfirmer(os.Stdin, os.Stderr)).
		WithInteractive(tui.NewRunner(service, os.Stdin, os.Stdout).WithPlatform(platformos.New()))
	os.Exit(runner.Run(context.Background(), os.Args[1:]))
}
