package learning

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestLearningAnalyticsV1ExcludesUnknownAndExplainsEveryMetric(t *testing.T) {
	t.Parallel()
	studentID := streakID(t, "student.analytics")
	asOfTime := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	asOf := streakTimestamp(t, asOfTime)
	seen := streakTimestamp(t, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))
	masteredOld := streakTimestamp(t, time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC))
	masteredRecent := streakTimestamp(t, time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC))
	states := []InstanceConceptState{
		analyticsState(t, studentID, "unknown", ExposureNotSeen, 0, seen, nil, nil),
		analyticsState(t, studentID, "introduced", ExposureIntroduced, .2, seen, nil, nil),
		analyticsState(t, studentID, "learning", ExposureLearning, .5, seen, nil, nil),
		analyticsState(t, studentID, "practicing", ExposurePracticing, .7, seen, nil, nil),
		analyticsState(t, studentID, "mastered", ExposureMastered, .9, seen, &masteredRecent, nil),
		analyticsState(t, studentID, "review-due", ExposureReviewDue, .8, seen, &masteredOld, &asOf),
	}
	sessions := []StudySession{
		streakSession(t, studentID, "analytics.today", time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC), 30*time.Minute),
		streakSession(t, studentID, "analytics.week", time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC), 60*time.Minute),
		streakSession(t, studentID, "analytics.month", time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC), 90*time.Minute),
		streakSession(t, studentID, "analytics.old", time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC), 120*time.Minute),
	}
	reviews := []ReviewItem{
		analyticsPendingReview(t, studentID, "due", asOfTime.Add(-time.Hour), asOfTime.Add(-24*time.Hour)),
		analyticsPendingReview(t, studentID, "future", asOfTime.Add(24*time.Hour), asOfTime.Add(-24*time.Hour)),
	}
	retention := []RetentionState{
		analyticsRetention(t, studentID, "fresh", asOfTime.Add(24*time.Hour), 4*24*time.Hour, asOfTime, RetentionFresh),
		analyticsRetention(t, studentID, "due", asOfTime.Add(-24*time.Hour), 4*24*time.Hour, asOfTime.Add(-24*time.Hour), RetentionDue),
		analyticsRetention(t, studentID, "overdue", asOfTime.Add(-10*24*time.Hour), 4*24*time.Hour, asOfTime.Add(-10*24*time.Hour), RetentionDue),
	}
	policy := DefaultLearningAnalyticsPolicy()
	policy.RankingLimit = 3
	input := LearningAnalyticsInput{
		StudentID: studentID, Timezone: "UTC", AsOf: asOf, ConceptStates: states,
		RetentionStates: retention, Reviews: reviews, Sessions: sessions, Policy: policy,
	}

	snapshot, err := CalculateLearningAnalyticsV1(input)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Progress.ConceptsIntroduced.Value != 5 || snapshot.Progress.ConceptsLearning.Value != 3 ||
		snapshot.Progress.ConceptsMastered.Value != 2 || snapshot.Progress.ReviewsDue.Value != 1 {
		t.Fatalf("progress = %+v", snapshot.Progress)
	}
	if snapshot.Mastery.AverageKnown.Value == nil || mathDifference(snapshot.Mastery.AverageKnown.Value.Value(), .62) > 1e-9 {
		t.Fatalf("known mastery average = %+v, want .62", snapshot.Mastery.AverageKnown.Value)
	}
	if got := analyticsConceptIDs(snapshot.Mastery.Strongest.Concepts); !reflect.DeepEqual(got, []string{"concept.analytics.mastered", "concept.analytics.review-due", "concept.analytics.practicing"}) {
		t.Fatalf("strongest = %v", got)
	}
	if got := analyticsConceptIDs(snapshot.Mastery.Weakest.Concepts); !reflect.DeepEqual(got, []string{"concept.analytics.introduced", "concept.analytics.learning", "concept.analytics.practicing"}) {
		t.Fatalf("weakest = %v", got)
	}
	if snapshot.Time.Today.Value != 30*time.Minute || snapshot.Time.Week.Value != 90*time.Minute ||
		snapshot.Time.Month.Value != 180*time.Minute || snapshot.Time.Total.Value != 300*time.Minute {
		t.Fatalf("time = %+v", snapshot.Time)
	}
	if snapshot.Retention.Fresh.Value != 1 || snapshot.Retention.Due.Value != 1 || snapshot.Retention.Overdue.Value != 1 {
		t.Fatalf("retention = %+v", snapshot.Retention)
	}
	if snapshot.Activity.ActiveDays.Value != 4 || snapshot.Activity.CurrentStreak.Value != 1 || snapshot.Activity.LongestStreak.Value != 1 {
		t.Fatalf("activity = %+v", snapshot.Activity)
	}
	if snapshot.Pace.WindowStart.String() != "2026-07-25" || snapshot.Pace.WindowEndExclusive.String() != "2026-08-22" ||
		mathDifference(snapshot.Pace.ConceptsMasteredPerWeek.Value, .5) > 1e-9 ||
		mathDifference(snapshot.Pace.StudyMinutesPerWeek.Value, 45) > 1e-9 {
		t.Fatalf("pace = %+v", snapshot.Pace)
	}

	shuffled := input
	shuffled.ConceptStates = reverseAnalyticsStates(states)
	shuffled.Sessions = reverseAnalyticsSessions(sessions)
	shuffled.RetentionStates = []RetentionState{retention[2], retention[0], retention[1]}
	shuffled.Reviews = []ReviewItem{reviews[1], reviews[0]}
	again, err := CalculateLearningAnalyticsV1(shuffled)
	again.Mastery.AverageKnown.Value = snapshot.Mastery.AverageKnown.Value
	if err != nil || !reflect.DeepEqual(again, snapshot) {
		t.Fatalf("shuffled calculation = (%+v, %v), want deterministic %+v", again, err, snapshot)
	}
}

