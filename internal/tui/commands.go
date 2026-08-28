package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/session"
)

// Service is the application boundary consumed by the terminal adapter.
type Service interface {
	LoadFoundation(ctx context.Context, command app.Command) (app.FoundationSnapshot, error)
	Execute(ctx context.Context, command app.Command) (app.Result, error)
	ResumeSession(ctx context.Context, command app.Command) (session.Resume, error)
	CheckpointSession(ctx context.Context, command app.Command, state session.State) error
	CompleteSession(ctx context.Context, command app.Command, state session.State) error
}

func initializeFoundationCmd(ctx context.Context, service Service, command app.Command) tea.Cmd {
	return func() tea.Msg {
		if service == nil {
			return foundationLoadFailedMsg{err: fmt.Errorf("Foundation service is unavailable")}
		}
		snapshot, err := service.LoadFoundation(ctx, command)
		if err != nil {
			return foundationLoadFailedMsg{err: err}
		}
		resumed, sessionErr := service.ResumeSession(ctx, command)
		var milestones []learning.Achievement
		var dashboard *learningapp.ProgressDashboard
		var dashboardErr error
		if snapshot.LearningPath {
			result, achievementErr := service.Execute(ctx, app.Command{Action: app.ActionAchievements, Workspace: command.Workspace})
			if achievementErr == nil && result.Achievements != nil {
				for _, achievement := range result.Achievements.NewlyUnlocked {
					if !achievement.Hidden {
						milestones = append(milestones, achievement)
					}
				}
			}
			result, dashboardErr = service.Execute(ctx, app.Command{Action: app.ActionDashboard, Workspace: command.Workspace})
			if dashboardErr == nil {
				if result.Dashboard == nil {
					dashboardErr = fmt.Errorf("progress dashboard was not returned")
				} else {
					view := *result.Dashboard
					dashboard = &view
				}
			}
		}
		return foundationInitializedMsg{
			snapshot:     snapshot,
			resume:       resumed,
			sessionErr:   sessionErr,
			milestones:   milestones,
			dashboard:    dashboard,
			dashboardErr: dashboardErr,
		}
	}
}

func loadDashboardCmd(ctx context.Context, service Service, base app.Command) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{Action: app.ActionDashboard, Workspace: base.Workspace})
		if err != nil {
			return dashboardLoadFailedMsg{err: err}
		}
		if result.Dashboard == nil {
			return dashboardLoadFailedMsg{err: fmt.Errorf("progress dashboard was not returned")}
		}
		return dashboardLoadedMsg{dashboard: *result.Dashboard}
	}
}

func loadFoundationCmd(ctx context.Context, service Service, command app.Command) tea.Cmd {
	return func() tea.Msg {
		if service == nil {
			return foundationLoadFailedMsg{err: fmt.Errorf("Foundation service is unavailable")}
		}
		snapshot, err := service.LoadFoundation(ctx, command)
		if err != nil {
			return foundationLoadFailedMsg{err: err}
		}
		return foundationLoadedMsg{snapshot: snapshot}
	}
}

func saveConfigCmd(ctx context.Context, service Service, base app.Command, key string, value config.Value) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{
			Action:          app.ActionConfig,
			Workspace:       base.Workspace,
			ConfigOperation: "set",
			ConfigKey:       key,
			ConfigValue:     value.String(),
		})
		if err != nil {
			return configSaveFailedMsg{err: err}
		}
		return configSavedMsg{key: key, value: value, message: result.Message}
	}
}

func openRoadmapCmd(ctx context.Context, service Service, base app.Command) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{
			Action:          app.ActionOpen,
			Workspace:       base.Workspace,
			ConfigOverrides: base.ConfigOverrides,
			OpenTarget:      "roadmap",
		})
		if err != nil {
			return roadmapOpenFailedMsg{err: err}
		}
		return roadmapOpenedMsg{message: result.Message}
	}
}

func loadProfileCmd(ctx context.Context, service Service, base app.Command) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{
			Action:           app.ActionProfile,
			Workspace:        base.Workspace,
			ProfileOperation: "show",
		})
		if err != nil {
			return profileLoadFailedMsg{err: err}
		}
		if result.Profile == nil {
			return profileLoadFailedMsg{err: fmt.Errorf("learner profile was not returned")}
		}
		return profileLoadedMsg{student: *result.Profile}
	}
}

func loadStreakCmd(ctx context.Context, service Service, base app.Command) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{Action: app.ActionStreak, Workspace: base.Workspace})
		if err != nil {
			return streakLoadFailedMsg{err: err}
		}
		if result.Streak == nil {
			return streakLoadFailedMsg{err: fmt.Errorf("study streak was not returned")}
		}
		return streakLoadedMsg{streak: *result.Streak}
	}
}

func loadReviewsCmd(ctx context.Context, service Service, base app.Command) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{Action: app.ActionReviews, Workspace: base.Workspace, ReviewsDue: true})
		if err != nil {
			return reviewsLoadFailedMsg{err: err}
		}
		if result.Reviews == nil {
			return reviewsLoadFailedMsg{err: fmt.Errorf("review queue was not returned")}
		}
		return reviewsLoadedMsg{reviews: *result.Reviews}
	}
}

