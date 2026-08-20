package application

import (
	"context"
	"errors"

	"github.com/mishaaac/kelyro/internal/learning"
)

type prerequisiteService struct {
	graph    *learning.KnowledgeGraph
	profiles ProfileService
	mastery  MasteryPolicyService
	states   ConceptStateRepository
}

// NewPrerequisiteService wires persistence reads around the pure in-memory
// graph. Each evaluation loads concept states once; traversal performs no I/O.
func NewPrerequisiteService(graph *learning.KnowledgeGraph, profiles ProfileService, mastery MasteryPolicyService, states ConceptStateRepository) PrerequisiteService {
	return &prerequisiteService{graph: graph, profiles: profiles, mastery: mastery, states: states}
}

func (service *prerequisiteService) EvaluateIntroduction(ctx context.Context, conceptID learning.ID, pack *learning.PackMasteryOverride) (learning.IntroductionDecision, error) {
	const operation = "evaluate concept prerequisites"
	if service == nil || service.graph == nil || service.profiles == nil || service.mastery == nil || service.states == nil {
		return learning.IntroductionDecision{}, Classify(ErrorUnavailable, operation, errors.New("prerequisite service dependencies are not configured"))
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return learning.IntroductionDecision{}, err
	}
	policy, err := service.mastery.Show(ctx, pack)
	if err != nil {
		return learning.IntroductionDecision{}, err
	}
	states, err := service.states.ListByStudent(ctx, student.ID)
	if err != nil {
		return learning.IntroductionDecision{}, repositoryError(operation, err)
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