func TestLearningAnalyticsV1HandlesEmptyProfileAndLocalDateRanges(t *testing.T) {
	t.Parallel()
	studentID := streakID(t, "student.analytics.empty")
	asOfTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	input := LearningAnalyticsInput{
		StudentID: studentID, Timezone: "America/Lima", AsOf: streakTimestamp(t, asOfTime),
		Policy: DefaultLearningAnalyticsPolicy(),
	}
	empty, err := CalculateLearningAnalyticsV1(input)
	if err != nil || empty.Mastery.AverageKnown.Value != nil || len(empty.Mastery.Strongest.Concepts) != 0 ||
		empty.Progress.ConceptsIntroduced.Value != 0 || empty.Time.Total.Value != 0 {
		t.Fatalf("empty analytics = (%+v, %v)", empty, err)
	}

	input.Sessions = []StudySession{
		streakSession(t, studentID, "analytics.before-local-midnight", time.Date(2026, 8, 24, 4, 49, 0, 0, time.UTC), 10*time.Minute),
		streakSession(t, studentID, "analytics.at-local-midnight", time.Date(2026, 8, 24, 5, 0, 0, 0, time.UTC), 10*time.Minute),
	}
	local, err := CalculateLearningAnalyticsV1(input)
	if err != nil {
		t.Fatal(err)
	}
	if local.Time.Today.Value != 10*time.Minute || local.Time.Week.Value != 10*time.Minute ||
		local.Time.Month.Value != 20*time.Minute || local.Time.Total.Value != 20*time.Minute {
		t.Fatalf("local date windows = %+v", local.Time)
	}
}

