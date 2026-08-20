package application

import (
	"context"
	"errors"

	"github.com/mishaaac/kelyro/internal/learning"
)

type prerequisiteService struct {
	graph     *learning.KnowledgeGraph
	profiles  ProfileService
	mastery   MasteryPolicyService
	instances CurriculumInstanceRepository
	states    InstanceConceptStateRepository
}

// NewPrerequisiteService wires persistence reads around the pure in-memory
// graph. Each evaluation loads concept states once; traversal performs no I/O.
func NewPrerequisiteService(graph *learning.KnowledgeGraph, profiles ProfileService, mastery MasteryPolicyService, instances CurriculumInstanceRepository, states InstanceConceptStateRepository) PrerequisiteService {
	return &prerequisiteService{graph: graph, profiles: profiles, mastery: mastery, instances: instances, states: states}
}

func (service *prerequisiteService) EvaluateIntroduction(ctx context.Context, instanceID, conceptID learning.ID, pack *learning.PackMasteryOverride) (learning.IntroductionDecision, error) {
	const operation = "evaluate concept prerequisites"
	if service == nil || service.graph == nil || service.profiles == nil || service.mastery == nil || service.instances == nil || service.states == nil {
		return learning.IntroductionDecision{}, Classify(ErrorUnavailable, operation, errors.New("prerequisite service dependencies are not configured"))
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return learning.IntroductionDecision{}, err
	}
	instance, err := service.instances.Get(ctx, instanceID)
	if err != nil {
		return learning.IntroductionDecision{}, repositoryError(operation, err)
	}
	if instance.StudentID != student.ID {
		return learning.IntroductionDecision{}, Classify(ErrorNotFound, operation, errors.New("curriculum instance not found"))
	}
	if service.graph.Reference() != instance.Curriculum {
		return learning.IntroductionDecision{}, Classify(ErrorInvalidState, operation, errors.New("knowledge graph does not match curriculum instance version"))
	}
	policy, err := service.mastery.Show(ctx, pack)
	if err != nil {
		return learning.IntroductionDecision{}, err
	}
	instanceStates, err := service.states.ListByInstance(ctx, instance.ID)
	if err != nil {
		return learning.IntroductionDecision{}, repositoryError(operation, err)
	}
	states := make([]learning.ConceptState, 0, len(instanceStates))
	for _, state := range instanceStates {
		states = append(states, state.ConceptState())
	}
	snapshot, err := learning.NewStudentStateSnapshot(states)
	if err != nil {
		return learning.IntroductionDecision{}, repositoryError(operation, err)
	}
	decision, err := service.graph.EvaluateIntroduction(conceptID, snapshot, policy)
	if err != nil {
		return learning.IntroductionDecision{}, invalid(operation, err)
	}
	return decision, nil
}
