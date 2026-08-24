//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/infra/curriculumyaml"
	"github.com/mishaaac/kelyro/internal/infra/developmentfixture"
	"github.com/mishaaac/kelyro/internal/infra/learningdb"
	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
)

const (
	reviewConcept       = "concept.review"
	prerequisiteConcept = "concept.prerequisite"
	nextConcept         = "concept.next"
)

func TestStudentLearningCoreEndToEnd(t *testing.T) {
	root := moduleRoot(t)
	binary := buildBinary(t, root)

	t.Run("01 new student persists setup and Today across reopen", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("piped stdin is not a Windows console handle; the service-level setup scenarios run on Windows")
		}
		test := newScenario(t, binary)
		test.mustRun("init")
		completeLearnerSetup(t, test)

		for command, expected := range map[string]string{
			"setup status": "Status: completed",
			"profile show": "Display name: Ada",
			"goal show":    "[active] Understand ratios",
			"today":        "Today\nGoal: Understand ratios",
		} {
			output := test.mustRun(strings.Fields(command)...)
			if !strings.Contains(output, expected) {
				t.Fatalf("reopened %s output missing %q:\n%s", command, expected, output)
			}
		}
	})

	t.Run("02 diagnostic persists estimated evidence and initial states", func(t *testing.T) {
		test := newScenario(t, binary)
		test.mustRun("init")
		ctx := context.Background()
		factory := learningdb.NewFactory("student-core-e2e")
		store, err := factory.Open(ctx, test.workspace)
		if err != nil {
			t.Fatal(err)
		}
		completed, attemptID := completeSetupWithDiagnostic(t, ctx, store.Setup())
		if completed.Setup.Status != learning.SetupCompleted || completed.Instance == nil {
			t.Fatalf("completed diagnostic setup = %+v", completed)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}

		reopened, err := factory.Open(ctx, test.workspace)
		if err != nil {
			t.Fatal(err)
		}
		curriculum, diagnostic, err := developmentfixture.FoundationDemo()
		if err != nil {
			t.Fatal(err)
		}
		result, err := reopened.Diagnostics().Result(ctx, attemptID, diagnostic)
		if err != nil || result.Partial || len(result.Estimates) != 2 {
			t.Fatalf("persisted diagnostic result = (%+v, %v)", result, err)
		}
		for _, estimate := range result.Estimates {
			if !estimate.Known || estimate.EvidenceCount == 0 || estimate.Confidence.Value() == 0 {
				t.Fatalf("diagnostic estimate = %+v", estimate)
			}
		}
		states, err := reopened.CurriculumInstances().States(ctx, completed.Instance.ID)
		if err != nil || len(states) != curriculumConceptCount(curriculum) {
			t.Fatalf("initial concept states = (%+v, %v)", states, err)
		}
		for _, state := range states {
			if state.Exposure != learning.ExposureNotSeen || state.Mastery.Value() != 0 {
				t.Fatalf("diagnostic invented confirmed mastery = %+v", state)
			}
		}
		if err := reopened.Close(); err != nil {
			t.Fatal(err)
		}

		database, err := sqlite.Open(ctx, test.workspace)
		if err != nil {
			t.Fatal(err)
		}
		defer database.Close()
		evidenceCount := 0
		for _, estimate := range result.Estimates {
			evidence, listErr := database.LearningRepositories().Evidence.ListByConcept(ctx, completed.Setup.StudentID, estimate.ConceptID)
			if listErr != nil {
				t.Fatal(listErr)
			}
			evidenceCount += len(evidence)
			for _, item := range evidence {
				if item.Type != learning.EvidenceDiagnosticObjective && item.Type != learning.EvidenceDiagnosticSelfReport {
					t.Fatalf("diagnostic evidence type = %s", item.Type)
				}
			}
		}
		if evidenceCount != 4 {
			t.Fatalf("diagnostic evidence count = %d, want 4", evidenceCount)
		}
	})

	t.Run("03 evidence recalculates mastery and unlocks dependent", func(t *testing.T) {
		harness := newStudentCoreHarness(t, binary, root)
		update := harness.recordMastery(t, prerequisiteConcept, "unlock")
		if !update.Progression.ThresholdMet || update.Progression.State.Exposure != learning.ExposureMastered {
			t.Fatalf("mastery progression = %+v", update.Progression)
		}
		dependent := dependentByID(t, update.Dependents, harness.id(t, nextConcept))
		if !dependent.NewlyEligible || !dependent.Decision.CanIntroduce {
			t.Fatalf("dependent unlock = %+v", dependent)
		}
		recalculated, err := harness.progression.Recalculate(harness.ctx, harness.instance.ID, harness.id(t, prerequisiteConcept), nil)
		if err != nil || !recalculated.Progression.ThresholdMet || !dependentByID(t, recalculated.Dependents, harness.id(t, nextConcept)).Decision.CanIntroduce {
			t.Fatalf("recalculated progression = (%+v, %v)", recalculated, err)
		}
		evidence, err := harness.repositories.Evidence.ListByConcept(harness.ctx, harness.student.ID, harness.id(t, prerequisiteConcept))
		if err != nil || len(evidence) != 1 {
			t.Fatalf("immutable evidence after recalculation = (%+v, %v)", evidence, err)
		}
	})

	t.Run("04 repeated mistake becomes a warm-up candidate", func(t *testing.T) {
		harness := newStudentCoreHarness(t, binary, root)
		first := harness.recordMistake(t, prerequisiteConcept)
		harness.clock.Advance(time.Minute)
		second := harness.recordMistake(t, prerequisiteConcept)
		if !first.Created || second.Created || second.Mistake.ID != first.Mistake.ID || second.Mistake.Occurrences != 2 {
			t.Fatalf("deduplicated mistake = first %+v second %+v", first, second)
		}
		plan, err := harness.warmUps.Select(harness.ctx, application.WarmUpRequest{
			Lesson: learning.WarmUpLessonCandidate{
				Curriculum: harness.curriculum.Reference, LessonID: harness.id(t, "lesson.core"),
				PrerequisiteConceptIDs: []learning.ID{harness.id(t, prerequisiteConcept)},
			},
			AvailableMinutes: 60,
		})
		if err != nil || len(plan.Items) != 1 || plan.Items[0].Concept.ID != harness.id(t, prerequisiteConcept) ||
			plan.Items[0].Reason != learning.WarmUpPrerequisiteRepeatedMistake {
			t.Fatalf("warm-up plan = (%+v, %v)", plan, err)
		}
	})

	t.Run("05 retention becomes due and successful review reschedules", func(t *testing.T) {
		harness := newStudentCoreHarness(t, binary, root)
		harness.recordMastery(t, reviewConcept, "retention")
		harness.clock.Advance(45 * 24 * time.Hour)
		calculation, err := harness.retention.Recalculate(harness.ctx, harness.id(t, reviewConcept))
		if err != nil || (calculation.State.Status != learning.RetentionDue && calculation.State.Status != learning.RetentionOverdue) {
			t.Fatalf("retention after time advance = (%+v, %v)", calculation, err)
		}
		queue, err := harness.reviews.List(harness.ctx, true)
		if err != nil || len(queue.Items) != 1 {
			t.Fatalf("due review queue = (%+v, %v)", queue, err)
		}
		completed, err := harness.reviews.RecordOutcome(harness.ctx, queue.Items[0].Item.ID, harness.score(t, .95))
		if err != nil || completed.Completed.Outcome != learning.ReviewOutcomeSuccess || completed.Next == nil ||
			!completed.Next.DueAt.After(harness.timestamp(t)) || completed.Retention.SuccessfulReviews != 1 {
			t.Fatalf("successful review reschedule = (%+v, %v)", completed, err)
		}
	})

	t.Run("06 daily plan orders review new learning and reinforcement within budget", func(t *testing.T) {
		harness := newStudentCoreHarness(t, binary, root)
		harness.recordMastery(t, reviewConcept, "daily-review")
		harness.recordMastery(t, prerequisiteConcept, "daily-prerequisite")
		harness.recordMistake(t, prerequisiteConcept)
		harness.clock.Advance(time.Minute)
		harness.recordMistake(t, prerequisiteConcept)
		harness.clock.Advance(45 * 24 * time.Hour)
		if _, err := harness.retention.Recalculate(harness.ctx, harness.id(t, reviewConcept)); err != nil {
			t.Fatal(err)
		}
		if _, err := harness.reviews.List(harness.ctx, true); err != nil {
			t.Fatal(err)
		}
		plan, err := harness.dailyPlan.Today(harness.ctx)
		if err != nil || len(plan.Items) != 3 {
			t.Fatalf("daily plan = (%+v, %v)", plan, err)
		}
		wantRoles := []learning.DailyPlanItemRole{learning.DailyPlanRoleWarmUp, learning.DailyPlanRoleReview, learning.DailyPlanRoleNewLearning}
		wantReasons := []learning.DailyPlanSelectionReason{
			learning.DailyPlanCriticalOverduePrerequisite,
			learning.DailyPlanImportantDueReview,
			learning.DailyPlanNextEligibleConcept,
		}
		wantConcepts := []learning.ID{harness.id(t, prerequisiteConcept), harness.id(t, reviewConcept), harness.id(t, nextConcept)}
		for index := range wantRoles {
			if plan.Items[index].Role != wantRoles[index] || plan.Items[index].Reason != wantReasons[index] ||
				plan.Items[index].ConceptIDs[0] != wantConcepts[index] || plan.Items[index].Position != index {
				t.Fatalf("daily plan item %d = %+v", index, plan.Items[index])
			}
		}
		if plan.PlannedMinutes+plan.BufferMinutes > plan.AvailableMinutes || plan.AvailableMinutes != 60 {
			t.Fatalf("daily plan budget = %+v", plan)
		}
	})

	t.Run("07 history and streak span consecutive days", func(t *testing.T) {
		harness := newStudentCoreHarness(t, binary, root)
		base := harness.clock.Now()
		for day := 0; day < 3; day++ {
			harness.clock.Set(base.AddDate(0, 0, day))
			if _, err := harness.sessions.Start(harness.ctx, harness.goal.ID, harness.instance.ID); err != nil {
				t.Fatal(err)
			}
			harness.clock.Advance(10 * time.Minute)
			if _, err := harness.sessions.RecordActivity(harness.ctx); err != nil {
				t.Fatal(err)
			}
			harness.clock.Advance(5 * time.Minute)
			if _, err := harness.sessions.Stop(harness.ctx); err != nil {
				t.Fatal(err)
			}
		}
		history, err := harness.history.List(harness.ctx, learning.StudyPeriodAll)
		if err != nil || len(history.Events) != 3 {
			t.Fatalf("multi-day history = (%+v, %v)", history, err)
		}
		for index := 1; index < len(history.Events); index++ {
			if history.Events[index-1].OccurredAt.Before(history.Events[index].OccurredAt) {
				t.Fatalf("history is not newest-first: %+v", history.Events)
			}
		}
		streak, err := harness.streaks.Show(harness.ctx)
		if err != nil || streak.CurrentDays != 3 || streak.LongestDays != 3 || streak.TotalActiveDays != 3 {
			t.Fatalf("three-day streak = (%+v, %v)", streak, err)
		}
		timeSummary, err := harness.history.Time(harness.ctx)
		if err != nil || timeSummary.TotalSessions != 3 || timeSummary.Total != 45*time.Minute {
			t.Fatalf("multi-day study time = (%+v, %v)", timeSummary, err)
		}
	})

	t.Run("08 generated Markdown protects manual changes", func(t *testing.T) {
		test := newScenario(t, binary)
		test.mustRun("init")
		completeSetupWithoutDiagnostic(t, test.workspace)
		test.mustRun("progress", "export")
		progressPath := filepath.Join(test.workspace, "00-roadmap", "PROGRESS.md")
		const manual = "\nStudent-owned E2E reflection.\n"
		file, err := os.OpenFile(progressPath, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteString(manual); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		before := readFile(t, progressPath)
		output, code := test.run("progress", "export")
		if code == 0 || !strings.Contains(strings.ToLower(output), "modified") {
			t.Fatalf("progress export did not protect manual edit (exit %d):\n%s", code, output)
		}
		if after := readFile(t, progressPath); !bytes.Equal(after, before) {
			t.Fatal("progress export overwrote manually edited Markdown")
		}
	})

	t.Run("09 I-01 Foundation workspace migrates forward", func(t *testing.T) {
		test := newScenario(t, binary)
		test.mustRun("init")
		path, err := platform.WorkspaceDBPath(test.workspace)
		if err != nil {
			t.Fatal(err)
		}
		for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
			if err := os.Remove(candidate); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}
		if _, err := sqlite.CreateI01FoundationFixture(context.Background(), test.workspace, time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
		test.mustRun("status")
		database, err := sqlite.Open(context.Background(), test.workspace)
		if err != nil {
			t.Fatal(err)
		}
		defer database.Close()
		version, err := database.SchemaVersion(context.Background())
		if err != nil || version != sqlite.LatestSchemaVersion() {
			t.Fatalf("migrated schema = (%d, %v), want %d", version, err, sqlite.LatestSchemaVersion())
		}
		value, found, err := database.Repositories().State.Get(context.Background(), "foundation", "i01-preserved")
		if err != nil || !found || string(value) != "preserved" {
			t.Fatalf("preserved I-01 state = (%q, %t, %v)", value, found, err)
		}
	})

	t.Run("10 Student Core commands remain offline", func(t *testing.T) {
		test := newScenario(t, binary)
		test.mustRun("init")
		completeSetupWithoutDiagnostic(t, test.workspace)
		if got := strings.TrimSpace(test.mustRun("config", "get", "privacy.allow_network")); got != "false" {
			t.Fatalf("privacy.allow_network = %q, want false", got)
		}
		for _, command := range [][]string{
			{"profile", "show"}, {"goal", "show"}, {"mastery"}, {"setup", "status"},
			{"status"}, {"progress"}, {"roadmap"}, {"today"}, {"mistakes"},
			{"session", "status"}, {"history"}, {"time"}, {"reviews", "due"}, {"streak"},
		} {
			output := test.mustRun(command...)
			if strings.Contains(output, "E2E network adapter was invoked") {
				t.Fatalf("offline Student Core command reached network adapter: %v\n%s", command, output)
			}
		}
		assertNoFileNamed(t, test.cache, "updates.json")
	})
}

