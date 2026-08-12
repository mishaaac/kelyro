package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
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
	ctx          context.Context
	service      Service
	command      app.Command
	snapshot     app.FoundationSnapshot
	screen       screen
	width        int
	height       int
	loading      bool
	saving       bool
	opening      bool
	loadErr      error
	notice       string
	configCursor int
	forceNoColor bool
	styles       styles
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
		forceNoColor: forceNoColor,
		styles:       newStyles(!forceNoColor),
	}
}

func (model Model) Init() tea.Cmd {
	return loadFoundationCmd(model.ctx, model.service, model.command)
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
	case foundationLoadFailedMsg:
		model.loading = false
		model.loadErr = message.err
		return model, nil
	case configSavedMsg:
		model.snapshot.Settings = cloneSettings(model.snapshot.Settings)
		model.snapshot.Settings[message.key] = message.value
		model.saving = false
		model.notice = message.message
		model.applyConfiguredColor()
		return model, nil
	case configSaveFailedMsg:
		model.saving = false
		model.notice = "Could not save configuration: " + message.err.Error()
		return model, nil
	case roadmapOpenedMsg:
		model.opening = false
		model.notice = message.message
		return model, nil
	case roadmapOpenFailedMsg:
		model.opening = false
		model.notice = "Could not open roadmap: " + message.err.Error()
		return model, nil
	}

	key, ok := message.(tea.KeyMsg)
	if !ok {
		return model, nil
	}
	keyName := key.String()
	if keyName == "ctrl+c" || keyName == "q" {
		return model, tea.Quit
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
		model.screen = screenHome
		model.notice = ""
		return model, nil
	}

	switch model.screen {
	case screenHome:
		switch keyName {
		case "enter", "r":
			model.screen = screenRoadmap
		case "d":
			model.screen = screenDoctor
		case "c":
			model.screen = screenConfig
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
