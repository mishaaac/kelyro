package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/platform"
)

func TestFoundationDatabaseMigratesToStudentCoreWithoutLosingState(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, err := platform.WorkspaceDBPath(root)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:3]); err != nil {
		t.Fatalf("migrate Foundation: %v", err)
	}
	if _, err := handle.Exec(`INSERT INTO app_state (namespace,key,value,updated_at) VALUES ('foundation','kept',X'6F6B',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate Student Core: %v", err)
	}
	var value []byte
	if err := handle.QueryRow(`SELECT value FROM app_state WHERE namespace='foundation' AND key='kept'`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if string(value) != "ok" {
		t.Fatalf("Foundation value=%q", value)
	}
	version, err := database.SchemaVersion(context.Background())
	if err != nil || version != LatestSchemaVersion() {
		t.Fatalf("schema=(%d,%v), want %d", version, err, LatestSchemaVersion())
	}
}

func TestIntegratedSetupMigrationIsForwardOnlyAndPreservesExistingState(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:10]); err != nil {
		t.Fatalf("migrate through v10: %v", err)
	}
	if _, err := handle.Exec(`INSERT INTO app_state (namespace,key,value,updated_at) VALUES ('foundation','step12-kept',X'6F6B',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate setup: %v", err)
	}
	var value []byte
	if err := handle.QueryRow(`SELECT value FROM app_state WHERE namespace='foundation' AND key='step12-kept'`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if string(value) != "ok" {
		t.Fatalf("Foundation value=%q", value)
	}
	var table string
	if err := handle.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='learner_setups'`).Scan(&table); err != nil || table != "learner_setups" {
		t.Fatalf("learner setup table = (%q, %v)", table, err)
	}
}

func TestMasteryEvidenceMigrationPreservesAndClassifiesLegacyRows(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:11]); err != nil {
		t.Fatalf("migrate through v11: %v", err)
	}
	timestamp := fixedTime.Format(timestampFormat)
	if _, err := handle.Exec(`INSERT INTO students (id,created_at,updated_at) VALUES ('student.legacy',?,?)`, timestamp, timestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO concept_registry (id) VALUES ('concept.legacy')`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO learning_evidence
(id,student_id,concept_id,evidence_type,source,score,observed_at) VALUES
('evidence.pass','student.legacy','concept.legacy','practice','legacy/pass',0.7,?),
('evidence.fail','student.legacy','concept.legacy','practice','legacy/fail',0,?)`, timestamp, timestamp); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate mastery evidence: %v", err)
	}
	items, err := database.LearningRepositories().Evidence.ListByConcept(context.Background(), mustID(t, "student.legacy"), mustID(t, "concept.legacy"))
	if err != nil || len(items) != 2 {
		t.Fatalf("legacy evidence = (%+v, %v)", items, err)
	}
	byID := map[string]learning.Evidence{items[0].ID.String(): items[0], items[1].ID.String(): items[1]}
	if byID["evidence.pass"].Type != learning.EvidencePracticeSuccess || byID["evidence.fail"].Type != learning.EvidencePracticeFailure ||
		byID["evidence.pass"].AlgorithmVersion != learning.LegacyEvidenceAlgorithmVersion {
		t.Fatalf("classified legacy evidence = %+v", items)
	}
}

func TestMistakeMemoryMigrationPreservesLegacyHistory(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:12]); err != nil {
		t.Fatalf("migrate through v12: %v", err)
	}
	first := fixedTime.Format(timestampFormat)
	resolved := fixedTime.Add(time.Hour).Format(timestampFormat)
	if _, err := handle.Exec(`INSERT INTO students (id,created_at,updated_at) VALUES ('student.legacy',?,?)`, first, first); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO concept_registry (id) VALUES ('concept.legacy')`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO mistakes (id,student_id,concept_id,description,occurred_at,resolved_at)
VALUES ('mistake.legacy','student.legacy','concept.legacy','Legacy confusion',?,?)`, first, resolved); err != nil {
		t.Fatal(err)
	}
	longID := "mistake." + strings.Repeat("x", 180)
	longDescription := strings.Repeat("detail", 120)
	if _, err := handle.Exec(`INSERT INTO mistakes (id,student_id,concept_id,description,occurred_at,resolved_at)
VALUES (?,'student.legacy','concept.legacy',?,?,NULL)`, longID, longDescription, first); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate mistake memory: %v", err)
	}
	mistake, err := database.LearningRepositories().Mistakes.Get(context.Background(), mustID(t, "student.legacy"), mustID(t, "mistake.legacy"))
	if err != nil || mistake.Key != "legacy:mistake.legacy" || mistake.Category != learning.MistakeUnknown || mistake.Status != learning.MistakeResolved || mistake.Occurrences != 1 {
		t.Fatalf("legacy mistake = (%+v, %v)", mistake, err)
	}
	longMistake, err := database.LearningRepositories().Mistakes.Get(context.Background(), mustID(t, "student.legacy"), mustID(t, longID))
	if err != nil || len(longMistake.Key) > learning.MaxMistakeKeyLength || len(longMistake.Summary) != learning.MaxMistakeSummaryLength {
		t.Fatalf("bounded legacy mistake = (key=%q summary_len=%d, %v)", longMistake.Key, len(longMistake.Summary), err)
	}
	var preservedDescription string
	if err := handle.QueryRow(`SELECT description FROM mistakes WHERE id = ?`, longID).Scan(&preservedDescription); err != nil || preservedDescription != longDescription {
		t.Fatalf("legacy description = (len=%d, %v), want len=%d", len(preservedDescription), err, len(longDescription))
	}
	history, err := database.LearningRepositories().Mistakes.ListEvents(context.Background(), mistake.ID)
	if err != nil || len(history) != 2 || history[0].Type != learning.MistakeObservedEvent || history[1].Type != learning.MistakeResolvedEvent {
		t.Fatalf("legacy history = (%+v, %v)", history, err)
	}
}

func TestStudySessionLifecycleMigrationPreservesLegacySessions(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:13]); err != nil {
		t.Fatalf("migrate through v13: %v", err)
	}
	repositories := database.LearningRepositories()
	student := testStudent(t)
	goal := testGoal(t, student.ID)
	if err := repositories.Students.Create(context.Background(), student); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Goals.Create(context.Background(), goal); err != nil {
		t.Fatal(err)
	}
	started := mustTimestamp(t, fixedTime.Add(time.Minute))
	legacy, err := learning.NewLearningSession(mustID(t, "session.legacy"), student.ID, goal.ID, started, mustTimestamp(t, fixedTime.Add(2*time.Minute)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.Sessions.Append(context.Background(), legacy); err != nil {
		t.Fatal(err)
	}

	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate study session lifecycle: %v", err)
	}
	loaded, err := database.LearningRepositories().Sessions.Get(context.Background(), legacy.ID)
	if err != nil || loaded.ID != legacy.ID || loaded.StudentID != legacy.StudentID || loaded.GoalID != legacy.GoalID ||
		loaded.StartedAt != legacy.StartedAt || loaded.EndedAt != legacy.EndedAt || len(loaded.Activities) != 0 {
		t.Fatalf("legacy session = (%+v, %v), want %+v", loaded, err, legacy)
	}
	var lifecycleCount int
	if err := handle.QueryRow(`SELECT COUNT(*) FROM study_session_lifecycle`).Scan(&lifecycleCount); err != nil || lifecycleCount != 0 {
		t.Fatalf("lifecycle rows = (%d, %v), want no fabricated rows", lifecycleCount, err)
	}
}

func TestStudyHistoryMigrationBackfillsEducationalFactsInUTC(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:14]); err != nil {
		t.Fatalf("migrate through v14: %v", err)
	}
	ctx := context.Background()
	repositories := database.LearningRepositories()
	student := testStudent(t)
	goal := testGoal(t, student.ID)
	if err := repositories.Students.Create(ctx, student); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Goals.Create(ctx, goal); err != nil {
		t.Fatal(err)
	}
	reference := learning.CurriculumRef{ID: mustID(t, "curriculum.history"), Version: "1.0.0"}
	concept := learning.Concept{ID: mustID(t, "concept.history"), TopicID: mustID(t, "topic.history"), Title: "History"}
	if err := database.SeedCurriculum(ctx, reference, []learning.Concept{concept}, nil); err != nil {
		t.Fatal(err)
	}
	instance, err := learning.NewCurriculumInstance(mustID(t, "instance.history"), student.ID, goal.ID, reference,
		learning.CurriculumSourceFixture, mustTimestamp(t, fixedTime))
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.CurriculumInstances.Create(ctx, instance); err != nil {
		t.Fatal(err)
	}
	evidence, err := learning.NewEvidence(mustID(t, "evidence.history"), student.ID, concept.ID, learning.EvidenceKnowledgeCheck,
		"fixture/history", mustScore(t, .8), mustTimestamp(t, fixedTime.Add(time.Minute)))
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	session, err := learning.NewStudySession(mustID(t, "session.history"), student.ID, goal.ID, instance.ID,
		mustTimestamp(t, fixedTime.Add(2*time.Minute)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	session, _ = session.RecordActivity(mustTimestamp(t, fixedTime.Add(7*time.Minute)))
	session, _ = session.Complete(mustTimestamp(t, fixedTime.Add(8*time.Minute)))
	if err := repositories.StudySessions.Create(ctx, session); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("migrate study history: %v", err)
	}
	events, err := database.LearningRepositories().History.ListByStudent(ctx, student.ID, nil, nil)
	if err != nil || len(events) != 2 {
		t.Fatalf("backfilled history = (%+v, %v)", events, err)
	}
	if events[0].Type != learning.StudyEventSessionCompleted || events[1].Type != learning.StudyEventEvidenceRecorded {
		t.Fatalf("backfilled types = %+v", events)
	}
	for _, event := range events {
		if event.OccurredAt.Time().Location() != time.UTC || event.Version != learning.StudyHistoryVersion {
			t.Fatalf("backfilled event = %+v", event)
		}
	}
}

func TestRetentionV1MigrationPreservesLegacySnapshotAndSupportsV1RoundTrip(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:15]); err != nil {
		t.Fatalf("migrate through v15: %v", err)
	}
	ctx := context.Background()
	repositories := database.LearningRepositories()
	student := testStudent(t)
	if err := repositories.Students.Create(ctx, student); err != nil {
		t.Fatal(err)
	}
	reference := learning.CurriculumRef{ID: mustID(t, "curriculum.retention"), Version: "1.0.0"}
	concept := learning.Concept{ID: mustID(t, "concept.retention"), TopicID: mustID(t, "topic.retention"), Title: "Retention"}
	if err := database.SeedCurriculum(ctx, reference, []learning.Concept{concept}, nil); err != nil {
		t.Fatal(err)
	}
	legacyMeasured := mustTimestamp(t, fixedTime)
	if _, err := handle.Exec(`INSERT INTO retention_state (student_id,concept_id,strength,measured_at) VALUES (?,?,?,?)`,
		student.ID.String(), concept.ID.String(), .6, encodeTimestamp(legacyMeasured)); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("migrate retention-v1: %v", err)
	}
	legacy, err := database.LearningRepositories().Retention.Get(ctx, student.ID, concept.ID)
	if err != nil || legacy.AlgorithmVersion != learning.LegacyRetentionAlgorithmVersion ||
		legacy.Status != learning.RetentionUnknown || legacy.Strength.Value() != .6 {
		t.Fatalf("legacy retention = (%+v, %v)", legacy, err)
	}

	lastPractice := mustTimestamp(t, fixedTime.Add(time.Hour))
	nextDue := mustTimestamp(t, fixedTime.Add(time.Hour+7*24*time.Hour))
	v1 := learning.RetentionState{
		StudentID: student.ID, ConceptID: concept.ID, LastSuccessfulRecall: &lastPractice, LastPractice: &lastPractice,
		ReviewCount: 2, SuccessfulReviews: 1, FailedReviews: 1, StabilityEstimate: 7 * 24 * time.Hour,
		Strength: mustScore(t, .72), Status: learning.RetentionStable, NextDueAt: &nextDue,
		MeasuredAt: mustTimestamp(t, fixedTime.Add(2*time.Hour)), AlgorithmVersion: learning.RetentionAlgorithmVersion,
	}
	if err := database.LearningRepositories().Retention.Save(ctx, v1); err != nil {
		t.Fatal(err)
	}
	loaded, err := database.LearningRepositories().Retention.Get(ctx, student.ID, concept.ID)
	if err != nil || !reflect.DeepEqual(loaded, v1) {
		t.Fatalf("retention-v1 roundtrip = (%+v, %v), want %+v", loaded, err, v1)
	}
	if _, err := handle.Exec(`UPDATE retention_state SET review_count=3 WHERE student_id=? AND concept_id=?`, student.ID.String(), concept.ID.String()); err == nil {
		t.Fatal("retention aggregate trigger accepted inconsistent review counts")
	}
}

func TestReviewSchedulerV1MigrationDeduplicatesPendingAndSupportsLifecycleRoundTrip(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:16]); err != nil {
		t.Fatalf("migrate through v16: %v", err)
	}
	ctx := context.Background()
	repositories := database.LearningRepositories()
	student := testStudent(t)
	if err := repositories.Students.Create(ctx, student); err != nil {
		t.Fatal(err)
	}
	reference := learning.CurriculumRef{ID: mustID(t, "curriculum.scheduler"), Version: "1.0.0"}
	concept := learning.Concept{ID: mustID(t, "concept.scheduler"), TopicID: mustID(t, "topic.scheduler"), Title: "Scheduler"}
	if err := database.SeedCurriculum(ctx, reference, []learning.Concept{concept}, nil); err != nil {
		t.Fatal(err)
	}
	introduced := mustTimestamp(t, fixedTime)
	firstDue := mustTimestamp(t, fixedTime.Add(24*time.Hour))
	secondDue := mustTimestamp(t, fixedTime.Add(48*time.Hour))
	if _, err := handle.Exec(`INSERT INTO review_schedule (student_id,concept_id,introduced_at,due_at,imported) VALUES (?,?,?,?,0)`,
		student.ID.String(), concept.ID.String(), encodeTimestamp(introduced), encodeTimestamp(firstDue)); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id  string
		due learning.Timestamp
	}{{"review.legacy.first", firstDue}, {"review.legacy.second", secondDue}} {
		if _, err := handle.Exec(`INSERT INTO review_items (id,student_id,concept_id,due_at,status,completed_at) VALUES (?,?,?,?, 'pending',NULL)`,
			row.id, student.ID.String(), concept.ID.String(), encodeTimestamp(row.due)); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("migrate review scheduler: %v", err)
	}
	legacySchedule, err := database.LearningRepositories().Reviews.GetSchedule(ctx, student.ID, concept.ID)
	if err != nil || legacySchedule.AlgorithmVersion != learning.LegacyReviewSchedulerVersion || legacySchedule.UpdatedAt != firstDue {
		t.Fatalf("legacy schedule = (%+v, %v)", legacySchedule, err)
	}
	legacyItems, err := database.LearningRepositories().Reviews.ListByStudent(ctx, student.ID)
	if err != nil || len(legacyItems) != 2 || legacyItems[0].Status != learning.ReviewPending || legacyItems[1].Status != learning.ReviewSkipped {
		t.Fatalf("deduplicated legacy items = (%+v, %v)", legacyItems, err)
	}
	keeper := legacyItems[0]
	keeper.Status = learning.ReviewSkipped
	if err := database.LearningRepositories().Reviews.UpdateItem(ctx, keeper); err != nil {
		t.Fatal(err)
	}
	now := mustTimestamp(t, fixedTime.Add(3*24*time.Hour))
	dueAt := mustTimestamp(t, fixedTime.Add(4*24*time.Hour))
	schedule, err := learning.NewReviewScheduleV1(student.ID, concept.ID, introduced, dueAt, learning.ReviewDeep, true, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.LearningRepositories().Reviews.SaveSchedule(ctx, schedule); err != nil {
		t.Fatal(err)
	}
	id, _ := learning.NewReviewItemIDV1(student.ID, concept.ID, dueAt, len(legacyItems))
	item, _ := learning.NewReviewItemV1(id, schedule, now)
	postponedDue := mustTimestamp(t, fixedTime.Add(5*24*time.Hour))
	item, _ = item.Postpone(postponedDue, now)
	if err := database.LearningRepositories().Reviews.CreateItem(ctx, item); err != nil {
		t.Fatal(err)
	}
	loaded, err := database.LearningRepositories().Reviews.GetItem(ctx, item.ID)
	if err != nil || !reflect.DeepEqual(loaded, item) {
		t.Fatalf("review item roundtrip = (%+v, %v), want %+v", loaded, err, item)
	}
	pending, err := database.LearningRepositories().Reviews.PendingByConcept(ctx, student.ID, concept.ID)
	if err != nil || pending.ID != item.ID {
		t.Fatalf("pending review = (%+v, %v)", pending, err)
	}
	duplicateID, _ := learning.NewReviewItemIDV1(student.ID, concept.ID, postponedDue, len(legacyItems)+1)
	duplicate, _ := learning.NewReviewItemV1(duplicateID, schedule, now)
	if err := database.LearningRepositories().Reviews.CreateItem(ctx, duplicate); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate pending error = %v, want conflict", err)
	}
	if _, err := handle.Exec(`UPDATE review_items SET status='completed',outcome='success',score=0.1,completed_at=? WHERE id=?`,
		encodeTimestamp(now), item.ID.String()); err == nil {
		t.Fatal("review trigger accepted success with failing score")
	}
}

func TestStreakV1MigrationPreservesLegacyStateAndSupportsRoundTrip(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:17]); err != nil {
		t.Fatalf("migrate through v17: %v", err)
	}
	ctx := context.Background()
	student := testStudent(t)
	if err := database.LearningRepositories().Students.Create(ctx, student); err != nil {
		t.Fatal(err)
	}
	lastStudy := mustTimestamp(t, fixedTime)
	if _, err := handle.Exec(`INSERT INTO streak_state (student_id,current_days,longest_days,last_study_at) VALUES (?,?,?,?)`,
		student.ID.String(), 2, 3, encodeTimestamp(lastStudy)); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("migrate streak-v1: %v", err)
	}
	legacy, err := database.LearningRepositories().Streaks.Get(ctx, student.ID)
	if err != nil || legacy.PolicyVersion != learning.LegacyStreakPolicyVersion || legacy.CurrentDays != 2 ||
		legacy.LongestDays != 3 || legacy.LastActiveLocalDate != nil || legacy.TotalActiveDays != 0 {
		t.Fatalf("legacy streak = (%+v, %v)", legacy, err)
	}

	lastDate, err := learning.NewLocalDate(lastStudy.Time().Format("2006-01-02"))
	if err != nil {
		t.Fatal(err)
	}
	v1 := learning.Streak{
		StudentID: student.ID, CurrentDays: 2, LongestDays: 3, LastActiveLocalDate: &lastDate,
		TotalActiveDays: 5, LastStudyAt: &lastStudy, Timezone: "UTC", MinimumActiveMinutes: 10,
		PolicyVersion: learning.StreakPolicyVersion,
	}
	if err := database.LearningRepositories().Streaks.Save(ctx, v1); err != nil {
		t.Fatal(err)
	}
	loaded, err := database.LearningRepositories().Streaks.Get(ctx, student.ID)
	if err != nil || !reflect.DeepEqual(loaded, v1) {
		t.Fatalf("streak-v1 roundtrip = (%+v, %v), want %+v", loaded, err, v1)
	}
	if _, err := handle.Exec(`UPDATE streak_state SET total_active_days=1 WHERE student_id=?`, student.ID.String()); err == nil {
		t.Fatal("streak trigger accepted longest above total active days")
	}
}

func TestAchievementV1MigrationPreservesLegacyRowsAndEnforcesUniqueUnlocks(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:18]); err != nil {
		t.Fatalf("migrate through v18: %v", err)
	}
	ctx := context.Background()
	student := testStudent(t)
	if err := database.LearningRepositories().Students.Create(ctx, student); err != nil {
		t.Fatal(err)
	}
	unlockedAt := mustTimestamp(t, fixedTime)
	if _, err := handle.Exec(`INSERT INTO achievement_definitions (key,name) VALUES ('legacy.first','Legacy first')`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO student_achievements (id,student_id,achievement_key,status,unlocked_at)
VALUES ('achievement.legacy',?,'legacy.first','unlocked',?)`, student.ID.String(), encodeTimestamp(unlockedAt)); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("migrate achievement-v1: %v", err)
	}
	legacy, err := database.LearningRepositories().Achievements.Get(ctx, mustID(t, "achievement.legacy"))
	if err != nil || legacy.Name != "Legacy first" || legacy.DefinitionVersion != "" || legacy.PolicyVersion != "" {
		t.Fatalf("legacy achievement = (%+v, %v)", legacy, err)
	}

	definition := learning.FoundationAchievementDefinitions()[0]
	if err := database.LearningRepositories().Achievements.SaveDefinition(ctx, definition); err != nil {
		t.Fatal(err)
	}
	achievement := learning.Achievement{
		ID: mustID(t, "achievement.student-1.first_session"), StudentID: student.ID,
		Key: definition.ID, Name: definition.Title, Description: definition.Description,
		Criteria: definition.Criteria, Config: definition.Config, Hidden: definition.Hidden,
		DefinitionVersion: definition.Version, Status: learning.AchievementUnlocked, UnlockedAt: &unlockedAt,
		Context: map[string]string{"session_id": "session.first"}, PolicyVersion: learning.AchievementPolicyVersion,
	}
	created, err := database.LearningRepositories().Achievements.Unlock(ctx, achievement)
	if err != nil || !created {
		t.Fatalf("first Unlock() = (%v, %v)", created, err)
	}
	created, err = database.LearningRepositories().Achievements.Unlock(ctx, achievement)
	if err != nil || created {
		t.Fatalf("duplicate Unlock() = (%v, %v)", created, err)
	}
	loaded, err := database.LearningRepositories().Achievements.Get(ctx, achievement.ID)
	if err != nil || !reflect.DeepEqual(loaded, achievement) {
		t.Fatalf("achievement-v1 roundtrip = (%+v, %v), want %+v", loaded, err, achievement)
	}
	definitions, err := database.LearningRepositories().Achievements.ListDefinitions(ctx)
	if err != nil || len(definitions) != 1 || !reflect.DeepEqual(definitions[0], definition) {
		t.Fatalf("definitions = (%+v, %v)", definitions, err)
	}
	if _, err := handle.Exec(`UPDATE achievement_definitions SET criteria_type='active_days' WHERE key=?`, definition.ID.String()); err == nil {
		t.Fatal("achievement trigger accepted active-days criterion without a count")
	}
}

