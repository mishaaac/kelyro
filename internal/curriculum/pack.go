package curriculum

import "fmt"

type PackDependency struct {
	PackID     ID
	Constraint string
}

func (dependency PackDependency) Validate() error {
	if err := dependency.PackID.Validate(); err != nil {
		return fmt.Errorf("pack dependency: %w", err)
	}
	return requireText("pack dependency constraint", dependency.Constraint)
}

type PackManifest struct {
	ID            ID
	Name          string
	Description   string
	Version       PackVersion
	SchemaVersion string
	Status        PackStatus
	CurriculumID  CurriculumID
	CreatedAt     Timestamp
	Dependencies  []PackDependency
}

func (manifest PackManifest) Validate() error {
	if err := manifest.ID.Validate(); err != nil {
		return fmt.Errorf("pack manifest: %w", err)
	}
	if err := requireText("pack name", manifest.Name); err != nil {
		return err
	}
	if err := requireText("pack description", manifest.Description); err != nil {
		return err
	}
	if err := manifest.Version.Validate(); err != nil {
		return err
	}
	if err := requireText("pack schema version", manifest.SchemaVersion); err != nil {
		return err
	}
	if err := manifest.Status.Validate(); err != nil {
		return err
	}
	if err := manifest.CurriculumID.Validate(); err != nil {
		return err
	}
	if err := manifest.CreatedAt.Validate(); err != nil {
		return fmt.Errorf("pack created at: %w", err)
	}
	seen := make(map[ID]struct{}, len(manifest.Dependencies))
	for _, dependency := range manifest.Dependencies {
		if err := dependency.Validate(); err != nil {
			return err
		}
		if dependency.PackID == manifest.ID {
			return fmt.Errorf("pack %q cannot depend on itself", manifest.ID)
		}
		if _, exists := seen[dependency.PackID]; exists {
			return fmt.Errorf("duplicate pack dependency %q", dependency.PackID)
		}
		seen[dependency.PackID] = struct{}{}
	}
	return nil
}

type LearningPack struct {
	Manifest    PackManifest
	Curriculum  CurriculumDefinition
	Environment *EnvironmentPack
}

func (pack LearningPack) Validate() error {
	if err := pack.Manifest.Validate(); err != nil {
		return err
	}
	if err := pack.Curriculum.Validate(); err != nil {
		return err
	}
	if pack.Manifest.CurriculumID != pack.Curriculum.ID {
		return fmt.Errorf("pack manifest curriculum does not match definition")
	}
	if pack.Environment != nil {
		if err := pack.Environment.Validate(); err != nil {
			return err
		}
		concepts := make(map[ConceptID]struct{}, len(pack.Curriculum.Concepts))
		for _, concept := range pack.Curriculum.Concepts {
			concepts[concept.ID] = struct{}{}
		}
		bundles := make(map[ID]struct{}, len(pack.Curriculum.SourceBundles))
		for _, bundle := range pack.Curriculum.SourceBundles {
			bundles[bundle.ID] = struct{}{}
		}
		for _, tool := range pack.Environment.Tools {
			if tool.IntroducedAt != nil {
				if _, exists := concepts[*tool.IntroducedAt]; !exists {
					return fmt.Errorf("tool %q references missing introduction concept %q", tool.ID, tool.IntroducedAt)
				}
			}
			for _, evidence := range tool.EvidenceRefs {
				if _, exists := bundles[evidence.BundleID]; !exists {
					return fmt.Errorf("tool %q evidence references undeclared source bundle %q", tool.ID, evidence.BundleID)
				}
			}
		}
	}
	return nil
}
