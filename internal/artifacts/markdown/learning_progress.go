package markdown

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
)

const (
	ProgressCreator                 = "student-learning-progress"
	LearningProgressTemplateVersion = "student-learning-learning/v1"
	RoadmapProgressTemplateVersion  = "student-learning-roadmap/v1"
	ProgressTemplateVersion         = "student-learning-progress/v1"
)

// GenerateProgress renders human-readable snapshots from the shared Student
// Core read model. The returned Markdown is a view; it never becomes an input
// to learning policy or persistence.
func GenerateProgress(view learningapp.ProgressDashboard) ([]Document, error) {
	if view.ReadModelVersion != learningapp.ProgressDashboardReadModelVersion {
		return nil, fmt.Errorf("progress dashboard read model %q is unsupported", view.ReadModelVersion)
	}

	return []Document{
		{Path: "LEARNING.md", Content: renderLearning(view), TemplateVersion: LearningProgressTemplateVersion},
		{Path: filepath.Join("00-roadmap", "ROADMAP.md"), Content: renderRoadmap(view), TemplateVersion: RoadmapProgressTemplateVersion},
		{Path: filepath.Join("00-roadmap", "PROGRESS.md"), Content: renderProgress(view), TemplateVersion: ProgressTemplateVersion},
	}, nil
}

func renderLearning(view learningapp.ProgressDashboard) []byte {
	var output bytes.Buffer
	if view.Goal == nil {
		output.WriteString("# Kelyro Learning\n\n")
		output.WriteString("No active learning goal. Run `kelyro setup status` to continue setup.\n")
		return output.Bytes()
	}

	fmt.Fprintf(&output, "# %s\n\n", markdownText(view.Goal.Title))
	output.WriteString("## Current\n\n")
	if view.Current == nil {
		output.WriteString("No active curriculum location.\n")
	} else {
		parts := []string{view.Current.Phase.Title, view.Current.Module.Title, view.Current.Lesson.Title, view.Current.Topic.Title, view.Current.Concept.Title}
		for index := range parts {
			parts[index] = markdownText(parts[index])
		}
		output.WriteString(strings.Join(parts, " / ") + "\n")
	}

	output.WriteString("\n## Today\n\n")
	renderToday(&output, view)

	output.WriteString("\n## Mastery\n\n")
	fmt.Fprintf(&output, "Required: %.0f%%\n\n", masteryThreshold(view)*100)
	fmt.Fprintf(&output, "Mastered: %d of %d concepts\n\n", view.OverallProgress.ConceptsMastered.Value, view.OverallProgress.ConceptsTotal.Value)
	if view.Mastery.AverageKnown.Value == nil {
		output.WriteString("Average for known concepts: unknown\n")
	} else {
		fmt.Fprintf(&output, "Average for known concepts: %.0f%%\n", view.Mastery.AverageKnown.Value.Value()*100)
	}

	output.WriteString("\n## Reviews\n\n")
	fmt.Fprintf(&output, "%d due\n", view.ReviewsDue.Value)
	return output.Bytes()
}

func renderToday(output *bytes.Buffer, view learningapp.ProgressDashboard) {
	if view.TodayPlan == nil {
		output.WriteString("No daily plan is available yet.\n")
		return
	}
	if len(view.TodayPlan.Items) == 0 {
		output.WriteString("Nothing urgent today.\n")
		return
	}
	for _, item := range view.TodayPlan.Items {
		title := "Learning activity"
		if len(item.ConceptIDs) > 0 {
			title = roadmapTitle(view, item.ConceptIDs[0])
		}
		role := strings.ReplaceAll(string(item.Role), "_", " ")
		fmt.Fprintf(output, "- %s: %s (%d min)\n", markdownText(role), markdownText(title), item.EstimatedMinutes)
	}
}

func renderRoadmap(view learningapp.ProgressDashboard) []byte {
	var output bytes.Buffer
	output.WriteString("# Roadmap\n")
	if view.Goal != nil {
		fmt.Fprintf(&output, "\nGoal: %s\n", markdownText(view.Goal.Title))
	}
	if view.Curriculum == nil || len(view.Roadmap) == 0 {
		output.WriteString("\nNo active curriculum. Run `kelyro setup status` to continue setup.\n")
		return output.Bytes()
	}

	output.WriteByte('\n')
	for _, node := range view.Roadmap {
		indent := strings.Repeat("  ", node.Depth)
		if node.Type != learning.CurriculumNodeConcept {
			label := strings.ReplaceAll(string(node.Type), "_", " ")
			fmt.Fprintf(&output, "%s- %s: %s\n", indent, markdownText(label), markdownText(node.Title))
			continue
		}
		status := strings.ReplaceAll(string(node.Status), "_", " ")
		fmt.Fprintf(&output, "%s- %s [%s]", indent, markdownText(node.Title), markdownText(status))
		if node.Mastery != nil {
			fmt.Fprintf(&output, " — %.0f%% mastery", node.Mastery.Value()*100)
		}
		output.WriteByte('\n')
		for _, reason := range node.LockReasons {
			fmt.Fprintf(&output, "%s  - Why: %s\n", indent, markdownText(reason))
		}
	}
	return output.Bytes()
}

