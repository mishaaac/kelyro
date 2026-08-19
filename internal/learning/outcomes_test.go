package learning

import "testing"

func TestRetentionReviewAndOutcomeValueObjectsValidate(t *testing.T) {
	t.Parallel()

	studentID := mustID(t, "student.ada")
	conceptID := mustID(t, "concept.arithmetic-mean")
	now := mustTimestamp(t, 12)
	retention := RetentionState{
		StudentID: studentID, ConceptID: conceptID,
		Strength: mustScore(t, 0.65), MeasuredAt: now,
	}
	if err := retention.Validate(); err != nil {
		t.Fatalf("RetentionState.Validate() error = %v", err)
	}

	completedAt := mustTimestamp(t, 13)
	review := ReviewItem{
		ID: mustID(t, "review.001"), StudentID: studentID, ConceptID: conceptID,
		DueAt: now, Status: ReviewCompleted, CompletedAt: &completedAt,
	}
	if err := review.Validate(); err != nil {
		t.Fatalf("ReviewItem.Validate() error = %v", err)
	}
	review.CompletedAt = nil
	if err := review.Validate(); err == nil {
		t.Fatal("ReviewItem.Validate() accepted completed item without timestamp")
	}

	streak := Streak{StudentID: studentID, CurrentDays: 2, LongestDays: 3, LastStudyAt: &now}
	if err := streak.Validate(); err != nil {
		t.Fatalf("Streak.Validate() error = %v", err)
	}
	streak.CurrentDays = 4
	if err := streak.Validate(); err == nil {
		t.Fatal("Streak.Validate() accepted current streak above longest")
	}

	achievement := Achievement{
		ID: mustID(t, "achievement.001"), StudentID: studentID,
		Key: mustID(t, "achievement.first-review"), Name: "First review",
		Status: AchievementUnlocked, UnlockedAt: &now,
	}
	if err := achievement.Validate(); err != nil {
		t.Fatalf("Achievement.Validate() error = %v", err)
	}
	achievement.Status = AchievementLocked
	if err := achievement.Validate(); err == nil {
		t.Fatal("Achievement.Validate() accepted locked achievement with timestamp")
	}

	milestone := Milestone{
		ID: mustID(t, "milestone.001"), StudentID: studentID, GoalID: mustID(t, "goal.statistics"),
		Name: "First module completed", ReachedAt: now,
	}
	if err := milestone.Validate(); err != nil {
		t.Fatalf("Milestone.Validate() error = %v", err)
	}
}

func TestAnalyticsAndDailyPlanUseExplicitValidatedMetrics(t *testing.T) {
	t.Parallel()

	studentID := mustID(t, "student.ada")
	goalID := mustID(t, "goal.statistics")
	conceptID := mustID(t, "concept.arithmetic-mean")
	now := mustTimestamp(t, 9)
	snapshot := AnalyticsSnapshot{
		StudentID: studentID, CapturedAt: now, StudyMinutes: 90,
		SessionsCompleted: 2, ConceptsIntroduced: 3, ConceptsMastered: 1, ReviewsDue: 1,
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("AnalyticsSnapshot.Validate() error = %v", err)
	}
	snapshot.ConceptsMastered = 4
	if err := snapshot.Validate(); err == nil {
		t.Fatal("AnalyticsSnapshot.Validate() accepted mastered above introduced")
	}

	item := DailyPlanItem{
		ID: mustID(t, "plan-item.001"), Type: DailyPlanReview,
		ConceptIDs: []ID{conceptID}, EstimatedMinutes: 20, Position: 0,
	}
	plan := DailyPlan{
		ID: mustID(t, "plan.2026-08-19"), StudentID: studentID, GoalID: goalID,
		Date: now, CreatedAt: now, Items: []DailyPlanItem{item},
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("DailyPlan.Validate() error = %v", err)
	}
	duplicate := item
	duplicate.ID = mustID(t, "plan-item.002")
	plan.Items = append(plan.Items, duplicate)
	if err := plan.Validate(); err == nil {
		t.Fatal("DailyPlan.Validate() accepted duplicate item position")
	}
}
