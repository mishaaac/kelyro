package application_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestProgressDashboardNewStudentHasExplicitEmptyState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	now := time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC)
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students),
		application.WithProfileClock(func() time.Time { return now.Add(-time.Hour) }))
	mastery := application.NewMasteryPolicyService(profiles, store.Repositories().Mastery,
		application.WithMasteryPolicyClock(func() time.Time { return now }))
	plans := application.NewAdaptiveDailyPlanService(profiles, mastery, store,
		application.WithAdaptiveDailyPlanClock(func() time.Time { return now }))
	dashboard := application.NewProgressDashboardService(profiles, mastery, plans, store,
		application.WithProgressDashboardClock(func() time.Time { return now }))

	view, err := dashboard.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.Goal != nil || view.Curriculum != nil || view.Current != nil || view.TodayPlan != nil || view.RecentMilestone != nil {
		t.Fatalf("new-student optional data = %+v", view)
	}
	if view.OverallProgress.ConceptsTotal.Value != 0 || view.OverallProgress.ConceptsIntroduced.Value != 0 ||
		view.Mastery.AverageKnown.Value != nil || view.ReviewsDue.Value != 0 || view.StudyTime.Total.Value != 0 ||
		view.Streak.CurrentStreak.Value != 0 || len(view.WeakConcepts) != 0 {
		t.Fatalf("new-student metrics = %+v", view)
	}
	if view.ReadModelVersion != application.ProgressDashboardReadModelVersion ||
		view.AnalyticsVersion != learning.LearningAnalyticsPolicyVersion || !view.GeneratedAt.Time().Equal(now) {
		t.Fatalf("new-student metadata = %+v", view)
	}
	if view.MasteryRequirement.Requirement.Threshold.Value() != .8 ||
		view.MasteryRequirement.Source != learning.MasterySourceStudentDefault {
		t.Fatalf("new-student mastery requirement = %+v", view.MasteryRequirement)
	}
}

