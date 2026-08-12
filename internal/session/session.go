// Package session defines versioned, presentation-independent workspace
// session state and the policy used to resume it safely.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/storage"
)

const (
	// CurrentVersion is the session payload version written by this build.
	CurrentVersion = 2
	stateNamespace = "foundation"
	stateKey       = "session"
)

// View identifies a presentation-independent Foundation destination.
type View string

const (
	ViewHome    View = "home"
	ViewDoctor  View = "doctor"
	ViewConfig  View = "config"
	ViewRoadmap View = "roadmap"
)

// State is the small durable subset needed to continue a workspace session.
// It intentionally excludes state that is cheap and safe to reconstruct.
type State struct {
	Version          int
	LastView         View
	LastArtifact     string
	LastCommand      string
	SetupFlags       map[string]bool
	SessionStartedAt time.Time
	SafeToResume     bool
}

// Default returns a safe state for a new session.
func Default() State {
	return State{
		Version:      CurrentVersion,
		LastView:     ViewHome,
		SetupFlags:   map[string]bool{},
		SafeToResume: true,
	}
}

// Clone copies state so asynchronous persistence cannot observe later map
// mutations made by a presentation adapter.
func (state State) Clone() State {
	cloned := state
	cloned.SetupFlags = make(map[string]bool, len(state.SetupFlags))
	for key, value := range state.SetupFlags {
		cloned.SetupFlags[key] = value
	}
	return cloned
}

// Resume describes how the previous payload was interpreted.
type Resume struct {
	State              State
	PreviousIncomplete bool
	Recovered          bool
	RecoveryReason     string
	MigratedFrom       int
}

// Store persists one workspace's session state.
type Store interface {
	Resume(ctx context.Context) (Resume, error)
	Checkpoint(ctx context.Context, state State) error
	Complete(ctx context.Context, state State) error
	Close() error
}

// WorkspaceStoreFactory opens session stores without exposing their database.
type WorkspaceStoreFactory interface {
	Open(ctx context.Context, workspaceRoot string) (Store, error)
}

// Manager applies versioning and recovery policy to neutral persistence
// contracts. Infrastructure adapters provide transactional repositories.
type Manager struct {
	states storage.StateStore
	audit  audit.Recorder
	now    func() time.Time
}

// NewManager creates a manager over repositories that share one operation or
// transaction.
func NewManager(states storage.StateStore, recorder audit.Recorder, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{states: states, audit: recorder, now: now}
}

// Resume loads the last safe state, repairs secondary invalid metadata with
// defaults, marks this session active, and records any recovery.
func (manager *Manager) Resume(ctx context.Context) (Resume, error) {
	if manager == nil || manager.states == nil {
		return Resume{}, errors.New("session state store is unavailable")
	}

	encoded, found, err := manager.states.Get(ctx, stateNamespace, stateKey)
	if err != nil {
		return Resume{}, err
	}

	result := Resume{State: Default()}
	active := false
	if found {
		decoded, wasActive, migratedFrom, decodeErr := decode(encoded)
		if decodeErr != nil {
			result.Recovered = true
			result.RecoveryReason = "invalid state"
		} else if !decoded.SafeToResume {
			result.Recovered = true
			result.RecoveryReason = "state was not safe to resume"
		} else {
			result.State = decoded
			active = wasActive
			result.MigratedFrom = migratedFrom
		}
	}

	result.PreviousIncomplete = active
	result.State.Version = CurrentVersion
	result.State.SessionStartedAt = manager.now().UTC()
	result.State.SafeToResume = true
	result.State = normalize(result.State)

	if err := manager.write(ctx, result.State, true); err != nil {
		return Resume{}, err
	}
	manager.recordResumeEvents(ctx, result)
	return result, nil
}

// Checkpoint atomically replaces the durable payload while keeping the crash
// marker active.
func (manager *Manager) Checkpoint(ctx context.Context, state State) error {
	if manager == nil || manager.states == nil {
		return errors.New("session state store is unavailable")
	}
	state = normalize(state)
	state.SafeToResume = true
	if state.SessionStartedAt.IsZero() {
		state.SessionStartedAt = manager.now().UTC()
	}
	return manager.write(ctx, state, true)
}

// Complete stores the final resumable state and clears the crash marker.
func (manager *Manager) Complete(ctx context.Context, state State) error {
	if manager == nil || manager.states == nil {
		return errors.New("session state store is unavailable")
	}
	state = normalize(state)
	state.SafeToResume = true
	if state.SessionStartedAt.IsZero() {
		state.SessionStartedAt = manager.now().UTC()
	}
	return manager.write(ctx, state, false)
}