func renderProgress(view learningapp.ProgressDashboard) []byte {
	var output bytes.Buffer
	output.WriteString("# Progress\n")
	if view.Goal == nil {
		output.WriteString("\nNo active learning goal. Run `kelyro setup status` to continue setup.\n")
		return output.Bytes()
	}

	fmt.Fprintf(&output, "\nGoal: %s\n", markdownText(view.Goal.Title))
	if !view.GeneratedAt.Time().IsZero() {
		generated := view.GeneratedAt.Time()
		if location, err := time.LoadLocation(view.Timezone); err == nil {
			generated = generated.In(location)
		}
		fmt.Fprintf(&output, "Snapshot: %s", generated.Format("2006-01-02 15:04"))
		if view.Timezone != "" {
			fmt.Fprintf(&output, " (%s)", markdownText(view.Timezone))
		}
		output.WriteByte('\n')
	}

	output.WriteString("\n## Study time\n\n")
	fmt.Fprintf(&output, "- Today: %s\n", duration(view.StudyTime.Today.Value))
	fmt.Fprintf(&output, "- This week: %s\n", duration(view.StudyTime.Week.Value))
	fmt.Fprintf(&output, "- This month: %s\n", duration(view.StudyTime.Month.Value))
	fmt.Fprintf(&output, "- Total: %s\n", duration(view.StudyTime.Total.Value))

	output.WriteString("\n## Concepts\n\n")
	fmt.Fprintf(&output, "- Mastered: %d of %d\n", view.OverallProgress.ConceptsMastered.Value, view.OverallProgress.ConceptsTotal.Value)
	fmt.Fprintf(&output, "- Learning: %d\n", view.OverallProgress.ConceptsLearning.Value)
	fmt.Fprintf(&output, "- Introduced: %d\n", view.OverallProgress.ConceptsIntroduced.Value)
	fmt.Fprintf(&output, "- Completion: %.0f%%\n", view.OverallProgress.Completion.Value)

	output.WriteString("\n## Reviews\n\n")
	fmt.Fprintf(&output, "- Due: %d\n", view.ReviewsDue.Value)

	output.WriteString("\n## Streak\n\n")
	fmt.Fprintf(&output, "- Current: %d %s\n", view.Streak.CurrentStreak.Value, dayLabel(view.Streak.CurrentStreak.Value))
	fmt.Fprintf(&output, "- Longest: %d %s\n", view.Streak.LongestStreak.Value, dayLabel(view.Streak.LongestStreak.Value))
	fmt.Fprintf(&output, "- Active days: %d\n", view.Streak.ActiveDays.Value)

	output.WriteString("\n## Recent milestones\n\n")
	if view.RecentMilestone == nil {
		output.WriteString("No milestones yet.\n")
	} else {
		fmt.Fprintf(&output, "- %s\n", markdownText(view.RecentMilestone.Name))
	}
	return output.Bytes()
}

func masteryThreshold(view learningapp.ProgressDashboard) float64 {
	if view.MasteryRequirement.PolicyVersion == learning.MasteryThresholdPolicyVersion {
		return view.MasteryRequirement.Requirement.Threshold.Value()
	}
	if view.Goal != nil {
		return view.Goal.MasteryThreshold.Value()
	}
	return 0
}

func roadmapTitle(view learningapp.ProgressDashboard, id learning.ID) string {
	for _, node := range view.Roadmap {
		if node.ID == id {
			return node.Title
		}
	}
	return "Learning activity"
}

func duration(value time.Duration) string {
	minutes := int(value / time.Minute)
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh%02dm", minutes/60, minutes%60)
}

func dayLabel(value int) string {
	if value == 1 {
		return "day"
	}
	return "days"
}

func markdownText(value string) string {
	normalized := singleLine(value)
	replacer := strings.NewReplacer(
		"\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]",
		"<", "\\<", ">", "\\>", "#", "\\#", "|", "\\|",
	)
	return replacer.Replace(normalized)
}
