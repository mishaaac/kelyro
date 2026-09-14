package curriculum

import (
	"errors"
	"fmt"
	"time"
)

const CurriculumCompilerVersionV1 = "curriculum-compiler-v1"

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
	CompilerVersion string
	SourcePolicy    SourceReferencePolicy
}

func (config CompilationConfig) Validate() error {
	if err := requireText("compiler version", config.CompilerVersion); err != nil {
		return err
	}
	return config.SourcePolicy.Validate()
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
