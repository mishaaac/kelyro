package app

import (
	"context"
	"testing"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceCoordinatesReviewLists(t *testing.T) {
	root := "/workspaces/reviews-lab"
	reviews := &fakeReviewScheduler{view: application.ReviewQueueView{DueOnly: true, AlgorithmVersion: "review-scheduler-v1"}}
	factory := &fakeProfileStoreFactory{reviews: reviews}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithProfiles(factory)
	result, err := service.Execute(context.Background(), Command{Action: ActionReviews, Workspace: root, ReviewsDue: true})
	if err != nil || result.Reviews == nil || !reviews.dueOnly || factory.closed != 1 {
		t.Fatalf("reviews = (%+v, %v), due=%v closed=%d", result, err, reviews.dueOnly, factory.closed)
	}
}

type fakeReviewScheduler struct {
	view    application.ReviewQueueView
	dueOnly bool
}

func (service *fakeReviewScheduler) List(_ context.Context, dueOnly bool) (application.ReviewQueueView, error) {
	service.dueOnly = dueOnly
	return service.view, nil
}

func (service *fakeReviewScheduler) Postpone(context.Context, learning.ID, learning.Timestamp) (learning.ReviewItem, error) {
	return learning.ReviewItem{}, nil
}

func (service *fakeReviewScheduler) Skip(context.Context, learning.ID) (learning.ReviewItem, error) {
	return learning.ReviewItem{}, nil
}

func (service *fakeReviewScheduler) RecordOutcome(context.Context, learning.ID, learning.MasteryScore) (application.ReviewOutcomeUpdate, error) {
	return application.ReviewOutcomeUpdate{}, nil
}
