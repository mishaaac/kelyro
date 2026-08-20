package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestMistakeMemoryCreatesDeduplicatesResolvesAndReopensWithHistory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students),
		application.WithProfileClock(func() time.Time { return mistakeTime(1) }))
	student, err := profiles.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conceptID := testID(t, "concept.mean")
	if err := store.SeedCurriculum(learning.CurriculumRef{ID: testID(t, "curriculum.stats"), Version: "1.0.0"},
		[]learning.Concept{{ID: conceptID, TopicID: testID(t, "topic.mean"), Title: "Mean"}}, nil); err != nil {
		t.Fatal(err)
	}

	now := mistakeTime(3)
	sequence := 0
	service := application.NewMistakeMemoryService(profiles, store,
		application.WithMistakeMemoryClock(func() time.Time { return now }),
		application.WithMistakeMemoryIDGenerator(func(kind string) (learning.ID, error) {
			sequence++
			return learning.NewID(fmt.Sprintf("%s.%02d", kind, sequence))
		}))
	input := application.RecordMistakeInput{
		ConceptID: conceptID, Key: "mean-vs-median", Category: learning.MistakeMisconception,
		Summary: "Confused mean and median", ObservedAt: testTimestamp(t, 2), SourceRef: "fixture/check/1",
	}
	created, err := service.Record(ctx, input)
	if err != nil || !created.Created || created.Mistake.Occurrences != 1 || created.Mistake.StudentID != student.ID {
		t.Fatalf("first Record() = (%+v, %v)", created, err)
	}
	input.ObservedAt = testTimestamp(t, 3)
	input.SourceRef = "fixture/check/2"
	deduplicated, err := service.Record(ctx, input)
	if err != nil || deduplicated.Created || deduplicated.Mistake.ID != created.Mistake.ID || deduplicated.Mistake.Occurrences != 2 {
		t.Fatalf("deduplicated Record() = (%+v, %v)", deduplicated, err)
	}

	now = mistakeTime(4)
	reinforced, err := service.Reinforce(ctx, created.Mistake.ID, "fixture/warm-up/1")
	if err != nil || reinforced.Mistake.Status != learning.MistakeReinforced {
		t.Fatalf("Reinforce() = (%+v, %v)", reinforced, err)
	}
	now = mistakeTime(5)
	resolved, err := service.Resolve(ctx, created.Mistake.ID, "fixture/check/3")
	if err != nil || resolved.Mistake.Status != learning.MistakeResolved || resolved.Mistake.ResolvedAt == nil {
		t.Fatalf("Resolve() = (%+v, %v)", resolved, err)
	}
	input.ObservedAt = testTimestamp(t, 6)
	input.SourceRef = "fixture/check/4"
	reopened, err := service.Record(ctx, input)
	if err != nil || reopened.Mistake.Status != learning.MistakeRecent || reopened.Mistake.ResolvedAt != nil || reopened.Mistake.Occurrences != 3 {
		t.Fatalf("reopen Record() = (%+v, %v)", reopened, err)
	}

	view, err := service.Get(ctx, created.Mistake.ID)
	if err != nil || len(view.History) != 5 {
		t.Fatalf("Get() = (%+v, %v)", view, err)
	}
	wantTypes := []learning.MistakeEventType{
		learning.MistakeObservedEvent, learning.MistakeObservedEvent, learning.MistakeReinforcedEvent,
		learning.MistakeResolvedEvent, learning.MistakeObservedEvent,
	}
	for index, want := range wantTypes {
		if view.History[index].Type != want {
			t.Fatalf("history[%d].Type = %q, want %q", index, view.History[index].Type, want)
		}
	}
	listed, err := service.List(ctx)
	if err != nil || len(listed) != 1 || listed[0].ID != created.Mistake.ID {
		t.Fatalf("List() = (%+v, %v)", listed, err)
	}
}

func TestMistakeMemoryRejectsKeyCollisionsAndRollsBackUnknownConcept(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students),
		application.WithProfileClock(func() time.Time { return mistakeTime(1) }))
	_, _ = profiles.Show(ctx)
	conceptID := testID(t, "concept.mean")
	if err := store.SeedCurriculum(learning.CurriculumRef{ID: testID(t, "curriculum.stats"), Version: "1.0.0"},
		[]learning.Concept{{ID: conceptID, TopicID: testID(t, "topic.mean"), Title: "Mean"}}, nil); err != nil {
		t.Fatal(err)
	}
	sequence := 0
	service := application.NewMistakeMemoryService(profiles, store, application.WithMistakeMemoryIDGenerator(func(kind string) (learning.ID, error) {
		sequence++
		return learning.NewID(fmt.Sprintf("%s.rollback.%02d", kind, sequence))
	}))
	input := application.RecordMistakeInput{ConceptID: conceptID, Key: "stable", Category: learning.MistakeConceptual,
		Summary: "Original summary", ObservedAt: testTimestamp(t, 2), SourceRef: "fixture/1"}
	if _, err := service.Record(ctx, input); err != nil {
		t.Fatal(err)
	}
	input.Summary = "Different meaning"
	input.ObservedAt = testTimestamp(t, 3)
	if _, err := service.Record(ctx, input); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("key collision error = %v, want conflict", err)
	}
	input.ConceptID = testID(t, "concept.unknown")
	input.Key = "unknown"
	input.Summary = "Unknown concept"
	if _, err := service.Record(ctx, input); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("unknown concept error = %v, want invalid state", err)
	}
	items, err := service.List(ctx)
	if err != nil || len(items) != 1 || items[0].Occurrences != 1 {
		t.Fatalf("List() after rollback = (%+v, %v)", items, err)
	}
}

func mistakeTime(hour int64) time.Time {
	return time.Unix(hour*3600, 0).UTC()
}
