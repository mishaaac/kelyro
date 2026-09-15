package curriculum

import (
	"errors"
	"fmt"
	"time"
)

const CurriculumCompilerVersionV1 = "curriculum-compiler-v1"

const ReproducibilityMetadataSchemaVersionV1 = "curriculum-build-info/v1"

type CompilationInput struct {
	Goal          LearningGoalSpec
	SourceBundles []SourceBundleRef
	RequestedAt   Timestamp
}

func (input CompilationInput) Validate(policy SourceReferencePolicy) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	if err := input.Goal.Validate(); err != nil {
		return err
	}
	if err := validateSourceBundleRefs(input.SourceBundles, policy); err != nil {
		return err
	}
	if err := input.RequestedAt.Validate(); err != nil {
		return fmt.Errorf("compilation requested at: %w", err)
	}
	return nil
}

type CompilationConfig struct {
	CompilerVersion   string
	SourcePolicy      SourceReferencePolicy
	PackSchemaVersion string
}

func (config CompilationConfig) Validate() error {
	if err := requireText("compiler version", config.CompilerVersion); err != nil {
		return err
	}
	if err := requireText("pack schema version", config.PackSchemaVersion); err != nil {
		return err
	}
	return config.SourcePolicy.Validate()
}

type CompilationPassVersion struct {
	Name    string
	Version string
}

func (version CompilationPassVersion) Validate() error {
	if err := requireText("compilation pass name", version.Name); err != nil {
		return err
	}
	return requireText("compilation pass version", version.Version)
}

// ReproducibilityMetadata freezes the complete, content-addressed recipe used
// to produce one compiled curriculum. It contains identities and hashes only,
// never raw source bodies or learner state.
type ReproducibilityMetadata struct {
	SchemaVersion     string
	CompilerVersion   string
	Passes            []CompilationPassVersion
	SourceBundles     []SourceBundleRef
	CompilationConfig CompilationConfig
	PackSchemaVersion string
	InputHash         string
	OutputHash        string
	BuiltAt           Timestamp
}

func (metadata ReproducibilityMetadata) Validate() error {
	if metadata.SchemaVersion != ReproducibilityMetadataSchemaVersionV1 {
		return fmt.Errorf("unsupported reproducibility metadata schema %q", metadata.SchemaVersion)
	}
	if err := requireText("reproducibility compiler version", metadata.CompilerVersion); err != nil {
		return err
	}
	if len(metadata.Passes) == 0 {
		return errors.New("reproducibility metadata has no pass versions")
	}
	seen := make(map[string]struct{}, len(metadata.Passes))
	for _, pass := range metadata.Passes {
		if err := pass.Validate(); err != nil {
			return err
		}
		if _, exists := seen[pass.Name]; exists {
			return fmt.Errorf("duplicate reproducibility pass %q", pass.Name)
		}
		seen[pass.Name] = struct{}{}
	}
	if err := metadata.CompilationConfig.Validate(); err != nil {
		return err
	}
	if metadata.CompilerVersion != metadata.CompilationConfig.CompilerVersion {
		return errors.New("reproducibility compiler version does not match compilation config")
	}
	if err := requireText("reproducibility pack schema version", metadata.PackSchemaVersion); err != nil {
		return err
	}
	if metadata.PackSchemaVersion != metadata.CompilationConfig.PackSchemaVersion {
		return errors.New("reproducibility pack schema version does not match compilation config")
	}
	if err := validateSourceBundleRefs(metadata.SourceBundles, metadata.CompilationConfig.SourcePolicy); err != nil {
		return err
	}
	if !contentHashPattern.MatchString(metadata.InputHash) {
		return errors.New("reproducibility input hash is not canonical sha256")
	}
	if !contentHashPattern.MatchString(metadata.OutputHash) {
		return errors.New("reproducibility output hash is not canonical sha256")
	}
	if err := metadata.BuiltAt.Validate(); err != nil {
		return fmt.Errorf("reproducibility build time: %w", err)
	}
	return nil
}

type CompilationPass struct {
	Name       string
	Version    string
	InputHash  string
	OutputHash string
	Warnings   []string
	Errors     []string
	Duration   time.Duration
}

func (pass CompilationPass) Validate() error {
	for _, field := range []struct{ name, value string }{
		{name: "compilation pass name", value: pass.Name},
		{name: "compilation pass version", value: pass.Version},
		{name: "compilation pass input hash", value: pass.InputHash},
		{name: "compilation pass output hash", value: pass.OutputHash},
	} {
		if err := requireText(field.name, field.value); err != nil {
			return err
		}
	}
	if pass.Duration < 0 {
		return errors.New("compilation pass duration is negative")
	}
	if err := validateTexts("compilation warnings", pass.Warnings); err != nil {
		return err
	}
	return validateTexts("compilation errors", pass.Errors)
}

