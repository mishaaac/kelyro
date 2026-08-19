package tui

import (
	"fmt"
	"strings"

	"github.com/mishaaac/kelyro/internal/config"
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
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[Enter] Continue", "[p] Profile", "[d] Doctor", "[c] Config", "[r] Roadmap", "[q] Quit")...)
	return lines
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
