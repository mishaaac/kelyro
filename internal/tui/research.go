package tui

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/research"
)

func (model Model) isResearchScreen() bool {
	switch model.screen {
	case screenResearch, screenSources, screenSourceDetail, screenClaimDetail, screenConflicts, screenFreshness:
		return true
	default:
		return false
	}
}

func (model *Model) researchBack() {
	model.scrollOffset = 0
	model.notice = ""
	switch model.screen {
	case screenSourceDetail:
		model.screen = screenSources
	case screenClaimDetail:
		model.screen = screenConflicts
	case screenSources, screenConflicts, screenFreshness:
		model.screen = screenResearch
	default:
		model.screen = screenHome
	}
}

func (model Model) updateResearch(keyName string) (tea.Model, tea.Cmd) {
	switch keyName {
	case "r":
		if !model.researchLoading {
			model.researchLoading = true
			model.researchErr = nil
			return model, loadResearchCmd(model.ctx, model.service, model.command)
		}
	case "s":
		model.screen = screenSources
		model.scrollOffset = 0
		model.sourcesLoading = true
		model.sourcesErr = nil
		return model, loadSourcesCmd(model.ctx, model.service, model.command)
	case "c":
		model.screen = screenConflicts
		model.scrollOffset = 0
		model.conflictsLoading = true
		model.conflictsErr = nil
		return model, loadConflictsCmd(model.ctx, model.service, model.command)
	case "f":
		model.screen = screenFreshness
		model.scrollOffset = 0
		model.freshnessLoading = true
		model.freshnessErr = nil
		return model, loadFreshnessCmd(model.ctx, model.service, model.command)
	}
	return model, nil
}

func (model Model) updateSources(keyName string) (tea.Model, tea.Cmd) {
	switch keyName {
	case "up", "k":
		model.sourceCursor = max(0, model.sourceCursor-1)
		model.revealSourceCursor()
	case "down", "j":
		model.sourceCursor = min(max(0, len(model.sources)-1), model.sourceCursor+1)
		model.revealSourceCursor()
	case "enter":
		if len(model.sources) > 0 && !model.sourcesLoading {
			model.screen = screenSourceDetail
			model.scrollOffset = 0
			model.sourcesLoading = true
			model.sourceDetailErr = nil
			model.notice = ""
			return model, loadSourceDetailCmd(model.ctx, model.service, model.command, model.sources[model.sourceCursor].Source.ID)
		}
	case "r":
		if !model.sourcesLoading {
			model.sourcesLoading = true
			model.sourcesErr = nil
			return model, loadSourcesCmd(model.ctx, model.service, model.command)
		}
	}
	return model, nil
}

func (model Model) updateSourceDetail(keyName string) (tea.Model, tea.Cmd) {
	if keyName == "r" && model.sourceDetail != nil && !model.sourcesLoading {
		model.sourcesLoading = true
		model.sourceDetailErr = nil
		return model, loadSourceDetailCmd(model.ctx, model.service, model.command, model.sourceDetail.Source.ID)
	}
	if keyName == "o" && model.sourceDetail != nil && !model.opening {
		model.opening = true
		model.notice = "Opening source..."
		return model, openSourceURLCmd(model.ctx, model.platform, model.sourceDetail.Source.Locator)
	}
	return model, nil
}

func (model Model) updateConflicts(keyName string) (tea.Model, tea.Cmd) {
	if len(model.conflicts) > 0 {
		selected := model.conflicts[model.conflictCursor]
		switch keyName {
		case "up", "k":
			model.conflictCursor = max(0, model.conflictCursor-1)
			model.claimCursor = 0
			model.revealConflictCursor()
		case "down", "j":
			model.conflictCursor = min(len(model.conflicts)-1, model.conflictCursor+1)
			model.claimCursor = 0
			model.revealConflictCursor()
		case "left":
			model.claimCursor = max(0, model.claimCursor-1)
		case "right":
			model.claimCursor = min(max(0, len(selected.ClaimIDs)-1), model.claimCursor+1)
		case "enter":
			if len(selected.ClaimIDs) > 0 && !model.claimLoading {
				claimID := selected.ClaimIDs[min(model.claimCursor, len(selected.ClaimIDs)-1)]
				model.screen = screenClaimDetail
				model.scrollOffset = 0
				model.claimLoading = true
				model.claimErr = nil
				model.claimGraph = nil
				return model, loadClaimCmd(model.ctx, model.service, model.command, claimID)
			}
		}
	}
	if keyName == "r" && !model.conflictsLoading {
		model.conflictsLoading = true
		model.conflictsErr = nil
		return model, loadConflictsCmd(model.ctx, model.service, model.command)
	}
	return model, nil
}

