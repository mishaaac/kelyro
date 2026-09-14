package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type PackDependencyResolverV1 struct{}

func NewPackDependencyResolverV1() PackDependencyResolverV1 { return PackDependencyResolverV1{} }

type dependencyConstraint struct {
	operator string
	version  semverParts
}

type dependencyDemand struct {
	requiredBy curriculum.ID
	raw        string
	clauses    []dependencyConstraint
}

type dependencyState struct {
	selected    map[curriculum.ID]curriculum.PackManifest
	constraints map[curriculum.ID][]dependencyDemand
	expanded    map[curriculum.ID]bool
}

func (PackDependencyResolverV1) Resolve(ctx context.Context, request PackDependencyResolutionRequest) (curriculum.PackDependencyResolution, error) {
	const operation = "resolve pack dependencies"
	if err := ctx.Err(); err != nil {
		return curriculum.PackDependencyResolution{}, ExternalError(operation, err)
	}
	if err := request.Root.Validate(); err != nil {
		return curriculum.PackDependencyResolution{}, Invalid(operation, err)
	}
	root := curriculum.PackReference{PackID: request.Root.ID, Version: request.Root.Version}
	catalog := make(map[curriculum.ID][]curriculum.PackManifest)
	seenVersions := make(map[string]struct{}, len(request.Available)+1)
	rootKey := request.Root.ID.String() + "\x00" + request.Root.Version.String()
	seenVersions[rootKey] = struct{}{}
	if err := validateManifestConstraints(request.Root); err != nil {
		return curriculum.PackDependencyResolution{}, Invalid(operation, err)
	}
	for _, manifest := range request.Available {
		if err := manifest.Validate(); err != nil {
			return curriculum.PackDependencyResolution{}, Invalid(operation, err)
		}
		if err := validateManifestConstraints(manifest); err != nil {
			return curriculum.PackDependencyResolution{}, Invalid(operation, err)
		}
		key := manifest.ID.String() + "\x00" + manifest.Version.String()
		if key == rootKey {
			continue
		}
		if _, exists := seenVersions[key]; exists {
			return curriculum.PackDependencyResolution{}, Invalid(operation, fmt.Errorf("duplicate available pack version %s@%s", manifest.ID, manifest.Version.String()))
		}
		seenVersions[key] = struct{}{}
		catalog[manifest.ID] = append(catalog[manifest.ID], manifest)
	}
	for id := range catalog {
		sort.Slice(catalog[id], func(i, j int) bool {
			compared := compareSemver(parseSemver(catalog[id][i].Version.String()), parseSemver(catalog[id][j].Version.String()))
			if compared != 0 {
				return compared > 0
			}
			return catalog[id][i].Version.String() < catalog[id][j].Version.String()
		})
	}

	initial := dependencyState{
		selected:    map[curriculum.ID]curriculum.PackManifest{request.Root.ID: request.Root},
		constraints: make(map[curriculum.ID][]dependencyDemand),
		expanded:    make(map[curriculum.ID]bool),
	}
	state, issue, ok, err := solveDependencies(ctx, initial, catalog)
	if err != nil {
		return curriculum.PackDependencyResolution{}, ExternalError(operation, err)
	}
	if !ok {
		result := curriculum.PackDependencyResolution{Root: root, Status: curriculum.PackDependenciesFailed, Issues: []curriculum.PackDependencyIssue{issue}, AlgorithmVersion: curriculum.PackDependencyResolverVersionV1}
		if err := result.Validate(); err != nil {
			return curriculum.PackDependencyResolution{}, Invalid(operation, err)
		}
		return result, nil
	}

	result := curriculum.PackDependencyResolution{Root: root, Status: curriculum.PackDependenciesResolved, AlgorithmVersion: curriculum.PackDependencyResolverVersionV1}
	for _, manifest := range state.selected {
		result.Selected = append(result.Selected, cloneDependencyManifest(manifest))
	}
	sort.Slice(result.Selected, func(i, j int) bool { return result.Selected[i].ID.String() < result.Selected[j].ID.String() })
	result.InstallOrder = dependencyInstallOrder(request.Root.ID, state.selected)
	if err := result.Validate(); err != nil {
		return curriculum.PackDependencyResolution{}, Invalid(operation, err)
	}
	return result, nil
}

