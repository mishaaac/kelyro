package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/doctor"
	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/portability"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/update"
)

func TestRunnerDispatchesFoundationCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantAction app.Action
	}{
		{name: "default TUI", wantAction: app.ActionTUI},
		{name: "init", args: []string{"init"}, wantAction: app.ActionInit},
		{name: "doctor", args: []string{"doctor"}, wantAction: app.ActionDoctor},
		{name: "config", args: []string{"config"}, wantAction: app.ActionConfig},
		{name: "secrets", args: []string{"secrets", "status"}, wantAction: app.ActionSecrets},
		{name: "status", args: []string{"status"}, wantAction: app.ActionStatus},
		{name: "progress", args: []string{"progress"}, wantAction: app.ActionProgress},
		{name: "roadmap", args: []string{"roadmap"}, wantAction: app.ActionRoadmap},
		{name: "today", args: []string{"today"}, wantAction: app.ActionToday},
		{name: "open", args: []string{"open"}, wantAction: app.ActionOpen},
		{name: "logs path", args: []string{"logs", "path"}, wantAction: app.ActionLogs},
		{name: "audit", args: []string{"audit"}, wantAction: app.ActionAudit},
		{name: "export", args: []string{"export"}, wantAction: app.ActionExport},
		{name: "import", args: []string{"import", "workspace.tar.gz"}, wantAction: app.ActionImport},
		{name: "update check", args: []string{"update", "check"}, wantAction: app.ActionUpdate},
		{name: "profile show", args: []string{"profile", "show"}, wantAction: app.ActionProfile},
		{name: "goal show", args: []string{"goal", "show"}, wantAction: app.ActionGoal},
		{name: "mastery threshold", args: []string{"mastery", "threshold"}, wantAction: app.ActionMastery},
		{name: "setup status", args: []string{"setup", "status"}, wantAction: app.ActionSetup},
		{name: "mistakes", args: []string{"mistakes"}, wantAction: app.ActionMistakes},
		{name: "session status", args: []string{"session", "status"}, wantAction: app.ActionSession},
		{name: "history", args: []string{"history"}, wantAction: app.ActionHistory},
		{name: "time", args: []string{"time"}, wantAction: app.ActionTime},
		{name: "reviews", args: []string{"reviews"}, wantAction: app.ActionReviews},
		{name: "streak", args: []string{"streak"}, wantAction: app.ActionStreak},
		{name: "maintenance", args: []string{"maintenance", "recalculate", "--dry-run"}, wantAction: app.ActionMaintenance},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeService{result: app.Result{Message: "done"}}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			runner := NewRunner(service, &stdout, &stderr)

			if exitCode := runner.Run(context.Background(), test.args); exitCode != ExitOK {
				t.Fatalf("Run() exit code = %d, want %d", exitCode, ExitOK)
			}
			if len(service.commands) != 1 {
				t.Fatalf("service calls = %d, want 1", len(service.commands))
			}
			if got := service.commands[0].Action; got != test.wantAction {
				t.Errorf("dispatched action = %q, want %q", got, test.wantAction)
			}
			if got, want := stdout.String(), "done\n"; got != want {
				t.Errorf("stdout = %q, want %q", got, want)
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunnerDispatchesAndRendersMaintenanceRecalculation(t *testing.T) {
	t.Parallel()
	impact := learningapp.RecalculationImpact{
		DryRun: true,
		Target: learningapp.AlgorithmVersionSummary{
			Mastery: []string{"mastery-v1"}, Retention: []string{"retention-v1"}, DailyPlan: []string{"daily-plan-v1"},
		},
		EvidenceRecords: 4, ConceptsScanned: 2, ConceptStatesChanged: 2, RetentionStatesChanged: 2,
		ReviewSchedulesChanged: 1, ReviewItemsChanged: 1, DailyPlansChanged: 1,
	}
	service := &fakeService{result: app.Result{Maintenance: &impact}}
	var stdout, stderr bytes.Buffer
	code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"--workspace", "project", "maintenance", "recalculate", "--dry-run"})
	if code != ExitOK || stderr.Len() != 0 {
		t.Fatalf("maintenance dry-run exit=%d stderr=%q", code, stderr.String())
	}
	if len(service.commands) != 1 || service.commands[0].Action != app.ActionMaintenance ||
		service.commands[0].MaintenanceOperation != "recalculate" || !service.commands[0].MaintenanceDryRun || service.commands[0].Workspace != "project" {
		t.Fatalf("maintenance command=%+v", service.commands)
	}
	for _, want := range []string{
		"Learning-state recalculation — dry run", "Target mastery: mastery-v1", "Evidence records read: 4 (unchanged)",
		"Would change concept states: 2", "Would change retention states: 2", "Would change review schedules: 1",
		"Would change review items: 1", "Would change daily plans: 1", "No learning state was written and no backup was created.",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("maintenance output lacks %q:\n%s", want, stdout.String())
		}
	}

	service = &fakeService{}
	stdout.Reset()
	stderr.Reset()
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"maintenance", "unknown"}); code != ExitUsage {
		t.Fatalf("invalid maintenance exit=%d, want %d", code, ExitUsage)
	}
	if len(service.commands) != 0 || !strings.Contains(stderr.String(), "maintenance requires recalculate") {
		t.Fatalf("invalid maintenance commands=%+v stderr=%q", service.commands, stderr.String())
	}
}

func TestRunnerRendersStudentCoreDashboardCommands(t *testing.T) {
	t.Parallel()
	dashboard := testCLIDashboard(t)
	tests := []struct {
		command string
		wants   []string
	}{
		{command: "status", wants: []string{"Learning status", "Goal: Backend Engineer with Go", "Current: Foundations / Go basics / Variables / Initialization / Short declarations", "Mastery threshold: 85%", "Mastered: 1", "Learning: 1", "Review due: 2"}},
		{command: "progress", wants: []string{"Progress", "Completion: 50%", "Mastered: 1 of 2 concepts", "Average mastery (known concepts): 65%", "Study today: 25m", "Study this week: 4h12m", "Current streak: 6 days", "Needs reinforcement", "Short declarations: 40% mastery"}},
		{command: "roadmap", wants: []string{"Roadmap", "Phase: Foundations", "Short declarations [current] 40% mastery", "Functions [locked]", "Why: Master Short declarations first.", "Legend: mastered, current, available, locked, review due"}},
		{command: "today", wants: []string{"Today", "Goal: Backend Engineer with Go", "Planned: 25 of 30 minutes", "1. new learning — Short declarations (25 min)", "Short declarations is the next eligible concept."}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.command, func(t *testing.T) {
			t.Parallel()
			service := &fakeService{result: app.Result{Dashboard: &dashboard}}
			var stdout, stderr bytes.Buffer
			if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{test.command}); code != ExitOK {
				t.Fatalf("%s exit=%d stderr=%q", test.command, code, stderr.String())
			}
			for _, want := range test.wants {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("%s output missing %q:\n%s", test.command, want, stdout.String())
				}
			}
		})
	}
}

func TestRunnerStudentCoreAliasesAndWorkspaceOverride(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		command   string
		operation string
	}{
		{command: "profile", operation: "show"},
		{command: "goal", operation: "show"},
		{command: "mastery", operation: "show"},
	} {
		service := &fakeService{result: app.Result{Message: "ok"}}
		if code := NewRunner(service, &bytes.Buffer{}, &bytes.Buffer{}).Run(context.Background(), []string{test.command}); code != ExitOK {
			t.Fatalf("%s exit=%d", test.command, code)
		}
		command := service.commands[0]
		var operation string
		switch test.command {
		case "profile":
			operation = command.ProfileOperation
		case "goal":
			operation = command.GoalOperation
		case "mastery":
			operation = command.MasteryOperation
		}
		if operation != test.operation {
			t.Errorf("%s operation=%q, want %q", test.command, operation, test.operation)
		}
	}

	service := &fakeService{result: app.Result{Message: "ok"}}
	interactive := &fakeInteractiveRunner{}
	workspacePath := filepath.Join("projects", "student core")
	runner := NewRunner(service, &bytes.Buffer{}, &bytes.Buffer{}).WithInteractive(interactive)
	if code := runner.Run(context.Background(), []string{"--workspace", workspacePath, "progress"}); code != ExitOK {
		t.Fatalf("progress exit=%d", code)
	}
	if len(service.commands) != 1 || service.commands[0].Workspace != workspacePath || service.commands[0].Action != app.ActionProgress {
		t.Fatalf("progress command=%+v", service.commands)
	}
	if len(interactive.commands) != 0 {
		t.Fatalf("explicit subcommand launched TUI: %+v", interactive.commands)
	}
}

