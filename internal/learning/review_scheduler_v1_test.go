package learning

import (
	"reflect"
	"testing"
	"time"
)

func TestReviewSchedulerV1SchedulesIntroducedConceptAndPreservesPostpone(t *testing.T) {
	t.Parallel()
	now := retentionTimestamp(t, 10*24*time.Hour)
	studentID := mustID(t, "student.scheduler")
	conceptID := mustID(t, "concept.scheduler")
	concept := schedulerConceptState(t, studentID, conceptID, retentionTimestamp(t, 0), now, .9)
	retention := schedulerRetentionState(t, studentID, conceptID, now, RetentionDue, .85, 2*24*time.Hour)

	schedule, ok, err := ScheduleReviewV1(ReviewSchedulingInput{Concept: concept, Retention: retention, CriticalPrerequisite: true, ScheduledAt: now})
	if err != nil || !ok || schedule.Type != ReviewQuickRecall || schedule.EstimatedMinutes != 5 || !schedule.CriticalPrerequisite ||
		schedule.AlgorithmVersion != ReviewSchedulerVersion || *schedule.IntroducedAt != *concept.IntroducedAt {
		t.Fatalf("ScheduleReviewV1() = (%+v, %v, %v)", schedule, ok, err)
	}
	id, _ := NewReviewItemIDV1(studentID, conceptID, schedule.DueAt, 0)
	item, _ := NewReviewItemV1(id, schedule, now)
	postponedUntil, _ := NewTimestamp(now.Time().Add(3 * 24 * time.Hour))
	postponed, err := item.Postpone(postponedUntil, now)
	if err != nil {
		t.Fatal(err)
	}
	schedule, ok, err = ScheduleReviewV1(ReviewSchedulingInput{Concept: concept, Retention: retention, History: []ReviewItem{postponed}, ScheduledAt: now})
	if err != nil || !ok || schedule.DueAt != postponedUntil {
		t.Fatalf("postponed schedule = (%+v, %v, %v)", schedule, ok, err)
	}
}

func TestReviewSchedulerV1FailureChoosesDeepReviewAndUnknownDoesNotSchedule(t *testing.T) {
	t.Parallel()
	now := retentionTimestamp(t, 10*24*time.Hour)
	studentID := mustID(t, "student.scheduler")
	conceptID := mustID(t, "concept.scheduler")
	concept := schedulerConceptState(t, studentID, conceptID, retentionTimestamp(t, 0), now, .8)
	retention := schedulerRetentionState(t, studentID, conceptID, now, RetentionDue, .75, 2*24*time.Hour)
	baseSchedule, _ := NewReviewScheduleV1(studentID, conceptID, *concept.IntroducedAt, *retention.NextDueAt, ReviewStandard, false, now)
	id, _ := NewReviewItemIDV1(studentID, conceptID, baseSchedule.DueAt, 0)
	item, _ := NewReviewItemV1(id, baseSchedule, retentionTimestamp(t, 5*24*time.Hour))
	failed, err := item.Complete(mustScore(t, .2), retentionTimestamp(t, 6*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	schedule, ok, err := ScheduleReviewV1(ReviewSchedulingInput{Concept: concept, Retention: retention, History: []ReviewItem{failed}, ScheduledAt: now})
	if err != nil || !ok || schedule.Type != ReviewDeep || schedule.EstimatedMinutes != 20 {
		t.Fatalf("failure schedule = (%+v, %v, %v)", schedule, ok, err)
	}
	unknown := RetentionState{StudentID: studentID, ConceptID: conceptID, Status: RetentionUnknown, MeasuredAt: now, AlgorithmVersion: RetentionAlgorithmVersion}
	if _, ok, err := ScheduleReviewV1(ReviewSchedulingInput{Concept: concept, Retention: unknown, ScheduledAt: now}); err != nil || ok {
		t.Fatalf("unknown schedule = (%v, %v), want none", ok, err)
	}
}

func TestReviewSchedulerV1RejectsDuplicatePendingHistory(t *testing.T) {
	t.Parallel()
	now := retentionTimestamp(t, 10*24*time.Hour)
	studentID := mustID(t, "student.scheduler")
	conceptID := mustID(t, "concept.scheduler")
	concept := schedulerConceptState(t, studentID, conceptID, retentionTimestamp(t, 0), now, .8)
	retention := schedulerRetentionState(t, studentID, conceptID, now, RetentionDue, .7, 2*24*time.Hour)
	schedule, _ := NewReviewScheduleV1(studentID, conceptID, *concept.IntroducedAt, *retention.NextDueAt, ReviewStandard, false, now)
	firstID, _ := NewReviewItemIDV1(studentID, conceptID, schedule.DueAt, 0)
	secondID, _ := NewReviewItemIDV1(studentID, conceptID, schedule.DueAt, 1)
	first, _ := NewReviewItemV1(firstID, schedule, now)
	second, _ := NewReviewItemV1(secondID, schedule, now)
	if _, _, err := ScheduleReviewV1(ReviewSchedulingInput{Concept: concept, Retention: retention, History: []ReviewItem{first, second}, ScheduledAt: now}); err == nil {
		t.Fatal("ScheduleReviewV1 accepted duplicate pending history")
	}
}

func TestReviewQueueV1SortsDueByOverdueWeaknessCriticalAndStableKeys(t *testing.T) {
	t.Parallel()
	now := retentionTimestamp(t, 10*24*time.Hour)
	studentID := mustID(t, "student.scheduler")
	candidates := []ReviewQueueCandidate{
		schedulerQueueCandidate(t, studentID, "stable", now, RetentionDue, .8, false, ReviewQuickRecall),
		schedulerQueueCandidate(t, studentID, "weak", now, RetentionDue, .2, false, ReviewQuickRecall),
		schedulerQueueCandidate(t, studentID, "critical", now, RetentionDue, .2, true, ReviewQuickRecall),
		schedulerQueueCandidate(t, studentID, "overdue", now, RetentionOverdue, .9, false, ReviewQuickRecall),
	}
	queue, err := BuildDueReviewQueueV1(Availability{DailyMinutes: 30, WeeklyDaysTarget: 5}, candidates, now)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(queue.Items))
	for index, item := range queue.Items {
		got[index] = item.Item.ConceptID.String()
	}
	want := []string{mustID(t, "concept.overdue").String(), mustID(t, "concept.critical").String(),
		mustID(t, "concept.weak").String(), mustID(t, "concept.stable").String()}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("due order = %v, want %v", got, want)
	}
}

