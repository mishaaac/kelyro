package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/session"
)

type screen uint8

const (
	screenHome screen = iota
	screenDoctor
	screenConfig
	screenRoadmap
)

// Model contains terminal-only state. It never discovers a workspace or reads
// configuration directly; asynchronous commands call the application service.
type Model struct {
	ctx               context.Context
	service           Service
	command           app.Command
	snapshot          app.FoundationSnapshot
	screen            screen
	width             int
	height            int
	loading           bool
	saving            bool
	opening           bool
	loadErr           error
	notice            string
	configCursor      int
	session           session.State
	sessionReady      bool
	checkpointing     bool
	checkpointPending bool
	quitting          bool
	forceNoColor      bool
	styles            styles
}

// NewModel creates the initial loading model. forceNoColor represents either
// --no-color or NO_COLOR and cannot be overridden by stored configuration.
func NewModel(ctx context.Context, service Service, command app.Command, forceNoColor bool) Model {
	if ctx == nil {
		ctx = context.Background()
	}
	return Model{
		ctx:          ctx,
		service:      service,
		command:      command,
		width:        80,
		height:       24,
		loading:      true,
		session:      session.Default(),
		forceNoColor: forceNoColor,
		styles:       newStyles(!forceNoColor),
	}
}

func (model Model) Init() tea.Cmd {
	return initializeFoundationCmd(model.ctx, model.service, model.command)
}

func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		return model, nil
	case foundationLoadedMsg:
		model.snapshot = message.snapshot
		model.loading = false
		model.loadErr = nil
		model.notice = ""
		model.applyConfiguredColor()
		return model, nil
	case foundationInitializedMsg:
		model.snapshot = message.snapshot
		model.loading = false
		model.loadErr = nil
		model.notice = ""
		model.applyConfiguredColor()
		if message.sessionErr != nil {
			model.notice = "Session state unavailable; continuing with defaults: " + message.sessionErr.Error()
			if message.resume.State.Version == 0 {
				if model.quitting {
					return model, tea.Quit
				}
				return model, nil
			}
		}
		model.sessionReady = true
		model.session = message.resume.State.Clone()
		if model.session.SafeToResume {
			model.screen = screenFromSession(model.session.LastView)
		}
		switch {
		case message.resume.Recovered && message.resume.PreviousIncomplete:
			model.notice = "Recovered invalid session state and detected an incomplete previous session."
		case message.resume.Recovered:
			model.notice = "Recovered invalid session state using safe defaults."
		case message.resume.PreviousIncomplete:
			model.notice = "Resumed after an incomplete previous session."
		case message.resume.MigratedFrom != 0:
			model.notice = fmt.Sprintf("Session state upgraded from version %d.", message.resume.MigratedFrom)
		}
		if model.quitting {
			return model, completeSessionCmd(model.ctx, model.service, model.command, model.session)
		}
		return model, nil
	case foundationLoadFailedMsg:
		model.loading = false
		model.loadErr = message.err
		if model.quitting {
			return model, tea.Quit
		}
		return model, nil
	case configSavedMsg:
		model.snapshot.Settings = cloneSettings(model.snapshot.Settings)
		model.snapshot.Settings[message.key] = message.value
		model.saving = false
		model.notice = message.message
		model.applyConfiguredColor()
		model.session.LastCommand = "config set " + message.key
		model.session.SetupFlags["configuration_touched"] = true
		return model.queueCheckpoint()
	case configSaveFailedMsg:
		model.saving = false
		model.notice = "Could not save configuration: " + message.err.Error()
		return model, nil
	case roadmapOpenedMsg:
		model.opening = false
		model.notice = message.message
		model.session.LastArtifact = "00-roadmap/ROADMAP.md"
		model.session.LastCommand = "open roadmap"
		return model.queueCheckpoint()
	case roadmapOpenFailedMsg:
		model.opening = false
		model.notice = "Could not open roadmap: " + message.err.Error()
		return model, nil
	case sessionCheckpointedMsg:
		return model.checkpointFinished(nil)
	case sessionCheckpointFailedMsg:
		return model.checkpointFinished(message.err)
	case sessionCompletedMsg, sessionCompleteFailedMsg:
		return model, tea.Quit
	}

	key, ok := message.(tea.KeyMsg)
	if !ok {
		return model, nil
	}
	keyName := key.String()
	if keyName == "ctrl+c" || keyName == "q" {
		return model.beginQuit()
	}
	if model.loading {
		return model, nil
	}
	if model.loadErr != nil {
		if keyName == "r" || keyName == "enter" {
			model.loading = true
			model.loadErr = nil
			return model, loadFoundationCmd(model.ctx, model.service, model.command)
		}
		return model, nil
	}

	if keyName == "esc" || keyName == "backspace" || keyName == "h" {
		changed := model.screen != screenHome
		model.screen = screenHome
		model.notice = ""
		if changed {
			model.session.LastView = session.ViewHome
			return model.queueCheckpoint()
		}
		return model, nil
	}

	switch model.screen {
	case screenHome:
		previous := model.screen
		switch keyName {
		case "enter", "r":
			model.screen = screenRoadmap
		case "d":
			model.screen = screenDoctor
		case "c":
			model.screen = screenConfig
		}
		if model.screen != previous {
			model.session.LastView = sessionView(model.screen)
			return model.queueCheckpoint()
		}
	case screenDoctor:
		if keyName == "r" {
			model.loading = true
			return model, loadFoundationCmd(model.ctx, model.service, model.command)
		}
	case screenConfig:
		return model.updateConfig(keyName)
	case screenRoadmap:
		if keyName == "o" && !model.opening {
			model.opening = true
			model.notice = "Opening roadmap..."
			return model, openRoadmapCmd(model.ctx, model.service, model.command)
		}
	}
	return model, nil
}