func TestLearningAnalyticsV1SortsLargeFixtureDeterministically(t *testing.T) {
	studentID := streakID(t, "student.analytics.large")
	seen := streakTimestamp(t, time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))
	states := make([]InstanceConceptState, 5000)
	for index := range states {
		score := float64(index%101) / 100
		states[index] = analyticsState(t, studentID, fmt.Sprintf("large.%04d", 4999-index), ExposureLearning, score, seen, nil, nil)
	}
	snapshot, err := CalculateLearningAnalyticsV1(LearningAnalyticsInput{
		StudentID: studentID, Timezone: "UTC", AsOf: streakTimestamp(t, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)),
		ConceptStates: states, Policy: DefaultLearningAnalyticsPolicy(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Progress.ConceptsIntroduced.Value != 5000 || len(snapshot.Mastery.Strongest.Concepts) != 5 || len(snapshot.Mastery.Weakest.Concepts) != 5 {
		t.Fatalf("large analytics summary = progress %+v strongest=%d weakest=%d", snapshot.Progress, len(snapshot.Mastery.Strongest.Concepts), len(snapshot.Mastery.Weakest.Concepts))
	}
	if snapshot.Mastery.Strongest.Concepts[0].Mastery.Value() != 1 || snapshot.Mastery.Weakest.Concepts[0].Mastery.Value() != 0 {
		t.Fatalf("large rankings = strongest %+v weakest %+v", snapshot.Mastery.Strongest.Concepts, snapshot.Mastery.Weakest.Concepts)
	}
}

func analyticsState(t *testing.T, studentID ID, suffix string, exposure ExposureState, score float64, seen Timestamp, masteredAt, reviewDueAt *Timestamp) InstanceConceptState {
	t.Helper()
	state := InstanceConceptState{
		CurriculumInstanceID: streakID(t, "instance.analytics"), StudentID: studentID,
		ConceptID: streakID(t, "concept.analytics."+suffix), Exposure: exposure, Mastery: mustScore(t, score), UpdatedAt: seen,
	}
	if exposure != ExposureNotSeen {
		first, last := seen, seen
		state.FirstSeenAt, state.LastSeenAt = &first, &last
	}
	if masteredAt != nil {
		mastered, last := *masteredAt, *masteredAt
		state.MasteredAt, state.LastSeenAt, state.UpdatedAt = &mastered, &last, last
	}
	if reviewDueAt != nil {
		due := *reviewDueAt
		state.ReviewDueAt, state.UpdatedAt = &due, due
	}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
	return state
}

func analyticsPendingReview(t *testing.T, studentID ID, suffix string, dueAt, createdAt time.Time) ReviewItem {
	t.Helper()
	return ReviewItem{
		ID: streakID(t, "review.analytics."+suffix), StudentID: studentID,
		ConceptID: streakID(t, "concept.review.analytics."+suffix), DueAt: streakTimestamp(t, dueAt),
		Type: ReviewStandard, EstimatedMinutes: 10, Status: ReviewPending,
		CreatedAt: streakTimestamp(t, createdAt), AlgorithmVersion: LegacyReviewSchedulerVersion,
	}
}

func analyticsRetention(t *testing.T, studentID ID, suffix string, dueAt time.Time, stability time.Duration, measuredAt time.Time, status RetentionStatus) RetentionState {
	t.Helper()
	due := streakTimestamp(t, dueAt)
	practice := streakTimestamp(t, dueAt.Add(-stability))
	state := RetentionState{
		StudentID: studentID, ConceptID: streakID(t, "concept.retention.analytics."+suffix),
		LastPractice: &practice, StabilityEstimate: stability, Strength: mustScore(t, .8),
		Status: status, NextDueAt: &due, MeasuredAt: streakTimestamp(t, measuredAt), AlgorithmVersion: RetentionAlgorithmVersion,
	}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
	return state
}

func analyticsConceptIDs(items []AnalyticsConceptMastery) []string {
	result := make([]string, len(items))
	for index, item := range items {
		result[index] = item.ConceptID.String()
	}
	return result
}

func reverseAnalyticsStates(source []InstanceConceptState) []InstanceConceptState {
	result := append([]InstanceConceptState(nil), source...)
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

func reverseAnalyticsSessions(source []StudySession) []StudySession {
	result := append([]StudySession(nil), source...)
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

func mathDifference(left, right float64) float64 {
	if left < right {
		return right - left
	}
	return left - right
}