func solveDependencies(ctx context.Context, state dependencyState, catalog map[curriculum.ID][]curriculum.PackManifest) (dependencyState, curriculum.PackDependencyIssue, bool, error) {
	if err := ctx.Err(); err != nil {
		return dependencyState{}, curriculum.PackDependencyIssue{}, false, err
	}
	selectedIDs := sortedManifestIDs(state.selected)
	for _, id := range selectedIDs {
		if demands := state.constraints[id]; len(demands) > 0 && !manifestSatisfies(state.selected[id], demands) {
			return dependencyState{}, incompatibleDependencyIssue(id, demands, catalog[id]), false, nil
		}
	}
	for _, id := range selectedIDs {
		if state.expanded[id] {
			continue
		}
		next := cloneDependencyState(state)
		next.expanded[id] = true
		dependencies := append([]curriculum.PackDependency(nil), next.selected[id].Dependencies...)
		sort.Slice(dependencies, func(i, j int) bool { return dependencies[i].PackID.String() < dependencies[j].PackID.String() })
		for _, dependency := range dependencies {
			clauses, _ := parseDependencyConstraint(dependency.Constraint)
			next.constraints[dependency.PackID] = append(next.constraints[dependency.PackID], dependencyDemand{requiredBy: id, raw: dependency.Constraint, clauses: clauses})
		}
		return solveDependencies(ctx, next, catalog)
	}

	needed := make([]curriculum.ID, 0)
	for id := range state.constraints {
		if _, exists := state.selected[id]; !exists {
			needed = append(needed, id)
		}
	}
	sort.Slice(needed, func(i, j int) bool { return needed[i].String() < needed[j].String() })
	if len(needed) == 0 {
		if issue, cyclic := dependencyCycle(state.selected); cyclic {
			return dependencyState{}, issue, false, nil
		}
		return state, curriculum.PackDependencyIssue{}, true, nil
	}
	id := needed[0]
	demands := state.constraints[id]
	available := catalog[id]
	if len(available) == 0 {
		return dependencyState{}, missingDependencyIssue(id, demands), false, nil
	}
	var firstIssue curriculum.PackDependencyIssue
	compatible := false
	for _, candidate := range available {
		if !manifestSatisfies(candidate, demands) {
			continue
		}
		compatible = true
		next := cloneDependencyState(state)
		next.selected[id] = candidate
		resolved, issue, ok, err := solveDependencies(ctx, next, catalog)
		if err != nil || ok {
			return resolved, issue, ok, err
		}
		if firstIssue.Kind == "" {
			firstIssue = issue
		}
	}
	if !compatible {
		return dependencyState{}, incompatibleDependencyIssue(id, demands, available), false, nil
	}
	return dependencyState{}, firstIssue, false, nil
}

func validateManifestConstraints(manifest curriculum.PackManifest) error {
	for _, dependency := range manifest.Dependencies {
		if _, err := parseDependencyConstraint(dependency.Constraint); err != nil {
			return fmt.Errorf("pack %q dependency %q: %w", manifest.ID, dependency.PackID, err)
		}
	}
	return nil
}

func parseDependencyConstraint(value string) ([]dependencyConstraint, error) {
	if value == "" || value != strings.TrimSpace(value) {
		return nil, fmt.Errorf("version constraint is empty or has surrounding whitespace")
	}
	var result []dependencyConstraint
	for _, raw := range strings.Fields(value) {
		operator, versionText := "=", raw
		for _, prefix := range []string{"<=", ">=", "<", ">", "="} {
			if strings.HasPrefix(raw, prefix) {
				operator, versionText = prefix, strings.TrimPrefix(raw, prefix)
				break
			}
		}
		version, err := curriculum.NewPackVersion(versionText)
		if err != nil {
			return nil, fmt.Errorf("invalid version constraint %q", value)
		}
		result = append(result, dependencyConstraint{operator: operator, version: parseSemver(version.String())})
	}
	return result, nil
}

func manifestSatisfies(manifest curriculum.PackManifest, demands []dependencyDemand) bool {
	version := parseSemver(manifest.Version.String())
	for _, demand := range demands {
		for _, clause := range demand.clauses {
			compared := compareSemver(version, clause.version)
			satisfied := compared == 0
			switch clause.operator {
			case "<":
				satisfied = compared < 0
			case "<=":
				satisfied = compared <= 0
			case ">":
				satisfied = compared > 0
			case ">=":
				satisfied = compared >= 0
			}
			if !satisfied {
				return false
			}
		}
	}
	return true
}

func missingDependencyIssue(id curriculum.ID, demands []dependencyDemand) curriculum.PackDependencyIssue {
	demand := sortedDemands(demands)[0]
	return curriculum.PackDependencyIssue{Kind: curriculum.PackDependencyMissing, PackID: id, RequiredBy: demand.requiredBy, Constraint: demand.raw, Path: []curriculum.ID{demand.requiredBy, id}, Reason: "no version of the required pack is available"}
}