type mutableClock struct{ current time.Time }

func (clock *mutableClock) Now() time.Time              { return clock.current }
func (clock *mutableClock) Advance(value time.Duration) { clock.current = clock.current.Add(value) }
func (clock *mutableClock) Set(value time.Time)         { clock.current = value }

type studentCoreHarness struct {
	ctx          context.Context
	clock        *mutableClock
	database     *sqlite.Database
	repositories application.Repositories
	student      learning.Student
	goal         learning.LearningGoal
	curriculum   learning.Curriculum
	instance     learning.CurriculumInstance
	progression  application.ProgressionService
	mistakes     application.MistakeMemoryService
	retention    application.RetentionService
	reviews      application.ReviewSchedulerService
	warmUps      application.WarmUpSelectorService
	sessions     application.StudySessionLifecycleService
	history      application.StudyHistoryService
	streaks      application.StreakService
	dailyPlan    application.AdaptiveDailyPlanService
}

func newStudentCoreHarness(t *testing.T, binary, root string) *studentCoreHarness {
	t.Helper()
	test := newScenario(t, binary)
	test.mustRun("init")
	ctx := context.Background()
	clock := &mutableClock{current: time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)}
	database, err := sqlite.Open(ctx, test.workspace, sqlite.WithAppVersion("student-core-e2e"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close Student Core E2E database: %v", err)
		}
	})
	repositories := database.LearningRepositories()
	profiles := application.NewProfileService(application.NewStudentService(repositories.Students), application.WithProfileClock(clock.Now))
	student, err := profiles.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dailyMinutes, timezone := 60, "UTC"
	student, err = profiles.Edit(ctx, application.ProfileChanges{DailyMinutes: &dailyMinutes, Timezone: &timezone})
	if err != nil {
		t.Fatal(err)
	}
	threshold, err := learning.NewMasteryThreshold(.8)
	if err != nil {
		t.Fatal(err)
	}
	goals := application.NewGoalLifecycleService(profiles, database, application.WithGoalClock(clock.Now),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return mustE2EID(t, "goal.student-core-e2e"), nil }))
	goal, err := goals.Set(ctx, application.SetGoalInput{
		Title: "Exercise Student Core", Domain: "General knowledge", TargetOutcome: "Complete the lifecycle fixture",
		StartingLevel: learning.ExperienceBeginner, MasteryThreshold: threshold,
	})
	if err != nil {
		t.Fatal(err)
	}
	curriculum := loadLifecycleCurriculum(t, root)
	instances := application.NewCurriculumInstanceService(profiles, database, application.WithCurriculumInstanceClock(clock.Now),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return mustE2EID(t, "instance.student-core-e2e"), nil }))
	instance, err := instances.Create(ctx, goal.ID, curriculum, learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	mastery := application.NewMasteryPolicyService(profiles, repositories.Mastery, application.WithMasteryPolicyClock(clock.Now))
	if _, err := mastery.SetWorkspaceOverride(ctx, threshold); err != nil {
		t.Fatal(err)
	}
	graph, err := learning.NewKnowledgeGraph(curriculum)
	if err != nil {
		t.Fatal(err)
	}
	mistakeIndex, sessionIndex := 0, 0
	harness := &studentCoreHarness{
		ctx: ctx, clock: clock, database: database, repositories: repositories, student: student, goal: goal,
		curriculum: curriculum, instance: instance,
		progression: application.NewProgressionService(graph, profiles, mastery, database, application.WithProgressionClock(clock.Now)),
		mistakes: application.NewMistakeMemoryService(profiles, database, application.WithMistakeMemoryClock(clock.Now),
			application.WithMistakeMemoryIDGenerator(func(kind string) (learning.ID, error) {
				mistakeIndex++
				return mustE2EID(t, fmt.Sprintf("%s.student-core-e2e.%d", kind, mistakeIndex)), nil
			})),
		retention: application.NewRetentionService(profiles, database, application.WithRetentionClock(clock.Now)),
		reviews:   application.NewReviewSchedulerService(profiles, database, application.WithReviewSchedulerClock(clock.Now)),
		warmUps:   application.NewWarmUpSelectorService(profiles, database, application.WithWarmUpSelectorClock(clock.Now)),
		sessions: application.NewStudySessionLifecycleService(profiles, database, application.WithStudySessionClock(clock.Now),
			application.WithStudySessionIDGenerator(func() (learning.ID, error) {
				sessionIndex++
				return mustE2EID(t, fmt.Sprintf("session.student-core-e2e.%d", sessionIndex)), nil
			})),
		history:   application.NewStudyHistoryService(profiles, database, application.WithStudyHistoryClock(clock.Now)),
		streaks:   application.NewStreakService(profiles, database, application.WithStreakClock(clock.Now)),
		dailyPlan: application.NewAdaptiveDailyPlanService(profiles, mastery, database, application.WithAdaptiveDailyPlanClock(clock.Now)),
	}
	return harness
}

