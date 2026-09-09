package curriculum

import "fmt"

type ProfessionalRole struct {
	ID          ID
	Name        string
	Description string
}

func (role ProfessionalRole) Validate() error {
	if err := role.ID.Validate(); err != nil {
		return fmt.Errorf("professional role: %w", err)
	}
	if err := requireText("professional role name", role.Name); err != nil {
		return err
	}
	return requireText("professional role description", role.Description)
}

type GoalOutcome struct {
	ID           ID
	Statement    string
	EvidenceRefs []EvidenceRef
}

func (outcome GoalOutcome) Validate() error {
	if err := outcome.ID.Validate(); err != nil {
		return fmt.Errorf("goal outcome: %w", err)
	}
	if err := requireText("goal outcome statement", outcome.Statement); err != nil {
		return err
	}
	return validateEvidenceRefs("goal outcome evidence", outcome.EvidenceRefs)
}

// LearningGoalSpec is learner-neutral compiler input. It is distinct from the
// learner-owned LearningGoal lifecycle in I-02.
type LearningGoalSpec struct {
	ID          ID
	Title       string
	Description string
	Domain      string
	Role        *ProfessionalRole
	Outcomes    []GoalOutcome
	Scope       []string
	Exclusions  []string
}

func (goal LearningGoalSpec) Validate() error {
	if err := goal.ID.Validate(); err != nil {
		return fmt.Errorf("learning goal spec: %w", err)
	}
	for _, field := range []struct{ name, value string }{
		{name: "learning goal title", value: goal.Title},
		{name: "learning goal description", value: goal.Description},
		{name: "learning goal domain", value: goal.Domain},
	} {
		if err := requireText(field.name, field.value); err != nil {
			return err
		}
	}
	if goal.Role != nil {
		if err := goal.Role.Validate(); err != nil {
			return err
		}
	}
	if len(goal.Outcomes) == 0 {
		return fmt.Errorf("learning goal outcomes are empty")
	}
	seen := make(map[ID]struct{}, len(goal.Outcomes))
	for _, outcome := range goal.Outcomes {
		if err := outcome.Validate(); err != nil {
			return err
		}
		if _, exists := seen[outcome.ID]; exists {
			return fmt.Errorf("learning goal contains duplicate outcome %q", outcome.ID)
		}
		seen[outcome.ID] = struct{}{}
	}
	if err := validateTexts("learning goal scope", goal.Scope); err != nil {
		return err
	}
	return validateTexts("learning goal exclusions", goal.Exclusions)
}
