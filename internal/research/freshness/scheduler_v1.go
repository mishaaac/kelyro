package freshness

import (
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

type SchedulingInput struct {
	Assessment         Assessment
	ConflictUnresolved bool
	SecuritySensitive  bool
	ManualRequest      bool
}

type Schedule struct {
	LastVerifiedAt   research.Timestamp
	NextVerifyAt     research.Timestamp
	Reason           research.VerificationReason
	Priority         research.VerificationPriority
	AlgorithmVersion string
}

func (schedule Schedule) Validate() error {
	if err := schedule.LastVerifiedAt.Validate(); err != nil {
		return fmt.Errorf("schedule last verified: %w", err)
	}
	if err := schedule.NextVerifyAt.Validate(); err != nil {
		return fmt.Errorf("schedule next verify: %w", err)
	}
	if schedule.NextVerifyAt.Before(schedule.LastVerifiedAt) {
		return fmt.Errorf("schedule next verification precedes last verification")
	}
	if err := schedule.Reason.Validate(); err != nil {
		return err
	}
	if err := schedule.Priority.Validate(); err != nil {
		return err
	}
	if schedule.AlgorithmVersion != research.RefreshSchedulingAlgorithmV1 {
		return fmt.Errorf("schedule algorithm version must be %q", research.RefreshSchedulingAlgorithmV1)
	}
	return nil
}

// ScheduleV1 derives deterministic re-verification state from a known
// freshness assessment. It performs no I/O and starts no background work.
func ScheduleV1(input SchedulingInput) (Schedule, error) {
	if err := input.Assessment.Validate(); err != nil {
		return Schedule{}, fmt.Errorf("schedule refresh-scheduling-v1: assessment: %w", err)
	}
	assessment := input.Assessment
	if assessment.State == research.FreshnessUnknown || assessment.LastVerifiedAt == nil {
		return Schedule{}, fmt.Errorf("schedule refresh-scheduling-v1: unknown freshness has no last verification")
	}

	next, err := research.NewTimestamp(assessment.LastVerifiedAt.Time().Add(time.Duration(assessment.EffectiveTTLDays) * 24 * time.Hour))
	if err != nil {
		return Schedule{}, fmt.Errorf("schedule refresh-scheduling-v1: TTL deadline: %w", err)
	}
	reason := research.VerificationTTLExpired
	priority := research.VerificationPriorityNormal

	// Event triggers are immediately due. Precedence is deterministic and
	// preserves the most actionable reason when more than one signal is set.
	switch {
	case input.ManualRequest:
		next, reason, priority = assessment.EvaluatedAt, research.VerificationManualRequest, research.VerificationPriorityCritical
	case input.SecuritySensitive:
		next, reason, priority = assessment.EvaluatedAt, research.VerificationSecuritySensitive, research.VerificationPriorityCritical
	case input.ConflictUnresolved:
		next, reason, priority = assessment.EvaluatedAt, research.VerificationConflictUnresolved, research.VerificationPriorityHigh
	case assessmentHasReason(assessment, ReasonSourceUpdated):
		next, reason, priority = assessment.EvaluatedAt, research.VerificationSourceChanged, research.VerificationPriorityHigh
	case assessmentHasReason(assessment, ReasonKnownNewRelease):
		next, reason, priority = assessment.EvaluatedAt, research.VerificationNewRelease, research.VerificationPriorityHigh
	}

	schedule := Schedule{
		LastVerifiedAt: *assessment.LastVerifiedAt, NextVerifyAt: next,
		Reason: reason, Priority: priority, AlgorithmVersion: research.RefreshSchedulingAlgorithmV1,
	}
	if err := schedule.Validate(); err != nil {
		return Schedule{}, fmt.Errorf("schedule refresh-scheduling-v1: %w", err)
	}
	return schedule, nil
}

func assessmentHasReason(assessment Assessment, code ReasonCode) bool {
	for _, reason := range assessment.Reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}
