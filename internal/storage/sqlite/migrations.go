package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mishaaac/kelyro/internal/audit"
)

const migrationTableSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY CHECK (version > 0),
    name TEXT NOT NULL CHECK (length(name) > 0),
    checksum TEXT NOT NULL CHECK (length(checksum) = 64),
    applied_at TEXT NOT NULL
)`

type migration struct {
	version     int
	name        string
	statements  []string
	destructive bool
}

var foundationMigrations = []migration{
	{
		version: 1,
		name:    "foundation repositories",
		statements: []string{
			`CREATE TABLE workspace_meta (
    key TEXT PRIMARY KEY CHECK (length(key) > 0),
    value BLOB NOT NULL,
    updated_at TEXT NOT NULL
)`,
			`CREATE TABLE app_state (
    namespace TEXT NOT NULL CHECK (length(namespace) > 0),
    key TEXT NOT NULL CHECK (length(key) > 0),
    value BLOB NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (namespace, key)
)`,
			`CREATE TABLE artifact_index (
    path TEXT PRIMARY KEY CHECK (length(path) > 0),
    ownership TEXT NOT NULL CHECK (
        ownership IN ('machine-owned', 'system-generated-human-readable', 'student-owned')
    ),
    updated_at TEXT NOT NULL
)`,
			`CREATE TABLE audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at TEXT NOT NULL,
    action TEXT NOT NULL CHECK (length(action) > 0),
    subject TEXT NOT NULL CHECK (length(subject) > 0),
    metadata_json TEXT NOT NULL
)`,
			`CREATE INDEX audit_events_occurred_at_idx ON audit_events (occurred_at, id)`,
		},
	},
	{
		version: 2,
		name:    "artifact integrity metadata",
		statements: []string{
			`ALTER TABLE artifact_index ADD COLUMN created_by TEXT NOT NULL DEFAULT 'legacy'`,
			`ALTER TABLE artifact_index ADD COLUMN content_hash TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE artifact_index ADD COLUMN created_at TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE artifact_index ADD COLUMN last_generated_at TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE artifact_index ADD COLUMN expected_version TEXT NOT NULL DEFAULT ''`,
			`UPDATE artifact_index
SET created_at = updated_at,
    last_generated_at = updated_at
