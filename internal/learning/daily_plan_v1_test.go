package learning

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDailyPlanV1StartsBrandNewStudentWithFirstEligibleConcept(t *testing.T) {
	t.Parallel()
	input := dailyPlanInput(t, 45)
	plan, err := BuildAdaptiveDailyPlanV1(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != DailyPlanReady || plan.BufferMinutes != 5 || plan.PlannedMinutes != 25 || len(plan.Items) != 1 ||
		plan.Items[0].Role != DailyPlanRoleNewLearning || plan.Items[0].ConceptIDs[0] != dailyPlanID(t, "concept.a") ||
		plan.Items[0].Reason != DailyPlanNextEligibleConcept || !strings.Contains(plan.Items[0].Explanation, "prerequisites are satisfied") {
		t.Fatalf("brand-new plan = %+v", plan)
	}
	if plan.PlannedMinutes+plan.BufferMinutes > plan.AvailableMinutes || plan.PolicyVersion != DailyPlanPolicyVersion {
		t.Fatalf("brand-new budget/version = %+v", plan)
	}
}

func TestDailyPlanV1CanProduceReviewOnlyDayWithCriticalWarmUpFirst(t *testing.T) {
	t.Parallel()
	input := dailyPlanInput(t, 20)
	input.ReviewsDue = []ReviewItem{
		dailyPlanReview(t, input, "concept.b", "review.standard", ReviewStandard, false, 2*24*time.Hour),
		dailyPlanReview(t, input, "concept.a", "review.critical", ReviewDeep, true, 10*24*time.Hour),
	}
	input.Retention = []RetentionState{
		dailyPlanRetention(t, input, "concept.a", 10*24*time.Hour, 24*time.Hour, .3),
		dailyPlanRetention(t, input, "concept.b", 2*24*time.Hour, 7*24*time.Hour, .6),
	}
	plan, err := BuildAdaptiveDailyPlanV1(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != DailyPlanReviewOnly || plan.PlannedMinutes != 15 || len(plan.Items) != 2 ||
		plan.Items[0].Role != DailyPlanRoleWarmUp || plan.Items[0].ConceptIDs[0] != dailyPlanID(t, "concept.a") ||
		plan.Items[1].Role != DailyPlanRoleReview || plan.Items[1].ConceptIDs[0] != dailyPlanID(t, "concept.b") {
		t.Fatalf("review-only plan = %+v", plan)
	}
}

func TestDailyPlanV1ReinforcesBlockingWeaknessWithoutIntroducingNextConcept(t *testing.T) {
	t.Parallel()
	input := dailyPlanInput(t, 45)
	input.States = []InstanceConceptState{dailyPlanState(t, input, "concept.a", ExposureLearning, .4)}
	plan, err := BuildAdaptiveDailyPlanV1(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != DailyPlanReviewOnly || len(plan.Items) != 1 || plan.Items[0].Role != DailyPlanRoleReinforcement ||
		plan.Items[0].Reason != DailyPlanBlockingWeakness || plan.Items[0].ConceptIDs[0] != dailyPlanID(t, "concept.a") {
		t.Fatalf("blocked plan = %+v", plan)
	}
	for _, item := range plan.Items {
		if item.Role == DailyPlanRoleNewLearning {
			t.Fatalf("blocked plan introduced %+v", item)
		}
	}
}

func TestDailyPlanV1ReturnsNothingUrgentWhenCurrentContentIsMastered(t *testing.T) {
	t.Parallel()
	input := dailyPlanInput(t, 45)
	for _, suffix := range []string{"concept.a", "concept.b", "concept.c"} {
		input.States = append(input.States, dailyPlanState(t, input, suffix, ExposureMastered, .9))
	}
	plan, err := BuildAdaptiveDailyPlanV1(input)
	if err != nil || plan.Status != DailyPlanNothingUrgent || len(plan.Items) != 0 || plan.PlannedMinutes != 0 {
		t.Fatalf("mastered plan = (%+v, %v)", plan, err)
	}
}

func TestDailyPlanV1HonorsTinyBudgetWithoutAggressiveOverflow(t *testing.T) {
	t.Parallel()
	input := dailyPlanInput(t, 5)
	plan, err := BuildAdaptiveDailyPlanV1(input)
	if err != nil || plan.Status != DailyPlanTimeLimited || plan.PlannedMinutes != 0 || plan.BufferMinutes != 0 || len(plan.Items) != 0 {
		t.Fatalf("tiny plan = (%+v, %v)", plan, err)
	}
	if plan.PlannedMinutes+plan.BufferMinutes > plan.AvailableMinutes {
		t.Fatalf("tiny plan exceeded budget: %+v", plan)
	}
}

func TestDailyPlanV1IsDeterministicAcrossFactOrdering(t *testing.T) {
	t.Parallel()
	input := dailyPlanInput(t, 45)
	input.States = []InstanceConceptState{
		dailyPlanState(t, input, "concept.b", ExposureMastered, .9),
		dailyPlanState(t, input, "concept.a", ExposureMastered, .9),
	}
	input.ReviewsDue = []ReviewItem{
		dailyPlanReview(t, input, "concept.b", "review.b", ReviewQuickRecall, false, 24*time.Hour),
		dailyPlanReview(t, input, "concept.a", "review.a", ReviewQuickRecall, false, 24*time.Hour),
	}
	first, err := BuildAdaptiveDailyPlanV1(input)
	if err != nil {
		t.Fatal(err)
	}
	reversed := input
	reversed.Concepts = reverseDailyPlanConcepts(input.Concepts)
	reversed.States = []InstanceConceptState{input.States[1], input.States[0]}
	reversed.ReviewsDue = []ReviewItem{input.ReviewsDue[1], input.ReviewsDue[0]}
	second, err := BuildAdaptiveDailyPlanV1(reversed)
	if err != nil || !reflect.DeepEqual(second, first) {
		t.Fatalf("reordered plan = (%+v, %v), want %+v", second, err, first)
	}
}

func dailyPlanInput(t *testing.T, available int) DailyPlanInput {
	t.Helper()
	generatedAt := dailyPlanTimestamp(t, time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC))
	return DailyPlanInput{
		StudentID: dailyPlanID(t, "student.daily"), GoalID: dailyPlanID(t, "goal.daily"),
		CurriculumInstanceID: dailyPlanID(t, "instance.daily"),
		Curriculum:           CurriculumRef{ID: dailyPlanID(t, "curriculum.daily"), Version: "1.0.0"},
		Timezone:             "UTC", Date: dailyPlanTimestamp(t, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)),
		GeneratedAt: generatedAt, AvailableMinutes: available, MasteryPolicy: progressionThreshold(t, .8),
		Concepts: []DailyPlanCurriculumConcept{
			{ConceptID: dailyPlanID(t, "concept.a"), Sequence: 0},
			{ConceptID: dailyPlanID(t, "concept.b"), Sequence: 1, PrerequisiteIDs: []ID{dailyPlanID(t, "concept.a")}},
			{ConceptID: dailyPlanID(t, "concept.c"), Sequence: 2, PrerequisiteIDs: []ID{dailyPlanID(t, "concept.b")}},
		},
		GenerationReason: DailyPlanGeneratedInitial, Policy: DefaultDailyPlanPolicy(),
	}
}

func dailyPlanState(t *testing.T, input DailyPlanInput, concept string, exposure ExposureState, mastery float64) InstanceConceptState {
	t.Helper()
	seenAt := dailyPlanTimestamp(t, input.GeneratedAt.Time().Add(-7*24*time.Hour))
	state := InstanceConceptState{
		CurriculumInstanceID: input.CurriculumInstanceID, StudentID: input.StudentID,
		ConceptID: dailyPlanID(t, concept), Exposure: exposure, Mastery: mustScore(t, mastery),
		FirstSeenAt: &seenAt, LastSeenAt: &seenAt, UpdatedAt: seenAt,
	}
	if exposure == ExposureMastered || exposure == ExposureReviewDue {
		state.MasteredAt = &seenAt
	}
	if exposure == ExposureReviewDue {
		dueAt := dailyPlanTimestamp(t, input.GeneratedAt.Time().Add(-time.Hour))
		state.ReviewDueAt, state.UpdatedAt = &dueAt, dueAt
	}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
	return state
}

func dailyPlanReview(t *testing.T, input DailyPlanInput, concept, id string, reviewType ReviewType, critical bool, overdueBy time.Duration) ReviewItem {
	t.Helper()
	dueAt := dailyPlanTimestamp(t, input.GeneratedAt.Time().Add(-overdueBy))
	createdAt := dailyPlanTimestamp(t, dueAt.Time().Add(-24*time.Hour))
	return ReviewItem{
		ID: dailyPlanID(t, id), StudentID: input.StudentID, ConceptID: dailyPlanID(t, concept), DueAt: dueAt,
		Type: reviewType, EstimatedMinutes: reviewType.EstimatedMinutes(), CriticalPrerequisite: critical,
		Status: ReviewPending, CreatedAt: createdAt, AlgorithmVersion: ReviewSchedulerVersion,
	}
}

func dailyPlanRetention(t *testing.T, input DailyPlanInput, concept string, dueAgo, stability time.Duration, strength float64) RetentionState {
	t.Helper()
	dueAt := dailyPlanTimestamp(t, input.GeneratedAt.Time().Add(-dueAgo))
	practiceAt := dailyPlanTimestamp(t, dueAt.Time().Add(-stability))
	state := RetentionState{
		StudentID: input.StudentID, ConceptID: dailyPlanID(t, concept), LastPractice: &practiceAt,
		StabilityEstimate: stability, Strength: mustScore(t, strength), Status: RetentionDue,
		NextDueAt: &dueAt, MeasuredAt: dueAt, AlgorithmVersion: RetentionAlgorithmVersion,
	}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
	return state
}

func reverseDailyPlanConcepts(source []DailyPlanCurriculumConcept) []DailyPlanCurriculumConcept {
	result := append([]DailyPlanCurriculumConcept(nil), source...)
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

func dailyPlanID(t *testing.T, value string) ID {
	t.Helper()
	id, err := NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func dailyPlanTimestamp(t *testing.T, value time.Time) Timestamp {
	t.Helper()
	timestamp, err := NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}
