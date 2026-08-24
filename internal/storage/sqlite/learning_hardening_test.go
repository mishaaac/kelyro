package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
)

func TestOpenRejectsOrphanLearningEvidenceWithoutDisclosingContent(t *testing.T) {
	database, root := openTestDatabase(t)
	ctx := context.Background()
	if _, err := database.sql.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatal(err)
	}
	const privateSource = "private diagnostic answer 8f29"
	_, err := database.sql.ExecContext(ctx, `INSERT INTO learning_evidence
(id,student_id,concept_id,evidence_type,source,score,observed_at,mastery_evidence_type,confidence,independence,difficulty,algorithm_version)
VALUES ('evidence.orphan','student.missing','concept.missing','diagnostic',?,0.5,?,'diagnostic_objective',1,1,0.5,'diagnostic-scoring-v1')`,
		privateSource, encodeTimestamp(mustTimestamp(t, fixedTime)))
	if err != nil {
		t.Fatalf("insert orphan evidence: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(ctx, root)
	if reopened != nil {
		_ = reopened.Close()
		t.Fatal("Open() accepted orphan learning evidence")
	}
	if !errors.Is(err, ErrIntegrity) {
		t.Fatalf("Open() error = %v, want ErrIntegrity", err)
	}
	if strings.Contains(err.Error(), privateSource) || strings.Contains(err.Error(), "student.missing") {
		t.Fatalf("integrity error disclosed row content: %q", err)
	}
}

func TestSnapshotValidatorRejectsRelationalCorruption(t *testing.T) {
	database, _ := openTestDatabase(t)
	ctx := context.Background()
	if _, err := database.sql.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO mistakes
(id,student_id,concept_id,description,occurred_at,mistake_key,category,summary,first_seen_at,last_seen_at,occurrences,status,source_ref)
VALUES ('mistake.orphan','student.missing','concept.missing','private',?,'orphan','unknown','private',?,?,1,'recent','test/hardening')`,
		encodeTimestamp(mustTimestamp(t, fixedTime)), encodeTimestamp(mustTimestamp(t, fixedTime)), encodeTimestamp(mustTimestamp(t, fixedTime))); err != nil {
		t.Fatal(err)
	}
	if _, err := (SnapshotValidator{}).Validate(ctx, database.Path()); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("SnapshotValidator.Validate() error = %v, want ErrIntegrity", err)
	}
}

func TestCurriculumMembershipGuardAndIntegrityScanRejectDanglingState(t *testing.T) {
	database, root := openTestDatabase(t)
	ctx := context.Background()
	student, goal, instance, outsideConcept := hardeningCurriculumScope(t, database)
	state, err := learning.NewInstanceConceptState(instance, outsideConcept, mustTimestamp(t, fixedTime.Add(time.Minute)))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.LearningRepositories().InstanceConceptStates.Save(ctx, state); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("cross-curriculum state error = %v, want invalid_state", err)
	}

	for _, trigger := range []string{
		"learner_curriculum_concept_states_membership_insert_guard",
		"learner_curriculum_concept_states_membership_update_guard",
	} {
		if _, err := database.sql.ExecContext(ctx, "DROP TRIGGER "+trigger); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.LearningRepositories().InstanceConceptStates.Save(ctx, state); err != nil {
		t.Fatalf("seed simulated legacy corruption: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, root)
	if reopened != nil {
		_ = reopened.Close()
		t.Fatal("Open() accepted a concept state outside its curriculum instance")
	}
	if !errors.Is(err, ErrIntegrity) || !strings.Contains(err.Error(), "curriculum state outside") {
		t.Fatalf("Open() error = %v, want safe dangling-state finding", err)
	}
	if strings.Contains(err.Error(), student.ID.String()) || strings.Contains(err.Error(), outsideConcept.String()) || strings.Contains(err.Error(), goal.ID.String()) {
		t.Fatalf("integrity error disclosed learner identifiers: %q", err)
	}
}

func TestDiagnosticObservationGuardRejectsMismatchedEvidenceOwnership(t *testing.T) {
	database, _ := openTestDatabase(t)
	ctx := context.Background()
	student, _, instance, outsideConcept := hardeningCurriculumScope(t, database)
	timestamp := encodeTimestamp(mustTimestamp(t, fixedTime.Add(time.Minute)))
	fingerprint := "sha256:" + strings.Repeat("a", 64)
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO diagnostic_attempts
(id,student_id,curriculum_instance_id,diagnostic_id,diagnostic_version,definition_fingerprint,status,started_at,updated_at)
VALUES ('attempt.hardening',?,?,'diagnostic.hardening','1.0.0',?,'in_progress',?,?)`,
		student.ID.String(), instance.ID.String(), fingerprint, timestamp, timestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO learning_evidence
(id,student_id,concept_id,evidence_type,source,score,observed_at,mastery_evidence_type,confidence,independence,difficulty,algorithm_version)
VALUES ('evidence.hardening',?,?,'diagnostic','fixture/hardening',0.5,?,'diagnostic_objective',1,1,0.5,'diagnostic-scoring-v1')`,
		student.ID.String(), outsideConcept.String(), timestamp); err != nil {
		t.Fatal(err)
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO diagnostic_observations
(attempt_id,item_id,concept_id,evidence_id,score,answered_at,position)
VALUES ('attempt.hardening','item.hardening',?,'evidence.hardening',0.5,?,0)`, outsideConcept.String(), timestamp); err == nil {
		t.Fatal("diagnostic observation accepted evidence outside the attempt curriculum")
	}
}

