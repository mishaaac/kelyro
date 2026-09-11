package curriculum

import (
	"strings"
	"testing"
)

func TestProfessionalLearningGoalRequiresCompleteCapabilities(t *testing.T) {
	t.Parallel()
	goal := professionalGoalFixture(t)
	if err := goal.Validate(); err != nil {
		t.Fatalf("complete professional goal rejected: %v", err)
	}

	for index, capability := range []OutcomeCapability{
		OutcomeCapabilityExplain, OutcomeCapabilityBuild, OutcomeCapabilityDebug,
		OutcomeCapabilityOperate, OutcomeCapabilityMaintain,
	} {
		index, capability := index, capability
		t.Run(string(capability), func(t *testing.T) {
			t.Parallel()
			incomplete := professionalGoalFixture(t)
			incomplete.Outcomes = append(incomplete.Outcomes[:index:index], incomplete.Outcomes[index+1:]...)
			err := incomplete.Validate()
			if err == nil || !strings.Contains(err.Error(), `missing "`+string(capability)+`" outcome capability`) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestOutcomeCategoriesAndCapabilitiesAreClosed(t *testing.T) {
	t.Parallel()
	for _, category := range []OutcomeCategory{
		OutcomeKnowledge, OutcomeApplication, OutcomeDebugging, OutcomeDesign,
		OutcomeToolUsage, OutcomeProduction, OutcomeSecurity,
		OutcomeCommunicationDocumentation, OutcomeMaintenance,
	} {
		if err := category.Validate(); err != nil {
			t.Errorf("category %q rejected: %v", category, err)
		}
	}
	if err := OutcomeCategory("syntax_only").Validate(); err == nil {
		t.Fatal("unknown outcome category accepted")
	}
	if err := OutcomeCapability("deploy").Validate(); err == nil {
		t.Fatal("unknown outcome capability accepted")
	}

	narrow := LearningGoalSpec{
		ID: mustID(t, "goal.narrow"), Title: "Narrow goal", Description: "Understand one topic.", Domain: "general",
		Outcomes: []GoalOutcome{{ID: mustID(t, "outcome.know"), Statement: "Explain the topic.", Category: OutcomeKnowledge}},
	}
	if err := narrow.Validate(); err != nil {
		t.Fatalf("non-professional goal without capability rejected: %v", err)
	}
}

func professionalGoalFixture(t *testing.T) LearningGoalSpec {
	t.Helper()
	return LearningGoalSpec{
		ID: mustID(t, "goal.professional"), Title: "Professional goal", Description: "Prepare for professional work.", Domain: "general",
		Role: &ProfessionalRole{ID: mustID(t, "role.professional"), Name: "Professional", Description: "Performs the target work."},
		Outcomes: []GoalOutcome{
			{ID: mustID(t, "outcome.explain"), Statement: "Explain the system.", Category: OutcomeKnowledge, Capability: OutcomeCapabilityExplain},
			{ID: mustID(t, "outcome.build"), Statement: "Build the system.", Category: OutcomeApplication, Capability: OutcomeCapabilityBuild},
			{ID: mustID(t, "outcome.debug"), Statement: "Debug the system.", Category: OutcomeDebugging, Capability: OutcomeCapabilityDebug},
			{ID: mustID(t, "outcome.operate"), Statement: "Operate the system.", Category: OutcomeProduction, Capability: OutcomeCapabilityOperate},
			{ID: mustID(t, "outcome.maintain"), Statement: "Maintain the system.", Category: OutcomeMaintenance, Capability: OutcomeCapabilityMaintain},
		},
	}
}