func loadHistoryCmd(ctx context.Context, service Service, base app.Command) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{Action: app.ActionHistory, Workspace: base.Workspace})
		if err != nil {
			return historyLoadFailedMsg{err: err}
		}
		if result.History == nil {
			return historyLoadFailedMsg{err: fmt.Errorf("study history was not returned")}
		}
		return historyLoadedMsg{history: *result.History}
	}
}

func loadResearchCmd(ctx context.Context, service Service, base app.Command) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{Action: app.ActionResearch, Workspace: base.Workspace, ResearchOperation: "stats"})
		if err != nil {
			return researchLoadFailedMsg{err: err}
		}
		if result.ResearchCostStats == nil {
			return researchLoadFailedMsg{err: fmt.Errorf("research summary was not returned")}
		}
		return researchLoadedMsg{stats: *result.ResearchCostStats}
	}
}

func loadSourcesCmd(ctx context.Context, service Service, base app.Command) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{Action: app.ActionSources, Workspace: base.Workspace, SourceRegistryOperation: "transparency"})
		if err != nil {
			return sourcesLoadFailedMsg{err: err}
		}
		if result.SourceTransparency == nil {
			result.SourceTransparency = make([]app.SourceCLIView, 0)
		}
		return sourcesLoadedMsg{sources: result.SourceTransparency}
	}
}

func loadSourceDetailCmd(ctx context.Context, service Service, base app.Command, sourceID research.SourceID) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{Action: app.ActionSources, Workspace: base.Workspace, SourceRegistryOperation: "source-show", SourceID: sourceID})
		if err != nil {
			return sourceDetailLoadFailedMsg{err: err}
		}
		if result.Source == nil {
			return sourceDetailLoadFailedMsg{err: fmt.Errorf("source detail was not returned")}
		}
		return sourceDetailLoadedMsg{source: *result.Source}
	}
}

func openSourceURLCmd(ctx context.Context, native platform.Platform, locator research.SourceLocator) tea.Cmd {
	return func() tea.Msg {
		if native == nil {
			return sourceURLOpenFailedMsg{err: fmt.Errorf("platform URL opener is unavailable")}
		}
		if err := ctx.Err(); err != nil {
			return sourceURLOpenFailedMsg{err: err}
		}
		if err := native.OpenURL(locator.String()); err != nil {
			return sourceURLOpenFailedMsg{err: err}
		}
		return sourceURLOpenedMsg{}
	}
}

func loadConflictsCmd(ctx context.Context, service Service, base app.Command) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{Action: app.ActionSources, Workspace: base.Workspace, SourceRegistryOperation: "conflicts"})
		if err != nil {
			return conflictsLoadFailedMsg{err: err}
		}
		if result.SourceConflicts == nil {
			result.SourceConflicts = make([]research.Conflict, 0)
		}
		return conflictsLoadedMsg{conflicts: result.SourceConflicts}
	}
}

func loadClaimCmd(ctx context.Context, service Service, base app.Command, claimID research.ClaimID) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{Action: app.ActionSources, Workspace: base.Workspace, SourceRegistryOperation: "trace", ProvenanceClaimID: claimID})
		if err != nil {
			return claimLoadFailedMsg{err: err}
		}
		if result.ProvenanceGraph == nil {
			return claimLoadFailedMsg{err: fmt.Errorf("claim provenance was not returned")}
		}
		return claimLoadedMsg{graph: *result.ProvenanceGraph}
	}
}

func loadFreshnessCmd(ctx context.Context, service Service, base app.Command) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{Action: app.ActionSources, Workspace: base.Workspace, SourceRegistryOperation: "stale"})
		if err != nil {
			return freshnessLoadFailedMsg{err: err}
		}
		if result.StaleSources == nil {
			result.StaleSources = make([]researchapp.FreshnessRecord, 0)
		}
		return freshnessLoadedMsg{records: result.StaleSources}
	}
}

func onboardingCmd(ctx context.Context, service Service, base app.Command, operation string, answers ...string) tea.Cmd {
	return func() tea.Msg {
		result, err := service.Execute(ctx, app.Command{
			Action: app.ActionSetup, Workspace: base.Workspace,
			SetupOperation: operation, SetupAnswers: answers,
		})
		if err != nil {
			return onboardingFailedMsg{err: err}
		}
		if result.Setup == nil {
			return onboardingFailedMsg{err: fmt.Errorf("learner setup state was not returned")}
		}
		return onboardingLoadedMsg{view: *result.Setup}
	}
}

func checkpointSessionCmd(ctx context.Context, service Service, command app.Command, state session.State) tea.Cmd {
	state = state.Clone()
	return func() tea.Msg {
		if err := service.CheckpointSession(ctx, command, state); err != nil {
			return sessionCheckpointFailedMsg{err: err}
		}
		return sessionCheckpointedMsg{}
	}
}

func completeSessionCmd(ctx context.Context, service Service, command app.Command, state session.State) tea.Cmd {
	state = state.Clone()
	return func() tea.Msg {
		if err := service.CompleteSession(ctx, command, state); err != nil {
			return sessionCompleteFailedMsg{err: err}
		}
		return sessionCompletedMsg{}
	}
}
