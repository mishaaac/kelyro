package sqlite

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/artifacts"
	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/platform"
)

var fixedTime = time.Date(2026, time.August, 12, 15, 30, 0, 123, time.UTC)

func TestTimestampEncodingPreservesTextOrderAndReadsLegacyPrecision(t *testing.T) {
	earlier := time.Date(2026, time.August, 24, 0, 0, 0, 100_000_000, time.UTC)
	later := earlier.Add(time.Nanosecond)
	encodedEarlier := earlier.Format(timestampFormat)
	encodedLater := later.Format(timestampFormat)
	if encodedEarlier >= encodedLater {
		t.Fatalf("encoded timestamps are not ordered: %q >= %q", encodedEarlier, encodedLater)
	}

	legacy := earlier.Format(time.RFC3339Nano)
	parsed, err := parseTimestamp(legacy)
	if err != nil {
		t.Fatalf("parse legacy timestamp %q: %v", legacy, err)
	}
	if !parsed.Equal(earlier) {
		t.Fatalf("parsed legacy timestamp = %s, want %s", parsed, earlier)
	}
}

func TestOpenCreatesAndMigratesNewDatabase(t *testing.T) {
	database, root := openTestDatabase(t)

	expectedPath, err := platform.WorkspaceDBPath(root)
	if err != nil {
		t.Fatalf("WorkspaceDBPath() error = %v", err)
	}
	if database.Path() != expectedPath {
		t.Fatalf("Path() = %q, want %q", database.Path(), expectedPath)
	}
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("database file was not created: %v", err)
	}

	version, err := database.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("SchemaVersion() error = %v", err)
	}
	if version != LatestSchemaVersion() {
		t.Fatalf("SchemaVersion() = %d, want %d", version, LatestSchemaVersion())
	}

	wantTables := []string{
		"achievement_definitions",
		"analytics_snapshots",
		"app_state",
		"artifact_index",
		"audit_events",
		"authority_profiles",
		"citations",
		"claim_sources",
		"claims",
		"concept_registry",
		"curriculum_definition_fingerprints",
		"curriculum_edges",
		"curriculum_instances",
		"curriculum_nodes",
		"daily_plan_item_concepts",
		"daily_plan_items",
		"daily_plans",
		"deprecation_records",
		"diagnostic_attempts",
		"diagnostic_observations",
		"drift_reports",
		"evidence",
		"freshness_state",
		"impact_reports",
		"learner_curriculum_concept_states",
		"learner_curriculum_instances",
		"learner_setups",
		"learning_evidence",
		"learning_goals",
		"mastery_threshold_settings",
		"milestones",
		"mistake_events",
		"mistakes",
		"onboarding_interviews",
		"release_records",
		"research_cache_entries",
		"research_runs",
		"research_topics",
		"retention_state",
		"review_items",
		"review_schedule",
		"schema_migrations",
		"source_aliases",
		"source_bundle_items",
		"source_bundles",
		"source_conflicts",
		"source_registry_entries",
		"source_snapshots",
		"sources",
		"streak_state",
		"student_achievements",
		"student_concept_states",
		"student_preferences",
		"student_preferred_days",
		"student_profiles",
		"students",
		"study_activities",
		"study_activity_concepts",
		"study_history_events",
		"study_session_lifecycle",
		"study_sessions",
		"trust_registry",
		"verification_results",
		"workspace_meta",
	}
	rows, err := database.sql.QueryContext(context.Background(), `
SELECT name FROM sqlite_master
WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
ORDER BY name`)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	var gotTables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			t.Fatalf("scan table: %v", err)
		}
		gotTables = append(gotTables, name)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close table rows: %v", err)
	}
	if !slices.Equal(gotTables, wantTables) {
		t.Fatalf("tables = %v, want %v", gotTables, wantTables)
	}

	var foreignKeys int
	if err := database.sql.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
	if err := database.CheckArtifactIndex(context.Background()); err != nil {
		t.Fatalf("CheckArtifactIndex() error = %v", err)
	}
}