func (model Model) researchView(width int) []string {
	lines := []string{model.styles.title.Render("Research"), ""}
	switch {
	case model.researchLoading && model.researchStats.AlgorithmVersion == "":
		lines = append(lines, "Loading research summary...")
	case model.researchErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load research summary"))
		lines = append(lines, wrapText(model.researchErr.Error(), width)...)
	default:
		stats := model.researchStats
		lines = append(lines,
			truncate(fmt.Sprintf("Runs: %d (%d stopped by budget)", stats.Runs, stats.BudgetStoppedRuns), width),
			truncate(fmt.Sprintf("Used: %d searches · %d fetches · %d bytes", stats.Used.SearchRequests, stats.Used.FetchRequests, stats.Used.Bytes), width),
			truncate(fmt.Sprintf("Cache saved: %d searches · %d fetches", stats.CacheSavings.SearchRequests, stats.CacheSavings.FetchRequests), width),
			truncate("As of: "+formatResearchTime(stats.AsOf.Time()), width),
			truncate("Algorithm: "+stats.AlgorithmVersion, width),
		)
	}
	if model.researchLoading && model.researchStats.AlgorithmVersion != "" {
		lines = append(lines, "", model.styles.muted.Render("Refreshing..."))
	}
	lines = append(lines, "", model.styles.heading.Render("Transparency"))
	lines = append(lines, wrapText("Sources show persisted authority, freshness, temporal scope, and version scope. Claims are traceable from unresolved conflicts.", width)...)
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[s] Sources", "[c] Conflicts", "[f] Freshness", "[r] Refresh", "[Esc/h] Home", "[q] Quit")...)
	return lines
}

