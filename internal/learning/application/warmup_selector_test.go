package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestWarmUpSelectorServiceReadsDurableSignalsAndCapsRemainingTime(t *testing.T) {
	t.Parallel()
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

	reference := learning.CurriculumRef{ID: testID(t, "curriculum.warm-up-service"), Version: "1.0.0"}
	topicID := testID(t, "topic.warm-up-service")
	concepts := []learning.Concept{
		{ID: testID(t, "concept.a"), TopicID: topicID, Title: "A"},
		{ID: testID(t, "concept.b"), TopicID: topicID, Title: "B"},
		{ID: testID(t, "concept.c"), TopicID: topicID, Title: "C"},
	}
	if err := store.SeedCurriculum(reference, concepts, nil); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"concept.a", "concept.b"} {
		item := applicationWarmUpReview(t, student.ID, testID(t, suffix), now)
		if err := repositories.Reviews.CreateItem(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	mistake, err := learning.NewMistake(testID(t, "mistake.warm-up-service"), student.ID, testID(t, "concept.b"),
		learning.MistakeKey("pattern.warm-up-service"), learning.MistakeConceptual, "Repeated misunderstanding",
		fixtureTimestamp(t, now.Add(-time.Hour)), "fixture/warm-up-service")
	if err != nil {
		t.Fatal(err)
	}
	mistake.Occurrences = 3
	if err := repositories.Mistakes.Create(ctx, mistake); err != nil {
		t.Fatal(err)
	}

	service := application.NewWarmUpSelectorService(profiles, store,
		application.WithWarmUpSelectorClock(func() time.Time { return now }))
	plan, err := service.Select(ctx, application.WarmUpRequest{
		Lesson: learning.WarmUpLessonCandidate{
			Curriculum: reference, LessonID: testID(t, "lesson.warm-up-service"),
			PrerequisiteConceptIDs: []learning.ID{testID(t, "concept.b")},
		},
		AvailableMinutes: 60,
	})
	if err != nil || plan.GeneratedAt != fixtureTimestamp(t, now) ||
		plan.AvailableMinutes != student.Profile.Availability.DailyMinutes || plan.BudgetMinutes != 10 ||
		len(plan.Items) != 2 || plan.Items[0].Concept.ID != testID(t, "concept.b") ||
		plan.Items[0].Reason != learning.WarmUpPrerequisiteReviewDue || plan.Items[1].Concept.ID != testID(t, "concept.a") {
		t.Fatalf("Select() = (%+v, %v)", plan, err)
	}
	items, err := repositories.Reviews.ListByStudent(ctx, student.ID)
	if err != nil || len(items) != 2 {
		t.Fatalf("warm-up mutated review backlog: (%+v, %v)", items, err)
	}
	evidence, err := repositories.Evidence.ListByConcept(ctx, student.ID, testID(t, "concept.b"))
	if err != nil || len(evidence) != 0 {
		t.Fatalf("warm-up created evidence: (%+v, %v)", evidence, err)
	}
}

func applicationWarmUpReview(t *testing.T, studentID, conceptID learning.ID, now time.Time) learning.ReviewItem {
	t.Helper()
	dueAt := fixtureTimestamp(t, now.Add(-time.Hour))
	introducedAt := fixtureTimestamp(t, now.Add(-48*time.Hour))
	schedule, err := learning.NewReviewScheduleV1(studentID, conceptID, introducedAt, dueAt,
		learning.ReviewQuickRecall, false, dueAt)
	if err != nil {
		t.Fatal(err)
	}
	itemID, err := learning.NewID("review.warm-up-service." + conceptID.String())
	if err != nil {
		t.Fatal(err)
	}
	item, err := learning.NewReviewItemV1(itemID, schedule, introducedAt)
	if err != nil {
		t.Fatal(err)
	}
	return item
}
