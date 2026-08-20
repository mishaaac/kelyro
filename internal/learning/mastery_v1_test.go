package learning

import (
	"math"
	"reflect"
	"testing"
)

func TestMasteryV1DistinguishesUnknownFromObservedFailure(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.mastery")
	conceptID := mustID(t, "concept.variables")

	unknown, err := CalculateMasteryV1(studentID, conceptID, nil)
	if err != nil || unknown.Known || unknown.EvidenceCount != 0 || unknown.PolicyVersion != MasteryAlgorithmVersion {
		t.Fatalf("unknown mastery = (%+v, %v)", unknown, err)
	}
	failure := masteryEvidence(t, "failure", studentID, conceptID, EvidencePracticeFailure, 0, DefaultEvidenceMetadata(), 1)
	failed, err := CalculateMasteryV1(studentID, conceptID, []Evidence{failure})
	if err != nil || !failed.Known || failed.Score.Value() != 0 || failed.EvidenceCount != 1 {
		t.Fatalf("failed mastery = (%+v, %v)", failed, err)
	}
}

func TestMasteryV1OneEvidenceAndInclusiveThresholdBoundary(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.mastery")
	conceptID := mustID(t, "concept.variables")
	item := masteryEvidence(t, "one", studentID, conceptID, EvidenceAssessment, .8, DefaultEvidenceMetadata(), 1)

	calculation, err := CalculateMasteryV1(studentID, conceptID, []Evidence{item})
	if err != nil || calculation.Score.Value() != .8 || calculation.TotalWeight != 1 {
		t.Fatalf("one-evidence mastery = (%+v, %v)", calculation, err)
	}
	met, err := calculation.MeetsThreshold(mustThreshold(t, .8))
	if err != nil || !met {
		t.Fatalf("MeetsThreshold(boundary) = (%v, %v)", met, err)
	}
	met, err = calculation.MeetsThreshold(mustThreshold(t, .81))
	if err != nil || met {
		t.Fatalf("MeetsThreshold(above) = (%v, %v)", met, err)
	}
}

func TestMasteryV1WeightsConflictingEvidenceByStrengthAndMetadata(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.mastery")
	conceptID := mustID(t, "concept.variables")
	strong := masteryEvidence(t, "strong", studentID, conceptID, EvidenceAssessment, 1, DefaultEvidenceMetadata(), 1)
	weakMetadata := EvidenceMetadata{Confidence: .5, Independence: 0, Difficulty: 0, AlgorithmVersion: "fixture/v1"}
	weak := masteryEvidence(t, "weak", studentID, conceptID, EvidenceDiagnosticSelfReport, 0, weakMetadata, 2)

	calculation, err := CalculateMasteryV1(studentID, conceptID, []Evidence{weak, strong})
	if err != nil {
		t.Fatal(err)
	}
	wantWeakWeight := .4 * .5 * .25 * .75
	want := 1 / (1 + wantWeakWeight)
	if math.Abs(calculation.Score.Value()-want) > 1e-12 {
		t.Fatalf("mastery = %.12f, want %.12f", calculation.Score.Value(), want)
	}
	if calculation.Contributions[0].EvidenceID != strong.ID || calculation.Contributions[1].EvidenceID != weak.ID {
		t.Fatalf("contribution order = %+v", calculation.Contributions)
	}
	if math.Abs(calculation.Contributions[0].NormalizedWeight+calculation.Contributions[1].NormalizedWeight-1) > 1e-12 {
		t.Fatalf("normalized weights = %+v", calculation.Contributions)
	}
}

func TestMasteryV1ConfidenceIndependenceAndDifficultyAffectWeight(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.mastery")
	conceptID := mustID(t, "concept.variables")
	low := EvidenceMetadata{Confidence: .2, Independence: .2, Difficulty: .2, AlgorithmVersion: "fixture/v1"}
	high := EvidenceMetadata{Confidence: 1, Independence: 1, Difficulty: 1, AlgorithmVersion: "fixture/v1"}
	items := []Evidence{
		masteryEvidence(t, "low", studentID, conceptID, EvidenceKnowledgeCheck, 0, low, 1),
		masteryEvidence(t, "high", studentID, conceptID, EvidenceKnowledgeCheck, 1, high, 2),
	}
	calculation, err := CalculateMasteryV1(studentID, conceptID, items)
	if err != nil {
		t.Fatal(err)
	}
	if calculation.Score.Value() <= .8 || calculation.Contributions[0].EffectiveWeight >= calculation.Contributions[1].EffectiveWeight {
		t.Fatalf("metadata weighting = %+v", calculation)
	}
}

