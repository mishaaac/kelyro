package curriculum

import "fmt"

const PackDependencyResolverVersionV1 = "pack-dependency-resolver-v1"

type PackReference struct {
	PackID  ID
	Version PackVersion
}

func (reference PackReference) Validate() error {
	if err := reference.PackID.Validate(); err != nil {
		return fmt.Errorf("pack reference: %w", err)
	}
	return reference.Version.Validate()
}

type PackDependencyResolutionStatus string

const (
	PackDependenciesResolved PackDependencyResolutionStatus = "resolved"
	PackDependenciesFailed   PackDependencyResolutionStatus = "failed"
)

func (status PackDependencyResolutionStatus) Validate() error {
	switch status {
	case PackDependenciesResolved, PackDependenciesFailed:
		return nil
	default:
		return fmt.Errorf("invalid pack dependency resolution status %q", status)
	}
}

type PackDependencyIssueKind string

const (
	PackDependencyMissing      PackDependencyIssueKind = "missing"
	PackDependencyIncompatible PackDependencyIssueKind = "incompatible"
	PackDependencyCycle        PackDependencyIssueKind = "cycle"
)

func (kind PackDependencyIssueKind) Validate() error {
	switch kind {
	case PackDependencyMissing, PackDependencyIncompatible, PackDependencyCycle:
		return nil
	default:
		return fmt.Errorf("invalid pack dependency issue kind %q", kind)
	}
}

type PackDependencyIssue struct {
	Kind              PackDependencyIssueKind
	PackID            ID
	RequiredBy        ID
	Constraint        string
	Path              []ID
	AvailableVersions []PackVersion
	Reason            string
}

func (issue PackDependencyIssue) Validate() error {
	if err := issue.Kind.Validate(); err != nil {
		return err
	}
	if err := issue.PackID.Validate(); err != nil {
		return fmt.Errorf("dependency issue pack: %w", err)
	}
	if err := issue.RequiredBy.Validate(); err != nil {
		return fmt.Errorf("dependency issue requiring pack: %w", err)
	}
	if err := requireText("dependency issue constraint", issue.Constraint); err != nil {
		return err
	}
	if len(issue.Path) < 2 {
		return fmt.Errorf("dependency issue path is incomplete")
	}
	if issue.Kind == PackDependencyCycle {
		if len(issue.Path) < 3 || issue.Path[0] != issue.Path[len(issue.Path)-1] {
			return fmt.Errorf("dependency cycle path must repeat its starting pack")
		}
		seen := make(map[ID]struct{}, len(issue.Path)-1)
		for _, id := range issue.Path[:len(issue.Path)-1] {
			if err := id.Validate(); err != nil {
				return err
			}
			if _, exists := seen[id]; exists {
				return fmt.Errorf("dependency cycle path repeats an intermediate pack %q", id)
			}
			seen[id] = struct{}{}
		}
	} else if err := validateIDs("dependency issue path", issue.Path); err != nil {
		return err
	}
	for _, version := range issue.AvailableVersions {
		if err := version.Validate(); err != nil {
			return err
		}
	}
	return requireText("dependency issue reason", issue.Reason)
}

type PackDependencyResolution struct {
	Root             PackReference
	Status           PackDependencyResolutionStatus
	Selected         []PackManifest
	InstallOrder     []PackReference
	Issues           []PackDependencyIssue
	AlgorithmVersion string
}

func (resolution PackDependencyResolution) Validate() error {
	if err := resolution.Root.Validate(); err != nil {
		return err
	}
	if err := resolution.Status.Validate(); err != nil {
		return err
	}
	if resolution.AlgorithmVersion != PackDependencyResolverVersionV1 {
		return fmt.Errorf("unsupported pack dependency resolver version %q", resolution.AlgorithmVersion)
	}
	for _, issue := range resolution.Issues {
		if err := issue.Validate(); err != nil {
			return err
		}
	}
	if resolution.Status == PackDependenciesFailed {
		if len(resolution.Issues) == 0 || len(resolution.Selected) != 0 || len(resolution.InstallOrder) != 0 {
			return fmt.Errorf("failed dependency resolution must contain only issues")
		}
		return nil
	}
	if len(resolution.Issues) != 0 || len(resolution.Selected) == 0 || len(resolution.InstallOrder) != len(resolution.Selected) {
		return fmt.Errorf("resolved dependencies must contain a complete issue-free selection")
	}
	selected := make(map[ID]PackVersion, len(resolution.Selected))
	for _, manifest := range resolution.Selected {
		if err := manifest.Validate(); err != nil {
			return err
		}
		if _, exists := selected[manifest.ID]; exists {
			return fmt.Errorf("dependency resolution selected pack %q more than once", manifest.ID)
		}
		selected[manifest.ID] = manifest.Version
	}
	if selected[resolution.Root.PackID] != resolution.Root.Version {
		return fmt.Errorf("dependency resolution does not contain its root")
	}
	positions := make(map[ID]int, len(resolution.InstallOrder))
	for index, reference := range resolution.InstallOrder {
		if err := reference.Validate(); err != nil {
			return err
		}
		version, exists := selected[reference.PackID]
		if !exists || version != reference.Version {
			return fmt.Errorf("install order references an unselected pack")
		}
		if _, exists := positions[reference.PackID]; exists {
			return fmt.Errorf("install order repeats pack %q", reference.PackID)
		}
		positions[reference.PackID] = index
	}
	for _, manifest := range resolution.Selected {
		for _, dependency := range manifest.Dependencies {
			dependencyPosition, exists := positions[dependency.PackID]
			if !exists {
				return fmt.Errorf("selected pack %q has unresolved dependency %q", manifest.ID, dependency.PackID)
			}
			if dependencyPosition >= positions[manifest.ID] {
				return fmt.Errorf("install order places dependency %q after %q", dependency.PackID, manifest.ID)
			}
		}
	}
	if resolution.InstallOrder[len(resolution.InstallOrder)-1] != resolution.Root {
		return fmt.Errorf("dependency resolution root must be installed last")
	}
	return nil
}
