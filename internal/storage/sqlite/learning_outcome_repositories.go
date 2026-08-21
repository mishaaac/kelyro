package sqlite

import (
	"context"
	"database/sql"

	"github.com/mishaaac/kelyro/internal/learning"
)

func (repository learningStreakRepository) Get(ctx context.Context, studentID learning.ID) (learning.Streak, error) {
	const operation = "get SQLite streak"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var studentValue, timezone, policyVersion string
	var current, longest, totalActive, minimumMinutes int
	var lastValue, lastLocalDateValue sql.NullString
	err := repository.executor.QueryRowContext(operationContext, `SELECT student_id,current_days,longest_days,last_study_at,
last_active_local_date,total_active_days,streak_timezone,minimum_active_minutes,policy_version
FROM streak_state WHERE student_id=?`, studentID.String()).Scan(&studentValue, &current, &longest, &lastValue,
		&lastLocalDateValue, &totalActive, &timezone, &minimumMinutes, &policyVersion)
	if err != nil {
		return learning.Streak{}, classifyLearningError(operation, err)
	}
	student, err := decodeID(studentValue)
	if err != nil {
		return learning.Streak{}, corruptLearning(operation, err)
	}
	last, err := decodeOptionalTimestamp(lastValue)
	if err != nil {
		return learning.Streak{}, corruptLearning(operation, err)
	}
	var lastLocalDate *learning.LocalDate
	if lastLocalDateValue.Valid {
		date, dateErr := learning.NewLocalDate(lastLocalDateValue.String)
		if dateErr != nil {
			return learning.Streak{}, corruptLearning(operation, dateErr)
		}
		lastLocalDate = &date
	}
	streak := learning.Streak{
		StudentID: student, CurrentDays: current, LongestDays: longest, LastStudyAt: last,
		LastActiveLocalDate: lastLocalDate, TotalActiveDays: totalActive, Timezone: timezone,
		MinimumActiveMinutes: minimumMinutes, PolicyVersion: policyVersion,
	}
	if err := streak.Validate(); err != nil {
		return learning.Streak{}, corruptLearning(operation, err)
	}
	return streak, nil
}

func (repository learningStreakRepository) Save(ctx context.Context, streak learning.Streak) error {
	const operation = "save SQLite streak"
	if err := streak.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var lastLocalDate any
	if streak.LastActiveLocalDate != nil {
		lastLocalDate = streak.LastActiveLocalDate.String()
	}
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO streak_state
(student_id,current_days,longest_days,last_study_at,last_active_local_date,total_active_days,streak_timezone,minimum_active_minutes,policy_version)
VALUES (?,?,?,?,?,?,?,?,?) ON CONFLICT(student_id) DO UPDATE SET
current_days=excluded.current_days,longest_days=excluded.longest_days,last_study_at=excluded.last_study_at,
last_active_local_date=excluded.last_active_local_date,total_active_days=excluded.total_active_days,
streak_timezone=excluded.streak_timezone,minimum_active_minutes=excluded.minimum_active_minutes,policy_version=excluded.policy_version`,
		streak.StudentID.String(), streak.CurrentDays, streak.LongestDays, encodeOptionalTimestamp(streak.LastStudyAt),
		lastLocalDate, streak.TotalActiveDays, streak.Timezone, streak.MinimumActiveMinutes, streak.PolicyVersion)
	return classifyLearningError(operation, err)
}

func (repository learningAchievementRepository) Get(ctx context.Context, id learning.ID) (learning.Achievement, error) {
	const operation = "get SQLite achievement"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	item, err := scanAchievement(repository.executor.QueryRowContext(operationContext, `SELECT a.id,a.student_id,a.achievement_key,d.name,a.status,a.unlocked_at FROM student_achievements a JOIN achievement_definitions d ON d.key=a.achievement_key WHERE a.id=?`, id.String()))
	if err != nil {
		return learning.Achievement{}, classifyLearningError(operation, err)
	}
	return item, nil
}

func (repository learningAchievementRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.Achievement, error) {
	const operation = "list SQLite achievements"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, `SELECT a.id,a.student_id,a.achievement_key,d.name,a.status,a.unlocked_at FROM student_achievements a JOIN achievement_definitions d ON d.key=a.achievement_key WHERE a.student_id=? ORDER BY a.id`, studentID.String())
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	items := make([]learning.Achievement, 0)
	for rows.Next() {
		item, err := scanAchievement(rows)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return items, nil
}

func (repository learningAchievementRepository) Save(ctx context.Context, item learning.Achievement) error {
	const operation = "save SQLite achievement"
	if err := item.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	return repository.atomic(ctx, operation, func(ctx context.Context, target executor) error {
		if _, err := target.ExecContext(ctx, `INSERT INTO achievement_definitions (key,name) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET name=excluded.name`, item.Key.String(), item.Name); err != nil {
			return err
		}
		_, err := target.ExecContext(ctx, `INSERT INTO student_achievements (id,student_id,achievement_key,status,unlocked_at) VALUES (?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET student_id=excluded.student_id,achievement_key=excluded.achievement_key,status=excluded.status,unlocked_at=excluded.unlocked_at`, item.ID.String(), item.StudentID.String(), item.Key.String(), item.Status, encodeOptionalTimestamp(item.UnlockedAt))
		return err
	})
}

func scanAchievement(scanner rowScanner) (learning.Achievement, error) {
	var idValue, studentValue, keyValue, name, status string
	var unlockedValue sql.NullString
	if err := scanner.Scan(&idValue, &studentValue, &keyValue, &name, &status, &unlockedValue); err != nil {
		return learning.Achievement{}, err
	}
	id, err := decodeID(idValue)
	if err != nil {
		return learning.Achievement{}, err
	}
	studentID, err := decodeID(studentValue)
	if err != nil {
		return learning.Achievement{}, err
	}
	key, err := decodeID(keyValue)
	if err != nil {
		return learning.Achievement{}, err
	}
	unlockedAt, err := decodeOptionalTimestamp(unlockedValue)
	if err != nil {
		return learning.Achievement{}, err
	}
	item := learning.Achievement{ID: id, StudentID: studentID, Key: key, Name: name, Status: learning.AchievementStatus(status), UnlockedAt: unlockedAt}
	return item, item.Validate()
}

func (repository learningAchievementRepository) AppendMilestone(ctx context.Context, item learning.Milestone) error {
	const operation = "append SQLite milestone"
	if err := item.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO milestones (id,student_id,goal_id,name,reached_at) VALUES (?,?,?,?,?)`, item.ID.String(), item.StudentID.String(), item.GoalID.String(), item.Name, encodeTimestamp(item.ReachedAt))
	return classifyLearningError(operation, err)
}

