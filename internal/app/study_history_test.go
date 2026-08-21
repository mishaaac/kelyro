package app

import (
	"context"
	"testing"

	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceCoordinatesStudyHistoryAndTime(t *testing.T) {
	root := "/workspaces/history-lab"
	history := &fakeStudyHistoryService{
		view: learningapp.StudyHistoryView{Period: learning.StudyPeriodToday, Timezone: "America/Lima"},
		time: learningapp.StudyTimeSummary{Timezone: "America/Lima", PolicyVersion: learning.TimeTrackingPolicyVersion},
	}
	factory := &fakeProfileStoreFactory{history: history}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithProfiles(factory)
	listed, err := service.Execute(context.Background(), Command{Action: ActionHistory, Workspace: root, HistoryToday: true})
	if err != nil || listed.History == nil || history.period != learning.StudyPeriodToday {
		t.Fatalf("history = (%+v, %v), period=%s", listed, err, history.period)
	}
	timed, err := service.Execute(context.Background(), Command{Action: ActionTime, Workspace: root})
	if err != nil || timed.StudyTime == nil || history.timeCalls != 1 {
		t.Fatalf("time = (%+v, %v), calls=%d", timed, err, history.timeCalls)
	}
	if factory.closed != 2 {
		t.Fatalf("closed = %d", factory.closed)
	}
}

type fakeStudyHistoryService struct {
	view      learningapp.StudyHistoryView
	time      learningapp.StudyTimeSummary
	period    learning.StudyPeriod
	timeCalls int
}

func (service *fakeStudyHistoryService) List(_ context.Context, period learning.StudyPeriod) (learningapp.StudyHistoryView, error) {
	service.period = period
	return service.view, nil
}

func (service *fakeStudyHistoryService) Time(context.Context) (learningapp.StudyTimeSummary, error) {
	service.timeCalls++
	return service.time, nil
}
