package learning

import (
	"reflect"
	"testing"
	"time"
)

func TestRetentionV1NewConceptIsUnknown(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.retention")
	conceptID := mustID(t, "concept.memory")
	mastery, err := CalculateMasteryV1(studentID, conceptID, nil)
	if err != nil {
		t.Fatal(err)
	}
	calculation, err := CalculateRetentionV1(mastery, nil, retentionTimestamp(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	state := calculation.State
	if state.Status != RetentionUnknown || state.NextDueAt != nil || state.LastPractice != nil ||
		state.StabilityEstimate != 0 || state.Strength.Value() != 0 || state.AlgorithmVersion != RetentionAlgorithmVersion {
		t.Fatalf("unknown retention = %+v", state)
	}
}

func TestRetentionV1MasteredTodayIsFreshAndDueLater(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.retention")
	conceptID := mustID(t, "concept.memory")
	observedAt := retentionTimestamp(t, 0)
	item := retentionEvidence(t, "mastered", studentID, conceptID, EvidenceAssessment, 1, .5, observedAt)
	mastery := retentionMastery(t, studentID, conceptID, []Evidence{item})

	calculation, err := CalculateRetentionV1(mastery, []Evidence{item}, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	state := calculation.State
	if state.Status != RetentionFresh || state.LastSuccessfulRecall == nil || *state.LastSuccessfulRecall != observedAt ||
		state.NextDueAt == nil || !state.NextDueAt.After(observedAt) || state.StabilityEstimate != 7*24*time.Hour {
		t.Fatalf("fresh retention = %+v", state)
	}

	twoDaysLater := retentionTimestamp(t, 48*time.Hour)
	later, err := CalculateRetentionV1(mastery, []Evidence{item}, twoDaysLater)
	if err != nil {
		t.Fatal(err)
	}
	if later.State.Status != RetentionStable || !later.State.NextDueAt.After(twoDaysLater) {
		t.Fatalf("later retention = %+v", later.State)
	}
}

func TestRetentionV1DueAndOverdueBoundaries(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.retention")
	conceptID := mustID(t, "concept.memory")
	observedAt := retentionTimestamp(t, 0)
	item := retentionEvidence(t, "boundary", studentID, conceptID, EvidenceKnowledgeCheck, .9, .5, observedAt)
	mastery := retentionMastery(t, studentID, conceptID, []Evidence{item})
	initial, err := CalculateRetentionV1(mastery, []Evidence{item}, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	dueAt := *initial.State.NextDueAt

	due, err := CalculateRetentionV1(mastery, []Evidence{item}, dueAt)
	if err != nil || due.State.Status != RetentionDue {
		t.Fatalf("at due boundary = (%+v, %v)", due.State, err)
	}
	overdueAt, _ := NewTimestamp(dueAt.Time().Add(initial.State.StabilityEstimate + time.Second))
	overdue, err := CalculateRetentionV1(mastery, []Evidence{item}, overdueAt)
	if err != nil || overdue.State.Status != RetentionOverdue {
		t.Fatalf("after overdue boundary = (%+v, %v)", overdue.State, err)
	}
}

func TestRetentionV1SuccessfulReviewExtendsAndFailureShortens(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.retention")
	conceptID := mustID(t, "concept.memory")
	base := retentionEvidence(t, "base", studentID, conceptID, EvidenceAssessment, .9, .5, retentionTimestamp(t, 0))
	baselineMastery := retentionMastery(t, studentID, conceptID, []Evidence{base})
	baseline, err := CalculateRetentionV1(baselineMastery, []Evidence{base}, retentionTimestamp(t, 0))
	if err != nil {
		t.Fatal(err)
	}

	success := retentionEvidence(t, "success", studentID, conceptID, EvidenceReviewRecall, .9, .5, retentionTimestamp(t, 24*time.Hour))
	successItems := []Evidence{success, base}
	successful, err := CalculateRetentionV1(retentionMastery(t, studentID, conceptID, successItems), successItems, success.ObservedAt)
	if err != nil {
		t.Fatal(err)
	}
	failure := retentionEvidence(t, "failure", studentID, conceptID, EvidenceReviewRecall, .2, .5, retentionTimestamp(t, 24*time.Hour))
	failureItems := []Evidence{base, failure}
	failed, err := CalculateRetentionV1(retentionMastery(t, studentID, conceptID, failureItems), failureItems, failure.ObservedAt)
	if err != nil {
		t.Fatal(err)
	}

	if successful.State.StabilityEstimate <= baseline.State.StabilityEstimate ||
		failed.State.StabilityEstimate >= baseline.State.StabilityEstimate {
		t.Fatalf("stability baseline=%s success=%s failure=%s", baseline.State.StabilityEstimate, successful.State.StabilityEstimate, failed.State.StabilityEstimate)
	}
	if successful.State.SuccessfulReviews != 1 || successful.State.FailedReviews != 0 ||
		failed.State.SuccessfulReviews != 0 || failed.State.FailedReviews != 1 || failed.State.Status != RetentionWeakening {
		t.Fatalf("review outcomes success=%+v failure=%+v", successful.State, failed.State)
	}
}

func TestRetentionV1DifficultyChangesStability(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.retention")
	conceptID := mustID(t, "concept.memory")
	observedAt := retentionTimestamp(t, 0)
	easy := retentionEvidence(t, "easy", studentID, conceptID, EvidenceAssessment, .9, 0, observedAt)
	hard := retentionEvidence(t, "hard", studentID, conceptID, EvidenceAssessment, .9, 1, observedAt)
	easyCalculation, err := CalculateRetentionV1(retentionMastery(t, studentID, conceptID, []Evidence{easy}), []Evidence{easy}, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	hardCalculation, err := CalculateRetentionV1(retentionMastery(t, studentID, conceptID, []Evidence{hard}), []Evidence{hard}, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if easyCalculation.State.StabilityEstimate <= hardCalculation.State.StabilityEstimate ||
		easyCalculation.DifficultyFactor != 1.25 || hardCalculation.DifficultyFactor != .75 {
		t.Fatalf("difficulty stability easy=%+v hard=%+v", easyCalculation, hardCalculation)
	}
}

func TestRetentionV1ClockBoundaryAndCanonicalOrdering(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.retention")
	conceptID := mustID(t, "concept.memory")
	boundary := retentionTimestamp(t, 24*time.Hour)
	first := retentionEvidence(t, "first", studentID, conceptID, EvidenceAssessment, .8, .3, retentionTimestamp(t, 0))
	second := retentionEvidence(t, "second", studentID, conceptID, EvidenceReviewRecall, .9, .7, boundary)
	items := []Evidence{second, first}
	mastery := retentionMastery(t, studentID, conceptID, items)

	forward, err := CalculateRetentionV1(mastery, items, boundary)
	if err != nil {
		t.Fatal(err)
	}
	reverse, err := CalculateRetentionV1(mastery, []Evidence{first, second}, boundary)
	if err != nil || !reflect.DeepEqual(forward, reverse) {
		t.Fatalf("canonical calculations differ: forward=%+v reverse=%+v err=%v", forward, reverse, err)
	}
	beforeBoundary, _ := NewTimestamp(boundary.Time().Add(-time.Nanosecond))
	if _, err := CalculateRetentionV1(mastery, items, beforeBoundary); err == nil {
		t.Fatal("CalculateRetentionV1 accepted evidence after the injected clock")
	}
}

func TestRetentionV1MarksAndClearsReviewDueWithoutChangingMastery(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.retention")
	conceptID := mustID(t, "concept.memory")
	masteredAt := retentionTimestamp(t, 0)
	state := InstanceConceptState{
		CurriculumInstanceID: mustID(t, "instance.retention"), StudentID: studentID, ConceptID: conceptID,
		Exposure: ExposureMastered, Mastery: mustScore(t, .9), FirstSeenAt: &masteredAt,
		LastSeenAt: &masteredAt, MasteredAt: &masteredAt, UpdatedAt: masteredAt,
	}
	item := retentionEvidence(t, "projection", studentID, conceptID, EvidenceAssessment, .9, .5, masteredAt)
	mastery := retentionMastery(t, studentID, conceptID, []Evidence{item})
	initial, err := CalculateRetentionV1(mastery, []Evidence{item}, masteredAt)
	if err != nil {
		t.Fatal(err)
	}
	due, err := CalculateRetentionV1(mastery, []Evidence{item}, *initial.State.NextDueAt)
	if err != nil {
		t.Fatal(err)
	}
	marked, err := ApplyRetentionV1(state, due.State)
	if err != nil || !marked.StateChanged || marked.State.Exposure != ExposureReviewDue || marked.State.Mastery != state.Mastery {
		t.Fatalf("marked review due = (%+v, %v)", marked, err)
	}

	recallAt, _ := NewTimestamp(due.State.MeasuredAt.Time().Add(time.Hour))
	recall := retentionEvidence(t, "projection-recall", studentID, conceptID, EvidenceReviewRecall, 1, .5, recallAt)
	items := []Evidence{item, recall}
	refreshed, err := CalculateRetentionV1(retentionMastery(t, studentID, conceptID, items), items, recallAt)
	if err != nil {
		t.Fatal(err)
	}
	cleared, err := ApplyRetentionV1(marked.State, refreshed.State)
	if err != nil || !cleared.StateChanged || cleared.State.Exposure != ExposureMastered || cleared.State.Mastery != state.Mastery {
		t.Fatalf("cleared review due = (%+v, %v)", cleared, err)
	}
}

func retentionTimestamp(t *testing.T, offset time.Duration) Timestamp {
	t.Helper()
	value, err := NewTimestamp(time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC).Add(offset))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func retentionEvidence(t *testing.T, suffix string, studentID, conceptID ID, evidenceType EvidenceType, score, difficulty float64, observedAt Timestamp) Evidence {
	t.Helper()
	item, err := NewEvidenceWithMetadata(mustID(t, "evidence.retention."+suffix), studentID, conceptID, evidenceType,
		"fixture/retention/"+suffix, mustScore(t, score), EvidenceMetadata{Confidence: 1, Independence: 1, Difficulty: difficulty, AlgorithmVersion: "fixture/retention-v1"}, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func retentionMastery(t *testing.T, studentID, conceptID ID, items []Evidence) MasteryCalculation {
	t.Helper()
	mastery, err := CalculateMasteryV1(studentID, conceptID, items)
	if err != nil {
		t.Fatal(err)
	}
	return mastery
}
