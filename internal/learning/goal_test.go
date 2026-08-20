package learning

import "testing"

func TestLearningGoalLifecycle(t *testing.T) {
	t.Parallel()

	createdAt := mustTimestamp(t, 10)
	goal, err := NewLearningGoal(
		mustID(t, "goal.calculus"), mustID(t, "student.ada"),
		GoalDetails{
			Title: "Learn differential equations", Description: "Prepare for applied modeling",
			Domain: "Mathematics", TargetOutcome: "Solve first-order differential equations",
			StartingLevel: ExperienceBeginner,
		}, mustThreshold(t, 0.8), createdAt,
	)
	if err != nil {
		t.Fatalf("NewLearningGoal() error = %v", err)
	}
	activatedAt := mustTimestamp(t, 11)
	goal, err = goal.Activate(activatedAt)
	if err != nil || goal.Status != GoalActive || goal.ActivatedAt == nil || *goal.ActivatedAt != activatedAt {
		t.Fatalf("Activate() = (%+v, %v)", goal, err)
	}
	pausedAt := mustTimestamp(t, 12)
	goal, err = goal.Pause(pausedAt)
	if err != nil || goal.Status != GoalPaused || goal.UpdatedAt != pausedAt {
		t.Fatalf("Pause() = (%+v, %v)", goal, err)
	}
	resumedAt := mustTimestamp(t, 13)
	goal, err = goal.Activate(resumedAt)
	if err != nil || goal.Status != GoalActive || *goal.ActivatedAt != activatedAt {
		t.Fatalf("resume Activate() = (%+v, %v)", goal, err)
	}
	completedAt := mustTimestamp(t, 14)
	goal, err = goal.Complete(completedAt)
	if err != nil || goal.Status != GoalCompleted || goal.CompletedAt == nil || *goal.CompletedAt != completedAt {
		t.Fatalf("Complete() = (%+v, %v)", goal, err)
	}
	if _, err := goal.Activate(mustTimestamp(t, 15)); err == nil {
		t.Fatal("Activate() accepted completed goal")
	}
}

func TestLearningGoalRejectsInvalidDetailsAndTransitions(t *testing.T) {
	t.Parallel()

	createdAt := mustTimestamp(t, 10)
	details := GoalDetails{
		Title: "Learn Git", Domain: "Software development",
		TargetOutcome: "Use Git in professional workflows", StartingLevel: ExperienceNovice,
	}
	goal, err := NewLearningGoal(mustID(t, "goal.git"), mustID(t, "student.ada"), details, mustThreshold(t, .8), createdAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := goal.Pause(mustTimestamp(t, 11)); err == nil {
		t.Fatal("Pause() accepted draft goal")
	}
	if _, err := goal.Activate(mustTimestamp(t, 9)); err == nil {
		t.Fatal("Activate() accepted a timestamp before creation")
	}
	active, err := goal.Activate(createdAt)
	if err != nil {
		t.Fatal(err)
	}
	future := mustTimestamp(t, 12)
	active.ActivatedAt = &future
	if err := active.Validate(); err == nil {
		t.Fatal("Validate() accepted activation after the latest update")
	}
	details.Domain = "   "
	if _, err := NewLearningGoal(mustID(t, "goal.invalid"), mustID(t, "student.ada"), details, mustThreshold(t, .8), createdAt); err == nil {
		t.Fatal("NewLearningGoal() accepted an empty domain")
	}
}
