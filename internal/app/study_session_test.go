package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceCoordinatesStudySessionStatusAndStop(t *testing.T) {
	ctx := context.Background()
	root := "/workspaces/session-lab"
	session := appTestStudySession(t)
	sessions := &fakeStudySessionLifecycle{current: session, stopped: session}
	factory := &fakeProfileStoreFactory{sessions: sessions}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithProfiles(factory)

	status, err := service.Execute(ctx, Command{Action: ActionSession, Workspace: root, SessionOperation: "status"})
	if err != nil || status.StudySession == nil || status.StudySession.ID != session.ID {
		t.Fatalf("session status = (%+v, %v)", status, err)
	}
	stopped, err := service.Execute(ctx, Command{Action: ActionSession, Workspace: root, SessionOperation: "stop"})
	if err != nil || stopped.StudySession == nil || sessions.stopCalls != 1 {
		t.Fatalf("session stop = (%+v, %v), calls=%d", stopped, err, sessions.stopCalls)
	}
	if factory.openRoot != root || factory.closed != 2 {
		t.Fatalf("session factory root=%q closed=%d", factory.openRoot, factory.closed)
	}
}

func TestServiceReportsNoActiveStudySession(t *testing.T) {
	factory := &fakeProfileStoreFactory{sessions: &fakeStudySessionLifecycle{currentErr: learningapp.ErrNotFound}}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: "/workspaces/session-lab"}}, nil).WithProfiles(factory)
	result, err := service.Execute(context.Background(), Command{Action: ActionSession, Workspace: "/workspaces/session-lab", SessionOperation: "status"})
	if err != nil || result.Message != "No active study session." {
		t.Fatalf("empty session status = (%+v, %v)", result, err)
	}
}

type fakeStudySessionLifecycle struct {
	current    learning.StudySession
	currentErr error
	stopped    learning.StudySession
	stopCalls  int
}

func (service *fakeStudySessionLifecycle) Start(context.Context, learning.ID, learning.ID) (learning.StudySession, error) {
	return learning.StudySession{}, errors.New("unexpected Start call")
}
func (service *fakeStudySessionLifecycle) Current(context.Context) (learning.StudySession, error) {
	return service.current, service.currentErr
}
func (service *fakeStudySessionLifecycle) RecordActivity(context.Context) (learning.StudySession, error) {
	return learning.StudySession{}, errors.New("unexpected RecordActivity call")
}
func (service *fakeStudySessionLifecycle) Stop(context.Context) (learning.StudySession, error) {
	service.stopCalls++
	return service.stopped, nil
}
func (service *fakeStudySessionLifecycle) Interrupt(context.Context) (learning.StudySession, error) {
	return learning.StudySession{}, errors.New("unexpected Interrupt call")
}
func (service *fakeStudySessionLifecycle) Recover(context.Context) (learning.StudySession, error) {
	return learning.StudySession{}, errors.New("unexpected Recover call")
}

func appTestStudySession(t *testing.T) learning.StudySession {
	t.Helper()
	started, _ := learning.NewTimestamp(time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC))
	id, _ := learning.NewID("session.app")
	studentID, _ := learning.NewID("student.primary")
	goalID, _ := learning.NewID("goal.app")
	instanceID, _ := learning.NewID("instance.app")
	session, err := learning.NewStudySession(id, studentID, goalID, instanceID, started, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return session
}
