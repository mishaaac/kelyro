package curriculum

import (
	"reflect"
	"testing"
)

func TestAtomicConceptPolicyV1ReturnsAllClosedResults(t *testing.T) {
	t.Parallel()
	complete := completeAtomicityCriteria()
	tests := []struct {
		name   string
		edit   func(*AtomicConceptCriteria)
		result Atomicity
		reason string
	}{
		{name: "atomic", edit: func(*AtomicConceptCriteria) {}, result: AtomicityAtomic, reason: "all_atomic_concept_criteria_satisfied"},
		{name: "needs split", edit: func(value *AtomicConceptCriteria) {
			value.IndependentlyAssessableParts = []string{"declaration", "assignment"}
		}, result: AtomicityNeedsSplit, reason: "multiple_independently_assessable_parts"},
		{name: "too fragmented", edit: func(value *AtomicConceptCriteria) {
			value.MeaningfulStandalone = AtomicityCriterionUnsatisfied
		}, result: AtomicityTooFragmented, reason: "criterion_unsatisfied:meaningful_standalone"},
		{name: "unknown", edit: func(value *AtomicConceptCriteria) {
			value.PrerequisiteBoundary = AtomicityCriterionUnknown
		}, result: AtomicityUnknown, reason: "criterion_unresolved:prerequisite_boundary"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			criteria := complete
			test.edit(&criteria)
			got, err := NewAtomicConceptPolicyV1().Assess(criteria)
			if err != nil {
				t.Fatalf("Assess() error = %v", err)
			}
			if got.Result != test.result || !reflect.DeepEqual(got.Reasons, []string{test.reason}) || got.PolicyVersion != AtomicConceptCriteriaVersionV1 {
				t.Fatalf("assessment = %+v", got)
			}
		})
	}
}

func TestAtomicConceptPolicyV1DoesNotHardcodeBroadTitles(t *testing.T) {
	t.Parallel()
	for _, title := range []string{"Go Programming", "Backend Development", "Databases"} {
		criteria := completeAtomicityCriteria()
		criteria.IndependentlyAssessableParts = []string{title + " foundation", title + " application"}
		assessment, err := NewAtomicConceptPolicyV1().Assess(criteria)
		if err != nil || assessment.Result != AtomicityNeedsSplit {
			t.Fatalf("%q assessment = %+v, error = %v", title, assessment, err)
		}
	}
}

func TestAtomicConceptCriteriaRejectsInvalidOrDuplicateSignals(t *testing.T) {
	t.Parallel()
	criteria := completeAtomicityCriteria()
	criteria.Named = "maybe"
	if _, err := NewAtomicConceptPolicyV1().Assess(criteria); err == nil {
		t.Fatal("invalid criterion state accepted")
	}
	criteria = completeAtomicityCriteria()
	criteria.IndependentlyAssessableParts = []string{"declaration", "declaration"}
	if _, err := NewAtomicConceptPolicyV1().Assess(criteria); err == nil {
		t.Fatal("duplicate independently assessable part accepted")
	}
}

func completeAtomicityCriteria() AtomicConceptCriteria {
	return AtomicConceptCriteria{
		Named: AtomicityCriterionSatisfied, Defined: AtomicityCriterionSatisfied,
		PrerequisiteBoundary: AtomicityCriterionSatisfied, Explainable: AtomicityCriterionSatisfied,
		Practicable: AtomicityCriterionSatisfied, Assessable: AtomicityCriterionSatisfied,
		Evidenced: AtomicityCriterionSatisfied, MeaningfulStandalone: AtomicityCriterionSatisfied,
	}
}
