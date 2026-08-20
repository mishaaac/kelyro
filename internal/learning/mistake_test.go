package learning

import (
	"strings"
	"testing"
	"time"
)

func TestMistakeLifecyclePreservesFirstSeenAndReopensAfterRecurrence(t *testing.T) {
	t.Parallel()
	first := mustTimestamp(t, 1)
	second := mustTimestamp(t, 2)
	third := mustTimestamp(t, 3)
	fourth := mustTimestamp(t, 4)
	mistake, err := NewMistake(mustID(t, "mistake.mean"), mustID(t, "student.ada"), mustID(t, "concept.mean"),
		MistakeKey("mean-vs-median"), MistakeMisconception, "Confused mean and median", first, "fixture/check/1")
	if err != nil {
		t.Fatal(err)
	}
	mistake, err = mistake.Observe(second, "fixture/check/2")
	if err != nil || mistake.Occurrences != 2 || mistake.FirstSeenAt != first || mistake.LastSeenAt != second {
		t.Fatalf("Observe() = (%+v, %v)", mistake, err)
	}
	mistake, err = mistake.Reinforce(third)
	if err != nil || mistake.Status != MistakeReinforced {
		t.Fatalf("Reinforce() = (%+v, %v)", mistake, err)
	}
	mistake, err = mistake.Resolve(third)
	if err != nil || mistake.Status != MistakeResolved || mistake.ResolvedAt == nil {
		t.Fatalf("Resolve() = (%+v, %v)", mistake, err)
	}
	mistake, err = mistake.Observe(fourth, "fixture/check/4")
	if err != nil || mistake.Status != MistakeRecent || mistake.ResolvedAt != nil || mistake.Occurrences != 3 {
		t.Fatalf("reopen Observe() = (%+v, %v)", mistake, err)
	}
}

func TestMistakeRejectsInvalidCategoriesChronologyAndOversizedContent(t *testing.T) {
	t.Parallel()
	first := mustTimestamp(t, 2)
	id := mustID(t, "mistake.invalid")
	studentID := mustID(t, "student.ada")
	conceptID := mustID(t, "concept.mean")
	for name, test := range map[string]struct {
		key      MistakeKey
		category MistakeCategory
		summary  string
		source   string
	}{
		"empty key":      {category: MistakeUnknown, summary: "summary", source: "source"},
		"bad category":   {key: "key", category: "programming-only", summary: "summary", source: "source"},
		"long summary":   {key: "key", category: MistakeUnknown, summary: strings.Repeat("x", MaxMistakeSummaryLength+1), source: "source"},
		"long source":    {key: "key", category: MistakeUnknown, summary: "summary", source: strings.Repeat("x", MaxMistakeSourceLength+1)},
		"padded summary": {key: "key", category: MistakeUnknown, summary: " summary ", source: "source"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewMistake(id, studentID, conceptID, test.key, test.category, test.summary, first, test.source); err == nil {
				t.Fatal("NewMistake() accepted invalid input")
			}
		})
	}

	mistake, _ := NewMistake(id, studentID, conceptID, "key", MistakeConceptual, "summary", first, "source")
	earlier, _ := NewTimestamp(first.Time().Add(-time.Minute))
	if _, err := mistake.Observe(earlier, "source"); err == nil {
		t.Fatal("Observe() accepted a recurrence before first seen")
	}
	resolved, _ := mistake.Resolve(first)
	if _, err := resolved.Resolve(first); err == nil {
		t.Fatal("Resolve() accepted an already resolved mistake")
	}
	if _, err := resolved.Reinforce(first); err == nil {
		t.Fatal("Reinforce() accepted a resolved mistake without recurrence")
	}
}

func TestMistakeEventRequiresTraceableBoundedSource(t *testing.T) {
	t.Parallel()
	event, err := NewMistakeEvent(mustID(t, "event.1"), mustID(t, "mistake.1"), MistakeObservedEvent, mustTimestamp(t, 1), "fixture/evaluator")
	if err != nil || event.Type != MistakeObservedEvent {
		t.Fatalf("NewMistakeEvent() = (%+v, %v)", event, err)
	}
	if _, err := NewMistakeEvent(mustID(t, "event.2"), mustID(t, "mistake.1"), "deleted", mustTimestamp(t, 1), "fixture/evaluator"); err == nil {
		t.Fatal("NewMistakeEvent() accepted invalid type")
	}
}
