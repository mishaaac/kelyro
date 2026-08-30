package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/artifacts"
	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/platform"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/storage"
	sqliteDriver "modernc.org/sqlite"
)

const defaultOperationTimeout = 5 * time.Second

var (
	// ErrIntegrity reports that SQLite found an invalid or corrupt database.
	ErrIntegrity = errors.New("SQLite integrity check failed")
	// ErrMigrationHistory reports that applied migrations do not match the
	// immutable migrations shipped by this build.
	ErrMigrationHistory = errors.New("SQLite migration history is inconsistent")
	// ErrBackupRequired prevents a destructive migration from running without
	// a successful backup callback.
	ErrBackupRequired = errors.New("backup is required before destructive migration")
)

// MigrationInfo identifies a migration for a backup provider without exposing
// its SQL statements.
type MigrationInfo struct {
	Version int
	Name    string
}

// BackupFunc creates and confirms a backup before a destructive migration.
type BackupFunc func(ctx context.Context, databasePath string, migration MigrationInfo) error

type options struct {
	timeout time.Duration
	now     func() time.Time
	backup  BackupFunc
	version string
}

// Option configures a Database.
type Option func(*options)

// WithOperationTimeout sets the maximum duration of an individual database
// operation. A caller's earlier context deadline always wins.
func WithOperationTimeout(timeout time.Duration) Option {
	return func(options *options) {
		options.timeout = timeout
	}
}

// WithDestructiveMigrationBackup installs the callback that future destructive
// migrations must complete before any migration SQL starts.
func WithDestructiveMigrationBackup(backup BackupFunc) Option {
	return func(options *options) {
		options.backup = backup
	}
}

// WithAppVersion supplies the build version stored with audit events emitted
// by this database instance.
func WithAppVersion(version string) Option {
	return func(options *options) {
		options.version = strings.TrimSpace(version)
	}
}

func withClock(now func() time.Time) Option {
	return func(options *options) {
		options.now = now
	}
}

// Repositories groups persistence contracts backed by the same database or
// transaction.
type Repositories struct {
	State         storage.StateStore
	WorkspaceMeta storage.WorkspaceMetaStore
	Artifacts     artifacts.Index
	Audit         audit.Trail
	Research      researchapp.Repositories
}

// Database owns one workspace-local SQLite connection pool. It is not a global
// singleton and must be closed by its caller.
type Database struct {
	sql     *sql.DB
	path    string
	timeout time.Duration
	now     func() time.Time
	backup  BackupFunc
	version string
}

// Open opens the workspace's .kelyro/learning.db, checks its integrity, and
// applies all pending migrations.
func Open(ctx context.Context, workspaceRoot string, configured ...Option) (*Database, error) {
	settings := options{
		timeout: defaultOperationTimeout,
		now:     time.Now,
		version: "unknown",
	}
	for _, configure := range configured {
		if configure != nil {
			configure(&settings)
		}
	}
	if settings.timeout <= 0 {
		return nil, errors.New("SQLite operation timeout must be positive")
	}
	if settings.now == nil {
		return nil, errors.New("SQLite clock must not be nil")
	}

	databasePath, err := platform.WorkspaceDBPath(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace database path: %w", err)
	}
	parent := filepath.Dir(databasePath)
	info, err := os.Lstat(parent)
	if err != nil {
		return nil, fmt.Errorf("inspect workspace database directory %s: %w", parent, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("workspace database directory %s is not a regular directory", parent)
	}
	if databaseInfo, statErr := os.Lstat(databasePath); statErr == nil {
		if !databaseInfo.Mode().IsRegular() || databaseInfo.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("workspace database %s is not a regular file", databasePath)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect workspace database %s: %w", databasePath, statErr)
	}

	handle, err := sql.Open("sqlite", databaseURI(databasePath, settings.timeout))
	if err != nil {
		return nil, fmt.Errorf("open workspace database: %w", err)
	}
	handle.SetMaxOpenConns(1)
	handle.SetMaxIdleConns(1)

	database := &Database{
		sql:     handle,
		path:    databasePath,
		timeout: settings.timeout,
		now:     settings.now,
		backup:  settings.backup,
		version: settings.version,
	}
	opened := false
	defer func() {
		if !opened {
			_ = handle.Close()
		}
	}()

	operationContext, cancel := database.operationContext(ctx)
	if err := handle.PingContext(operationContext); err != nil {
		cancel()
		return nil, sqliteOperationError("connect to workspace database", err)
	}
	cancel()

	if err := os.Chmod(databasePath, 0o600); err != nil {
		return nil, fmt.Errorf("restrict workspace database permissions: %w", err)
	}
	if err := database.checkForeignKeys(ctx); err != nil {
		return nil, err
	}
	if err := database.checkIntegrity(ctx); err != nil {
		return nil, err
	}
	if err := database.Migrate(ctx); err != nil {
		return nil, err
	}
	if err := database.checkStudentCoreIntegrity(ctx); err != nil {
		return nil, err
	}

	opened = true
	return database, nil
}

// Path returns the normalized workspace database path.
func (database *Database) Path() string {
	return database.path
}

// Close releases the database connection pool.
func (database *Database) Close() error {
	if database == nil || database.sql == nil {
		return nil
	}
	return database.sql.Close()
}

// Repositories returns adapters that execute independent atomic statements.
func (database *Database) Repositories() Repositories {
	return newRepositories(database.sql, database.timeout, database.now, database.version)
}

// CheckArtifactIndex verifies that the Foundation artifact index exists and is
// readable without exposing SQL details to diagnostics callers.
func (database *Database) CheckArtifactIndex(ctx context.Context) error {
	operationContext, cancel := database.operationContext(ctx)
	defer cancel()

	var count int
	if err := database.sql.QueryRowContext(operationContext, "SELECT COUNT(*) FROM artifact_index").Scan(&count); err != nil {
		return fmt.Errorf("check artifact index: %w", err)
	}
	return nil
}

// WithTransaction runs repository work in one transaction. Returning an error
// from work rolls back every write.
func (database *Database) WithTransaction(ctx context.Context, work func(Repositories) error) error {
	if work == nil {
		return errors.New("SQLite transaction callback must not be nil")
	}

	operationContext, cancel := database.operationContext(ctx)
	defer cancel()

	transaction, err := database.sql.BeginTx(operationContext, nil)
	if err != nil {
		return fmt.Errorf("begin SQLite transaction: %w", err)
	}

	if err := work(newRepositories(transaction, database.timeout, database.now, database.version)); err != nil {
		if rollbackErr := transaction.Rollback(); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback SQLite transaction: %w", rollbackErr))
		}
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit SQLite transaction: %w", err)
	}

	return nil
}

