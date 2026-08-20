package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type ProgressionOption func(*progressionService)

func WithProgressionClock(now func() time.Time) ProgressionOption {
	return func(service *progressionService) {
		if now != nil {
			service.now = now
		}
	}
}

type progressionService struct {
	graph      *learning.KnowledgeGraph
	profiles   ProfileService
	thresholds MasteryPolicyService
	unitOfWork UnitOfWork
	now        func() time.Time
}

func NewProgressionService(graph *learning.KnowledgeGraph, profiles ProfileService, thresholds MasteryPolicyService, unitOfWork UnitOfWork, options ...ProgressionOption) ProgressionService {
	service := &progressionService{graph: graph, profiles: profiles, thresholds: thresholds, unitOfWork: unitOfWork, now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *progressionService) RecordEvidence(ctx context.Context, instanceID learning.ID, evidence learning.Evidence, pack *learning.PackMasteryOverride) (ProgressionUpdate, error) {
	const operation = "record evidence and update progression"
	if err := evidence.Validate(); err != nil {
		return ProgressionUpdate{}, invalid(operation, err)
	}
	return service.update(ctx, operation, instanceID, evidence.ConceptID, &evidence, pack)
}

func (service *progressionService) Recalculate(ctx context.Context, instanceID, conceptID learning.ID, pack *learning.PackMasteryOverride) (ProgressionUpdate, error) {
	const operation = "recalculate concept progression"
	if err := validatePair("curriculum instance", instanceID, "concept", conceptID); err != nil {
		return ProgressionUpdate{}, invalid(operation, err)
	}
	return service.update(ctx, operation, instanceID, conceptID, nil, pack)
}

func (service *progressionService) update(ctx context.Context, operation string, instanceID, conceptID learning.ID, evidence *learning.Evidence, pack *learning.PackMasteryOverride) (ProgressionUpdate, error) {
	if service == nil || service.graph == nil || service.profiles == nil || service.thresholds == nil || service.unitOfWork == nil || service.now == nil {
		return ProgressionUpdate{}, Classify(ErrorUnavailable, operation, errors.New("progression service dependencies are not configured"))
	}
	if err := validatePair("curriculum instance", instanceID, "concept", conceptID); err != nil {
		return ProgressionUpdate{}, invalid(operation, err)
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return ProgressionUpdate{}, err
	}
	if evidence != nil && (evidence.StudentID != student.ID || evidence.ConceptID != conceptID) {
		return ProgressionUpdate{}, invalid(operation, errors.New("evidence belongs to another student or concept"))
	}
	threshold, err := service.thresholds.Show(ctx, pack)
	if err != nil {
		return ProgressionUpdate{}, err
	}
	updatedAt, err := learning.NewTimestamp(service.now())
	if err != nil {
		return ProgressionUpdate{}, invalid(operation, err)
	}

	var update ProgressionUpdate
	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		instance, loadErr := repositories.CurriculumInstances.Get(ctx, instanceID)
		if loadErr != nil {
			return loadErr
		}
		if instance.StudentID != student.ID {
			return Classify(ErrorNotFound, operation, errors.New("curriculum instance not found"))
		}
		if instance.Status != learning.CurriculumInstanceActive {
			return Classify(ErrorInvalidState, operation, fmt.Errorf("curriculum instance must be active, got %q", instance.Status))
		}
		if service.graph.Reference() != instance.Curriculum {
			return Classify(ErrorInvalidState, operation, errors.New("knowledge graph does not match curriculum instance version"))
		}
		if _, loadErr := repositories.Curricula.Concept(ctx, instance.Curriculum, conceptID); loadErr != nil {
			return loadErr
		}

		beforeStates, loadErr := repositories.InstanceConceptStates.ListByInstance(ctx, instance.ID)
		if loadErr != nil {
			return loadErr
		}
		state, present := findInstanceState(beforeStates, conceptID)
		if !present {
			state, loadErr = learning.NewInstanceConceptState(instance, conceptID, instance.CreatedAt)
			if loadErr != nil {
				return Classify(ErrorInvalidState, operation, loadErr)
			}
		}
		if evidence != nil {
			if appendErr := repositories.Evidence.Append(ctx, *evidence); appendErr != nil {
				return appendErr
			}
		}

		calculator := NewMasteryCalculationService(repositories.Evidence)
		calculation, calculateErr := calculator.Calculate(ctx, student.ID, conceptID)
		if calculateErr != nil {
			return calculateErr
		}
		progression, applyErr := learning.ApplyProgressionV1(state, calculation, threshold, instance.CreatedAt, updatedAt)
		if applyErr != nil {
			return Classify(ErrorInvalidState, operation, applyErr)
		}
		if progression.StateChanged {
			if saveErr := repositories.InstanceConceptStates.Save(ctx, progression.State); saveErr != nil {
				return saveErr
			}
		}

		beforeSnapshot, snapshotErr := progressionSnapshot(beforeStates)
		if snapshotErr != nil {
			return Classify(ErrorInvalidState, operation, snapshotErr)
		}
		afterStates := replaceInstanceState(beforeStates, progression.State, progression.StateChanged)
		afterSnapshot, snapshotErr := progressionSnapshot(afterStates)
		if snapshotErr != nil {
			return Classify(ErrorInvalidState, operation, snapshotErr)
		}
		dependents, graphErr := service.graph.GetDependents(conceptID)
		if graphErr != nil {
			return Classify(ErrorInvalidState, operation, graphErr)
		}
		decisions := make([]DependentProgression, 0, len(dependents))
		for _, dependentID := range dependents {
			before, evaluateErr := service.graph.EvaluateIntroduction(dependentID, beforeSnapshot, threshold)
			if evaluateErr != nil {
				return Classify(ErrorInvalidState, operation, evaluateErr)
			}
			after, evaluateErr := service.graph.EvaluateIntroduction(dependentID, afterSnapshot, threshold)
			if evaluateErr != nil {
				return Classify(ErrorInvalidState, operation, evaluateErr)
			}
			decisions = append(decisions, DependentProgression{Decision: after, WasEligible: before.CanIntroduce, NewlyEligible: !before.CanIntroduce && after.CanIntroduce})
		}
		update = ProgressionUpdate{Progression: progression, Dependents: decisions}
		return nil
	})
	if err != nil {
		return ProgressionUpdate{}, repositoryError(operation, err)
	}
	return update, nil
}

func findInstanceState(states []learning.InstanceConceptState, conceptID learning.ID) (learning.InstanceConceptState, bool) {
	for _, state := range states {
		if state.ConceptID == conceptID {
			return state, true
		}
	}
	return learning.InstanceConceptState{}, false
}

func replaceInstanceState(states []learning.InstanceConceptState, updated learning.InstanceConceptState, replace bool) []learning.InstanceConceptState {
	cloned := append([]learning.InstanceConceptState(nil), states...)
	if !replace {
		return cloned
	}
	for index := range cloned {
		if cloned[index].ConceptID == updated.ConceptID {
			cloned[index] = updated
			return cloned
		}
	}
	return append(cloned, updated)
}

func progressionSnapshot(states []learning.InstanceConceptState) (learning.StudentStateSnapshot, error) {
	projected := make([]learning.ConceptState, 0, len(states))
	for _, state := range states {
		projected = append(projected, state.ConceptState())
	}
	return learning.NewStudentStateSnapshot(projected)
}

var _ ProgressionService = (*progressionService)(nil)
