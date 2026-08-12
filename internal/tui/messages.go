package tui

import (
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
)

type foundationLoadedMsg struct {
	snapshot app.FoundationSnapshot
}

type foundationLoadFailedMsg struct{ err error }

type configSavedMsg struct {
	key     string
	value   config.Value
	message string
}

type configSaveFailedMsg struct{ err error }

type roadmapOpenedMsg struct{ message string }

type roadmapOpenFailedMsg struct{ err error }