func TestRunnerDispatchesProgressArtifactExport(t *testing.T) {
	t.Parallel()
	service := &fakeService{result: app.Result{Message: "Updated learning progress artifacts:\n- LEARNING.md"}}
	var stdout, stderr bytes.Buffer
	code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"progress", "export"})
	if code != ExitOK || stderr.Len() != 0 {
		t.Fatalf("progress export exit=%d stderr=%q", code, stderr.String())
	}
	if len(service.commands) != 1 || service.commands[0].Action != app.ActionProgress || service.commands[0].ProgressOperation != "export" {
		t.Fatalf("progress export command=%+v", service.commands)
	}
	if !strings.Contains(stdout.String(), "LEARNING.md") {
		t.Fatalf("progress export output=%q", stdout.String())
	}

	service = &fakeService{}
	stdout.Reset()
	stderr.Reset()
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"progress", "unexpected"}); code != ExitUsage {
		t.Fatalf("invalid progress subcommand exit=%d, want %d", code, ExitUsage)
	}
	if len(service.commands) != 0 || !strings.Contains(stderr.String(), "progress accepts no arguments or export") {
		t.Fatalf("invalid progress commands=%+v stderr=%q", service.commands, stderr.String())
	}
}

func TestRunnerHandlesIncompleteStudentCoreAndMissingWorkspace(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"status", "progress", "roadmap", "today"} {
		service := &fakeService{result: app.Result{Dashboard: &learningapp.ProgressDashboard{}}}
		var stdout, stderr bytes.Buffer
		if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{command}); code != ExitOK {
			t.Fatalf("%s incomplete exit=%d stderr=%q", command, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "kelyro setup status") {
			t.Errorf("%s incomplete output lacks guidance:\n%s", command, stdout.String())
		}
	}

	service := &fakeService{err: errors.New("workspace not initialized")}
	var stdout, stderr bytes.Buffer
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"status"}); code != ExitFailure {
		t.Fatalf("missing workspace exit=%d, want %d", code, ExitFailure)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "kelyro status: workspace not initialized") {
		t.Fatalf("missing workspace stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunnerDashboardDistinguishesUnknownFromKnownZeroMastery(t *testing.T) {
	t.Parallel()
	dashboard := testCLIDashboard(t)
	dashboard.Mastery.AverageKnown.Value = nil
	zero, err := learning.NewMasteryScore(0)
	if err != nil {
		t.Fatal(err)
	}
	dashboard.Roadmap[1].Mastery = &zero

	service := &fakeService{result: app.Result{Dashboard: &dashboard}}
	var stdout, stderr bytes.Buffer
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"progress"}); code != ExitOK {
		t.Fatalf("progress exit=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Average mastery (known concepts): unknown") {
		t.Fatalf("unknown mastery output:\n%s", stdout.String())
	}

	service = &fakeService{result: app.Result{Dashboard: &dashboard}}
	stdout.Reset()
	stderr.Reset()
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"roadmap"}); code != ExitOK {
		t.Fatalf("roadmap exit=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Short declarations [current] 0% mastery") {
		t.Fatalf("known zero mastery output:\n%s", stdout.String())
	}
}

func testCLIDashboard(t *testing.T) learningapp.ProgressDashboard {
	t.Helper()
	goal := testLearningGoal(t)
	requirement, err := learning.MasteryRequirementFromThreshold(goal.MasteryThreshold)
	if err != nil {
		t.Fatal(err)
	}
	phaseID := mustCLIid(t, "phase.cli")
	moduleID := mustCLIid(t, "module.cli")
	lessonID := mustCLIid(t, "lesson.cli")
	topicID := mustCLIid(t, "topic.cli")
	currentID := mustCLIid(t, "concept.short-declarations")
	lockedID := mustCLIid(t, "concept.functions")
	currentMastery, _ := learning.NewMasteryScore(.4)
	averageMastery, _ := learning.NewMasteryScore(.65)
	plan := &learning.DailyPlan{
		AvailableMinutes: 30,
		PlannedMinutes:   25,
		Status:           learning.DailyPlanReady,
		Items: []learning.DailyPlanItem{{
			Role: learning.DailyPlanRoleNewLearning, ConceptIDs: []learning.ID{currentID}, EstimatedMinutes: 25,
			Explanation: "concept.short-declarations is the next eligible concept.",
		}},
	}
	return learningapp.ProgressDashboard{
		Goal:       &goal,
		Curriculum: &learningapp.DashboardCurriculum{ConceptsTotal: 2},
		Current: &learningapp.DashboardLocation{
			Phase: learningapp.DashboardNode{ID: phaseID, Title: "Foundations"}, Module: learningapp.DashboardNode{ID: moduleID, Title: "Go basics"},
			Lesson: learningapp.DashboardNode{ID: lessonID, Title: "Variables"}, Topic: learningapp.DashboardNode{ID: topicID, Title: "Initialization"},
			Concept: learningapp.DashboardNode{ID: currentID, Title: "Short declarations"},
		},
		OverallProgress: learningapp.DashboardOverallProgress{
			ConceptsTotal: learning.AnalyticsCountMetric{Value: 2}, ConceptsIntroduced: learning.AnalyticsCountMetric{Value: 2},
			ConceptsLearning: learning.AnalyticsCountMetric{Value: 1}, ConceptsMastered: learning.AnalyticsCountMetric{Value: 1},
			Completion: learning.AnalyticsRateMetric{Value: 50},
		},
		Mastery:            learningapp.DashboardMasterySummary{KnownConcepts: learning.AnalyticsCountMetric{Value: 2}, AverageKnown: learning.AnalyticsScoreMetric{Value: &averageMastery}},
		MasteryRequirement: learning.ResolvedMasteryThreshold{Requirement: requirement, Source: learning.MasterySourceStudentDefault, PolicyVersion: learning.MasteryThresholdPolicyVersion},
		ReviewsDue:         learning.AnalyticsCountMetric{Value: 2},
		TodayPlan:          plan,
		StudyTime: learning.AnalyticsTime{
			Today: learning.AnalyticsDurationMetric{Value: 25 * time.Minute}, Week: learning.AnalyticsDurationMetric{Value: 4*time.Hour + 12*time.Minute},
		},
		Streak:       learning.AnalyticsActivity{CurrentStreak: learning.AnalyticsCountMetric{Value: 6}},
		WeakConcepts: []learningapp.DashboardWeakConcept{{ConceptID: currentID, Title: "Short declarations", Mastery: currentMastery}},
		Roadmap: []learningapp.DashboardRoadmapNode{
			{ID: phaseID, Type: learning.CurriculumNodePhase, Title: "Foundations", Depth: 0},
			{ID: currentID, ParentID: &phaseID, Type: learning.CurriculumNodeConcept, Title: "Short declarations", Depth: 1, Status: learningapp.DashboardRoadmapCurrent, Mastery: &currentMastery},
			{ID: lockedID, ParentID: &phaseID, Type: learning.CurriculumNodeConcept, Title: "Functions", Depth: 1, Status: learningapp.DashboardRoadmapLocked, LockReasons: []string{"Master Short declarations first."}},
		},
	}
}

func TestRunnerParsesAndFormatsDueReviews(t *testing.T) {
	studentID := mustCLIid(t, "student.primary")
	conceptID := mustCLIid(t, "concept.review")
	itemID := mustCLIid(t, "review.cli")
	dueAt := mustCLITimestamp(t, time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC))
	createdAt := mustCLITimestamp(t, time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC))
	strength, _ := learning.NewMasteryScore(.42)
	item := learning.ReviewItem{ID: itemID, StudentID: studentID, ConceptID: conceptID, DueAt: dueAt,
		Type: learning.ReviewDeep, EstimatedMinutes: 20, CriticalPrerequisite: true, Status: learning.ReviewPending,
		CreatedAt: createdAt, AlgorithmVersion: learning.ReviewSchedulerVersion}
	view := learningapp.ReviewQueueView{Items: []learning.ReviewQueueItem{{Item: item, Strength: strength,
		Status: learning.RetentionOverdue, Overdue: true, Critical: true}}, Pending: 3, BudgetMinutes: 30,
		UsedMinutes: 20, TotalDueMinutes: 50, Timezone: "America/Lima", GeneratedAt: dueAt,
		DueOnly: true, AlgorithmVersion: learning.ReviewSchedulerVersion}
	service := &fakeService{result: app.Result{Reviews: &view}}
	var stdout, stderr bytes.Buffer
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"reviews", "due"}); code != ExitOK {
		t.Fatalf("reviews due exit=%d stderr=%q", code, stderr.String())
	}
	if command := service.commands[0]; command.Action != app.ActionReviews || !command.ReviewsDue {
		t.Fatalf("reviews command = %+v", command)
	}
	for _, want := range []string{"Reviews — due", "Timezone: America/Lima", "Daily budget: 30 minutes", "2026-08-21T09:00:00-05:00",
		"concept.review", "20 min", "strength 42%", "overdue", "critical-prerequisite", "Policy: review-scheduler-v1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("reviews output missing %q:\n%s", want, stdout.String())
		}
	}
	for _, args := range [][]string{{"reviews", "later"}, {"reviews", "due", "extra"}} {
		stdout.Reset()
		stderr.Reset()
		if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), args); code != ExitUsage {
			t.Fatalf("Run(%v) exit=%d, want failure", args, code)
		}
	}
}