func TestOpenRejectsInvalidStudentCoreTimezone(t *testing.T) {
	database, root := openTestDatabase(t)
	student := testStudent(t)
	if err := database.LearningRepositories().Students.Create(context.Background(), student); err != nil {
		t.Fatal(err)
	}
	if _, err := database.sql.ExecContext(context.Background(), "UPDATE student_profiles SET timezone='Mars/Olympus_Mons' WHERE student_id=?", student.ID.String()); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(context.Background(), root)
	if reopened != nil {
		_ = reopened.Close()
		t.Fatal("Open() accepted invalid Student Core timezone")
	}
	if !errors.Is(err, ErrIntegrity) || strings.Contains(err.Error(), "Mars/Olympus_Mons") {
		t.Fatalf("Open() error = %v, want safe timezone integrity finding", err)
	}
}

func TestStudentProfileSchemaContainsOnlyRequiredLearningFields(t *testing.T) {
	database, _ := openTestDatabase(t)
	rows, err := database.sql.QueryContext(context.Background(), "PRAGMA table_info(student_profiles)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, kind string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, name)
	}
	want := []string{"student_id", "display_name", "experience", "weekly_minutes", "preferred_display_name", "preferred_language", "daily_minutes", "weekly_days_target", "timezone"}
	if !reflect.DeepEqual(columns, want) {
		t.Fatalf("student profile columns = %v, want privacy allowlist %v", columns, want)
	}
}