func (database *Database) operationContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, database.timeout)
}

func (database *Database) checkForeignKeys(ctx context.Context) error {
	operationContext, cancel := database.operationContext(ctx)
	defer cancel()

	var enabled int
	if err := database.sql.QueryRowContext(operationContext, "PRAGMA foreign_keys").Scan(&enabled); err != nil {
		return fmt.Errorf("verify SQLite foreign keys: %w", err)
	}
	if enabled != 1 {
		return errors.New("SQLite foreign keys are not enabled")
	}
	return nil
}

func (database *Database) checkIntegrity(ctx context.Context) error {
	operationContext, cancel := database.operationContext(ctx)
	defer cancel()

	rows, err := database.sql.QueryContext(operationContext, "PRAGMA quick_check")
	if err != nil {
		return sqliteOperationError("check SQLite integrity", err)
	}
	defer rows.Close()

	checked := false
	for rows.Next() {
		checked = true
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("%w: read result: %v", ErrIntegrity, err)
		}
		if result != "ok" {
			return fmt.Errorf("%w: %s", ErrIntegrity, result)
		}
	}
	if err := rows.Err(); err != nil {
		return sqliteOperationError("check SQLite integrity", err)
	}
	if !checked {
		return fmt.Errorf("%w: no result", ErrIntegrity)
	}
	return nil
}

func (database *Database) checkStudentCoreIntegrity(ctx context.Context) error {
	operationContext, cancel := database.operationContext(ctx)
	defer cancel()
	if err := checkRelationalIntegrity(operationContext, database.sql); err != nil {
		return err
	}
	return checkStudentCoreIntegrity(operationContext, database.sql, LatestSchemaVersion())
}

func sqliteOperationError(operation string, err error) error {
	var driverErr *sqliteDriver.Error
	if errors.As(err, &driverErr) {
		switch driverErr.Code() & 0xff {
		case 11, 26: // SQLITE_CORRUPT and SQLITE_NOTADB, including extended codes.
			return fmt.Errorf("%w: %s: %v", ErrIntegrity, operation, err)
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func databaseURI(path string, timeout time.Duration) string {
	slashed := filepath.ToSlash(path)
	databaseURL := url.URL{Scheme: "file"}
	if strings.HasPrefix(slashed, "//") {
		parts := strings.SplitN(strings.TrimPrefix(slashed, "//"), "/", 2)
		databaseURL.Host = parts[0]
		if len(parts) == 2 {
			databaseURL.Path = "/" + parts[1]
		}
	} else {
		if filepath.VolumeName(path) != "" && !strings.HasPrefix(slashed, "/") {
			slashed = "/" + slashed
		}
		databaseURL.Path = slashed
	}

	busyMilliseconds := max(timeout.Milliseconds(), 1)
	query := databaseURL.Query()
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "busy_timeout("+strconv.FormatInt(busyMilliseconds, 10)+")")
	databaseURL.RawQuery = query.Encode()
	return databaseURL.String()
}
