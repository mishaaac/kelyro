package curriculum

import "fmt"

// CurriculumDefinition is the immutable, learner-neutral output language of
// I-04. The hierarchy is a UX structure; Prerequisites remain the pedagogical
// graph. Student progress belongs exclusively to I-02 curriculum instances.
type CurriculumDefinition struct {
	ID                   CurriculumID
	Version              CurriculumVersion
	Title                string
	Description          string
	Goal                 LearningGoalSpec
	Competencies         CompetencyMatrix
	Concepts             []Concept
	Prerequisites        []Prerequisite
	Vocabulary           VocabularyGraph
	Phases               []Phase
	Modules              []Module
	Lessons              []LessonSpec
	Topics               []TopicSpec
	CoverageRequirements []CoverageRequirement
	SourcePolicy         SourceReferencePolicy
	SourceBundles        []SourceBundleRef
	CreatedAt            Timestamp
}

func (definition CurriculumDefinition) Validate() error {
	if err := definition.ID.Validate(); err != nil {
		return fmt.Errorf("curriculum definition: %w", err)
	}
	if err := definition.Version.Validate(); err != nil {
		return err
	}
	if err := requireText("curriculum title", definition.Title); err != nil {
		return err
	}
	if err := requireText("curriculum description", definition.Description); err != nil {
		return err
	}
	if err := definition.Goal.Validate(); err != nil {
		return err
	}
	if err := definition.Competencies.Validate(); err != nil {
		return err
	}
	if definition.Competencies.GoalID != definition.Goal.ID {
		return fmt.Errorf("competency matrix goal does not match curriculum goal")
	}
	if err := definition.SourcePolicy.Validate(); err != nil {
		return err
	}
	if err := validateSourceBundleRefs(definition.SourceBundles, definition.SourcePolicy); err != nil {
		return err
	}
	if err := definition.CreatedAt.Validate(); err != nil {
		return fmt.Errorf("curriculum created at: %w", err)
	}

	outcomes := make(map[ID]struct{}, len(definition.Goal.Outcomes))
	for _, outcome := range definition.Goal.Outcomes {
		outcomes[outcome.ID] = struct{}{}
	}
	competencies := make(map[ID]struct{}, len(definition.Competencies.Competencies))
	for _, competency := range definition.Competencies.Competencies {
		if _, exists := outcomes[competency.OutcomeID]; !exists {
			return fmt.Errorf("competency %q references missing outcome %q", competency.ID, competency.OutcomeID)
		}
		competencies[competency.ID] = struct{}{}
	}

	concepts, err := validateConcepts(definition.Concepts)
	if err != nil {
		return err
	}
	for _, competency := range definition.Competencies.Competencies {
		for _, conceptID := range competency.ConceptRefs {
			if _, exists := concepts[conceptID]; !exists {
				return fmt.Errorf("competency %q references missing concept %q", competency.ID, conceptID)
			}
		}
	}
	if err := validatePrerequisites(definition.Prerequisites, concepts); err != nil {
		return err
	}
	if err := validateVocabulary(definition.Vocabulary, concepts); err != nil {
		return err
	}
	if err := validateHierarchy(definition, concepts); err != nil {
		return err
	}
	if err := validateCoverageRequirements(definition, competencies, concepts); err != nil {
		return err
	}
	return validateDeclaredEvidence(definition)
}

func validateConcepts(values []Concept) (map[ConceptID]struct{}, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("curriculum concepts are empty")
	}
	seen := make(map[ConceptID]struct{}, len(values))
	for _, concept := range values {
		if err := concept.Validate(); err != nil {
			return nil, err
		}
		if _, exists := seen[concept.ID]; exists {
			return nil, fmt.Errorf("duplicate concept id %q", concept.ID)
		}
		seen[concept.ID] = struct{}{}
	}
	return seen, nil
}

