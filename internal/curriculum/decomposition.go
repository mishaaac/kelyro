package curriculum

import "fmt"

const GoalDecomposerVersionV1 = "goal-decomposer-v1"

// CompetencyAreaSpec is a pack-authored area candidate. The decomposer selects
// it by declared scope/role applicability; core never invents domain areas.
type CompetencyAreaSpec struct {
	ID                      ID
	Name                    string
	Description             string
	Scopes                  []string
	OutcomeIDs              []ID
	RequiredForProfessional bool
	EvidenceRefs            []EvidenceRef
}

func (area CompetencyAreaSpec) Validate() error {
	if err := area.ID.Validate(); err != nil {
		return fmt.Errorf("competency area: %w", err)
	}
	if err := requireText("competency area name", area.Name); err != nil {
		return err
	}
	if err := requireText("competency area description", area.Description); err != nil {
		return err
	}
	if err := validateTexts("competency area scopes", area.Scopes); err != nil {
		return err
	}
	if err := validateIDs("competency area outcomes", area.OutcomeIDs); err != nil {
		return err
	}
	if len(area.OutcomeIDs) == 0 {
		return fmt.Errorf("competency area %q has no outcomes", area.ID)
	}
	if err := validateEvidenceRefs("competency area evidence", area.EvidenceRefs); err != nil {
		return err
	}
	if len(area.EvidenceRefs) == 0 {
		return fmt.Errorf("competency area %q has no evidence", area.ID)
	}
	return nil
}

type DomainProfile struct {
	ID              ID
	Version         string
	Domain          string
	SupportedScopes []string
	Areas           []CompetencyAreaSpec
}

func (profile DomainProfile) Validate() error {
	if err := profile.ID.Validate(); err != nil {
		return fmt.Errorf("domain profile: %w", err)
	}
	if err := requireText("domain profile version", profile.Version); err != nil {
		return err
	}
	if err := requireText("domain profile domain", profile.Domain); err != nil {
		return err
	}
	if err := validateUniqueTexts("domain profile supported scopes", profile.SupportedScopes); err != nil {
		return err
	}
	if len(profile.SupportedScopes) == 0 {
		return fmt.Errorf("domain profile supported scopes are empty")
	}
	if len(profile.Areas) == 0 {
		return fmt.Errorf("domain profile competency areas are empty")
	}
	seen := make(map[ID]struct{}, len(profile.Areas))
	for _, area := range profile.Areas {
		if err := area.Validate(); err != nil {
			return err
		}
		if _, exists := seen[area.ID]; exists {
			return fmt.Errorf("domain profile contains duplicate area %q", area.ID)
		}
		seen[area.ID] = struct{}{}
		for _, scope := range area.Scopes {
			if !containsText(profile.SupportedScopes, scope) {
				return fmt.Errorf("competency area %q uses unsupported profile scope %q", area.ID, scope)
			}
		}
	}
	return nil
}

type CompetencyArea struct {
	ID           ID
	Name         string
	Description  string
	OutcomeIDs   []ID
	EvidenceRefs []EvidenceRef
}

func (area CompetencyArea) Validate() error {
	return CompetencyAreaSpec{ID: area.ID, Name: area.Name, Description: area.Description, OutcomeIDs: area.OutcomeIDs, EvidenceRefs: area.EvidenceRefs}.Validate()
}

type GoalDecomposition struct {
	GoalID           ID
	Outcomes         []GoalOutcome
	CompetencyAreas  []CompetencyArea
	Scope            []string
	Exclusions       []string
	ProfileID        ID
	ProfileVersion   string
	AlgorithmVersion string
}

func (decomposition GoalDecomposition) Validate() error {
	if err := decomposition.GoalID.Validate(); err != nil {
		return fmt.Errorf("goal decomposition: %w", err)
	}
	if len(decomposition.Outcomes) == 0 {
		return fmt.Errorf("goal decomposition outcomes are empty")
	}
	outcomes := make(map[ID]struct{}, len(decomposition.Outcomes))
	for _, outcome := range decomposition.Outcomes {
		if err := outcome.Validate(); err != nil {
			return err
		}
		if _, exists := outcomes[outcome.ID]; exists {
			return fmt.Errorf("goal decomposition contains duplicate outcome %q", outcome.ID)
		}
		outcomes[outcome.ID] = struct{}{}
	}
	if len(decomposition.CompetencyAreas) == 0 {
		return fmt.Errorf("goal decomposition competency areas are empty")
	}
	covered := make(map[ID]struct{}, len(outcomes))
	areas := make(map[ID]struct{}, len(decomposition.CompetencyAreas))
	for _, area := range decomposition.CompetencyAreas {
		if err := area.Validate(); err != nil {
			return err
		}
		if _, exists := areas[area.ID]; exists {
			return fmt.Errorf("goal decomposition contains duplicate area %q", area.ID)
		}
		areas[area.ID] = struct{}{}
		for _, outcomeID := range area.OutcomeIDs {
			if _, exists := outcomes[outcomeID]; !exists {
				return fmt.Errorf("competency area %q references unknown outcome %q", area.ID, outcomeID)
			}
			covered[outcomeID] = struct{}{}
		}
	}
	for outcomeID := range outcomes {
		if _, exists := covered[outcomeID]; !exists {
			return fmt.Errorf("goal outcome %q is not covered by a competency area", outcomeID)
		}
	}
	if err := validateUniqueTexts("goal decomposition scope", decomposition.Scope); err != nil {
		return err
	}
	if err := validateUniqueTexts("goal decomposition exclusions", decomposition.Exclusions); err != nil {
		return err
	}
	if err := decomposition.ProfileID.Validate(); err != nil {
		return fmt.Errorf("goal decomposition profile: %w", err)
	}
	if err := requireText("goal decomposition profile version", decomposition.ProfileVersion); err != nil {
		return err
	}
	if decomposition.AlgorithmVersion != GoalDecomposerVersionV1 {
		return fmt.Errorf("unsupported goal decomposer version %q", decomposition.AlgorithmVersion)
	}
	return nil
}

func validateUniqueTexts(name string, values []string) error {
	if err := validateTexts(name, values); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%s contains duplicate value %q", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func containsText(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