func TestReviewQueueV1RespectsDailyBudgetAndDefersItemsThatDoNotFit(t *testing.T) {
	t.Parallel()
	now := retentionTimestamp(t, 10*24*time.Hour)
	studentID := mustID(t, "student.scheduler")
	candidates := []ReviewQueueCandidate{
		schedulerQueueCandidate(t, studentID, "deep", now, RetentionOverdue, .1, false, ReviewDeep),
		schedulerQueueCandidate(t, studentID, "standard", now, RetentionDue, .5, false, ReviewStandard),
		schedulerQueueCandidate(t, studentID, "quick", now, RetentionDue, .9, false, ReviewQuickRecall),
	}
	queue, err := BuildDueReviewQueueV1(Availability{DailyMinutes: 15, WeeklyDaysTarget: 5}, candidates, now)
	if err != nil || queue.UsedMinutes != 15 || queue.TotalDueMinutes != 35 || len(queue.Items) != 2 || len(queue.Deferred) != 1 ||
		queue.Deferred[0].Item.ConceptID != mustID(t, "concept.deep") {
		t.Fatalf("budgeted queue = (%+v, %v)", queue, err)
	}
}

func TestReviewItemV1PostponeSkipAndOutcomeSemantics(t *testing.T) {
	t.Parallel()
	now := retentionTimestamp(t, 10*24*time.Hour)
	studentID := mustID(t, "student.scheduler")
	conceptID := mustID(t, "concept.scheduler")
	schedule, _ := NewReviewScheduleV1(studentID, conceptID, retentionTimestamp(t, 0), retentionTimestamp(t, 11*24*time.Hour), ReviewStandard, false, now)
	id, _ := NewReviewItemIDV1(studentID, conceptID, schedule.DueAt, 0)
	item, _ := NewReviewItemV1(id, schedule, now)
	postponedDue := retentionTimestamp(t, 12*24*time.Hour)
	postponed, err := item.Postpone(postponedDue, now)
	if err != nil || postponed.Status != ReviewPending || postponed.PostponeCount != 1 {
		t.Fatalf("Postpone() = (%+v, %v)", postponed, err)
	}
	skipped, err := item.Skip(now)
	if err != nil || skipped.Status != ReviewSkipped || skipped.Outcome != ReviewOutcomeNone || skipped.Score != nil {
		t.Fatalf("Skip() = (%+v, %v)", skipped, err)
	}
	succeeded, err := item.Complete(mustScore(t, .7), now)
	if err != nil || succeeded.Outcome != ReviewOutcomeSuccess {
		t.Fatalf("Complete(success) = (%+v, %v)", succeeded, err)
	}
	failed, err := item.Complete(mustScore(t, .69), now)
	if err != nil || failed.Outcome != ReviewOutcomeFailure {
		t.Fatalf("Complete(failure) = (%+v, %v)", failed, err)
	}
}

func schedulerConceptState(t *testing.T, studentID, conceptID ID, introducedAt, updatedAt Timestamp, mastery float64) ConceptState {
	t.Helper()
	state := ConceptState{StudentID: studentID, ConceptID: conceptID, Exposure: ExposureMastered,
		Mastery: mustScore(t, mastery), IntroducedAt: &introducedAt, UpdatedAt: updatedAt}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
	return state
}

func schedulerRetentionState(t *testing.T, studentID, conceptID ID, measuredAt Timestamp, status RetentionStatus, strength float64, stability time.Duration) RetentionState {
	t.Helper()
	var lastPractice Timestamp
	if status == RetentionOverdue {
		lastPractice, _ = NewTimestamp(measuredAt.Time().Add(-2*stability - time.Second))
	} else {
		lastPractice, _ = NewTimestamp(measuredAt.Time().Add(-stability))
	}
	dueAt, _ := NewTimestamp(lastPractice.Time().Add(stability))
	state := RetentionState{StudentID: studentID, ConceptID: conceptID, LastPractice: &lastPractice,
		StabilityEstimate: stability, Strength: mustScore(t, strength), Status: status, NextDueAt: &dueAt,
		MeasuredAt: measuredAt, AlgorithmVersion: RetentionAlgorithmVersion}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
	return state
}

func schedulerQueueCandidate(t *testing.T, studentID ID, suffix string, now Timestamp, status RetentionStatus, strength float64, critical bool, reviewType ReviewType) ReviewQueueCandidate {
	t.Helper()
	conceptID := mustID(t, "concept."+suffix)
	retention := schedulerRetentionState(t, studentID, conceptID, now, status, strength, 24*time.Hour)
	schedule, err := NewReviewScheduleV1(studentID, conceptID, retentionTimestamp(t, 0), *retention.NextDueAt, reviewType, critical, now)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := NewReviewItemIDV1(studentID, conceptID, schedule.DueAt, 0)
	item, err := NewReviewItemV1(id, schedule, retentionTimestamp(t, 5*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return ReviewQueueCandidate{Item: item, Retention: retention}
}
