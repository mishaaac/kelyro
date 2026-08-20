package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceCoordinatesWorkspaceMistakeQueries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := "/workspaces/mistake-lab"
	store := memory.New()
	profiles := learningapp.NewProfileService(learningapp.NewStudentService(store.Repositories().Students),
		learningapp.WithProfileClock(func() time.Time { return time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC) }))
	_, _ = profiles.Show(ctx)
	conceptID, _ := learning.NewID("concept.mean")
	topicID, _ := learning.NewID("topic.mean")
	curriculumID, _ := learning.NewID("curriculum.stats")
	if err := store.SeedCurriculum(learning.CurriculumRef{ID: curriculumID, Version: "1.0.0"},
		[]learning.Concept{{ID: conceptID, TopicID: topicID, Title: "Mean"}}, nil); err != nil {
		t.Fatal(err)
	}
	sequence := 0
	mistakes := learningapp.NewMistakeMemoryService(profiles, store, learningapp.WithMistakeMemoryIDGenerator(func(kind string) (learning.ID, error) {
		sequence++
		return learning.NewID(fmt.Sprintf("%s.app.%d", kind, sequence))
	}))
	observedAt, _ := learning.NewTimestamp(time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC))
	recorded, err := mistakes.Record(ctx, learningapp.RecordMistakeInput{ConceptID: conceptID, Key: "mean-vs-median",
		Category: learning.MistakeMisconception, Summary: "Confused mean and median", ObservedAt: observedAt, SourceRef: "fixture/check/1"})
	if err != nil {
		t.Fatal(err)
	}
	factory := &fakeProfileStoreFactory{profiles: profiles, mistakes: mistakes}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithProfiles(factory)
	listed, err := service.Execute(ctx, Command{Action: ActionMistakes, Workspace: root, MistakeOperation: "list"})
	if err != nil || len(listed.Mistakes) != 1 || listed.Mistakes[0].ID != recorded.Mistake.ID {
		t.Fatalf("mistakes list = (%+v, %v)", listed, err)
	}
	shown, err := service.Execute(ctx, Command{Action: ActionMistakes, Workspace: root, MistakeOperation: "show", MistakeID: recorded.Mistake.ID})
	if err != nil || shown.Mistake == nil || len(shown.Mistake.History) != 1 {
		t.Fatalf("mistakes show = (%+v, %v)", shown, err)
	}
	if factory.openRoot != root || factory.closed != 2 {
		t.Fatalf("mistake factory root=%q closed=%d", factory.openRoot, factory.closed)
	}
}
