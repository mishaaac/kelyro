package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

var _ application.UnitOfWork = (*memory.Store)(nil)

func TestStudentAndGoalServicesUsePersistenceNeutralRepositories(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	students := application.NewStudentService(repositories.Students)
	goals := application.NewGoalService(repositories.Goals)
	student := testStudent(t)
	goal := testGoal(t, student.ID)

	if err := students.Create(ctx, student); err != nil {
		t.Fatalf("StudentService.Create() error = %v", err)
	}
	if err := goals.Create(ctx, goal); err != nil {
		t.Fatalf("GoalService.Create() error = %v", err)
	}
	gotStudent, err := students.Get(ctx, student.ID)
	if err != nil {
		t.Fatalf("StudentService.Get() error = %v", err)
	}
	gotStudent.Profile.Preferences[0] = learning.PreferenceProjects
	reloaded, err := students.Get(ctx, student.ID)
	if err != nil {
		t.Fatalf("StudentService.Get() reload error = %v", err)
	}
	if reloaded.Profile.Preferences[0] != learning.PreferenceTheoryFirst {
		t.Fatal("memory repository leaked a mutable student slice")
	}

	listed, err := goals.List(ctx, student.ID)
	if err != nil {
		t.Fatalf("GoalService.List() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != goal.ID {
		t.Fatalf("GoalService.List() = %+v", listed)
	}

	goal.Status = learning.GoalActive
	goal.UpdatedAt = testTimestamp(t, 2)
	if err := goals.Update(ctx, goal); err != nil {
		t.Fatalf("GoalService.Update() error = %v", err)
	}
	updated, err := goals.Get(ctx, goal.ID)
	if err != nil || updated.Status != learning.GoalActive {
		t.Fatalf("GoalService.Get() = (%+v, %v)", updated, err)
	}
}

func TestServicesClassifyDomainAndRepositoryErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	service := application.NewStudentService(store.Repositories().Students)
	student := testStudent(t)

	invalidStudent := student
	invalidStudent.Profile.Timezone = "Mars/Olympus"
	if err := service.Create(ctx, invalidStudent); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("invalid Create() error = %v, want invalid state", err)
	}
	if err := service.Create(ctx, student); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	if err := service.Create(ctx, student); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate Create() error = %v, want conflict", err)
	}
	if _, err := service.Get(ctx, testID(t, "student.missing")); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("missing Get() error = %v, want not found", err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.Get(canceled, student.ID); !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("canceled Get() error = %v, want unavailable", err)
	}

	wantCause := errors.New("driver exploded")
	failing := application.NewStudentService(failingStudentRepository{err: wantCause})
	_, err := failing.Get(ctx, student.ID)
	if !errors.Is(err, application.ErrPersistenceFailure) || !errors.Is(err, wantCause) {
		t.Fatalf("unclassified repository error = %v, want persistence failure preserving cause", err)
	}
	if kind, ok := application.KindOf(err); !ok || kind != application.ErrorPersistenceFailure {
		t.Fatalf("KindOf() = (%q, %v)", kind, ok)
	}
}

func TestProgressServiceAssemblesStoredFactsWithoutCalculatingMastery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	service := application.NewProgressService(repositories.Concepts, repositories.Evidence, repositories.Mistakes)
	studentID := testID(t, "student.ada")
	conceptID := testID(t, "concept.mean")
	observedAt := testTimestamp(t, 3)
	introducedAt := testTimestamp(t, 1)
	state := learning.ConceptState{
		StudentID: studentID, ConceptID: conceptID, Exposure: learning.ExposurePracticing,
		Mastery: testScore(t, 0.4), IntroducedAt: &introducedAt, UpdatedAt: observedAt,
	}
	evidence, err := learning.NewEvidence(
		testID(t, "evidence.1"), studentID, conceptID, learning.EvidencePractice,
		"fixture.practice.1", testScore(t, 0.9), observedAt,
	)
	if err != nil {
		t.Fatalf("NewEvidence() error = %v", err)
	}
	mistake, err := learning.NewMistake(
		testID(t, "mistake.1"), studentID, conceptID, "Confused mean and median", observedAt,
	)
	if err != nil {
		t.Fatalf("NewMistake() error = %v", err)
	}

	if err := service.SaveConceptState(ctx, state); err != nil {
		t.Fatalf("SaveConceptState() error = %v", err)
	}
	if err := service.RecordEvidence(ctx, evidence); err != nil {
		t.Fatalf("RecordEvidence() error = %v", err)
	}
	if err := service.RecordMistake(ctx, mistake); err != nil {
		t.Fatalf("RecordMistake() error = %v", err)
	}
	progress, err := service.Concept(ctx, studentID, conceptID)
	if err != nil {
		t.Fatalf("Concept() error = %v", err)
	}
	if progress.State.Mastery.Value() != 0.4 || len(progress.Evidence) != 1 || len(progress.Mistakes) != 1 {
		t.Fatalf("Concept() = %+v", progress)
	}
}

