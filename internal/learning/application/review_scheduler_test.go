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

func TestReviewSchedulerServiceCreatesOneIdempotentDueItem(t *testing.T) {
	t.Parallel()
	fixture := newReviewSchedulerFixture(t, 30, "concept.a")
	view, err := fixture.service.List(fixture.ctx, true)
	if err != nil || len(view.Items) != 1 || view.Pending != 1 || view.Items[0].Item.ConceptID != testID(t, "concept.a") ||
		!view.Items[0].Critical || view.AlgorithmVersion != learning.ReviewSchedulerVersion || view.Timezone != "UTC" {
		t.Fatalf("List(due) = (%+v, %v)", view, err)
	}
	again, err := fixture.service.List(fixture.ctx, true)
	if err != nil || len(again.Items) != 1 || again.Items[0].Item.ID != view.Items[0].Item.ID {
		t.Fatalf("idempotent List(due) = (%+v, %v)", again, err)
	}
	items, err := fixture.repositories.Reviews.ListByStudent(fixture.ctx, fixture.student.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("persisted items = (%+v, %v)", items, err)
	}
}

func TestReviewSchedulerServiceRespectsLimitedTimeAndPostpone(t *testing.T) {
	t.Parallel()
	fixture := newReviewSchedulerFixture(t, 15, "concept.a", "concept.b")
	view, err := fixture.service.List(fixture.ctx, true)
	if err != nil || view.UsedMinutes > 15 || view.TotalDueMinutes != 40 || len(view.Items) != 0 || len(view.Deferred) != 2 {
		t.Fatalf("limited queue = (%+v, %v)", view, err)
	}
	all, err := fixture.service.List(fixture.ctx, false)
	if err != nil || len(all.Items) != 2 {
		t.Fatalf("List(all) = (%+v, %v)", all, err)
	}
	until, _ := learning.NewTimestamp(fixture.now.Add(48 * time.Hour))
	postponed, err := fixture.service.Postpone(fixture.ctx, all.Items[0].Item.ID, until)
	if err != nil || postponed.PostponeCount != 1 || postponed.DueAt != until {
		t.Fatalf("Postpone() = (%+v, %v)", postponed, err)
	}
	all, err = fixture.service.List(fixture.ctx, false)
	if err != nil || all.Items[0].Item.DueAt == all.Items[1].Item.DueAt {
		t.Fatalf("postponement was not preserved: (%+v, %v)", all, err)
	}
}

func TestReviewSchedulerServiceSkipDoesNotRecordPassingEvidence(t *testing.T) {
	t.Parallel()
	fixture := newReviewSchedulerFixture(t, 30, "concept.a")
	view, err := fixture.service.List(fixture.ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	skipped, err := fixture.service.Skip(fixture.ctx, view.Items[0].Item.ID)
	if err != nil || skipped.Status != learning.ReviewSkipped || skipped.Score != nil || skipped.Outcome != learning.ReviewOutcomeNone {
		t.Fatalf("Skip() = (%+v, %v)", skipped, err)
	}
	evidence, err := fixture.repositories.Evidence.ListByConcept(fixture.ctx, fixture.student.ID, skipped.ConceptID)
	if err != nil || len(evidence) != 1 {
		t.Fatalf("skip evidence = (%+v, %v), want original only", evidence, err)
	}
	items, err := fixture.repositories.Reviews.ListByStudent(fixture.ctx, fixture.student.ID)
	if err != nil || len(items) != 2 || findReviewStatus(items, learning.ReviewPending) == nil {
		t.Fatalf("skip replacement = (%+v, %v)", items, err)
	}
}

func TestReviewSchedulerServiceSuccessIsIdempotentAndExtendsSchedule(t *testing.T) {
	t.Parallel()
	fixture := newReviewSchedulerFixture(t, 30, "concept.a")
	view, _ := fixture.service.List(fixture.ctx, false)
	item := view.Items[0].Item
	update, err := fixture.service.RecordOutcome(fixture.ctx, item.ID, testScore(t, .9))
	if err != nil || update.Completed.Outcome != learning.ReviewOutcomeSuccess || update.Next == nil ||
		update.Next.Type != learning.ReviewQuickRecall || !update.Next.DueAt.After(fixtureTimestamp(t, fixture.now)) {
		t.Fatalf("RecordOutcome(success) = (%+v, %v)", update, err)
	}
	retry, err := fixture.service.RecordOutcome(fixture.ctx, item.ID, testScore(t, .9))
	if err != nil || retry.Completed.ID != update.Completed.ID || retry.Next == nil || retry.Next.ID != update.Next.ID {
		t.Fatalf("idempotent outcome = (%+v, %v)", retry, err)
	}
	evidence, err := fixture.repositories.Evidence.ListByConcept(fixture.ctx, fixture.student.ID, item.ConceptID)
	if err != nil || len(evidence) != 2 || evidence[1].Type != learning.EvidenceReviewRecall {
		t.Fatalf("review evidence = (%+v, %v)", evidence, err)
	}
	if _, err := fixture.service.RecordOutcome(fixture.ctx, item.ID, testScore(t, .1)); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("different retry error = %v, want conflict", err)
	}
}

