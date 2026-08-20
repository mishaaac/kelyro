package tui

import (
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/session"
)

type foundationInitializedMsg struct {
	snapshot   app.FoundationSnapshot
	resume     session.Resume
	sessionErr error
}

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

type profileLoadedMsg struct{ student learning.Student }

type profileLoadFailedMsg struct{ err error }

type onboardingLoadedMsg struct{ view learningapp.LearnerSetupView }

type onboardingFailedMsg struct{ err error }

type sessionCheckpointedMsg struct{}

type sessionCheckpointFailedMsg struct{ err error }

type sessionCompletedMsg struct{}

type sessionCompleteFailedMsg struct{ err error }