func (harness *studentCoreHarness) recordMastery(t *testing.T, concept, suffix string) application.ProgressionUpdate {
	t.Helper()
	evidence, err := learning.NewEvidenceWithMetadata(
		harness.id(t, "evidence."+suffix), harness.student.ID, harness.id(t, concept), learning.EvidenceAssessment,
		"fixture/student-core-lifecycle/v1/"+suffix, harness.score(t, .95), learning.EvidenceMetadata{
			Confidence: 1, Independence: 1, Difficulty: .5, AlgorithmVersion: "student-core-lifecycle-evidence/v1",
		}, harness.timestamp(t),
	)
	if err != nil {
		t.Fatal(err)
	}
	update, err := harness.progression.RecordEvidence(harness.ctx, harness.instance.ID, evidence, nil)
	if err != nil {
		t.Fatal(err)
	}
	return update
}

func (harness *studentCoreHarness) recordMistake(t *testing.T, concept string) application.MistakeRecordResult {
	t.Helper()
	result, err := harness.mistakes.Record(harness.ctx, application.RecordMistakeInput{
		ConceptID: harness.id(t, concept), Key: learning.MistakeKey("repeated-conceptual-pattern"),
		Category: learning.MistakeConceptual, Summary: "Confuses the stable fixture relationship",
		ObservedAt: harness.timestamp(t), SourceRef: "fixture/student-core-lifecycle/v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func (harness *studentCoreHarness) id(t *testing.T, value string) learning.ID {
	t.Helper()
	return mustE2EID(t, value)
}

func (harness *studentCoreHarness) score(t *testing.T, value float64) learning.MasteryScore {
	t.Helper()
	score, err := learning.NewMasteryScore(value)
	if err != nil {
		t.Fatal(err)
	}
	return score
}

func (harness *studentCoreHarness) timestamp(t *testing.T) learning.Timestamp {
	t.Helper()
	timestamp, err := learning.NewTimestamp(harness.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}

func completeSetupWithDiagnostic(t *testing.T, ctx context.Context, service application.LearnerSetupService) (application.LearnerSetupView, learning.ID) {
	t.Helper()
	if _, err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	for _, answer := range []string{
		"Ada", "Ratios", "Mathematics", "Reason proportionally", "beginner", "novice",
		"30", "5", "theory_first", "0.80", "yes", "",
	} {
		if _, err := service.SubmitOnboarding(ctx, answer); err != nil {
			t.Fatal(err)
		}
	}
	view, err := service.Confirm(ctx)
	if err != nil || view.Setup.DiagnosticAttemptID == nil {
		t.Fatalf("confirm diagnostic setup = (%+v, %v)", view, err)
	}
	attemptID := *view.Setup.DiagnosticAttemptID
	for _, answers := range [][]string{{"multiplicative"}, {"three to two"}, {"4:6", "6:9"}, {"confident"}} {
		view, err = service.SubmitDiagnostic(ctx, answers)
		if err != nil {
			t.Fatal(err)
		}
	}
	return view, attemptID
}

func completeSetupWithoutDiagnostic(t *testing.T, workspace string) {
	t.Helper()
	ctx := context.Background()
	store, err := learningdb.NewFactory("student-core-e2e").Open(ctx, workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close setup store: %v", err)
		}
	}()
	service := store.Setup()
	if _, err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	for _, answer := range []string{
		"Ada", "Ratios", "Mathematics", "Reason proportionally", "beginner", "novice",
		"60", "5", "theory_first", "0.80", "no", "",
	} {
		if _, err := service.SubmitOnboarding(ctx, answer); err != nil {
			t.Fatal(err)
		}
	}
	view, err := service.Confirm(ctx)
	if err != nil || view.Setup.Status != learning.SetupCompleted {
		t.Fatalf("complete setup without diagnostic = (%+v, %v)", view, err)
	}
}

func loadLifecycleCurriculum(t *testing.T, root string) learning.Curriculum {
	t.Helper()
	path := filepath.Join(root, "tests", "e2e", "testdata", "student-core-lifecycle-v1.yaml")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	curriculum, err := curriculumyaml.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	return curriculum
}

func dependentByID(t *testing.T, dependents []application.DependentProgression, id learning.ID) application.DependentProgression {
	t.Helper()
	for _, dependent := range dependents {
		if dependent.Decision.ConceptID == id {
			return dependent
		}
	}
	t.Fatalf("dependent %s not found in %+v", id, dependents)
	return application.DependentProgression{}
}

func mustE2EID(t *testing.T, value string) learning.ID {
	t.Helper()
	id, err := learning.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func curriculumConceptCount(curriculum learning.Curriculum) int {
	count := 0
	for _, node := range curriculum.Nodes {
		if node.Type == learning.CurriculumNodeConcept {
			count++
		}
	}
	return count
}
