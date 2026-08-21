package learning

import (
	"strings"
	"testing"
	"time"
)

func TestWarmUpSelectorCanReturnEmptyWithoutDueReviewsOrRepeatedMistakes(t *testing.T) {
	t.Parallel()
	input := warmUpInput(t, 30, "concept.a")
	plan, err := SelectWarmUpV1(input)
	if err != nil || len(plan.Items) != 0 || plan.BudgetMinutes != 10 || plan.UsedMinutes != 0 ||
		plan.AlgorithmVersion != WarmUpSelectorVersion {
		t.Fatalf("SelectWarmUpV1() = (%+v, %v)", plan, err)
	}
}

func TestWarmUpSelectorPrioritizesCriticalPrerequisiteDue(t *testing.T) {
	t.Parallel()
	input := warmUpInput(t, 15, "concept.z", "concept.a")
	input.Lesson.PrerequisiteConceptIDs = []ID{warmUpID(t, "concept.z")}
	input.DueReviews = []ReviewItem{
		warmUpReview(t, input, "concept.a", 2*time.Hour),
		warmUpReview(t, input, "concept.z", time.Hour),
	}
	plan, err := SelectWarmUpV1(input)
	if err != nil || len(plan.Items) != 1 || plan.Items[0].Concept.ID != warmUpID(t, "concept.z") ||
		plan.Items[0].Reason != WarmUpPrerequisiteReviewDue || plan.Items[0].Priority != 1 ||
		!strings.Contains(plan.Items[0].Explanation, "Prerequisite") {
		t.Fatalf("SelectWarmUpV1() = (%+v, %v)", plan, err)
	}
}

func TestWarmUpSelectorIncludesRepeatedMistakeWithoutDueReview(t *testing.T) {
	t.Parallel()
	input := warmUpInput(t, 30, "concept.a")
	input.Mistakes = []Mistake{warmUpMistake(t, input, "concept.a", 3)}
	plan, err := SelectWarmUpV1(input)
	if err != nil || len(plan.Items) != 1 || plan.Items[0].Reason != WarmUpRepeatedMistake ||
		!strings.Contains(plan.Items[0].Explanation, "repeated unresolved mistake") {
		t.Fatalf("SelectWarmUpV1() = (%+v, %v)", plan, err)
	}

	input.Mistakes[0].Occurrences = 1
	plan, err = SelectWarmUpV1(input)
	if err != nil || len(plan.Items) != 0 {
		t.Fatalf("single occurrence plan = (%+v, %v), want empty", plan, err)
	}
}

func TestWarmUpSelectorCapsBudgetAndPreservesNewContentTime(t *testing.T) {
	t.Parallel()
	input := warmUpInput(t, 60, "concept.a", "concept.b", "concept.c", "concept.d")
	for _, suffix := range []string{"concept.a", "concept.b", "concept.c", "concept.d"} {
		input.DueReviews = append(input.DueReviews, warmUpReview(t, input, suffix, time.Hour))
	}
	plan, err := SelectWarmUpV1(input)
	if err != nil || plan.BudgetMinutes != 15 || plan.UsedMinutes != 15 || len(plan.Items) != 3 ||
		plan.UsedMinutes >= plan.AvailableMinutes {
		t.Fatalf("SelectWarmUpV1(60) = (%+v, %v)", plan, err)
	}

	input.AvailableMinutes = 10
	plan, err = SelectWarmUpV1(input)
	if err != nil || plan.BudgetMinutes != 0 || len(plan.Items) != 0 {
		t.Fatalf("SelectWarmUpV1(10) = (%+v, %v), want empty", plan, err)
	}
}

func TestWarmUpSelectorRotatesRecentTiesThenUsesStableID(t *testing.T) {
	t.Parallel()
	input := warmUpInput(t, 15, "concept.c", "concept.a", "concept.b")
	for _, suffix := range []string{"concept.c", "concept.a", "concept.b"} {
		input.DueReviews = append(input.DueReviews, warmUpReview(t, input, suffix, time.Hour))
	}
	input.RecentConceptIDs = []ID{warmUpID(t, "concept.a")}
	plan, err := SelectWarmUpV1(input)
	if err != nil || len(plan.Items) != 1 || plan.Items[0].Concept.ID != warmUpID(t, "concept.b") {
		t.Fatalf("rotated selection = (%+v, %v), want concept.b", plan, err)
	}

	input.RecentConceptIDs = nil
	plan, err = SelectWarmUpV1(input)
	if err != nil || len(plan.Items) != 1 || plan.Items[0].Concept.ID != warmUpID(t, "concept.a") {
		t.Fatalf("stable tie selection = (%+v, %v), want concept.a", plan, err)
	}
}

func warmUpInput(t *testing.T, available int, conceptSuffixes ...string) WarmUpSelectionInput {
	t.Helper()
	studentID := warmUpID(t, "student.warm-up")
	topicID := warmUpID(t, "topic.warm-up")
	concepts := make([]Concept, 0, len(conceptSuffixes))
	for _, suffix := range conceptSuffixes {
		concepts = append(concepts, Concept{ID: warmUpID(t, suffix), TopicID: topicID, Title: suffix})
	}
	return WarmUpSelectionInput{
		StudentID: studentID,
		Lesson: WarmUpLessonCandidate{
			Curriculum: CurriculumRef{ID: warmUpID(t, "curriculum.warm-up"), Version: "1.0.0"},
			LessonID:   warmUpID(t, "lesson.warm-up"),
		},
		Concepts: concepts, AvailableMinutes: available,
		GeneratedAt: warmUpTimestamp(t, time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC)),
	}
}

func warmUpReview(t *testing.T, input WarmUpSelectionInput, conceptSuffix string, overdueBy time.Duration) ReviewItem {
	t.Helper()
	dueAt := warmUpTimestamp(t, input.GeneratedAt.Time().Add(-overdueBy))
	schedule, err := NewReviewScheduleV1(input.StudentID, warmUpID(t, conceptSuffix),
		warmUpTimestamp(t, dueAt.Time().Add(-24*time.Hour)), dueAt, ReviewQuickRecall, false, dueAt)
	if err != nil {
		t.Fatal(err)
	}
	item, err := NewReviewItemV1(warmUpID(t, "review."+conceptSuffix), schedule, dueAt)
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func warmUpMistake(t *testing.T, input WarmUpSelectionInput, conceptSuffix string, occurrences int) Mistake {
	t.Helper()
	observedAt := warmUpTimestamp(t, input.GeneratedAt.Time().Add(-time.Hour))
	mistake, err := NewMistake(warmUpID(t, "mistake."+conceptSuffix), input.StudentID, warmUpID(t, conceptSuffix),
		MistakeKey("pattern."+conceptSuffix), MistakeConceptual, "Repeated misunderstanding", observedAt, "fixture/warm-up")
	if err != nil {
		t.Fatal(err)
	}
	mistake.Occurrences = occurrences
	return mistake
}

func warmUpID(t *testing.T, value string) ID {
	t.Helper()
	id, err := NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func warmUpTimestamp(t *testing.T, value time.Time) Timestamp {
	t.Helper()
	timestamp, err := NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}
