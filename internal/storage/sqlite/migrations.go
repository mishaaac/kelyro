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
	{
		version: 5,
		name:    "learner profile settings",
		statements: []string{
			`ALTER TABLE student_profiles ADD COLUMN preferred_display_name TEXT CHECK (preferred_display_name IS NULL OR length(trim(preferred_display_name)) > 0)`,
			`ALTER TABLE student_profiles ADD COLUMN preferred_language TEXT NOT NULL DEFAULT 'en' CHECK (length(trim(preferred_language)) > 0)`,
			`ALTER TABLE student_profiles ADD COLUMN daily_minutes INTEGER NOT NULL DEFAULT 30 CHECK (daily_minutes BETWEEN 5 AND 1440)`,
			`ALTER TABLE student_profiles ADD COLUMN weekly_days_target INTEGER NOT NULL DEFAULT 5 CHECK (weekly_days_target BETWEEN 1 AND 7)`,
			`ALTER TABLE student_profiles ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC' CHECK (length(trim(timezone)) > 0)`,
			`UPDATE student_profiles
SET preferred_display_name = display_name,
    weekly_days_target = CASE
        WHEN (SELECT COUNT(*) FROM student_preferred_days d WHERE d.student_id = student_profiles.student_id) BETWEEN 1 AND 7
        THEN (SELECT COUNT(*) FROM student_preferred_days d WHERE d.student_id = student_profiles.student_id)
        ELSE 5
    END`,
			`UPDATE student_profiles
SET daily_minutes = MIN(1440, MAX(5, CAST(
    (weekly_minutes + weekly_days_target - 1) / weekly_days_target AS INTEGER
)))`,
		},
	},
	{
		version: 6,
		name:    "learning goal lifecycle",
		statements: []string{
			`ALTER TABLE learning_goals ADD COLUMN description TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE learning_goals ADD COLUMN domain TEXT NOT NULL DEFAULT 'General' CHECK (length(trim(domain)) > 0)`,
			`ALTER TABLE learning_goals ADD COLUMN target_outcome TEXT NOT NULL DEFAULT 'Continue learning' CHECK (length(trim(target_outcome)) > 0)`,
			`ALTER TABLE learning_goals ADD COLUMN starting_level TEXT NOT NULL DEFAULT 'novice' CHECK (starting_level IN ('novice', 'beginner', 'intermediate', 'advanced'))`,
			`ALTER TABLE learning_goals ADD COLUMN activated_at TEXT CHECK (activated_at IS NULL OR activated_at GLOB '*Z')`,
			`ALTER TABLE learning_goals ADD COLUMN completed_at TEXT CHECK (completed_at IS NULL OR completed_at GLOB '*Z')`,
			`UPDATE learning_goals
SET target_outcome = title,
    activated_at = CASE WHEN status IN ('active', 'paused', 'completed') THEN updated_at ELSE NULL END,
    completed_at = CASE WHEN status = 'completed' THEN updated_at ELSE NULL END`,
			`UPDATE learning_goals AS goal
SET status = 'paused'
WHERE status = 'active'
  AND id <> (
      SELECT candidate.id
      FROM learning_goals AS candidate
      WHERE candidate.student_id = goal.student_id AND candidate.status = 'active'
      ORDER BY candidate.updated_at DESC, candidate.id DESC
      LIMIT 1
  )`,
			`CREATE UNIQUE INDEX learning_goals_one_active_idx ON learning_goals (student_id) WHERE status = 'active'`,
		},
	},
	{
		version: 7,
		name:    "resumable onboarding",
		statements: []string{
			`CREATE TABLE onboarding_interviews (
    student_id TEXT PRIMARY KEY,
    flow_id TEXT NOT NULL CHECK (length(trim(flow_id)) > 0),
    flow_version TEXT NOT NULL CHECK (length(trim(flow_version)) > 0),
    status TEXT NOT NULL CHECK (status IN ('not_started', 'in_progress', 'completed', 'cancelled')),
    current_question_id TEXT NOT NULL DEFAULT '',
    answers_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL CHECK (created_at GLOB '*Z'),
    updated_at TEXT NOT NULL CHECK (updated_at GLOB '*Z'),
    completed_at TEXT CHECK (completed_at IS NULL OR completed_at GLOB '*Z'),
    cancelled_at TEXT CHECK (cancelled_at IS NULL OR cancelled_at GLOB '*Z'),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    CHECK (updated_at >= created_at),
    CHECK (
        (status = 'not_started' AND current_question_id = '' AND completed_at IS NULL AND cancelled_at IS NULL) OR
        (status = 'in_progress' AND length(current_question_id) > 0 AND completed_at IS NULL AND cancelled_at IS NULL) OR
        (status = 'completed' AND current_question_id = '' AND completed_at IS NOT NULL AND cancelled_at IS NULL) OR
        (status = 'cancelled' AND current_question_id = '' AND completed_at IS NULL AND cancelled_at IS NOT NULL)
    )
)`,
		},
	},
	{
		version: 8,
		name:    "mastery threshold policy",
		statements: []string{
			`CREATE TABLE mastery_threshold_settings (
    student_id TEXT PRIMARY KEY,
    policy_version TEXT NOT NULL CHECK (policy_version = 'threshold-v1'),
    student_default REAL NOT NULL CHECK (student_default BETWEEN 0.50 AND 0.99),
    workspace_override REAL CHECK (workspace_override IS NULL OR workspace_override BETWEEN 0.50 AND 0.99),
    updated_at TEXT NOT NULL CHECK (updated_at GLOB '*Z'),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE
)`,
			`INSERT INTO mastery_threshold_settings (student_id, policy_version, student_default, workspace_override, updated_at)
SELECT student.id,
       'threshold-v1',
       COALESCE((
           SELECT CASE WHEN goal.mastery_threshold BETWEEN 0.50 AND 0.99 THEN goal.mastery_threshold END
           FROM learning_goals AS goal
           WHERE goal.student_id = student.id AND goal.status = 'active'
           ORDER BY goal.updated_at DESC, goal.id DESC
           LIMIT 1
       ), 0.80),
       NULL,
       student.updated_at
FROM students AS student`,
		},
	},
	{
		version: 9,
		name:    "learner curriculum instances",
		statements: []string{
			`CREATE TABLE curriculum_definition_fingerprints (
    curriculum_id TEXT NOT NULL,
    curriculum_version TEXT NOT NULL,
    fingerprint TEXT NOT NULL CHECK (length(fingerprint) = 71 AND fingerprint GLOB 'sha256:*'),
    PRIMARY KEY (curriculum_id, curriculum_version),
    FOREIGN KEY (curriculum_id, curriculum_version) REFERENCES curriculum_instances(id, version) ON DELETE CASCADE
)`,
			`CREATE TABLE learner_curriculum_instances (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    student_id TEXT NOT NULL,
    goal_id TEXT NOT NULL,
    curriculum_id TEXT NOT NULL,
    curriculum_version TEXT NOT NULL,
    source_kind TEXT NOT NULL CHECK (source_kind IN ('fixture', 'import', 'pack')),
    status TEXT NOT NULL CHECK (status IN ('active', 'paused', 'completed', 'archived')),
    created_at TEXT NOT NULL CHECK (created_at GLOB '*Z'),
    updated_at TEXT NOT NULL CHECK (updated_at GLOB '*Z'),
    UNIQUE (id, student_id),
    UNIQUE (student_id, goal_id, curriculum_id, curriculum_version),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (goal_id, student_id) REFERENCES learning_goals(id, student_id) ON DELETE CASCADE,
    FOREIGN KEY (curriculum_id, curriculum_version) REFERENCES curriculum_instances(id, version),
    CHECK (updated_at >= created_at)
)`,
			`CREATE INDEX learner_curriculum_instances_student_idx
ON learner_curriculum_instances (student_id, status, created_at, id)`,
			`CREATE TABLE learner_curriculum_concept_states (
    curriculum_instance_id TEXT NOT NULL,
    student_id TEXT NOT NULL,
    concept_id TEXT NOT NULL,
    exposure TEXT NOT NULL CHECK (exposure IN ('not_seen', 'introduced', 'learning', 'practicing', 'mastered', 'review_due')),
    mastery REAL NOT NULL CHECK (mastery BETWEEN 0 AND 1),
    first_seen_at TEXT CHECK (first_seen_at IS NULL OR first_seen_at GLOB '*Z'),
    last_seen_at TEXT CHECK (last_seen_at IS NULL OR last_seen_at GLOB '*Z'),
    mastered_at TEXT CHECK (mastered_at IS NULL OR mastered_at GLOB '*Z'),
    review_due_at TEXT CHECK (review_due_at IS NULL OR review_due_at GLOB '*Z'),
    manual_flags_json TEXT NOT NULL DEFAULT '[]',
    updated_at TEXT NOT NULL CHECK (updated_at GLOB '*Z'),
    PRIMARY KEY (curriculum_instance_id, concept_id),
    FOREIGN KEY (curriculum_instance_id, student_id) REFERENCES learner_curriculum_instances(id, student_id) ON DELETE CASCADE,
    FOREIGN KEY (concept_id) REFERENCES concept_registry(id),
    CHECK ((first_seen_at IS NULL) = (last_seen_at IS NULL)),
    CHECK (first_seen_at IS NULL OR last_seen_at >= first_seen_at),
    CHECK (last_seen_at IS NULL OR updated_at >= last_seen_at),
    CHECK (mastered_at IS NULL OR (first_seen_at IS NOT NULL AND mastered_at >= first_seen_at AND mastered_at <= last_seen_at)),
    CHECK (review_due_at IS NULL OR first_seen_at IS NOT NULL),
    CHECK ((exposure = 'not_seen' AND first_seen_at IS NULL AND mastered_at IS NULL AND review_due_at IS NULL) OR
           (exposure <> 'not_seen' AND first_seen_at IS NOT NULL)),
    CHECK (exposure NOT IN ('mastered', 'review_due') OR mastered_at IS NOT NULL),
    CHECK (exposure <> 'review_due' OR review_due_at IS NOT NULL)
)`,
			`CREATE INDEX learner_curriculum_concept_states_exposure_idx
ON learner_curriculum_concept_states (curriculum_instance_id, exposure, concept_id)`,
		},
	},
	{
		version: 10,
		name:    "deterministic diagnostics",
		statements: []string{
			`CREATE TABLE diagnostic_attempts (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    student_id TEXT NOT NULL,
    curriculum_instance_id TEXT NOT NULL,
    diagnostic_id TEXT NOT NULL CHECK (length(diagnostic_id) > 0),
    diagnostic_version TEXT NOT NULL CHECK (length(trim(diagnostic_version)) > 0),
    definition_fingerprint TEXT NOT NULL CHECK (length(definition_fingerprint) = 71 AND definition_fingerprint GLOB 'sha256:*'),
    status TEXT NOT NULL CHECK (status IN ('in_progress', 'completed', 'skipped')),
    started_at TEXT NOT NULL CHECK (started_at GLOB '*Z'),
    updated_at TEXT NOT NULL CHECK (updated_at GLOB '*Z'),
    completed_at TEXT CHECK (completed_at IS NULL OR completed_at GLOB '*Z'),
    skipped_at TEXT CHECK (skipped_at IS NULL OR skipped_at GLOB '*Z'),
    UNIQUE (student_id, curriculum_instance_id, diagnostic_id, diagnostic_version),
    FOREIGN KEY (curriculum_instance_id, student_id) REFERENCES learner_curriculum_instances(id, student_id) ON DELETE CASCADE,
    CHECK (updated_at >= started_at),
    CHECK (
        (status = 'in_progress' AND completed_at IS NULL AND skipped_at IS NULL) OR
        (status = 'completed' AND completed_at IS NOT NULL AND skipped_at IS NULL AND completed_at = updated_at) OR
        (status = 'skipped' AND completed_at IS NULL AND skipped_at IS NOT NULL AND skipped_at = updated_at)
    )
)`,
			`CREATE INDEX diagnostic_attempts_student_status_idx
ON diagnostic_attempts (student_id, status, updated_at, id)`,
			`CREATE TABLE diagnostic_observations (
    attempt_id TEXT NOT NULL,
    item_id TEXT NOT NULL CHECK (length(item_id) > 0),
    concept_id TEXT NOT NULL,
    evidence_id TEXT NOT NULL,
    score REAL NOT NULL CHECK (score BETWEEN 0 AND 1),
    answered_at TEXT NOT NULL CHECK (answered_at GLOB '*Z'),
    position INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY (attempt_id, item_id),
    UNIQUE (attempt_id, position),
    UNIQUE (evidence_id),
    FOREIGN KEY (attempt_id) REFERENCES diagnostic_attempts(id) ON DELETE CASCADE,
    FOREIGN KEY (concept_id) REFERENCES concept_registry(id),
    FOREIGN KEY (evidence_id) REFERENCES learning_evidence(id)
)`,
			`CREATE INDEX diagnostic_observations_concept_idx
ON diagnostic_observations (attempt_id, concept_id, position)`,
		},
	},
	{
		version: 11,
		name:    "integrated learner setup",
		statements: []string{
			`CREATE UNIQUE INDEX diagnostic_attempts_setup_owner_idx
ON diagnostic_attempts (id, student_id, curriculum_instance_id)`,
			`CREATE TABLE learner_setups (
    student_id TEXT PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('awaiting_onboarding', 'awaiting_diagnostic', 'initializing', 'completed')),
    curriculum_instance_id TEXT,
    diagnostic_attempt_id TEXT,
    diagnostic_opt_in INTEGER NOT NULL CHECK (diagnostic_opt_in IN (0, 1)),
    created_at TEXT NOT NULL CHECK (created_at GLOB '*Z'),
    updated_at TEXT NOT NULL CHECK (updated_at GLOB '*Z'),
    setup_completed_at TEXT CHECK (setup_completed_at IS NULL OR setup_completed_at GLOB '*Z'),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (curriculum_instance_id, student_id) REFERENCES learner_curriculum_instances(id, student_id),
    FOREIGN KEY (diagnostic_attempt_id, student_id, curriculum_instance_id)
        REFERENCES diagnostic_attempts(id, student_id, curriculum_instance_id),
    CHECK (updated_at >= created_at),
    CHECK (
        (status = 'awaiting_onboarding' AND curriculum_instance_id IS NULL AND diagnostic_attempt_id IS NULL AND diagnostic_opt_in = 0 AND setup_completed_at IS NULL) OR
        (status = 'awaiting_diagnostic' AND curriculum_instance_id IS NOT NULL AND diagnostic_attempt_id IS NOT NULL AND diagnostic_opt_in = 1 AND setup_completed_at IS NULL) OR
        (status = 'initializing' AND curriculum_instance_id IS NOT NULL AND
            ((diagnostic_opt_in = 1 AND diagnostic_attempt_id IS NOT NULL) OR (diagnostic_opt_in = 0 AND diagnostic_attempt_id IS NULL)) AND setup_completed_at IS NULL) OR
        (status = 'completed' AND curriculum_instance_id IS NOT NULL AND
            ((diagnostic_opt_in = 1 AND diagnostic_attempt_id IS NOT NULL) OR (diagnostic_opt_in = 0 AND diagnostic_attempt_id IS NULL)) AND
            setup_completed_at IS NOT NULL AND setup_completed_at = updated_at)
    )
)`,
			`CREATE INDEX learner_setups_status_idx ON learner_setups (status, updated_at, student_id)`,
		},
	},
	{
		version: 12,
		name:    "evidence metadata for mastery v1",
		statements: []string{
			`ALTER TABLE learning_evidence ADD COLUMN mastery_evidence_type TEXT NOT NULL DEFAULT 'manual_import'
CHECK (mastery_evidence_type IN ('diagnostic_objective', 'diagnostic_self_report', 'knowledge_check', 'practice_success', 'practice_failure', 'assessment', 'project_evidence', 'review_recall', 'manual_import'))`,
			`ALTER TABLE learning_evidence ADD COLUMN confidence REAL NOT NULL DEFAULT 1 CHECK (confidence > 0 AND confidence <= 1)`,
			`ALTER TABLE learning_evidence ADD COLUMN independence REAL NOT NULL DEFAULT 1 CHECK (independence BETWEEN 0 AND 1)`,
			`ALTER TABLE learning_evidence ADD COLUMN difficulty REAL NOT NULL DEFAULT 0.5 CHECK (difficulty BETWEEN 0 AND 1)`,
			`ALTER TABLE learning_evidence ADD COLUMN algorithm_version TEXT NOT NULL DEFAULT 'legacy-evidence/v1' CHECK (length(trim(algorithm_version)) > 0)`,
			`UPDATE learning_evidence
SET mastery_evidence_type = CASE evidence_type
    WHEN 'diagnostic' THEN 'diagnostic_objective'
    WHEN 'practice' THEN CASE WHEN score = 0 THEN 'practice_failure' ELSE 'practice_success' END
    WHEN 'assessment' THEN 'assessment'
    WHEN 'review' THEN 'review_recall'
    WHEN 'observation' THEN 'project_evidence'
    WHEN 'import' THEN 'manual_import'
END`,
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