func TestMigrateRepeatedDoesNotReapply(t *testing.T) {
	database, _ := openTestDatabase(t)

	var beforeCount int
	var beforeAppliedAt string
	if err := database.sql.QueryRowContext(context.Background(), `
SELECT COUNT(*), MAX(applied_at) FROM schema_migrations`,
	).Scan(&beforeCount, &beforeAppliedAt); err != nil {
		t.Fatalf("read migration history before repeat: %v", err)
	}

	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var afterCount int
	var afterAppliedAt string
	if err := database.sql.QueryRowContext(context.Background(), `
SELECT COUNT(*), MAX(applied_at) FROM schema_migrations`,
	).Scan(&afterCount, &afterAppliedAt); err != nil {
		t.Fatalf("read migration history after repeat: %v", err)
	}
	if beforeCount != afterCount || beforeAppliedAt != afterAppliedAt {
		t.Fatalf("repeated migration changed history: before=(%d, %q), after=(%d, %q)",
			beforeCount, beforeAppliedAt, afterCount, afterAppliedAt)
	}
}

func TestSnapshotValidatorChecksIntegrityAndMigrationHistoryReadOnly(t *testing.T) {
	database, _ := openTestDatabase(t)
	validator := SnapshotValidator{}
	version, err := validator.Validate(context.Background(), database.Path())
	if err != nil || version != LatestSchemaVersion() {
		t.Fatalf("Validate() = (%d, %v), want schema %d", version, err, LatestSchemaVersion())
	}
	if _, err := database.sql.ExecContext(context.Background(),
		"UPDATE schema_migrations SET checksum = ? WHERE version = 1", strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := validator.Validate(context.Background(), database.Path()); !errors.Is(err, ErrMigrationHistory) {
		t.Fatalf("Validate(tampered) error = %v, want ErrMigrationHistory", err)
	}
}

func TestMigrationRollsBackAndReportsFailingStatement(t *testing.T) {
	database, _ := openTestDatabase(t)
	migrations := append([]migration(nil), foundationMigrations...)
	migrations = append(migrations, migration{
		version: LatestSchemaVersion() + 1,
		name:    "broken migration",
		statements: []string{
			"CREATE TABLE should_rollback (id INTEGER PRIMARY KEY)",
			"INSERT INTO table_that_does_not_exist (id) VALUES (1)",
		},
	})

	err := database.migrate(context.Background(), migrations)
	if err == nil {
		t.Fatal("migrate() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("migration %d (broken migration), statement 2", LatestSchemaVersion()+1)) {
		t.Fatalf("migrate() error = %q, want migration and statement context", err)
	}

	version, err := database.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("SchemaVersion() error = %v", err)
	}
	if version != LatestSchemaVersion() {
		t.Fatalf("SchemaVersion() = %d after rollback, want %d", version, LatestSchemaVersion())
	}
	if tableExists(t, database, "should_rollback") {
		t.Fatal("table from failed migration exists; DDL was not rolled back")
	}
}

func TestMigrationHistoryDetectsPublishedMigrationChanges(t *testing.T) {
	database, _ := openTestDatabase(t)

	if _, err := database.sql.ExecContext(context.Background(), `
UPDATE schema_migrations SET checksum = ? WHERE version = 1`, strings.Repeat("0", 64)); err != nil {
		t.Fatalf("tamper migration history: %v", err)
	}

	err := database.Migrate(context.Background())
	if !errors.Is(err, ErrMigrationHistory) {
		t.Fatalf("Migrate() error = %v, want ErrMigrationHistory", err)
	}
}

func TestDestructiveMigrationRequiresSuccessfulBackup(t *testing.T) {
	database, _ := openTestDatabase(t)
	migrations := destructiveTestMigrations()

	err := database.migrate(context.Background(), migrations)
	if !errors.Is(err, ErrBackupRequired) {
		t.Fatalf("migrate() error = %v, want ErrBackupRequired", err)
	}
	if tableExists(t, database, "destructive_result") {
		t.Fatal("destructive migration ran without a backup")
	}

	backupCalled := false
	database.backup = func(_ context.Context, path string, info MigrationInfo) error {
		backupCalled = true
		if path != database.Path() {
			t.Fatalf("backup path = %q, want %q", path, database.Path())
		}
		if info != (MigrationInfo{Version: LatestSchemaVersion() + 1, Name: "destructive test"}) {
			t.Fatalf("backup migration = %+v", info)
		}
		if tableExists(t, database, "destructive_result") {
			t.Fatal("migration SQL ran before backup callback")
		}
		return nil
	}
	if err := database.migrate(context.Background(), migrations); err != nil {
		t.Fatalf("migrate() with backup error = %v", err)
	}
	if !backupCalled {
		t.Fatal("backup callback was not called")
	}
	if !tableExists(t, database, "destructive_result") {
		t.Fatal("destructive migration did not run after backup")
	}
}

func TestRepositoriesCRUD(t *testing.T) {
	database, _ := openTestDatabase(t)
	repositories := database.Repositories()
	ctx := context.Background()

	if err := repositories.State.Set(ctx, "ui", "last_view", []byte("roadmap")); err != nil {
		t.Fatalf("State.Set() error = %v", err)
	}
	value, found, err := repositories.State.Get(ctx, "ui", "last_view")
	if err != nil || !found || string(value) != "roadmap" {
		t.Fatalf("State.Get() = (%q, %v, %v), want (roadmap, true, nil)", value, found, err)
	}
	if err := repositories.State.Set(ctx, "ui", "last_view", []byte("status")); err != nil {
		t.Fatalf("State.Set() update error = %v", err)
	}
	value, found, err = repositories.State.Get(ctx, "ui", "last_view")
	if err != nil || !found || string(value) != "status" {
		t.Fatalf("State.Get() after update = (%q, %v, %v)", value, found, err)
	}
	if err := repositories.State.Delete(ctx, "ui", "last_view"); err != nil {
		t.Fatalf("State.Delete() error = %v", err)
	}
	if _, found, err := repositories.State.Get(ctx, "ui", "last_view"); err != nil || found {
		t.Fatalf("State.Get() after delete = (found %v, error %v)", found, err)
	}

	if err := repositories.WorkspaceMeta.Set(ctx, "workspace_id", []byte("workspace-123")); err != nil {
		t.Fatalf("WorkspaceMeta.Set() error = %v", err)
	}
	value, found, err = repositories.WorkspaceMeta.Get(ctx, "workspace_id")
	if err != nil || !found || string(value) != "workspace-123" {
		t.Fatalf("WorkspaceMeta.Get() = (%q, %v, %v)", value, found, err)
	}
	if err := repositories.WorkspaceMeta.Delete(ctx, "workspace_id"); err != nil {
		t.Fatalf("WorkspaceMeta.Delete() error = %v", err)
	}

	wantArtifact := artifacts.Artifact{
		Path:            "LEARNING.md",
		Ownership:       artifacts.SystemGeneratedHumanReadable,
		CreatedBy:       "kelyro.test",
		ContentHash:     artifacts.Hash([]byte("content")),
		CreatedAt:       fixedTime.Add(-time.Minute),
		LastGeneratedAt: fixedTime,
		ExpectedVersion: "learning/v1",
	}
	if err := repositories.Artifacts.Put(ctx, wantArtifact); err != nil {
		t.Fatalf("Artifacts.Put() error = %v", err)
	}
	gotArtifact, found, err := repositories.Artifacts.Get(ctx, wantArtifact.Path)
	if err != nil || !found || gotArtifact != wantArtifact {
		t.Fatalf("Artifacts.Get() = (%+v, %v, %v), want (%+v, true, nil)", gotArtifact, found, err, wantArtifact)
	}
	if err := repositories.Artifacts.Delete(ctx, wantArtifact.Path); err != nil {
		t.Fatalf("Artifacts.Delete() error = %v", err)
	}

	event := audit.Event{
		Name:     "workspace.opened",
		Actor:    audit.ActorUser,
		Subject:  "workspace-123",
		Metadata: map[string]string{"source": "test"},
	}
	if err := repositories.Audit.Record(ctx, event); err != nil {
		t.Fatalf("Audit.Record() error = %v", err)
	}
	var occurredAt, action, actor, subject, metadata, appVersion string
	if err := database.sql.QueryRowContext(ctx, `
SELECT occurred_at, action, actor, subject, metadata_json, app_version
FROM audit_events WHERE action = 'workspace.opened'`,
	).Scan(&occurredAt, &action, &actor, &subject, &metadata, &appVersion); err != nil {
		t.Fatalf("read audit event: %v", err)
	}
	if occurredAt != fixedTime.Format(timestampFormat) || action != event.Name || actor != string(event.Actor) || subject != event.Subject || metadata != `{"source":"test"}` || appVersion != "unknown" {
		t.Fatalf("stored audit event = (%q, %q, %q, %q, %q, %q)", occurredAt, action, actor, subject, metadata, appVersion)
	}
	entries, err := repositories.Audit.List(ctx)
	if err != nil {
		t.Fatalf("Audit.List() error = %v", err)
	}
	if got := entries[len(entries)-1]; got.Event != event.Name || got.Actor != event.Actor || got.AppVersion != "unknown" || got.Metadata["source"] != "test" {
		t.Fatalf("Audit.List() last entry = %+v", got)
	}
}

func TestWithTransactionCommitsAndRollsBackRepositoriesTogether(t *testing.T) {
	database, _ := openTestDatabase(t)
	ctx := context.Background()

	if err := database.WithTransaction(ctx, func(repositories Repositories) error {
		if err := repositories.State.Set(ctx, "transaction", "committed", []byte("yes")); err != nil {
			return err
		}
		return repositories.WorkspaceMeta.Set(ctx, "committed", []byte("yes"))
	}); err != nil {
		t.Fatalf("WithTransaction() commit error = %v", err)
	}
	if _, found, err := database.Repositories().State.Get(ctx, "transaction", "committed"); err != nil || !found {
		t.Fatalf("committed state = (found %v, error %v)", found, err)
	}

	wantErr := errors.New("cancel transaction")
	err := database.WithTransaction(ctx, func(repositories Repositories) error {
		if err := repositories.State.Set(ctx, "transaction", "rolled_back", []byte("no")); err != nil {
			return err
		}
		if err := repositories.WorkspaceMeta.Set(ctx, "rolled_back", []byte("no")); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WithTransaction() error = %v, want %v", err, wantErr)
	}
	if _, found, err := database.Repositories().State.Get(ctx, "transaction", "rolled_back"); err != nil || found {
		t.Fatalf("rolled-back state = (found %v, error %v)", found, err)
	}
	if _, found, err := database.Repositories().WorkspaceMeta.Get(ctx, "rolled_back"); err != nil || found {
		t.Fatalf("rolled-back metadata = (found %v, error %v)", found, err)
	}
}

func TestDatabaseInstancesAreIsolated(t *testing.T) {
	first, _ := openTestDatabase(t)
	second, _ := openTestDatabase(t)
	ctx := context.Background()

	if err := first.Repositories().State.Set(ctx, "isolation", "key", []byte("first")); err != nil {
		t.Fatalf("first State.Set() error = %v", err)
	}
	if _, found, err := second.Repositories().State.Get(ctx, "isolation", "key"); err != nil || found {
		t.Fatalf("second State.Get() = (found %v, error %v), want absent", found, err)
	}
}

func TestCloseReleasesDatabaseForReopen(t *testing.T) {
	root := newWorkspaceRoot(t)
	database, err := Open(context.Background(), root)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(context.Background(), root)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestCancelledContextStopsRepositoryOperation(t *testing.T) {
	database, _ := openTestDatabase(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := database.Repositories().State.Set(ctx, "context", "cancelled", []byte("value"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("State.Set() error = %v, want context.Canceled", err)
	}
}

func TestOpenRejectsCorruptDatabase(t *testing.T) {
	root := newWorkspaceRoot(t)
	databasePath, err := platform.WorkspaceDBPath(root)
	if err != nil {
		t.Fatalf("WorkspaceDBPath() error = %v", err)
	}
	original := []byte("this is not a SQLite database")
	if err := os.WriteFile(databasePath, original, 0o600); err != nil {
		t.Fatalf("write corrupt database: %v", err)
	}

	database, err := Open(context.Background(), root)
	if database != nil {
		_ = database.Close()
		t.Fatal("Open() returned a database for corrupt input")
	}
	if !errors.Is(err, ErrIntegrity) {
		t.Fatalf("Open() error = %v, want ErrIntegrity", err)
	}
	got, readErr := os.ReadFile(databasePath)
	if readErr != nil {
		t.Fatalf("read corrupt database after Open(): %v", readErr)
	}
	if string(got) != string(original) {
		t.Fatal("Open() silently replaced the corrupt database")
	}
}

func TestOpenRejectsSymlinkedDatabaseAndInternalDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks may require elevated Windows privileges")
	}

	t.Run("database", func(t *testing.T) {
		root := newWorkspaceRoot(t)
		databasePath, _ := platform.WorkspaceDBPath(root)
		target := filepath.Join(t.TempDir(), "outside.db")
		if err := os.WriteFile(target, []byte("outside"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, databasePath); err != nil {
			t.Fatal(err)
		}
		if database, err := Open(context.Background(), root); err == nil || database != nil {
			t.Fatalf("Open() = (%v, %v), want symlink rejection", database, err)
		}
	})

	t.Run("internal directory", func(t *testing.T) {
		root := t.TempDir()
		target := t.TempDir()
		internal, _ := platform.WorkspaceInternalDir(root)
		if err := os.Symlink(target, internal); err != nil {
			t.Fatal(err)
		}
		if database, err := Open(context.Background(), root); err == nil || database != nil {
			t.Fatalf("Open() = (%v, %v), want symlink rejection", database, err)
		}
	})
}

func TestOpenReportsLockedDatabaseWithoutMisclassifyingCorruption(t *testing.T) {
	database, root := openTestDatabase(t)
	if _, err := database.sql.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatalf("BEGIN EXCLUSIVE error = %v", err)
	}

	locked, err := Open(context.Background(), root, WithOperationTimeout(100*time.Millisecond))
	if locked != nil {
		_ = locked.Close()
		t.Fatal("Open() returned a database while an exclusive lock was held")
	}
	if err == nil || errors.Is(err, ErrIntegrity) {
		t.Fatalf("Open(locked) error = %v, want operational lock error", err)
	}
	if _, rollbackErr := database.sql.ExecContext(context.Background(), "ROLLBACK"); rollbackErr != nil {
		t.Fatalf("ROLLBACK error = %v", rollbackErr)
	}

	reopened, err := Open(context.Background(), root)
	if err != nil {
		t.Fatalf("Open() after releasing lock error = %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestDatabaseURIHandlesSpacesAndPragmas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace with spaces", "learning.db")
	dsn := databaseURI(path, 1500*time.Millisecond)
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", dsn, err)
	}
	if parsed.Scheme != "file" {
		t.Fatalf("database URI scheme = %q, want file", parsed.Scheme)
	}
	wantPath := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" && !strings.HasPrefix(wantPath, "/") {
		wantPath = "/" + wantPath
	}
	if parsed.Path != wantPath {
		t.Fatalf("database URI path = %q, want %q", parsed.Path, wantPath)
	}
	wantPragmas := []string{"foreign_keys(1)", "busy_timeout(1500)"}
	if got := parsed.Query()["_pragma"]; !slices.Equal(got, wantPragmas) {
		t.Fatalf("database URI pragmas = %v, want %v", got, wantPragmas)
	}
}

func TestRepositoryValidationRejectsEmptyKeysAndInvalidOwnership(t *testing.T) {
	database, _ := openTestDatabase(t)
	repositories := database.Repositories()
	ctx := context.Background()

	if err := repositories.State.Set(ctx, "", "key", []byte("value")); err == nil {
		t.Fatal("State.Set() accepted an empty namespace")
	}
	if err := repositories.WorkspaceMeta.Set(ctx, " ", []byte("value")); err == nil {
		t.Fatal("WorkspaceMeta.Set() accepted an empty key")
	}
	if err := repositories.Artifacts.Put(ctx, artifacts.Artifact{Path: "x", Ownership: "unknown"}); err == nil {
		t.Fatal("Artifacts.Put() accepted invalid ownership")
	}
	if err := repositories.Audit.Record(ctx, audit.Event{Name: "", Actor: audit.ActorUser, Subject: "x"}); err == nil {
		t.Fatal("Audit.Record() accepted an empty action")
	}
}

func openTestDatabase(t *testing.T) (*Database, string) {
	t.Helper()
	root := newWorkspaceRoot(t)
	database, err := Open(context.Background(), root, withClock(func() time.Time { return fixedTime }))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return database, root
}

func newWorkspaceRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	internal, err := platform.WorkspaceInternalDir(root)
	if err != nil {
		t.Fatalf("WorkspaceInternalDir() error = %v", err)
	}
	if err := os.MkdirAll(internal, 0o755); err != nil {
		t.Fatalf("create workspace internals: %v", err)
	}
	return root
}

func tableExists(t *testing.T, database *Database, name string) bool {
	t.Helper()
	var count int
	if err := database.sql.QueryRowContext(context.Background(), `
SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count); err != nil {
		t.Fatalf("query table %s: %v", name, err)
	}
	return count == 1
}

func destructiveTestMigrations() []migration {
	migrations := append([]migration(nil), foundationMigrations...)
	return append(migrations, migration{
		version:     LatestSchemaVersion() + 1,
		name:        "destructive test",
		destructive: true,
		statements:  []string{"CREATE TABLE destructive_result (id INTEGER PRIMARY KEY)"},
	})
}