func TestRunnerFormatsStreakWithoutPunitiveLanguage(t *testing.T) {
	studentID := mustCLIid(t, "student.primary")
	lastDate, err := learning.NewLocalDate("2026-08-21")
	if err != nil {
		t.Fatal(err)
	}
	lastStudy := mustCLITimestamp(t, time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC))
	streak := learning.Streak{
		StudentID: studentID, CurrentDays: 6, LongestDays: 9, LastActiveLocalDate: &lastDate,
		TotalActiveDays: 20, LastStudyAt: &lastStudy, Timezone: "America/Lima", MinimumActiveMinutes: 10,
		PolicyVersion: learning.StreakPolicyVersion,
	}
	service := &fakeService{result: app.Result{Streak: &streak}}
	var stdout, stderr bytes.Buffer
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"streak"}); code != ExitOK {
		t.Fatalf("streak exit=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"Streak: 6 days", "Longest: 9 days", "Total active days: 20", "Last active date: 2026-08-21",
		"Timezone: America/Lima", "Policy: streak-v1", "does not change mastery or block learning"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("streak output missing %q:\n%s", want, stdout.String())
		}
	}
	for _, unwanted := range []string{"lost", "failure", "should have", "must study"} {
		if strings.Contains(strings.ToLower(stdout.String()), unwanted) {
			t.Errorf("streak output contains punitive language %q:\n%s", unwanted, stdout.String())
		}
	}
	one := streak
	one.CurrentDays = 1
	if got := formatStreak(one); !strings.Contains(got, "Streak: 1 day") {
		t.Errorf("singular streak output = %q", got)
	}
	stdout.Reset()
	stderr.Reset()
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"streak", "extra"}); code != ExitUsage {
		t.Fatalf("streak extra exit=%d, want usage", code)
	}
}

func TestRunnerParsesAndRendersStudyHistoryAndTime(t *testing.T) {
	t.Parallel()
	event := mustCLIStudyEvent(t, time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC))
	service := &fakeService{result: app.Result{History: &learningapp.StudyHistoryView{
		Events: []learning.StudyEvent{event}, Period: learning.StudyPeriodToday, Timezone: "America/Lima",
	}}}
	var stdout, stderr bytes.Buffer
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"history", "--today"}); code != ExitOK {
		t.Fatalf("history exit=%d stderr=%q", code, stderr.String())
	}
	if command := service.commands[0]; command.Action != app.ActionHistory || !command.HistoryToday {
		t.Fatalf("history command = %+v", command)
	}
	for _, want := range []string{"Study history — today", "Timezone: America/Lima", "2026-08-21T09:00:00-05:00", "evidence.recorded", "concept=concept.cli"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("history output missing %q:\n%s", want, stdout.String())
		}
	}

	service = &fakeService{result: app.Result{StudyTime: &learningapp.StudyTimeSummary{
		Today: 15 * time.Minute, Week: 45 * time.Minute, Month: time.Hour, Total: 2 * time.Hour,
		TodaySessions: 1, WeekSessions: 2, MonthSessions: 3, TotalSessions: 4, Timezone: "America/Lima",
		ByConcept:     []learning.StudyTimeBreakdown{{ID: mustCLIid(t, "concept.cli"), Duration: 15 * time.Minute, Sessions: 1}},
		PolicyVersion: learning.TimeTrackingPolicyVersion,
	}}}
	stdout.Reset()
	stderr.Reset()
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"time"}); code != ExitOK {
		t.Fatalf("time exit=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"Study time", "Today: 15m0s (1 sessions)", "Total: 2h0m0s (4 sessions)", "concept.cli=15m0s", "By module: unavailable", "Policy: time-tracking-v1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("time output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunnerRejectsInvalidStudyHistoryCommands(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"history", "extra"}, {"time", "extra"}, {"time", "--today"}} {
		var stderr bytes.Buffer
		if code := NewRunner(&fakeService{}, &bytes.Buffer{}, &stderr).Run(context.Background(), args); code != ExitUsage {
			t.Fatalf("Run(%v) exit=%d stderr=%q", args, code, stderr.String())
		}
	}
}

func mustCLIStudyEvent(t *testing.T, at time.Time) learning.StudyEvent {
	t.Helper()
	timestamp := mustCLITimestamp(t, at)
	goalID, instanceID, conceptID := mustCLIid(t, "goal.cli"), mustCLIid(t, "instance.cli"), mustCLIid(t, "concept.cli")
	event, err := learning.NewStudyEvent(mustCLIid(t, "history.cli"), mustCLIid(t, "student.primary"), learning.StudyEventEvidenceRecorded,
		mustCLIid(t, "evidence.cli"), timestamp, &goalID, &instanceID, &conceptID)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func TestRunnerParsesAndRendersStudySession(t *testing.T) {
	t.Parallel()
	started, _ := learning.NewTimestamp(time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC))
	session, err := learning.NewStudySession(mustCLIid(t, "session.cli"), mustCLIid(t, "student.primary"), mustCLIid(t, "goal.cli"), mustCLIid(t, "instance.cli"), started, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	session, _ = session.RecordActivity(mustCLITimestamp(t, started.Time().Add(5*time.Minute)))
	service := &fakeService{result: app.Result{StudySession: &session}}
	var stdout, stderr bytes.Buffer
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"session", "status"}); code != ExitOK {
		t.Fatalf("session status exit=%d stderr=%q", code, stderr.String())
	}
	if command := service.commands[0]; command.Action != app.ActionSession || command.SessionOperation != "status" {
		t.Fatalf("session command = %+v", command)
	}
	for _, want := range []string{"Study session", "Status: active", "Goal: goal.cli", "Active time: 5m0s", "Activities: 1", "Policy: study-session-v1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("session output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunnerRejectsInvalidStudySessionCommands(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"session"}, {"session", "start"}, {"session", "status", "extra"}} {
		var stderr bytes.Buffer
		if code := NewRunner(&fakeService{}, &bytes.Buffer{}, &stderr).Run(context.Background(), args); code != ExitUsage {
			t.Fatalf("Run(%v) exit=%d stderr=%q", args, code, stderr.String())
		}
	}
}

func TestRunnerParsesAndRendersMistakeMemory(t *testing.T) {
	t.Parallel()
	first, _ := learning.NewTimestamp(time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC))
	last, _ := learning.NewTimestamp(time.Date(2026, 8, 20, 11, 0, 0, 0, time.UTC))
	resolved, _ := learning.NewTimestamp(time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
	mistake, err := learning.NewMistake(mustCLIid(t, "mistake.mean"), mustCLIid(t, "student.primary"), mustCLIid(t, "concept.mean"),
		"mean-vs-median", learning.MistakeMisconception, "Confused mean and median", first, "fixture/check/1")
	if err != nil {
		t.Fatal(err)
	}
	mistake, _ = mistake.Observe(last, "fixture/check/2")
	mistake, _ = mistake.Resolve(resolved)
	service := &fakeService{result: app.Result{Mistakes: []learning.Mistake{mistake}}}
	var stdout, stderr bytes.Buffer
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"mistakes"}); code != ExitOK {
		t.Fatalf("mistakes exit=%d stderr=%q", code, stderr.String())
	}
	if command := service.commands[0]; command.Action != app.ActionMistakes || command.MistakeOperation != "list" {
		t.Fatalf("mistakes command = %+v", command)
	}
	for _, want := range []string{"Mistakes (1)", "[resolved]", "Confused mean and median", "2 occurrence(s)"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("mistakes output missing %q:\n%s", want, stdout.String())
		}
	}

	event, _ := learning.NewMistakeEvent(mustCLIid(t, "mistake-event.1"), mistake.ID, learning.MistakeObservedEvent, first, "fixture/check/1")
	service = &fakeService{result: app.Result{Mistake: &learningapp.MistakeView{Mistake: mistake, History: []learning.MistakeEvent{event}}}}
	stdout.Reset()
	stderr.Reset()
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"mistakes", "show", "mistake.mean"}); code != ExitOK {
		t.Fatalf("mistakes show exit=%d stderr=%q", code, stderr.String())
	}
	if command := service.commands[0]; command.MistakeOperation != "show" || command.MistakeID.String() != "mistake.mean" {
		t.Fatalf("mistakes show command = %+v", command)
	}
	for _, want := range []string{"Mistake memory", "Key: mean-vs-median", "Latest source: fixture/check/2", "History (1)", "observed at"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("mistake detail missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunnerRejectsInvalidMistakeCommands(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"mistakes", "show"}, {"mistakes", "unknown"}, {"mistakes", "show", "bad id"}} {
		var stderr bytes.Buffer
		if code := NewRunner(&fakeService{}, &bytes.Buffer{}, &stderr).Run(context.Background(), args); code != ExitUsage {
			t.Fatalf("Run(%v) exit=%d stderr=%q", args, code, stderr.String())
		}
	}
}