func TestMemoryUnitOfWorkCommitsAndRollsBackRepositoryWrites(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	studentID := testID(t, "student.ada")
	conceptID := testID(t, "concept.mean")
	observedAt := testTimestamp(t, 3)
	evidence, err := learning.NewEvidence(
		testID(t, "evidence.rollback"), studentID, conceptID, learning.EvidencePractice,
		"fixture.practice.rollback", testScore(t, 0.7), observedAt,
	)
	if err != nil {
		t.Fatalf("NewEvidence() error = %v", err)
	}
	wantRollback := errors.New("cancel progress update")
	err = store.WithinTransaction(ctx, func(repositories application.Repositories) error {
		if err := repositories.Evidence.Append(ctx, evidence); err != nil {
			return err
		}
		return wantRollback
	})
	if !errors.Is(err, wantRollback) {
		t.Fatalf("WithinTransaction() rollback error = %v", err)
	}
	items, err := store.Repositories().Evidence.ListByConcept(ctx, studentID, conceptID)
	if err != nil || len(items) != 0 {
		t.Fatalf("evidence after rollback = (%+v, %v)", items, err)
	}

	if err := store.WithinTransaction(ctx, func(repositories application.Repositories) error {
		return repositories.Evidence.Append(ctx, evidence)
	}); err != nil {
		t.Fatalf("WithinTransaction() commit error = %v", err)
	}
	items, err = store.Repositories().Evidence.ListByConcept(ctx, studentID, conceptID)
	if err != nil || len(items) != 1 {
		t.Fatalf("evidence after commit = (%+v, %v)", items, err)
	}
}

