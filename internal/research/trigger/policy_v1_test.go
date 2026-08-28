package trigger_test

import (
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/trigger"
)

func TestResearchTriggerV1CombinesAllSignalsInStableOrder(t *testing.T) {
	t.Parallel()
	input := triggerFixture(t)
	stale := research.FreshnessStale
	input.Signals = trigger.Signals{
		Manual: true, EvidenceCount: 0, FreshnessState: &stale, NewTechnologyRelease: true,
		DeprecationDetected: true, UnresolvedConflicts: 2, CurriculumCompileRequested: true,
		SecuritySensitiveRefresh: true,
	}
	decision, err := trigger.EvaluateV1(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []research.ResearchTrigger{
		research.ResearchTriggerManual, research.ResearchTriggerSecurityRefresh,
		research.ResearchTriggerDeprecation, research.ResearchTriggerConflict,
		research.ResearchTriggerNewRelease, research.ResearchTriggerFreshnessExpired,
		research.ResearchTriggerMissingEvidence, research.ResearchTriggerCurriculumCompile,
	}
	if !decision.ShouldResearch || decision.Priority != research.VerificationPriorityCritical || decision.QueueItem == nil || len(decision.Triggers) != len(want) {
		t.Fatalf("decision = %+v", decision)
	}
	for index := range want {
		if decision.Triggers[index] != want[index] {
			t.Fatalf("trigger %d = %q, want %q", index, decision.Triggers[index], want[index])
		}
	}
	if err := decision.QueueItem.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestResearchTriggerV1DoesNotQueueFreshSufficientEvidence(t *testing.T) {
	t.Parallel()
	input := triggerFixture(t)
	fresh := research.FreshnessFresh
	next, _ := research.NewTimestamp(input.AsOf.Time().Add(time.Hour))
	input.Signals = trigger.Signals{EvidenceCount: 3, FreshnessState: &fresh, NextVerifyAt: &next}
	decision, err := trigger.EvaluateV1(input)
	if err != nil || decision.ShouldResearch || decision.QueueItem != nil || len(decision.Triggers) != 0 {
		t.Fatalf("no-trigger decision = (%+v,%v)", decision, err)
	}
	next = input.AsOf
	input.Signals.NextVerifyAt = &next
	decision, err = trigger.EvaluateV1(input)
	if err != nil || !decision.ShouldResearch || decision.Triggers[0] != research.ResearchTriggerFreshnessExpired {
		t.Fatalf("expiry-boundary decision = (%+v,%v)", decision, err)
	}
}

func triggerFixture(t *testing.T) trigger.Input {
	t.Helper()
	at, _ := research.NewTimestamp(time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	topic, _ := research.NewResearchTopic("range-over-func", "software", "go")
	requestID, _ := research.NewID("request.trigger-policy")
	queueID, _ := research.NewID("queue.trigger-policy")
	return trigger.Input{
		QueueID: queueID, AsOf: at,
		Request: research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: at},
	}
}