func mustCLIid(t *testing.T, value string) learning.ID {
	t.Helper()
	id, err := learning.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustCLITimestamp(t *testing.T, value time.Time) learning.Timestamp {
	t.Helper()
	timestamp, err := learning.NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}

func TestRunnerRendersIntegratedLearnerSetupStatus(t *testing.T) {
	t.Parallel()
	view := learningapp.LearnerSetupView{Setup: learning.LearnerSetup{Status: learning.SetupCompleted}, Instance: &learning.CurriculumInstance{
		Curriculum: learning.CurriculumRef{}, Source: learning.CurriculumSourceFixture,
	}}
	service := &fakeService{result: app.Result{Setup: &view}}
	var stdout, stderr bytes.Buffer
	if code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"setup", "status"}); code != ExitOK {
		t.Fatalf("setup status exit=%d stderr=%q", code, stderr.String())
	}
	if command := service.commands[0]; command.Action != app.ActionSetup || command.SetupOperation != "status" {
		t.Fatalf("setup command = %+v", command)
	}
	for _, want := range []string{"Learner setup", "Status: completed", "Source: fixture", "Diagnostic: not selected"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("setup output missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunnerParsesAndRendersMasteryThreshold(t *testing.T) {
	t.Parallel()
	requirement, _ := learning.NewMasteryRequirement(.85)
	resolved := learning.ResolvedMasteryThreshold{
		Requirement: requirement, Source: learning.MasterySourceWorkspaceOverride,
		PolicyVersion: learning.MasteryThresholdPolicyVersion,
	}
	service := &fakeService{result: app.Result{Mastery: &resolved}}
	var stdout, stderr bytes.Buffer
	if exitCode := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"mastery", "threshold", "set", "85"}); exitCode != ExitOK {
		t.Fatalf("mastery set exit=%d stderr=%q", exitCode, stderr.String())
	}
	if len(service.commands) != 1 || service.commands[0].Action != app.ActionMastery || service.commands[0].MasteryOperation != "set" || service.commands[0].MasteryThreshold.Value() != .85 {
		t.Fatalf("mastery command = %+v", service.commands)
	}
	for _, want := range []string{"Required mastery: 85%", "Mode: Strict", "Source: Workspace override", "Policy: threshold-v1", "not an assessment grade"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("mastery output missing %q:\n%s", want, stdout.String())
		}
	}

	for _, test := range []struct {
		args      []string
		operation string
	}{
		{[]string{"mastery", "threshold"}, "show"},
		{[]string{"mastery", "threshold", "set-default", "70"}, "set-default"},
		{[]string{"mastery", "threshold", "reset"}, "reset"},
	} {
		service := &fakeService{result: app.Result{Mastery: &resolved}}
		if exitCode := NewRunner(service, &bytes.Buffer{}, &bytes.Buffer{}).Run(context.Background(), test.args); exitCode != ExitOK {
			t.Fatalf("Run(%v) exit=%d", test.args, exitCode)
		}
		if service.commands[0].MasteryOperation != test.operation {
			t.Errorf("Run(%v) operation=%q", test.args, service.commands[0].MasteryOperation)
		}
	}
}

func TestRunnerRejectsInvalidMasteryThresholdCommands(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"mastery", "threshold", "set"}, {"mastery", "threshold", "set", "49"},
		{"mastery", "threshold", "set", "100"}, {"mastery", "threshold", "set", "85.5"},
		{"mastery", "threshold", "unknown"},
	} {
		var stderr bytes.Buffer
		if exitCode := NewRunner(&fakeService{}, &bytes.Buffer{}, &stderr).Run(context.Background(), args); exitCode != ExitUsage {
			t.Fatalf("Run(%v) exit=%d stderr=%q", args, exitCode, stderr.String())
		}
	}
}

func TestRunnerParsesLearningGoalLifecycleAndRendersGoal(t *testing.T) {
	t.Parallel()

	goal := testLearningGoal(t)
	service := &fakeService{result: app.Result{Goal: &goal}}
	var stdout, stderr bytes.Buffer
	exitCode := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{
		"goal", "set", "--title", "Backend Engineer with Go", "--description", "Production services",
		"--domain", "Software engineering", "--target-outcome", "Build production backend services",
		"--starting-level", "intermediate", "--mastery-threshold", "0.85",
	})
	if exitCode != ExitOK || stderr.Len() != 0 || len(service.commands) != 1 {
		t.Fatalf("goal set exit=%d commands=%+v stderr=%q", exitCode, service.commands, stderr.String())
	}
	command := service.commands[0]
	if command.Action != app.ActionGoal || command.GoalOperation != "set" || command.GoalInput.Title != "Backend Engineer with Go" ||
		command.GoalInput.Domain != "Software engineering" || command.GoalInput.StartingLevel != learning.ExperienceIntermediate ||
		command.GoalInput.MasteryThreshold.Value() != .85 {
		t.Fatalf("goal command = %+v", command)
	}
	for _, want := range []string{"Learning goal", "Title: Backend Engineer with Go", "Domain: Software engineering", "Status: active", "Mastery threshold: 0.85"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("goal output missing %q:\n%s", want, stdout.String())
		}
	}

	service = &fakeService{result: app.Result{Goals: []learning.LearningGoal{goal}}}
	stdout.Reset()
	stderr.Reset()
	if exitCode := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"goal", "show"}); exitCode != ExitOK {
		t.Fatalf("goal show exit=%d stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Learning goals (1)") || !strings.Contains(stdout.String(), "[active]") {
		t.Fatalf("goal history output = %q", stdout.String())
	}
}

func TestRunnerRejectsIncompleteLearningGoalCommands(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"goal", "set", "--title", "Missing fields"},
		{"goal", "show", "--domain", "Invalid here"},
		{"goal", "set", "--title", "X", "--domain", "Y", "--target-outcome", "Z", "--mastery-threshold", "1.1"},
		{"goal", "set", "--title", "X", "--domain", "Y", "--target-outcome", "Z", "--mastery-threshold", "0.49"},
	} {
		var stderr bytes.Buffer
		if exitCode := NewRunner(&fakeService{}, &bytes.Buffer{}, &stderr).Run(context.Background(), args); exitCode != ExitUsage {
			t.Fatalf("Run(%v) exit=%d stderr=%q", args, exitCode, stderr.String())
		}
	}
}

func testLearningGoal(t *testing.T) learning.LearningGoal {
	t.Helper()
	id, _ := learning.NewID("goal.go")
	studentID, _ := learning.NewID("student.primary")
	timestamp, _ := learning.NewTimestamp(time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC))
	threshold, _ := learning.NewMasteryThreshold(.85)
	goal, err := learning.NewLearningGoal(id, studentID, learning.GoalDetails{
		Title: "Backend Engineer with Go", Description: "Production services", Domain: "Software engineering",
		TargetOutcome: "Build production backend services", StartingLevel: learning.ExperienceIntermediate,
	}, threshold, timestamp)
	if err != nil {
		t.Fatal(err)
	}
	goal, err = goal.Activate(timestamp)
	if err != nil {
		t.Fatal(err)
	}
	return goal
}

func TestRunnerParsesProfileEditAndRendersHumanReadableProfile(t *testing.T) {
	t.Parallel()

	student := testProfileStudent(t)
	service := &fakeService{result: app.Result{Profile: &student}}
	var stdout, stderr bytes.Buffer
	runner := NewRunner(service, &stdout, &stderr)
	exitCode := runner.Run(context.Background(), []string{
		"profile", "edit", "--display-name", "Ada", "--experience=intermediate",
		"--language", "es-PE", "--daily-minutes=45", "--weekly-days", "4",
		"--learning-styles", "practice,reflection", "--timezone", "America/Lima",
	})
	if exitCode != ExitOK || stderr.Len() != 0 || len(service.commands) != 1 {
		t.Fatalf("profile edit exit=%d commands=%+v stderr=%q", exitCode, service.commands, stderr.String())
	}
	command := service.commands[0]
	if command.Action != app.ActionProfile || command.ProfileOperation != "edit" || command.ProfileChanges.DisplayName == nil || *command.ProfileChanges.DisplayName != "Ada" ||
		command.ProfileChanges.DailyMinutes == nil || *command.ProfileChanges.DailyMinutes != 45 || command.ProfileChanges.Preferences == nil || len(*command.ProfileChanges.Preferences) != 2 {
		t.Fatalf("profile command = %+v", command)
	}
	for _, expected := range []string{"Learner profile", "Display name: Ada", "General experience: intermediate", "Preferred language: es-PE", "Daily time budget: 45 minutes", "Weekly study target: 4 days", "Learning styles: practice, reflection", "Timezone: America/Lima"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("profile output missing %q:\n%s", expected, stdout.String())
		}
	}
}

