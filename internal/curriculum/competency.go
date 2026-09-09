package curriculum

import "fmt"

type Competency struct {
	ID            ID
	Area          string
	OutcomeID     ID
	ExpectedLevel CompetencyLevel
	EvidenceRefs  []EvidenceRef
	ConceptRefs   []ConceptID
}

func (competency Competency) Validate() error {
	if err := competency.ID.Validate(); err != nil {
		return fmt.Errorf("competency: %w", err)
	}
	if err := requireText("competency area", competency.Area); err != nil {
		return err
	}
	if err := competency.OutcomeID.Validate(); err != nil {
		return fmt.Errorf("competency outcome: %w", err)
	}
	if err := competency.ExpectedLevel.Validate(); err != nil {
		return err
	}
	if err := validateEvidenceRefs("competency evidence", competency.EvidenceRefs); err != nil {
		return err
	}
	return validateConceptIDs("competency concepts", competency.ConceptRefs)
}

type CompetencyMatrix struct {
	Version      string
	GoalID       ID
	Competencies []Competency
}

func (matrix CompetencyMatrix) Validate() error {
	if err := requireText("competency matrix version", matrix.Version); err != nil {
		return err
	}
	if err := matrix.GoalID.Validate(); err != nil {
		return fmt.Errorf("competency matrix goal: %w", err)
	}
	if len(matrix.Competencies) == 0 {
		return fmt.Errorf("competency matrix is empty")
	}
	seen := make(map[ID]struct{}, len(matrix.Competencies))
	for _, competency := range matrix.Competencies {
		if err := competency.Validate(); err != nil {
			return err
		}
		if _, exists := seen[competency.ID]; exists {
			return fmt.Errorf("competency matrix contains duplicate competency %q", competency.ID)
		}
		seen[competency.ID] = struct{}{}
	}
	return nil
}
