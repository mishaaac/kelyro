package markdown

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
)

func TestGenerateProgressMatchesGoldenDocuments(t *testing.T) {
	t.Parallel()
	view := progressGoldenView(t)
	documents, err := GenerateProgress(view)
	if err != nil {
		t.Fatalf("GenerateProgress() error = %v", err)
	}
	if len(documents) != 3 {
		t.Fatalf("GenerateProgress() documents = %d, want 3", len(documents))
	}

	goldens := []string{"LEARNING.progress.md.golden", "ROADMAP.progress.md.golden", "PROGRESS.progress.md.golden"}
	versions := []string{LearningProgressTemplateVersion, RoadmapProgressTemplateVersion, ProgressTemplateVersion}
	for index, document := range documents {
		want, readErr := os.ReadFile(filepath.Join("testdata", goldens[index]))
		if readErr != nil {
			t.Fatalf("ReadFile(%q): %v", goldens[index], readErr)
		}
		if !bytes.Equal(document.Content, want) {
			t.Errorf("%s content:\n%s\nwant golden:\n%s", document.Path, document.Content, want)
		}
		if document.TemplateVersion != versions[index] {
			t.Errorf("%s version = %q, want %q", document.Path, document.TemplateVersion, versions[index])
		}
		if !utf8.Valid(document.Content) || bytes.Contains(document.Content, []byte{'\r'}) || !bytes.HasSuffix(document.Content, []byte{'\n'}) {
			t.Errorf("%s is not canonical LF-terminated UTF-8", document.Path)
		}
	}
}

func TestGenerateProgressKeepsInternalIDsAndMarkdownInjectionOut(t *testing.T) {
	t.Parallel()
	view := progressGoldenView(t)
	view.Goal.Title = "Backend\n# injected"
	documents, err := GenerateProgress(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, document := range documents {
		for _, internal := range []string{"student.private", "goal.private", "instance.private", "concept.private"} {
			if bytes.Contains(document.Content, []byte(internal)) {
				t.Errorf("%s contains internal id %q:\n%s", document.Path, internal, document.Content)
			}
		}
	}
	if !bytes.Contains(documents[0].Content, []byte("# Backend \\# injected\n")) {
		t.Fatalf("goal title was not normalized and escaped:\n%s", documents[0].Content)
	}
}

func TestGenerateProgressRendersIncompleteSetupWithoutPrivateState(t *testing.T) {
	t.Parallel()
	documents, err := GenerateProgress(learningapp.ProgressDashboard{ReadModelVersion: learningapp.ProgressDashboardReadModelVersion})
	if err != nil {
		t.Fatal(err)
	}
	for _, document := range documents {
		if !bytes.Contains(document.Content, []byte("kelyro setup status")) {
			t.Errorf("%s lacks actionable empty state:\n%s", document.Path, document.Content)
		}
	}
}

func TestGenerateProgressRejectsUnknownReadModel(t *testing.T) {
	t.Parallel()
	if _, err := GenerateProgress(learningapp.ProgressDashboard{ReadModelVersion: "future/v2"}); err == nil {
		t.Fatal("GenerateProgress() error = nil, want unsupported read model error")
	}
}

func progressGoldenView(t *testing.T) learningapp.ProgressDashboard {
	t.Helper()
	id := func(value string) learning.ID {
		result, err := learning.NewID(value)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	threshold, _ := learning.NewMasteryThreshold(.85)
	requirement, _ := learning.MasteryRequirementFromThreshold(threshold)
	mastery, _ := learning.NewMasteryScore(.4)
	average, _ := learning.NewMasteryScore(.65)
	generatedAt, _ := learning.NewTimestamp(time.Date(2026, 8, 21, 15, 30, 0, 0, time.UTC))
	currentID := id("concept.private")
	phaseID := id("phase.private")
	goal := &learning.LearningGoal{ID: id("goal.private"), StudentID: id("student.private"), Title: "Backend Engineering with Go", MasteryThreshold: threshold}
	return learningapp.ProgressDashboard{
		StudentID: id("student.private"), GeneratedAt: generatedAt, Timezone: "America/Lima", Goal: goal,
		Curriculum: &learningapp.DashboardCurriculum{Instance: learning.CurriculumInstance{ID: id("instance.private")}, ConceptsTotal: 2},
		Current: &learningapp.DashboardLocation{
			Phase: learningapp.DashboardNode{Title: "Foundations"}, Module: learningapp.DashboardNode{Title: "Go basics"},
			Lesson: learningapp.DashboardNode{Title: "Variables"}, Topic: learningapp.DashboardNode{Title: "Initialization"},
			Concept: learningapp.DashboardNode{ID: currentID, Title: "Short declarations"},
		},
		OverallProgress: learningapp.DashboardOverallProgress{
			ConceptsTotal: learning.AnalyticsCountMetric{Value: 2}, ConceptsIntroduced: learning.AnalyticsCountMetric{Value: 2},
			ConceptsLearning: learning.AnalyticsCountMetric{Value: 1}, ConceptsMastered: learning.AnalyticsCountMetric{Value: 1},
			Completion: learning.AnalyticsRateMetric{Value: 50},
		},
		Mastery:            learningapp.DashboardMasterySummary{AverageKnown: learning.AnalyticsScoreMetric{Value: &average}},
		MasteryRequirement: learning.ResolvedMasteryThreshold{Requirement: requirement, PolicyVersion: learning.MasteryThresholdPolicyVersion},
		ReviewsDue:         learning.AnalyticsCountMetric{Value: 2},
		TodayPlan:          &learning.DailyPlan{Items: []learning.DailyPlanItem{{Role: learning.DailyPlanRoleNewLearning, ConceptIDs: []learning.ID{currentID}, EstimatedMinutes: 25}}},
		StudyTime: learning.AnalyticsTime{
			Today: learning.AnalyticsDurationMetric{Value: 25 * time.Minute}, Week: learning.AnalyticsDurationMetric{Value: 4*time.Hour + 12*time.Minute},
			Month: learning.AnalyticsDurationMetric{Value: 9 * time.Hour}, Total: learning.AnalyticsDurationMetric{Value: 21*time.Hour + 5*time.Minute},
		},
		Streak: learning.AnalyticsActivity{
			CurrentStreak: learning.AnalyticsCountMetric{Value: 6}, LongestStreak: learning.AnalyticsCountMetric{Value: 9}, ActiveDays: learning.AnalyticsCountMetric{Value: 14},
		},
		RecentMilestone: &learning.Milestone{Name: "First concept mastered"},
		Roadmap: []learningapp.DashboardRoadmapNode{
			{ID: phaseID, Type: learning.CurriculumNodePhase, Title: "Foundations", Depth: 0},
			{ID: currentID, ParentID: &phaseID, Type: learning.CurriculumNodeConcept, Title: "Short declarations", Depth: 1, Status: learningapp.DashboardRoadmapCurrent, Mastery: &mastery},
			{ID: id("concept.locked"), ParentID: &phaseID, Type: learning.CurriculumNodeConcept, Title: "Functions", Depth: 1, Status: learningapp.DashboardRoadmapLocked, LockReasons: []string{"Master Short declarations first."}},
		},
		ReadModelVersion: learningapp.ProgressDashboardReadModelVersion,
	}
}
