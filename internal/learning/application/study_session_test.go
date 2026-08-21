package application_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestStudySessionServiceStartActivityStopAndDuplicateActive(t *testing.T) {
	fixture := newStudySessionFixture(t)
	session, err := fixture.sessions.Start(fixture.ctx, fixture.goal.ID, fixture.instance.ID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if _, err := fixture.sessions.Start(fixture.ctx, fixture.goal.ID, fixture.instance.ID); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate Start() error = %v, want conflict", err)
	}
	fixture.now = fixture.now.Add(5 * time.Minute)
	session, err = fixture.sessions.RecordActivity(fixture.ctx)
	if err != nil {
		t.Fatalf("RecordActivity(first) error = %v", err)
	}
	fixture.now = fixture.now.Add(30 * time.Minute)
	session, err = fixture.sessions.RecordActivity(fixture.ctx)
	if err != nil {
		t.Fatalf("RecordActivity(after idle) error = %v", err)
	}
	fixture.now = fixture.now.Add(5 * time.Minute)
	session, err = fixture.sessions.Stop(fixture.ctx)
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if session.Status != learning.StudySessionCompleted || session.ActivityCount != 2 || session.ActiveDuration != 25*time.Minute {
		t.Fatalf("completed session = %+v", session)
	}
	if _, err := fixture.sessions.Current(fixture.ctx); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Current(after stop) error = %v, want not found", err)
	}
}

func TestStudySessionServiceCrashRecoveryAndAutomaticReplacement(t *testing.T) {
	fixture := newStudySessionFixture(t)
	started, err := fixture.sessions.Start(fixture.ctx, fixture.goal.ID, fixture.instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	fixture.now = fixture.now.Add(10 * time.Minute)
	resumable, err := fixture.sessions.Recover(fixture.ctx)
	if err != nil || resumable.ID != started.ID || resumable.Status != learning.StudySessionActive {
		t.Fatalf("Recover(recent) = (%+v, %v)", resumable, err)
	}

	fixture.now = fixture.now.Add(20 * time.Minute)
	recovered, err := fixture.sessions.Recover(fixture.ctx)
	if err != nil {
		t.Fatalf("Recover(stale) error = %v", err)
	}
	if recovered.Status != learning.StudySessionRecovered || recovered.EndedAt == nil || recovered.ActiveDuration != 15*time.Minute {
		t.Fatalf("recovered session = %+v", recovered)
	}

	replacement, err := fixture.sessions.Start(fixture.ctx, fixture.goal.ID, fixture.instance.ID)
	if err != nil || replacement.ID == started.ID || replacement.Status != learning.StudySessionActive {
		t.Fatalf("replacement Start() = (%+v, %v)", replacement, err)
	}
}

func TestStudySessionServiceStartRecoversStaleActiveAtomically(t *testing.T) {
	fixture := newStudySessionFixture(t)
	first, err := fixture.sessions.Start(fixture.ctx, fixture.goal.ID, fixture.instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	fixture.now = fixture.now.Add(time.Hour)
	second, err := fixture.sessions.Start(fixture.ctx, fixture.goal.ID, fixture.instance.ID)
	if err != nil {
		t.Fatalf("Start(after crash) error = %v", err)
	}
	if second.ID == first.ID {
		t.Fatal("Start(after crash) reused session identity")
	}
	storedFirst, err := fixture.store.Repositories().StudySessions.Get(fixture.ctx, first.ID)
	if err != nil || storedFirst.Status != learning.StudySessionRecovered || storedFirst.ActiveDuration != 15*time.Minute {
		t.Fatalf("stored recovered session = (%+v, %v)", storedFirst, err)
	}
}

func TestStudySessionServiceRollsBackRecoveryWhenReplacementFails(t *testing.T) {
	fixture := newStudySessionFixture(t)
	first, err := fixture.sessions.Start(fixture.ctx, fixture.goal.ID, fixture.instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	fixture.now = fixture.now.Add(time.Hour)
	failing := application.NewStudySessionLifecycleService(fixture.profiles, fixture.store,
		application.WithStudySessionClock(func() time.Time { return fixture.now }),
		application.WithStudySessionIdleTimeout(15*time.Minute),
		application.WithStudySessionIDGenerator(func() (learning.ID, error) { return first.ID, nil }),
	)
	if _, err := failing.Start(fixture.ctx, fixture.goal.ID, fixture.instance.ID); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("Start(duplicate replacement id) error = %v, want conflict", err)
	}
	stored, err := fixture.store.Repositories().StudySessions.Get(fixture.ctx, first.ID)
	if err != nil || stored.Status != learning.StudySessionActive || stored.EndedAt != nil {
		t.Fatalf("rolled-back active session = (%+v, %v)", stored, err)
	}
}

type studySessionFixture struct {
	ctx      context.Context
	now      time.Time
	store    *memory.Store
	profiles application.ProfileService
	goal     learning.LearningGoal
	instance learning.CurriculumInstance
	sessions application.StudySessionLifecycleService
}

func newStudySessionFixture(t *testing.T) *studySessionFixture {
	t.Helper()
	fixture := &studySessionFixture{
		ctx: context.Background(), now: time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC), store: memory.New(),
	}
	clock := func() time.Time { return fixture.now }
	profiles := application.NewProfileService(
		application.NewStudentService(fixture.store.Repositories().Students),
		application.WithProfileClock(clock),
	)
	fixture.profiles = profiles
	goals := application.NewGoalLifecycleService(profiles, fixture.store,
		application.WithGoalClock(clock),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.study-session"), nil }),
	)
	goal, err := goals.Set(fixture.ctx, goalInput(t, "Study sessions", "General knowledge"))
	if err != nil {
		t.Fatalf("set goal: %v", err)
	}
	instances := application.NewCurriculumInstanceService(profiles, fixture.store,
		application.WithCurriculumInstanceClock(clock),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.study-session"), nil }),
	)
	instance, err := instances.Create(fixture.ctx, goal.ID, instanceTestCurriculum(t, "1.0.0"), learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatalf("create curriculum instance: %v", err)
	}
	nextID := 0
	fixture.sessions = application.NewStudySessionLifecycleService(profiles, fixture.store,
		application.WithStudySessionClock(clock),
		application.WithStudySessionIdleTimeout(15*time.Minute),
		application.WithStudySessionIDGenerator(func() (learning.ID, error) {
			nextID++
			return testID(t, "session.study-session."+strconv.Itoa(nextID)), nil
		}),
	)
	fixture.goal, fixture.instance = goal, instance
	return fixture
}