func TestRunnerRejectsIncompleteProfileCommands(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"profile", "edit"}, {"profile", "show", "--timezone", "UTC"}} {
		var stderr bytes.Buffer
		if exitCode := NewRunner(&fakeService{}, &bytes.Buffer{}, &stderr).Run(context.Background(), args); exitCode != ExitUsage {
			t.Fatalf("Run(%v) exit=%d stderr=%q", args, exitCode, stderr.String())
		}
	}
}

func testProfileStudent(t *testing.T) learning.Student {
	t.Helper()
	id, _ := learning.NewID("student.primary")
	timestamp, _ := learning.NewTimestamp(time.Date(2026, time.August, 19, 15, 0, 0, 0, time.UTC))
	profile := learning.DefaultStudentProfile()
	profile.DisplayName = "Ada"
	profile.Experience = learning.ExperienceIntermediate
	profile.PreferredLanguage = "es-PE"
	profile.Availability.DailyMinutes = 45
	profile.Availability.WeeklyDaysTarget = 4
	profile.Preferences = []learning.StudyPreference{learning.PreferencePractice, learning.PreferenceReflection}
	profile.Timezone = "America/Lima"
	student, err := learning.NewStudent(id, profile, timestamp)
	if err != nil {
		t.Fatal(err)
	}
	return student
}

func TestRunnerParsesAndRendersUpdateChecks(t *testing.T) {
	t.Parallel()
	service := &fakeService{result: app.Result{Update: &update.Result{
		Status: update.UpdateAvailable, Source: update.SourceCache, Channel: update.Prerelease,
		CurrentVersion: "1.0.0", LatestVersion: "1.1.0-beta.1", ReleaseURL: "https://github.com/mishaaac/kelyro/releases/tag/v1.1.0-beta.1",
	}}}
	var stdout, stderr bytes.Buffer
	exitCode := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"update", "check"})
	if exitCode != ExitOK || stderr.Len() != 0 || len(service.commands) != 1 {
		t.Fatalf("Run(update check) exit=%d commands=%+v stderr=%q", exitCode, service.commands, stderr.String())
	}
	if service.commands[0].UpdateOperation != "check" {
		t.Fatalf("update operation = %q", service.commands[0].UpdateOperation)
	}
	for _, want := range []string{"Update available: 1.0.0 -> 1.1.0-beta.1", "source=cache", "Release:", "signed artifacts and checksums"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("update output = %q, want %q", stdout.String(), want)
		}
	}

	service = &fakeService{result: app.Result{Message: "unused"}}
	stdout.Reset()
	stderr.Reset()
	if exitCode := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"update"}); exitCode != ExitOK {
		t.Fatalf("Run(update) exit=%d stderr=%q", exitCode, stderr.String())
	}
	if service.commands[0].UpdateOperation != "install" {
		t.Fatalf("default update operation = %q", service.commands[0].UpdateOperation)
	}
}

func TestRunnerRendersDevelopmentUpdateCheck(t *testing.T) {
	t.Parallel()
	service := &fakeService{result: app.Result{Update: &update.Result{
		Status: update.Unavailable, Source: update.SourceNone, Channel: update.Stable,
		CurrentVersion: "dev", Detail: "development build",
	}}}
	var stdout, stderr bytes.Buffer

	exitCode := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"update", "check"})
	if exitCode != ExitOK || stderr.Len() != 0 {
		t.Fatalf("Run(update check) exit=%d stderr=%q", exitCode, stderr.String())
	}
	const want = "Update check unavailable (current=dev channel=stable): development build.\n"
	if got := stdout.String(); got != want {
		t.Fatalf("update output = %q, want %q", got, want)
	}
}

func TestRunnerPassesWorkspaceAndAcceptsReservedFlags(t *testing.T) {
	t.Parallel()

	workspacePath := filepath.Join("projects", "learning")
	service := &fakeService{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := NewRunner(service, &stdout, &stderr)

	args := []string{"--no-color", "status", "--verbose", "--workspace", workspacePath}
	if exitCode := runner.Run(context.Background(), args); exitCode != ExitOK {
		t.Fatalf("Run() exit code = %d, want %d; stderr = %q", exitCode, ExitOK, stderr.String())
	}
	if len(service.commands) != 1 {
		t.Fatalf("service calls = %d, want 1", len(service.commands))
	}
	if got := service.commands[0].Workspace; got != workspacePath {
		t.Errorf("workspace = %q, want %q", got, workspacePath)
	}
	if !service.commands[0].Verbose {
		t.Error("verbose flag was not forwarded to the application")
	}
}

func TestRunnerParsesAndRendersPortabilityCommands(t *testing.T) {
	t.Parallel()

	exportService := &fakeService{result: app.Result{Portability: &portability.Report{
		ArchivePath: "/tmp/workspace.tar.gz", Mode: portability.ModeFull, FileCount: 4, TotalSize: 120,
	}}}
	var stdout, stderr bytes.Buffer
	exitCode := NewRunner(exportService, &stdout, &stderr).Run(context.Background(), []string{
		"export", "--full", "--output", "/tmp/workspace.tar.gz",
	})
	if exitCode != ExitOK || len(exportService.commands) != 1 {
		t.Fatalf("Run(export) = %d, commands=%+v, stderr=%q", exitCode, exportService.commands, stderr.String())
	}
	exportCommand := exportService.commands[0]
	if exportCommand.ExportMode != portability.ModeFull || exportCommand.ExportOutput != "/tmp/workspace.tar.gz" {
		t.Fatalf("export command = %+v", exportCommand)
	}
	if !strings.Contains(stdout.String(), "Exported full workspace") || !strings.Contains(stdout.String(), "files=4 bytes=120") {
		t.Fatalf("export stdout = %q", stdout.String())
	}

	importService := &fakeService{result: app.Result{Portability: &portability.Report{
		ArchivePath: "/tmp/workspace.tar.gz", Destination: "/tmp/target", Mode: portability.ModeFull,
		DryRun: true, FileCount: 4, Creates: []string{"LEARNING.md"}, Conflicts: []string{"notes.md"},
	}}}
	stdout.Reset()
	stderr.Reset()
	exitCode = NewRunner(importService, &stdout, &stderr).Run(context.Background(), []string{
		"import", "/tmp/workspace.tar.gz", "--dry-run", "--conflict=overwrite",
	})
	if exitCode != ExitOK || len(importService.commands) != 1 {
		t.Fatalf("Run(import) = %d, commands=%+v, stderr=%q", exitCode, importService.commands, stderr.String())
	}
	importCommand := importService.commands[0]
	if !importCommand.ImportDryRun || importCommand.ImportConflicts != portability.ConflictOverwrite || importCommand.ImportArchive != "/tmp/workspace.tar.gz" {
		t.Fatalf("import command = %+v", importCommand)
	}
	if !strings.Contains(stdout.String(), "Import dry run") || !strings.Contains(stdout.String(), "Conflicts: notes.md") {
		t.Fatalf("import stdout = %q", stdout.String())
	}
}

func TestRunnerRendersAuditTrail(t *testing.T) {
	t.Parallel()
	service := &fakeService{result: app.Result{Audit: []audit.Entry{{
		Timestamp: time.Date(2026, 8, 12, 15, 30, 0, 0, time.UTC),
		Event:     "config.changed", Actor: audit.ActorUser, Subject: "ui.color", AppVersion: "1.2.3",
	}}}}
	var stdout, stderr bytes.Buffer
	if exitCode := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"audit"}); exitCode != ExitOK {
		t.Fatalf("Run() exit code = %d; stderr = %q", exitCode, stderr.String())
	}
	for _, want := range []string{"2026-08-12T15:30:00", "config.changed", "actor=user", "subject=ui.color", "version=1.2.3"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}

func TestRunnerRejectsIncompleteLogsCommand(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	exitCode := NewRunner(&fakeService{}, &bytes.Buffer{}, &stderr).Run(context.Background(), []string{"logs"})
	if exitCode != ExitUsage || !strings.Contains(stderr.String(), "logs requires the path command") {
		t.Fatalf("Run(logs) = %d, stderr %q", exitCode, stderr.String())
	}
}

