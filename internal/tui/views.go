package tui

import (
	"fmt"
	"strconv"
	"strings"

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
		"",
		model.styles.heading.Render("Status"),
	}
	lines = append(lines, statusLines(model.snapshot.Checks, false, model.styles, width)...)
	lines = append(lines, "")
	if model.snapshot.LearningPath {
		lines = append(lines, "Learning path ready.")
	} else {
		lines = append(lines, model.styles.muted.Render("No learning path yet."))
	}
	if len(model.milestones) > 0 {
		lines = append(lines, "", model.styles.success.Render("Milestone unlocked"))
		for _, achievement := range model.milestones {
			lines = append(lines, truncate(achievement.Name, width))
		}
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[Enter] Continue", "[s] Setup", "[p] Profile", "[k] Streak", "[d] Doctor", "[c] Config", "[r] Roadmap", "[q] Quit")...)
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
	if model.snapshot.LearningPath {
		lines = append(lines, "Your learning roadmap is ready.")
	} else {
		lines = append(lines, model.styles.muted.Render("No learning path yet."), "")
		lines = append(lines, wrapText("The Curriculum Compiler will populate this view in a future implementation.", width)...)
	}
	if model.notice != "" {
		lines = append(lines, "")
		lines = append(lines, wrapText(model.notice, width)...)
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[o] Open ROADMAP.md", "[Esc/h] Home", "[q] Quit")...)
	return lines
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
