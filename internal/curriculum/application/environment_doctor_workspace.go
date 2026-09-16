package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

// WorkspaceEnvironmentDoctorV1 composes the active immutable pack with the
// learner's current I-02 position. The resulting plan remains inert metadata;
// Doctor owns all trusted tool discovery and execution.
type WorkspaceEnvironmentDoctorV1 struct {
	packs     PackInstallService
	positions StudentCurriculumPositionService
	planner   EnvironmentDoctorPlanService
	platform  string
}

func NewWorkspaceEnvironmentDoctorV1(packs PackInstallService, positions StudentCurriculumPositionService, planner EnvironmentDoctorPlanService, platform string) *WorkspaceEnvironmentDoctorV1 {
	return &WorkspaceEnvironmentDoctorV1{packs: packs, positions: positions, planner: planner, platform: platform}
}

func (service *WorkspaceEnvironmentDoctorV1) PlanForWorkspace(ctx context.Context, workspaceRoot string) (*curriculum.EnvironmentDoctorPlan, error) {
	const operation = "plan workspace curriculum environment"
	if service == nil || service.packs == nil || service.positions == nil || service.planner == nil || service.platform == "" {
		return nil, Classify(ErrorUnavailable, operation, fmt.Errorf("workspace environment doctor dependencies are not configured"))
	}
	installed, err := service.packs.Active(ctx, workspaceRoot)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if installed.Pack.Environment == nil {
		return nil, nil
	}
	current, err := service.positions.CurrentConcept(ctx, workspaceRoot)
	if err != nil {
		return nil, ExternalError(operation, err)
	}
	graph, err := NewKnowledgeGraphCompilerV1().Compile(ctx, KnowledgeGraphCompilationRequest{
		Concepts: installed.Pack.Curriculum.Concepts, Prerequisites: installed.Pack.Curriculum.Prerequisites,
	})
	if err != nil {
		return nil, err
	}
	hierarchy := curriculum.CurriculumHierarchy{
		Phases: installed.Pack.Curriculum.Phases, Modules: installed.Pack.Curriculum.Modules,
		Lessons: installed.Pack.Curriculum.Lessons, Topics: installed.Pack.Curriculum.Topics,
		AlgorithmVersion: hierarchyAlgorithmVersion(installed.Pack),
	}
	plan, err := service.planner.Plan(ctx, EnvironmentDoctorPlanRequest{
		Environment: *installed.Pack.Environment, Concepts: installed.Pack.Curriculum.Concepts,
		Graph: graph, Hierarchy: hierarchy, CurrentConceptID: current, Platform: service.platform,
	})
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func hierarchyAlgorithmVersion(pack curriculum.LearningPack) string {
	if pack.BuildInfo != nil {
		for _, pass := range pack.BuildInfo.Passes {
			if pass.Name == "hierarchy" {
				return pass.Version
			}
		}
	}
	return curriculum.CurriculumHierarchyBuilderVersionV1
}

var _ WorkspaceEnvironmentDoctorService = (*WorkspaceEnvironmentDoctorV1)(nil)