WHERE created_at = '' OR last_generated_at = ''`,
		},
	},
	{
		version: 3,
		name:    "structured audit identity",
		statements: []string{
			`ALTER TABLE audit_events ADD COLUMN actor TEXT NOT NULL DEFAULT 'system' CHECK (actor IN ('system', 'user', 'plugin'))`,
			`ALTER TABLE audit_events ADD COLUMN app_version TEXT NOT NULL DEFAULT 'unknown' CHECK (length(app_version) > 0)`,
		},
	},
	{
		version: 4,
		name:    "student learning core",
		statements: []string{
			`CREATE TABLE students (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    created_at TEXT NOT NULL CHECK (created_at GLOB '*Z'),
    updated_at TEXT NOT NULL CHECK (updated_at GLOB '*Z'),
    CHECK (updated_at >= created_at)
)`,
			`CREATE TABLE student_profiles (
    student_id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL CHECK (length(trim(display_name)) > 0),
    experience TEXT NOT NULL CHECK (experience IN ('novice', 'beginner', 'intermediate', 'advanced')),
    weekly_minutes INTEGER NOT NULL CHECK (weekly_minutes > 0),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE
)`,
			`CREATE TABLE student_preferences (
    student_id TEXT NOT NULL,
    preference TEXT NOT NULL CHECK (preference IN ('theory_first', 'practice', 'projects', 'reflection')),
    position INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY (student_id, preference),
    UNIQUE (student_id, position),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE
)`,
			`CREATE TABLE student_preferred_days (
    student_id TEXT NOT NULL,
    weekday INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    position INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY (student_id, weekday),
    UNIQUE (student_id, position),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE
)`,
			`CREATE TABLE learning_goals (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    student_id TEXT NOT NULL,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    status TEXT NOT NULL CHECK (status IN ('draft', 'active', 'paused', 'completed', 'archived')),
    mastery_threshold REAL NOT NULL CHECK (mastery_threshold BETWEEN 0 AND 1),
    created_at TEXT NOT NULL CHECK (created_at GLOB '*Z'),
    updated_at TEXT NOT NULL CHECK (updated_at GLOB '*Z'),
    UNIQUE (id, student_id),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    CHECK (updated_at >= created_at)
)`,
			`CREATE INDEX learning_goals_student_status_idx ON learning_goals (student_id, status, updated_at DESC)`,
			`CREATE INDEX learning_goals_active_idx ON learning_goals (student_id, updated_at DESC) WHERE status = 'active'`,
			`CREATE TABLE curriculum_instances (
    id TEXT NOT NULL CHECK (length(id) > 0),
    version TEXT NOT NULL CHECK (length(trim(version)) > 0),
    PRIMARY KEY (id, version)
)`,
			`CREATE TABLE concept_registry (
    id TEXT PRIMARY KEY CHECK (length(id) > 0)
)`,
			`CREATE TABLE curriculum_nodes (
    curriculum_id TEXT NOT NULL,
    curriculum_version TEXT NOT NULL,
    node_id TEXT NOT NULL CHECK (length(node_id) > 0),
    node_type TEXT NOT NULL CHECK (node_type IN ('phase', 'module', 'lesson', 'topic', 'concept')),
    parent_node_id TEXT,
    concept_id TEXT,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    position INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
    PRIMARY KEY (curriculum_id, curriculum_version, node_id),
    FOREIGN KEY (curriculum_id, curriculum_version) REFERENCES curriculum_instances(id, version) ON DELETE CASCADE,
    FOREIGN KEY (curriculum_id, curriculum_version, parent_node_id) REFERENCES curriculum_nodes(curriculum_id, curriculum_version, node_id) ON DELETE CASCADE,
    FOREIGN KEY (concept_id) REFERENCES concept_registry(id),
    CHECK ((node_type = 'concept' AND concept_id = node_id) OR (node_type <> 'concept' AND concept_id IS NULL))
)`,
			`CREATE INDEX curriculum_nodes_concept_idx ON curriculum_nodes (curriculum_id, curriculum_version, concept_id) WHERE node_type = 'concept'`,
			`CREATE INDEX curriculum_nodes_parent_idx ON curriculum_nodes (curriculum_id, curriculum_version, parent_node_id, position)`,
			`CREATE TABLE curriculum_edges (
    curriculum_id TEXT NOT NULL,
    curriculum_version TEXT NOT NULL,
    concept_id TEXT NOT NULL,
    required_concept_id TEXT NOT NULL,
    PRIMARY KEY (curriculum_id, curriculum_version, concept_id, required_concept_id),
    FOREIGN KEY (curriculum_id, curriculum_version, concept_id) REFERENCES curriculum_nodes(curriculum_id, curriculum_version, node_id) ON DELETE CASCADE,
    FOREIGN KEY (curriculum_id, curriculum_version, required_concept_id) REFERENCES curriculum_nodes(curriculum_id, curriculum_version, node_id) ON DELETE CASCADE,
    CHECK (concept_id <> required_concept_id)
)`,
			`CREATE INDEX curriculum_edges_required_idx ON curriculum_edges (curriculum_id, curriculum_version, required_concept_id)`,
			`CREATE TABLE student_concept_states (
    student_id TEXT NOT NULL,
    concept_id TEXT NOT NULL,
    exposure TEXT NOT NULL CHECK (exposure IN ('not_seen', 'introduced', 'learning', 'practicing', 'mastered', 'review_due')),
    mastery REAL NOT NULL CHECK (mastery BETWEEN 0 AND 1),
    introduced_at TEXT CHECK (introduced_at IS NULL OR introduced_at GLOB '*Z'),
    updated_at TEXT NOT NULL CHECK (updated_at GLOB '*Z'),
    PRIMARY KEY (student_id, concept_id),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (concept_id) REFERENCES concept_registry(id),
    CHECK ((exposure = 'not_seen' AND introduced_at IS NULL) OR (exposure <> 'not_seen' AND introduced_at IS NOT NULL)),
    CHECK (introduced_at IS NULL OR updated_at >= introduced_at)
)`,
			`CREATE INDEX student_concept_states_lookup_idx ON student_concept_states (student_id, exposure, concept_id)`,
			`CREATE TABLE learning_evidence (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    student_id TEXT NOT NULL,
    concept_id TEXT NOT NULL,
    evidence_type TEXT NOT NULL CHECK (evidence_type IN ('diagnostic', 'practice', 'assessment', 'review', 'observation', 'import')),
    source TEXT NOT NULL CHECK (length(trim(source)) > 0),
    score REAL NOT NULL CHECK (score BETWEEN 0 AND 1),
    observed_at TEXT NOT NULL CHECK (observed_at GLOB '*Z'),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (concept_id) REFERENCES concept_registry(id)
)`,
			`CREATE INDEX learning_evidence_concept_idx ON learning_evidence (student_id, concept_id, observed_at, id)`,
			`CREATE TABLE mistakes (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    student_id TEXT NOT NULL,
    concept_id TEXT NOT NULL,
    description TEXT NOT NULL CHECK (length(trim(description)) > 0),
    occurred_at TEXT NOT NULL CHECK (occurred_at GLOB '*Z'),
    resolved_at TEXT CHECK (resolved_at IS NULL OR resolved_at GLOB '*Z'),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (concept_id) REFERENCES concept_registry(id),
    CHECK (resolved_at IS NULL OR resolved_at >= occurred_at)
)`,
			`CREATE INDEX mistakes_concept_idx ON mistakes (student_id, concept_id, occurred_at, id)`,
			`CREATE TABLE retention_state (
    student_id TEXT NOT NULL,
    concept_id TEXT NOT NULL,
    strength REAL NOT NULL CHECK (strength BETWEEN 0 AND 1),
    measured_at TEXT NOT NULL CHECK (measured_at GLOB '*Z'),
    PRIMARY KEY (student_id, concept_id),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (concept_id) REFERENCES concept_registry(id)
)`,
			`CREATE TABLE study_sessions (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    student_id TEXT NOT NULL,
    goal_id TEXT NOT NULL,
    started_at TEXT NOT NULL CHECK (started_at GLOB '*Z'),
    ended_at TEXT NOT NULL CHECK (ended_at GLOB '*Z'),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (goal_id, student_id) REFERENCES learning_goals(id, student_id),
    CHECK (started_at < ended_at)
)`,
			`CREATE INDEX study_sessions_goal_timeline_idx ON study_sessions (student_id, goal_id, started_at, id)`,
			`CREATE INDEX study_sessions_range_idx ON study_sessions (student_id, started_at, ended_at)`,
			`CREATE TABLE study_activities (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    session_id TEXT NOT NULL,
    activity_type TEXT NOT NULL CHECK (activity_type IN ('theory', 'practice', 'assessment', 'review', 'reflection')),
    started_at TEXT NOT NULL CHECK (started_at GLOB '*Z'),
    ended_at TEXT NOT NULL CHECK (ended_at GLOB '*Z'),
    position INTEGER NOT NULL CHECK (position >= 0),
    FOREIGN KEY (session_id) REFERENCES study_sessions(id) ON DELETE CASCADE,
    CHECK (started_at < ended_at)
)`,
			`CREATE TABLE study_activity_concepts (
    activity_id TEXT NOT NULL,
    concept_id TEXT NOT NULL,
    position INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY (activity_id, concept_id),
    UNIQUE (activity_id, position),
    FOREIGN KEY (activity_id) REFERENCES study_activities(id) ON DELETE CASCADE,
    FOREIGN KEY (concept_id) REFERENCES concept_registry(id)
)`,
			`CREATE TABLE review_schedule (
    student_id TEXT NOT NULL,
    concept_id TEXT NOT NULL,
    introduced_at TEXT CHECK (introduced_at IS NULL OR introduced_at GLOB '*Z'),
    due_at TEXT NOT NULL CHECK (due_at GLOB '*Z'),
    imported INTEGER NOT NULL CHECK (imported IN (0, 1)),
    PRIMARY KEY (student_id, concept_id),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (concept_id) REFERENCES concept_registry(id),
    CHECK (imported = 1 OR (introduced_at IS NOT NULL AND due_at >= introduced_at))
)`,
			`CREATE INDEX review_schedule_due_idx ON review_schedule (student_id, due_at, concept_id)`,
			`CREATE TABLE review_items (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    student_id TEXT NOT NULL,
    concept_id TEXT NOT NULL,
    due_at TEXT NOT NULL CHECK (due_at GLOB '*Z'),
    status TEXT NOT NULL CHECK (status IN ('pending', 'completed', 'skipped')),
    completed_at TEXT CHECK (completed_at IS NULL OR completed_at GLOB '*Z'),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (concept_id) REFERENCES concept_registry(id),
    CHECK ((status = 'completed' AND completed_at IS NOT NULL) OR (status <> 'completed' AND completed_at IS NULL))
)`,
			`CREATE INDEX review_items_due_idx ON review_items (student_id, status, due_at, id)`,
			`CREATE TABLE streak_state (
    student_id TEXT PRIMARY KEY,
    current_days INTEGER NOT NULL CHECK (current_days >= 0),
    longest_days INTEGER NOT NULL CHECK (longest_days >= current_days),
    last_study_at TEXT CHECK (last_study_at IS NULL OR last_study_at GLOB '*Z'),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    CHECK (current_days = 0 OR last_study_at IS NOT NULL)
)`,
			`CREATE TABLE achievement_definitions (
    key TEXT PRIMARY KEY CHECK (length(key) > 0),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0)
)`,
			`CREATE TABLE student_achievements (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    student_id TEXT NOT NULL,
    achievement_key TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('locked', 'unlocked')),
    unlocked_at TEXT CHECK (unlocked_at IS NULL OR unlocked_at GLOB '*Z'),
    UNIQUE (student_id, achievement_key),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (achievement_key) REFERENCES achievement_definitions(key),
    CHECK ((status = 'unlocked' AND unlocked_at IS NOT NULL) OR (status = 'locked' AND unlocked_at IS NULL))
)`,
			`CREATE TABLE milestones (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    student_id TEXT NOT NULL,
    goal_id TEXT NOT NULL,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    reached_at TEXT NOT NULL CHECK (reached_at GLOB '*Z'),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (goal_id, student_id) REFERENCES learning_goals(id, student_id)
)`,
			`CREATE INDEX milestones_goal_timeline_idx ON milestones (student_id, goal_id, reached_at, id)`,
			`CREATE TABLE analytics_snapshots (
    student_id TEXT NOT NULL,
    captured_at TEXT NOT NULL CHECK (captured_at GLOB '*Z'),
    study_minutes INTEGER NOT NULL CHECK (study_minutes >= 0),
    sessions_completed INTEGER NOT NULL CHECK (sessions_completed >= 0),
    concepts_introduced INTEGER NOT NULL CHECK (concepts_introduced >= 0),
    concepts_mastered INTEGER NOT NULL CHECK (concepts_mastered BETWEEN 0 AND concepts_introduced),
    reviews_due INTEGER NOT NULL CHECK (reviews_due >= 0),
    PRIMARY KEY (student_id, captured_at),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE
)`,
			`CREATE INDEX analytics_snapshots_latest_idx ON analytics_snapshots (student_id, captured_at DESC)`,
			`CREATE TABLE daily_plans (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    student_id TEXT NOT NULL,
    goal_id TEXT NOT NULL,
    plan_date TEXT NOT NULL CHECK (plan_date GLOB '*Z'),
    created_at TEXT NOT NULL CHECK (created_at GLOB '*Z'),
    UNIQUE (student_id, goal_id, plan_date),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (goal_id, student_id) REFERENCES learning_goals(id, student_id)
)`,
			`CREATE TABLE daily_plan_items (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    plan_id TEXT NOT NULL,
    item_type TEXT NOT NULL CHECK (item_type IN ('learn', 'practice', 'review', 'reflect')),
    estimated_minutes INTEGER NOT NULL CHECK (estimated_minutes > 0),
    position INTEGER NOT NULL CHECK (position >= 0),
    UNIQUE (plan_id, position),
    FOREIGN KEY (plan_id) REFERENCES daily_plans(id) ON DELETE CASCADE
)`,
			`CREATE TABLE daily_plan_item_concepts (
    item_id TEXT NOT NULL,
    concept_id TEXT NOT NULL,
    position INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY (item_id, concept_id),
    UNIQUE (item_id, position),
    FOREIGN KEY (item_id) REFERENCES daily_plan_items(id) ON DELETE CASCADE,
    FOREIGN KEY (concept_id) REFERENCES concept_registry(id)
)`,
		},
	},
}

// LatestSchemaVersion returns the newest migration version embedded in this
// build.
func LatestSchemaVersion() int {
	if len(foundationMigrations) == 0 {
		return 0
	}
	return foundationMigrations[len(foundationMigrations)-1].version
}

// Migrate validates the applied history and transactionally applies every
// pending migration.
func (database *Database) Migrate(ctx context.Context) error {
	return database.migrate(ctx, foundationMigrations)
}

// SchemaVersion returns the greatest successfully applied migration version.
func (database *Database) SchemaVersion(ctx context.Context) (int, error) {
	operationContext, cancel := database.operationContext(ctx)
	defer cancel()

	var version int
	if err := database.sql.QueryRowContext(operationContext,
		"SELECT COALESCE(MAX(version), 0) FROM schema_migrations",
	).Scan(&version); err != nil {
		return 0, fmt.Errorf("read SQLite schema version: %w", err)
	}
	return version, nil
}

func (database *Database) migrate(ctx context.Context, migrations []migration) error {
	if err := validateMigrations(migrations); err != nil {
		return err
	}

	operationContext, cancel := database.operationContext(ctx)
	defer cancel()

	if _, err := database.sql.ExecContext(operationContext, migrationTableSQL); err != nil {
		return fmt.Errorf("initialize SQLite migration history: %w", err)
	}

	applied, err := loadAppliedMigrations(operationContext, database.sql)
	if err != nil {
		return err
	}
	if err := validateAppliedMigrations(applied, migrations); err != nil {
		return err
	}

	for _, next := range migrations {
		if _, exists := applied[next.version]; exists {
			continue
		}
		if next.destructive {
			if database.backup == nil {
				return fmt.Errorf("%w: migration %d (%s)", ErrBackupRequired, next.version, next.name)
			}
			if err := database.backup(operationContext, database.path, MigrationInfo{
				Version: next.version,
				Name:    next.name,
			}); err != nil {
				return fmt.Errorf("backup before migration %d (%s): %w", next.version, next.name, err)
			}
		}
		if err := database.applyMigration(operationContext, next); err != nil {
			return err
		}
		if next.version >= 3 {
			if err := database.Repositories().Audit.Record(operationContext, audit.Event{
				Name:    "migration.applied",
				Actor:   audit.ActorSystem,
				Subject: "workspace-database",
				Metadata: map[string]string{
					"version": strconv.Itoa(next.version),
					"name":    next.name,
				},
			}); err != nil {
				return fmt.Errorf("audit migration %d (%s): %w", next.version, next.name, err)
			}
		}
	}

	return nil
}

func (database *Database) applyMigration(ctx context.Context, next migration) error {
	transaction, err := database.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d (%s): %w", next.version, next.name, err)
	}
	defer transaction.Rollback()

	for statementIndex, statement := range next.statements {
		if _, err := transaction.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migration %d (%s), statement %d: %w", next.version, next.name, statementIndex+1, err)
		}
	}
	if _, err := transaction.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, ?)",
		next.version,
		next.name,
		migrationChecksum(next),
		database.now().UTC().Format(timestampFormat),
	); err != nil {
		return fmt.Errorf("record migration %d (%s): %w", next.version, next.name, err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit migration %d (%s): %w", next.version, next.name, err)
	}
	return nil
}

type appliedMigration struct {
	name     string
	checksum string
}

func loadAppliedMigrations(ctx context.Context, database *sql.DB) (map[int]appliedMigration, error) {
	rows, err := database.QueryContext(ctx,
		"SELECT version, name, checksum FROM schema_migrations ORDER BY version",
	)
	if err != nil {
		return nil, fmt.Errorf("read SQLite migration history: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]appliedMigration)
	for rows.Next() {
		var version int
		var record appliedMigration
		if err := rows.Scan(&version, &record.name, &record.checksum); err != nil {
			return nil, fmt.Errorf("scan SQLite migration history: %w", err)
		}
		applied[version] = record
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SQLite migration history: %w", err)
	}
	return applied, nil
}

func validateMigrations(migrations []migration) error {
	for index, candidate := range migrations {
		expectedVersion := index + 1
		switch {
		case candidate.version != expectedVersion:
			return fmt.Errorf("migration definitions must be consecutive from 1: expected %d, got %d", expectedVersion, candidate.version)
		case strings.TrimSpace(candidate.name) == "":
			return fmt.Errorf("migration %d has an empty name", candidate.version)
		case len(candidate.statements) == 0:
			return fmt.Errorf("migration %d (%s) has no statements", candidate.version, candidate.name)
		}
	}
	return nil
}

func validateAppliedMigrations(applied map[int]appliedMigration, migrations []migration) error {
	versions := make([]int, 0, len(applied))
	for version := range applied {
		versions = append(versions, version)
	}
	sort.Ints(versions)

	for index, version := range versions {
		if version != index+1 || version > len(migrations) {
			return fmt.Errorf("%w: unexpected applied version %d", ErrMigrationHistory, version)
		}
		expected := migrations[version-1]
		record := applied[version]
		if record.name != expected.name || record.checksum != migrationChecksum(expected) {
			return fmt.Errorf("%w: migration %d (%s) was modified", ErrMigrationHistory, version, record.name)
		}
	}
	return nil
}

func migrationChecksum(candidate migration) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(strconv.Itoa(candidate.version)))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(candidate.name))
	if candidate.destructive {
		_, _ = digest.Write([]byte{1})
	} else {
		_, _ = digest.Write([]byte{0})
	}
	for _, statement := range candidate.statements {
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(statement))
	}
	return hex.EncodeToString(digest.Sum(nil))
}
