package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
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
	want := []string{"curriculum_nodes_concept_idx", "diagnostic_attempts_student_status_idx", "diagnostic_observations_concept_idx", "learning_goals_active_idx", "learning_goals_one_active_idx", "learning_evidence_concept_idx", "review_items_due_idx", "study_sessions_goal_timeline_idx", "study_sessions_range_idx"}
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
	concepts, err := repositories.Curricula.Concepts(ctx, reference)
	if err != nil || !reflect.DeepEqual(concepts, []learning.Concept{conceptA, conceptB}) {
		t.Fatalf("concepts=(%+v,%v)", concepts, err)
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
	mistake, _ := learning.NewMistake(mustID(t, "mistake-1"), student.ID, conceptA.ID, "mixed two rules", introduced)
	if err := repositories.Mistakes.Create(ctx, mistake); err != nil {
		t.Fatal(err)
	}
	resolved := mustTimestamp(t, fixedTime.Add(3*time.Minute))
	mistake.ResolvedAt = &resolved
	if err := repositories.Mistakes.Update(ctx, mistake); err != nil {
		t.Fatal(err)
	}
	mistakes, err := repositories.Mistakes.ListByConcept(ctx, student.ID, conceptA.ID)
	if err != nil || !reflect.DeepEqual(mistakes, []learning.Mistake{mistake}) {
		t.Fatalf("mistakes=(%+v,%v)", mistakes, err)
	}
	retention := learning.RetentionState{StudentID: student.ID, ConceptID: conceptA.ID, Strength: mustScore(t, .6), MeasuredAt: resolved}
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
	schedule, _ := learning.NewReviewSchedule(student.ID, conceptA.ID, &introduced, mustTimestamp(t, fixedTime.Add(24*time.Hour)), false)
	if err := repositories.Reviews.SaveSchedule(ctx, schedule); err != nil {
		t.Fatal(err)
	}
	gotSchedule, err := repositories.Reviews.GetSchedule(ctx, student.ID, conceptA.ID)
	if err != nil || !reflect.DeepEqual(gotSchedule, schedule) {
		t.Fatalf("schedule=(%+v,%v)", gotSchedule, err)
	}
	review := learning.ReviewItem{ID: mustID(t, "review-1"), StudentID: student.ID, ConceptID: conceptA.ID, DueAt: schedule.DueAt, Status: learning.ReviewPending}
	if err := repositories.Reviews.CreateItem(ctx, review); err != nil {
		t.Fatal(err)
	}
	due, err := repositories.Reviews.ListDue(ctx, student.ID, schedule.DueAt)
	if err != nil || !reflect.DeepEqual(due, []learning.ReviewItem{review}) {
		t.Fatalf("due=(%+v,%v)", due, err)
	}

	lastStudy := session.EndedAt
	streak := learning.Streak{StudentID: student.ID, CurrentDays: 2, LongestDays: 3, LastStudyAt: &lastStudy}
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
