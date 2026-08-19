package session

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/audit"
)

var (
	previousTime = time.Date(2026, time.August, 11, 10, 0, 0, 0, time.UTC)
	currentTime  = time.Date(2026, time.August, 12, 11, 30, 0, 0, time.UTC)
)

func TestNormalQuitResumesLastSafeContext(t *testing.T) {
	states := &memoryStateStore{}
	recorder := &memoryAuditRecorder{}
	manager := NewManager(states, recorder, func() time.Time { return currentTime })

	started, err := manager.Resume(context.Background())
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	state := started.State
	state.LastView = ViewRoadmap
	state.LastArtifact = "00-roadmap/ROADMAP.md"
	state.LastCommand = "open roadmap"
	state.SetupFlags["configuration_touched"] = true
	if err := manager.Checkpoint(context.Background(), state); err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}
	if err := manager.Complete(context.Background(), state); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	resumed, err := manager.Resume(context.Background())
	if err != nil {
		t.Fatalf("second Resume() error = %v", err)
	}
	if resumed.PreviousIncomplete || resumed.Recovered {
		t.Fatalf("normal resume = %+v", resumed)
	}
	if resumed.State.LastView != ViewRoadmap || resumed.State.LastArtifact != "00-roadmap/ROADMAP.md" || resumed.State.LastCommand != "open roadmap" {
		t.Errorf("resumed context = %+v", resumed.State)
	}
	if !resumed.State.SetupFlags["configuration_touched"] {
		t.Error("setup flags were not resumed")
	}
	if len(recorder.events) != 0 {
		t.Errorf("normal resume audit events = %#v", recorder.events)
	}
}

func TestProfileViewIsAResumableDestination(t *testing.T) {
	t.Parallel()

	states := &memoryStateStore{}
	manager := NewManager(states, nil, func() time.Time { return currentTime })
	started, err := manager.Resume(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	state := started.State
	state.LastView = ViewProfile
	if err := manager.Complete(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	resumed, err := manager.Resume(context.Background())
	if err != nil || resumed.Recovered || resumed.State.LastView != ViewProfile {
		t.Fatalf("profile resume = (%+v, %v)", resumed, err)
	}
}

func TestCorruptStateRecoversDefaultsAndRecordsAudit(t *testing.T) {
	states := &memoryStateStore{value: []byte("{not-json"), found: true}
	recorder := &memoryAuditRecorder{}
	manager := NewManager(states, recorder, func() time.Time { return currentTime })

	resumed, err := manager.Resume(context.Background())
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if !resumed.Recovered || resumed.RecoveryReason != "invalid state" {
		t.Fatalf("recovery = %+v", resumed)
	}
	if resumed.State.LastView != ViewHome || !resumed.State.SafeToResume {
		t.Errorf("recovered state = %+v", resumed.State)
	}
	if len(recorder.events) != 1 || recorder.events[0].Name != "session.recovered" || recorder.events[0].Actor != audit.ActorSystem {
		t.Fatalf("audit events = %#v", recorder.events)
	}
	if _, _, _, err := decode(states.value); err != nil {
		t.Fatalf("repaired payload remains invalid: %v", err)
	}
}

func TestInterruptedSessionIsDetectedAndAudited(t *testing.T) {
	states := &memoryStateStore{}
	recorder := &memoryAuditRecorder{}
	manager := NewManager(states, recorder, func() time.Time { return currentTime })

	if _, err := manager.Resume(context.Background()); err != nil {
		t.Fatalf("first Resume() error = %v", err)
	}
	resumed, err := manager.Resume(context.Background())
	if err != nil {
		t.Fatalf("second Resume() error = %v", err)
	}
	if !resumed.PreviousIncomplete || resumed.Recovered {
		t.Fatalf("interrupted resume = %+v", resumed)
	}
	if len(recorder.events) != 1 || recorder.events[0].Metadata["reason"] != "previous session was incomplete" {
		t.Fatalf("audit events = %#v", recorder.events)
	}
}

func TestVersionOneStateMigratesWithoutLosingContext(t *testing.T) {
	payload, err := json.Marshal(persistedState{
		Version:          1,
		LastView:         ViewConfig,
		LastCommand:      "config set ui.color",
		SetupFlags:       map[string]bool{"configuration_touched": true},
		SessionStartedAt: previousTime,
		Active:           false,
	})
	if err != nil {
		t.Fatal(err)
	}
	states := &memoryStateStore{value: payload, found: true}
	recorder := &memoryAuditRecorder{}
	manager := NewManager(states, recorder, func() time.Time { return currentTime })

	resumed, err := manager.Resume(context.Background())
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if resumed.MigratedFrom != 1 || resumed.State.Version != CurrentVersion {
		t.Fatalf("migration result = %+v", resumed)
	}
	if resumed.State.LastView != ViewConfig || resumed.State.LastCommand != "config set ui.color" {
		t.Errorf("migrated state = %+v", resumed.State)
	}
	if len(recorder.events) != 1 || recorder.events[0].Name != "session.migrated" || recorder.events[0].Actor != audit.ActorSystem {
		t.Fatalf("audit events = %#v", recorder.events)
	}
}

type memoryStateStore struct {
	value []byte
	found bool
}

func (store *memoryStateStore) Get(context.Context, string, string) ([]byte, bool, error) {
	return append([]byte(nil), store.value...), store.found, nil
}

func (store *memoryStateStore) Set(_ context.Context, _, _ string, value []byte) error {
	store.value = append([]byte(nil), value...)
	store.found = true
	return nil
}

func (store *memoryStateStore) Delete(context.Context, string, string) error {
	store.value = nil
	store.found = false
	return nil
}

type memoryAuditRecorder struct {
	events []audit.Event
}

func (recorder *memoryAuditRecorder) Record(_ context.Context, event audit.Event) error {
	recorder.events = append(recorder.events, event)
	return nil
}
