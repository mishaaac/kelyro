package research_test

import (
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestResearchTriggerDedupeKeyIgnoresRequestIdentity(t *testing.T) {
	t.Parallel()
	at, _ := research.NewTimestamp(time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	topic, _ := research.NewResearchTopic("Range Over Func", "software", "Go")
	firstID, _ := research.NewID("request.trigger-one")
	secondID, _ := research.NewID("request.trigger-two")
	first := research.ResearchRequest{ID: firstID, Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: at}
	second := first
	second.ID = secondID
	second.RequestedAt, _ = research.NewTimestamp(at.Time().Add(time.Minute))
	firstKey, err := research.ResearchTriggerDedupeKeyV1(first)
	if err != nil {
		t.Fatal(err)
	}
	secondKey, err := research.ResearchTriggerDedupeKeyV1(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstKey != secondKey || len(firstKey) != 72 {
		t.Fatalf("dedupe keys = %q and %q", firstKey, secondKey)
	}
}