func validatePrerequisites(values []Prerequisite, concepts map[ConceptID]struct{}) error {
	type edge struct {
		concept  ConceptID
		required ConceptID
		kind     PrerequisiteKind
	}
	seen := make(map[edge]struct{}, len(values))
	for _, prerequisite := range values {
		if err := prerequisite.Validate(); err != nil {
			return err
		}
		if _, exists := concepts[prerequisite.ConceptID]; !exists {
			return fmt.Errorf("prerequisite edge references missing concept %q", prerequisite.ConceptID)
		}
		if _, exists := concepts[prerequisite.RequiredConceptID]; !exists {
			return fmt.Errorf("prerequisite edge references missing required concept %q", prerequisite.RequiredConceptID)
		}
		key := edge{concept: prerequisite.ConceptID, required: prerequisite.RequiredConceptID, kind: prerequisite.Kind}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate prerequisite edge from %q to %q", prerequisite.ConceptID, prerequisite.RequiredConceptID)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateVocabulary(graph VocabularyGraph, concepts map[ConceptID]struct{}) error {
	if err := graph.Validate(); err != nil {
		return err
	}
	for _, term := range graph.Terms {
		for _, reference := range append([]ConceptID{term.CanonicalConceptID, term.IntroducedBy}, term.UsedBy...) {
			if _, exists := concepts[reference]; !exists {
				return fmt.Errorf("vocabulary term %q references missing concept %q", term.Term, reference)
			}
		}
	}
	return nil
}

func validateHierarchy(definition CurriculumDefinition, concepts map[ConceptID]struct{}) error {
	if len(definition.Phases) == 0 || len(definition.Modules) == 0 || len(definition.Lessons) == 0 || len(definition.Topics) == 0 {
		return fmt.Errorf("curriculum hierarchy requires phase, module, lesson, and topic nodes")
	}
	allNodeIDs := make(map[string]string, len(definition.Phases)+len(definition.Modules)+len(definition.Lessons)+len(definition.Topics)+len(concepts))
	register := func(kind, value string) error {
		if previous, exists := allNodeIDs[value]; exists {
			return fmt.Errorf("duplicate curriculum node id %q used by %s and %s", value, previous, kind)
		}
		allNodeIDs[value] = kind
		return nil
	}
	for conceptID := range concepts {
		if err := register("concept", conceptID.String()); err != nil {
			return err
		}
	}

	phases := make(map[ID]struct{}, len(definition.Phases))
	for _, phase := range definition.Phases {
		if err := phase.Validate(); err != nil {
			return err
		}
		if err := register("phase", phase.ID.String()); err != nil {
			return err
		}
		phases[phase.ID] = struct{}{}
	}
	modules := make(map[ID]struct{}, len(definition.Modules))
	moduleOrder := make(map[struct {
		parent ID
		order  int
	}]ID, len(definition.Modules))
	for _, module := range definition.Modules {
		if err := module.Validate(); err != nil {
			return err
		}
		if _, exists := phases[module.PhaseID]; !exists {
			return fmt.Errorf("module %q references missing phase %q", module.ID, module.PhaseID)
		}
		if err := register("module", module.ID.String()); err != nil {
			return err
		}
		key := struct {
			parent ID
			order  int
		}{module.PhaseID, module.Order}
		if previous, exists := moduleOrder[key]; exists {
			return fmt.Errorf("modules %q and %q share order %d", previous, module.ID, module.Order)
		}
		moduleOrder[key] = module.ID
		modules[module.ID] = struct{}{}
	}
	lessons := make(map[ID]struct{}, len(definition.Lessons))
	lessonOrder := make(map[struct {
		parent ID
		order  int
	}]ID, len(definition.Lessons))
	for _, lesson := range definition.Lessons {
		if err := lesson.Validate(); err != nil {
			return err
		}
		if _, exists := modules[lesson.ModuleID]; !exists {
			return fmt.Errorf("lesson %q references missing module %q", lesson.ID, lesson.ModuleID)
		}
		if err := register("lesson", lesson.ID.String()); err != nil {
			return err
		}
		key := struct {
			parent ID
			order  int
		}{lesson.ModuleID, lesson.Order}
		if previous, exists := lessonOrder[key]; exists {
			return fmt.Errorf("lessons %q and %q share order %d", previous, lesson.ID, lesson.Order)
		}
		lessonOrder[key] = lesson.ID
		lessons[lesson.ID] = struct{}{}
	}
	topicOrder := make(map[struct {
		parent ID
		order  int
	}]ID, len(definition.Topics))
	assignedConcepts := make(map[ConceptID]ID, len(concepts))
	for _, topic := range definition.Topics {
		if err := topic.Validate(); err != nil {
			return err
		}
		if _, exists := lessons[topic.LessonID]; !exists {
			return fmt.Errorf("topic %q references missing lesson %q", topic.ID, topic.LessonID)
		}
		if err := register("topic", topic.ID.String()); err != nil {
			return err
		}
		key := struct {
			parent ID
			order  int
		}{topic.LessonID, topic.Order}
		if previous, exists := topicOrder[key]; exists {
			return fmt.Errorf("topics %q and %q share order %d", previous, topic.ID, topic.Order)
		}
		topicOrder[key] = topic.ID
		for _, conceptID := range topic.ConceptIDs {
			if _, exists := concepts[conceptID]; !exists {
				return fmt.Errorf("topic %q references missing concept %q", topic.ID, conceptID)
			}
			if previous, exists := assignedConcepts[conceptID]; exists {
				return fmt.Errorf("concept %q is assigned to topics %q and %q", conceptID, previous, topic.ID)
			}
			assignedConcepts[conceptID] = topic.ID
		}
	}
	for conceptID := range concepts {
		if _, exists := assignedConcepts[conceptID]; !exists {
			return fmt.Errorf("concept %q is missing from curriculum hierarchy", conceptID)
		}
	}
	return validateRootOrders(definition.Phases)
}

func validateRootOrders(phases []Phase) error {
	seen := make(map[int]ID, len(phases))
	for _, phase := range phases {
		if previous, exists := seen[phase.Order]; exists {
			return fmt.Errorf("phases %q and %q share order %d", previous, phase.ID, phase.Order)
		}
		seen[phase.Order] = phase.ID
	}
	return nil
}

func validateCoverageRequirements(definition CurriculumDefinition, competencies map[ID]struct{}, concepts map[ConceptID]struct{}) error {
	seen := make(map[ID]struct{}, len(definition.CoverageRequirements))
	for _, requirement := range definition.CoverageRequirements {
		if err := requirement.Validate(); err != nil {
			return err
		}
		if _, exists := seen[requirement.ID]; exists {
			return fmt.Errorf("duplicate coverage requirement %q", requirement.ID)
		}
		seen[requirement.ID] = struct{}{}
		switch requirement.TargetKind {
		case CoverageTargetCurriculum:
			if requirement.TargetID.String() != definition.ID.String() {
				return fmt.Errorf("coverage requirement %q references another curriculum", requirement.ID)
			}
		case CoverageTargetGoal:
			if requirement.TargetID != definition.Goal.ID {
				return fmt.Errorf("coverage requirement %q references missing goal", requirement.ID)
			}
		case CoverageTargetCompetency:
			if _, exists := competencies[requirement.TargetID]; !exists {
				return fmt.Errorf("coverage requirement %q references missing competency", requirement.ID)
			}
		case CoverageTargetConcept:
			conceptID, err := NewConceptID(requirement.TargetID.String())
			if err != nil {
				return err
			}
			if _, exists := concepts[conceptID]; !exists {
				return fmt.Errorf("coverage requirement %q references missing concept", requirement.ID)
			}
		}
	}
	return nil
}

func validateDeclaredEvidence(definition CurriculumDefinition) error {
	bundles := make(map[ID]struct{}, len(definition.SourceBundles))
	for _, bundle := range definition.SourceBundles {
		bundles[bundle.ID] = struct{}{}
	}
	collections := make([][]EvidenceRef, 0, len(definition.Goal.Outcomes)+len(definition.Competencies.Competencies)+len(definition.Concepts)+len(definition.Prerequisites)+len(definition.CoverageRequirements))
	for _, outcome := range definition.Goal.Outcomes {
		collections = append(collections, outcome.EvidenceRefs)
	}
	for _, competency := range definition.Competencies.Competencies {
		collections = append(collections, competency.EvidenceRefs)
	}
	for _, concept := range definition.Concepts {
		collections = append(collections, concept.EvidenceRefs)
	}
	for _, prerequisite := range definition.Prerequisites {
		collections = append(collections, prerequisite.EvidenceRefs)
	}
	for _, requirement := range definition.CoverageRequirements {
		collections = append(collections, requirement.EvidenceRefs)
	}
	for _, collection := range collections {
		for _, reference := range collection {
			if _, exists := bundles[reference.BundleID]; !exists {
				return fmt.Errorf("evidence references undeclared source bundle %q", reference.BundleID)
			}
		}
	}
	return nil
}