func TestDailyPlanV1MigrationPreservesLegacyRowsAndSupportsExplainableSnapshots(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:19]); err != nil {
		t.Fatalf("migrate through v19: %v", err)
	}
	ctx := context.Background()
	student := testStudent(t)
	goal := testGoal(t, student.ID)
	if err := database.LearningRepositories().Students.Create(ctx, student); err != nil {
		t.Fatal(err)
	}
	if err := database.LearningRepositories().Goals.Create(ctx, goal); err != nil {
		t.Fatal(err)
	}
	reference := learning.CurriculumRef{ID: mustID(t, "curriculum.daily-migration"), Version: "1.0.0"}
	concept := learning.Concept{ID: mustID(t, "concept.daily-migration"), TopicID: mustID(t, "topic.daily-migration"), Title: "Daily planning"}
	if err := database.SeedCurriculum(ctx, reference, []learning.Concept{concept}, nil); err != nil {
		t.Fatal(err)
	}
	instance, err := learning.NewCurriculumInstance(mustID(t, "instance.daily-migration"), student.ID, goal.ID,
		reference, learning.CurriculumSourceFixture, mustTimestamp(t, fixedTime))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.LearningRepositories().CurriculumInstances.Create(ctx, instance); err != nil {
		t.Fatal(err)
	}
	legacyDate := mustTimestamp(t, fixedTime.Add(24*time.Hour))
	if _, err := handle.Exec(`INSERT INTO daily_plans (id,student_id,goal_id,plan_date,created_at) VALUES ('plan.legacy',?,?,?,?)`,
		student.ID.String(), goal.ID.String(), encodeTimestamp(legacyDate), encodeTimestamp(legacyDate)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO daily_plan_items (id,plan_id,item_type,estimated_minutes,position) VALUES ('plan-item.legacy','plan.legacy','review',10,0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO daily_plan_item_concepts (item_id,concept_id,position) VALUES ('plan-item.legacy',?,0)`, concept.ID.String()); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("migrate daily-plan-v1: %v", err)
	}
	legacy, err := database.LearningRepositories().DailyPlans.ForDate(ctx, student.ID, goal.ID, legacyDate)
	if err != nil || legacy.PolicyVersion != "" || legacy.Items[0].Role != "" || legacy.Items[0].Explanation != "" {
		t.Fatalf("legacy daily plan = (%+v, %v)", legacy, err)
	}

	planDate := mustTimestamp(t, time.Date(2026, 8, 22, 5, 0, 0, 0, time.UTC))
	createdAt := mustTimestamp(t, time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC))
	plan := learning.DailyPlan{
		ID: mustID(t, "plan.daily-v1"), StudentID: student.ID, GoalID: goal.ID, CurriculumInstanceID: instance.ID,
		Date: planDate, CreatedAt: createdAt, Timezone: "America/Lima", AvailableMinutes: 45,
		PlannedMinutes: 25, BufferMinutes: 5, Status: learning.DailyPlanReady,
		GenerationReason: learning.DailyPlanGeneratedInitial, SourceFingerprint: "sha256:" + strings.Repeat("0", 64),
		PolicyVersion: learning.DailyPlanPolicyVersion,
		Items: []learning.DailyPlanItem{{
			ID: mustID(t, "plan-item.daily-v1"), Type: learning.DailyPlanLearn, Role: learning.DailyPlanRoleNewLearning,
			Reason: learning.DailyPlanNextEligibleConcept, Explanation: "Next eligible curriculum concept.",
			ConceptIDs: []learning.ID{concept.ID}, EstimatedMinutes: 25, Position: 0,
		}},
	}
	if err := database.LearningRepositories().DailyPlans.Save(ctx, plan); err != nil {
		t.Fatal(err)
	}
	loaded, err := database.LearningRepositories().DailyPlans.ForDate(ctx, student.ID, goal.ID, planDate)
	if err != nil || !reflect.DeepEqual(loaded, plan) {
		t.Fatalf("daily-plan-v1 roundtrip = (%+v, %v), want %+v", loaded, err, plan)
	}
	if _, err := handle.Exec(`UPDATE daily_plans SET planned_minutes=available_minutes+1 WHERE id=?`, plan.ID.String()); err == nil {
		t.Fatal("daily plan trigger accepted an over-budget snapshot")
	}
}

func TestStudentCoreV4ProfileMigratesToProfileSettings(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:4]); err != nil {
		t.Fatalf("migrate through v4: %v", err)
	}
	timestamp := fixedTime.Format(timestampFormat)
	if _, err := handle.Exec(`INSERT INTO students (id,created_at,updated_at) VALUES ('student.legacy',?,?)`, timestamp, timestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO student_profiles (student_id,display_name,experience,weekly_minutes) VALUES ('student.legacy','Legacy Learner','beginner',180)`); err != nil {
		t.Fatal(err)
	}
	for position, day := range []int{1, 3, 5} {
		if _, err := handle.Exec(`INSERT INTO student_preferred_days (student_id,weekday,position) VALUES ('student.legacy',?,?)`, day, position); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate profile settings: %v", err)
	}
	student, err := database.LearningRepositories().Students.Get(context.Background(), mustID(t, "student.legacy"))
	if err != nil {
		t.Fatalf("get migrated student: %v", err)
	}
	if student.Profile.DisplayName != "Legacy Learner" || student.Profile.PreferredLanguage != "en" ||
		student.Profile.Availability.DailyMinutes != 60 || student.Profile.Availability.WeeklyDaysTarget != 3 || student.Profile.Timezone != "UTC" {
		t.Fatalf("migrated profile = %+v", student.Profile)
	}
}