func TestRunnerExplainsDoctorToolFromMaintainedGuidance(t *testing.T) {
	t.Parallel()

	service := &fakeService{result: app.Result{Guidance: &doctor.Guidance{
		ToolID:           "lazygit",
		DisplayName:      "lazygit",
		Requirement:      doctor.Optional,
		Description:      "A terminal interface for Git repositories.",
		WhyNeeded:        "It can help inspect branches, commits, and diffs, but it is not required.",
		FoundationFirst:  "Kelyro teaches Git with the Git CLI first.",
		Platform:         "linux",
		PlatformGuidance: "Use an installation method maintained by the project.",
		LearnMore:        "https://github.com/jesseduffield/lazygit#installation",
	}}}
	var stdout, stderr bytes.Buffer
	exitCode := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"doctor", "--explain", "lazygit"})
	if exitCode != ExitOK {
		t.Fatalf("Run() exit code = %d; stderr = %q", exitCode, stderr.String())
	}
	if len(service.commands) != 1 || service.commands[0].Action != app.ActionDoctor || service.commands[0].DoctorExplain != "lazygit" {
		t.Fatalf("commands = %#v", service.commands)
	}
	for _, want := range []string{"lazygit — Optional", "What it is:", "Why:", "Foundation first:", "On linux:", "Official documentation:", "Git CLI first", service.result.Guidance.LearnMore} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout = %q, want %q", stdout.String(), want)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRunnerLaunchesInteractiveAdapterForDefaultCommand(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	interactive := &fakeInteractiveRunner{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := NewRunner(service, &stdout, &stderr).WithInteractive(interactive)

	if exitCode := runner.Run(context.Background(), []string{"--workspace", "learning lab", "--no-color"}); exitCode != ExitOK {
		t.Fatalf("Run() exit code = %d; stderr = %q", exitCode, stderr.String())
	}
	if len(service.commands) != 0 {
		t.Errorf("application Execute calls = %d, want TUI to own its service calls", len(service.commands))
	}
	if len(interactive.commands) != 1 {
		t.Fatalf("interactive calls = %d", len(interactive.commands))
	}
	command := interactive.commands[0]
	if command.Action != app.ActionTUI || command.Workspace != "learning lab" {
		t.Errorf("interactive command = %#v", command)
	}
	if command.ConfigOverrides[config.KeyUIColor].String() != "never" {
		t.Errorf("interactive color override = %q", command.ConfigOverrides[config.KeyUIColor].String())
	}
}

func TestRunnerReportsInteractiveFailure(t *testing.T) {
	t.Parallel()

	interactive := &fakeInteractiveRunner{err: errors.New("terminal unavailable")}
	var stderr bytes.Buffer
	runner := NewRunner(&fakeService{}, &bytes.Buffer{}, &stderr).WithInteractive(interactive)
	if exitCode := runner.Run(context.Background(), nil); exitCode != ExitFailure {
		t.Fatalf("Run() exit code = %d, want failure", exitCode)
	}
	if got := stderr.String(); got != "kelyro tui: terminal unavailable\n" {
		t.Errorf("stderr = %q", got)
	}
}

func TestRunnerPassesExplicitNestedInitialization(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := NewRunner(service, &stdout, &stderr)

	if exitCode := runner.Run(context.Background(), []string{"init", "--allow-nested"}); exitCode != ExitOK {
		t.Fatalf("Run() exit code = %d, want %d; stderr = %q", exitCode, ExitOK, stderr.String())
	}
	if len(service.commands) != 1 || !service.commands[0].AllowNested {
		t.Errorf("commands = %#v, want one command with AllowNested", service.commands)
	}
}

func TestRunnerParsesOpenTargets(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		args       []string
		wantTarget string
	}{
		{name: "learning by default", args: []string{"open"}},
		{name: "roadmap", args: []string{"open", "roadmap"}, wantTarget: "roadmap"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := &fakeService{}
			var stderr bytes.Buffer
			runner := NewRunner(service, &bytes.Buffer{}, &stderr)
			if exitCode := runner.Run(context.Background(), test.args); exitCode != ExitOK {
				t.Fatalf("Run() exit code = %d; stderr = %q", exitCode, stderr.String())
			}
			if len(service.commands) != 1 || service.commands[0].OpenTarget != test.wantTarget {
				t.Errorf("commands = %#v, want open target %q", service.commands, test.wantTarget)
			}
		})
	}
}

func TestRunnerParsesConfigCommandsScopesAndOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		wantOperation string
		wantScope     config.Scope
		wantKey       string
		wantValue     string
		wantColor     string
	}{
		{name: "config defaults to show", args: []string{"config"}, wantOperation: "show"},
		{name: "show global", args: []string{"--global", "config", "show"}, wantOperation: "show", wantScope: config.ScopeGlobal},
		{name: "project path", args: []string{"config", "path", "--project"}, wantOperation: "path", wantScope: config.ScopeProject},
		{name: "get", args: []string{"config", "get", "ui.color"}, wantOperation: "get", wantKey: "ui.color"},
		{name: "set", args: []string{"config", "set", "editor.command", "code --wait"}, wantOperation: "set", wantKey: "editor.command", wantValue: "code --wait"},
		{name: "CLI color override", args: []string{"--no-color", "config", "show"}, wantOperation: "show", wantColor: "never"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := &fakeService{}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			runner := NewRunner(service, &stdout, &stderr)

			if exitCode := runner.Run(context.Background(), test.args); exitCode != ExitOK {
				t.Fatalf("Run() exit code = %d; stderr = %q", exitCode, stderr.String())
			}
			if len(service.commands) != 1 {
				t.Fatalf("service calls = %d, want 1", len(service.commands))
			}
			command := service.commands[0]
			if command.ConfigOperation != test.wantOperation || command.ConfigScope != test.wantScope || command.ConfigKey != test.wantKey || command.ConfigValue != test.wantValue {
				t.Errorf("config command = %#v", command)
			}
			if test.wantColor != "" && command.ConfigOverrides[config.KeyUIColor].String() != test.wantColor {
				t.Errorf("ui.color override = %q, want %q", command.ConfigOverrides[config.KeyUIColor].String(), test.wantColor)
			}
		})
	}
}

func TestRunnerParsesAndDispatchesSecretCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		wantOperation string
		wantName      string
	}{
		{name: "status", args: []string{"secrets", "status"}, wantOperation: "status"},
		{name: "delete", args: []string{"secrets", "delete", "openai"}, wantOperation: "delete", wantName: "openai"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := &fakeService{}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			runner := NewRunner(service, &stdout, &stderr)
			if exitCode := runner.Run(context.Background(), test.args); exitCode != ExitOK {
				t.Fatalf("Run() exit code = %d; stderr = %q", exitCode, stderr.String())
			}
			command := service.commands[0]
			if command.SecretOperation != test.wantOperation || command.SecretName != test.wantName {
				t.Fatalf("secret command = %#v", command)
			}
		})
	}
}

func TestRunnerReadsSecretOutsideArgumentsAndOutput(t *testing.T) {
	t.Parallel()

	secret := "sensitive-manual-input"
	service := &fakeService{result: app.Result{Message: "configured"}}
	reader := &fakeSecretReader{value: secret}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := NewRunner(service, &stdout, &stderr).WithSecretReader(reader)

	if exitCode := runner.Run(context.Background(), []string{"secrets", "set", "openai"}); exitCode != ExitOK {
		t.Fatalf("Run() exit code = %d; stderr = %q", exitCode, stderr.String())
	}
	if reader.prompt != "Secret value: " {
		t.Fatalf("secret prompt = %q", reader.prompt)
	}
	if len(service.commands) != 1 || service.commands[0].SecretValue != secret || service.commands[0].SecretName != "openai" {
		t.Fatalf("commands = %#v", service.commands)
	}
	if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
		t.Fatal("CLI output exposed secret")
	}
}

func TestRunnerHelp(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"help"}, {"--help"}, {"init", "--help"}} {
		service := &fakeService{}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		runner := NewRunner(service, &stdout, &stderr)

		if exitCode := runner.Run(context.Background(), args); exitCode != ExitOK {
			t.Fatalf("Run(%q) exit code = %d, want %d", args, exitCode, ExitOK)
		}
		for _, expected := range []string{"Usage:", "Commands:", "init", "doctor", "status", "progress", "roadmap", "today", "--no-color", "--workspace PATH"} {
			if !strings.Contains(stdout.String(), expected) {
				t.Errorf("Run(%q) output does not contain %q", args, expected)
			}
		}
		if len(service.commands) != 0 {
			t.Errorf("Run(%q) dispatched %d service calls, want 0", args, len(service.commands))
		}
		if stderr.Len() != 0 {
			t.Errorf("Run(%q) stderr = %q, want empty", args, stderr.String())
		}
	}
}

func TestRunnerVersion(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"version"}, {"--version"}} {
		service := &fakeService{}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		runner := NewRunner(service, &stdout, &stderr)

		if exitCode := runner.Run(context.Background(), args); exitCode != ExitOK {
			t.Fatalf("Run(%q) exit code = %d, want %d", args, exitCode, ExitOK)
		}
		if got, want := stdout.String(), "kelyro dev (commit unknown, built unknown)\n"; got != want {
			t.Errorf("Run(%q) output = %q, want %q", args, got, want)
		}
		if len(service.commands) != 0 {
			t.Errorf("Run(%q) dispatched %d service calls, want 0", args, len(service.commands))
		}
		if stderr.Len() != 0 {
			t.Errorf("Run(%q) stderr = %q, want empty", args, stderr.String())
		}
	}
}

func TestRunnerRejectsInvalidArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "unknown command", args: []string{"learn"}, message: `unknown command "learn"`},
		{name: "unknown option", args: []string{"--json"}, message: `unknown option "--json"`},
		{name: "missing workspace", args: []string{"--workspace"}, message: "option --workspace requires a path"},
		{name: "empty workspace", args: []string{"--workspace="}, message: "option --workspace requires a path"},
		{name: "extra argument", args: []string{"status", "extra"}, message: "status does not accept positional arguments"},
		{name: "conflicting output modes", args: []string{"--verbose", "--quiet"}, message: "options --verbose and --quiet cannot be combined"},
		{name: "version with command", args: []string{"init", "--version"}, message: "option --version cannot be combined with a command"},
		{name: "nested without init", args: []string{"status", "--allow-nested"}, message: "option --allow-nested requires the init command"},
		{name: "scope conflict", args: []string{"config", "--global", "--project"}, message: "options --global and --project cannot be combined"},
		{name: "scope without config", args: []string{"status", "--global"}, message: "configuration scope options require the config command"},
		{name: "unknown config command", args: []string{"config", "edit"}, message: `unknown config command "edit"`},
		{name: "get missing key", args: []string{"config", "get"}, message: "config get requires exactly one key"},
		{name: "set missing value", args: []string{"config", "set", "ui.color"}, message: "config set requires a key and value"},
		{name: "show extra argument", args: []string{"config", "show", "extra"}, message: "config show does not accept arguments"},
		{name: "secrets missing operation", args: []string{"secrets"}, message: "secrets requires status, set, or delete"},
		{name: "unknown secrets operation", args: []string{"secrets", "get", "openai"}, message: `unknown secrets command "get"`},
		{name: "secrets status extra", args: []string{"secrets", "status", "openai"}, message: "secrets status does not accept arguments"},
		{name: "secrets set missing name", args: []string{"secrets", "set"}, message: "secrets set requires exactly one name"},
		{name: "unknown open artifact", args: []string{"open", "lesson"}, message: "open accepts only the optional roadmap artifact"},
		{name: "too many open artifacts", args: []string{"open", "roadmap", "extra"}, message: "open accepts only the optional roadmap artifact"},
		{name: "explain missing tool", args: []string{"doctor", "--explain"}, message: "option --explain requires a tool id"},
		{name: "explain followed by option", args: []string{"doctor", "--explain", "--quiet"}, message: "option --explain requires a tool id"},
		{name: "explain without doctor", args: []string{"status", "--explain", "git"}, message: "option --explain requires the doctor command"},
		{name: "doctor positional argument", args: []string{"doctor", "git"}, message: "doctor does not accept positional arguments"},
		{name: "backup missing operation", args: []string{"backup"}, message: "backup requires create, list, or restore"},
		{name: "backup unknown operation", args: []string{"backup", "copy"}, message: `unknown backup command "copy"`},
		{name: "backup restore missing id", args: []string{"backup", "restore"}, message: "backup restore requires exactly one id"},
		{name: "yes without restore", args: []string{"backup", "create", "--yes"}, message: "option --yes requires backup restore or setup reset"},
		{name: "full without export", args: []string{"status", "--full"}, message: "option --full requires the export command"},
		{name: "output without export", args: []string{"status", "--output", "archive.tar.gz"}, message: "option --output requires the export command"},
		{name: "export positional", args: []string{"export", "archive.tar.gz"}, message: "export does not accept positional arguments"},
		{name: "import missing archive", args: []string{"import"}, message: "import requires exactly one archive file"},
		{name: "dry-run without import", args: []string{"status", "--dry-run"}, message: "option --dry-run requires the import command"},
		{name: "invalid conflict", args: []string{"import", "archive.tar.gz", "--conflict", "merge"}, message: "option --conflict requires fail, keep, or overwrite"},
		{name: "unknown update operation", args: []string{"update", "now"}, message: "update accepts only the optional check command"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeService{}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			runner := NewRunner(service, &stdout, &stderr)

			if exitCode := runner.Run(context.Background(), test.args); exitCode != ExitUsage {
				t.Fatalf("Run() exit code = %d, want %d", exitCode, ExitUsage)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), "kelyro: "+test.message) {
				t.Errorf("stderr = %q, want error containing %q", stderr.String(), test.message)
			}
			if !strings.Contains(stderr.String(), "Run 'kelyro help' for usage.") {
				t.Errorf("stderr = %q, want usage hint", stderr.String())
			}
			if len(service.commands) != 0 {
				t.Errorf("service calls = %d, want 0", len(service.commands))
			}
		})
	}
}

func TestRunnerReturnsFailureForServiceError(t *testing.T) {
	t.Parallel()

	service := &fakeService{err: errors.New("diagnostic failed")}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := NewRunner(service, &stdout, &stderr)

	if exitCode := runner.Run(context.Background(), []string{"doctor"}); exitCode != ExitFailure {
		t.Fatalf("Run() exit code = %d, want %d", exitCode, ExitFailure)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if got, want := stderr.String(), "kelyro doctor: diagnostic failed\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestRunnerRendersDoctorReportAndFailsOnlyForRequiredChecks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		report   doctor.Report
		wantExit int
	}{
		{
			name: "missing recommended tool remains successful",
			report: doctor.Report{Checks: []doctor.Check{
				{ID: "platform.os", Section: doctor.SectionPlatform, DisplayName: "OS detected", Requirement: doctor.Required, State: doctor.Pass, Detail: "linux"},
				{ID: "tool.git", Section: doctor.SectionDevelopment, DisplayName: "Git", Requirement: doctor.Recommended, State: doctor.Miss, Detail: "not found", WhyNeeded: "Track changes.", LearnMore: "https://git-scm.com/"},
			}},
			wantExit: ExitOK,
		},
		{
			name:     "missing required tool fails",
			report:   doctor.Report{Checks: []doctor.Check{{ID: "tool.docker", Section: doctor.SectionDevelopment, DisplayName: "Docker", Requirement: doctor.Required, State: doctor.Miss, Detail: "not found"}}},
			wantExit: ExitFailure,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := &fakeService{result: app.Result{Diagnostics: &test.report, Failed: test.report.Failed()}}
			var stdout, stderr bytes.Buffer
			exitCode := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"doctor"})
			if exitCode != test.wantExit {
				t.Fatalf("Run() exit code = %d, want %d", exitCode, test.wantExit)
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q", stderr.String())
			}
			for _, text := range []string{test.report.Checks[0].Section, test.report.Checks[0].DisplayName, test.report.Checks[0].Detail} {
				if !strings.Contains(stdout.String(), text) {
					t.Errorf("stdout = %q, want %q", stdout.String(), text)
				}
			}
			if test.wantExit == ExitOK && (!strings.Contains(stdout.String(), "[recommended]") || !strings.Contains(stdout.String(), "Why: Track changes.")) {
				t.Errorf("stdout lacks requirement metadata: %q", stdout.String())
			}
		})
	}
}

