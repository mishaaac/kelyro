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
	{
		version: 13,
		name:    "persistent mistake memory",
		statements: []string{
			`ALTER TABLE mistakes ADD COLUMN mistake_key TEXT NOT NULL DEFAULT 'legacy'
CHECK (length(trim(mistake_key)) BETWEEN 1 AND 128)`,
			`ALTER TABLE mistakes ADD COLUMN category TEXT NOT NULL DEFAULT 'unknown'
CHECK (category IN ('conceptual', 'syntax', 'procedure', 'misconception', 'careless', 'tooling', 'unknown'))`,
			`ALTER TABLE mistakes ADD COLUMN summary TEXT NOT NULL DEFAULT 'legacy'
CHECK (length(trim(summary)) BETWEEN 1 AND 500)`,
			`ALTER TABLE mistakes ADD COLUMN first_seen_at TEXT NOT NULL DEFAULT '1970-01-01T00:00:00Z'
CHECK (first_seen_at GLOB '*Z')`,
			`ALTER TABLE mistakes ADD COLUMN last_seen_at TEXT NOT NULL DEFAULT '1970-01-01T00:00:00Z'
CHECK (last_seen_at GLOB '*Z')`,
			`ALTER TABLE mistakes ADD COLUMN occurrences INTEGER NOT NULL DEFAULT 1 CHECK (occurrences > 0)`,
			`ALTER TABLE mistakes ADD COLUMN status TEXT NOT NULL DEFAULT 'recent'
CHECK (status IN ('recent', 'reinforced', 'resolved'))`,
			`ALTER TABLE mistakes ADD COLUMN source_ref TEXT NOT NULL DEFAULT 'legacy:migration/v13'
CHECK (length(trim(source_ref)) BETWEEN 1 AND 256)`,
			`UPDATE mistakes
SET mistake_key = CASE
        WHEN length('legacy:' || id) <= 128 THEN 'legacy:' || id
        ELSE 'legacy-row:' || rowid
    END,
    summary = substr(trim(description), 1, 500),
    first_seen_at = occurred_at,
    last_seen_at = occurred_at,
    status = CASE WHEN resolved_at IS NULL THEN 'recent' ELSE 'resolved' END`,
			`CREATE UNIQUE INDEX mistakes_dedupe_idx
ON mistakes (student_id, concept_id, mistake_key)`,
			`CREATE INDEX mistakes_student_recent_idx
ON mistakes (student_id, last_seen_at DESC, id)`,
			`CREATE TRIGGER mistakes_memory_insert_guard
BEFORE INSERT ON mistakes
WHEN length(trim(NEW.summary)) NOT BETWEEN 1 AND 500
  OR NEW.first_seen_at > NEW.last_seen_at
  OR (NEW.status = 'resolved') <> (NEW.resolved_at IS NOT NULL)
  OR (NEW.resolved_at IS NOT NULL AND NEW.resolved_at < NEW.last_seen_at)
BEGIN SELECT RAISE(ABORT, 'invalid mistake memory aggregate'); END`,
			`CREATE TRIGGER mistakes_memory_update_guard
BEFORE UPDATE ON mistakes
WHEN length(trim(NEW.summary)) NOT BETWEEN 1 AND 500
  OR NEW.first_seen_at > NEW.last_seen_at
  OR (NEW.status = 'resolved') <> (NEW.resolved_at IS NOT NULL)
  OR (NEW.resolved_at IS NOT NULL AND NEW.resolved_at < NEW.last_seen_at)
BEGIN SELECT RAISE(ABORT, 'invalid mistake memory aggregate'); END`,
			`CREATE TABLE mistake_events (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    mistake_id TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('observed', 'reinforced', 'resolved')),
    occurred_at TEXT NOT NULL CHECK (occurred_at GLOB '*Z'),
    source_ref TEXT NOT NULL CHECK (length(trim(source_ref)) BETWEEN 1 AND 256),
    FOREIGN KEY (mistake_id) REFERENCES mistakes(id) ON DELETE CASCADE
)`,
			`CREATE INDEX mistake_events_history_idx
ON mistake_events (mistake_id, occurred_at, id)`,
			`INSERT INTO mistake_events (id, mistake_id, event_type, occurred_at, source_ref)
SELECT 'mistake-event.legacy.observed.' || id, id, 'observed', occurred_at, 'legacy:migration/v13'
FROM mistakes`,
			`INSERT INTO mistake_events (id, mistake_id, event_type, occurred_at, source_ref)
SELECT 'mistake-event.legacy.resolved.' || id, id, 'resolved', resolved_at, 'legacy:migration/v13'
FROM mistakes WHERE resolved_at IS NOT NULL`,
		},
	},
	{
		version: 14,
		name:    "persistent study session lifecycle",
		statements: []string{
			`CREATE UNIQUE INDEX learner_curriculum_instances_session_parent_idx
ON learner_curriculum_instances (id, student_id, goal_id)`,
			`CREATE TABLE study_session_lifecycle (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    student_id TEXT NOT NULL,
    goal_id TEXT NOT NULL,
    curriculum_instance_id TEXT NOT NULL,
    started_at TEXT NOT NULL CHECK (started_at GLOB '*Z'),
    ended_at TEXT CHECK (ended_at IS NULL OR ended_at GLOB '*Z'),
    last_activity_at TEXT NOT NULL CHECK (last_activity_at GLOB '*Z'),
    status TEXT NOT NULL CHECK (status IN ('active', 'completed', 'interrupted', 'recovered')),
    active_duration_ns INTEGER NOT NULL CHECK (active_duration_ns >= 0),
    activity_count INTEGER NOT NULL CHECK (activity_count >= 0),
    policy_version TEXT NOT NULL CHECK (policy_version = 'study-session-v1'),
    idle_timeout_ns INTEGER NOT NULL CHECK (idle_timeout_ns > 0),
    FOREIGN KEY (curriculum_instance_id, student_id, goal_id)
        REFERENCES learner_curriculum_instances(id, student_id, goal_id),
    CHECK (last_activity_at >= started_at),
    CHECK (ended_at IS NULL OR ended_at >= last_activity_at),
    CHECK ((status = 'active' AND ended_at IS NULL) OR
           (status <> 'active' AND ended_at IS NOT NULL))
)`,
			`CREATE UNIQUE INDEX study_session_lifecycle_one_active_idx
ON study_session_lifecycle (student_id) WHERE status = 'active'`,
			`CREATE INDEX study_session_lifecycle_goal_timeline_idx
ON study_session_lifecycle (student_id, goal_id, started_at, id)`,
		},
	},
	{
		version: 15,
		name:    "study history timeline",
		statements: []string{
			`CREATE TABLE study_history_events (
    id TEXT PRIMARY KEY CHECK (length(id) > 0),
    student_id TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN (
        'onboarding.completed','diagnostic.completed','concept.introduced','evidence.recorded',
        'concept.mastered','review.completed','session.completed','achievement.unlocked'
    )),
    source_id TEXT NOT NULL CHECK (length(source_id) > 0),
    occurred_at TEXT NOT NULL CHECK (occurred_at GLOB '*Z'),
    goal_id TEXT,
    curriculum_instance_id TEXT,
    concept_id TEXT,
    history_version TEXT NOT NULL CHECK (history_version = 'study-history-v1'),
    UNIQUE (student_id, event_type, source_id),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (goal_id, student_id) REFERENCES learning_goals(id, student_id),
    FOREIGN KEY (curriculum_instance_id, student_id, goal_id)
        REFERENCES learner_curriculum_instances(id, student_id, goal_id),
    FOREIGN KEY (concept_id) REFERENCES concept_registry(id),
    CHECK (curriculum_instance_id IS NULL OR goal_id IS NOT NULL),
    CHECK (event_type <> 'diagnostic.completed' OR curriculum_instance_id IS NOT NULL),
    CHECK (event_type NOT IN ('concept.introduced','evidence.recorded','concept.mastered','review.completed') OR concept_id IS NOT NULL)
)`,
			`CREATE INDEX study_history_events_timeline_idx
ON study_history_events (student_id, occurred_at DESC, id DESC)`,
			`CREATE INDEX study_history_events_concept_idx
ON study_history_events (student_id, concept_id, occurred_at DESC) WHERE concept_id IS NOT NULL`,
			`INSERT INTO study_history_events
(id,student_id,event_type,source_id,occurred_at,history_version)
SELECT 'history.legacy.onboarding.' || rowid, student_id, 'onboarding.completed',
       'onboarding.legacy.' || rowid, completed_at, 'study-history-v1'
FROM onboarding_interviews WHERE status = 'completed'`,
			`INSERT INTO study_history_events
(id,student_id,event_type,source_id,occurred_at,goal_id,curriculum_instance_id,history_version)
SELECT 'history.legacy.diagnostic.' || attempt.rowid, attempt.student_id, 'diagnostic.completed',
       'diagnostic.legacy.' || attempt.rowid, attempt.completed_at, instance.goal_id,
       attempt.curriculum_instance_id, 'study-history-v1'
FROM diagnostic_attempts AS attempt
JOIN learner_curriculum_instances AS instance ON instance.id = attempt.curriculum_instance_id
WHERE attempt.status = 'completed'`,
			`INSERT INTO study_history_events
(id,student_id,event_type,source_id,occurred_at,concept_id,history_version)
SELECT 'history.legacy.evidence.' || rowid, student_id, 'evidence.recorded',
       'evidence.legacy.' || rowid, observed_at, concept_id, 'study-history-v1'
FROM learning_evidence`,
			`INSERT INTO study_history_events
(id,student_id,event_type,source_id,occurred_at,goal_id,curriculum_instance_id,concept_id,history_version)
SELECT 'history.legacy.introduced.' || state.rowid, state.student_id, 'concept.introduced',
       'concept-introduced.legacy.' || state.rowid, state.first_seen_at, instance.goal_id,
       state.curriculum_instance_id, state.concept_id, 'study-history-v1'
FROM learner_curriculum_concept_states AS state
JOIN learner_curriculum_instances AS instance ON instance.id = state.curriculum_instance_id
WHERE state.first_seen_at IS NOT NULL`,
			`INSERT INTO study_history_events
(id,student_id,event_type,source_id,occurred_at,goal_id,curriculum_instance_id,concept_id,history_version)
SELECT 'history.legacy.mastered.' || state.rowid, state.student_id, 'concept.mastered',
       'concept-mastered.legacy.' || state.rowid, state.mastered_at, instance.goal_id,
       state.curriculum_instance_id, state.concept_id, 'study-history-v1'
FROM learner_curriculum_concept_states AS state
JOIN learner_curriculum_instances AS instance ON instance.id = state.curriculum_instance_id
WHERE state.mastered_at IS NOT NULL`,
			`INSERT INTO study_history_events
(id,student_id,event_type,source_id,occurred_at,concept_id,history_version)
SELECT 'history.legacy.review.' || rowid, student_id, 'review.completed',
       'review.legacy.' || rowid, completed_at, concept_id, 'study-history-v1'
FROM review_items WHERE status = 'completed'`,
			`INSERT INTO study_history_events
(id,student_id,event_type,source_id,occurred_at,goal_id,curriculum_instance_id,history_version)
SELECT 'history.legacy.session-lifecycle.' || rowid, student_id, 'session.completed',
       'session-lifecycle.legacy.' || rowid, ended_at, goal_id, curriculum_instance_id, 'study-history-v1'
FROM study_session_lifecycle WHERE status = 'completed'`,
			`INSERT INTO study_history_events
(id,student_id,event_type,source_id,occurred_at,goal_id,history_version)
SELECT 'history.legacy.session.' || rowid, student_id, 'session.completed',
       'session.legacy.' || rowid, ended_at, goal_id, 'study-history-v1'
FROM study_sessions`,
			`INSERT INTO study_history_events
(id,student_id,event_type,source_id,occurred_at,history_version)
SELECT 'history.legacy.achievement.' || rowid, student_id, 'achievement.unlocked',
       'achievement.legacy.' || rowid, unlocked_at, 'study-history-v1'
FROM student_achievements WHERE status = 'unlocked'`,
		},
	},
	{
		version: 16,
		name:    "retention model v1",
		statements: []string{
			`ALTER TABLE retention_state ADD COLUMN last_successful_recall TEXT
CHECK (last_successful_recall IS NULL OR last_successful_recall GLOB '*Z')`,
			`ALTER TABLE retention_state ADD COLUMN last_practice TEXT
CHECK (last_practice IS NULL OR last_practice GLOB '*Z')`,
			`ALTER TABLE retention_state ADD COLUMN review_count INTEGER NOT NULL DEFAULT 0 CHECK (review_count >= 0)`,
			`ALTER TABLE retention_state ADD COLUMN successful_reviews INTEGER NOT NULL DEFAULT 0 CHECK (successful_reviews >= 0)`,
			`ALTER TABLE retention_state ADD COLUMN failed_reviews INTEGER NOT NULL DEFAULT 0 CHECK (failed_reviews >= 0)`,
			`ALTER TABLE retention_state ADD COLUMN stability_estimate_seconds INTEGER NOT NULL DEFAULT 0 CHECK (stability_estimate_seconds >= 0)`,
			`ALTER TABLE retention_state ADD COLUMN retention_status TEXT NOT NULL DEFAULT 'unknown'
CHECK (retention_status IN ('fresh','stable','weakening','due','overdue','unknown'))`,
			`ALTER TABLE retention_state ADD COLUMN next_due_at TEXT
CHECK (next_due_at IS NULL OR next_due_at GLOB '*Z')`,
			`ALTER TABLE retention_state ADD COLUMN algorithm_version TEXT NOT NULL DEFAULT 'legacy-retention/v0'
CHECK (algorithm_version IN ('legacy-retention/v0','retention-v1'))`,
			`CREATE TRIGGER retention_state_v1_insert_guard
BEFORE INSERT ON retention_state
WHEN NEW.review_count <> NEW.successful_reviews + NEW.failed_reviews
  OR (NEW.last_successful_recall IS NOT NULL AND NEW.last_practice IS NULL)
  OR NEW.last_successful_recall > NEW.measured_at
  OR NEW.last_practice > NEW.measured_at
  OR (NEW.algorithm_version = 'legacy-retention/v0' AND
      (NEW.retention_status <> 'unknown' OR NEW.last_successful_recall IS NOT NULL OR NEW.last_practice IS NOT NULL OR
       NEW.review_count <> 0 OR NEW.stability_estimate_seconds <> 0 OR NEW.next_due_at IS NOT NULL))
  OR (NEW.algorithm_version = 'retention-v1' AND NEW.retention_status = 'unknown' AND
      (NEW.strength <> 0 OR NEW.last_successful_recall IS NOT NULL OR NEW.last_practice IS NOT NULL OR
       NEW.review_count <> 0 OR NEW.stability_estimate_seconds <> 0 OR NEW.next_due_at IS NOT NULL))
  OR (NEW.algorithm_version = 'retention-v1' AND NEW.retention_status <> 'unknown' AND
      (NEW.last_practice IS NULL OR NEW.stability_estimate_seconds <= 0 OR NEW.next_due_at IS NULL OR NEW.next_due_at <= NEW.last_practice))
BEGIN SELECT RAISE(ABORT, 'invalid retention-v1 aggregate'); END`,
			`CREATE TRIGGER retention_state_v1_update_guard
BEFORE UPDATE ON retention_state
WHEN NEW.review_count <> NEW.successful_reviews + NEW.failed_reviews
  OR (NEW.last_successful_recall IS NOT NULL AND NEW.last_practice IS NULL)
  OR NEW.last_successful_recall > NEW.measured_at
  OR NEW.last_practice > NEW.measured_at
  OR (NEW.algorithm_version = 'legacy-retention/v0' AND
      (NEW.retention_status <> 'unknown' OR NEW.last_successful_recall IS NOT NULL OR NEW.last_practice IS NOT NULL OR
       NEW.review_count <> 0 OR NEW.stability_estimate_seconds <> 0 OR NEW.next_due_at IS NOT NULL))
  OR (NEW.algorithm_version = 'retention-v1' AND NEW.retention_status = 'unknown' AND
      (NEW.strength <> 0 OR NEW.last_successful_recall IS NOT NULL OR NEW.last_practice IS NOT NULL OR
       NEW.review_count <> 0 OR NEW.stability_estimate_seconds <> 0 OR NEW.next_due_at IS NOT NULL))
  OR (NEW.algorithm_version = 'retention-v1' AND NEW.retention_status <> 'unknown' AND
      (NEW.last_practice IS NULL OR NEW.stability_estimate_seconds <= 0 OR NEW.next_due_at IS NULL OR NEW.next_due_at <= NEW.last_practice))
BEGIN SELECT RAISE(ABORT, 'invalid retention-v1 aggregate'); END`,
		},
	},
	{
		version: 17,
		name:    "spaced repetition scheduler v1",
		statements: []string{
			`ALTER TABLE review_schedule ADD COLUMN review_type TEXT NOT NULL DEFAULT 'standard_review'
CHECK (review_type IN ('quick_recall','standard_review','deep_review'))`,
			`ALTER TABLE review_schedule ADD COLUMN estimated_minutes INTEGER NOT NULL DEFAULT 10 CHECK (estimated_minutes IN (5,10,20))`,
			`ALTER TABLE review_schedule ADD COLUMN critical_prerequisite INTEGER NOT NULL DEFAULT 0 CHECK (critical_prerequisite IN (0,1))`,
			`ALTER TABLE review_schedule ADD COLUMN updated_at TEXT NOT NULL DEFAULT '1970-01-01T00:00:00Z' CHECK (updated_at GLOB '*Z')`,
			`ALTER TABLE review_schedule ADD COLUMN algorithm_version TEXT NOT NULL DEFAULT 'legacy-review/v0'
CHECK (algorithm_version IN ('legacy-review/v0','review-scheduler-v1'))`,
			`UPDATE review_schedule SET updated_at=due_at WHERE updated_at='1970-01-01T00:00:00Z'`,
			`CREATE TRIGGER review_schedule_v1_insert_guard
BEFORE INSERT ON review_schedule
WHEN (NEW.review_type='quick_recall' AND NEW.estimated_minutes<>5)
  OR (NEW.review_type='standard_review' AND NEW.estimated_minutes<>10)
  OR (NEW.review_type='deep_review' AND NEW.estimated_minutes<>20)
  OR (NEW.algorithm_version='review-scheduler-v1' AND (NEW.imported<>0 OR NEW.introduced_at IS NULL))
BEGIN SELECT RAISE(ABORT, 'invalid review schedule aggregate'); END`,
			`CREATE TRIGGER review_schedule_v1_update_guard
BEFORE UPDATE ON review_schedule
WHEN (NEW.review_type='quick_recall' AND NEW.estimated_minutes<>5)
  OR (NEW.review_type='standard_review' AND NEW.estimated_minutes<>10)
  OR (NEW.review_type='deep_review' AND NEW.estimated_minutes<>20)
  OR (NEW.algorithm_version='review-scheduler-v1' AND (NEW.imported<>0 OR NEW.introduced_at IS NULL))
BEGIN SELECT RAISE(ABORT, 'invalid review schedule aggregate'); END`,
			`ALTER TABLE review_items ADD COLUMN review_type TEXT NOT NULL DEFAULT 'standard_review'
CHECK (review_type IN ('quick_recall','standard_review','deep_review'))`,
			`ALTER TABLE review_items ADD COLUMN estimated_minutes INTEGER NOT NULL DEFAULT 10 CHECK (estimated_minutes IN (5,10,20))`,
			`ALTER TABLE review_items ADD COLUMN critical_prerequisite INTEGER NOT NULL DEFAULT 0 CHECK (critical_prerequisite IN (0,1))`,
			`ALTER TABLE review_items ADD COLUMN outcome TEXT NOT NULL DEFAULT '' CHECK (outcome IN ('','success','failure'))`,
			`ALTER TABLE review_items ADD COLUMN score REAL CHECK (score IS NULL OR score BETWEEN 0 AND 1)`,
			`ALTER TABLE review_items ADD COLUMN skipped_at TEXT CHECK (skipped_at IS NULL OR skipped_at GLOB '*Z')`,
			`ALTER TABLE review_items ADD COLUMN postponed_at TEXT CHECK (postponed_at IS NULL OR postponed_at GLOB '*Z')`,
			`ALTER TABLE review_items ADD COLUMN postpone_count INTEGER NOT NULL DEFAULT 0 CHECK (postpone_count >= 0)`,
			`ALTER TABLE review_items ADD COLUMN created_at TEXT NOT NULL DEFAULT '1970-01-01T00:00:00Z' CHECK (created_at GLOB '*Z')`,
			`ALTER TABLE review_items ADD COLUMN scheduler_version TEXT NOT NULL DEFAULT 'legacy-review/v0'
CHECK (scheduler_version IN ('legacy-review/v0','review-scheduler-v1'))`,
			`UPDATE review_items SET created_at=due_at WHERE created_at='1970-01-01T00:00:00Z'`,
			`UPDATE review_items AS duplicate SET status='skipped'
WHERE duplicate.status='pending' AND EXISTS (
  SELECT 1 FROM review_items AS keeper
  WHERE keeper.student_id=duplicate.student_id AND keeper.concept_id=duplicate.concept_id AND keeper.status='pending'
    AND (keeper.due_at<duplicate.due_at OR (keeper.due_at=duplicate.due_at AND keeper.id<duplicate.id))
)`,
			`CREATE UNIQUE INDEX review_items_one_pending_idx ON review_items (student_id,concept_id) WHERE status='pending'`,
			`CREATE TRIGGER review_items_v1_insert_guard
BEFORE INSERT ON review_items
WHEN (NEW.review_type='quick_recall' AND NEW.estimated_minutes<>5)
  OR (NEW.review_type='standard_review' AND NEW.estimated_minutes<>10)
  OR (NEW.review_type='deep_review' AND NEW.estimated_minutes<>20)
  OR (NEW.postpone_count=0)<>(NEW.postponed_at IS NULL)
  OR (NEW.scheduler_version='review-scheduler-v1' AND (
       (NEW.status='pending' AND (NEW.outcome<>'' OR NEW.score IS NOT NULL OR NEW.completed_at IS NOT NULL OR NEW.skipped_at IS NOT NULL))
    OR (NEW.status='completed' AND (NEW.outcome='' OR NEW.score IS NULL OR NEW.completed_at IS NULL OR NEW.skipped_at IS NOT NULL))
    OR (NEW.status='skipped' AND (NEW.outcome<>'' OR NEW.score IS NOT NULL OR NEW.completed_at IS NOT NULL OR NEW.skipped_at IS NULL))
    OR (NEW.outcome='success' AND NEW.score<0.7) OR (NEW.outcome='failure' AND NEW.score>=0.7)))
BEGIN SELECT RAISE(ABORT, 'invalid review item aggregate'); END`,
			`CREATE TRIGGER review_items_v1_update_guard
BEFORE UPDATE ON review_items
WHEN (NEW.review_type='quick_recall' AND NEW.estimated_minutes<>5)
  OR (NEW.review_type='standard_review' AND NEW.estimated_minutes<>10)
  OR (NEW.review_type='deep_review' AND NEW.estimated_minutes<>20)
  OR (NEW.postpone_count=0)<>(NEW.postponed_at IS NULL)
  OR (NEW.scheduler_version='review-scheduler-v1' AND (
       (NEW.status='pending' AND (NEW.outcome<>'' OR NEW.score IS NOT NULL OR NEW.completed_at IS NOT NULL OR NEW.skipped_at IS NOT NULL))
    OR (NEW.status='completed' AND (NEW.outcome='' OR NEW.score IS NULL OR NEW.completed_at IS NULL OR NEW.skipped_at IS NOT NULL))
    OR (NEW.status='skipped' AND (NEW.outcome<>'' OR NEW.score IS NOT NULL OR NEW.completed_at IS NOT NULL OR NEW.skipped_at IS NULL))
    OR (NEW.outcome='success' AND NEW.score<0.7) OR (NEW.outcome='failure' AND NEW.score>=0.7)))
BEGIN SELECT RAISE(ABORT, 'invalid review item aggregate'); END`,
		},
	},
	{
		version: 18,
		name:    "non-punitive study streak v1",
		statements: []string{
			`ALTER TABLE streak_state ADD COLUMN last_active_local_date TEXT`,
			`ALTER TABLE streak_state ADD COLUMN total_active_days INTEGER NOT NULL DEFAULT 0 CHECK (total_active_days >= 0)`,
			`ALTER TABLE streak_state ADD COLUMN streak_timezone TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE streak_state ADD COLUMN minimum_active_minutes INTEGER NOT NULL DEFAULT 0 CHECK (minimum_active_minutes BETWEEN 0 AND 1440)`,
			`ALTER TABLE streak_state ADD COLUMN policy_version TEXT NOT NULL DEFAULT 'legacy-streak/v0'
CHECK (policy_version IN ('legacy-streak/v0','streak-v1'))`,
			`CREATE TRIGGER streak_state_v1_insert_guard
BEFORE INSERT ON streak_state
WHEN (NEW.policy_version='legacy-streak/v0' AND
      (NEW.last_active_local_date IS NOT NULL OR NEW.total_active_days<>0 OR NEW.streak_timezone<>'' OR NEW.minimum_active_minutes<>0))
  OR (NEW.policy_version='streak-v1' AND (
       length(trim(NEW.streak_timezone))=0 OR NEW.minimum_active_minutes<1 OR NEW.longest_days>NEW.total_active_days
    OR (NEW.total_active_days=0 AND (NEW.current_days<>0 OR NEW.longest_days<>0 OR NEW.last_active_local_date IS NOT NULL OR NEW.last_study_at IS NOT NULL))
    OR (NEW.total_active_days>0 AND (NEW.longest_days=0 OR NEW.last_active_local_date IS NULL OR NEW.last_study_at IS NULL))
    OR (NEW.last_active_local_date IS NOT NULL AND (length(NEW.last_active_local_date)<>10 OR substr(NEW.last_active_local_date,5,1)<>'-' OR substr(NEW.last_active_local_date,8,1)<>'-'))))
BEGIN SELECT RAISE(ABORT, 'invalid streak-v1 aggregate'); END`,
			`CREATE TRIGGER streak_state_v1_update_guard
BEFORE UPDATE ON streak_state
WHEN (NEW.policy_version='legacy-streak/v0' AND
      (NEW.last_active_local_date IS NOT NULL OR NEW.total_active_days<>0 OR NEW.streak_timezone<>'' OR NEW.minimum_active_minutes<>0))
  OR (NEW.policy_version='streak-v1' AND (
       length(trim(NEW.streak_timezone))=0 OR NEW.minimum_active_minutes<1 OR NEW.longest_days>NEW.total_active_days
    OR (NEW.total_active_days=0 AND (NEW.current_days<>0 OR NEW.longest_days<>0 OR NEW.last_active_local_date IS NOT NULL OR NEW.last_study_at IS NOT NULL))
    OR (NEW.total_active_days>0 AND (NEW.longest_days=0 OR NEW.last_active_local_date IS NULL OR NEW.last_study_at IS NULL))
    OR (NEW.last_active_local_date IS NOT NULL AND (length(NEW.last_active_local_date)<>10 OR substr(NEW.last_active_local_date,5,1)<>'-' OR substr(NEW.last_active_local_date,8,1)<>'-'))))
BEGIN SELECT RAISE(ABORT, 'invalid streak-v1 aggregate'); END`,
		},
	},
	{
		version: 19,
		name:    "learning achievement framework v1",
		statements: []string{
			`ALTER TABLE achievement_definitions ADD COLUMN description TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE achievement_definitions ADD COLUMN criteria_type TEXT NOT NULL DEFAULT 'legacy'
CHECK (criteria_type IN ('legacy','first_session','first_concept_mastered','active_days','study_minutes','first_review_completed','module_mastered'))`,
			`ALTER TABLE achievement_definitions ADD COLUMN criteria_config_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(criteria_config_json))`,
			`ALTER TABLE achievement_definitions ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0,1))`,
			`ALTER TABLE achievement_definitions ADD COLUMN definition_version TEXT NOT NULL DEFAULT 'legacy-achievement/v0'
CHECK (definition_version IN ('legacy-achievement/v0','achievement-definition/v1'))`,
			`ALTER TABLE student_achievements ADD COLUMN context_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(context_json) AND json_type(context_json)='object')`,
			`ALTER TABLE student_achievements ADD COLUMN policy_version TEXT NOT NULL DEFAULT 'legacy-achievement/v0'
CHECK (policy_version IN ('legacy-achievement/v0','achievement-v1'))`,
			`CREATE TRIGGER achievement_definitions_v1_insert_guard
BEFORE INSERT ON achievement_definitions
WHEN (NEW.definition_version='legacy-achievement/v0' AND
      (NEW.description<>'' OR NEW.criteria_type<>'legacy' OR NEW.criteria_config_json<>'{}' OR NEW.hidden<>0))
  OR (NEW.definition_version='achievement-definition/v1' AND (
       length(trim(NEW.description))=0 OR NEW.criteria_type='legacy'
    OR (NEW.criteria_type='active_days' AND (COALESCE(json_extract(NEW.criteria_config_json,'$.count'),0)<1 OR COALESCE(json_extract(NEW.criteria_config_json,'$.minutes'),0)<>0))
    OR (NEW.criteria_type='study_minutes' AND (COALESCE(json_extract(NEW.criteria_config_json,'$.minutes'),0)<1 OR COALESCE(json_extract(NEW.criteria_config_json,'$.count'),0)<>0))
    OR (NEW.criteria_type NOT IN ('active_days','study_minutes') AND NEW.criteria_config_json<>'{}')))
BEGIN SELECT RAISE(ABORT, 'invalid achievement definition v1'); END`,
			`CREATE TRIGGER achievement_definitions_v1_update_guard
BEFORE UPDATE ON achievement_definitions
WHEN (NEW.definition_version='legacy-achievement/v0' AND
      (NEW.description<>'' OR NEW.criteria_type<>'legacy' OR NEW.criteria_config_json<>'{}' OR NEW.hidden<>0))
  OR (NEW.definition_version='achievement-definition/v1' AND (
       length(trim(NEW.description))=0 OR NEW.criteria_type='legacy'
    OR (NEW.criteria_type='active_days' AND (COALESCE(json_extract(NEW.criteria_config_json,'$.count'),0)<1 OR COALESCE(json_extract(NEW.criteria_config_json,'$.minutes'),0)<>0))
    OR (NEW.criteria_type='study_minutes' AND (COALESCE(json_extract(NEW.criteria_config_json,'$.minutes'),0)<1 OR COALESCE(json_extract(NEW.criteria_config_json,'$.count'),0)<>0))
    OR (NEW.criteria_type NOT IN ('active_days','study_minutes') AND NEW.criteria_config_json<>'{}')))
BEGIN SELECT RAISE(ABORT, 'invalid achievement definition v1'); END`,
			`CREATE TRIGGER student_achievements_v1_insert_guard
BEFORE INSERT ON student_achievements
WHEN NEW.policy_version='achievement-v1' AND (NEW.status<>'unlocked' OR NEW.unlocked_at IS NULL)
BEGIN SELECT RAISE(ABORT, 'invalid student achievement v1'); END`,
			`CREATE TRIGGER student_achievements_v1_update_guard
BEFORE UPDATE ON student_achievements
WHEN NEW.policy_version='achievement-v1' AND (NEW.status<>'unlocked' OR NEW.unlocked_at IS NULL)
BEGIN SELECT RAISE(ABORT, 'invalid student achievement v1'); END`,
		},
	},
	{
		version: 20,
		name:    "adaptive daily plan v1",
		statements: []string{
			`ALTER TABLE daily_plans ADD COLUMN curriculum_instance_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE daily_plans ADD COLUMN timezone TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE daily_plans ADD COLUMN available_minutes INTEGER NOT NULL DEFAULT 0 CHECK (available_minutes BETWEEN 0 AND 1440)`,
			`ALTER TABLE daily_plans ADD COLUMN planned_minutes INTEGER NOT NULL DEFAULT 0 CHECK (planned_minutes BETWEEN 0 AND 1440)`,
			`ALTER TABLE daily_plans ADD COLUMN buffer_minutes INTEGER NOT NULL DEFAULT 0 CHECK (buffer_minutes BETWEEN 0 AND 1440)`,
			`ALTER TABLE daily_plans ADD COLUMN plan_status TEXT NOT NULL DEFAULT 'legacy'
CHECK (plan_status IN ('legacy','ready','review_only','nothing_urgent','time_limited'))`,
			`ALTER TABLE daily_plans ADD COLUMN generation_reason TEXT NOT NULL DEFAULT 'legacy'
CHECK (generation_reason IN ('legacy','initial','source_changed','policy_changed'))`,
			`ALTER TABLE daily_plans ADD COLUMN source_fingerprint TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE daily_plans ADD COLUMN policy_version TEXT NOT NULL DEFAULT 'legacy-daily-plan/v0'
CHECK (policy_version IN ('legacy-daily-plan/v0','daily-plan-v1'))`,
			`ALTER TABLE daily_plan_items ADD COLUMN item_role TEXT NOT NULL DEFAULT 'legacy'
CHECK (item_role IN ('legacy','warm_up','review','new_learning','reinforcement'))`,
			`ALTER TABLE daily_plan_items ADD COLUMN selection_reason TEXT NOT NULL DEFAULT 'legacy'
CHECK (selection_reason IN ('legacy','critical_overdue_prerequisite','important_due_review','blocking_weakness','next_eligible_concept','optional_extra_practice'))`,
			`ALTER TABLE daily_plan_items ADD COLUMN explanation TEXT NOT NULL DEFAULT ''`,
			`CREATE TRIGGER daily_plans_v1_insert_guard
BEFORE INSERT ON daily_plans
WHEN (NEW.policy_version='legacy-daily-plan/v0' AND
      (NEW.curriculum_instance_id<>'' OR NEW.timezone<>'' OR NEW.available_minutes<>0 OR NEW.planned_minutes<>0 OR
       NEW.buffer_minutes<>0 OR NEW.plan_status<>'legacy' OR NEW.generation_reason<>'legacy' OR NEW.source_fingerprint<>''))
  OR (NEW.policy_version='daily-plan-v1' AND
      (length(trim(NEW.curriculum_instance_id))=0 OR length(trim(NEW.timezone))=0 OR
       NEW.planned_minutes+NEW.buffer_minutes>NEW.available_minutes OR NEW.plan_status='legacy' OR
       NEW.generation_reason='legacy' OR length(NEW.source_fingerprint)<>71 OR NEW.source_fingerprint NOT GLOB 'sha256:*' OR
       NOT EXISTS (SELECT 1 FROM learner_curriculum_instances i
                   WHERE i.id=NEW.curriculum_instance_id AND i.student_id=NEW.student_id AND i.goal_id=NEW.goal_id)))
BEGIN SELECT RAISE(ABORT, 'invalid daily-plan-v1 snapshot'); END`,
			`CREATE TRIGGER daily_plans_v1_update_guard
BEFORE UPDATE ON daily_plans
WHEN (NEW.policy_version='legacy-daily-plan/v0' AND
      (NEW.curriculum_instance_id<>'' OR NEW.timezone<>'' OR NEW.available_minutes<>0 OR NEW.planned_minutes<>0 OR
       NEW.buffer_minutes<>0 OR NEW.plan_status<>'legacy' OR NEW.generation_reason<>'legacy' OR NEW.source_fingerprint<>''))
  OR (NEW.policy_version='daily-plan-v1' AND
      (length(trim(NEW.curriculum_instance_id))=0 OR length(trim(NEW.timezone))=0 OR
       NEW.planned_minutes+NEW.buffer_minutes>NEW.available_minutes OR NEW.plan_status='legacy' OR
       NEW.generation_reason='legacy' OR length(NEW.source_fingerprint)<>71 OR NEW.source_fingerprint NOT GLOB 'sha256:*' OR
       NOT EXISTS (SELECT 1 FROM learner_curriculum_instances i
                   WHERE i.id=NEW.curriculum_instance_id AND i.student_id=NEW.student_id AND i.goal_id=NEW.goal_id)))
BEGIN SELECT RAISE(ABORT, 'invalid daily-plan-v1 snapshot'); END`,
			`CREATE TRIGGER daily_plan_items_v1_insert_guard
BEFORE INSERT ON daily_plan_items
WHEN EXISTS (SELECT 1 FROM daily_plans p WHERE p.id=NEW.plan_id AND p.policy_version='daily-plan-v1')
 AND (NEW.item_role='legacy' OR NEW.selection_reason='legacy' OR length(trim(NEW.explanation))=0 OR
      (NEW.item_role='warm_up' AND (NEW.item_type<>'review' OR NEW.selection_reason<>'critical_overdue_prerequisite')) OR
      (NEW.item_role='review' AND (NEW.item_type<>'review' OR NEW.selection_reason<>'important_due_review')) OR
      (NEW.item_role='new_learning' AND (NEW.item_type<>'learn' OR NEW.selection_reason<>'next_eligible_concept')) OR
      (NEW.item_role='reinforcement' AND (NEW.item_type<>'practice' OR NEW.selection_reason NOT IN ('blocking_weakness','optional_extra_practice'))))
BEGIN SELECT RAISE(ABORT, 'invalid daily-plan-v1 item'); END`,
			`CREATE TRIGGER daily_plan_items_v1_update_guard
BEFORE UPDATE ON daily_plan_items
WHEN EXISTS (SELECT 1 FROM daily_plans p WHERE p.id=NEW.plan_id AND p.policy_version='daily-plan-v1')
 AND (NEW.item_role='legacy' OR NEW.selection_reason='legacy' OR length(trim(NEW.explanation))=0 OR
      (NEW.item_role='warm_up' AND (NEW.item_type<>'review' OR NEW.selection_reason<>'critical_overdue_prerequisite')) OR
      (NEW.item_role='review' AND (NEW.item_type<>'review' OR NEW.selection_reason<>'important_due_review')) OR
      (NEW.item_role='new_learning' AND (NEW.item_type<>'learn' OR NEW.selection_reason<>'next_eligible_concept')) OR
      (NEW.item_role='reinforcement' AND (NEW.item_type<>'practice' OR NEW.selection_reason NOT IN ('blocking_weakness','optional_extra_practice'))))
BEGIN SELECT RAISE(ABORT, 'invalid daily-plan-v1 item'); END`,
		},
	},
	{
		version: 21,
		name:    "learning derived-state algorithm versions",
		statements: []string{
			`ALTER TABLE learner_curriculum_concept_states ADD COLUMN mastery_algorithm_version TEXT NOT NULL DEFAULT 'mastery-v1'
CHECK (length(trim(mastery_algorithm_version)) > 0)`,
			`ALTER TABLE learner_curriculum_concept_states ADD COLUMN progression_policy_version TEXT NOT NULL DEFAULT 'progression-v1'
CHECK (length(trim(progression_policy_version)) > 0)`,
		},
	},
	{
		version: 22,
		name:    "student core integrity and query hardening",
		statements: []string{
			`CREATE INDEX learner_curriculum_instances_timeline_idx
ON learner_curriculum_instances (student_id, created_at, id)`,
			`CREATE INDEX study_session_lifecycle_student_timeline_idx
ON study_session_lifecycle (student_id, started_at, id)`,
			`CREATE INDEX review_items_student_timeline_idx
ON review_items (student_id, due_at, id)`,
			`CREATE TRIGGER learner_curriculum_concept_states_membership_insert_guard
BEFORE INSERT ON learner_curriculum_concept_states
WHEN NOT EXISTS (
  SELECT 1
  FROM learner_curriculum_instances AS instance
  JOIN curriculum_nodes AS node
    ON node.curriculum_id = instance.curriculum_id
   AND node.curriculum_version = instance.curriculum_version
   AND node.node_id = NEW.concept_id
   AND node.node_type = 'concept'
  WHERE instance.id = NEW.curriculum_instance_id
    AND instance.student_id = NEW.student_id
)
BEGIN SELECT RAISE(ABORT, 'concept state is outside curriculum instance'); END`,
			`CREATE TRIGGER learner_curriculum_concept_states_membership_update_guard
BEFORE UPDATE ON learner_curriculum_concept_states
WHEN NOT EXISTS (
  SELECT 1
  FROM learner_curriculum_instances AS instance
  JOIN curriculum_nodes AS node
    ON node.curriculum_id = instance.curriculum_id
   AND node.curriculum_version = instance.curriculum_version
   AND node.node_id = NEW.concept_id
   AND node.node_type = 'concept'
  WHERE instance.id = NEW.curriculum_instance_id
    AND instance.student_id = NEW.student_id
)
BEGIN SELECT RAISE(ABORT, 'concept state is outside curriculum instance'); END`,
			`CREATE TRIGGER diagnostic_observations_ownership_insert_guard
BEFORE INSERT ON diagnostic_observations
WHEN NOT EXISTS (
  SELECT 1
  FROM diagnostic_attempts AS attempt
  JOIN learner_curriculum_instances AS instance
    ON instance.id = attempt.curriculum_instance_id
   AND instance.student_id = attempt.student_id
  JOIN curriculum_nodes AS node
    ON node.curriculum_id = instance.curriculum_id
   AND node.curriculum_version = instance.curriculum_version
   AND node.node_id = NEW.concept_id
   AND node.node_type = 'concept'
  JOIN learning_evidence AS evidence
    ON evidence.id = NEW.evidence_id
   AND evidence.student_id = attempt.student_id
   AND evidence.concept_id = NEW.concept_id
  WHERE attempt.id = NEW.attempt_id
)
BEGIN SELECT RAISE(ABORT, 'diagnostic observation ownership mismatch'); END`,
			`CREATE TRIGGER diagnostic_observations_ownership_update_guard
BEFORE UPDATE ON diagnostic_observations
WHEN NOT EXISTS (
  SELECT 1
  FROM diagnostic_attempts AS attempt
  JOIN learner_curriculum_instances AS instance
    ON instance.id = attempt.curriculum_instance_id
   AND instance.student_id = attempt.student_id
  JOIN curriculum_nodes AS node
    ON node.curriculum_id = instance.curriculum_id
   AND node.curriculum_version = instance.curriculum_version
   AND node.node_id = NEW.concept_id
   AND node.node_type = 'concept'
  JOIN learning_evidence AS evidence
    ON evidence.id = NEW.evidence_id
   AND evidence.student_id = attempt.student_id
   AND evidence.concept_id = NEW.concept_id
  WHERE attempt.id = NEW.attempt_id
)
BEGIN SELECT RAISE(ABORT, 'diagnostic observation ownership mismatch'); END`,
		},
	},
	{
		version: 23,
		name:    "research and source intelligence persistence",
		statements: []string{
			`CREATE TABLE research_topics (
    request_id TEXT PRIMARY KEY CHECK (length(trim(request_id)) > 0),
    subject TEXT NOT NULL CHECK (length(trim(subject)) > 0),
    domain TEXT NOT NULL DEFAULT '',
    technology TEXT NOT NULL DEFAULT '',
    purpose TEXT NOT NULL CHECK (purpose IN ('concept_definition','current_usage','version_behavior','release_status','deprecation_check','prerequisite_research','production_practice','security_guidance')),
    target_version TEXT,
    requested_at TEXT NOT NULL CHECK (requested_at GLOB '*Z')
)`,
			`CREATE TABLE research_runs (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    request_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('planned','running','completed','failed','cancelled')),
    started_at TEXT NOT NULL CHECK (started_at GLOB '*Z'),
    completed_at TEXT CHECK (completed_at IS NULL OR completed_at GLOB '*Z'),
    FOREIGN KEY (request_id) REFERENCES research_topics(request_id),
    CHECK ((status IN ('completed','failed','cancelled') AND completed_at IS NOT NULL) OR
           (status IN ('planned','running') AND completed_at IS NULL)),
    CHECK (completed_at IS NULL OR completed_at >= started_at)
)`,
			`CREATE INDEX research_runs_request_idx ON research_runs (request_id, started_at, id)`,
			`CREATE TABLE sources (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    kind TEXT NOT NULL CHECK (kind IN ('official_documentation','specification','standard','release_notes','official_blog','package_reference','official_tutorial','source_code','issue_tracker','community_article','community_forum','video','paper','book_reference','other')),
    locator TEXT NOT NULL UNIQUE CHECK (locator GLOB 'http://*' OR locator GLOB 'https://*'),
    version TEXT,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    publisher TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT '',
    published_at TEXT CHECK (published_at IS NULL OR published_at GLOB '*Z'),
    updated_at TEXT CHECK (updated_at IS NULL OR updated_at GLOB '*Z'),
    created_at TEXT NOT NULL CHECK (created_at GLOB '*Z'),
    CHECK (published_at IS NULL OR updated_at IS NULL OR updated_at >= published_at)
)`,
			`CREATE INDEX sources_locator_idx ON sources (locator)`,
			`CREATE TABLE source_aliases (
    locator TEXT PRIMARY KEY CHECK (locator GLOB 'http://*' OR locator GLOB 'https://*'),
    source_id TEXT NOT NULL,
    created_at TEXT NOT NULL CHECK (created_at GLOB '*Z'),
    FOREIGN KEY (source_id) REFERENCES sources(id) ON DELETE CASCADE
)`,
			`CREATE INDEX source_aliases_source_idx ON source_aliases (source_id, locator)`,
			`CREATE TABLE source_snapshots (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    source_id TEXT NOT NULL,
    locator TEXT NOT NULL CHECK (locator GLOB 'http://*' OR locator GLOB 'https://*'),
    fetched_at TEXT NOT NULL CHECK (fetched_at GLOB '*Z'),
    status_code INTEGER NOT NULL CHECK (status_code BETWEEN 100 AND 599),
    content_type TEXT NOT NULL DEFAULT '',
    etag TEXT NOT NULL DEFAULT '',
    last_modified TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    content_length INTEGER NOT NULL CHECK (content_length >= 0),
    fetch_version TEXT NOT NULL CHECK (length(trim(fetch_version)) > 0),
    UNIQUE (id, source_id),
    FOREIGN KEY (source_id) REFERENCES sources(id) ON DELETE CASCADE
)`,
			`CREATE INDEX source_snapshots_latest_idx ON source_snapshots (source_id, fetched_at DESC, id DESC)`,
			`CREATE TABLE authority_profiles (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    version TEXT NOT NULL CHECK (length(trim(version)) > 0),
    domain TEXT NOT NULL CHECK (length(trim(domain)) > 0),
    topic_pattern TEXT NOT NULL DEFAULT '',
    preferred_kinds_json TEXT NOT NULL CHECK (json_valid(preferred_kinds_json) AND json_type(preferred_kinds_json) = 'array' AND json_array_length(preferred_kinds_json) > 0),
    minimum_tier TEXT NOT NULL CHECK (minimum_tier IN ('A','B','C','D','E')),
    created_at TEXT NOT NULL CHECK (created_at GLOB '*Z')
)`,
			`CREATE TABLE trust_registry (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('accepted','accepted_as_supplement','requires_verification','rejected')),
    tier TEXT NOT NULL CHECK (tier IN ('A','B','C','D','E')),
    reasons_json TEXT NOT NULL CHECK (json_valid(reasons_json) AND json_type(reasons_json) = 'array' AND json_array_length(reasons_json) > 0),
    policy_version TEXT NOT NULL CHECK (length(trim(policy_version)) > 0),
    evaluated_at TEXT NOT NULL CHECK (evaluated_at GLOB '*Z'),
    FOREIGN KEY (source_id) REFERENCES sources(id) ON DELETE CASCADE
)`,
			`CREATE INDEX trust_registry_latest_idx ON trust_registry (source_id, evaluated_at DESC, id DESC)`,
			`CREATE TABLE evidence (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    source_id TEXT NOT NULL,
    snapshot_id TEXT NOT NULL,
    location TEXT NOT NULL CHECK (length(trim(location)) > 0),
    excerpt TEXT NOT NULL CHECK (length(trim(excerpt)) > 0 AND length(CAST(excerpt AS BLOB)) <= 8192),
    excerpt_hash TEXT NOT NULL CHECK (length(trim(excerpt_hash)) > 0),
    extracted_at TEXT NOT NULL CHECK (extracted_at GLOB '*Z'),
    extractor_version TEXT NOT NULL CHECK (length(trim(extractor_version)) > 0),
    FOREIGN KEY (snapshot_id, source_id) REFERENCES source_snapshots(id, source_id) ON DELETE CASCADE
)`,
			`CREATE INDEX evidence_source_idx ON evidence (source_id, id)`,
			`CREATE INDEX evidence_snapshot_idx ON evidence (snapshot_id, id)`,
			`CREATE TABLE claims (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    topic_subject TEXT NOT NULL CHECK (length(trim(topic_subject)) > 0),
    topic_domain TEXT NOT NULL DEFAULT '',
    topic_technology TEXT NOT NULL DEFAULT '',
    statement TEXT NOT NULL CHECK (length(trim(statement)) > 0),
    claim_type TEXT NOT NULL CHECK (claim_type IN ('definition','requirement','behavior','version_change','deprecation','recommendation','warning','example','compatibility','security','historical')),
    version_scope TEXT,
    confidence REAL NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    evidence_ids_json TEXT NOT NULL CHECK (json_valid(evidence_ids_json) AND json_type(evidence_ids_json) = 'array' AND json_array_length(evidence_ids_json) > 0),
    created_at TEXT NOT NULL CHECK (created_at GLOB '*Z')
)`,
			`CREATE INDEX claims_topic_idx ON claims (topic_subject, topic_domain, topic_technology, created_at, id)`,
			`CREATE TABLE claim_sources (
    claim_id TEXT NOT NULL,
    source_id TEXT NOT NULL,
    position INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY (claim_id, source_id),
    UNIQUE (claim_id, position),
    FOREIGN KEY (claim_id) REFERENCES claims(id) ON DELETE CASCADE,
    FOREIGN KEY (source_id) REFERENCES sources(id)
)`,
			`CREATE TABLE citations (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    source_id TEXT NOT NULL,
    snapshot_id TEXT NOT NULL,
    evidence_id TEXT NOT NULL,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    locator TEXT NOT NULL CHECK (locator GLOB 'http://*' OR locator GLOB 'https://*'),
    deep_link_locator TEXT,
    deep_link_label TEXT NOT NULL DEFAULT '',
    snapshot_date TEXT NOT NULL CHECK (snapshot_date GLOB '*Z'),
    last_verified TEXT NOT NULL CHECK (last_verified GLOB '*Z' AND last_verified >= snapshot_date),
    FOREIGN KEY (snapshot_id, source_id) REFERENCES source_snapshots(id, source_id),
    FOREIGN KEY (evidence_id) REFERENCES evidence(id)
)`,
			`CREATE INDEX citations_last_verified_idx ON citations (last_verified, id)`,
			`CREATE TABLE source_bundles (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    run_id TEXT NOT NULL,
    topic_subject TEXT NOT NULL CHECK (length(trim(topic_subject)) > 0),
    topic_domain TEXT NOT NULL DEFAULT '',
    topic_technology TEXT NOT NULL DEFAULT '',
    purpose TEXT NOT NULL CHECK (purpose IN ('concept_definition','current_usage','version_behavior','release_status','deprecation_check','prerequisite_research','production_practice','security_guidance')),
    target_version TEXT,
    state TEXT NOT NULL CHECK (state IN ('ready','ready_with_caveats','incomplete','conflicted')),
    verified_at TEXT NOT NULL CHECK (verified_at GLOB '*Z'),
    FOREIGN KEY (run_id) REFERENCES research_runs(id)
)`,
			`CREATE TABLE source_bundle_items (
    bundle_id TEXT NOT NULL,
    item_type TEXT NOT NULL CHECK (item_type IN ('claim','source')),
    item_id TEXT NOT NULL CHECK (length(trim(item_id)) > 0),
    position INTEGER NOT NULL CHECK (position >= 0),
    PRIMARY KEY (bundle_id, item_type, item_id),
    UNIQUE (bundle_id, item_type, position),
    FOREIGN KEY (bundle_id) REFERENCES source_bundles(id) ON DELETE CASCADE
)`,
			`CREATE TABLE release_records (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    technology_id TEXT NOT NULL CHECK (length(trim(technology_id)) > 0),
    version TEXT NOT NULL CHECK (length(trim(version)) > 0),
    channel TEXT NOT NULL CHECK (channel IN ('stable','preview','beta','rc','experimental','nightly','unknown')),
    status TEXT NOT NULL CHECK (status IN ('current','superseded','legacy','eol','unknown')),
    source_ids_json TEXT NOT NULL CHECK (json_valid(source_ids_json) AND json_type(source_ids_json) = 'array' AND json_array_length(source_ids_json) > 0),
    released_at TEXT CHECK (released_at IS NULL OR released_at GLOB '*Z'),
    verified_at TEXT NOT NULL CHECK (verified_at GLOB '*Z')
)`,
			`CREATE INDEX release_records_technology_version_idx ON release_records (technology_id, version, verified_at, id)`,
			`CREATE TABLE deprecation_records (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    subject TEXT NOT NULL CHECK (length(trim(subject)) > 0),
    status TEXT NOT NULL CHECK (status IN ('deprecated','removed','legacy','historical_only','superseded')),
    introduced_in TEXT, deprecated_in TEXT, removed_in TEXT, replacement TEXT NOT NULL DEFAULT '',
    source_ids_json TEXT NOT NULL CHECK (json_valid(source_ids_json) AND json_array_length(source_ids_json) > 0),
    evidence_ids_json TEXT NOT NULL CHECK (json_valid(evidence_ids_json) AND json_array_length(evidence_ids_json) > 0),
    verified_at TEXT NOT NULL CHECK (verified_at GLOB '*Z')
)`,
			`CREATE INDEX deprecation_records_verified_idx ON deprecation_records (verified_at, id)`,
			`CREATE TABLE freshness_state (
    subject_id TEXT PRIMARY KEY CHECK (length(trim(subject_id)) > 0),
    state TEXT NOT NULL CHECK (state IN ('fresh','aging','stale','unknown')),
    score REAL NOT NULL CHECK (score BETWEEN 0 AND 1),
    last_verified_at TEXT NOT NULL CHECK (last_verified_at GLOB '*Z'),
    next_verify_at TEXT CHECK (next_verify_at IS NULL OR (next_verify_at GLOB '*Z' AND next_verify_at >= last_verified_at)),
    algorithm_version TEXT NOT NULL CHECK (length(trim(algorithm_version)) > 0)
)`,
			`CREATE INDEX freshness_state_due_idx ON freshness_state (next_verify_at, subject_id) WHERE next_verify_at IS NOT NULL`,
			`CREATE INDEX freshness_state_last_verified_idx ON freshness_state (last_verified_at, subject_id)`,
			`CREATE TABLE verification_results (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    claim_id TEXT NOT NULL CHECK (length(trim(claim_id)) > 0),
    status TEXT NOT NULL CHECK (status IN ('verified','verified_with_caveat','insufficient_evidence','conflicted','rejected')),
    source_ids_json TEXT NOT NULL CHECK (json_valid(source_ids_json) AND json_type(source_ids_json) = 'array' AND json_array_length(source_ids_json) > 0),
    confidence REAL NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    verified_at TEXT NOT NULL CHECK (verified_at GLOB '*Z')
)`,
			`CREATE INDEX verification_results_claim_idx ON verification_results (claim_id, verified_at DESC, id DESC)`,
			`CREATE TABLE source_conflicts (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    conflict_type TEXT NOT NULL CHECK (conflict_type IN ('direct_contradiction','version_mismatch','temporal_mismatch','scope_mismatch','recommendation_disagreement','authority_mismatch')),
    claim_ids_json TEXT NOT NULL CHECK (json_valid(claim_ids_json) AND json_array_length(claim_ids_json) >= 2),
    resolution TEXT NOT NULL DEFAULT '', unresolved INTEGER NOT NULL CHECK (unresolved IN (0,1)),
    detected_at TEXT NOT NULL CHECK (detected_at GLOB '*Z'),
    CHECK ((unresolved = 1 AND resolution = '') OR (unresolved = 0 AND length(trim(resolution)) > 0))
)`,
			`CREATE TABLE research_cache_entries (
    cache_key TEXT PRIMARY KEY CHECK (length(trim(cache_key)) > 0),
    payload BLOB NOT NULL CHECK (length(payload) BETWEEN 1 AND 1048576),
    content_hash TEXT NOT NULL CHECK (length(trim(content_hash)) > 0),
    stored_at TEXT NOT NULL CHECK (stored_at GLOB '*Z'),
    expires_at TEXT CHECK (expires_at IS NULL OR (expires_at GLOB '*Z' AND expires_at >= stored_at))
)`,
			`CREATE INDEX research_cache_expiry_idx ON research_cache_entries (expires_at, cache_key) WHERE expires_at IS NOT NULL`,
			`CREATE TABLE drift_reports (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0), old_bundle_id TEXT NOT NULL,
    new_bundle_id TEXT, drift_type TEXT NOT NULL CHECK (drift_type IN ('source_changed','claim_invalidated','version_superseded','recommendation_changed','deprecation_introduced','scope_changed')),
    severity TEXT NOT NULL CHECK (severity IN ('informational','minor','important','critical')),
    affected_claim_ids_json TEXT NOT NULL CHECK (json_valid(affected_claim_ids_json) AND json_array_length(affected_claim_ids_json) > 0),
    old_evidence_ids_json TEXT NOT NULL CHECK (json_valid(old_evidence_ids_json) AND json_array_length(old_evidence_ids_json) > 0),
    new_evidence_ids_json TEXT NOT NULL CHECK (json_valid(new_evidence_ids_json)),
    detected_at TEXT NOT NULL CHECK (detected_at GLOB '*Z')
)`,
			`CREATE TABLE impact_reports (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0), drift_report_id TEXT NOT NULL,
    affected_bundle_ids_json TEXT NOT NULL CHECK (json_valid(affected_bundle_ids_json) AND json_array_length(affected_bundle_ids_json) > 0),
    affected_claim_ids_json TEXT NOT NULL CHECK (json_valid(affected_claim_ids_json) AND json_array_length(affected_claim_ids_json) > 0),
    severity TEXT NOT NULL CHECK (severity IN ('informational','minor','important','critical')),
    recommended_action TEXT NOT NULL CHECK (recommended_action IN ('no_action','reverify','review_curriculum','recompile_future','manual_review')),
    assessed_at TEXT NOT NULL CHECK (assessed_at GLOB '*Z'),
    FOREIGN KEY (drift_report_id) REFERENCES drift_reports(id) ON DELETE CASCADE
)`,
		},
	},
	{
		version: 24,
		name:    "topic-aware authority profiles",
		statements: []string{
			`ALTER TABLE authority_profiles ADD COLUMN preferred_domains_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(preferred_domains_json) AND json_type(preferred_domains_json) = 'array')`,
			`ALTER TABLE authority_profiles ADD COLUMN preferred_organizations_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(preferred_organizations_json) AND json_type(preferred_organizations_json) = 'array')`,
			`ALTER TABLE authority_profiles ADD COLUMN minimum_corroboration INTEGER NOT NULL DEFAULT 1 CHECK (minimum_corroboration >= 1)`,
			`ALTER TABLE authority_profiles ADD COLUMN supplementary_kinds_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(supplementary_kinds_json) AND json_type(supplementary_kinds_json) = 'array')`,
		},
	},
	{
		version: 25,
		name:    "trusted source registry",
		statements: []string{
			`CREATE TABLE source_registry_entries (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    organization TEXT NOT NULL CHECK (length(trim(organization)) > 0),
    canonical_domains_json TEXT NOT NULL CHECK (json_valid(canonical_domains_json) AND json_type(canonical_domains_json) = 'array' AND json_array_length(canonical_domains_json) > 0),
    source_kinds_json TEXT NOT NULL CHECK (json_valid(source_kinds_json) AND json_type(source_kinds_json) = 'array' AND json_array_length(source_kinds_json) > 0),
    authority_hints_json TEXT NOT NULL CHECK (json_valid(authority_hints_json) AND json_type(authority_hints_json) = 'array' AND json_array_length(authority_hints_json) > 0),
    research_domains_json TEXT NOT NULL CHECK (json_valid(research_domains_json) AND json_type(research_domains_json) = 'array' AND json_array_length(research_domains_json) > 0),
    topic_patterns_json TEXT NOT NULL CHECK (json_valid(topic_patterns_json) AND json_type(topic_patterns_json) = 'array' AND json_array_length(topic_patterns_json) > 0),
    notes TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('trusted','conditional','historical','deprecated','blocked')),
    added_at TEXT NOT NULL CHECK (added_at GLOB '*Z'),
    last_reviewed_at TEXT NOT NULL CHECK (last_reviewed_at GLOB '*Z' AND last_reviewed_at >= added_at)
)`,
			`CREATE INDEX source_registry_status_idx ON source_registry_entries (status, organization, id)`,
			`CREATE TRIGGER source_registry_domains_insert_guard
BEFORE INSERT ON source_registry_entries
WHEN EXISTS (
    SELECT 1 FROM source_registry_entries AS existing,
                  json_each(existing.canonical_domains_json) AS old_domain,
                  json_each(NEW.canonical_domains_json) AS new_domain
    WHERE existing.id <> NEW.id
      AND lower(old_domain.value) = lower(new_domain.value)
)
BEGIN SELECT RAISE(ABORT, 'duplicate source registry domain'); END`,
			`CREATE TRIGGER source_registry_domains_update_guard
BEFORE UPDATE OF canonical_domains_json ON source_registry_entries
WHEN EXISTS (
    SELECT 1 FROM source_registry_entries AS existing,
                  json_each(existing.canonical_domains_json) AS old_domain,
                  json_each(NEW.canonical_domains_json) AS new_domain
    WHERE existing.id <> NEW.id
      AND lower(old_domain.value) = lower(new_domain.value)
)
BEGIN SELECT RAISE(ABORT, 'duplicate source registry domain'); END`,
		},
	},
	{
		version: 26,
		name:    "structured evidence and claim scopes",
		statements: []string{
			`ALTER TABLE evidence ADD COLUMN context_before TEXT NOT NULL DEFAULT '' CHECK ((context_before = '' OR length(trim(context_before)) > 0) AND length(CAST(context_before AS BLOB)) <= 2048)`,
			`ALTER TABLE evidence ADD COLUMN context_after TEXT NOT NULL DEFAULT '' CHECK ((context_after = '' OR length(trim(context_after)) > 0) AND length(CAST(context_after AS BLOB)) <= 2048)`,
			`ALTER TABLE claims ADD COLUMN scope TEXT NOT NULL DEFAULT 'general' CHECK (length(trim(scope)) > 0 AND length(CAST(scope AS BLOB)) <= 1024)`,
			`ALTER TABLE claims ADD COLUMN status_scope TEXT NOT NULL DEFAULT 'all' CHECK (status_scope IN ('all','stable','preview','experimental','legacy'))`,
		},
	},
	{
		version: 27,
		name:    "claim provenance graphs",
		statements: []string{
			`CREATE TABLE provenance_graphs (
    id TEXT PRIMARY KEY CHECK (length(trim(id)) > 0),
    claim_id TEXT NOT NULL CHECK (length(trim(claim_id)) > 0),
    graph_json TEXT NOT NULL CHECK (json_valid(graph_json) AND length(CAST(graph_json AS BLOB)) <= 262144),
    recorded_at TEXT NOT NULL CHECK (recorded_at GLOB '*Z'),
    algorithm_version TEXT NOT NULL CHECK (algorithm_version = 'provenance-graph-v1'),
    CHECK (COALESCE(json_type(graph_json, '$.graph_id') = 'text' AND json_extract(graph_json, '$.graph_id') = id, 0)),
    CHECK (COALESCE(json_type(graph_json, '$.claim_id') = 'text' AND json_extract(graph_json, '$.claim_id') = claim_id, 0)),
    CHECK (COALESCE(json_type(graph_json, '$.algorithm_version') = 'text' AND json_extract(graph_json, '$.algorithm_version') = algorithm_version, 0))
)`,
			`CREATE INDEX provenance_graphs_claim_latest_idx ON provenance_graphs (claim_id, recorded_at DESC, id DESC)`,
		},
	},
	{
		version: 28,
		name:    "stable citation deep links",
		statements: []string{
			`ALTER TABLE citations ADD COLUMN link_strategy TEXT NOT NULL DEFAULT 'canonical_fallback' CHECK (link_strategy IN ('url_anchor','package_symbol','spec_section','release_heading','source_permalink','canonical_fallback'))`,
			`ALTER TABLE citations ADD COLUMN section TEXT NOT NULL DEFAULT 'unspecified' CHECK (length(trim(section)) > 0 AND length(CAST(section AS BLOB)) <= 2048)`,
			`ALTER TABLE citations ADD COLUMN version_scope TEXT`,
			`ALTER TABLE citations ADD COLUMN algorithm_version TEXT NOT NULL DEFAULT 'citation-v1' CHECK (algorithm_version = 'citation-v1')`,
			`UPDATE citations SET link_strategy='url_anchor',deep_link_label=CASE WHEN deep_link_label='' THEN section ELSE deep_link_label END WHERE deep_link_locator IS NOT NULL`,
			`CREATE TRIGGER citations_deep_link_insert BEFORE INSERT ON citations WHEN length(CAST(NEW.deep_link_label AS BLOB)) > 2048 OR (NEW.link_strategy='canonical_fallback' AND NEW.deep_link_locator IS NOT NULL) OR (NEW.link_strategy<>'canonical_fallback' AND NEW.deep_link_locator IS NULL) BEGIN SELECT RAISE(ABORT, 'invalid citation deep link'); END`,
			`CREATE TRIGGER citations_deep_link_update BEFORE UPDATE ON citations WHEN length(CAST(NEW.deep_link_label AS BLOB)) > 2048 OR (NEW.link_strategy='canonical_fallback' AND NEW.deep_link_locator IS NOT NULL) OR (NEW.link_strategy<>'canonical_fallback' AND NEW.deep_link_locator IS NULL) BEGIN SELECT RAISE(ABORT, 'invalid citation deep link'); END`,
			`CREATE INDEX citations_evidence_idx ON citations (evidence_id, id)`,
		},
	},
	{
		version: 29,
		name:    "authority freshness TTL hints",
		statements: []string{
			`ALTER TABLE authority_profiles ADD COLUMN freshness_ttl_hints_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(freshness_ttl_hints_json) AND json_type(freshness_ttl_hints_json) = 'array' AND json_array_length(freshness_ttl_hints_json) <= 64)`,
		},
	},
	{
		version: 30,
		name:    "refresh scheduling metadata",
		statements: []string{
			`ALTER TABLE freshness_state ADD COLUMN scheduling_json TEXT NOT NULL DEFAULT '{"verification_reason":"ttl_expired","priority":"normal","algorithm_version":"refresh-scheduling-v1"}' CHECK (json_valid(scheduling_json) AND json_type(scheduling_json) = 'object' AND length(CAST(scheduling_json AS BLOB)) <= 256 AND json_remove(scheduling_json,'$.verification_reason','$.priority','$.algorithm_version') = '{}' AND COALESCE(json_extract(scheduling_json,'$.verification_reason') IN ('ttl_expired','new_release_detected','source_changed','conflict_unresolved','security_sensitive','manual_request'),0) AND COALESCE(json_extract(scheduling_json,'$.priority') IN ('normal','high','critical'),0) AND COALESCE(json_extract(scheduling_json,'$.algorithm_version') = 'refresh-scheduling-v1',0))`,
		},
	},
	{
		version: 31,
		name:    "versioned deprecation intelligence",
		statements: []string{
			`ALTER TABLE deprecation_records ADD COLUMN determination TEXT NOT NULL DEFAULT 'legacy_unclassified' CHECK (determination IN ('explicit_evidence','multi_source_strong_inference','legacy_unclassified'))`,
			`ALTER TABLE deprecation_records ADD COLUMN algorithm_version TEXT NOT NULL DEFAULT 'deprecation-unversioned-legacy' CHECK ((algorithm_version = 'deprecation-intelligence-v1' AND determination IN ('explicit_evidence','multi_source_strong_inference')) OR (algorithm_version = 'deprecation-unversioned-legacy' AND determination = 'legacy_unclassified')) CHECK (determination <> 'multi_source_strong_inference' OR (json_array_length(source_ids_json) >= 2 AND json_array_length(evidence_ids_json) >= 2))`,
			`CREATE INDEX deprecation_records_subject_history_idx ON deprecation_records (subject, verified_at, id)`,
		},
	},
	{
		version: 32,
		name:    "historical source temporal scopes",
		statements: []string{
			`ALTER TABLE sources ADD COLUMN temporal_scope TEXT NOT NULL DEFAULT 'current' CHECK (temporal_scope IN ('current','historical','version_bound','archived')) CHECK (temporal_scope <> 'version_bound' OR version IS NOT NULL)`,
			`ALTER TABLE citations ADD COLUMN temporal_scope TEXT NOT NULL DEFAULT 'current' CHECK (temporal_scope IN ('current','historical','version_bound','archived')) CHECK (temporal_scope <> 'version_bound' OR version_scope IS NOT NULL)`,
			`ALTER TABLE citations ADD COLUMN temporal_warning TEXT NOT NULL DEFAULT '' CHECK ((temporal_scope = 'current' AND temporal_warning = '') OR (temporal_scope <> 'current' AND length(trim(temporal_warning)) > 0))`,
			`ALTER TABLE citations ADD COLUMN temporal_algorithm_version TEXT NOT NULL DEFAULT 'source-temporal-legacy-current' CHECK ((temporal_algorithm_version = 'source-temporal-legacy-current' AND temporal_scope = 'current' AND temporal_warning = '') OR temporal_algorithm_version = 'source-temporal-policy-v1')`,
			`ALTER TABLE source_bundle_items ADD COLUMN temporal_scope TEXT CHECK (temporal_scope IS NULL OR temporal_scope IN ('current','historical','version_bound','archived'))`,
			`UPDATE source_bundle_items SET temporal_scope='current' WHERE item_type='source'`,
			`CREATE TRIGGER source_bundle_item_temporal_insert_guard BEFORE INSERT ON source_bundle_items WHEN (NEW.item_type='source' AND NEW.temporal_scope IS NULL) OR (NEW.item_type='claim' AND NEW.temporal_scope IS NOT NULL) BEGIN SELECT RAISE(ABORT, 'invalid source bundle temporal scope'); END`,
			`CREATE TRIGGER source_bundle_item_temporal_update_guard BEFORE UPDATE ON source_bundle_items WHEN (NEW.item_type='source' AND NEW.temporal_scope IS NULL) OR (NEW.item_type='claim' AND NEW.temporal_scope IS NOT NULL) BEGIN SELECT RAISE(ABORT, 'invalid source bundle temporal scope'); END`,
		},
	},
	{
		version: 33,
		name:    "versioned conflict resolver outcomes",
		statements: []string{
			`ALTER TABLE source_conflicts ADD COLUMN confidence REAL NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 1)`,
			`ALTER TABLE source_conflicts ADD COLUMN reason TEXT NOT NULL DEFAULT 'Legacy conflict record without a resolver explanation.' CHECK (length(trim(reason)) > 0)`,
			`ALTER TABLE source_conflicts ADD COLUMN winning_claim_id TEXT`,
			`ALTER TABLE source_conflicts ADD COLUMN winning_source_id TEXT`,
			`ALTER TABLE source_conflicts ADD COLUMN winning_scope TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE source_conflicts ADD COLUMN algorithm_version TEXT NOT NULL DEFAULT 'conflict-unversioned-legacy' CHECK (algorithm_version IN ('conflict-resolver-v1','conflict-unversioned-legacy')) CHECK (algorithm_version <> 'conflict-resolver-v1' OR json_array_length(claim_ids_json) = 2) CHECK ((winning_claim_id IS NULL) = (winning_source_id IS NULL)) CHECK ((winning_claim_id IS NULL AND winning_scope = '') OR (winning_claim_id IS NOT NULL AND length(trim(winning_scope)) > 0)) CHECK (unresolved = 0 OR winning_claim_id IS NULL) CHECK (algorithm_version <> 'conflict-unversioned-legacy' OR winning_claim_id IS NULL)`,
			`CREATE INDEX source_conflicts_detected_idx ON source_conflicts (detected_at, id)`,
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

	if _, err := database.sql.ExecContext(operationContext, migrationTableSQL); err != nil {
		cancel()
		return fmt.Errorf("initialize SQLite migration history: %w", err)
	}

	applied, err := loadAppliedMigrations(operationContext, database.sql)
	cancel()
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
			backupContext, cancelBackup := database.operationContext(ctx)
			err := database.backup(backupContext, database.path, MigrationInfo{
				Version: next.version,
				Name:    next.name,
			})
			cancelBackup()
			if err != nil {
				return fmt.Errorf("backup before migration %d (%s): %w", next.version, next.name, err)
			}
		}
		migrationContext, cancelMigration := database.operationContext(ctx)
		err := database.applyMigration(migrationContext, next)
		cancelMigration()
		if err != nil {
			return err
		}
		if next.version >= 3 {
			if err := database.Repositories().Audit.Record(ctx, audit.Event{
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