func TestMasteryV1CanonicalizesOrderingIncludingEqualTimestamps(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.mastery")
	conceptID := mustID(t, "concept.variables")
	items := []Evidence{
		masteryEvidence(t, "c", studentID, conceptID, EvidenceAssessment, .2, DefaultEvidenceMetadata(), 2),
		masteryEvidence(t, "b", studentID, conceptID, EvidencePracticeSuccess, .7, DefaultEvidenceMetadata(), 1),
		masteryEvidence(t, "a", studentID, conceptID, EvidenceKnowledgeCheck, .9, DefaultEvidenceMetadata(), 1),
	}
	forward, err := CalculateMasteryV1(studentID, conceptID, items)
	if err != nil {
		t.Fatal(err)
	}
	reverse, err := CalculateMasteryV1(studentID, conceptID, []Evidence{items[2], items[1], items[0]})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(forward, reverse) {
		t.Fatalf("non-deterministic calculations:\nforward=%+v\nreverse=%+v", forward, reverse)
	}
	wantOrder := []ID{items[2].ID, items[1].ID, items[0].ID}
	for index, want := range wantOrder {
		if forward.Contributions[index].EvidenceID != want {
			t.Fatalf("contribution %d = %s, want %s", index, forward.Contributions[index].EvidenceID, want)
		}
	}
}

func TestMasteryV1RejectsMalformedMismatchedAndDuplicateEvidence(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.mastery")
	conceptID := mustID(t, "concept.variables")
	valid := masteryEvidence(t, "valid", studentID, conceptID, EvidenceAssessment, .8, DefaultEvidenceMetadata(), 1)

	malformed := valid
	malformed.Confidence = math.NaN()
	wrongOwner := valid
	wrongOwner.ID = mustID(t, "evidence.wrong")
	wrongOwner.StudentID = mustID(t, "student.other")
	for name, items := range map[string][]Evidence{
		"malformed":   {malformed},
		"wrong owner": {wrongOwner},
		"duplicate":   {valid, valid},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := CalculateMasteryV1(studentID, conceptID, items); err == nil {
				t.Fatalf("CalculateMasteryV1(%s) accepted invalid evidence", name)
			}
		})
	}
}

func TestEvidenceModelAcceptsAllV1TypesAndRejectsMalformedMetadata(t *testing.T) {
	t.Parallel()
	studentID := mustID(t, "student.mastery")
	conceptID := mustID(t, "concept.variables")
	types := []EvidenceType{
		EvidenceDiagnosticObjective, EvidenceDiagnosticSelfReport, EvidenceKnowledgeCheck,
		EvidencePracticeSuccess, EvidencePracticeFailure, EvidenceAssessment,
		EvidenceProject, EvidenceReviewRecall, EvidenceManualImport,
	}
	for _, evidenceType := range types {
		if _, err := NewEvidenceWithMetadata(mustID(t, "evidence."+string(evidenceType)), studentID, conceptID, evidenceType,
			"fixture/source", mustScore(t, .5), DefaultEvidenceMetadata(), mustTimestamp(t, 1)); err != nil {
			t.Errorf("NewEvidenceWithMetadata(%s) error = %v", evidenceType, err)
		}
	}
	for name, metadata := range map[string]EvidenceMetadata{
		"zero confidence":     {Confidence: 0, Independence: 1, Difficulty: .5, AlgorithmVersion: "fixture/v1"},
		"high independence":   {Confidence: 1, Independence: 1.1, Difficulty: .5, AlgorithmVersion: "fixture/v1"},
		"negative difficulty": {Confidence: 1, Independence: 1, Difficulty: -.1, AlgorithmVersion: "fixture/v1"},
		"empty algorithm":     {Confidence: 1, Independence: 1, Difficulty: .5},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewEvidenceWithMetadata(mustID(t, "evidence.invalid"), studentID, conceptID, EvidenceAssessment,
				"fixture/source", mustScore(t, .5), metadata, mustTimestamp(t, 1)); err == nil {
				t.Fatalf("NewEvidenceWithMetadata(%s) accepted malformed metadata", name)
			}
		})
	}
}

func masteryEvidence(t *testing.T, suffix string, studentID, conceptID ID, evidenceType EvidenceType, score float64, metadata EvidenceMetadata, hour int) Evidence {
	t.Helper()
	item, err := NewEvidenceWithMetadata(mustID(t, "evidence."+suffix), studentID, conceptID, evidenceType,
		"fixture/"+suffix, mustScore(t, score), metadata, mustTimestamp(t, hour))
	if err != nil {
		t.Fatalf("NewEvidenceWithMetadata() error = %v", err)
	}
	return item
}