func TestRunnerQuietSuppressesSuccessfulOutput(t *testing.T) {
	t.Parallel()

	service := &fakeService{result: app.Result{Message: "hidden"}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := NewRunner(service, &stdout, &stderr)

	if exitCode := runner.Run(context.Background(), []string{"--quiet", "status"}); exitCode != ExitOK {
		t.Fatalf("Run() exit code = %d, want %d", exitCode, ExitOK)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if len(service.commands) != 1 || service.commands[0].Action != app.ActionStatus {
		t.Errorf("commands = %#v, want one status command", service.commands)
	}
}

func TestRunnerParsesAndRendersSourceRegistryCommands(t *testing.T) {
	t.Parallel()
	entry := cliRegistryEntry(t)
	t.Run("list", func(t *testing.T) {
		service := &fakeService{result: app.Result{SourceRegistryEntries: []research.SourceRegistryEntry{entry}}}
		var stdout, stderr bytes.Buffer
		code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"sources", "registry", "list"})
		if code != ExitOK || stderr.Len() != 0 || !strings.Contains(stdout.String(), "registry.blocked-example [blocked]") {
			t.Fatalf("list = code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
		}
		if len(service.commands) != 1 || service.commands[0].Action != app.ActionSources || service.commands[0].SourceRegistryOperation != "list" {
			t.Fatalf("list command = %+v", service.commands)
		}
	})
	t.Run("show", func(t *testing.T) {
		service := &fakeService{result: app.Result{SourceRegistryEntry: &entry}}
		var stdout, stderr bytes.Buffer
		code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"sources", "registry", "show", entry.ID.String()})
		if code != ExitOK || stderr.Len() != 0 || !strings.Contains(stdout.String(), "Status: blocked") || !strings.Contains(stdout.String(), "not evidence") {
			t.Fatalf("show = code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
		}
		if len(service.commands) != 1 || service.commands[0].SourceRegistryID != entry.ID || service.commands[0].SourceRegistryOperation != "show" {
			t.Fatalf("show command = %+v", service.commands)
		}
	})
	t.Run("trace", func(t *testing.T) {
		graph := cliProvenanceGraph(t)
		service := &fakeService{result: app.Result{ProvenanceGraph: &graph}}
		var stdout, stderr bytes.Buffer
		code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"sources", "trace", graph.ClaimID.String()})
		if code != ExitOK || stderr.Len() != 0 || !strings.Contains(stdout.String(), "Claim provenance: claim.cli-trace") || !strings.Contains(stdout.String(), "historical snapshot") {
			t.Fatalf("trace = code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
		}
		if len(service.commands) != 1 || service.commands[0].ProvenanceClaimID != graph.ClaimID || service.commands[0].SourceRegistryOperation != "trace" {
			t.Fatalf("trace command = %+v", service.commands)
		}
	})
	t.Run("stale", func(t *testing.T) {
		last, _ := research.NewTimestamp(time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC))
		dueAt, _ := research.NewTimestamp(time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC))
		score, _ := research.NewFreshnessScore(.4)
		record := researchapp.FreshnessRecord{
			SubjectID: cliID(t, "claim.cli-stale"), State: research.FreshnessStale, Score: score,
			LastVerifiedAt: last, NextVerifyAt: &dueAt, VerificationReason: research.VerificationManualRequest,
			Priority: research.VerificationPriorityCritical, AlgorithmVersion: research.FreshnessAlgorithmV1,
			SchedulingAlgorithmVersion: research.RefreshSchedulingAlgorithmV1,
		}
		service := &fakeService{result: app.Result{StaleSources: []researchapp.FreshnessRecord{record}}}
		var stdout, stderr bytes.Buffer
		code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"sources", "stale"})
		if code != ExitOK || stderr.Len() != 0 || !strings.Contains(stdout.String(), "claim.cli-stale [critical]") || !strings.Contains(stdout.String(), "manual_request") {
			t.Fatalf("stale = code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
		}
		if len(service.commands) != 1 || service.commands[0].SourceRegistryOperation != "stale" {
			t.Fatalf("stale command = %+v", service.commands)
		}
	})
	for _, args := range [][]string{{"sources"}, {"sources", "registry"}, {"sources", "registry", "show"}, {"sources", "registry", "delete", "id"}, {"sources", "trace"}} {
		var stderr bytes.Buffer
		if code := NewRunner(&fakeService{}, &bytes.Buffer{}, &stderr).Run(context.Background(), args); code != ExitUsage {
			t.Fatalf("Run(%v) code = %d, want usage; stderr=%q", args, code, stderr.String())
		}
	}
}

func TestRunnerParsesAndRendersResearchCacheCommands(t *testing.T) {
	t.Parallel()
	t.Run("status", func(t *testing.T) {
		status := researchapp.ResearchCacheStatus{
			AlgorithmVersion: researchapp.ResearchCacheAlgorithmV1,
			TotalEntries:     3, TotalPayloadBytes: 240, StaleEntries: 1, CorruptEntries: 1, CorruptBytes: 12,
			Layers: []researchapp.CacheLayerStatus{
				{Layer: researchapp.CacheLayerDiscovery, Entries: 2, PayloadBytes: 120, StaleEntries: 1},
				{Layer: researchapp.CacheLayerSourceBundle, Entries: 1, PayloadBytes: 120},
			},
		}
		service := &fakeService{result: app.Result{ResearchCacheStatus: &status}}
		var stdout, stderr bytes.Buffer
		code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"research", "cache", "status"})
		if code != ExitOK || stderr.Len() != 0 || !strings.Contains(stdout.String(), "Algorithm: research-cache-v1") ||
			!strings.Contains(stdout.String(), "discovery: 2 entries") || !strings.Contains(stdout.String(), "Corrupt: 1 entries") {
			t.Fatalf("status = code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
		}
		if len(service.commands) != 1 || service.commands[0].Action != app.ActionResearch || service.commands[0].ResearchCacheOperation != "status" {
			t.Fatalf("status command = %+v", service.commands)
		}
	})
	t.Run("clear", func(t *testing.T) {
		cleared := researchapp.ResearchCacheClearResult{RemovedEntries: 4, RemovedBytes: 512}
		service := &fakeService{result: app.Result{ResearchCacheCleared: &cleared}}
		var stdout, stderr bytes.Buffer
		code := NewRunner(service, &stdout, &stderr).Run(context.Background(), []string{"research", "cache", "clear"})
		if code != ExitOK || stderr.Len() != 0 || !strings.Contains(stdout.String(), "Removed: 4 entries, 512 bytes") ||
			!strings.Contains(stdout.String(), "evidence were not modified") {
			t.Fatalf("clear = code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
		}
		if len(service.commands) != 1 || service.commands[0].ResearchCacheOperation != "clear" {
			t.Fatalf("clear command = %+v", service.commands)
		}
	})
	for _, args := range [][]string{{"research"}, {"research", "cache"}, {"research", "cache", "delete"}, {"research", "status"}} {
		var stderr bytes.Buffer
		if code := NewRunner(&fakeService{}, &bytes.Buffer{}, &stderr).Run(context.Background(), args); code != ExitUsage {
			t.Fatalf("Run(%v) code = %d, want usage; stderr=%q", args, code, stderr.String())
		}
	}
}

func cliID(t *testing.T, value string) research.ID {
	t.Helper()
	result, err := research.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func cliProvenanceGraph(t *testing.T) research.ProvenanceGraph {
	t.Helper()
	id := func(value string) research.ID {
		result, err := research.NewID(value)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	claimID, _ := research.NewClaimID("claim.cli-trace")
	claimNode := id(claimID.String())
	at, _ := research.NewTimestamp(time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC))
	recorded, _ := research.NewTimestamp(at.Time().Add(time.Hour))
	request, run, source := id("request.cli-trace"), id("run.cli-trace"), id("source.cli-trace")
	snapshot, evidence := id("snapshot.cli-trace"), id("evidence.cli-trace")
	return research.ProvenanceGraph{
		ID: id("graph.cli-trace"), ClaimID: claimID, RecordedAt: recorded, AlgorithmVersion: research.ProvenanceGraphAlgorithmV1,
		Nodes: []research.ProvenanceNode{
			{ID: request, Kind: research.ProvenanceRequest, Label: "trace request", OccurredAt: at},
			{ID: run, Kind: research.ProvenanceRun, Label: "trace run", OccurredAt: at},
			{ID: source, Kind: research.ProvenanceSource, Label: "manual source", OccurredAt: at},
			{ID: snapshot, Kind: research.ProvenanceSnapshot, Label: "historical snapshot", OccurredAt: at, ToolVersion: "fetch/v1"},
			{ID: evidence, Kind: research.ProvenanceEvidence, Label: "section 1", OccurredAt: at, ToolVersion: "extract/v1"},
			{ID: claimNode, Kind: research.ProvenanceClaim, Label: "traceable claim", OccurredAt: at},
		},
		Edges: []research.ProvenanceEdge{
			{From: request, To: run}, {From: run, To: source}, {From: source, To: snapshot},
			{From: snapshot, To: evidence}, {From: evidence, To: claimNode},
		},
	}
}

func cliRegistryEntry(t *testing.T) research.SourceRegistryEntry {
	t.Helper()
	id, _ := research.NewID("registry.blocked-example")
	domain, _ := research.NewCanonicalDomain("blocked.example")
	at, _ := research.NewTimestamp(time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC))
	return research.SourceRegistryEntry{
		ID: id, Organization: "Blocked Example", CanonicalDomains: []research.CanonicalDomain{domain},
		SourceKinds:     []research.SourceKind{research.SourceOther},
		AuthorityHints:  []research.RegistryAuthorityHint{{SourceKind: research.SourceOther, Tier: research.AuthorityTierE, Reason: "Explicitly blocked fixture."}},
		ResearchDomains: []string{"*"}, TopicPatterns: []string{"*"}, Notes: "Do not use.",
		Status: research.RegistryBlocked, AddedAt: at, LastReviewedAt: at,
	}
}

type fakeService struct {
	commands []app.Command
	result   app.Result
	err      error
}

type fakeSecretReader struct {
	value  string
	err    error
	prompt string
}

type fakeInteractiveRunner struct {
	commands []app.Command
	err      error
}

func (runner *fakeInteractiveRunner) Run(_ context.Context, command app.Command) error {
	runner.commands = append(runner.commands, command)
	return runner.err
}

func (reader *fakeSecretReader) ReadSecret(prompt string) (string, error) {
	reader.prompt = prompt
	return reader.value, reader.err
}

func (service *fakeService) Execute(_ context.Context, command app.Command) (app.Result, error) {
	service.commands = append(service.commands, command)
	return service.result, service.err
}