func TestLearningGoalMigrationPreservesHistoryAndResolvesActiveDuplicates(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:5]); err != nil {
		t.Fatalf("migrate through v5: %v", err)
	}
	timestamp := fixedTime.Format(timestampFormat)
	later := fixedTime.Add(time.Minute).Format(timestampFormat)
	if _, err := handle.Exec(`INSERT INTO students (id,created_at,updated_at) VALUES ('student.legacy',?,?)`, timestamp, later); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO student_profiles (student_id,display_name,experience,weekly_minutes) VALUES ('student.legacy','Legacy Learner','beginner',180)`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO learning_goals (id,student_id,title,status,mastery_threshold,created_at,updated_at) VALUES
('goal.old','student.legacy','Old goal','active',0.8,?,?),
('goal.new','student.legacy','New goal','active',0.8,?,?)`, timestamp, timestamp, timestamp, later); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate goal lifecycle: %v", err)
	}
	goals, err := database.LearningRepositories().Goals.ListByStudent(context.Background(), mustID(t, "student.legacy"))
	if err != nil || len(goals) != 2 {
		t.Fatalf("migrated goals = (%+v, %v)", goals, err)
	}
	byID := map[string]learning.LearningGoal{goals[0].ID.String(): goals[0], goals[1].ID.String(): goals[1]}
	if byID["goal.old"].Status != learning.GoalPaused || byID["goal.new"].Status != learning.GoalActive ||
		byID["goal.new"].TargetOutcome != "New goal" || byID["goal.new"].ActivatedAt == nil {
		t.Fatalf("migrated goal lifecycle = %+v", goals)
	}
}

