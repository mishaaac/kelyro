package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type integrityQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type studentCoreIntegrityCheck struct {
	minimumVersion int
	name           string
	query          string
}

var studentCoreIntegrityChecks = []studentCoreIntegrityCheck{
	{4, "duplicate active learning goal", `SELECT COUNT(*) FROM (
SELECT student_id FROM learning_goals WHERE status='active' GROUP BY student_id HAVING COUNT(*)>1)`},
	{4, "invalid legacy concept mastery", `SELECT COUNT(*) FROM student_concept_states
WHERE mastery IS NULL OR mastery < 0 OR mastery > 1`},
	{4, "duplicate pending review", `SELECT COUNT(*) FROM (
SELECT student_id,concept_id FROM review_items WHERE status='pending' GROUP BY student_id,concept_id HAVING COUNT(*)>1)`},
	{4, "duplicate student achievement", `SELECT COUNT(*) FROM (
SELECT student_id,achievement_key FROM student_achievements GROUP BY student_id,achievement_key HAVING COUNT(*)>1)`},
	{9, "invalid curriculum-scoped mastery", `SELECT COUNT(*) FROM learner_curriculum_concept_states
WHERE mastery IS NULL OR mastery < 0 OR mastery > 1`},
	{9, "curriculum state outside its instance definition", `SELECT COUNT(*)
FROM learner_curriculum_concept_states AS state
JOIN learner_curriculum_instances AS instance
  ON instance.id=state.curriculum_instance_id AND instance.student_id=state.student_id
LEFT JOIN curriculum_nodes AS node
  ON node.curriculum_id=instance.curriculum_id AND node.curriculum_version=instance.curriculum_version
 AND node.node_id=state.concept_id AND node.node_type='concept'
WHERE node.node_id IS NULL`},
	{10, "diagnostic observation ownership mismatch", `SELECT COUNT(*)
FROM diagnostic_observations AS observation
JOIN diagnostic_attempts AS attempt ON attempt.id=observation.attempt_id
JOIN learner_curriculum_instances AS instance
  ON instance.id=attempt.curriculum_instance_id AND instance.student_id=attempt.student_id
JOIN learning_evidence AS evidence ON evidence.id=observation.evidence_id
LEFT JOIN curriculum_nodes AS node
  ON node.curriculum_id=instance.curriculum_id AND node.curriculum_version=instance.curriculum_version
 AND node.node_id=observation.concept_id AND node.node_type='concept'
WHERE evidence.student_id<>attempt.student_id OR evidence.concept_id<>observation.concept_id OR node.node_id IS NULL`},
	{14, "duplicate active study session", `SELECT COUNT(*) FROM (
SELECT student_id FROM study_session_lifecycle WHERE status='active' GROUP BY student_id HAVING COUNT(*)>1)`},
}

func checkRelationalIntegrity(ctx context.Context, queryer integrityQueryer) error {
	rows, err := queryer.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return sqliteOperationError("check SQLite foreign-key integrity", err)
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("%w: foreign-key violation", ErrIntegrity)
	}
	if err := rows.Err(); err != nil {
		return sqliteOperationError("check SQLite foreign-key integrity", err)
	}
	return nil
}

func checkStudentCoreIntegrity(ctx context.Context, queryer integrityQueryer, schemaVersion int) error {
	for _, check := range studentCoreIntegrityChecks {
		if schemaVersion < check.minimumVersion {
			continue
		}
		var findings int
		if err := queryer.QueryRowContext(ctx, check.query).Scan(&findings); err != nil {
			return sqliteOperationError("check Student Core integrity", err)
		}
		if findings != 0 {
			return fmt.Errorf("%w: %s", ErrIntegrity, check.name)
		}
	}
	return checkStudentCoreTimezones(ctx, queryer, schemaVersion)
}

func checkStudentCoreTimezones(ctx context.Context, queryer integrityQueryer, schemaVersion int) error {
	if schemaVersion < 5 {
		return nil
	}
	query := "SELECT timezone FROM student_profiles"
	if schemaVersion >= 18 {
		query += " UNION SELECT streak_timezone FROM streak_state WHERE policy_version='streak-v1'"
	}
	if schemaVersion >= 20 {
		query += " UNION SELECT timezone FROM daily_plans WHERE policy_version='daily-plan-v1'"
	}
	rows, err := queryer.QueryContext(ctx, query)
	if err != nil {
		return sqliteOperationError("check Student Core timezones", err)
	}
	defer rows.Close()
	for rows.Next() {
		var timezone string
		if err := rows.Scan(&timezone); err != nil {
			return sqliteOperationError("read Student Core timezone", err)
		}
		if _, err := time.LoadLocation(timezone); err != nil {
			return fmt.Errorf("%w: invalid Student Core timezone", ErrIntegrity)
		}
	}
	if err := rows.Err(); err != nil {
		return sqliteOperationError("check Student Core timezones", err)
	}
	return nil
}
