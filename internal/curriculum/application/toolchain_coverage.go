package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type ToolchainCoverageV1 struct{}

func NewToolchainCoverageV1() ToolchainCoverageV1 {
	return ToolchainCoverageV1{}
}

type environmentPackKey struct {
	id      string
	version string
}

func (ToolchainCoverageV1) Analyze(ctx context.Context, request ToolchainCoverageRequest) (curriculum.ToolchainCoverageReport, error) {
	const operation = "analyze toolchain coverage"
	if err := ctx.Err(); err != nil {
		return curriculum.ToolchainCoverageReport{}, ExternalError(operation, err)
	}
	if err := request.GoalID.Validate(); err != nil {
		return curriculum.ToolchainCoverageReport{}, Invalid(operation, err)
	}
	knownEvidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.ToolchainCoverageReport{}, Invalid(operation, err)
	}
	concepts := make(map[curriculum.ConceptID]curriculum.Concept, len(request.Concepts))
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return curriculum.ToolchainCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := concepts[concept.ID]; exists {
			return curriculum.ToolchainCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate toolchain concept %q", concept.ID))
		}
		if err := requireKnownEvidence(concept.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.ToolchainCoverageReport{}, Invalid(operation, fmt.Errorf("toolchain concept %q: %w", concept.ID, err))
		}
		concepts[concept.ID] = concept
	}
	packs := make(map[environmentPackKey]curriculum.EnvironmentPack, len(request.EnvironmentPacks))
	for _, pack := range request.EnvironmentPacks {
		if err := pack.Validate(); err != nil {
			return curriculum.ToolchainCoverageReport{}, Invalid(operation, err)
		}
		key := environmentPackKey{id: pack.ID.String(), version: pack.Version.String()}
		if _, exists := packs[key]; exists {
			return curriculum.ToolchainCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate environment pack %q version %q", pack.ID, pack.Version))
		}
		for _, tool := range pack.Tools {
			if err := requireKnownEvidence(tool.EvidenceRefs, knownEvidence); err != nil {
				return curriculum.ToolchainCoverageReport{}, Invalid(operation, fmt.Errorf("environment tool %q: %w", tool.ID, err))
			}
		}
		packs[key] = pack
	}
	requirements := make(map[curriculum.ID]curriculum.ToolchainCoverageRequirement, len(request.Requirements))
	seenTargets := make(map[string]struct{}, len(request.Requirements))
	for _, requirement := range request.Requirements {
		if err := requirement.Validate(); err != nil {
			return curriculum.ToolchainCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := requirements[requirement.ID]; exists {
			return curriculum.ToolchainCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate toolchain requirement %q", requirement.ID))
		}
		targetKey := requirement.EnvironmentPack.ID.String() + "\x00" + requirement.EnvironmentPack.Version.String() + "\x00" + requirement.ToolID.String()
		if _, exists := seenTargets[targetKey]; exists {
			return curriculum.ToolchainCoverageReport{}, Invalid(operation, fmt.Errorf("multiple toolchain requirements target tool %q in the same environment pack", requirement.ToolID))
		}
		seenTargets[targetKey] = struct{}{}
		if err := requireKnownEvidence(requirement.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.ToolchainCoverageReport{}, Invalid(operation, fmt.Errorf("toolchain requirement %q: %w", requirement.ID, err))
		}
		requirements[requirement.ID] = requirement
	}
	if len(requirements) == 0 {
		return curriculum.ToolchainCoverageReport{}, Invalid(operation, fmt.Errorf("toolchain coverage has no declared requirements"))
	}

	report := curriculum.ToolchainCoverageReport{AlgorithmVersion: curriculum.ToolchainCoverageVersionV1}
	ids := make([]curriculum.ID, 0, len(requirements))
	for id := range requirements {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return curriculum.ToolchainCoverageReport{}, ExternalError(operation, err)
		}
		requirement := requirements[id]
		report.CoverageRequirements = append(report.CoverageRequirements, curriculum.CoverageRequirement{
			ID: requirement.ID, Dimension: curriculum.CoverageToolchain, TargetKind: curriculum.CoverageTargetGoal,
			TargetID: request.GoalID, Description: "Provide tool " + requirement.ToolID.String() + " through environment pack " + requirement.EnvironmentPack.ID.String() + " at level " + string(requirement.MinimumLevel) + ".",
			EvidenceRefs: sortedEvidenceCopy(requirement.EvidenceRefs),
		})
		result := curriculum.ToolchainCoverageResult{
			RequirementID: requirement.ID, ToolID: requirement.ToolID,
			EnvironmentPack: requirement.EnvironmentPack, Status: curriculum.CoverageMissing,
		}
		packKey := environmentPackKey{id: requirement.EnvironmentPack.ID.String(), version: requirement.EnvironmentPack.Version.String()}
		pack, exists := packs[packKey]
		if !exists {
			result.MissingFields = []string{"environment_pack"}
			result.Reasons = []string{"referenced_environment_pack_unavailable"}
			report.Results = append(report.Results, result)
			continue
		}
		tool, exists := findEnvironmentTool(pack.Tools, requirement.ToolID)
		if !exists {
			result.MissingFields = []string{"tool"}
			result.Reasons = []string{"required_tool_absent_from_environment_pack"}
			report.Results = append(report.Results, result)
			continue
		}
		resolved := cloneToolRequirement(tool)
		result.ResolvedTool = &resolved
		if tool.IntroducedAt == nil {
			result.MissingFields = append(result.MissingFields, "when_introduced")
		} else if _, exists := concepts[*tool.IntroducedAt]; !exists {
			result.MissingFields = append(result.MissingFields, "introduction_concept")
		}
		if !toolLevelSatisfies(tool.Level, requirement.MinimumLevel) {
			result.MissingFields = append(result.MissingFields, "requirement_level")
		}
		if len(tool.Platforms) == 0 {
			result.MissingFields = append(result.MissingFields, "platform_notes")
		}
		if len(tool.EvidenceRefs) == 0 {
			result.MissingFields = append(result.MissingFields, "evidence")
		}
		sort.Strings(result.MissingFields)
		if len(result.MissingFields) == 0 {
			result.Status = curriculum.CoverageCovered
			result.Reasons = []string{"tool_requirement_complete"}
			report.CoverageSupports = append(report.CoverageSupports, curriculum.CoverageSupport{
				ID: requirement.ID, RequirementID: requirement.ID,
				ConceptIDs:   []curriculum.ConceptID{*tool.IntroducedAt},
				EvidenceRefs: sortedEvidenceCopy(tool.EvidenceRefs), ArtifactRefs: []curriculum.ID{pack.ID},
				Reason: "Environment pack " + pack.ID.String() + " provides the tool: " + tool.Purpose,
			})
		} else {
			result.Status = curriculum.CoveragePartial
			result.Reasons = []string{"tool_requirement_metadata_incomplete"}
		}
		report.Results = append(report.Results, result)
	}
	sort.Slice(report.CoverageSupports, func(i, j int) bool {
		if report.CoverageSupports[i].RequirementID != report.CoverageSupports[j].RequirementID {
			return report.CoverageSupports[i].RequirementID.String() < report.CoverageSupports[j].RequirementID.String()
		}
		return report.CoverageSupports[i].ID.String() < report.CoverageSupports[j].ID.String()
	})
	if err := report.Validate(); err != nil {
		return curriculum.ToolchainCoverageReport{}, Invalid(operation, err)
	}
	return report, nil
}

func findEnvironmentTool(tools []curriculum.ToolRequirement, id curriculum.ID) (curriculum.ToolRequirement, bool) {
	for _, tool := range tools {
		if tool.ID == id {
			return tool, true
		}
	}
	return curriculum.ToolRequirement{}, false
}

func cloneToolRequirement(tool curriculum.ToolRequirement) curriculum.ToolRequirement {
	tool.Platforms = append([]string(nil), tool.Platforms...)
	sort.Strings(tool.Platforms)
	tool.EvidenceRefs = sortedEvidenceCopy(tool.EvidenceRefs)
	if tool.IntroducedAt != nil {
		introduced := *tool.IntroducedAt
		tool.IntroducedAt = &introduced
	}
	return tool
}

func toolLevelSatisfies(actual, minimum curriculum.ToolRequirementLevel) bool {
	rank := func(level curriculum.ToolRequirementLevel) int {
		switch level {
		case curriculum.ToolRequired:
			return 3
		case curriculum.ToolRecommended:
			return 2
		case curriculum.ToolOptional:
			return 1
		default:
			return 0
		}
	}
	return rank(actual) >= rank(minimum)
}
