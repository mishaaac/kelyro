package curriculum

import "fmt"

const AtomicConceptCriteriaVersionV1 = "atomic-concept-criteria-v1"

type AtomicityCriterionState string

const (
	AtomicityCriterionSatisfied   AtomicityCriterionState = "satisfied"
	AtomicityCriterionUnsatisfied AtomicityCriterionState = "unsatisfied"
	AtomicityCriterionUnknown     AtomicityCriterionState = "unknown"
)

func (state AtomicityCriterionState) Validate() error {
	switch state {
	case AtomicityCriterionSatisfied, AtomicityCriterionUnsatisfied, AtomicityCriterionUnknown:
		return nil
	default:
		return fmt.Errorf("invalid atomicity criterion state %q", state)
	}
}

// AtomicConceptCriteria records explicit, domain-supplied signals. The policy
// never guesses atomicity from a title or from domain-specific vocabulary.
type AtomicConceptCriteria struct {
	Named                        AtomicityCriterionState
	Defined                      AtomicityCriterionState
	PrerequisiteBoundary         AtomicityCriterionState
	Explainable                  AtomicityCriterionState
	Practicable                  AtomicityCriterionState
	Assessable                   AtomicityCriterionState
	Evidenced                    AtomicityCriterionState
	MeaningfulStandalone         AtomicityCriterionState
	IndependentlyAssessableParts []string
}

func (criteria AtomicConceptCriteria) Validate() error {
	states := []struct {
		name  string
		value AtomicityCriterionState
	}{
		{name: "named", value: criteria.Named},
		{name: "defined", value: criteria.Defined},
		{name: "prerequisite_boundary", value: criteria.PrerequisiteBoundary},
		{name: "explainable", value: criteria.Explainable},
		{name: "practicable", value: criteria.Practicable},
		{name: "assessable", value: criteria.Assessable},
		{name: "evidenced", value: criteria.Evidenced},
		{name: "meaningful_standalone", value: criteria.MeaningfulStandalone},
	}
	for _, state := range states {
		if err := state.value.Validate(); err != nil {
			return fmt.Errorf("atomicity criterion %s: %w", state.name, err)
		}
	}
	return validateUniqueTexts("independently assessable parts", criteria.IndependentlyAssessableParts)
}

type AtomicityAssessment struct {
	Result        Atomicity
	Reasons       []string
	PolicyVersion string
}

func (assessment AtomicityAssessment) Validate() error {
	if err := assessment.Result.Validate(); err != nil {
		return err
	}
	if err := validateUniqueTexts("atomicity reasons", assessment.Reasons); err != nil {
		return err
	}
	if len(assessment.Reasons) == 0 {
		return fmt.Errorf("atomicity assessment has no reasons")
	}
	if assessment.PolicyVersion != AtomicConceptCriteriaVersionV1 {
		return fmt.Errorf("unsupported atomicity policy version %q", assessment.PolicyVersion)
	}
	return nil
}

type AtomicConceptPolicyV1 struct{}

func NewAtomicConceptPolicyV1() AtomicConceptPolicyV1 {
	return AtomicConceptPolicyV1{}
}

func (AtomicConceptPolicyV1) Version() string {
	return AtomicConceptCriteriaVersionV1
}

func (AtomicConceptPolicyV1) Assess(criteria AtomicConceptCriteria) (AtomicityAssessment, error) {
	if err := criteria.Validate(); err != nil {
		return AtomicityAssessment{}, err
	}
	assessment := AtomicityAssessment{PolicyVersion: AtomicConceptCriteriaVersionV1}
	if len(criteria.IndependentlyAssessableParts) > 1 {
		assessment.Result = AtomicityNeedsSplit
		assessment.Reasons = []string{"multiple_independently_assessable_parts"}
		return assessment, nil
	}

	fragmented := unsatisfiedCriteria([]namedAtomicityState{
		{name: "meaningful_standalone", value: criteria.MeaningfulStandalone},
		{name: "explainable", value: criteria.Explainable},
		{name: "practicable", value: criteria.Practicable},
		{name: "assessable", value: criteria.Assessable},
	})
	if len(fragmented) > 0 {
		assessment.Result = AtomicityTooFragmented
		assessment.Reasons = prefixReasons("criterion_unsatisfied:", fragmented)
		return assessment, nil
	}

	all := []namedAtomicityState{
		{name: "named", value: criteria.Named},
		{name: "defined", value: criteria.Defined},
		{name: "prerequisite_boundary", value: criteria.PrerequisiteBoundary},
		{name: "explainable", value: criteria.Explainable},
		{name: "practicable", value: criteria.Practicable},
		{name: "assessable", value: criteria.Assessable},
		{name: "evidenced", value: criteria.Evidenced},
		{name: "meaningful_standalone", value: criteria.MeaningfulStandalone},
	}
	incomplete := make([]string, 0)
	for _, criterion := range all {
		if criterion.value != AtomicityCriterionSatisfied {
			incomplete = append(incomplete, criterion.name)
		}
	}
	if len(incomplete) > 0 {
		assessment.Result = AtomicityUnknown
		assessment.Reasons = prefixReasons("criterion_unresolved:", incomplete)
		return assessment, nil
	}

	assessment.Result = AtomicityAtomic
	assessment.Reasons = []string{"all_atomic_concept_criteria_satisfied"}
	return assessment, nil
}

type namedAtomicityState struct {
	name  string
	value AtomicityCriterionState
}

func unsatisfiedCriteria(values []namedAtomicityState) []string {
	result := make([]string, 0)
	for _, criterion := range values {
		if criterion.value == AtomicityCriterionUnsatisfied {
			result = append(result, criterion.name)
		}
	}
	return result
}

func prefixReasons(prefix string, values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = prefix + value
	}
	return result
}