func TestMasteryPolicyMigrationCarriesForwardActiveGoalThreshold(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:7]); err != nil {
		t.Fatalf("migrate through v7: %v", err)
	}
	timestamp := fixedTime.Format(timestampFormat)
	if _, err := handle.Exec(`INSERT INTO students (id,created_at,updated_at) VALUES ('student.onboarded',?,?)`, timestamp, timestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO student_profiles
(student_id,display_name,experience,weekly_minutes) VALUES ('student.onboarded','Ada','beginner',150)`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO learning_goals
(id,student_id,title,status,mastery_threshold,created_at,updated_at) VALUES ('goal.onboarded','student.onboarded','Calculus','active',0.85,?,?)`, timestamp, timestamp); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate mastery policy: %v", err)
	}
	settings, err := database.LearningRepositories().Mastery.Get(context.Background(), mustID(t, "student.onboarded"))
	if err != nil || settings.StudentDefault.Value() != .85 || settings.WorkspaceOverride != nil {
		t.Fatalf("migrated mastery settings = (%+v, %v)", settings, err)
	}
}

func TestCurriculumInstanceMigrationPreservesLegacyConceptStateWithoutInferringInstances(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:8]); err != nil {
		t.Fatalf("migrate through v8: %v", err)
	}
	ctx := context.Background()
	student := testStudent(t)
	if err := database.LearningRepositories().Students.Create(ctx, student); err != nil {
		t.Fatal(err)
	}
	reference := learning.CurriculumRef{ID: mustID(t, "fixture.legacy"), Version: "1.0.0"}
	concept := learning.Concept{ID: mustID(t, "concept.legacy"), TopicID: mustID(t, "topic.legacy"), Title: "Legacy concept"}
	if err := database.SeedCurriculum(ctx, reference, []learning.Concept{concept}, nil); err != nil {
		t.Fatal(err)
	}
	introduced := mustTimestamp(t, fixedTime)
	legacy := learning.ConceptState{
		StudentID: student.ID, ConceptID: concept.ID, Exposure: learning.ExposureLearning,
		Mastery: mustScore(t, .61), IntroducedAt: &introduced, UpdatedAt: introduced,
	}
	if err := database.LearningRepositories().Concepts.Save(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("migrate learner curriculum instances: %v", err)
	}
	loaded, err := database.LearningRepositories().Concepts.Get(ctx, student.ID, concept.ID)
	if err != nil || loaded.Exposure != learning.ExposureLearning || loaded.Mastery.Value() != .61 {
		t.Fatalf("legacy state after migration = (%+v, %v)", loaded, err)
	}
	for _, table := range []string{"learner_curriculum_instances", "learner_curriculum_concept_states"} {
		var count int
		if err := handle.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s count = %d, want no inferred learner data", table, count)
		}
	}
}

