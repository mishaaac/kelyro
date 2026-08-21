package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
)

func (model Model) View() string {
	width := model.contentWidth()
	var lines []string
	switch {
	case model.loading:
		lines = []string{model.styles.title.Render("Kelyro"), "", "Loading Foundation state...", "", "[q] Quit"}
	case model.loadErr != nil:
		lines = []string{model.styles.title.Render("Kelyro"), "", model.styles.failure.Render("Could not load workspace"), ""}
		lines = append(lines, wrapText(model.loadErr.Error(), width)...)
		lines = append(lines, "", "[r/Enter] Retry   [q] Quit")
	default:
		switch model.screen {
		case screenDoctor:
			lines = model.doctorView(width)
		case screenConfig:
			lines = model.configView(width)
		case screenRoadmap:
			lines = model.roadmapView(width)
		case screenToday:
			lines = model.todayView(width)
		case screenProgress:
			lines = model.progressView(width)
		case screenConcept:
			lines = model.conceptView(width)
		case screenReviews:
			lines = model.reviewsView(width)
		case screenHistory:
			lines = model.historyView(width)
		case screenGoal:
			lines = model.goalView(width)
		case screenProfile:
			lines = model.profileView(width)
		case screenStreak:
			lines = model.streakView(width)
		case screenOnboarding:
			lines = model.onboardingView(width)
		default:
			lines = model.homeView(width)
		}
	}
	return model.frame(lines, width)
}