func (repository learningAchievementRepository) ListMilestones(ctx context.Context, studentID, goalID learning.ID) ([]learning.Milestone, error) {
	const operation = "list SQLite milestones"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, `SELECT id,student_id,goal_id,name,reached_at FROM milestones WHERE student_id=? AND goal_id=? ORDER BY reached_at,id`, studentID.String(), goalID.String())
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	items := make([]learning.Milestone, 0)
	for rows.Next() {
		var idValue, studentValue, goalValue, name, reachedValue string
		if err := rows.Scan(&idValue, &studentValue, &goalValue, &name, &reachedValue); err != nil {
			return nil, corruptLearning(operation, err)
		}
		id, err := decodeID(idValue)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		student, err := decodeID(studentValue)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		goal, err := decodeID(goalValue)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		reached, err := decodeTimestamp(reachedValue)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		item := learning.Milestone{ID: id, StudentID: student, GoalID: goal, Name: name, ReachedAt: reached}
		if err := item.Validate(); err != nil {
			return nil, corruptLearning(operation, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return items, nil
}

func (repository learningAnalyticsRepository) Append(ctx context.Context, item learning.AnalyticsSnapshot) error {
	const operation = "append SQLite analytics"
	if err := item.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO analytics_snapshots (student_id,captured_at,study_minutes,sessions_completed,concepts_introduced,concepts_mastered,reviews_due) VALUES (?,?,?,?,?,?,?)`, item.StudentID.String(), encodeTimestamp(item.CapturedAt), item.StudyMinutes, item.SessionsCompleted, item.ConceptsIntroduced, item.ConceptsMastered, item.ReviewsDue)
	return classifyLearningError(operation, err)
}

func (repository learningAnalyticsRepository) Latest(ctx context.Context, studentID learning.ID) (learning.AnalyticsSnapshot, error) {
	const operation = "get latest SQLite analytics"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var studentValue, capturedValue string
	var item learning.AnalyticsSnapshot
	err := repository.executor.QueryRowContext(operationContext, `SELECT student_id,captured_at,study_minutes,sessions_completed,concepts_introduced,concepts_mastered,reviews_due FROM analytics_snapshots WHERE student_id=? ORDER BY captured_at DESC LIMIT 1`, studentID.String()).Scan(&studentValue, &capturedValue, &item.StudyMinutes, &item.SessionsCompleted, &item.ConceptsIntroduced, &item.ConceptsMastered, &item.ReviewsDue)
	if err != nil {
		return learning.AnalyticsSnapshot{}, classifyLearningError(operation, err)
	}
	student, err := decodeID(studentValue)
	if err != nil {
		return learning.AnalyticsSnapshot{}, corruptLearning(operation, err)
	}
	captured, err := decodeTimestamp(capturedValue)
	if err != nil {
		return learning.AnalyticsSnapshot{}, corruptLearning(operation, err)
	}
	item.StudentID = student
	item.CapturedAt = captured
	if err := item.Validate(); err != nil {
		return learning.AnalyticsSnapshot{}, corruptLearning(operation, err)
	}
	return item, nil
}

func (repository learningDailyPlanRepository) Save(ctx context.Context, plan learning.DailyPlan) error {
	const operation = "save SQLite daily plan"
	if err := plan.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	return repository.atomic(ctx, operation, func(ctx context.Context, target executor) error {
		if _, err := target.ExecContext(ctx, "DELETE FROM daily_plans WHERE student_id=? AND goal_id=? AND plan_date=?", plan.StudentID.String(), plan.GoalID.String(), encodeTimestamp(plan.Date)); err != nil {
			return err
		}
		if _, err := target.ExecContext(ctx, `INSERT INTO daily_plans (id,student_id,goal_id,plan_date,created_at) VALUES (?,?,?,?,?)`, plan.ID.String(), plan.StudentID.String(), plan.GoalID.String(), encodeTimestamp(plan.Date), encodeTimestamp(plan.CreatedAt)); err != nil {
			return err
		}
		for _, item := range plan.Items {
			if _, err := target.ExecContext(ctx, `INSERT INTO daily_plan_items (id,plan_id,item_type,estimated_minutes,position) VALUES (?,?,?,?,?)`, item.ID.String(), plan.ID.String(), item.Type, item.EstimatedMinutes, item.Position); err != nil {
				return err
			}
			for position, conceptID := range item.ConceptIDs {
				if _, err := target.ExecContext(ctx, `INSERT INTO daily_plan_item_concepts (item_id,concept_id,position) VALUES (?,?,?)`, item.ID.String(), conceptID.String(), position); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (repository learningDailyPlanRepository) ForDate(ctx context.Context, studentID, goalID learning.ID, date learning.Timestamp) (learning.DailyPlan, error) {
	const operation = "get SQLite daily plan"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var idValue, studentValue, goalValue, dateValue, createdValue string
	err := repository.executor.QueryRowContext(operationContext, `SELECT id,student_id,goal_id,plan_date,created_at FROM daily_plans WHERE student_id=? AND goal_id=? AND plan_date=?`, studentID.String(), goalID.String(), encodeTimestamp(date)).Scan(&idValue, &studentValue, &goalValue, &dateValue, &createdValue)
	if err != nil {
		return learning.DailyPlan{}, classifyLearningError(operation, err)
	}
	id, err := decodeID(idValue)
	if err != nil {
		return learning.DailyPlan{}, corruptLearning(operation, err)
	}
	student, err := decodeID(studentValue)
	if err != nil {
		return learning.DailyPlan{}, corruptLearning(operation, err)
	}
	goal, err := decodeID(goalValue)
	if err != nil {
		return learning.DailyPlan{}, corruptLearning(operation, err)
	}
	planDate, err := decodeTimestamp(dateValue)
	if err != nil {
		return learning.DailyPlan{}, corruptLearning(operation, err)
	}
	created, err := decodeTimestamp(createdValue)
	if err != nil {
		return learning.DailyPlan{}, corruptLearning(operation, err)
	}
	plan := learning.DailyPlan{ID: id, StudentID: student, GoalID: goal, Date: planDate, CreatedAt: created}
	rows, err := repository.executor.QueryContext(operationContext, `SELECT id,item_type,estimated_minutes,position FROM daily_plan_items WHERE plan_id=? ORDER BY position`, id.String())
	if err != nil {
		return learning.DailyPlan{}, classifyLearningError(operation, err)
	}
	for rows.Next() {
		var itemIDValue, itemType string
		var minutes, position int
		if err := rows.Scan(&itemIDValue, &itemType, &minutes, &position); err != nil {
			_ = rows.Close()
			return learning.DailyPlan{}, corruptLearning(operation, err)
		}
		itemID, err := decodeID(itemIDValue)
		if err != nil {
			_ = rows.Close()
			return learning.DailyPlan{}, corruptLearning(operation, err)
		}
		plan.Items = append(plan.Items, learning.DailyPlanItem{ID: itemID, Type: learning.DailyPlanItemType(itemType), EstimatedMinutes: minutes, Position: position})
	}
	if err := rows.Close(); err != nil {
		return learning.DailyPlan{}, classifyLearningError(operation, err)
	}
	if err := rows.Err(); err != nil {
		return learning.DailyPlan{}, classifyLearningError(operation, err)
	}
	for index := range plan.Items {
		values, err := queryStrings(operationContext, repository.executor, "SELECT concept_id FROM daily_plan_item_concepts WHERE item_id=? ORDER BY position", plan.Items[index].ID.String())
		if err != nil {
			return learning.DailyPlan{}, classifyLearningError(operation, err)
		}
		for _, value := range values {
			conceptID, err := decodeID(value)
			if err != nil {
				return learning.DailyPlan{}, corruptLearning(operation, err)
			}
			plan.Items[index].ConceptIDs = append(plan.Items[index].ConceptIDs, conceptID)
		}
	}
	if err := plan.Validate(); err != nil {
		return learning.DailyPlan{}, corruptLearning(operation, err)
	}
	return plan, nil
}
