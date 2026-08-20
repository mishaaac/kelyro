package learning

import (
	"reflect"
	"testing"
)

func TestProgressionV1UsesExplicitEvidenceStagesAndExactThreshold(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.progression")
	conceptID := mustID(t, "concept.progression")
	instanceID := mustID(t, "instance.progression")
	startedAt := mustTimestamp(t, 8)
	updatedAt := mustTimestamp(t, 12)
	threshold := progressionThreshold(t, .8)

	tests := []struct {
		name         string
		evidenceType EvidenceType
		score        float64
		wantExposure ExposureState
		wantMet      bool
	}{
		{name: "diagnostic introduces", evidenceType: EvidenceDiagnosticObjective, score: .79, wantExposure: ExposureIntroduced},
		{name: "knowledge check learns", evidenceType: EvidenceKnowledgeCheck, score: .79, wantExposure: ExposureLearning},
		{name: "practice practices", evidenceType: EvidencePracticeSuccess, score: .79, wantExposure: ExposurePracticing},
		{name: "exact threshold masters", evidenceType: EvidenceAssessment, score: .8, wantExposure: ExposureMastered, wantMet: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := InstanceConceptState{
				CurriculumInstanceID: instanceID, StudentID: studentID, ConceptID: conceptID,
				Exposure: ExposureNotSeen, Mastery: mustScore(t, 0), UpdatedAt: startedAt,
			}
			item := masteryEvidence(t, string(test.evidenceType), studentID, conceptID, test.evidenceType, test.score, DefaultEvidenceMetadata(), 10)
			calculation, err := CalculateMasteryV1(studentID, conceptID, []Evidence{item})
			if err != nil {
				t.Fatal(err)
			}
			progression, err := ApplyProgressionV1(state, calculation, threshold, startedAt, updatedAt)
			if err != nil {
				t.Fatal(err)
			}
			if progression.State.Exposure != test.wantExposure || progression.ThresholdMet != test.wantMet ||
				progression.State.Mastery.Value() != test.score || progression.PolicyVersion != ProgressionPolicyVersion {
				t.Fatalf("ApplyProgressionV1() = %+v", progression)
			}
			if progression.State.FirstSeenAt == nil || progression.State.LastSeenAt == nil ||
				progression.State.FirstSeenAt.Time() != item.ObservedAt.Time() || progression.State.LastSeenAt.Time() != item.ObservedAt.Time() {
				t.Fatalf("progression observation timestamps = %+v", progression.State)
			}
			if test.wantMet && (progression.State.MasteredAt == nil || progression.State.MasteredAt.Time() != item.ObservedAt.Time()) {
				t.Fatalf("mastered timestamp = %+v", progression.State.MasteredAt)
			}
		})
	}
}

func TestProgressionV1UsesFurthestObservedLearningStage(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.progression")
	conceptID := mustID(t, "concept.progression")
	startedAt := mustTimestamp(t, 8)
	state := InstanceConceptState{
		CurriculumInstanceID: mustID(t, "instance.progression"), StudentID: studentID, ConceptID: conceptID,
		Exposure: ExposureNotSeen, Mastery: mustScore(t, 0), UpdatedAt: startedAt,
	}
	items := []Evidence{
		masteryEvidence(t, "practice.early", studentID, conceptID, EvidencePracticeFailure, 0, DefaultEvidenceMetadata(), 9),
		masteryEvidence(t, "diagnostic.later", studentID, conceptID, EvidenceDiagnosticSelfReport, .2, DefaultEvidenceMetadata(), 10),
	}
	calculation, err := CalculateMasteryV1(studentID, conceptID, items)
	if err != nil {
		t.Fatal(err)
	}
	progression, err := ApplyProgressionV1(state, calculation, progressionThreshold(t, .8), startedAt, mustTimestamp(t, 12))
	if err != nil {
		t.Fatal(err)
	}
	if progression.State.Exposure != ExposurePracticing {
		t.Fatalf("exposure = %q, want practicing", progression.State.Exposure)
	}
}

func TestProgressionV1MasteryIsReversibleAndPreservesHistoricalMastery(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.progression")
	conceptID := mustID(t, "concept.progression")
	startedAt := mustTimestamp(t, 8)
	seenAt := mustTimestamp(t, 9)
	masteredAt := mustTimestamp(t, 10)
	state := InstanceConceptState{
		CurriculumInstanceID: mustID(t, "instance.progression"), StudentID: studentID, ConceptID: conceptID,
		Exposure: ExposureMastered, Mastery: mustScore(t, .8), FirstSeenAt: &seenAt,
		LastSeenAt: &masteredAt, MasteredAt: &masteredAt, UpdatedAt: masteredAt,
	}
	item := masteryEvidence(t, "recalculated", studentID, conceptID, EvidenceAssessment, .79, DefaultEvidenceMetadata(), 11)
	calculation, err := CalculateMasteryV1(studentID, conceptID, []Evidence{item})
	if err != nil {
		t.Fatal(err)
	}
	progression, err := ApplyProgressionV1(state, calculation, progressionThreshold(t, .8), startedAt, mustTimestamp(t, 12))
	if err != nil {
		t.Fatal(err)
	}
	if progression.ThresholdMet || progression.State.Exposure != ExposurePracticing || progression.State.MasteredAt == nil || *progression.State.MasteredAt != masteredAt {
		t.Fatalf("reversible mastery = %+v", progression)
	}
}