func (model Model) homeView(width int) []string {
	lines := []string{
		model.styles.title.Render("Kelyro"),
		truncate("Workspace: "+model.snapshot.WorkspaceName, width),
	}
	if !model.snapshot.LearningPath {
		lines = append(lines, "", model.styles.muted.Render("No learning path yet."), "")
		lines = append(lines, wrapText("Complete setup to create your learner profile, goal, and curriculum.", width)...)
	} else if model.dashboardErr != nil {
		lines = append(lines, "", model.styles.failure.Render("Could not load learning dashboard"))
		lines = append(lines, wrapText(model.dashboardErr.Error(), width)...)
	} else if model.dashboard.Goal == nil {
		lines = append(lines, "", model.styles.muted.Render("No active learning goal."))
	} else {
		lines = append(lines, "", model.styles.heading.Render(truncate(model.dashboard.Goal.Title, width)), "")
		lines = append(lines, "Progress")
		lines = append(lines, progressBar(model.dashboard.OverallProgress.Completion.Value, width))
		lines = append(lines, "", model.styles.heading.Render("Today"))
		lines = append(lines, fmt.Sprintf("Reviews due: %d", model.dashboard.ReviewsDue.Value))
		next := dashboardNextTitle(model.dashboard)
		if next == "" {
			next = "Nothing urgent"
		}
		lines = append(lines, truncate("Next: "+next, width))
		lines = append(lines, "", fmt.Sprintf("Mastery required: %.0f%%", dashboardMasteryThreshold(model.dashboard)*100))
		lines = append(lines, fmt.Sprintf("Streak: %d %s", model.dashboard.Streak.CurrentStreak.Value, streakDayWord(model.dashboard.Streak.CurrentStreak.Value)))
		lines = append(lines, "Study this week: "+formatStudyDuration(model.dashboard.StudyTime.Week.Value))
	}
	if model.dashboardLoading {
		lines = append(lines, "", model.styles.muted.Render("Refreshing learning progress..."))
	}
	if len(model.milestones) > 0 {
		lines = append(lines, "", model.styles.success.Render("Milestone unlocked"))
		for _, achievement := range model.milestones {
			lines = append(lines, truncate(achievement.Name, width))
		}
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[Enter] Today", "[r] Roadmap", "[p] Progress", "[c] Concept", "[v] Reviews", "[h] History", "[g] Goal", "[o] Profile", "[f] Refresh", "[s] Setup", "[d] Doctor", "[C] Config", "[k] Streak", "[q] Quit")...)
	return lines
}

func (model Model) onboardingView(width int) []string {
	lines := []string{model.styles.title.Render("Kelyro Setup"), ""}
	if model.onboardingLoading && model.setup.Setup.Status == "" {
		return append(lines, "Loading learner setup...", "", "[Ctrl+C] Quit")
	}
	if model.onboardingErr != nil {
		lines = append(lines, model.styles.failure.Render("Could not update setup"))
		lines = append(lines, wrapText(model.onboardingErr.Error(), width)...)
		lines = append(lines, "")
	}
	if model.setup.Setup.Status == learning.SetupCompleted {
		lines = append(lines, model.styles.success.Render("Setup complete."), "")
		lines = append(lines, "Your learner profile, goal, curriculum, and initial concept state are ready.", "", "[Enter/Esc] Home   [Ctrl+C] Quit")
		return lines
	}
	if model.setup.Diagnostic != nil {
		return model.diagnosticSetupView(lines, width)
	}
	switch model.onboarding.Interview.Status {
	case learning.OnboardingCancelled:
		lines = append(lines, "Setup cancelled. Restart setup when you are ready.", "", "[Enter/Esc] Home   [Ctrl+C] Quit")
		return lines
	}
	if model.onboarding.Position > 0 {
		lines = append(lines, fmt.Sprintf("Step %d of %d", model.onboarding.Position, model.onboarding.Total), "")
	}
	question := model.onboarding.Question
	lines = append(lines, wrapText(question.Prompt, width)...)
	lines = append(lines, "")
	switch question.Kind {
	case learning.OnboardingTextQuestion:
		value := model.onboardingInput
		if value == "" {
			value = model.styles.muted.Render("type your answer")
		}
		lines = append(lines, truncate("> "+value, width))
	case learning.OnboardingChoiceQuestion:
		for index, option := range question.Options {
			prefix := "  "
			if index == model.onboardingCursor {
				prefix = "> "
			}
			lines = append(lines, truncate(prefix+option.Label, width))
		}
	case learning.OnboardingReviewQuestion, learning.OnboardingConfirmQuestion:
		lines = append(lines, onboardingSummaryLines(model.onboarding.Interview.Answers, width)...)
		if question.Kind == learning.OnboardingConfirmQuestion {
			lines = append(lines, "", "> Confirm setup")
		}
	}
	if model.onboardingLoading {
		lines = append(lines, "", model.styles.muted.Render("Saving..."))
	}
	lines = append(lines, "")
	primary := "[Enter] Continue"
	if question.Kind == learning.OnboardingConfirmQuestion {
		primary = "[Enter] Confirm"
	}
	lines = append(lines, shortcutLines(width, primary, "[Ctrl+B] Back", "[Esc] Save & leave", "[Ctrl+X] Cancel", "[Ctrl+C] Quit")...)
	return lines
}

func (model Model) diagnosticSetupView(lines []string, width int) []string {
	view := model.setup.Diagnostic
	if view.Item == nil {
		return append(lines, "Finalizing learner setup...")
	}
	item := view.Item
	lines = append(lines, model.styles.heading.Render("Optional initial diagnostic"), "")
	lines = append(lines, wrapText(item.Prompt, width)...)
	lines = append(lines, "")
	if item.Kind == learning.DiagnosticShortAnswer {
		value := model.onboardingInput
		if value == "" {
			value = model.styles.muted.Render("type your answer")
		}
		lines = append(lines, truncate("> "+value, width))
	} else {
		for index, option := range item.Options {
			prefix := "  "
			if index == model.onboardingCursor {
				prefix = "> "
			}
			if item.Kind == learning.DiagnosticMultipleChoice {
				mark := "[ ] "
				if model.diagnosticAnswers[index] {
					mark = "[x] "
				}
				prefix += mark
			}
			lines = append(lines, truncate(prefix+option.Label, width))
		}
	}
	if model.onboardingLoading {
		lines = append(lines, "", model.styles.muted.Render("Saving..."))
	}
	lines = append(lines, "")
	primary := "[Enter] Continue"
	if item.Kind == learning.DiagnosticMultipleChoice {
		primary = "[Space] Select  [Enter] Continue"
	}
	lines = append(lines, shortcutLines(width, primary, "[x] Skip diagnostic", "[Ctrl+C] Quit")...)
	return lines
}

func onboardingSummaryLines(answers map[string]string, width int) []string {
	items := []struct{ label, question string }{
		{"Name", learningapp.OnboardingDisplayNameQuestion},
		{"Goal", learningapp.OnboardingGoalTitleQuestion},
		{"Domain", learningapp.OnboardingGoalDomainQuestion},
		{"Outcome", learningapp.OnboardingGoalOutcomeQuestion},
		{"General experience", learningapp.OnboardingBackgroundQuestion},
		{"Subject experience", learningapp.OnboardingPriorExperienceQuestion},
		{"Daily time", learningapp.OnboardingDailyMinutesQuestion},
		{"Weekly days", learningapp.OnboardingWeeklyDaysQuestion},
		{"Study preference", learningapp.OnboardingStudyPreferenceQuestion},
		{"Required mastery", learningapp.OnboardingMasteryStrictnessQuestion},
		{"Diagnostic", learningapp.OnboardingDiagnosticOptInQuestion},
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		value := answers[item.question]
		if value == "" {
			value = "<not set>"
		}
		value = onboardingAnswerLabel(item.question, value)
		lines = append(lines, truncate(item.label+": "+value, width))
	}
	return lines
}

func onboardingAnswerLabel(question, value string) string {
	switch question {
	case learningapp.OnboardingDailyMinutesQuestion:
		return value + " minutes"
	case learningapp.OnboardingWeeklyDaysQuestion:
		return value + " days"
	case learningapp.OnboardingMasteryStrictnessQuestion:
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return fmt.Sprintf("%.0f%%", parsed*100)
		}
	case learningapp.OnboardingDiagnosticOptInQuestion:
		if value == "yes" {
			return "offer after setup"
		}
		return "skip"
	}
	return strings.ReplaceAll(value, "_", " ")
}