func TestReviewSchedulerServiceFailureCreatesShortDeepSchedule(t *testing.T) {
	t.Parallel()
	fixture := newReviewSchedulerFixture(t, 30, "concept.a")
	view, _ := fixture.service.List(fixture.ctx, false)
	update, err := fixture.service.RecordOutcome(fixture.ctx, view.Items[0].Item.ID, testScore(t, .1))
	if err != nil || update.Completed.Outcome != learning.ReviewOutcomeFailure || update.Next == nil ||
		update.Next.Type != learning.ReviewDeep || update.Retention.FailedReviews != 1 {
		t.Fatalf("RecordOutcome(failure) = (%+v, %v)", update, err)
	}
	if update.Next.DueAt.Time().Sub(fixture.now) > 48*time.Hour {
		t.Fatalf("failure next due = %s, want short interval", update.Next.DueAt.Time())
	}
}

type reviewSchedulerFixture struct {
	ctx          context.Context
	store        *memory.Store
	repositories application.Repositories
	student      learning.Student
	now          time.Time
	service      application.ReviewSchedulerService
}

func newReviewSchedulerFixture(t *testing.T, dailyMinutes int, conceptSuffixes ...string) reviewSchedulerFixture {
	t.Helper()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	createdAt := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	now := time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC)
	profiles := application.NewProfileService(application.NewStudentService(repositories.Students),
		application.WithProfileClock(func() time.Time { return createdAt }))
	student, err := profiles.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dailyMinutes != student.Profile.Availability.DailyMinutes {
		if student, err = profiles.Edit(ctx, application.ProfileChanges{DailyMinutes: &dailyMinutes}); err != nil {
			t.Fatal(err)
		}
	}
	goals := application.NewGoalLifecycleService(profiles, store, application.WithGoalClock(func() time.Time { return createdAt }),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.review-scheduler"), nil }))
	goal, err := goals.Set(ctx, goalInput(t, "Review concepts", "General knowledge"))
	if err != nil {
		t.Fatal(err)
	}
	curriculum := instanceTestCurriculum(t, "1.0.0")
	instances := application.NewCurriculumInstanceService(profiles, store,
		application.WithCurriculumInstanceClock(func() time.Time { return createdAt }),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.review-scheduler"), nil }))
	instance, err := instances.Create(ctx, goal.ID, curriculum, learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	observedAt := fixtureTimestamp(t, createdAt.Add(time.Hour))
	for _, suffix := range conceptSuffixes {
		conceptID := testID(t, suffix)
		evidence, err := learning.NewEvidenceWithMetadata(testID(t, "evidence.review-scheduler."+suffix), student.ID, conceptID,
			learning.EvidenceAssessment, "fixture/review-scheduler/"+suffix, testScore(t, .9), learning.EvidenceMetadata{
				Confidence: 1, Independence: 1, Difficulty: .5, AlgorithmVersion: "fixture/review-scheduler-v1",
			}, observedAt)
		if err != nil {
			t.Fatal(err)
		}
		if err := repositories.Evidence.Append(ctx, evidence); err != nil {
			t.Fatal(err)
		}
		state, err := instances.State(ctx, instance.ID, conceptID)
		if err != nil {
			t.Fatal(err)
		}
		state.Exposure, state.Mastery = learning.ExposureMastered, testScore(t, .9)
		state.FirstSeenAt, state.LastSeenAt, state.MasteredAt = &observedAt, &observedAt, &observedAt
		state.UpdatedAt = observedAt
		if err := instances.SaveState(ctx, state); err != nil {
			t.Fatal(err)
		}
	}
	service := application.NewReviewSchedulerService(profiles, store, application.WithReviewSchedulerClock(func() time.Time { return now }))
	return reviewSchedulerFixture{ctx: ctx, store: store, repositories: repositories, student: student, now: now, service: service}
}

func fixtureTimestamp(t *testing.T, value time.Time) learning.Timestamp {
	t.Helper()
	timestamp, err := learning.NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}

func findReviewStatus(items []learning.ReviewItem, status learning.ReviewStatus) *learning.ReviewItem {
	for index := range items {
		if items[index].Status == status {
			return &items[index]
		}
	}
	return nil
}
