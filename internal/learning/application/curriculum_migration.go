package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/learning"
)

const CurriculumInstanceMigrationPolicyV1 = "curriculum-instance-migration-v1"

type CurriculumInstanceMigrationRequest struct {
	PlanID                       string
	BackupID                     string
	OldCurriculum                learning.CurriculumRef
	NewCurriculum                learning.Curriculum
	PreserveConceptIDs           []learning.ID
	InitializeUnknownConceptIDs  []learning.ID
	RecalculateUnlockEligibility bool
}

type CurriculumInstanceMigrationImpact struct {
	PlanID                        string
	BackupID                      string
	SourceInstancesArchived       int
	TargetInstancesCreated        int
	ConceptStatesPreserved        int
	UnknownConceptStatesCreated   int
	RecalculatedUnlockEligibility bool
	PolicyVersion                 string
}

func (service *curriculumInstanceService) Migrate(ctx context.Context, request CurriculumInstanceMigrationRequest) (CurriculumInstanceMigrationImpact, error) {
	const operation = "migrate curriculum instances"
	if err := validateCurriculumInstanceMigrationRequest(request); err != nil {
		return CurriculumInstanceMigrationImpact{}, invalid(operation, err)
	}
	student, err := service.student(ctx, operation)
	if err != nil {
		return CurriculumInstanceMigrationImpact{}, err
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return CurriculumInstanceMigrationImpact{}, err
	}
	if service.generateID == nil {
		return CurriculumInstanceMigrationImpact{}, Classify(ErrorUnavailable, operation, errors.New("curriculum instance id generator is not configured"))
	}
	impact := CurriculumInstanceMigrationImpact{
		PlanID: request.PlanID, BackupID: request.BackupID,
		RecalculatedUnlockEligibility: request.RecalculateUnlockEligibility,
		PolicyVersion:                 CurriculumInstanceMigrationPolicyV1,
	}
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		if repositories.Definitions == nil || repositories.Curricula == nil || repositories.CurriculumInstances == nil || repositories.InstanceConceptStates == nil {
			return Classify(ErrorUnavailable, operation, errors.New("curriculum migration repositories are not configured"))
		}
		if err := repositories.Definitions.Install(ctx, request.NewCurriculum); err != nil {
			return err
		}
		instances, err := repositories.CurriculumInstances.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		targetByGoal := make(map[learning.ID]struct{})
		for _, instance := range instances {
			if instance.Curriculum == request.NewCurriculum.Reference {
				targetByGoal[instance.GoalID] = struct{}{}
			}
		}
		for _, source := range instances {
			if source.Curriculum != request.OldCurriculum || source.Status == learning.CurriculumInstanceArchived {
				continue
			}
			if _, exists := targetByGoal[source.GoalID]; exists {
				return Classify(ErrorConflict, operation, fmt.Errorf("goal %q already has a target curriculum instance", source.GoalID))
			}
			for _, conceptID := range request.PreserveConceptIDs {
				if _, err := repositories.Curricula.Concept(ctx, source.Curriculum, conceptID); err != nil {
					return Classify(ErrorInvalidState, operation, fmt.Errorf("preserved concept %q is absent from source curriculum: %w", conceptID, err))
				}
			}
			id, err := service.generateID()
			if err != nil {
				return Classify(ErrorUnavailable, operation, fmt.Errorf("generate target curriculum instance id: %w", err))
			}
			target, err := learning.NewCurriculumInstance(id, source.StudentID, source.GoalID, request.NewCurriculum.Reference, source.Source, timestamp)
			if err != nil {
				return Classify(ErrorInvalidState, operation, err)
			}
			target.Status = source.Status
			if err := target.Validate(); err != nil {
				return Classify(ErrorInvalidState, operation, err)
			}
			if err := repositories.CurriculumInstances.Create(ctx, target); err != nil {
				return err
			}
			for _, conceptID := range request.PreserveConceptIDs {
				state, err := repositories.InstanceConceptStates.Get(ctx, source.ID, conceptID)
				if errors.Is(err, ErrNotFound) {
					continue
				}
				if err != nil {
					return err
				}
				state.CurriculumInstanceID = target.ID
				state.UpdatedAt = timestamp
				if err := repositories.InstanceConceptStates.Save(ctx, state); err != nil {
					return err
				}
				impact.ConceptStatesPreserved++
			}
			for _, conceptID := range request.InitializeUnknownConceptIDs {
				state, err := learning.NewInstanceConceptState(target, conceptID, timestamp)
				if err != nil {
					return Classify(ErrorInvalidState, operation, err)
				}
				if err := repositories.InstanceConceptStates.Save(ctx, state); err != nil {
					return err
				}
				impact.UnknownConceptStatesCreated++
			}
			source.Status = learning.CurriculumInstanceArchived
			source.UpdatedAt = timestamp
			if err := repositories.CurriculumInstances.Update(ctx, source); err != nil {
				return err
			}
			targetByGoal[source.GoalID] = struct{}{}
			impact.SourceInstancesArchived++
			impact.TargetInstancesCreated++
		}
		return nil
	})
	if err != nil {
		return CurriculumInstanceMigrationImpact{}, err
	}
	return impact, nil
}

func validateCurriculumInstanceMigrationRequest(request CurriculumInstanceMigrationRequest) error {
	if strings.TrimSpace(request.PlanID) == "" {
		return errors.New("migration plan id is required")
	}
	if strings.TrimSpace(request.BackupID) == "" {
		return errors.New("backup id is required before curriculum migration")
	}
	if err := request.OldCurriculum.Validate(); err != nil {
		return err
	}
	if err := request.NewCurriculum.Validate(); err != nil {
		return err
	}
	if request.OldCurriculum.ID != request.NewCurriculum.Reference.ID || request.OldCurriculum.Version == request.NewCurriculum.Reference.Version {
		return errors.New("curriculum migration requires distinct versions of one curriculum")
	}
	targets := make(map[learning.ID]struct{})
	for name, values := range map[string][]learning.ID{"preserve concept ids": request.PreserveConceptIDs, "initialize concept ids": request.InitializeUnknownConceptIDs} {
		if !sort.SliceIsSorted(values, func(i, j int) bool { return values[i].String() < values[j].String() }) {
			return fmt.Errorf("%s must be sorted", name)
		}
		for _, id := range values {
			if err := id.Validate(); err != nil {
				return err
			}
			if _, duplicate := targets[id]; duplicate {
				return fmt.Errorf("target concept %q appears in multiple migration directives", id)
			}
			targets[id] = struct{}{}
			if node, exists := request.NewCurriculum.Node(id); !exists || node.Type != learning.CurriculumNodeConcept {
				return fmt.Errorf("target concept %q is absent from new curriculum", id)
			}
		}
	}
	return nil
}
