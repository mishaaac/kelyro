package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
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
		return foundationInitializedMsg{
			snapshot:   snapshot,
			resume:     resumed,
			sessionErr: sessionErr,
		}
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
