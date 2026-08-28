package trigger

import (
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
)

type Signals struct {
	Manual                     bool
	EvidenceCount              int
	FreshnessState             *research.FreshnessState
	NextVerifyAt               *research.Timestamp
	NewTechnologyRelease       bool
	DeprecationDetected        bool
	UnresolvedConflicts        int
	CurriculumCompileRequested bool
	SecuritySensitiveRefresh   bool
}

func (signals Signals) Validate() error {
	if signals.EvidenceCount < 0 {
		return fmt.Errorf("research trigger evidence count is negative")
	}
	if signals.UnresolvedConflicts < 0 {
		return fmt.Errorf("research trigger unresolved conflict count is negative")
	}
	if signals.FreshnessState != nil {
		if err := signals.FreshnessState.Validate(); err != nil {
			return err
		}
	}
	if signals.NextVerifyAt != nil {
		if err := signals.NextVerifyAt.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type Input struct {
	QueueID research.ID
	Request research.ResearchRequest
	Signals Signals
	AsOf    research.Timestamp
}

func (input Input) Validate() error {
	if err := input.QueueID.Validate(); err != nil {
		return fmt.Errorf("research trigger queue id: %w", err)
	}
	if err := input.Request.Validate(); err != nil {
		return err
	}
	if err := input.Signals.Validate(); err != nil {
		return err
	}
	if err := input.AsOf.Validate(); err != nil {
		return err
	}
	if input.AsOf.Before(input.Request.RequestedAt) {
		return fmt.Errorf("research trigger evaluation precedes request")
	}
	return nil
}

type Decision struct {
	ShouldResearch   bool
	Triggers         []research.ResearchTrigger
	Priority         research.VerificationPriority
	QueueItem        *research.ResearchQueueItem
	AlgorithmVersion string
}

func EvaluateV1(input Input) (Decision, error) {
	if err := input.Validate(); err != nil {
		return Decision{}, fmt.Errorf("evaluate research-trigger-v1: %w", err)
	}
	triggers := collectTriggers(input.Signals, input.AsOf)
	decision := Decision{Triggers: triggers, AlgorithmVersion: research.ResearchTriggerAlgorithmV1}
	if len(triggers) == 0 {
		return decision, nil
	}
	decision.ShouldResearch = true
	decision.Priority = priorityFor(triggers)
	dedupeKey, err := research.ResearchTriggerDedupeKeyV1(input.Request)
	if err != nil {
		return Decision{}, fmt.Errorf("evaluate research-trigger-v1: %w", err)
	}
	item := research.ResearchQueueItem{
		ID: input.QueueID, Request: input.Request, Triggers: append([]research.ResearchTrigger(nil), triggers...),
		Priority: decision.Priority, DedupeKey: dedupeKey, Status: research.ResearchQueueQueued,
		QueuedAt: input.AsOf, AlgorithmVersion: research.ResearchTriggerAlgorithmV1,
	}
	if err := item.Validate(); err != nil {
		return Decision{}, fmt.Errorf("evaluate research-trigger-v1: %w", err)
	}
	decision.QueueItem = &item
	return decision, nil
}

func collectTriggers(signals Signals, asOf research.Timestamp) []research.ResearchTrigger {
	triggers := make([]research.ResearchTrigger, 0, 8)
	if signals.Manual {
		triggers = append(triggers, research.ResearchTriggerManual)
	}
	if signals.SecuritySensitiveRefresh {
		triggers = append(triggers, research.ResearchTriggerSecurityRefresh)
	}
	if signals.DeprecationDetected {
		triggers = append(triggers, research.ResearchTriggerDeprecation)
	}
	if signals.UnresolvedConflicts > 0 {
		triggers = append(triggers, research.ResearchTriggerConflict)
	}
	if signals.NewTechnologyRelease {
		triggers = append(triggers, research.ResearchTriggerNewRelease)
	}
	if freshnessExpired(signals, asOf) {
		triggers = append(triggers, research.ResearchTriggerFreshnessExpired)
	}
	if signals.EvidenceCount == 0 {
		triggers = append(triggers, research.ResearchTriggerMissingEvidence)
	}
	if signals.CurriculumCompileRequested {
		triggers = append(triggers, research.ResearchTriggerCurriculumCompile)
	}
	return triggers
}

func freshnessExpired(signals Signals, asOf research.Timestamp) bool {
	if signals.FreshnessState != nil && *signals.FreshnessState == research.FreshnessStale {
		return true
	}
	return signals.NextVerifyAt != nil && !asOf.Before(*signals.NextVerifyAt)
}

func priorityFor(triggers []research.ResearchTrigger) research.VerificationPriority {
	priority := research.VerificationPriorityNormal
	for _, item := range triggers {
		switch item {
		case research.ResearchTriggerManual, research.ResearchTriggerSecurityRefresh:
			return research.VerificationPriorityCritical
		case research.ResearchTriggerDeprecation, research.ResearchTriggerConflict, research.ResearchTriggerNewRelease:
			priority = research.VerificationPriorityHigh
		}
	}
	return priority
}