func TestDiagnosticMigrationFromV9IsAdditive(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, _ := platform.WorkspaceDBPath(root)
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:9]); err != nil {
		t.Fatalf("migrate through v9: %v", err)
	}
	if _, err := handle.Exec(`INSERT INTO app_state (namespace,key,value,updated_at) VALUES ('step10','kept',X'6F6B',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate diagnostics: %v", err)
	}
	var value []byte
	if err := handle.QueryRow(`SELECT value FROM app_state WHERE namespace='step10' AND key='kept'`).Scan(&value); err != nil || string(value) != "ok" {
		t.Fatalf("preserved value = (%q, %v)", value, err)
	}
	for _, table := range []string{"diagnostic_attempts", "diagnostic_observations"} {
		var count int
		if err := handle.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("table %s count=%d err=%v", table, count, err)
		}
	}
}

func TestStudentCoreSchemaHasRequiredIndexesAndConstraints(t *testing.T) {
	database, _ := openTestDatabase(t)
	ctx := context.Background()
	repositories := database.LearningRepositories()
	want := []string{"curriculum_nodes_concept_idx", "diagnostic_attempts_student_status_idx", "diagnostic_observations_concept_idx", "learning_goals_active_idx", "learning_goals_one_active_idx", "learning_evidence_concept_idx", "review_items_due_idx", "study_session_lifecycle_goal_timeline_idx", "study_session_lifecycle_one_active_idx", "study_sessions_goal_timeline_idx", "study_sessions_range_idx"}
	for _, name := range want {
		var count int
		if err := database.sql.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", name).Scan(&count); err != nil || count != 1 {
			t.Fatalf("index %s count=%d err=%v", name, count, err)
		}
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO learning_goals (id,student_id,title,status,mastery_threshold,created_at,updated_at) VALUES ('orphan','missing','x','draft',0.8,?,?)`, fixedTime.Format(timestampFormat), fixedTime.Format(timestampFormat)); err == nil {
		t.Fatal("foreign key accepted orphan goal")
	}
	orphanGoal := testGoal(t, mustID(t, "missing-student"))
	if err := repositories.Goals.Create(ctx, orphanGoal); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("orphan goal error = %v, want invalid_state", err)
	}
	student := testStudent(t)
	if err := repositories.Students.Create(ctx, student); err != nil {
		t.Fatal(err)
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO learning_goals (id,student_id,title,status,mastery_threshold,created_at,updated_at) VALUES ('invalid',?,'x','draft',1.1,?,?)`, student.ID.String(), fixedTime.Format(timestampFormat), fixedTime.Format(timestampFormat)); err == nil {
		t.Fatal("mastery constraint accepted 1.1")
	}
	goal := testGoal(t, student.ID)
	if err := repositories.Goals.Create(ctx, goal); err != nil {
		t.Fatal(err)
	}
	firstActive := goal
	firstActive.ID = mustID(t, "goal-active-1")
	firstActive, activateErr := firstActive.Activate(mustTimestamp(t, fixedTime.Add(time.Minute)))
	if activateErr != nil {
		t.Fatal(activateErr)
	}
	if err := repositories.Goals.Create(ctx, firstActive); err != nil {
		t.Fatal(err)
	}
	secondActive := goal
	secondActive.ID = mustID(t, "goal-active-2")
	secondActive, activateErr = secondActive.Activate(mustTimestamp(t, fixedTime.Add(time.Minute)))
	if activateErr != nil {
		t.Fatal(activateErr)
	}
	if err := repositories.Goals.Create(ctx, secondActive); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("second active goal error = %v, want conflict", err)
	}
	if _, err := database.sql.ExecContext(ctx, "DELETE FROM students WHERE id = ?", student.ID.String()); err != nil {
		t.Fatal(err)
	}
	var goals int
	if err := database.sql.QueryRowContext(ctx, "SELECT COUNT(*) FROM learning_goals WHERE student_id = ?", student.ID.String()).Scan(&goals); err != nil {
		t.Fatal(err)
	}
	if goals != 0 {
		t.Fatalf("student delete left %d goals, want cascade", goals)
	}
}

func TestSQLiteLearningRepositoryRoundTrips(t *testing.T) {
	database, _ := openTestDatabase(t)
	ctx := context.Background()
	repositories := database.LearningRepositories()
	student := testStudent(t)
	if err := repositories.Students.Create(ctx, student); err != nil {
		t.Fatalf("Students.Create: %v", err)
	}
	if err := repositories.Students.Create(ctx, student); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate student=%v", err)
	}
	gotStudent, err := repositories.Students.Get(ctx, student.ID)
	if err != nil || !reflect.DeepEqual(gotStudent, student) {
		t.Fatalf("student=(%+v,%v), want %+v", gotStudent, err, student)
	}
	student.Profile.DisplayName = "Ada Updated"
	student.UpdatedAt = mustTimestamp(t, fixedTime.Add(time.Minute))
	if err := repositories.Students.Update(ctx, student); err != nil {
		t.Fatal(err)
	}
	gotStudent, _ = repositories.Students.Get(ctx, student.ID)
	if !reflect.DeepEqual(gotStudent, student) {
		t.Fatalf("updated student=%+v", gotStudent)
	}
	student.Profile.DisplayName = ""
	student.UpdatedAt = mustTimestamp(t, fixedTime.Add(2*time.Minute))
	if err := repositories.Students.Update(ctx, student); err != nil {
		t.Fatal(err)
	}
	gotStudent, _ = repositories.Students.Get(ctx, student.ID)
	if !reflect.DeepEqual(gotStudent, student) {
		t.Fatalf("student with empty display name=%+v", gotStudent)
	}

	goal := testGoal(t, student.ID)
	if err := repositories.Goals.Create(ctx, goal); err != nil {
		t.Fatalf("Goals.Create: %v", err)
	}
	gotGoal, err := repositories.Goals.Get(ctx, goal.ID)
	if err != nil || gotGoal != goal {
		t.Fatalf("goal=(%+v,%v)", gotGoal, err)
	}
	goals, err := repositories.Goals.ListByStudent(ctx, student.ID)
	if err != nil || len(goals) != 1 || goals[0] != goal {
		t.Fatalf("goals=(%+v,%v)", goals, err)
	}

	reference := learning.CurriculumRef{ID: mustID(t, "curriculum-core"), Version: "fixture/v1"}
	conceptA := learning.Concept{ID: mustID(t, "concept-a"), TopicID: mustID(t, "topic-a"), Title: "Concept A"}
	conceptB := learning.Concept{ID: mustID(t, "concept-b"), TopicID: mustID(t, "topic-a"), Title: "Concept B"}
	edge, _ := learning.NewPrerequisite(conceptB.ID, conceptA.ID)
	if err := database.SeedCurriculum(ctx, reference, []learning.Concept{conceptB, conceptA}, []learning.Prerequisite{edge}); err != nil {
		t.Fatalf("SeedCurriculum: %v", err)
	}
	curriculumInstance, err := learning.NewCurriculumInstance(mustID(t, "instance-session"), student.ID, goal.ID, reference, learning.CurriculumSourceFixture, mustTimestamp(t, fixedTime))
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.CurriculumInstances.Create(ctx, curriculumInstance); err != nil {
		t.Fatalf("CurriculumInstances.Create: %v", err)
	}
	concepts, err := repositories.Curricula.Concepts(ctx, reference)
	if err != nil || !reflect.DeepEqual(concepts, []learning.Concept{conceptA, conceptB}) {
		t.Fatalf("concepts=(%+v,%v)", concepts, err)
	}
	planningConcepts, err := repositories.Curricula.PlanningConcepts(ctx, reference)
	if err != nil || !reflect.DeepEqual(planningConcepts, []learning.DailyPlanCurriculumConcept{
		{ConceptID: conceptA.ID, Sequence: 0},
		{ConceptID: conceptB.ID, Sequence: 1, PrerequisiteIDs: []learning.ID{conceptA.ID}},
	}) {
		t.Fatalf("planning concepts=(%+v,%v)", planningConcepts, err)
	}
	prerequisites, err := repositories.Curricula.Prerequisites(ctx, reference, conceptB.ID)
	if err != nil || !reflect.DeepEqual(prerequisites, []learning.Prerequisite{edge}) {
		t.Fatalf("prerequisites=(%+v,%v)", prerequisites, err)
	}

	introduced := mustTimestamp(t, fixedTime.Add(2*time.Minute))
	state := learning.ConceptState{StudentID: student.ID, ConceptID: conceptA.ID, Exposure: learning.ExposureLearning, Mastery: mustScore(t, .4), IntroducedAt: &introduced, UpdatedAt: introduced}
	if err := repositories.Concepts.Save(ctx, state); err != nil {
		t.Fatal(err)
	}
	gotState, err := repositories.Concepts.Get(ctx, student.ID, conceptA.ID)
	if err != nil || !reflect.DeepEqual(gotState, state) {
		t.Fatalf("state=(%+v,%v)", gotState, err)
	}
	evidence, _ := learning.NewEvidenceWithMetadata(mustID(t, "evidence-1"), student.ID, conceptA.ID, learning.EvidenceKnowledgeCheck,
		"fixture/knowledge-check", mustScore(t, .7), learning.EvidenceMetadata{
			Confidence: .8, Independence: .6, Difficulty: .9, AlgorithmVersion: "fixture-evaluator/v1",
		}, introduced)
	if err := repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	evidenceItems, err := repositories.Evidence.ListByConcept(ctx, student.ID, conceptA.ID)
	if err != nil || !reflect.DeepEqual(evidenceItems, []learning.Evidence{evidence}) {
		t.Fatalf("evidence=(%+v,%v)", evidenceItems, err)
	}
	for name, statement := range map[string]string{
		"confidence":   `UPDATE learning_evidence SET confidence=0 WHERE id='evidence-1'`,
		"independence": `UPDATE learning_evidence SET independence=1.1 WHERE id='evidence-1'`,
		"difficulty":   `UPDATE learning_evidence SET difficulty=-0.1 WHERE id='evidence-1'`,
		"algorithm":    `UPDATE learning_evidence SET algorithm_version=' ' WHERE id='evidence-1'`,
	} {
		if _, err := database.sql.ExecContext(ctx, statement); err == nil {
			t.Fatalf("evidence %s constraint accepted malformed metadata", name)
		}
	}
	mistake, _ := learning.NewMistake(mustID(t, "mistake-1"), student.ID, conceptA.ID, learning.MistakeKey("mixed-rules"),
		learning.MistakeProcedure, "mixed two rules", introduced, "fixture/evaluator/1")
	if err := repositories.Mistakes.Create(ctx, mistake); err != nil {
		t.Fatal(err)
	}
	event, _ := learning.NewMistakeEvent(mustID(t, "mistake-event-1"), mistake.ID, learning.MistakeObservedEvent, introduced, "fixture/evaluator/1")
	if err := repositories.Mistakes.AppendEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	duplicate, _ := learning.NewMistake(mustID(t, "mistake-duplicate"), student.ID, conceptA.ID, mistake.Key,
		mistake.Category, mistake.Summary, introduced, "fixture/evaluator/duplicate")
	if err := repositories.Mistakes.Create(ctx, duplicate); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate mistake key error = %v, want conflict", err)
	}
	resolved := mustTimestamp(t, fixedTime.Add(3*time.Minute))
	mistake, _ = mistake.Resolve(resolved)
	if err := repositories.Mistakes.Update(ctx, mistake); err != nil {
		t.Fatal(err)
	}
	mistakes, err := repositories.Mistakes.ListByConcept(ctx, student.ID, conceptA.ID)
	if err != nil || !reflect.DeepEqual(mistakes, []learning.Mistake{mistake}) {
		t.Fatalf("mistakes=(%+v,%v)", mistakes, err)
	}
	history, err := repositories.Mistakes.ListEvents(ctx, mistake.ID)
	if err != nil || !reflect.DeepEqual(history, []learning.MistakeEvent{event}) {
		t.Fatalf("mistake history=(%+v,%v)", history, err)
	}
	retention := learning.RetentionState{StudentID: student.ID, ConceptID: conceptA.ID, Strength: mustScore(t, .6),
		Status: learning.RetentionUnknown, MeasuredAt: resolved, AlgorithmVersion: learning.LegacyRetentionAlgorithmVersion}
	if err := repositories.Retention.Save(ctx, retention); err != nil {
		t.Fatal(err)
	}
	gotRetention, err := repositories.Retention.Get(ctx, student.ID, conceptA.ID)
	if err != nil || gotRetention != retention {
		t.Fatalf("retention=(%+v,%v)", gotRetention, err)
	}

	activity := learning.StudyActivity{ID: mustID(t, "activity-1"), ConceptIDs: []learning.ID{conceptA.ID, conceptB.ID}, Type: learning.ActivityPractice, StartedAt: introduced, EndedAt: resolved}
	session, _ := learning.NewLearningSession(mustID(t, "session-1"), student.ID, goal.ID, introduced, mustTimestamp(t, fixedTime.Add(4*time.Minute)), []learning.StudyActivity{activity})
	if err := repositories.Sessions.Append(ctx, session); err != nil {
		t.Fatal(err)
	}
	gotSession, err := repositories.Sessions.Get(ctx, session.ID)
	if err != nil || !reflect.DeepEqual(gotSession, session) {
		t.Fatalf("session=(%+v,%v), want %+v", gotSession, err, session)
	}
	studySession, err := learning.NewStudySession(mustID(t, "study-session-1"), student.ID, goal.ID, curriculumInstance.ID, introduced, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.StudySessions.Create(ctx, studySession); err != nil {
		t.Fatal(err)
	}
	duplicateActive, _ := learning.NewStudySession(mustID(t, "study-session-duplicate"), student.ID, goal.ID, curriculumInstance.ID, introduced, 15*time.Minute)
	if err := repositories.StudySessions.Create(ctx, duplicateActive); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate active study session error = %v, want conflict", err)
	}
	studySession, _ = studySession.RecordActivity(resolved)
	studySession, _ = studySession.Complete(mustTimestamp(t, fixedTime.Add(4*time.Minute)))
	if err := repositories.StudySessions.Update(ctx, studySession); err != nil {
		t.Fatal(err)
	}
	gotStudySession, err := repositories.StudySessions.Get(ctx, studySession.ID)
	if err != nil || !reflect.DeepEqual(gotStudySession, studySession) {
		t.Fatalf("study session=(%+v,%v), want %+v", gotStudySession, err, studySession)
	}
	listedStudySessions, err := repositories.StudySessions.ListByGoal(ctx, student.ID, goal.ID)
	if err != nil || !reflect.DeepEqual(listedStudySessions, []learning.StudySession{studySession}) {
		t.Fatalf("study sessions=(%+v,%v)", listedStudySessions, err)
	}
	historyEvent, err := learning.NewStudyEvent(mustID(t, "history-1"), student.ID, learning.StudyEventEvidenceRecorded,
		evidence.ID, evidence.ObservedAt, &goal.ID, &curriculumInstance.ID, &conceptA.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.History.Record(ctx, historyEvent); err != nil {
		t.Fatal(err)
	}
	if err := repositories.History.Record(ctx, historyEvent); err != nil {
		t.Fatalf("idempotent history record: %v", err)
	}
	conflicting := historyEvent
	conflicting.ID = mustID(t, "history-conflict")
	if err := repositories.History.Record(ctx, conflicting); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("conflicting history record = %v, want conflict", err)
	}
	gotHistory, err := repositories.History.Get(ctx, historyEvent.ID)
	if err != nil || !reflect.DeepEqual(gotHistory, historyEvent) {
		t.Fatalf("history=(%+v,%v), want %+v", gotHistory, err, historyEvent)
	}
	from, to := mustTimestamp(t, evidence.ObservedAt.Time().Add(-time.Second)), mustTimestamp(t, evidence.ObservedAt.Time().Add(time.Second))
	historyItems, err := repositories.History.ListByStudent(ctx, student.ID, &from, &to)
	if err != nil || !reflect.DeepEqual(historyItems, []learning.StudyEvent{historyEvent}) {
		t.Fatalf("filtered history=(%+v,%v)", historyItems, err)
	}
	schedule, _ := learning.NewReviewSchedule(student.ID, conceptA.ID, &introduced, mustTimestamp(t, fixedTime.Add(24*time.Hour)), false)
	if err := repositories.Reviews.SaveSchedule(ctx, schedule); err != nil {
		t.Fatal(err)
	}
	gotSchedule, err := repositories.Reviews.GetSchedule(ctx, student.ID, conceptA.ID)
	if err != nil || !reflect.DeepEqual(gotSchedule, schedule) {
		t.Fatalf("schedule=(%+v,%v)", gotSchedule, err)
	}
	review := learning.ReviewItem{ID: mustID(t, "review-1"), StudentID: student.ID, ConceptID: conceptA.ID,
		DueAt: schedule.DueAt, Type: learning.ReviewStandard, EstimatedMinutes: 10, Status: learning.ReviewPending,
		CreatedAt: schedule.DueAt, AlgorithmVersion: learning.LegacyReviewSchedulerVersion}
	if err := repositories.Reviews.CreateItem(ctx, review); err != nil {
		t.Fatal(err)
	}
	due, err := repositories.Reviews.ListDue(ctx, student.ID, schedule.DueAt)
	if err != nil || !reflect.DeepEqual(due, []learning.ReviewItem{review}) {
		t.Fatalf("due=(%+v,%v)", due, err)
	}

	lastStudy := session.EndedAt
	streak := learning.Streak{StudentID: student.ID, CurrentDays: 2, LongestDays: 3, LastStudyAt: &lastStudy,
		PolicyVersion: learning.LegacyStreakPolicyVersion}
	if err := repositories.Streaks.Save(ctx, streak); err != nil {
		t.Fatal(err)
	}
	gotStreak, err := repositories.Streaks.Get(ctx, student.ID)
	if err != nil || !reflect.DeepEqual(gotStreak, streak) {
		t.Fatalf("streak=(%+v,%v)", gotStreak, err)
	}
	unlocked := resolved
	achievement := learning.Achievement{ID: mustID(t, "achievement-1"), StudentID: student.ID, Key: mustID(t, "first-session"), Name: "First session", Status: learning.AchievementUnlocked, UnlockedAt: &unlocked}
	if err := repositories.Achievements.Save(ctx, achievement); err != nil {
		t.Fatal(err)
	}
	gotAchievement, err := repositories.Achievements.Get(ctx, achievement.ID)
	if err != nil || !reflect.DeepEqual(gotAchievement, achievement) {
		t.Fatalf("achievement=(%+v,%v)", gotAchievement, err)
	}
	milestone := learning.Milestone{ID: mustID(t, "milestone-1"), StudentID: student.ID, GoalID: goal.ID, Name: "Started", ReachedAt: introduced}
	if err := repositories.Achievements.AppendMilestone(ctx, milestone); err != nil {
		t.Fatal(err)
	}
	milestones, err := repositories.Achievements.ListMilestones(ctx, student.ID, goal.ID)
	if err != nil || !reflect.DeepEqual(milestones, []learning.Milestone{milestone}) {
		t.Fatalf("milestones=(%+v,%v)", milestones, err)
	}
	analytics := learning.AnalyticsSnapshot{StudentID: student.ID, CapturedAt: resolved, StudyMinutes: 20, SessionsCompleted: 1, ConceptsIntroduced: 2, ConceptsMastered: 1, ReviewsDue: 1}
	if err := repositories.Analytics.Append(ctx, analytics); err != nil {
		t.Fatal(err)
	}
	gotAnalytics, err := repositories.Analytics.Latest(ctx, student.ID)
	if err != nil || gotAnalytics != analytics {
		t.Fatalf("analytics=(%+v,%v)", gotAnalytics, err)
	}
	plan := learning.DailyPlan{ID: mustID(t, "plan-1"), StudentID: student.ID, GoalID: goal.ID, Date: mustTimestamp(t, fixedTime.Add(48*time.Hour)), CreatedAt: resolved, Items: []learning.DailyPlanItem{{ID: mustID(t, "plan-item-1"), Type: learning.DailyPlanReview, ConceptIDs: []learning.ID{conceptA.ID}, EstimatedMinutes: 10, Position: 0}}}
	if err := repositories.DailyPlans.Save(ctx, plan); err != nil {
		t.Fatal(err)
	}
	gotPlan, err := repositories.DailyPlans.ForDate(ctx, student.ID, goal.ID, plan.Date)
	if err != nil || !reflect.DeepEqual(gotPlan, plan) {
		t.Fatalf("plan=(%+v,%v), want %+v", gotPlan, err, plan)
	}
}

func TestSQLiteLearningUnitOfWorkRollsBack(t *testing.T) {
	database, _ := openTestDatabase(t)
	student := testStudent(t)
	wantErr := errors.New("stop")
	err := database.WithinTransaction(context.Background(), func(repositories application.Repositories) error {
		if err := repositories.Students.Create(context.Background(), student); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("transaction error=%v", err)
	}
	if _, err := database.LearningRepositories().Students.Get(context.Background(), student.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("rolled back student error=%v", err)
	}
}

func TestSQLiteLearningRepositoriesClassifyCancellation(t *testing.T) {
	database, _ := openTestDatabase(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := database.LearningRepositories().Students.Get(ctx, mustID(t, "student-1"))
	if !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("cancelled read error = %v, want unavailable", err)
	}
}

func TestSQLiteOnboardingRoundTripAndCorruptPayloadDetection(t *testing.T) {
	t.Parallel()
	database, _ := openTestDatabase(t)
	ctx := context.Background()
	student := testStudent(t)
	if err := database.LearningRepositories().Students.Create(ctx, student); err != nil {
		t.Fatal(err)
	}
	flow := application.DefaultOnboardingFlow()
	interview, err := learning.NewOnboardingInterview(student.ID, flow, mustTimestamp(t, fixedTime))
	if err != nil {
		t.Fatal(err)
	}
	interview, _ = interview.Start(flow, mustTimestamp(t, fixedTime.Add(time.Minute)))
	interview, _ = interview.Submit(flow, "Ada", mustTimestamp(t, fixedTime.Add(2*time.Minute)))
	if err := database.LearningRepositories().Onboarding.Save(ctx, interview); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := database.LearningRepositories().Onboarding.Get(ctx, student.ID)
	if err != nil || got.CurrentQuestionID != application.OnboardingGoalTitleQuestion || got.Answers[application.OnboardingDisplayNameQuestion] != "Ada" {
		t.Fatalf("Get() = (%+v, %v)", got, err)
	}
	if _, err := database.sql.ExecContext(ctx, "UPDATE onboarding_interviews SET answers_json = '{broken' WHERE student_id = ?", student.ID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := database.LearningRepositories().Onboarding.Get(ctx, student.ID); !errors.Is(err, application.ErrPersistenceFailure) {
		t.Fatalf("corrupt Get() error = %v, want persistence failure", err)
	}
}

func TestSQLiteDiagnosticRoundTripKeepsEvidenceLinked(t *testing.T) {
	t.Parallel()
	database, _ := openTestDatabase(t)
	ctx := context.Background()
	repositories := database.LearningRepositories()
	student := testStudent(t)
	if err := repositories.Students.Create(ctx, student); err != nil {
		t.Fatal(err)
	}
	goal := testGoal(t, student.ID)
	goal, err := goal.Activate(mustTimestamp(t, fixedTime.Add(time.Minute)))
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.Goals.Create(ctx, goal); err != nil {
		t.Fatal(err)
	}
	reference := learning.CurriculumRef{ID: mustID(t, "fixture.diagnostic"), Version: "1.0.0"}
	concept := learning.Concept{ID: mustID(t, "concept.diagnostic"), TopicID: mustID(t, "topic.diagnostic"), Title: "Diagnostic concept"}
	if err := database.SeedCurriculum(ctx, reference, []learning.Concept{concept}, nil); err != nil {
		t.Fatal(err)
	}
	instance, err := learning.NewCurriculumInstance(mustID(t, "instance.diagnostic"), student.ID, goal.ID, reference, learning.CurriculumSourceFixture, mustTimestamp(t, fixedTime.Add(2*time.Minute)))
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.CurriculumInstances.Create(ctx, instance); err != nil {
		t.Fatal(err)
	}
	diagnostic := sqliteDiagnosticFixture(t, reference, concept.ID)
	attempt, err := learning.NewDiagnosticAttempt(mustID(t, "attempt.diagnostic"), student.ID, instance.ID, diagnostic, mustTimestamp(t, fixedTime.Add(3*time.Minute)))
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.Diagnostics.Create(ctx, attempt); err != nil {
		t.Fatal(err)
	}
	answeredAt := mustTimestamp(t, fixedTime.Add(4*time.Minute))
	evidence, err := learning.NewEvidence(mustID(t, "evidence.diagnostic"), student.ID, concept.ID, learning.EvidenceDiagnostic, "diagnostic fixture", mustScore(t, 1), answeredAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	attempt, err = attempt.Record(learning.DiagnosticObservation{ItemID: diagnostic.Items()[0].ID, ConceptID: concept.ID, Score: evidence.Score, EvidenceID: evidence.ID, AnsweredAt: answeredAt})
	if err != nil {
		t.Fatal(err)
	}
	attempt, err = attempt.Complete(answeredAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.Diagnostics.Save(ctx, attempt); err != nil {
		t.Fatal(err)
	}
	loaded, err := repositories.Diagnostics.Get(ctx, attempt.ID)
	if err != nil || !reflect.DeepEqual(loaded, attempt) {
		t.Fatalf("Get() = (%+v, %v), want %+v", loaded, err, attempt)
	}
	found, err := repositories.Diagnostics.Find(ctx, student.ID, instance.ID, diagnostic.Reference)
	if err != nil || found.ID != attempt.ID {
		t.Fatalf("Find() = (%+v, %v)", found, err)
	}
	if _, err := database.sql.ExecContext(ctx, "DELETE FROM learning_evidence WHERE id=?", evidence.ID.String()); err == nil {
		t.Fatal("SQLite deleted evidence referenced by diagnostic observation")
	}
	if err := repositories.Diagnostics.Save(ctx, attempt); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("Save(terminal) error = %v", err)
	}
}

func sqliteDiagnosticFixture(t *testing.T, curriculum learning.CurriculumRef, conceptID learning.ID) learning.Diagnostic {
	t.Helper()
	diagnostic, err := learning.NewDiagnostic(learning.DiagnosticContractVersion, learning.DiagnosticScoringPolicyVersion,
		learning.DiagnosticRef{ID: mustID(t, "diagnostic.sqlite"), Version: "1.0.0"}, curriculum, "SQLite diagnostic",
		[]learning.DiagnosticSection{{ID: mustID(t, "section.sqlite"), Title: "Section", Items: []learning.DiagnosticItem{{
			ID: mustID(t, "item.sqlite"), ConceptID: conceptID, Kind: learning.DiagnosticSingleChoice, Prompt: "Choose yes",
			Options: []learning.DiagnosticOption{{Value: "yes", Label: "Yes"}, {Value: "no", Label: "No"}}, AcceptedAnswers: []string{"yes"},
		}}}})
	if err != nil {
		t.Fatal(err)
	}
	return diagnostic
}

func TestSQLiteMasteryThresholdRoundTripAndConstraints(t *testing.T) {
	t.Parallel()
	database, _ := openTestDatabase(t)
	ctx := context.Background()
	student := testStudent(t)
	if err := database.LearningRepositories().Students.Create(ctx, student); err != nil {
		t.Fatal(err)
	}
	settings, err := learning.NewMasteryThresholdSettings(student.ID, mustTimestamp(t, fixedTime))
	if err != nil {
		t.Fatal(err)
	}
	strict, _ := learning.NewMasteryThreshold(.85)
	settings, _ = settings.SetWorkspaceOverride(strict, mustTimestamp(t, fixedTime.Add(time.Minute)))
	if err := database.LearningRepositories().Mastery.Save(ctx, settings); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := database.LearningRepositories().Mastery.Get(ctx, student.ID)
	if err != nil || got.WorkspaceOverride == nil || got.WorkspaceOverride.Value() != .85 || got.PolicyVersion != learning.MasteryThresholdPolicyVersion {
		t.Fatalf("Get() = (%+v, %v)", got, err)
	}
	if _, err := database.sql.ExecContext(ctx, "UPDATE mastery_threshold_settings SET workspace_override = 0.49 WHERE student_id = ?", student.ID.String()); err == nil {
		t.Fatal("SQLite accepted workspace threshold below 50%")
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO mastery_threshold_settings
(student_id, policy_version, student_default, updated_at) VALUES ('student.missing', 'threshold-v1', 0.8, ?)`, encodeTimestamp(mustTimestamp(t, fixedTime))); err == nil {
		t.Fatal("SQLite accepted mastery settings without a student")
	}
}