func TestSessionReviewAnalyticsAndDailyPlanServices(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	studentID := testID(t, "student.ada")
	goalID := testID(t, "goal.statistics")
	conceptID := testID(t, "concept.mean")
	start := testTimestamp(t, 10)
	end := testTimestamp(t, 12)
	session, err := learning.NewLearningSession(
		testID(t, "session.1"), studentID, goalID, start, end,
		[]learning.StudyActivity{{
			ID: testID(t, "activity.1"), ConceptIDs: []learning.ID{conceptID},
			Type: learning.ActivityTheory, StartedAt: start, EndedAt: testTimestamp(t, 11),
		}},
	)
	if err != nil {
		t.Fatalf("NewLearningSession() error = %v", err)
	}
	sessions := application.NewSessionService(repositories.Sessions)
	if err := sessions.Record(ctx, session); err != nil {
		t.Fatalf("SessionService.Record() error = %v", err)
	}
	listedSessions, err := sessions.List(ctx, studentID, goalID)
	if err != nil || len(listedSessions) != 1 {
		t.Fatalf("SessionService.List() = (%+v, %v)", listedSessions, err)
	}

	reviews := application.NewReviewService(repositories.Reviews)
	review := learning.ReviewItem{
		ID: testID(t, "review.1"), StudentID: studentID, ConceptID: conceptID,
		DueAt: end, Status: learning.ReviewPending,
	}
	if err := reviews.Create(ctx, review); err != nil {
		t.Fatalf("ReviewService.Create() error = %v", err)
	}
	due, err := reviews.Due(ctx, studentID, end)
	if err != nil || len(due) != 1 {
		t.Fatalf("ReviewService.Due() = (%+v, %v)", due, err)
	}

	analytics := application.NewAnalyticsService(repositories.Analytics)
	snapshot := learning.AnalyticsSnapshot{
		StudentID: studentID, CapturedAt: end, StudyMinutes: 120,
		SessionsCompleted: 1, ConceptsIntroduced: 1,
	}
	if err := analytics.Record(ctx, snapshot); err != nil {
		t.Fatalf("AnalyticsService.Record() error = %v", err)
	}
	latest, err := analytics.Latest(ctx, studentID)
	if err != nil || latest.StudyMinutes != 120 {
		t.Fatalf("AnalyticsService.Latest() = (%+v, %v)", latest, err)
	}

	plans := application.NewDailyPlanService(repositories.DailyPlans)
	plan := learning.DailyPlan{
		ID: testID(t, "plan.1"), StudentID: studentID, GoalID: goalID, Date: start, CreatedAt: start,
		Items: []learning.DailyPlanItem{{
			ID: testID(t, "plan-item.1"), Type: learning.DailyPlanLearn,
			ConceptIDs: []learning.ID{conceptID}, EstimatedMinutes: 20, Position: 0,
		}},
	}
	if err := plans.Save(ctx, plan); err != nil {
		t.Fatalf("DailyPlanService.Save() error = %v", err)
	}
	loadedPlan, err := plans.ForDate(ctx, studentID, goalID, start)
	if err != nil || loadedPlan.ID != plan.ID {
		t.Fatalf("DailyPlanService.ForDate() = (%+v, %v)", loadedPlan, err)
	}
}

type failingStudentRepository struct{ err error }

func (repository failingStudentRepository) Create(context.Context, learning.Student) error {
	return repository.err
}

func (repository failingStudentRepository) Get(context.Context, learning.ID) (learning.Student, error) {
	return learning.Student{}, repository.err
}

func (repository failingStudentRepository) Update(context.Context, learning.Student) error {
	return repository.err
}

func testStudent(t *testing.T) learning.Student {
	t.Helper()
	student, err := learning.NewStudent(
		testID(t, "student.ada"),
		learning.StudentProfile{
			DisplayName: "Ada", Experience: learning.ExperienceBeginner, PreferredLanguage: "en",
			Preferences:  []learning.StudyPreference{learning.PreferenceTheoryFirst},
			Availability: learning.Availability{DailyMinutes: 60, WeeklyDaysTarget: 3, PreferredDays: []int{1, 3, 5}},
			Timezone:     "UTC",
		},
		testTimestamp(t, 1),
	)
	if err != nil {
		t.Fatalf("NewStudent() error = %v", err)
	}
	return student
}

func testGoal(t *testing.T, studentID learning.ID) learning.LearningGoal {
	t.Helper()
	threshold, err := learning.NewMasteryThreshold(0.8)
	if err != nil {
		t.Fatalf("NewMasteryThreshold() error = %v", err)
	}
	goal, err := learning.NewLearningGoal(
		testID(t, "goal.statistics"), studentID, "Learn statistics", threshold, testTimestamp(t, 1),
	)
	if err != nil {
		t.Fatalf("NewLearningGoal() error = %v", err)
	}
	return goal
}

func testID(t *testing.T, value string) learning.ID {
	t.Helper()
	id, err := learning.NewID(value)
	if err != nil {
		t.Fatalf("NewID(%q) error = %v", value, err)
	}
	return id
}

func testTimestamp(t *testing.T, hour int64) learning.Timestamp {
	t.Helper()
	timestamp, err := learning.NewTimestamp(time.Unix(hour*3600, 0))
	if err != nil {
		t.Fatalf("NewTimestamp() error = %v", err)
	}
	return timestamp
}

func testScore(t *testing.T, value float64) learning.MasteryScore {
	t.Helper()
	score, err := learning.NewMasteryScore(value)
	if err != nil {
		t.Fatalf("NewMasteryScore() error = %v", err)
	}
	return score
}
