package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/doctor"
)

func diagnosticLines(report doctor.Report, style styles, width int) []string {
	var lines []string
	for _, section := range report.Sections() {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, style.heading.Render(section))
		for _, check := range report.ChecksIn(section) {
			marker := "✓"
			lineStyle := style.success
			if check.State == doctor.Fail {
				marker = "✗"
				lineStyle = style.failure
			} else if check.State == doctor.Miss {
				marker = "○"
				lineStyle = style.muted
			}
			label := check.DisplayName
			if check.Requirement != doctor.Required {
				label += " [" + string(check.Requirement) + "]"
			}
			lines = append(lines, lineStyle.Render(marker)+" "+truncate(label, max(1, width-2)))
			if check.Detail != "" {
				for _, detail := range wrapText(check.Detail, max(12, width-2)) {
					lines = append(lines, style.muted.Render("  "+detail))
				}
			}
			if check.WhyNeeded != "" {
				for _, why := range wrapText("Why: "+check.WhyNeeded, max(12, width-2)) {
					lines = append(lines, style.muted.Render("  "+why))
				}
			}
			if check.State != doctor.Pass && check.LearnMore != "" {
				for _, link := range wrapText("Learn more: "+check.LearnMore, max(12, width-2)) {
					lines = append(lines, style.muted.Render("  "+link))
				}
			}
		}
	}
	return lines
}

func statusLines(checks []app.FoundationCheck, doctor bool, style styles, width int) []string {
	lines := make([]string, 0, len(checks)*2)
	for _, check := range checks {
		marker := "!"
		label := "FAIL"
		lineStyle := style.failure
		if check.OK {
			marker = "✓"
			label = "PASS"
			lineStyle = style.success
		}
		if doctor {
			lines = append(lines, lineStyle.Render(label)+"  "+truncate(check.Name, max(1, width-6)))
		} else {
			lines = append(lines, lineStyle.Render(marker)+" "+truncate(check.Name, max(1, width-2)))
		}
		if check.Detail != "" {
			for _, detail := range wrapText(check.Detail, max(12, width-2)) {
				lines = append(lines, style.muted.Render("  "+detail))
			}
		}
	}
	return lines
}

func shortcutLines(width int, shortcuts ...string) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	current := ""
	for _, shortcut := range shortcuts {
		candidate := shortcut
		if current != "" {
			candidate = current + "   " + shortcut
		}
		if lipgloss.Width(candidate) <= width {
			current = candidate
			continue
		}
		if current != "" {
			lines = append(lines, current)
		}
		current = truncate(shortcut, width)
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func wrapText(text string, width int) []string {
	if width < 1 {
		return []string{""}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	lines := []string{truncate(words[0], width)}
	for _, word := range words[1:] {
		last := len(lines) - 1
		candidate := lines[last] + " " + word
		if lipgloss.Width(candidate) <= width {
			lines[last] = candidate
		} else {
			lines = append(lines, truncate(word, width))
		}
	}
	return lines
}

func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	result := ""
	for len(text) > 0 {
		_, size := utf8.DecodeRuneInString(text)
		candidate := result + text[:size]
		if lipgloss.Width(candidate)+1 > width {
			break
		}
		result = candidate
		text = text[size:]
	}
	return result + "…"
}
