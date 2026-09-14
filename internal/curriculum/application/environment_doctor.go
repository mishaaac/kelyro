package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type EnvironmentDoctorPlannerV1 struct{}

func NewEnvironmentDoctorPlannerV1() EnvironmentDoctorPlannerV1 { return EnvironmentDoctorPlannerV1{} }

type curriculumLocation struct {
	phaseTitle  string
	moduleTitle string
}

func (EnvironmentDoctorPlannerV1) Plan(ctx context.Context, request EnvironmentDoctorPlanRequest) (curriculum.EnvironmentDoctorPlan, error) {
	const operation = "plan curriculum-aware environment diagnostics"
	if err := ctx.Err(); err != nil {
		return curriculum.EnvironmentDoctorPlan{}, ExternalError(operation, err)
	}
	if err := request.Environment.ValidatePortableV1(); err != nil {
		return curriculum.EnvironmentDoctorPlan{}, Invalid(operation, err)
	}
	if err := request.Graph.Validate(); err != nil {
		return curriculum.EnvironmentDoctorPlan{}, Invalid(operation, err)
	}
	if err := request.Hierarchy.Validate(request.Concepts); err != nil {
		return curriculum.EnvironmentDoctorPlan{}, Invalid(operation, err)
	}
	if !containsPlatform(request.Environment.SupportedPlatforms, request.Platform) {
		return curriculum.EnvironmentDoctorPlan{}, Invalid(operation, fmt.Errorf("environment pack does not support platform %q", request.Platform))
	}
	positions := make(map[curriculum.ConceptID]int, len(request.Graph.TopologicalOrder))
	for index, conceptID := range request.Graph.TopologicalOrder {
		positions[conceptID] = index
	}
	currentPosition, exists := positions[request.CurrentConceptID]
	if !exists {
		return curriculum.EnvironmentDoctorPlan{}, Invalid(operation, fmt.Errorf("current concept %q is not in the compiled graph", request.CurrentConceptID))
	}
	locations := hierarchyLocations(request.Hierarchy)
	currentLocation := locations[request.CurrentConceptID]
	guidance := make(map[curriculum.ID]curriculum.ToolInstallGuidance, len(request.Environment.InstallGuidance))
	for _, item := range request.Environment.InstallGuidance {
		guidance[item.ID] = item
	}

	plan := curriculum.EnvironmentDoctorPlan{
		EnvironmentPack: curriculum.EnvironmentPackReference{ID: request.Environment.ID, Version: request.Environment.Version},
		Platform:        request.Platform, CurrentConceptID: request.CurrentConceptID,
		AlgorithmVersion: curriculum.EnvironmentDoctorPlannerVersionV1,
	}
	for _, tool := range request.Environment.Tools {
		if err := ctx.Err(); err != nil {
			return curriculum.EnvironmentDoctorPlan{}, ExternalError(operation, err)
		}
		if !containsPlatform(tool.Platforms, request.Platform) {
			continue
		}
		introducedPosition, introducedExists := positions[*tool.IntroducedAt]
		neededPosition, neededExists := positions[*tool.WhenNeeded]
		if !introducedExists || !neededExists {
			return curriculum.EnvironmentDoctorPlan{}, Invalid(operation, fmt.Errorf("tool %q timing references concepts outside the compiled graph", tool.ID))
		}
		if neededPosition < introducedPosition {
			return curriculum.EnvironmentDoctorPlan{}, Invalid(operation, fmt.Errorf("tool %q is needed before its introduction", tool.ID))
		}
		selectedGuidance, found := platformInstallGuidance(tool, guidance, request.Platform)
		if !found {
			return curriculum.EnvironmentDoctorPlan{}, Invalid(operation, fmt.Errorf("tool %q has no official guidance for platform %q", tool.ID, request.Platform))
		}
		neededLocation := locations[*tool.WhenNeeded]
		timing := curriculum.EnvironmentToolCurrent
		reason := fmt.Sprintf("%s It is %s for the current curriculum in module %s.", tool.Purpose, tool.Level, neededLocation.moduleTitle)
		if currentPosition < introducedPosition {
			timing = curriculum.EnvironmentToolNotNeededYet
			reason = fmt.Sprintf("%s It is introduced later and is not needed yet; first required in module %s.", tool.Purpose, neededLocation.moduleTitle)
		} else if currentPosition < neededPosition {
			timing = curriculum.EnvironmentToolFuture
			reason = fmt.Sprintf("%s It has been introduced and will be needed in future module %s.", tool.Purpose, neededLocation.moduleTitle)
		}
		plan.Tools = append(plan.Tools, curriculum.EnvironmentToolDiagnostic{
			ToolID: tool.ID, DisplayName: tool.DisplayName, Level: tool.Level, MinimumVersion: tool.MinimumVersion,
			Timing: timing, WhyNeeded: reason,
			CurrentPhase: currentLocation.phaseTitle, CurrentModule: currentLocation.moduleTitle,
			NeededPhase: neededLocation.phaseTitle, NeededModule: neededLocation.moduleTitle,
			OfficialSourceName: selectedGuidance.SourceName, OfficialURL: selectedGuidance.OfficialURL,
			InstallInstructions: selectedGuidance.Instructions,
		})
	}
	sort.Slice(plan.Tools, func(i, j int) bool { return plan.Tools[i].ToolID.String() < plan.Tools[j].ToolID.String() })
	if err := plan.Validate(); err != nil {
		return curriculum.EnvironmentDoctorPlan{}, Invalid(operation, err)
	}
	return plan, nil
}

func hierarchyLocations(hierarchy curriculum.CurriculumHierarchy) map[curriculum.ConceptID]curriculumLocation {
	phases := make(map[curriculum.ID]curriculum.Phase, len(hierarchy.Phases))
	for _, phase := range hierarchy.Phases {
		phases[phase.ID] = phase
	}
	modules := make(map[curriculum.ID]curriculum.Module, len(hierarchy.Modules))
	for _, module := range hierarchy.Modules {
		modules[module.ID] = module
	}
	lessons := make(map[curriculum.ID]curriculum.LessonSpec, len(hierarchy.Lessons))
	for _, lesson := range hierarchy.Lessons {
		lessons[lesson.ID] = lesson
	}
	result := make(map[curriculum.ConceptID]curriculumLocation)
	for _, topic := range hierarchy.Topics {
		module := modules[lessons[topic.LessonID].ModuleID]
		location := curriculumLocation{phaseTitle: phases[module.PhaseID].Title, moduleTitle: module.Title}
		for _, conceptID := range topic.ConceptIDs {
			result[conceptID] = location
		}
	}
	return result
}

func platformInstallGuidance(tool curriculum.ToolRequirement, guidance map[curriculum.ID]curriculum.ToolInstallGuidance, platform string) (curriculum.ToolInstallGuidance, bool) {
	var candidates []curriculum.ToolInstallGuidance
	for _, reference := range tool.InstallGuidanceRefs {
		if item := guidance[reference]; item.Platform == platform {
			candidates = append(candidates, item)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID.String() < candidates[j].ID.String() })
	if len(candidates) == 0 {
		return curriculum.ToolInstallGuidance{}, false
	}
	return candidates[0], true
}

func containsPlatform(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

var _ EnvironmentDoctorPlanService = EnvironmentDoctorPlannerV1{}