func TestProgressionV1LeavesReviewDueLifecycleToRetention(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.progression")
	conceptID := mustID(t, "concept.progression")
	startedAt := mustTimestamp(t, 8)
	seenAt := mustTimestamp(t, 9)
	masteredAt := mustTimestamp(t, 10)
	reviewDueAt := mustTimestamp(t, 11)
	state := InstanceConceptState{
		CurriculumInstanceID: mustID(t, "instance.progression"), StudentID: studentID, ConceptID: conceptID,
		Exposure: ExposureReviewDue, Mastery: mustScore(t, .7), FirstSeenAt: &seenAt, LastSeenAt: &masteredAt,
		MasteredAt: &masteredAt, ReviewDueAt: &reviewDueAt, UpdatedAt: reviewDueAt,
	}
	item := masteryEvidence(t, "review", studentID, conceptID, EvidenceReviewRecall, 1, DefaultEvidenceMetadata(), 11)
	calculation, err := CalculateMasteryV1(studentID, conceptID, []Evidence{item})
	if err != nil {
		t.Fatal(err)
	}
	progression, err := ApplyProgressionV1(state, calculation, progressionThreshold(t, .8), startedAt, mustTimestamp(t, 12))
	if err != nil {
		t.Fatal(err)
	}
	if !progression.ThresholdMet || progression.State.Exposure != ExposureReviewDue || progression.State.ReviewDueAt == nil || *progression.State.ReviewDueAt != reviewDueAt {
		t.Fatalf("review-due progression = %+v", progression)
	}
}

func TestProgressionV1UnknownIsNotZeroAndDoesNotMutateState(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.progression")
	conceptID := mustID(t, "concept.progression")
	startedAt := mustTimestamp(t, 8)
	state := InstanceConceptState{
		CurriculumInstanceID: mustID(t, "instance.progression"), StudentID: studentID, ConceptID: conceptID,
		Exposure: ExposureNotSeen, Mastery: mustScore(t, 0), UpdatedAt: startedAt,
	}
	calculation, err := CalculateMasteryV1(studentID, conceptID, nil)
	if err != nil {
		t.Fatal(err)
	}
	progression, err := ApplyProgressionV1(state, calculation, progressionThreshold(t, .8), startedAt, mustTimestamp(t, 12))
	if err != nil {
		t.Fatal(err)
	}
	if progression.StateChanged || progression.ThresholdMet || !reflect.DeepEqual(progression.State, state) {
		t.Fatalf("unknown progression = %+v", progression)
	}
}

func TestProgressionV1ClampsLongitudinalEvidenceAndRejectsFutureEvidence(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.progression")
	conceptID := mustID(t, "concept.progression")
	startedAt := mustTimestamp(t, 8)
	state := InstanceConceptState{
		CurriculumInstanceID: mustID(t, "instance.progression"), StudentID: studentID, ConceptID: conceptID,
		Exposure: ExposureNotSeen, Mastery: mustScore(t, 0), UpdatedAt: startedAt,
	}
	old := masteryEvidence(t, "old", studentID, conceptID, EvidenceManualImport, .6, DefaultEvidenceMetadata(), 7)
	calculation, err := CalculateMasteryV1(studentID, conceptID, []Evidence{old})
	if err != nil {
		t.Fatal(err)
	}
	progression, err := ApplyProgressionV1(state, calculation, progressionThreshold(t, .8), startedAt, mustTimestamp(t, 10))
	if err != nil {
		t.Fatal(err)
	}
	if progression.State.FirstSeenAt == nil || *progression.State.FirstSeenAt != startedAt || *progression.State.LastSeenAt != startedAt {
		t.Fatalf("clamped state = %+v", progression.State)
	}

	future := masteryEvidence(t, "future", studentID, conceptID, EvidenceAssessment, 1, DefaultEvidenceMetadata(), 12)
	futureCalculation, err := CalculateMasteryV1(studentID, conceptID, []Evidence{future})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyProgressionV1(state, futureCalculation, progressionThreshold(t, .8), startedAt, mustTimestamp(t, 11)); err == nil {
		t.Fatal("ApplyProgressionV1() accepted future evidence")
	}
}

func TestProgressionV1RejectsMalformedMasteryCalculation(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.progression")
	conceptID := mustID(t, "concept.progression")
	startedAt := mustTimestamp(t, 8)
	state := InstanceConceptState{
		CurriculumInstanceID: mustID(t, "instance.progression"), StudentID: studentID, ConceptID: conceptID,
		Exposure: ExposureNotSeen, Mastery: mustScore(t, 0), UpdatedAt: startedAt,
	}
	item := masteryEvidence(t, "malformed-calculation", studentID, conceptID, EvidenceAssessment, .8, DefaultEvidenceMetadata(), 9)
	calculation, err := CalculateMasteryV1(studentID, conceptID, []Evidence{item})
	if err != nil {
		t.Fatal(err)
	}
	calculation.TotalWeight += .1
	if _, err := ApplyProgressionV1(state, calculation, progressionThreshold(t, .8), startedAt, mustTimestamp(t, 10)); err == nil {
		t.Fatal("ApplyProgressionV1() accepted malformed mastery aggregates")
	}
}

func progressionThreshold(t *testing.T, value float64) ResolvedMasteryThreshold {
	t.Helper()
	requirement, err := NewMasteryRequirement(value)
	if err != nil {
		t.Fatal(err)
	}
	return ResolvedMasteryThreshold{Requirement: requirement, Source: MasterySourceStudentDefault, PolicyVersion: MasteryThresholdPolicyVersion}
}
