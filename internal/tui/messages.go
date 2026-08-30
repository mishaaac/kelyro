package tui

import (
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/session"
)

type foundationInitializedMsg struct {
	snapshot     app.FoundationSnapshot
	resume       session.Resume
	sessionErr   error
	milestones   []learning.Achievement
	dashboard    *learningapp.ProgressDashboard
	dashboardErr error
}

type foundationLoadedMsg struct {
	snapshot app.FoundationSnapshot
}

type foundationLoadFailedMsg struct{ err error }

type dashboardLoadedMsg struct{ dashboard learningapp.ProgressDashboard }

type dashboardLoadFailedMsg struct{ err error }

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

type streakLoadedMsg struct{ streak learning.Streak }

type streakLoadFailedMsg struct{ err error }

type reviewsLoadedMsg struct{ reviews learningapp.ReviewQueueView }

type reviewsLoadFailedMsg struct{ err error }

type historyLoadedMsg struct{ history learningapp.StudyHistoryView }

type historyLoadFailedMsg struct{ err error }

type onboardingLoadedMsg struct{ view learningapp.LearnerSetupView }

type onboardingFailedMsg struct{ err error }

type sessionCheckpointedMsg struct{}

type sessionCheckpointFailedMsg struct{ err error }

type sessionCompletedMsg struct{}

type sessionCompleteFailedMsg struct{ err error }

type researchLoadedMsg struct{ stats researchapp.ResearchCostStats }
type researchLoadFailedMsg struct{ err error }
type sourcesLoadedMsg struct{ sources []app.SourceCLIView }
type sourcesLoadFailedMsg struct{ err error }
type sourceDetailLoadedMsg struct{ source app.SourceCLIView }
type sourceDetailLoadFailedMsg struct{ err error }
type sourceURLOpenedMsg struct{}
type sourceURLOpenFailedMsg struct{ err error }
type conflictsLoadedMsg struct{ conflicts []research.Conflict }
type conflictsLoadFailedMsg struct{ err error }
type claimLoadedMsg struct{ graph research.ProvenanceGraph }
type claimLoadFailedMsg struct{ err error }
type freshnessLoadedMsg struct{ records []researchapp.FreshnessRecord }
type freshnessLoadFailedMsg struct{ err error }