func incompatibleDependencyIssue(id curriculum.ID, demands []dependencyDemand, available []curriculum.PackManifest) curriculum.PackDependencyIssue {
	demand := sortedDemands(demands)[0]
	issue := curriculum.PackDependencyIssue{Kind: curriculum.PackDependencyIncompatible, PackID: id, RequiredBy: demand.requiredBy, Constraint: joinedDemandConstraints(demands), Path: []curriculum.ID{demand.requiredBy, id}, Reason: "available versions do not satisfy all required constraints"}
	for _, manifest := range available {
		issue.AvailableVersions = append(issue.AvailableVersions, manifest.Version)
	}
	return issue
}

func sortedDemands(values []dependencyDemand) []dependencyDemand {
	result := append([]dependencyDemand(nil), values...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].requiredBy != result[j].requiredBy {
			return result[i].requiredBy.String() < result[j].requiredBy.String()
		}
		return result[i].raw < result[j].raw
	})
	return result
}

func joinedDemandConstraints(values []dependencyDemand) string {
	values = sortedDemands(values)
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = value.requiredBy.String() + ":" + value.raw
	}
	return strings.Join(parts, "; ")
}

func dependencyCycle(selected map[curriculum.ID]curriculum.PackManifest) (curriculum.PackDependencyIssue, bool) {
	state := make(map[curriculum.ID]int, len(selected))
	var stack []curriculum.ID
	var visit func(curriculum.ID) (curriculum.PackDependencyIssue, bool)
	visit = func(id curriculum.ID) (curriculum.PackDependencyIssue, bool) {
		state[id] = 1
		stack = append(stack, id)
		dependencies := append([]curriculum.PackDependency(nil), selected[id].Dependencies...)
		sort.Slice(dependencies, func(i, j int) bool { return dependencies[i].PackID.String() < dependencies[j].PackID.String() })
		for _, dependency := range dependencies {
			if state[dependency.PackID] == 1 {
				start := 0
				for stack[start] != dependency.PackID {
					start++
				}
				path := append([]curriculum.ID(nil), stack[start:]...)
				path = append(path, dependency.PackID)
				return curriculum.PackDependencyIssue{Kind: curriculum.PackDependencyCycle, PackID: dependency.PackID, RequiredBy: id, Constraint: dependency.Constraint, Path: path, Reason: "pack dependency graph contains a cycle"}, true
			}
			if state[dependency.PackID] == 0 {
				if issue, found := visit(dependency.PackID); found {
					return issue, true
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = 2
		return curriculum.PackDependencyIssue{}, false
	}
	for _, id := range sortedManifestIDs(selected) {
		if state[id] == 0 {
			if issue, found := visit(id); found {
				return issue, true
			}
		}
	}
	return curriculum.PackDependencyIssue{}, false
}

func dependencyInstallOrder(root curriculum.ID, selected map[curriculum.ID]curriculum.PackManifest) []curriculum.PackReference {
	visited := make(map[curriculum.ID]bool, len(selected))
	var result []curriculum.PackReference
	var visit func(curriculum.ID)
	visit = func(id curriculum.ID) {
		if visited[id] {
			return
		}
		visited[id] = true
		dependencies := append([]curriculum.PackDependency(nil), selected[id].Dependencies...)
		sort.Slice(dependencies, func(i, j int) bool { return dependencies[i].PackID.String() < dependencies[j].PackID.String() })
		for _, dependency := range dependencies {
			visit(dependency.PackID)
		}
		result = append(result, curriculum.PackReference{PackID: id, Version: selected[id].Version})
	}
	visit(root)
	return result
}

func sortedManifestIDs(values map[curriculum.ID]curriculum.PackManifest) []curriculum.ID {
	result := make([]curriculum.ID, 0, len(values))
	for id := range values {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func cloneDependencyState(value dependencyState) dependencyState {
	result := dependencyState{selected: make(map[curriculum.ID]curriculum.PackManifest, len(value.selected)), constraints: make(map[curriculum.ID][]dependencyDemand, len(value.constraints)), expanded: make(map[curriculum.ID]bool, len(value.expanded))}
	for id, manifest := range value.selected {
		result.selected[id] = manifest
	}
	for id, demands := range value.constraints {
		result.constraints[id] = append([]dependencyDemand(nil), demands...)
	}
	for id, expanded := range value.expanded {
		result.expanded[id] = expanded
	}
	return result
}

func cloneDependencyManifest(value curriculum.PackManifest) curriculum.PackManifest {
	value.Authors = append([]string(nil), value.Authors...)
	value.Maintainers = append([]string(nil), value.Maintainers...)
	value.Dependencies = append([]curriculum.PackDependency(nil), value.Dependencies...)
	return value
}

var _ PackDependencyResolverService = PackDependencyResolverV1{}
