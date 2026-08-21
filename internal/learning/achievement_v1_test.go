package learning

import (
	"testing"
	"time"
)

func TestAchievementV1UnlocksMultipleConditionsAtHistoricalInstants(t *testing.T) {
	t.Parallel()
	studentID := streakID(t, "student.achievement")
	goalID := streakID(t, "goal.achievement")
	instanceID := streakID(t, "instance.achievement")
	conceptA, conceptB := streakID(t, "concept.achievement.a"), streakID(t, "concept.achievement.b")
	moduleID := streakID(t, "module.achievement")
	base := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	sessions := make([]StudySession, 0, 7)
	for day := 0; day < 7; day++ {
		sessions = append(sessions, streakSession(t, studentID, timeKey(base, day), base.AddDate(0, 0, day), 100*time.Minute))
		sessions[day].GoalID = goalID
		sessions[day].CurriculumInstanceID = instanceID
	}
	masteredA := streakTimestamp(t, base.AddDate(0, 0, 1))
	masteredB := streakTimestamp(t, base.AddDate(0, 0, 2))
	reviewedAt := streakTimestamp(t, base.AddDate(0, 0, 3))
	review := ReviewItem{
		ID: streakID(t, "review.achievement"), StudentID: studentID, ConceptID: conceptA,
		DueAt: masteredB, Type: ReviewStandard, EstimatedMinutes: 10, Status: ReviewCompleted,
		CompletedAt: &reviewedAt, CreatedAt: masteredB, AlgorithmVersion: LegacyReviewSchedulerVersion,
	}
	input := AchievementEvaluationInput{
		StudentID: studentID, Timezone: "UTC", AsOf: streakTimestamp(t, base.AddDate(0, 0, 7)),
		Sessions: sessions, Reviews: []ReviewItem{review}, StreakPolicy: DefaultStreakPolicy(),
		Modules: []AchievementModuleProgress{{
			StudentID: studentID, CurriculumInstanceID: instanceID, GoalID: goalID,
			Curriculum: CurriculumRef{ID: streakID(t, "curriculum.achievement"), Version: "1.0.0"}, ModuleID: moduleID,
			Concepts: []AchievementConceptProgress{{ConceptID: conceptA, MasteredAt: &masteredA}, {ConceptID: conceptB, MasteredAt: &masteredB}},
		}},
	}

	achievements, err := EvaluateAchievementsV1(FoundationAchievementDefinitions(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(achievements) != 6 {
		t.Fatalf("achievements = %+v, want all six Foundation definitions", achievements)
	}
	byKey := make(map[string]Achievement, len(achievements))
	for _, achievement := range achievements {
		byKey[achievement.Key.String()] = achievement
		if achievement.PolicyVersion != AchievementPolicyVersion || achievement.Status != AchievementUnlocked {
			t.Fatalf("achievement metadata = %+v", achievement)
		}
	}
	assertAchievementTime(t, byKey["first_session"], *sessions[0].EndedAt)
	assertAchievementTime(t, byKey["first_concept_mastered"], masteredA)
	assertAchievementTime(t, byKey["module_mastered"], masteredB)
	assertAchievementTime(t, byKey["first_review_completed"], reviewedAt)
	assertAchievementTime(t, byKey["ten_hours_studied"], *sessions[5].EndedAt)
	assertAchievementTime(t, byKey["seven_active_days"], *sessions[6].EndedAt)
	if byKey["module_mastered"].Context["module_id"] != moduleID.String() ||
		byKey["seven_active_days"].Context["active_days"] != "7" {
		t.Fatalf("achievement contexts = module %+v active %+v", byKey["module_mastered"].Context, byKey["seven_active_days"].Context)
	}
}

func TestAchievementV1LeavesUnsatisfiedDefinitionsLockedOutsidePersistence(t *testing.T) {
	t.Parallel()
	input := AchievementEvaluationInput{
		StudentID: streakID(t, "student.achievement.empty"), Timezone: "UTC",
		AsOf:         streakTimestamp(t, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)),
		StreakPolicy: DefaultStreakPolicy(),
	}
	achievements, err := EvaluateAchievementsV1(FoundationAchievementDefinitions(), input)
	if err != nil || len(achievements) != 0 {
		t.Fatalf("EvaluateAchievementsV1() = (%+v, %v), want no unlock candidates", achievements, err)
	}
}

func assertAchievementTime(t *testing.T, achievement Achievement, want Timestamp) {
	t.Helper()
	if achievement.UnlockedAt == nil || *achievement.UnlockedAt != want {
		t.Fatalf("achievement %q unlocked at %+v, want %+v", achievement.Key, achievement.UnlockedAt, want)
	}
}