func TestConcurrentActiveGoalWritesHaveSingleWinner(t *testing.T) {
	first, root := openTestDatabase(t)
	second, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	student := testStudent(t)
	if err := first.LearningRepositories().Students.Create(context.Background(), student); err != nil {
		t.Fatal(err)
	}
	goals := make([]learning.LearningGoal, 2)
	for index := range goals {
		goal := testGoal(t, student.ID)
		goal.ID = mustID(t, "goal.concurrent."+string(rune('a'+index)))
		goals[index], err = goal.Activate(mustTimestamp(t, fixedTime.Add(time.Minute)))
		if err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for index, database := range []*Database{first, second} {
		go func(index int, database *Database) {
			<-start
			results <- database.LearningRepositories().Goals.Create(context.Background(), goals[index])
		}(index, database)
	}
	close(start)
	var successes, conflicts int
	for range goals {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, application.ErrConflict):
			conflicts++
		default:
			t.Fatalf("concurrent goal write error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent goal writes successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestLearningWriteClassifiesBusyDatabaseAsUnavailable(t *testing.T) {
	first, root := openTestDatabase(t)
	second, err := Open(context.Background(), root, WithOperationTimeout(75*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	student := testStudent(t)
	if err := first.LearningRepositories().Students.Create(context.Background(), student); err != nil {
		t.Fatal(err)
	}
	transaction, err := first.sql.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.ExecContext(context.Background(), "UPDATE students SET updated_at=updated_at WHERE id=?", student.ID.String()); err != nil {
		_ = transaction.Rollback()
		t.Fatal(err)
	}
	goal := testGoal(t, student.ID)
	err = second.LearningRepositories().Goals.Create(context.Background(), goal)
	if rollbackErr := transaction.Rollback(); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("busy learning write error = %v, want unavailable", err)
	}
}

func TestVersionedLargeStudentCoreFixtureUsesBoundedIndexedProjections(t *testing.T) {
	if raceEnabled {
		t.Skip("deterministic 11,000-row scale fixture is covered by the normal suite")
	}
	const (
		phaseCount    = 50
		moduleCount   = 150
		lessonCount   = 500
		conceptCount  = 2000
		evidenceCount = 6000
	)
	root := newWorkspaceRoot(t)
	database, err := Open(context.Background(), root, WithOperationTimeout(30*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	ctx := context.Background()
	curriculum := studentCoreScaleFixture(t)
	counts := map[learning.CurriculumNodeType]int{}
	for _, node := range curriculum.Nodes {
		counts[node.Type]++
	}
	if counts[learning.CurriculumNodePhase] != phaseCount || counts[learning.CurriculumNodeModule] != moduleCount ||
		counts[learning.CurriculumNodeLesson] != lessonCount || counts[learning.CurriculumNodeConcept] != conceptCount {
		t.Fatalf("large fixture hierarchy counts = %v", counts)
	}
	repositories := database.LearningRepositories()
	student := testStudent(t)
	goal := testGoal(t, student.ID)
	if err := repositories.Students.Create(ctx, student); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Goals.Create(ctx, goal); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Definitions.Install(ctx, curriculum); err != nil {
		t.Fatal(err)
	}
	instance, err := learning.NewCurriculumInstance(mustID(t, "instance.student-core-scale"), student.ID, goal.ID,
		curriculum.Reference, learning.CurriculumSourceFixture, mustTimestamp(t, fixedTime))
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.CurriculumInstances.Create(ctx, instance); err != nil {
		t.Fatal(err)
	}
	seedLargeStudentFacts(t, database, student.ID, instance.ID, curriculum, evidenceCount)

	outline, err := repositories.Curricula.Outline(ctx, curriculum.Reference)
	if err != nil || len(outline) != len(curriculum.Nodes) {
		t.Fatalf("large outline = (%d, %v), want %d", len(outline), err, len(curriculum.Nodes))
	}
	planning, err := repositories.Curricula.PlanningConcepts(ctx, curriculum.Reference)
	if err != nil || len(planning) != conceptCount {
		t.Fatalf("large planning projection = (%d, %v), want %d", len(planning), err, conceptCount)
	}
	states, err := repositories.InstanceConceptStates.ListByInstance(ctx, instance.ID)
	if err != nil || len(states) != conceptCount {
		t.Fatalf("large state projection = (%d, %v), want %d", len(states), err, conceptCount)
	}
	evidence, err := repositories.Evidence.ListByStudent(ctx, student.ID)
	if err != nil || len(evidence) != evidenceCount {
		t.Fatalf("large evidence projection = (%d, %v), want %d", len(evidence), err, evidenceCount)
	}

	assertQueryPlanUses(t, database, "learner_curriculum_instances_timeline_idx",
		"SELECT id FROM learner_curriculum_instances WHERE student_id=? ORDER BY created_at,id", student.ID.String())
	assertQueryPlanUses(t, database, "study_session_lifecycle_student_timeline_idx",
		"SELECT id FROM study_session_lifecycle WHERE student_id=? ORDER BY started_at,id", student.ID.String())
	assertQueryPlanUses(t, database, "review_items_student_timeline_idx",
		"SELECT id FROM review_items WHERE student_id=? ORDER BY due_at,id", student.ID.String())
	assertQueryPlanUses(t, database, "learning_evidence_concept_idx",
		"SELECT id FROM learning_evidence WHERE student_id=? ORDER BY concept_id,observed_at,id", student.ID.String())
}

func hardeningCurriculumScope(t *testing.T, database *Database) (learning.Student, learning.LearningGoal, learning.CurriculumInstance, learning.ID) {
	t.Helper()
	ctx := context.Background()
	repositories := database.LearningRepositories()
	student := testStudent(t)
	if err := repositories.Students.Create(ctx, student); err != nil {
		t.Fatal(err)
	}
	goal := testGoal(t, student.ID)
	if err := repositories.Goals.Create(ctx, goal); err != nil {
		t.Fatal(err)
	}
	inside := learning.Concept{ID: mustID(t, "concept.hardening.inside"), TopicID: mustID(t, "topic.hardening.inside"), Title: "Inside"}
	outside := learning.Concept{ID: mustID(t, "concept.hardening.outside"), TopicID: mustID(t, "topic.hardening.outside"), Title: "Outside"}
	reference := learning.CurriculumRef{ID: mustID(t, "curriculum.hardening.inside"), Version: "hardening-v1"}
	outsideReference := learning.CurriculumRef{ID: mustID(t, "curriculum.hardening.outside"), Version: "hardening-v1"}
	if err := database.SeedCurriculum(ctx, reference, []learning.Concept{inside}, nil); err != nil {
		t.Fatal(err)
	}
	if err := database.SeedCurriculum(ctx, outsideReference, []learning.Concept{outside}, nil); err != nil {
		t.Fatal(err)
	}
	instance, err := learning.NewCurriculumInstance(mustID(t, "instance.hardening"), student.ID, goal.ID, reference,
		learning.CurriculumSourceFixture, mustTimestamp(t, fixedTime))
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.CurriculumInstances.Create(ctx, instance); err != nil {
		t.Fatal(err)
	}
	return student, goal, instance, outside.ID
}

const studentCoreScaleFixtureVersion = "student-core-scale/v1"

func studentCoreScaleFixture(t *testing.T) learning.Curriculum {
	t.Helper()
	status := learning.CurriculumStatusMetadata{State: learning.CurriculumNodeActive}
	nodes := make([]learning.CurriculumNode, 0, 3200)
	lessonIndex, conceptIndex := 0, 0
	for phaseIndex := 0; phaseIndex < 50; phaseIndex++ {
		phaseID := mustID(t, fmt.Sprintf("phase.scale.%02d", phaseIndex))
		nodes = append(nodes, scaleNode(phaseID, learning.CurriculumNodePhase, nil, phaseIndex, status, nil))
		for moduleOffset := 0; moduleOffset < 3; moduleOffset++ {
			moduleIndex := phaseIndex*3 + moduleOffset
			moduleID := mustID(t, fmt.Sprintf("module.scale.%03d", moduleIndex))
			nodes = append(nodes, scaleNode(moduleID, learning.CurriculumNodeModule, &phaseID, moduleOffset, status, nil))
			lessonsInModule := 3
			if moduleIndex < 50 {
				lessonsInModule = 4
			}
			for lessonOffset := 0; lessonOffset < lessonsInModule; lessonOffset++ {
				lessonID := mustID(t, fmt.Sprintf("lesson.scale.%03d", lessonIndex))
				nodes = append(nodes, scaleNode(lessonID, learning.CurriculumNodeLesson, &moduleID, lessonOffset, status, nil))
				topicID := mustID(t, fmt.Sprintf("topic.scale.%03d", lessonIndex))
				nodes = append(nodes, scaleNode(topicID, learning.CurriculumNodeTopic, &lessonID, 0, status, nil))
				var previous learning.ID
				for conceptOffset := 0; conceptOffset < 4; conceptOffset++ {
					conceptID := mustID(t, fmt.Sprintf("concept.scale.%04d", conceptIndex))
					definition := &learning.ConceptDefinition{
						Objectives: []string{"Understand the deterministic scale concept."},
						Difficulty: learning.ConceptDifficultyFoundational, EstimatedEffortMinutes: 10,
						AssessmentExpectations: []string{"Explain the deterministic scale concept."},
					}
					if conceptOffset > 0 {
						definition.Prerequisites = []learning.ConceptPrerequisite{{ConceptID: previous, Requirement: learning.PrerequisiteMastered}}
					}
					nodes = append(nodes, scaleNode(conceptID, learning.CurriculumNodeConcept, &topicID, conceptOffset, status, definition))
					previous = conceptID
					conceptIndex++
				}
				lessonIndex++
			}
		}
	}
	curriculum, err := learning.NewCurriculum(learning.CurriculumContractVersion,
		learning.CurriculumRef{ID: mustID(t, "fixture.student-core-scale"), Version: studentCoreScaleFixtureVersion},
		"Student Core scale fixture", "Deterministic integrity and query-plan fixture.", nodes)
	if err != nil {
		t.Fatal(err)
	}
	return curriculum
}

func scaleNode(id learning.ID, kind learning.CurriculumNodeType, parent *learning.ID, order int,
	status learning.CurriculumStatusMetadata, definition *learning.ConceptDefinition,
) learning.CurriculumNode {
	return learning.CurriculumNode{ID: id, Type: kind, ParentID: parent, Title: id.String(),
		Description: "Deterministic Student Core scale fixture node.", Order: order, Status: status,
		Version: studentCoreScaleFixtureVersion, Concept: definition}
}

func seedLargeStudentFacts(t *testing.T, database *Database, studentID, instanceID learning.ID,
	curriculum learning.Curriculum, evidenceCount int,
) {
	t.Helper()
	tx, err := database.sql.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	stateStatement, err := tx.Prepare(`INSERT INTO learner_curriculum_concept_states
(curriculum_instance_id,student_id,concept_id,exposure,mastery,mastery_algorithm_version,progression_policy_version,manual_flags_json,updated_at)
VALUES (?, ?, ?, 'not_seen', 0, 'mastery-v1', 'progression-v1', '[]', ?)`)
	if err != nil {
		t.Fatal(err)
	}
	defer stateStatement.Close()
	evidenceStatement, err := tx.Prepare(`INSERT INTO learning_evidence
(id,student_id,concept_id,evidence_type,source,score,observed_at,mastery_evidence_type,confidence,independence,difficulty,algorithm_version)
VALUES (?, ?, ?, 'practice', 'fixture/student-core-scale/v1', 0.5, ?, 'practice_success', 1, 1, 0.5, 'fixture/student-core-scale/v1')`)
	if err != nil {
		t.Fatal(err)
	}
	defer evidenceStatement.Close()
	timestamp := encodeTimestamp(mustTimestamp(t, fixedTime))
	concepts := make([]learning.ID, 0, 2000)
	for _, node := range curriculum.Nodes {
		if node.Type == learning.CurriculumNodeConcept {
			concepts = append(concepts, node.ID)
			if _, err := stateStatement.Exec(instanceID.String(), studentID.String(), node.ID.String(), timestamp); err != nil {
				t.Fatal(err)
			}
		}
	}
	for index := 0; index < evidenceCount; index++ {
		conceptID := concepts[index%len(concepts)]
		if _, err := evidenceStatement.Exec(fmt.Sprintf("evidence.scale.%05d", index), studentID.String(), conceptID.String(), timestamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := stateStatement.Close(); err != nil {
		t.Fatal(err)
	}
	if err := evidenceStatement.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func assertQueryPlanUses(t *testing.T, database *Database, index, query string, arguments ...any) {
	t.Helper()
	rows, err := database.sql.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, arguments...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var details []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		details = append(details, detail)
	}
	joined := strings.Join(details, " | ")
	if !strings.Contains(joined, index) || strings.Contains(joined, "USE TEMP B-TREE") {
		t.Fatalf("query plan = %q, want index %q without temporary sort", joined, index)
	}
}
