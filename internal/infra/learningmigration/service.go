// Package learningmigration projects I-04 curriculum definitions into the
// published I-02 consumption contract and delegates all learner writes to the
// Student Core application service.
package learningmigration

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/backup"
	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/platform"
)

const CurriculumConsumptionProjectionVersionV1 = "curriculum-consumption-projection-v1"

type Service struct {
	stores    learningapp.ProfileStoreFactory
	validator backup.Validator
}

func New(stores learningapp.ProfileStoreFactory, validator backup.Validator) *Service {
	return &Service{stores: stores, validator: validator}
}

func (service *Service) Apply(ctx context.Context, request curriculumapp.StudentCurriculumMigrationRequest) (result curriculumapp.StudentCurriculumMigrationResult, err error) {
	if service == nil || service.stores == nil {
		return result, fmt.Errorf("Student Core migration store is not configured")
	}
	if strings.TrimSpace(request.WorkspaceRoot) == "" || strings.TrimSpace(request.BackupID) == "" {
		return result, fmt.Errorf("workspace root and backup id are required")
	}
	if err := request.Old.Validate(); err != nil {
		return result, fmt.Errorf("old curriculum: %w", err)
	}
	if err := request.New.Validate(); err != nil {
		return result, fmt.Errorf("new curriculum: %w", err)
	}
	if err := request.Plan.Validate(); err != nil {
		return result, fmt.Errorf("migration plan: %w", err)
	}
	definition, err := ProjectCurriculum(request.New)
	if err != nil {
		return result, err
	}
	preserve, initialize, err := migrationConceptIDs(request.Plan)
	if err != nil {
		return result, err
	}
	oldID, err := learning.NewID(request.Old.ID.String())
	if err != nil {
		return result, err
	}
	store, err := service.stores.Open(ctx, request.WorkspaceRoot)
	if err != nil {
		return result, err
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	impact, err := store.CurriculumInstances().Migrate(ctx, learningapp.CurriculumInstanceMigrationRequest{
		PlanID: request.Plan.ID.String(), BackupID: request.BackupID,
		OldCurriculum: learning.CurriculumRef{ID: oldID, Version: request.Old.Version.String()}, NewCurriculum: definition,
		PreserveConceptIDs: preserve, InitializeUnknownConceptIDs: initialize,
		RecalculateUnlockEligibility: request.Plan.RecalculateUnlockEligibility,
	})
	if err != nil {
		return result, err
	}
	return curriculumapp.StudentCurriculumMigrationResult{
		SourceInstancesArchived: impact.SourceInstancesArchived, TargetInstancesCreated: impact.TargetInstancesCreated,
		ConceptStatesPreserved: impact.ConceptStatesPreserved, UnknownConceptStatesCreated: impact.UnknownConceptStatesCreated,
		RecalculatedUnlocks: impact.RecalculatedUnlockEligibility, ProjectionVersion: CurriculumConsumptionProjectionVersionV1,
	}, nil
}

func (service *Service) CheckIntegrity(ctx context.Context, workspaceRoot string) error {
	if service == nil || service.validator == nil {
		return fmt.Errorf("Student Core integrity validator is not configured")
	}
	path, err := platform.WorkspaceDBPath(workspaceRoot)
	if err != nil {
		return err
	}
	_, err = service.validator.Validate(ctx, path)
	return err
}

// ProjectCurriculum performs the deterministic mechanical projection from the
// richer I-04 model to I-02's established consumption contract.
func ProjectCurriculum(definition curriculum.CurriculumDefinition) (learning.Curriculum, error) {
	if err := definition.Validate(); err != nil {
		return learning.Curriculum{}, err
	}
	toID := func(value string) (learning.ID, error) { return learning.NewID(value) }
	curriculumID, err := toID(definition.ID.String())
	if err != nil {
		return learning.Curriculum{}, err
	}
	nodes := make([]learning.CurriculumNode, 0, len(definition.Phases)+len(definition.Modules)+len(definition.Lessons)+len(definition.Topics)+len(definition.Concepts))
	active := learning.CurriculumStatusMetadata{State: learning.CurriculumNodeActive}
	version := definition.Version.String()
	for _, phase := range definition.Phases {
		id, _ := toID(phase.ID.String())
		nodes = append(nodes, learning.CurriculumNode{ID: id, Type: learning.CurriculumNodePhase, Title: phase.Title, Description: phase.Description, Order: phase.Order, Status: active, Version: version})
	}
	for _, module := range definition.Modules {
		id, _ := toID(module.ID.String())
		parent, _ := toID(module.PhaseID.String())
		nodes = append(nodes, learning.CurriculumNode{ID: id, Type: learning.CurriculumNodeModule, ParentID: &parent, Title: module.Title, Description: module.Description, Order: module.Order, Status: active, Version: version})
	}
	for _, lesson := range definition.Lessons {
		id, _ := toID(lesson.ID.String())
		parent, _ := toID(lesson.ModuleID.String())
		nodes = append(nodes, learning.CurriculumNode{ID: id, Type: learning.CurriculumNodeLesson, ParentID: &parent, Title: lesson.Title, Description: lesson.Description, Order: lesson.Order, Status: active, Version: version})
	}
	topicByConcept := make(map[curriculum.ConceptID]struct {
		id    learning.ID
		order int
	})
	for _, topic := range definition.Topics {
		id, _ := toID(topic.ID.String())
		parent, _ := toID(topic.LessonID.String())
		nodes = append(nodes, learning.CurriculumNode{ID: id, Type: learning.CurriculumNodeTopic, ParentID: &parent, Title: topic.Title, Description: topic.Description, Order: topic.Order, Status: active, Version: version})
		for order, conceptID := range topic.ConceptIDs {
			topicByConcept[conceptID] = struct {
				id    learning.ID
				order int
			}{id: id, order: order}
		}
	}
	prerequisites := consumptionPrerequisites(definition.Prerequisites)
	for _, concept := range definition.Concepts {
		id, _ := toID(concept.ID.String())
		location := topicByConcept[concept.ID]
		status := active
		if concept.Status == curriculum.ConceptDeprecated || concept.Status == curriculum.ConceptLegacy || concept.Status == curriculum.ConceptHistorical {
			status = learning.CurriculumStatusMetadata{State: learning.CurriculumNodeDeprecated, Note: "Preserved historical guidance."}
		}
		conceptPrerequisites := make([]learning.ConceptPrerequisite, 0, len(prerequisites[concept.ID]))
		for _, prerequisite := range prerequisites[concept.ID] {
			required, _ := toID(prerequisite.id.String())
			conceptPrerequisites = append(conceptPrerequisites, learning.ConceptPrerequisite{ConceptID: required, Requirement: prerequisite.requirement})
		}
		nodes = append(nodes, learning.CurriculumNode{
			ID: id, Type: learning.CurriculumNodeConcept, ParentID: &location.id, Title: concept.Title, Description: concept.Definition,
			Order: location.order, Status: status, Version: concept.Version,
			Concept: &learning.ConceptDefinition{
				Objectives: []string{concept.Definition}, Prerequisites: conceptPrerequisites,
				Difficulty: learning.ConceptDifficulty(concept.Difficulty), EstimatedEffortMinutes: int(concept.Difficulty) * 30,
				TheoryRequired: concept.Foundational, AssessmentExpectations: []string{concept.Definition},
			},
		})
	}
	return learning.NewCurriculum(learning.CurriculumContractVersion,
		learning.CurriculumRef{ID: curriculumID, Version: version}, definition.Title, definition.Description, nodes)
}

type projectedPrerequisite struct {
	id          curriculum.ConceptID
	requirement learning.PrerequisiteRequirement
}

func consumptionPrerequisites(values []curriculum.Prerequisite) map[curriculum.ConceptID][]projectedPrerequisite {
	byConcept := make(map[curriculum.ConceptID]map[curriculum.ConceptID]learning.PrerequisiteRequirement)
	for _, value := range values {
		if value.Kind == curriculum.PrerequisiteRecommended {
			continue
		}
		requirement := learning.PrerequisiteIntroduced
		if value.Kind == curriculum.PrerequisiteHard {
			requirement = learning.PrerequisiteMastered
		}
		if byConcept[value.ConceptID] == nil {
			byConcept[value.ConceptID] = make(map[curriculum.ConceptID]learning.PrerequisiteRequirement)
		}
		current, exists := byConcept[value.ConceptID][value.RequiredConceptID]
		if !exists || current != learning.PrerequisiteMastered {
			byConcept[value.ConceptID][value.RequiredConceptID] = requirement
		}
	}
	result := make(map[curriculum.ConceptID][]projectedPrerequisite, len(byConcept))
	for conceptID, requirements := range byConcept {
		for id, requirement := range requirements {
			result[conceptID] = append(result[conceptID], projectedPrerequisite{id: id, requirement: requirement})
		}
		sort.Slice(result[conceptID], func(i, j int) bool { return result[conceptID][i].id.String() < result[conceptID][j].id.String() })
	}
	return result
}

func migrationConceptIDs(plan curriculum.CurriculumMigrationPlan) (preserve, initialize []learning.ID, err error) {
	for _, action := range plan.Actions {
		switch action.Kind {
		case curriculum.MigrationPreserveState:
			id, convertErr := learning.NewID(action.ToConceptIDs[0].String())
			if convertErr != nil {
				return nil, nil, convertErr
			}
			preserve = append(preserve, id)
		case curriculum.MigrationInitializeUnknown, curriculum.MigrationSplitNoTransfer, curriculum.MigrationMergeNoTransfer:
			for _, conceptID := range action.ToConceptIDs {
				id, convertErr := learning.NewID(conceptID.String())
				if convertErr != nil {
					return nil, nil, convertErr
				}
				initialize = append(initialize, id)
			}
		}
	}
	sort.Slice(preserve, func(i, j int) bool { return preserve[i].String() < preserve[j].String() })
	sort.Slice(initialize, func(i, j int) bool { return initialize[i].String() < initialize[j].String() })
	return preserve, initialize, nil
}

var _ curriculumapp.StudentCurriculumMigrationService = (*Service)(nil)