func (model Model) sourcesView(width int) []string {
	lines := []string{model.styles.title.Render("Sources"), ""}
	switch {
	case model.sourcesLoading && len(model.sources) == 0:
		lines = append(lines, "Loading stored sources...")
	case model.sourcesErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load sources"))
		lines = append(lines, wrapText(model.sourcesErr.Error(), width)...)
	case len(model.sources) == 0:
		lines = append(lines, model.styles.muted.Render("No sources recorded."))
	default:
		for index, view := range model.sources {
			marker := "  "
			style := model.styles.muted
			if index == model.sourceCursor {
				marker = "> "
				style = model.styles.selected
			}
			title := view.Source.Metadata.Title
			lines = append(lines, style.Render(truncate(marker+title, width)))
			metadata := fmt.Sprintf("  %s · %s · %s", sourceKindLabel(view.Source.Kind), sourceAuthorityLabel(view), sourceFreshnessLabel(view))
			lines = append(lines, truncate(metadata, width))
			if warning := sourceWarning(view.Source); warning != "" {
				lines = append(lines, model.styles.failure.Render(truncate("  ! "+warning, width)))
			}
		}
	}
	if model.sourcesLoading && len(model.sources) > 0 {
		lines = append(lines, "", model.styles.muted.Render("Refreshing..."))
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[↑/↓] Select", "[Enter] Detail", "[r] Refresh", "[Esc] Research", "[h] Home", "[q] Quit")...)
	return lines
}

func (model Model) sourceDetailView(width int) []string {
	lines := []string{model.styles.title.Render("Source detail"), ""}
	switch {
	case model.sourcesLoading && model.sourceDetail == nil:
		lines = append(lines, "Loading source detail...")
	case model.sourceDetailErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load source detail"))
		lines = append(lines, wrapText(model.sourceDetailErr.Error(), width)...)
	case model.sourceDetail == nil:
		lines = append(lines, model.styles.muted.Render("No source selected."))
	default:
		view := *model.sourceDetail
		source := view.Source
		lines = append(lines,
			model.styles.heading.Render(truncate(source.Metadata.Title, width)),
			truncate("ID: "+source.ID.String(), width),
			truncate("Kind: "+sourceKindLabel(source.Kind), width),
			truncate("Authority: "+sourceAuthorityDetail(view), width),
			truncate("Freshness: "+sourceFreshnessDetail(view), width),
			truncate("Version scope: "+sourceVersionScope(source), width),
			truncate("Location: "+compactLocator(source.Locator, width-len("Location: ")), width),
		)
		if source.Metadata.Publisher != "" {
			lines = append(lines, truncate("Publisher: "+source.Metadata.Publisher, width))
		}
		if warning := sourceWarning(source); warning != "" {
			lines = append(lines, "", model.styles.failure.Render("Warning"))
			lines = append(lines, wrapText(warning, width)...)
		}
		if view.LatestSnapshot == nil {
			lines = append(lines, "", "Latest snapshot: not available")
		} else {
			lines = append(lines, "", truncate("Latest snapshot: "+view.LatestSnapshot.ID.String(), width), truncate("Fetched: "+formatResearchTime(view.LatestSnapshot.FetchedAt.Time()), width))
			if view.LatestSnapshot.Fetch.ContentHash != "" {
				lines = append(lines, "Content hash: "+truncate(view.LatestSnapshot.Fetch.ContentHash, max(1, width-len("Content hash: "))))
			}
		}
	}
	if model.notice != "" {
		lines = append(lines, "")
		lines = append(lines, wrapText(model.notice, width)...)
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[o] Open in browser", "[r] Refresh", "[Esc] Sources", "[h] Home", "[q] Quit")...)
	return lines
}

func (model Model) conflictsView(width int) []string {
	lines := []string{model.styles.title.Render("Conflicts"), ""}
	switch {
	case model.conflictsLoading && len(model.conflicts) == 0:
		lines = append(lines, "Loading unresolved conflicts...")
	case model.conflictsErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load conflicts"))
		lines = append(lines, wrapText(model.conflictsErr.Error(), width)...)
	case len(model.conflicts) == 0:
		lines = append(lines, model.styles.success.Render(truncate("No unresolved conflicts.", width)))
	default:
		for index, conflict := range model.conflicts {
			marker := "  "
			style := model.styles.muted
			if index == model.conflictCursor {
				marker = "> "
				style = model.styles.selected
			}
			lines = append(lines, style.Render(truncate(marker+strings.ReplaceAll(string(conflict.Type), "_", " ")+" · "+conflict.ID.String(), width)))
			lines = append(lines, wrapText("  "+conflict.Reason, width)...)
			claims := make([]string, len(conflict.ClaimIDs))
			for claimIndex, claimID := range conflict.ClaimIDs {
				claims[claimIndex] = claimID.String()
				if index == model.conflictCursor && claimIndex == model.claimCursor {
					claims[claimIndex] = "[" + claims[claimIndex] + "]"
				}
			}
			lines = append(lines, wrapText("  Claims: "+strings.Join(claims, " · "), width)...)
		}
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[↑/↓] Conflict", "[←/→] Claim", "[Enter] Claim detail", "[r] Refresh", "[Esc] Research", "[h] Home", "[q] Quit")...)
	return lines
}

func (model Model) claimDetailView(width int) []string {
	lines := []string{model.styles.title.Render("Claim detail"), ""}
	switch {
	case model.claimLoading && model.claimGraph == nil:
		lines = append(lines, "Loading claim provenance...")
	case model.claimErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load claim detail"))
		lines = append(lines, wrapText(model.claimErr.Error(), width)...)
	case model.claimGraph == nil:
		lines = append(lines, model.styles.muted.Render("No claim selected."))
	default:
		graph := model.claimGraph
		lines = append(lines,
			truncate("Claim: "+graph.ClaimID.String(), width),
			truncate("Recorded: "+formatResearchTime(graph.RecordedAt.Time()), width),
			truncate("Algorithm: "+graph.AlgorithmVersion, width),
			"",
			model.styles.heading.Render("Provenance"),
		)
		for _, node := range graph.Nodes {
			label := fmt.Sprintf("- %s · %s", strings.ReplaceAll(string(node.Kind), "_", " "), node.Label)
			lines = append(lines, wrapText(label, width)...)
			if node.ToolVersion != "" {
				lines = append(lines, model.styles.muted.Render(truncate("  Tool: "+node.ToolVersion, width)))
			}
		}
	}
	lines = append(lines, "")
	lines = append(lines, shortcutLines(width, "[r] Refresh", "[Esc] Conflicts", "[h] Home", "[q] Quit")...)
	return lines
}

func (model Model) freshnessView(width int) []string {
	lines := []string{model.styles.title.Render("Freshness"), ""}
	switch {
	case model.freshnessLoading && len(model.staleSources) == 0:
		lines = append(lines, "Loading reverification schedule...")
	case model.freshnessErr != nil:
		lines = append(lines, model.styles.failure.Render("Could not load freshness"))
		lines = append(lines, wrapText(model.freshnessErr.Error(), width)...)
	case len(model.staleSources) == 0:
		for _, line := range wrapText("Nothing is currently due for reverification.", width) {
			lines = append(lines, model.styles.success.Render(line))
		}
	default:
		for _, record := range model.staleSources {
			lines = append(lines, model.styles.heading.Render(truncate(record.SubjectID.String(), width)))
			lines = append(lines,
				truncate(fmt.Sprintf("State: %s · score %.0f%% · priority %s", record.State, record.Score.Value()*100, record.Priority), width),
				truncate("Last verified: "+formatResearchTime(record.LastVerifiedAt.Time()), width),
			)
			if record.NextVerifyAt != nil {
				lines = append(lines, truncate("Due: "+formatResearchTime(record.NextVerifyAt.Time())+" · "+strings.ReplaceAll(string(record.VerificationReason), "_", " "), width))
			}
			lines = append(lines, "")
		}
	}
	lines = append(lines, shortcutLines(width, "[r] Refresh", "[Esc] Research", "[h] Home", "[q] Quit")...)
	return lines
}

func sourceKindLabel(kind research.SourceKind) string {
	return strings.ReplaceAll(string(kind), "_", " ")
}

func sourceAuthorityLabel(view app.SourceCLIView) string {
	if view.TrustDecision == nil {
		return "authority unknown"
	}
	return "tier " + string(view.TrustDecision.Tier)
}

func sourceAuthorityDetail(view app.SourceCLIView) string {
	if view.TrustDecision == nil {
		return "unknown (no persisted trust decision)"
	}
	return fmt.Sprintf("tier %s · %s · %s", view.TrustDecision.Tier, strings.ReplaceAll(string(view.TrustDecision.State), "_", " "), view.TrustDecision.Policy)
}

func sourceFreshnessLabel(view app.SourceCLIView) string {
	if view.Freshness == nil {
		return "freshness unknown"
	}
	return fmt.Sprintf("%s %.0f%%", view.Freshness.State, view.Freshness.Score.Value()*100)
}

func sourceFreshnessDetail(view app.SourceCLIView) string {
	if view.Freshness == nil {
		return "unknown (no persisted verification)"
	}
	return fmt.Sprintf("%s · %.0f%% · last verified %s", view.Freshness.State, view.Freshness.Score.Value()*100, formatResearchTime(view.Freshness.LastVerifiedAt.Time()))
}

func sourceVersionScope(source research.Source) string {
	if source.Version != nil {
		return string(source.TemporalScope) + " · " + source.Version.String()
	}
	return string(source.TemporalScope)
}

func sourceWarning(source research.Source) string {
	warning, err := source.TemporalScope.Warning(source.Version)
	if err != nil {
		return "Invalid temporal scope metadata."
	}
	return warning
}

func compactLocator(locator research.SourceLocator, width int) string {
	parsed, err := url.Parse(locator.String())
	if err != nil {
		return "invalid locator"
	}
	display := parsed.Host + parsed.EscapedPath()
	if display == "" {
		display = parsed.Host
	}
	return truncate(display, max(1, width))
}

func formatResearchTime(value time.Time) string {
	if value.IsZero() {
		return "not available"
	}
	return value.UTC().Format("2006-01-02 15:04 UTC")
}

func (model *Model) revealSourceCursor() {
	target := 2
	for index := 0; index < model.sourceCursor && index < len(model.sources); index++ {
		target += 2
		if sourceWarning(model.sources[index].Source) != "" {
			target++
		}
	}
	model.revealLine(target)
}

func (model *Model) revealConflictCursor() {
	width := model.contentWidth()
	target := 2
	for index := 0; index < model.conflictCursor && index < len(model.conflicts); index++ {
		conflict := model.conflicts[index]
		target++
		target += len(wrapText("  "+conflict.Reason, width))
		claims := make([]string, len(conflict.ClaimIDs))
		for claimIndex, claimID := range conflict.ClaimIDs {
			claims[claimIndex] = claimID.String()
		}
		target += len(wrapText("  Claims: "+strings.Join(claims, " · "), width))
	}
	model.revealLine(target)
}

func (model *Model) revealLine(target int) {
	lineCount := len(model.viewLines(model.contentWidth()))
	pageHeight, overflow := model.viewportHeight(lineCount)
	if !overflow {
		model.scrollOffset = 0
		return
	}
	if target < model.scrollOffset {
		model.scrollOffset = target
	} else if target >= model.scrollOffset+pageHeight {
		model.scrollOffset = target - pageHeight + 1
	}
	model.scrollOffset = min(max(0, model.scrollOffset), lineCount-pageHeight)
}