func (model Model) profileView(width int) []string {
	lines := []string{model.styles.title.Render("Learner profile"), ""}
	switch {
	case model.profileLoading:
		lines = append(lines, "Loading profile...")
	case model.profileErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load profile"))
		lines = append(lines, wrapText(model.profileErr.Error(), width)...)
	default:
		displayName := model.profile.Profile.DisplayName
		if displayName == "" {
			displayName = "<not set>"
		}
		preferences := make([]string, len(model.profile.Profile.Preferences))
		for index, preference := range model.profile.Profile.Preferences {
			preferences[index] = string(preference)
		}
		styles := strings.Join(preferences, ", ")
		if styles == "" {
			styles = "<none>"
		}
		values := []string{
			"Display name: " + displayName,
			"General experience: " + string(model.profile.Profile.Experience),
			"Preferred language: " + model.profile.Profile.PreferredLanguage,
			fmt.Sprintf("Daily time budget: %d minutes", model.profile.Profile.Availability.DailyMinutes),
			fmt.Sprintf("Weekly study target: %d days", model.profile.Profile.Availability.WeeklyDaysTarget),
			"Learning styles: " + styles,
			"Timezone: " + model.profile.Profile.Timezone,
		}
		for _, value := range values {
			lines = append(lines, truncate(value, width))
		}
		lines = append(lines, "")
		for _, line := range wrapText("Edit with `kelyro profile edit`.", width) {
			lines = append(lines, model.styles.muted.Render(line))
		}
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[r] Refresh", "[Esc/h] Home", "[q] Quit")...)
	return lines
}

func (model Model) streakView(width int) []string {
	lines := []string{model.styles.title.Render("Study consistency"), ""}
	switch {
	case model.streakLoading:
		lines = append(lines, "Calculating streak from study history...")
	case model.streakErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not calculate streak"))
		lines = append(lines, wrapText(model.streakErr.Error(), width)...)
	default:
		lastActive := "none yet"
		if model.streak.LastActiveLocalDate != nil {
			lastActive = model.streak.LastActiveLocalDate.String()
		}
		values := []string{
			fmt.Sprintf("Streak: %d %s", model.streak.CurrentDays, streakDayWord(model.streak.CurrentDays)),
			fmt.Sprintf("Longest: %d %s", model.streak.LongestDays, streakDayWord(model.streak.LongestDays)),
			fmt.Sprintf("Total active days: %d", model.streak.TotalActiveDays),
			"Last active date: " + lastActive,
			"Timezone: " + model.streak.Timezone,
		}
		for _, value := range values {
			lines = append(lines, truncate(value, width))
		}
		lines = append(lines, "")
		for _, line := range wrapText("Consistency is informational; it does not change mastery or block learning.", width) {
			lines = append(lines, model.styles.muted.Render(line))
		}
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[r] Refresh", "[Esc/h] Home", "[q] Quit")...)
	return lines
}

func streakDayWord(days int) string {
	if days == 1 {
		return "day"
	}
	return "days"
}

func (model Model) doctorView(width int) []string {
	lines := []string{model.styles.title.Render("Doctor"), truncate("Workspace: "+model.snapshot.WorkspaceRoot, width), ""}
	if len(model.snapshot.Diagnostics.Checks) > 0 {
		lines = append(lines, diagnosticLines(model.snapshot.Diagnostics, model.styles, width)...)
	} else {
		lines = append(lines, statusLines(model.snapshot.Checks, true, model.styles, width)...)
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[r] Refresh", "[Esc/h] Home", "[q] Quit")...)
	return lines
}

func (model Model) configView(width int) []string {
	lines := []string{model.styles.title.Render("Config")}
	for _, line := range wrapText("Enter toggles the settings supported by this minimal wizard.", width) {
		lines = append(lines, model.styles.muted.Render(line))
	}
	lines = append(lines, "")
	keys := config.CommonKeys()
	for index, key := range keys {
		marker := "  "
		lineStyle := model.styles.muted
		if index == model.configCursor {
			marker = "> "
			lineStyle = model.styles.selected
		}
		value := "<unset>"
		if configured, ok := model.snapshot.Settings[key]; ok {
			value = configured.String()
			if _, stringValue := configured.StringField(); stringValue {
				value = fmt.Sprintf("%q", value)
			}
		}
		lines = append(lines, lineStyle.Render(truncate(marker+key+" = "+value, width)))
	}
	if model.notice != "" {
		lines = append(lines, "")
		lines = append(lines, wrapText(model.notice, width)...)
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[↑/↓] Select", "[Enter] Change", "[Esc/h] Home", "[q] Quit")...)
	return lines
}

func (model Model) roadmapView(width int) []string {
	lines := []string{model.styles.title.Render("Roadmap"), ""}
	switch {
	case model.dashboardLoading && model.dashboard.Curriculum == nil:
		lines = append(lines, "Loading roadmap...")
	case model.dashboardErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load roadmap"))
		lines = append(lines, wrapText(model.dashboardErr.Error(), width)...)
	case model.dashboard.Curriculum == nil:
		lines = append(lines, model.styles.muted.Render("No active curriculum."), "")
		lines = append(lines, wrapText("Complete setup or activate a learning goal to create a roadmap.", width)...)
	default:
		for _, node := range model.dashboard.Roadmap {
			indent := strings.Repeat("  ", node.Depth)
			if node.Type != learning.CurriculumNodeConcept {
				label := strings.ToUpper(string(node.Type)[:1]) + string(node.Type)[1:] + ": " + node.Title
				lines = append(lines, truncate(indent+label, width))
				continue
			}
			status := roadmapStatusLabel(node.Status)
			lines = append(lines, truncate(indent+"- "+node.Title+" ["+status+"]", width))
			for _, reason := range node.LockReasons {
				lines = append(lines, wrapText(indent+"  Why: "+reason, width)...)
			}
		}
		lines = append(lines, "")
		for _, line := range wrapText("Legend: mastered, current, available, locked, review due", width) {
			lines = append(lines, model.styles.muted.Render(line))
		}
	}
	if model.notice != "" {
		lines = append(lines, "")
		lines = append(lines, wrapText(model.notice, width)...)
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[r] Refresh", "[o] Open ROADMAP.md", "[Esc/h] Home", "[q] Quit")...)
	return lines
}

func (model Model) todayView(width int) []string {
	lines := []string{model.styles.title.Render("Today"), ""}
	switch {
	case model.dashboardLoading && model.dashboard.Goal == nil:
		lines = append(lines, "Building today's plan...")
	case model.dashboardErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load today's plan"))
		lines = append(lines, wrapText(model.dashboardErr.Error(), width)...)
	case model.dashboard.Goal == nil:
		lines = append(lines, model.styles.muted.Render("No active learning goal."))
	case model.dashboard.TodayPlan == nil:
		lines = append(lines, model.styles.muted.Render("No daily plan is available yet."))
	default:
		plan := model.dashboard.TodayPlan
		lines = append(lines, truncate(model.dashboard.Goal.Title, width))
		lines = append(lines, truncate(fmt.Sprintf("Planned: %d of %d minutes", plan.PlannedMinutes, plan.AvailableMinutes), width), "")
		if len(plan.Items) == 0 {
			message := "Nothing urgent today. Your current progress has no scheduled work."
			if plan.Status == learning.DailyPlanTimeLimited {
				message = "Today's time budget is too small for the next useful study item."
			}
			lines = append(lines, wrapText(message, width)...)
		}
		for index, item := range plan.Items {
			title := dashboardConceptTitle(model.dashboard, item.ConceptIDs[0])
			lines = append(lines, truncate(fmt.Sprintf("%d. %s - %s (%dm)", index+1, dailyPlanRoleLabel(item.Role), title, item.EstimatedMinutes), width))
			lines = append(lines, wrapText("   "+dailyPlanExplanation(model.dashboard, item), width)...)
		}
	}
	if model.dashboardLoading {
		lines = append(lines, "", model.styles.muted.Render("Refreshing..."))
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[r] Refresh", "[Esc/h] Home", "[q] Quit")...)
	return lines
}

func (model Model) progressView(width int) []string {
	lines := []string{model.styles.title.Render("Progress"), ""}
	switch {
	case model.dashboardErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load progress"))
		lines = append(lines, wrapText(model.dashboardErr.Error(), width)...)
	case model.dashboard.Goal == nil:
		lines = append(lines, model.styles.muted.Render("No active learning goal."))
	default:
		progress := model.dashboard.OverallProgress
		lines = append(lines, truncate(model.dashboard.Goal.Title, width))
		lines = append(lines, progressBar(progress.Completion.Value, width))
		lines = append(lines,
			truncate(fmt.Sprintf("Mastered: %d of %d concepts", progress.ConceptsMastered.Value, progress.ConceptsTotal.Value), width),
			truncate(fmt.Sprintf("Learning: %d", progress.ConceptsLearning.Value), width),
			truncate(fmt.Sprintf("Introduced: %d", progress.ConceptsIntroduced.Value), width),
			"",
		)
		average := "unknown"
		if model.dashboard.Mastery.AverageKnown.Value != nil {
			average = fmt.Sprintf("%.0f%%", model.dashboard.Mastery.AverageKnown.Value.Value()*100)
		}
		lines = append(lines, truncate("Average mastery (known concepts): "+average, width))
		lines = append(lines, truncate(fmt.Sprintf("Current mastery requirement: %.0f%%", dashboardMasteryThreshold(model.dashboard)*100), width))
		for _, line := range wrapText("Completion counts mastered curriculum concepts; average mastery excludes unknown concepts.", width) {
			lines = append(lines, model.styles.muted.Render(line))
		}
		lines = append(lines, "", fmt.Sprintf("Reviews due: %d", model.dashboard.ReviewsDue.Value))
		lines = append(lines, "Study today: "+formatStudyDuration(model.dashboard.StudyTime.Today.Value))
		lines = append(lines, "Study this week: "+formatStudyDuration(model.dashboard.StudyTime.Week.Value))
		lines = append(lines, fmt.Sprintf("Current streak: %d %s", model.dashboard.Streak.CurrentStreak.Value, streakDayWord(model.dashboard.Streak.CurrentStreak.Value)))
		if model.dashboard.RecentMilestone != nil {
			lines = append(lines, "Recent milestone: "+model.dashboard.RecentMilestone.Name)
		}
		if len(model.dashboard.WeakConcepts) > 0 {
			lines = append(lines, "", model.styles.heading.Render("Needs reinforcement"))
			for _, concept := range model.dashboard.WeakConcepts {
				lines = append(lines, truncate(fmt.Sprintf("- %s: %.0f%% mastery", concept.Title, concept.Mastery.Value()*100), width))
			}
		}
	}
	if model.dashboardLoading {
		lines = append(lines, "", model.styles.muted.Render("Refreshing..."))
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[r] Refresh", "[Esc/h] Home", "[q] Quit")...)
	return lines
}

func (model Model) conceptView(width int) []string {
	lines := []string{model.styles.title.Render("Concept detail"), ""}
	switch {
	case model.dashboardErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load concept detail"))
		lines = append(lines, wrapText(model.dashboardErr.Error(), width)...)
	case model.dashboard.Current == nil:
		lines = append(lines, model.styles.muted.Render("No current concept."), "")
		lines = append(lines, wrapText("The active curriculum may be complete, or no curriculum is active.", width)...)
	default:
		current := model.dashboard.Current
		lines = append(lines, model.styles.heading.Render(truncate(current.Concept.Title, width)), "")
		lines = append(lines,
			truncate("Phase: "+current.Phase.Title, width),
			truncate("Module: "+current.Module.Title, width),
			truncate("Lesson: "+current.Lesson.Title, width),
			truncate("Topic: "+current.Topic.Title, width),
			"",
		)
		if node := dashboardRoadmapConcept(model.dashboard, current.Concept.ID); node != nil {
			lines = append(lines, "Status: "+roadmapStatusLabel(node.Status))
			mastery := "unknown"
			if node.Mastery != nil {
				mastery = fmt.Sprintf("%.0f%%", node.Mastery.Value()*100)
			}
			lines = append(lines, "Mastery: "+mastery)
			for _, reason := range node.LockReasons {
				lines = append(lines, wrapText("Why locked: "+reason, width)...)
			}
		}
	}
	if model.dashboardLoading {
		lines = append(lines, "", model.styles.muted.Render("Refreshing..."))
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[r] Refresh", "[Esc/h] Home", "[q] Quit")...)
	return lines
}

func (model Model) reviewsView(width int) []string {
	lines := []string{model.styles.title.Render("Reviews"), ""}
	switch {
	case model.reviewsLoading && model.reviews.GeneratedAt == (learning.Timestamp{}):
		lines = append(lines, "Loading due reviews...")
	case model.reviewsErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load reviews"))
		lines = append(lines, wrapText(model.reviewsErr.Error(), width)...)
	case len(model.reviews.Items) == 0:
		lines = append(lines, model.styles.muted.Render("No reviews are due."))
	default:
		lines = append(lines, fmt.Sprintf("Due now: %d (%d minutes)", len(model.reviews.Items), model.reviews.UsedMinutes), "")
		for _, item := range model.reviews.Items {
			title := dashboardConceptTitle(model.dashboard, item.Item.ConceptID)
			flags := []string{string(item.Status)}
			if item.Overdue {
				flags = append(flags, "overdue")
			}
			if item.Critical {
				flags = append(flags, "prerequisite")
			}
			lines = append(lines, truncate(fmt.Sprintf("- %s (%dm, %s)", title, item.Item.EstimatedMinutes, strings.Join(flags, ", ")), width))
		}
		if len(model.reviews.Deferred) > 0 {
			lines = append(lines, "", fmt.Sprintf("Deferred by today's budget: %d", len(model.reviews.Deferred)))
		}
	}
	if model.reviewsLoading && model.reviews.GeneratedAt != (learning.Timestamp{}) {
		lines = append(lines, "", model.styles.muted.Render("Refreshing..."))
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[r] Refresh", "[Esc/h] Home", "[q] Quit")...)
	return lines
}

func (model Model) historyView(width int) []string {
	lines := []string{model.styles.title.Render("Study history"), ""}
	switch {
	case model.historyLoading && model.history.Timezone == "":
		lines = append(lines, "Loading study history...")
	case model.historyErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load study history"))
		lines = append(lines, wrapText(model.historyErr.Error(), width)...)
	case len(model.history.Events) == 0:
		for _, line := range wrapText("No learning activity recorded yet.", width) {
			lines = append(lines, model.styles.muted.Render(line))
		}
	default:
		start := len(model.history.Events) - 1
		limit := max(0, start-11)
		for index := start; index >= limit; index-- {
			event := model.history.Events[index]
			label := studyEventLabel(event.Type)
			if event.ConceptID != nil {
				label += ": " + dashboardConceptTitle(model.dashboard, *event.ConceptID)
			}
			lines = append(lines, truncate(event.OccurredAt.Time().Format("2006-01-02 15:04")+"  "+label, width))
		}
		if len(model.history.Events) > 12 {
			lines = append(lines, model.styles.muted.Render(fmt.Sprintf("Showing 12 of %d events.", len(model.history.Events))))
		}
	}
	if model.historyLoading && model.history.Timezone != "" {
		lines = append(lines, "", model.styles.muted.Render("Refreshing..."))
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[r] Refresh", "[Esc/h] Home", "[q] Quit")...)
	return lines
}

func (model Model) goalView(width int) []string {
	lines := []string{model.styles.title.Render("Learning goal"), ""}
	switch {
	case model.dashboardErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load learning goal"))
		lines = append(lines, wrapText(model.dashboardErr.Error(), width)...)
	case model.dashboard.Goal == nil:
		lines = append(lines, model.styles.muted.Render("No active learning goal."))
	default:
		goal := model.dashboard.Goal
		lines = append(lines,
			model.styles.heading.Render(truncate(goal.Title, width)),
			truncate("Domain: "+goal.Domain, width),
			truncate("Target: "+goal.TargetOutcome, width),
			truncate("Starting level: "+string(goal.StartingLevel), width),
			truncate("Status: "+string(goal.Status), width),
			truncate(fmt.Sprintf("Goal default mastery: %.0f%%", goal.MasteryThreshold.Value()*100), width),
			truncate(fmt.Sprintf("Current mastery requirement: %.0f%% (%s)", dashboardMasteryThreshold(model.dashboard)*100,
				model.dashboard.MasteryRequirement.Source.DisplayName()), width),
		)
		if goal.Description != "" {
			lines = append(lines, "")
			lines = append(lines, wrapText(goal.Description, width)...)
		}
		lines = append(lines, "")
		for _, line := range wrapText("Goal changes remain available through `kelyro goal`.", width) {
			lines = append(lines, model.styles.muted.Render(line))
		}
	}
	if model.dashboardLoading {
		lines = append(lines, "", model.styles.muted.Render("Refreshing..."))
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[r] Refresh", "[Esc/h] Home", "[q] Quit")...)
	return lines
}

func progressBar(percent float64, width int) string {
	barWidth := 20
	if width < 30 {
		barWidth = max(3, width-8)
	}
	filled := int(percent * float64(barWidth) / 100)
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}
	return truncate("["+strings.Repeat("#", filled)+strings.Repeat("-", barWidth-filled)+fmt.Sprintf("] %.0f%%", percent), width)
}

func formatStudyDuration(duration time.Duration) string {
	minutes := int(duration / time.Minute)
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh%02dm", minutes/60, minutes%60)
}

func dashboardNextTitle(dashboard learningapp.ProgressDashboard) string {
	if dashboard.TodayPlan != nil && len(dashboard.TodayPlan.Items) > 0 && len(dashboard.TodayPlan.Items[0].ConceptIDs) > 0 {
		return dashboardConceptTitle(dashboard, dashboard.TodayPlan.Items[0].ConceptIDs[0])
	}
	if dashboard.Current != nil {
		return dashboard.Current.Concept.Title
	}
	return ""
}

func dashboardMasteryThreshold(dashboard learningapp.ProgressDashboard) float64 {
	if dashboard.MasteryRequirement.PolicyVersion == learning.MasteryThresholdPolicyVersion {
		return dashboard.MasteryRequirement.Requirement.Threshold.Value()
	}
	if dashboard.Goal != nil {
		return dashboard.Goal.MasteryThreshold.Value()
	}
	return 0
}

func dashboardConceptTitle(dashboard learningapp.ProgressDashboard, conceptID learning.ID) string {
	for _, node := range dashboard.Roadmap {
		if node.ID == conceptID {
			return node.Title
		}
	}
	return conceptID.String()
}

func dashboardRoadmapConcept(dashboard learningapp.ProgressDashboard, conceptID learning.ID) *learningapp.DashboardRoadmapNode {
	for index := range dashboard.Roadmap {
		if dashboard.Roadmap[index].ID == conceptID {
			return &dashboard.Roadmap[index]
		}
	}
	return nil
}

func roadmapStatusLabel(status learningapp.DashboardRoadmapStatus) string {
	if status == learningapp.DashboardRoadmapReviewDue {
		return "review due"
	}
	return strings.ReplaceAll(string(status), "_", " ")
}

func dailyPlanRoleLabel(role learning.DailyPlanItemRole) string {
	return strings.ReplaceAll(string(role), "_", " ")
}

func dailyPlanExplanation(dashboard learningapp.ProgressDashboard, item learning.DailyPlanItem) string {
	explanation := item.Explanation
	for _, conceptID := range item.ConceptIDs {
		explanation = strings.ReplaceAll(explanation, conceptID.String(), dashboardConceptTitle(dashboard, conceptID))
	}
	return explanation
}

func studyEventLabel(eventType learning.StudyEventType) string {
	return strings.ReplaceAll(string(eventType), ".", " ")
}

func (model Model) contentWidth() int {
	width := model.width
	if width <= 0 {
		width = 80
	}
	padding := 4
	if width <= 32 {
		padding = 2
	}
	width -= padding
	if width > 88 {
		width = 88
	}
	if width < 1 {
		width = 1
	}
	return width
}

func (model Model) frame(lines []string, width int) string {
	padding := 2
	if model.width > 0 && model.width <= 32 {
		padding = 1
	}
	prefix := strings.Repeat(" ", padding)
	for index, line := range lines {
		if line != "" {
			lines[index] = prefix + line
		}
	}
	return "\n" + strings.Join(lines, "\n") + "\n"
}
