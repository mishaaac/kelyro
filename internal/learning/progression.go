package learning

import "fmt"

const ProgressionPolicyVersion = "progression-v1"

// ConceptProgression is the pure result of connecting a mastery calculation
// to one instance-scoped concept state. Unlock eligibility is derived
// separately by KnowledgeGraph from the resulting snapshot.
type ConceptProgression struct {
	State         InstanceConceptState
	Mastery       MasteryCalculation
	Threshold     ResolvedMasteryThreshold
	ThresholdMet  bool
	StateChanged  bool
	PolicyVersion string
}

// ApplyProgressionV1 updates mastery and exposure without owning retention.
// instanceStartedAt is the earliest timestamp that instance-local exposure may
// record when longitudinal evidence predates the curriculum instance.
func ApplyProgressionV1(state InstanceConceptState, calculation MasteryCalculation, threshold ResolvedMasteryThreshold, instanceStartedAt, updatedAt Timestamp) (ConceptProgression, error) {
	if err := state.Validate(); err != nil {
		return ConceptProgression{}, fmt.Errorf("apply progression: %w", err)
	}
	if err := calculation.Validate(); err != nil {
		return ConceptProgression{}, fmt.Errorf("apply progression: %w", err)
	}
	if calculation.StudentID != state.StudentID || calculation.ConceptID != state.ConceptID {
		return ConceptProgression{}, fmt.Errorf("apply progression: mastery calculation belongs to another state")
	}
	if err := threshold.Validate(); err != nil {
		return ConceptProgression{}, fmt.Errorf("apply progression: %w", err)
	}
	if err := instanceStartedAt.Validate(); err != nil {
		return ConceptProgression{}, fmt.Errorf("apply progression instance start: %w", err)
	}
	if err := updatedAt.Validate(); err != nil {
		return ConceptProgression{}, fmt.Errorf("apply progression update: %w", err)
	}
	if updatedAt.Before(instanceStartedAt) || updatedAt.Before(state.UpdatedAt) {
		return ConceptProgression{}, fmt.Errorf("apply progression update precedes instance or current state")
	}

	result := ConceptProgression{
		State: state, Mastery: calculation, Threshold: threshold,
		PolicyVersion: ProgressionPolicyVersion,
	}
	if !calculation.Known {
		return result, nil
	}

	firstSeen := calculation.Contributions[0].OccurredAt
	lastSeen := calculation.Contributions[len(calculation.Contributions)-1].OccurredAt
	if firstSeen.Before(instanceStartedAt) {
		firstSeen = instanceStartedAt
	}
	if lastSeen.Before(instanceStartedAt) {
		lastSeen = instanceStartedAt
	}
	if lastSeen.After(updatedAt) {
		return ConceptProgression{}, fmt.Errorf("apply progression evidence occurs after update")
	}
	masteryObservedAt := lastSeen
	if state.FirstSeenAt != nil && state.FirstSeenAt.Before(firstSeen) {
		firstSeen = *state.FirstSeenAt
	}
	if state.LastSeenAt != nil && state.LastSeenAt.After(lastSeen) {
		lastSeen = *state.LastSeenAt
	}

	updated := state
	updated.Mastery = calculation.Score
	updated.FirstSeenAt = &firstSeen
	updated.LastSeenAt = &lastSeen
	updated.UpdatedAt = updatedAt
	result.ThresholdMet = threshold.Requirement.SatisfiedBy(calculation.Score)

	// Retention owns review_due and its schedule. Recalculation may change the
	// score while leaving that lifecycle state intact for a later review policy.
	if state.Exposure != ExposureReviewDue {
		switch {
		case result.ThresholdMet:
			updated.Exposure = ExposureMastered
			if updated.MasteredAt == nil {
				masteredAt := masteryObservedAt
				updated.MasteredAt = &masteredAt
			}
		case state.MasteredAt != nil || state.Exposure == ExposureMastered:
			updated.Exposure = ExposurePracticing
		default:
			updated.Exposure = furthestLearningExposure(state.Exposure, calculation.Contributions)
		}
	}
	if err := updated.Validate(); err != nil {
		return ConceptProgression{}, fmt.Errorf("apply progression result: %w", err)
	}
	result.StateChanged = true
	result.State = updated
	return result, nil
}

func furthestLearningExposure(current ExposureState, contributions []MasteryContribution) ExposureState {
	best := current
	if progressionExposureRank(best) < 0 {
		best = ExposureNotSeen
	}
	for _, contribution := range contributions {
		candidate := exposureForEvidence(contribution.Type)
		if progressionExposureRank(candidate) > progressionExposureRank(best) {
			best = candidate
		}
	}
	return best
}

func exposureForEvidence(evidenceType EvidenceType) ExposureState {
	switch evidenceType {
	case EvidenceDiagnosticObjective, EvidenceDiagnosticSelfReport, EvidenceManualImport:
		return ExposureIntroduced
	case EvidenceKnowledgeCheck:
		return ExposureLearning
	case EvidencePracticeSuccess, EvidencePracticeFailure, EvidenceAssessment, EvidenceProject, EvidenceReviewRecall:
		return ExposurePracticing
	default:
		return ExposureNotSeen
	}
}

func progressionExposureRank(exposure ExposureState) int {
	switch exposure {
	case ExposureNotSeen:
		return 0
	case ExposureIntroduced:
		return 1
	case ExposureLearning:
		return 2
	case ExposurePracticing:
		return 3
	default:
		return -1
	}
}