func TestProgressDashboardShowsPartialActiveCurriculum(t *testing.T) {
	t.Parallel()
	fixture := newProgressDashboardFixture(t, instanceTestCurriculum(t, "dashboard-partial"))
	student, err := fixture.profiles.Show(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	conceptA := testID(t, "concept.a")
	conceptB := testID(t, "concept.b")
	fixture.saveState(t, conceptA, learning.ExposureMastered, .9, fixture.now.Add(-4*time.Hour), true)
	fixture.saveState(t, conceptB, learning.ExposureLearning, .4, fixture.now.Add(-3*time.Hour), false)
	if err := fixture.store.Repositories().StudySessions.Create(fixture.ctx,
		completedStudySession(t, "session.dashboard", student.ID, fixture.goal.ID, fixture.instance.ID, fixture.now.Add(-30*time.Minute), 15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	first := learning.Milestone{ID: testID(t, "milestone.dashboard.first"), StudentID: student.ID, GoalID: fixture.goal.ID,
		Name: "First", ReachedAt: fixtureTimestamp(t, fixture.now.Add(-2*time.Hour))}
	latest := learning.Milestone{ID: testID(t, "milestone.dashboard.latest"), StudentID: student.ID, GoalID: fixture.goal.ID,
		Name: "Latest", ReachedAt: fixtureTimestamp(t, fixture.now.Add(-time.Hour))}
	for _, milestone := range []learning.Milestone{latest, first} {
		if err := fixture.store.Repositories().Achievements.AppendMilestone(fixture.ctx, milestone); err != nil {
			t.Fatal(err)
		}
	}

	view, err := fixture.dashboard.Show(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.Goal == nil || view.Goal.ID != fixture.goal.ID || view.Curriculum == nil ||
		view.Curriculum.Instance.ID != fixture.instance.ID || view.Curriculum.ConceptsTotal != 2 {
		t.Fatalf("active context = %+v", view)
	}
	if view.OverallProgress.ConceptsIntroduced.Value != 2 || view.OverallProgress.ConceptsLearning.Value != 1 ||
		view.OverallProgress.ConceptsMastered.Value != 1 || view.OverallProgress.Completion.Value != 50 {
		t.Fatalf("overall progress = %+v", view.OverallProgress)
	}
	if view.Mastery.AverageKnown.Value == nil || view.Mastery.AverageKnown.Value.Value() != .65 ||
		view.Mastery.KnownConcepts.Value != 2 {
		t.Fatalf("mastery summary = %+v", view.Mastery)
	}
	if view.Current == nil || view.Current.Phase.Title != "Phase" || view.Current.Module.Title != "Module" ||
		view.Current.Lesson.Title != "Lesson" || view.Current.Concept.ID != conceptB {
		t.Fatalf("current location = %+v", view.Current)
	}
	concepts := dashboardRoadmapConcepts(view.Roadmap)
	if len(concepts) != 2 || concepts[0].Status != application.DashboardRoadmapMastered || concepts[0].Mastery == nil ||
		concepts[1].Status != application.DashboardRoadmapCurrent || concepts[1].Mastery == nil || len(concepts[1].LockReasons) != 0 {
		t.Fatalf("roadmap concepts = %+v", concepts)
	}
	wantTypes := []learning.CurriculumNodeType{learning.CurriculumNodePhase, learning.CurriculumNodeModule,
		learning.CurriculumNodeLesson, learning.CurriculumNodeTopic, learning.CurriculumNodeConcept, learning.CurriculumNodeConcept}
	if len(view.Roadmap) != len(wantTypes) {
		t.Fatalf("roadmap length = %d, want %d", len(view.Roadmap), len(wantTypes))
	}
	for index, want := range wantTypes {
		if view.Roadmap[index].Type != want {
			t.Fatalf("roadmap[%d] type = %q, want %q: %+v", index, view.Roadmap[index].Type, want, view.Roadmap)
		}
	}
	if len(view.WeakConcepts) != 2 || view.WeakConcepts[0].ConceptID != conceptB || view.WeakConcepts[0].Title != "B" {
		t.Fatalf("weak concepts = %+v", view.WeakConcepts)
	}
	if view.TodayPlan == nil || view.TodayPlan.CurriculumInstanceID != fixture.instance.ID ||
		view.StudyTime.Today.Value != 15*time.Minute || view.Streak.CurrentStreak.Value != 1 ||
		view.RecentMilestone == nil || view.RecentMilestone.ID != latest.ID {
		t.Fatalf("dashboard outcomes = %+v", view)
	}
}

func TestProgressDashboardExplainsLockedRoadmapConcept(t *testing.T) {
	t.Parallel()
	fixture := newProgressDashboardFixture(t, instanceTestCurriculum(t, "dashboard-locked"))
	fixture.saveState(t, testID(t, "concept.a"), learning.ExposureLearning, .4, fixture.now.Add(-time.Hour), false)

	view, err := fixture.dashboard.Show(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	concepts := dashboardRoadmapConcepts(view.Roadmap)
	if len(concepts) != 2 || concepts[0].Status != application.DashboardRoadmapCurrent ||
		concepts[1].Status != application.DashboardRoadmapLocked || len(concepts[1].LockReasons) != 1 ||
		concepts[1].LockReasons[0] != "Requires mastery of A; current mastery is 40%." {
		t.Fatalf("locked roadmap = %+v", concepts)
	}
}

func TestProgressDashboardShowsEffectiveMasteryOverride(t *testing.T) {
	t.Parallel()
	fixture := newProgressDashboardFixture(t, instanceTestCurriculum(t, "dashboard-mastery-override"))
	threshold, err := learning.NewMasteryThreshold(.9)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.mastery.SetWorkspaceOverride(fixture.ctx, threshold); err != nil {
		t.Fatal(err)
	}

	view, err := fixture.dashboard.Show(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.MasteryRequirement.Requirement.Threshold.Value() != .9 ||
		view.MasteryRequirement.Source != learning.MasterySourceWorkspaceOverride {
		t.Fatalf("effective mastery requirement = %+v", view.MasteryRequirement)
	}
}

func dashboardRoadmapConcepts(nodes []application.DashboardRoadmapNode) []application.DashboardRoadmapNode {
	concepts := make([]application.DashboardRoadmapNode, 0)
	for _, node := range nodes {
		if node.Type == learning.CurriculumNodeConcept {
			concepts = append(concepts, node)
		}
	}
	return concepts
}

func TestProgressDashboardCountsDueReviewsInActiveCurriculum(t *testing.T) {
	t.Parallel()
	fixture := newProgressDashboardFixture(t, instanceTestCurriculum(t, "dashboard-review"))
	student, err := fixture.profiles.Show(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	conceptID := testID(t, "concept.a")
	fixture.saveState(t, conceptID, learning.ExposureMastered, .9, fixture.now.Add(-72*time.Hour), true)
	review := applicationWarmUpReview(t, student.ID, conceptID, fixture.now)
	if err := fixture.store.Repositories().Reviews.CreateItem(fixture.ctx, review); err != nil {
		t.Fatal(err)
	}

	view, err := fixture.dashboard.Show(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.ReviewsDue.Value != 1 || view.TodayPlan == nil {
		t.Fatalf("due-review dashboard = %+v", view)
	}
	found := false
	for _, item := range view.TodayPlan.Items {
		if item.ConceptIDs[0] == conceptID && (item.Role == learning.DailyPlanRoleReview || item.Role == learning.DailyPlanRoleWarmUp) {
			found = true
		}
	}
	if !found {
		t.Fatalf("today plan did not include due review: %+v", view.TodayPlan.Items)
	}
}

func TestProgressDashboardRefreshesAfterLearningStateAndSessionChange(t *testing.T) {
	t.Parallel()
	fixture := newProgressDashboardFixture(t, instanceTestCurriculum(t, "dashboard-refresh"))
	student, err := fixture.profiles.Show(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	before, err := fixture.dashboard.Show(fixture.ctx)
	if err != nil || before.TodayPlan == nil || before.Current == nil || before.Current.Concept.ID != testID(t, "concept.a") {
		t.Fatalf("dashboard before refresh = (%+v, %v)", before, err)
	}
	fixture.saveState(t, testID(t, "concept.a"), learning.ExposureMastered, .9, fixture.now.Add(-time.Hour), true)
	if err := fixture.store.Repositories().StudySessions.Create(fixture.ctx,
		completedStudySession(t, "session.dashboard.refresh", student.ID, fixture.goal.ID, fixture.instance.ID,
			fixture.now.Add(-20*time.Minute), 12*time.Minute)); err != nil {
		t.Fatal(err)
	}
	after, err := fixture.dashboard.Show(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.OverallProgress.ConceptsMastered.Value != 1 || after.Current == nil ||
		after.Current.Concept.ID != testID(t, "concept.b") || after.StudyTime.Today.Value != 12*time.Minute ||
		after.TodayPlan == nil || after.TodayPlan.SourceFingerprint == before.TodayPlan.SourceFingerprint {
		t.Fatalf("dashboard after refresh = %+v", after)
	}
}

func TestProgressDashboardIgnoresPausedGoalContext(t *testing.T) {
	t.Parallel()
	fixture := newProgressDashboardFixture(t, instanceTestCurriculum(t, "dashboard-paused"))
	if _, err := fixture.goals.Pause(fixture.ctx); err != nil {
		t.Fatal(err)
	}
	view, err := fixture.dashboard.Show(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.Goal != nil || view.Curriculum != nil || view.Current != nil || view.TodayPlan != nil ||
		view.OverallProgress.ConceptsTotal.Value != 0 {
		t.Fatalf("paused-goal dashboard = %+v", view)
	}
}

func TestProgressDashboardHandlesThousandsOfConcepts(t *testing.T) {
	t.Parallel()
	const conceptCount = 5000
	fixture := newProgressDashboardFixture(t, progressDashboardCurriculum(t, conceptCount))
	view, err := fixture.dashboard.Show(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.Curriculum == nil || view.Curriculum.ConceptsTotal != conceptCount ||
		view.OverallProgress.ConceptsTotal.Value != conceptCount || view.Current == nil ||
		view.Current.Concept.ID != testID(t, "concept.dashboard.00000") || view.TodayPlan == nil {
		t.Fatalf("large dashboard summary = curriculum=%+v progress=%+v current=%+v plan=%+v",
			view.Curriculum, view.OverallProgress, view.Current, view.TodayPlan)
	}
}

type progressDashboardFixture struct {
	ctx       context.Context
	store     *memory.Store
	profiles  application.ProfileService
	goals     application.GoalLifecycleService
	instances application.CurriculumInstanceService
	mastery   application.MasteryPolicyService
	goal      learning.LearningGoal
	instance  learning.CurriculumInstance
	dashboard application.ProgressDashboardService
	now       time.Time
}

func newProgressDashboardFixture(t *testing.T, curriculum learning.Curriculum) *progressDashboardFixture {
	t.Helper()
	ctx := context.Background()
	store := memory.New()
	createdAt := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	now := time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC)
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students),
		application.WithProfileClock(func() time.Time { return createdAt }))
	goals := application.NewGoalLifecycleService(profiles, store,
		application.WithGoalClock(func() time.Time { return createdAt.Add(time.Hour) }),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.dashboard"), nil }))
	goal, err := goals.Set(ctx, goalInput(t, "Dashboard fixture", "General knowledge"))
	if err != nil {
		t.Fatal(err)
	}
	instances := application.NewCurriculumInstanceService(profiles, store,
		application.WithCurriculumInstanceClock(func() time.Time { return createdAt.Add(2 * time.Hour) }),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.dashboard"), nil }))
	instance, err := instances.Create(ctx, goal.ID, curriculum, learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	mastery := application.NewMasteryPolicyService(profiles, store.Repositories().Mastery,
		application.WithMasteryPolicyClock(func() time.Time { return createdAt.Add(3 * time.Hour) }))
	plans := application.NewAdaptiveDailyPlanService(profiles, mastery, store,
		application.WithAdaptiveDailyPlanClock(func() time.Time { return now }))
	dashboard := application.NewProgressDashboardService(profiles, mastery, plans, store,
		application.WithProgressDashboardClock(func() time.Time { return now }))
	return &progressDashboardFixture{
		ctx: ctx, store: store, profiles: profiles, goals: goals, instances: instances, mastery: mastery,
		goal: goal, instance: instance, dashboard: dashboard, now: now,
	}
}

func (fixture *progressDashboardFixture) saveState(t *testing.T, conceptID learning.ID, exposure learning.ExposureState,
	mastery float64, observed time.Time, mastered bool) {
	t.Helper()
	state, err := fixture.instances.State(fixture.ctx, fixture.instance.ID, conceptID)
	if err != nil {
		t.Fatal(err)
	}
	timestamp := fixtureTimestamp(t, observed)
	state.Exposure = exposure
	state.Mastery = testScore(t, mastery)
	state.FirstSeenAt, state.LastSeenAt = &timestamp, &timestamp
	if mastered {
		state.MasteredAt = &timestamp
	}
	state.UpdatedAt = timestamp
	if err := fixture.instances.SaveState(fixture.ctx, state); err != nil {
		t.Fatal(err)
	}
}

func progressDashboardCurriculum(t *testing.T, conceptCount int) learning.Curriculum {
	t.Helper()
	phaseID := testID(t, "phase.dashboard")
	moduleID := testID(t, "module.dashboard")
	lessonID := testID(t, "lesson.dashboard")
	topicID := testID(t, "topic.dashboard")
	status := learning.CurriculumStatusMetadata{State: learning.CurriculumNodeActive}
	version := "large-v1"
	nodes := []learning.CurriculumNode{
		{ID: phaseID, Type: learning.CurriculumNodePhase, Title: "Phase", Description: "Phase.", Status: status, Version: version},
		{ID: moduleID, Type: learning.CurriculumNodeModule, ParentID: &phaseID, Title: "Module", Description: "Module.", Status: status, Version: version},
		{ID: lessonID, Type: learning.CurriculumNodeLesson, ParentID: &moduleID, Title: "Lesson", Description: "Lesson.", Status: status, Version: version},
		{ID: topicID, Type: learning.CurriculumNodeTopic, ParentID: &lessonID, Title: "Topic", Description: "Topic.", Status: status, Version: version},
	}
	for index := 0; index < conceptCount; index++ {
		id := testID(t, fmt.Sprintf("concept.dashboard.%05d", index))
		nodes = append(nodes, learning.CurriculumNode{
			ID: id, Type: learning.CurriculumNodeConcept, ParentID: &topicID, Title: fmt.Sprintf("Concept %d", index),
			Description: "Deterministic dashboard scale fixture.", Order: index, Status: status, Version: version,
			Concept: &learning.ConceptDefinition{
				Objectives: []string{"Understand the concept."}, Difficulty: learning.ConceptDifficultyFoundational,
				EstimatedEffortMinutes: 10, AssessmentExpectations: []string{"Explain the concept."},
			},
		})
	}
	curriculum, err := learning.NewCurriculum(learning.CurriculumContractVersion,
		learning.CurriculumRef{ID: testID(t, "fixture.dashboard.large"), Version: version},
		"Large dashboard fixture", "Deterministic scale fixture for the progress dashboard.", nodes)
	if err != nil {
		t.Fatal(err)
	}
	return curriculum
}