func testStudent(t *testing.T) learning.Student {
	t.Helper()
	student, err := learning.NewStudent(mustID(t, "student-1"), learning.StudentProfile{DisplayName: "Ada", Experience: learning.ExperienceBeginner, PreferredLanguage: "es-PE", Preferences: []learning.StudyPreference{learning.PreferencePractice, learning.PreferenceTheoryFirst}, Availability: learning.Availability{DailyMinutes: 60, WeeklyDaysTarget: 3, PreferredDays: []int{1, 3, 5}}, Timezone: "America/Lima"}, mustTimestamp(t, fixedTime))
	if err != nil {
		t.Fatal(err)
	}
	return student
}
func testGoal(t *testing.T, studentID learning.ID) learning.LearningGoal {
	t.Helper()
	goal, err := learning.NewLearningGoal(mustID(t, "goal-1"), studentID, learning.GoalDetails{
		Title: "Learn a subject", Domain: "General", TargetOutcome: "Apply the subject",
		StartingLevel: learning.ExperienceNovice,
	}, mustThreshold(t, .8), mustTimestamp(t, fixedTime))
	if err != nil {
		t.Fatal(err)
	}
	return goal
}
func mustID(t *testing.T, value string) learning.ID {
	t.Helper()
	id, err := learning.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func mustTimestamp(t *testing.T, value time.Time) learning.Timestamp {
	t.Helper()
	timestamp, err := learning.NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}
func mustScore(t *testing.T, value float64) learning.MasteryScore {
	t.Helper()
	score, err := learning.NewMasteryScore(value)
	if err != nil {
		t.Fatal(err)
	}
	return score
}
func mustThreshold(t *testing.T, value float64) learning.MasteryThreshold {
	t.Helper()
	threshold, err := learning.NewMasteryThreshold(value)
	if err != nil {
		t.Fatal(err)
	}
	return threshold
}
