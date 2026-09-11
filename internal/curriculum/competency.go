package curriculum

import (
	"fmt"
	"sort"
)

const CompetencyMatrixVersionV1 = "competency-matrix-v1"

type CompetencyDimension struct {
	ID            ID
	ExpectedLevel CompetencyLevel
}

func (dimension CompetencyDimension) Validate() error {
	if err := dimension.ID.Validate(); err != nil {
		return fmt.Errorf("competency dimension: %w", err)
	}
	return dimension.ExpectedLevel.Validate()
}

type Competency struct {
	ID            ID
	AreaID        ID
	Area          string
	OutcomeID     ID
	ExpectedLevel CompetencyLevel
	Dimensions    []CompetencyDimension
	EvidenceRefs  []EvidenceRef
	ConceptRefs   []ConceptID
	ParentID      *ID
}

func (competency Competency) Validate() error {
	if err := competency.ID.Validate(); err != nil {
		return fmt.Errorf("competency: %w", err)
	}
	if err := competency.AreaID.Validate(); err != nil {
		return fmt.Errorf("competency area id: %w", err)
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
	seenDimensions := make(map[ID]struct{}, len(competency.Dimensions))
	for _, dimension := range competency.Dimensions {
		if err := dimension.Validate(); err != nil {
			return err
		}
		if _, exists := seenDimensions[dimension.ID]; exists {
			return fmt.Errorf("competency %q contains duplicate dimension %q", competency.ID, dimension.ID)
		}
		seenDimensions[dimension.ID] = struct{}{}
	}
	if err := validateEvidenceRefs("competency evidence", competency.EvidenceRefs); err != nil {
		return err
	}
	if err := validateConceptIDs("competency concepts", competency.ConceptRefs); err != nil {
		return err
	}
	if competency.ParentID != nil {
		if err := competency.ParentID.Validate(); err != nil {
			return fmt.Errorf("competency parent: %w", err)
		}
		if *competency.ParentID == competency.ID {
			return fmt.Errorf("competency %q cannot be its own parent", competency.ID)
		}
	}
	return nil
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
	byID := make(map[ID]Competency, len(matrix.Competencies))
	for _, competency := range matrix.Competencies {
		if err := competency.Validate(); err != nil {
			return err
		}
		if _, exists := seen[competency.ID]; exists {
			return fmt.Errorf("competency matrix contains duplicate competency %q", competency.ID)
		}
		seen[competency.ID] = struct{}{}
		byID[competency.ID] = competency
	}
	for _, competency := range matrix.Competencies {
		if competency.ParentID == nil {
			continue
		}
		parent, exists := byID[*competency.ParentID]
		if !exists {
			return fmt.Errorf("competency %q references missing parent %q", competency.ID, competency.ParentID)
		}
		if parent.AreaID != competency.AreaID {
			return fmt.Errorf("competency %q parent belongs to another area", competency.ID)
		}
	}
	return validateCompetencyHierarchy(byID)
}

func validateCompetencyHierarchy(values map[ID]Competency) error {
	const (
		unvisited = iota
		visiting
		visited
	)
	state := make(map[ID]int, len(values))
	var visit func(ID) error
	visit = func(id ID) error {
		switch state[id] {
		case visiting:
			return fmt.Errorf("competency hierarchy contains a cycle at %q", id)
		case visited:
			return nil
		}
		state[id] = visiting
		if parent := values[id].ParentID; parent != nil {
			if err := visit(*parent); err != nil {
				return err
			}
		}
		state[id] = visited
		return nil
	}
	ids := make([]ID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	for _, id := range ids {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}