type persistedState struct {
	Version          int             `json:"version"`
	LastView         View            `json:"last_view"`
	LastArtifact     string          `json:"last_artifact,omitempty"`
	LastCommand      string          `json:"last_command,omitempty"`
	SetupFlags       map[string]bool `json:"setup_flags,omitempty"`
	SessionStartedAt time.Time       `json:"session_started_at"`
	SafeToResume     *bool           `json:"safe_to_resume,omitempty"`
	Active           bool            `json:"active"`
}

func (manager *Manager) write(ctx context.Context, state State, active bool) error {
	safe := state.SafeToResume
	payload := persistedState{
		Version:          CurrentVersion,
		LastView:         state.LastView,
		LastArtifact:     state.LastArtifact,
		LastCommand:      state.LastCommand,
		SetupFlags:       state.SetupFlags,
		SessionStartedAt: state.SessionStartedAt.UTC(),
		SafeToResume:     &safe,
		Active:           active,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode session state: %w", err)
	}
	if err := manager.states.Set(ctx, stateNamespace, stateKey, encoded); err != nil {
		return fmt.Errorf("persist session state: %w", err)
	}
	return nil
}

func decode(encoded []byte) (State, bool, int, error) {
	var payload persistedState
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return State{}, false, 0, fmt.Errorf("decode session state: %w", err)
	}
	if payload.Version != 1 && payload.Version != CurrentVersion {
		return State{}, false, 0, fmt.Errorf("unsupported session state version %d", payload.Version)
	}
	if !validView(payload.LastView) {
		return State{}, false, 0, fmt.Errorf("invalid last view %q", payload.LastView)
	}
	if payload.SessionStartedAt.IsZero() {
		return State{}, false, 0, errors.New("session timestamp is missing")
	}
	if payload.Version == CurrentVersion && payload.SafeToResume == nil {
		return State{}, false, 0, errors.New("safe resume marker is missing")
	}

	safe := true // Version 1 predates the explicit safe-resume marker.
	if payload.SafeToResume != nil {
		safe = *payload.SafeToResume
	}
	migratedFrom := 0
	if payload.Version != CurrentVersion {
		migratedFrom = payload.Version
	}
	return normalize(State{
		Version:          CurrentVersion,
		LastView:         payload.LastView,
		LastArtifact:     payload.LastArtifact,
		LastCommand:      payload.LastCommand,
		SetupFlags:       payload.SetupFlags,
		SessionStartedAt: payload.SessionStartedAt,
		SafeToResume:     safe,
	}), payload.Active, migratedFrom, nil
}

func normalize(state State) State {
	state.Version = CurrentVersion
	if !validView(state.LastView) {
		state.LastView = ViewHome
	}
	state.LastArtifact = strings.TrimSpace(state.LastArtifact)
	state.LastCommand = strings.TrimSpace(state.LastCommand)
	if state.SetupFlags == nil {
		state.SetupFlags = map[string]bool{}
	}
	return state
}

func validView(view View) bool {
	switch view {
	case ViewHome, ViewDoctor, ViewConfig, ViewRoadmap:
		return true
	default:
		return false
	}
}

func (manager *Manager) recordResumeEvents(ctx context.Context, result Resume) {
	if manager.audit == nil {
		return
	}
	if result.Recovered || result.PreviousIncomplete {
		reason := result.RecoveryReason
		if result.PreviousIncomplete {
			if reason != "" {
				reason += ", "
			}
			reason += "previous session was incomplete"
		}
		// Audit is best effort: secondary diagnostic persistence must never make
		// a recovered workspace unavailable.
		_ = manager.audit.Record(ctx, audit.Event{
			Name:    "session.recovered",
			Actor:   audit.ActorSystem,
			Subject: "workspace-session",
			Metadata: map[string]string{
				"reason": reason,
			},
		})
	}
	if result.MigratedFrom != 0 {
		_ = manager.audit.Record(ctx, audit.Event{
			Name:    "session.migrated",
			Actor:   audit.ActorSystem,
			Subject: "workspace-session",
			Metadata: map[string]string{
				"from_version": fmt.Sprintf("%d", result.MigratedFrom),
				"to_version":   fmt.Sprintf("%d", CurrentVersion),
			},
		})
	}
}