type CompilationResult struct {
	Curriculum  CurriculumDefinition
	Passes      []CompilationPass
	BuildInfo   *ReproducibilityMetadata
	Coverage    []CoverageResult
	Gaps        []Gap
	Warnings    []string
	Diagnostics *CompilationDiagnostics
}

type CompilationDiagnostics struct {
	Decomposition       GoalDecomposition
	Granularity         GranularityResult
	Graph               KnowledgeGraphCompilation
	Vocabulary          VocabularyGraphCompilation
	Hierarchy           CurriculumHierarchy
	Coverage            CoverageReport
	GapScan             GapScanReport
	DefinitionBeforeUse DefinitionBeforeUseAuditResult
	ZeroAssumption      ZeroAssumptionAuditResult
	Temporal            TemporalClassificationResult
	Guidance            GuidanceClassificationResult
	BeginnerSimulation  BeginnerSimulationResult
	ExpertCoverage      ExpertCoverageReviewResult
	Review              *CurriculumReviewResult
}

func (diagnostics CompilationDiagnostics) Validate(concepts []Concept) error {
	for _, validation := range []func() error{
		diagnostics.Decomposition.Validate,
		diagnostics.Granularity.Validate,
		diagnostics.Graph.Validate,
		diagnostics.Vocabulary.Validate,
		func() error { return diagnostics.Hierarchy.Validate(concepts) },
		diagnostics.Coverage.Validate,
		diagnostics.GapScan.Validate,
		diagnostics.DefinitionBeforeUse.Validate,
		diagnostics.ZeroAssumption.Validate,
		diagnostics.Temporal.Validate,
		diagnostics.Guidance.Validate,
		diagnostics.BeginnerSimulation.Validate,
		diagnostics.ExpertCoverage.Validate,
	} {
		if err := validation(); err != nil {
			return err
		}
	}
	if diagnostics.Review != nil {
		if err := diagnostics.Review.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (result CompilationResult) Validate() error {
	if err := result.Curriculum.Validate(); err != nil {
		return err
	}
	if len(result.Passes) == 0 {
		return errors.New("compilation result has no passes")
	}
	for _, pass := range result.Passes {
		if err := pass.Validate(); err != nil {
			return err
		}
	}
	if result.BuildInfo != nil {
		if err := result.BuildInfo.Validate(); err != nil {
			return err
		}
		if result.BuildInfo.CompilerVersion != result.Passes[0].Version {
			return errors.New("reproducibility compiler version does not match validation pass")
		}
		if result.BuildInfo.InputHash != result.Passes[0].InputHash {
			return errors.New("reproducibility input hash does not match validation pass")
		}
		if len(result.BuildInfo.SourceBundles) != len(result.Curriculum.SourceBundles) {
			return errors.New("reproducibility source bundles do not match curriculum")
		}
		for index := range result.Curriculum.SourceBundles {
			if result.BuildInfo.SourceBundles[index] != result.Curriculum.SourceBundles[index] {
				return fmt.Errorf("reproducibility source bundle %d does not match curriculum", index)
			}
		}
		if len(result.BuildInfo.Passes) != len(result.Passes) {
			return errors.New("reproducibility pass versions do not match compilation passes")
		}
		for index, pass := range result.Passes {
			version := result.BuildInfo.Passes[index]
			if version.Name != pass.Name || version.Version != pass.Version {
				return fmt.Errorf("reproducibility pass %d does not match compilation trace", index)
			}
		}
		if result.BuildInfo.OutputHash != result.Passes[len(result.Passes)-1].OutputHash {
			return errors.New("reproducibility output hash does not match final pass")
		}
	}
	for _, coverage := range result.Coverage {
		if err := coverage.Validate(); err != nil {
			return err
		}
	}
	for _, gap := range result.Gaps {
		if err := gap.Validate(); err != nil {
			return err
		}
	}
	if result.Diagnostics != nil {
		if err := result.Diagnostics.Validate(result.Curriculum.Concepts); err != nil {
			return err
		}
	}
	return validateTexts("compilation result warnings", result.Warnings)
}

func validateSourceBundleRefs(references []SourceBundleRef, policy SourceReferencePolicy) error {
	if policy == SourceReferencesRequired && len(references) == 0 {
		return errors.New("source bundle references are required")
	}
	seen := make(map[ID]struct{}, len(references))
	for _, reference := range references {
		if err := reference.Validate(); err != nil {
			return err
		}
		if _, exists := seen[reference.ID]; exists {
			return fmt.Errorf("duplicate source bundle reference %q", reference.ID)
		}
		seen[reference.ID] = struct{}{}
	}
	return nil
}
