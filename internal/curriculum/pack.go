package curriculum

import "fmt"

const LearningPackSchemaVersionV1 = "learning-pack/v1"

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
	ID                   ID
	Name                 string
	Description          string
	Version              PackVersion
	SchemaVersion        string
	Domain               string
	Target               string
	Authors              []string
	Maintainers          []string
	License              string
	CreatedAt            Timestamp
	MinimumKelyroVersion PackVersion
	Dependencies         []PackDependency
	EnvironmentEntry     string
	CurriculumEntry      string
	SourceEvidenceEntry  string
	BuildInfoEntry       string
	Status               PackStatus
	CurriculumID         CurriculumID
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
	for _, field := range []struct{ name, value string }{
		{name: "pack domain", value: manifest.Domain},
		{name: "pack target", value: manifest.Target},
		{name: "pack license", value: manifest.License},
		{name: "pack curriculum entry", value: manifest.CurriculumEntry},
		{name: "pack source evidence entry", value: manifest.SourceEvidenceEntry},
	} {
		if err := requireText(field.name, field.value); err != nil {
			return err
		}
	}
	if err := validateTexts("pack authors", manifest.Authors); err != nil {
		return err
	}
	if len(manifest.Authors) == 0 {
		return fmt.Errorf("pack authors are empty")
	}
	if err := validateTexts("pack maintainers", manifest.Maintainers); err != nil {
		return err
	}
	if len(manifest.Maintainers) == 0 {
		return fmt.Errorf("pack maintainers are empty")
	}
	if err := manifest.MinimumKelyroVersion.Validate(); err != nil {
		return fmt.Errorf("minimum Kelyro version: %w", err)
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
	BuildInfo   *ReproducibilityMetadata
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
	if pack.BuildInfo != nil {
		if err := pack.BuildInfo.Validate(); err != nil {
			return fmt.Errorf("pack build info: %w", err)
		}
		if pack.BuildInfo.PackSchemaVersion != pack.Manifest.SchemaVersion {
			return fmt.Errorf("pack build info schema does not match manifest")
		}
		if len(pack.BuildInfo.SourceBundles) != len(pack.Curriculum.SourceBundles) {
			return fmt.Errorf("pack build info source bundles do not match curriculum")
		}
		for index := range pack.Curriculum.SourceBundles {
			if pack.BuildInfo.SourceBundles[index] != pack.Curriculum.SourceBundles[index] {
				return fmt.Errorf("pack build info source bundle %d does not match curriculum", index)
			}
		}
	}
	if pack.Environment != nil {
		if err := pack.Environment.ValidatePortableV1(); err != nil {
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
			if _, exists := concepts[*tool.WhenNeeded]; !exists {
				return fmt.Errorf("tool %q references missing when-needed concept %q", tool.ID, tool.WhenNeeded)
			}
			for _, evidence := range tool.EvidenceRefs {
				if _, exists := bundles[evidence.BundleID]; !exists {
					return fmt.Errorf("tool %q evidence references undeclared source bundle %q", tool.ID, evidence.BundleID)
				}
			}
		}
		for _, guidance := range pack.Environment.InstallGuidance {
			for _, evidence := range guidance.EvidenceRefs {
				if _, exists := bundles[evidence.BundleID]; !exists {
					return fmt.Errorf("install guidance %q evidence references undeclared source bundle %q", guidance.ID, evidence.BundleID)
				}
			}
		}
	}
	return nil
}