func (model Model) queueCheckpoint() (tea.Model, tea.Cmd) {
	if !model.sessionReady || model.quitting {
		return model, nil
	}
	if model.checkpointing {
		model.checkpointPending = true
		return model, nil
	}
	model.checkpointing = true
	return model, checkpointSessionCmd(model.ctx, model.service, model.command, model.session)
}

func (model Model) checkpointFinished(err error) (tea.Model, tea.Cmd) {
	model.checkpointing = false
	if err != nil && !model.quitting {
		model.notice = "Could not save session state: " + err.Error()
	}
	if model.quitting {
		model.checkpointPending = false
		return model, completeSessionCmd(model.ctx, model.service, model.command, model.session)
	}
	if model.checkpointPending {
		model.checkpointPending = false
		model.checkpointing = true
		return model, checkpointSessionCmd(model.ctx, model.service, model.command, model.session)
	}
	return model, nil
}

func (model Model) beginQuit() (tea.Model, tea.Cmd) {
	if model.quitting {
		return model, nil
	}
	if !model.sessionReady {
		if model.loading {
			model.quitting = true
			return model, nil
		}
		return model, tea.Quit
	}
	model.quitting = true
	model.checkpointPending = false
	if model.checkpointing {
		return model, nil
	}
	return model, completeSessionCmd(model.ctx, model.service, model.command, model.session)
}

func sessionView(current screen) session.View {
	switch current {
	case screenDoctor:
		return session.ViewDoctor
	case screenConfig:
		return session.ViewConfig
	case screenRoadmap:
		return session.ViewRoadmap
	default:
		return session.ViewHome
	}
}

func screenFromSession(view session.View) screen {
	switch view {
	case session.ViewDoctor:
		return screenDoctor
	case session.ViewConfig:
		return screenConfig
	case session.ViewRoadmap:
		return screenRoadmap
	default:
		return screenHome
	}
}

func (model Model) updateConfig(key string) (tea.Model, tea.Cmd) {
	keys := config.Keys()
	switch key {
	case "up", "k":
		if model.configCursor > 0 {
			model.configCursor--
		}
	case "down", "j":
		if model.configCursor < len(keys)-1 {
			model.configCursor++
		}
	case "enter", " ":
		if model.saving {
			return model, nil
		}
		selected := keys[model.configCursor]
		value, editable := nextConfigValue(selected, model.snapshot.Settings[selected])
		if !editable {
			model.notice = fmt.Sprintf("%s is read-only in this minimal wizard; use `kelyro config set`.", selected)
			return model, nil
		}
		model.saving = true
		model.notice = "Saving configuration..."
		return model, saveConfigCmd(model.ctx, model.service, model.command, selected, value)
	}
	return model, nil
}

func nextConfigValue(key string, current config.Value) (config.Value, bool) {
	switch key {
	case config.KeyUIColor:
		values := []string{"auto", "always", "never"}
		for index, value := range values {
			if current.String() == value {
				return config.StringValue(values[(index+1)%len(values)]), true
			}
		}
		return config.StringValue("auto"), true
	case config.KeyEditorPrompt, config.KeyAllowNetwork, config.KeyUpdateCheck:
		value, ok := current.BoolField()
		if !ok {
			return config.Value{}, false
		}
		return config.BoolValue(!value), true
	default:
		return config.Value{}, false
	}
}

func (model *Model) applyConfiguredColor() {
	color := "auto"
	if value, ok := model.snapshot.Settings[config.KeyUIColor]; ok {
		color = value.String()
	}
	model.styles = newStyles(!model.forceNoColor && color != "never")
}

func cloneSettings(source config.Settings) config.Settings {
	result := make(config.Settings, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
