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
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
	"github.com/mishaaac/kelyro/internal/doctor"
	"github.com/mishaaac/kelyro/internal/infra/artifactfs"
	"github.com/mishaaac/kelyro/internal/infra/auditsqlite"
	"github.com/mishaaac/kelyro/internal/infra/backupfs"
	"github.com/mishaaac/kelyro/internal/infra/configfs"
	"github.com/mishaaac/kelyro/internal/infra/doctoros"
	"github.com/mishaaac/kelyro/internal/infra/doctorsqlite"
	"github.com/mishaaac/kelyro/internal/infra/editoros"
	"github.com/mishaaac/kelyro/internal/infra/learningdb"
	"github.com/mishaaac/kelyro/internal/infra/learningmigration"
	"github.com/mishaaac/kelyro/internal/infra/learningpack"
	"github.com/mishaaac/kelyro/internal/infra/logfs"
	"github.com/mishaaac/kelyro/internal/infra/packcatalogfs"
	"github.com/mishaaac/kelyro/internal/infra/packfs"
	"github.com/mishaaac/kelyro/internal/infra/platformos"
	"github.com/mishaaac/kelyro/internal/infra/portabilityfs"
	"github.com/mishaaac/kelyro/internal/infra/researchcachefs"
	"github.com/mishaaac/kelyro/internal/infra/researchdb"
	"github.com/mishaaac/kelyro/internal/infra/researchfetch"
	"github.com/mishaaac/kelyro/internal/infra/researchhttp"
	"github.com/mishaaac/kelyro/internal/infra/researchnormalize"
	"github.com/mishaaac/kelyro/internal/infra/researchsearch"
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
	secrets := newSecretStore()
	searchTransport := researchsearch.DefaultTransportConfig()
	searchTransport.UserAgent = "Kelyro/" + version.Version
	researchSearch, err := researchsearch.NewFactory(searchTransport)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kelyro: initialize research search:", err)
		os.Exit(1)
	}
	fetchTransport := researchhttp.DefaultConfig()
	fetchTransport.UserAgent = "Kelyro/" + version.Version
	researchHTTP, err := researchhttp.New(fetchTransport, nil, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kelyro: initialize research fetch:", err)
		os.Exit(1)
	}
	researchFetcher := researchfetch.New(researchHTTP)
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
	auditStores := auditsqlite.NewFactory(version.Version).WithMigrationBackup(migrationBackup)
	profileStores := learningdb.NewFactory(version.Version).WithMigrationBackup(migrationBackup)
	service := app.NewService(workspaces, os.Getwd).
		WithConfig(configs).
		WithSecrets(secrets).
		WithArtifactStores(artifactfs.NewFactory(version.Version).WithMigrationBackup(migrationBackup)).
		WithSessionStores(sessiondb.NewFactory(version.Version).WithMigrationBackup(migrationBackup)).
		WithEditor(editoros.New()).
		WithDoctor(doctor.New(doctoros.New(), doctorsqlite.New().WithMigrationBackup(migrationBackup), doctor.DefaultRegistry())).
		WithLogging(logfs.New()).
		WithAudit(auditStores).
		WithBackups(backups).
		WithPortability(portable).
		WithUpdates(updates).
		WithResearchStores(researchdb.NewFactory(version.Version).WithMigrationBackup(migrationBackup)).
		WithResearchCaches(researchcachefs.NewFactory()).
		WithResearchSearch(researchSearch).
		WithResearchFetcher(researchFetcher).
		WithResearchNormalizer(researchnormalize.New()).
		WithProfiles(profileStores)
	packValidator := learningpack.NewValidator()
	packManager := curriculumapp.NewPackInstallerV1(packValidator, curriculumapp.NewPackDependencyResolverV1(), packfs.NewRepository(packValidator), curriculumapp.SystemClock{})
	packCatalog := curriculumapp.NewPackCatalogV1(nil, packcatalogfs.NewCache(), version.Version)
	packUpgradePreview := curriculumapp.NewPackUpgradePlannerV1(packManager, curriculumapp.NewCurriculumChangeClassifierV1(), curriculumapp.NewCurriculumMigrationPlannerV1(), curriculumapp.NewPackVersioningPolicyV1())
	curriculumInspector := curriculumapp.NewCurriculumInspectorV1()
	curriculumView := curriculumapp.NewCurriculumWorkspaceViewV1(packManager, curriculumInspector, packUpgradePreview)
	packBackupRetention := func(_ context.Context, root string) (int, error) {
		global, err := configs.LoadGlobal()
		if err != nil {
			return 0, err
		}
		project, err := configs.LoadProject(root)
		if err != nil {
			return 0, err
		}
		settings, err := config.Resolve(global, project)
		if err != nil {
			return 0, err
		}
		retention, ok := settings[config.KeyBackupRetention].NumberField()
		if !ok {
			return 0, fmt.Errorf("backup retention configuration is invalid")
		}
		return int(retention), nil
	}
	packUpgrade := curriculumapp.NewPackUpgradeExecutorV1(packUpgradePreview, packManager, backups,
		learningmigration.New(profileStores, sqlite.SnapshotValidator{}), auditStores, packBackupRetention, version.Version)
	runner := cli.NewRunner(service, os.Stdout, os.Stderr).
		WithSecretReader(cli.NewTerminalSecretReader(os.Stdin, os.Stderr)).
		WithConfirmer(cli.NewTextConfirmer(os.Stdin, os.Stderr)).
		WithInteractive(tui.NewRunner(service, os.Stdin, os.Stdout).WithPlatform(platformos.New()).WithCurriculum(curriculumView, workspaces, os.Getwd)).
		WithPackValidator(packValidator).
		WithPackManager(packManager, workspaces, os.Getwd).
		WithPackCatalog(packCatalog).
		WithPackUpgrade(packUpgrade).
		WithCurriculumInspector(curriculumInspector)
	os.Exit(runner.Run(context.Background(), os.Args[1:]))
}
