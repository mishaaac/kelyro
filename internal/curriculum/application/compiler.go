package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type CurriculumCompilerV1 struct{}

func NewCurriculumCompilerV1() CurriculumCompilerV1 { return CurriculumCompilerV1{} }

func (CurriculumCompilerV1) Compile(ctx context.Context, request CurriculumCompileRequest) (curriculum.CompilationResult, error) {
	const operation = "compile curriculum"
	result := curriculum.CompilationResult{}
	appendPass := func(pass curriculum.CompilationPass) { result.Passes = append(result.Passes, pass) }
	fail := func(pass curriculum.CompilationPass, err error) (curriculum.CompilationResult, error) {
		appendPass(pass)
		return result, err
	}

	_, pass, err := runCompilerPass(ctx, "validate-input", curriculum.CurriculumCompilerVersionV1, request, func(ctx context.Context) (bool, error) {
		if request.Config.CompilerVersion != curriculum.CurriculumCompilerVersionV1 {
			return false, Invalid(operation, fmt.Errorf("unsupported compiler version %q", request.Config.CompilerVersion))
		}
		if err := request.Config.Validate(); err != nil {
			return false, Invalid(operation, err)
		}
		if err := request.Input.Validate(request.Config.SourcePolicy); err != nil {
			return false, Invalid(operation, err)
		}
		if err := validateBuildMetadata(request.Metadata); err != nil {
			return false, Invalid(operation, err)
		}
		if err := validateCompilerEvidence(request.Input.SourceBundles, request.EvidenceSets); err != nil {
			return false, Invalid(operation, err)
		}
		return true, nil
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	decomposition, pass, err := runCompilerPass(ctx, "goal-decomposition", curriculum.GoalDecomposerVersionV1, struct {
		Goal     curriculum.LearningGoalSpec
		Profile  curriculum.DomainProfile
		Evidence []curriculum.CurriculumEvidenceSet
	}{request.Input.Goal, request.DomainProfile, request.EvidenceSets}, func(ctx context.Context) (curriculum.GoalDecomposition, error) {
		return NewGoalDecomposerV1().Decompose(ctx, GoalDecompositionRequest{Goal: request.Input.Goal, EvidenceSets: request.EvidenceSets, Profile: request.DomainProfile})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	matrix, pass, err := runCompilerPass(ctx, "competency-matrix", curriculum.CompetencyMatrixVersionV1, struct {
		Decomposition curriculum.GoalDecomposition
		Competencies  []curriculum.Competency
	}{decomposition, request.Competencies}, func(ctx context.Context) (curriculum.CompetencyMatrix, error) {
		return NewCompetencyMatrixBuilderV1().Build(ctx, CompetencyMatrixRequest{Decomposition: decomposition, Competencies: request.Competencies, EvidenceSets: request.EvidenceSets})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	candidates, pass, err := runCompilerPass(ctx, "concept-candidates", curriculum.ConceptCandidateExtractorVersionV1, request.EvidenceSets, func(ctx context.Context) (curriculum.ConceptCandidateSet, error) {
		return NewConceptCandidateExtractorV1().Extract(ctx, ConceptCandidateExtractionRequest{EvidenceSets: request.EvidenceSets})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	concepts, pass, err := runCompilerPass(ctx, "atomize", curriculum.AtomizerVersionV1, struct {
		Candidates curriculum.ConceptCandidateSet
		Plans      []AtomizationPlan
	}{candidates, request.AtomizationPlans}, func(ctx context.Context) ([]curriculum.Concept, error) {
		plans := make(map[curriculum.ID]AtomizationPlan, len(request.AtomizationPlans))
		for _, plan := range request.AtomizationPlans {
			if err := plan.CandidateID.Validate(); err != nil {
				return nil, Invalid(operation, err)
			}
			if _, exists := plans[plan.CandidateID]; exists {
				return nil, Invalid(operation, fmt.Errorf("duplicate atomization plan %q", plan.CandidateID))
			}
			plans[plan.CandidateID] = plan
		}
		atomizer := NewConceptAtomizerV1(curriculum.NewAtomicConceptPolicyV1())
		var values []curriculum.Concept
		for _, candidate := range candidates.Candidates {
			plan, exists := plans[candidate.ID]
			if !exists {
				return nil, Invalid(operation, fmt.Errorf("candidate %q has no atomization plan", candidate.ID))
			}
			set, err := atomizer.Atomize(ctx, ConceptAtomizationRequest{Candidate: candidate, EvidenceSets: request.EvidenceSets, CandidateCriteria: plan.Criteria, DomainHints: plan.Hints})
			if err != nil {
				return nil, err
			}
			values = append(values, set.Concepts...)
			delete(plans, candidate.ID)
		}
		if len(plans) != 0 {
			return nil, Invalid(operation, fmt.Errorf("atomization plan references an unknown candidate"))
		}
		sort.Slice(values, func(i, j int) bool { return values[i].ID.String() < values[j].ID.String() })
		return values, nil
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	granularity, pass, err := runCompilerPass(ctx, "granularity", curriculum.GranularityGuardVersionV1, concepts, func(ctx context.Context) (curriculum.GranularityResult, error) {
		return NewGranularityGuardV1(curriculum.NewAtomicConceptPolicyV1()).Review(ctx, GranularityReviewRequest{Concepts: concepts})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	extraction, pass, err := runCompilerPass(ctx, "prerequisite-extraction", curriculum.PrerequisiteExtractorVersionV1, struct {
		Concepts  []curriculum.Concept
		Semantics []curriculum.ConceptPrerequisiteSemantic
	}{concepts, request.PrerequisiteSemantics}, func(ctx context.Context) (curriculum.PrerequisiteExtraction, error) {
		return NewPrerequisiteExtractorV1().Extract(ctx, PrerequisiteExtractionRequest{Concepts: concepts, EvidenceSets: request.EvidenceSets, Semantics: request.PrerequisiteSemantics})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	expansion, pass, err := runCompilerPass(ctx, "prerequisite-expansion", curriculum.PrerequisiteExpansionVersionV1, struct {
		Concepts  []curriculum.Concept
		Edges     []curriculum.Prerequisite
		Available []curriculum.Concept
	}{concepts, extraction.Edges(), request.AvailableConcepts}, func(ctx context.Context) (curriculum.PrerequisiteExpansionResult, error) {
		return NewPrerequisiteExpansionV1().Expand(ctx, PrerequisiteExpansionRequest{Concepts: concepts, Prerequisites: extraction.Edges(), Semantics: request.PrerequisiteSemantics, AvailableConcepts: request.AvailableConcepts, EvidenceSets: request.EvidenceSets})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)
	concepts = append(concepts, expansion.AddedConcepts...)
	sort.Slice(concepts, func(i, j int) bool { return concepts[i].ID.String() < concepts[j].ID.String() })

	vocabulary, pass, err := runCompilerPass(ctx, "vocabulary-graph", curriculum.VocabularyGraphBuilderVersionV1, struct {
		Concepts    []curriculum.Concept
		Definitions []curriculum.VocabularyDefinition
		Uses        []curriculum.VocabularyUse
		Baseline    []curriculum.DomainVocabularyBaselineTerm
	}{concepts, request.VocabularyDefinitions, request.VocabularyUses, request.VocabularyBaseline}, func(ctx context.Context) (curriculum.VocabularyGraphCompilation, error) {
		return NewVocabularyGraphBuilderV1().Build(ctx, VocabularyGraphRequest{Concepts: concepts, Definitions: request.VocabularyDefinitions, Uses: request.VocabularyUses, DomainBaseline: request.VocabularyBaseline})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	graph, pass, err := runCompilerPass(ctx, "knowledge-graph", curriculum.KnowledgeGraphCompilerVersionV1, struct {
		Concepts []curriculum.Concept
		Edges    []curriculum.Prerequisite
	}{concepts, expansion.ExpandedPrerequisites}, func(ctx context.Context) (curriculum.KnowledgeGraphCompilation, error) {
		return NewKnowledgeGraphCompilerV1().Compile(ctx, KnowledgeGraphCompilationRequest{Concepts: concepts, Prerequisites: expansion.ExpandedPrerequisites})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	hierarchy, pass, err := runCompilerPass(ctx, "hierarchy", curriculum.CurriculumHierarchyBuilderVersionV1, struct {
		Matrix  curriculum.CompetencyMatrix
		Graph   curriculum.KnowledgeGraphCompilation
		Context []curriculum.PracticeContextAssignment
	}{matrix, graph, request.PracticeContext}, func(ctx context.Context) (curriculum.CurriculumHierarchy, error) {
		return NewCurriculumHierarchyBuilderV1().Build(ctx, CurriculumHierarchyBuildRequest{Competencies: matrix, Concepts: concepts, Graph: graph, PracticeContext: request.PracticeContext})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	coverage, pass, err := runCompilerPass(ctx, "coverage", curriculum.CoverageEngineVersionV1, struct {
		Requirements []curriculum.CoverageRequirement
		Supports     []curriculum.CoverageSupport
	}{request.CoverageRequirements, request.CoverageSupports}, func(ctx context.Context) (curriculum.CoverageReport, error) {
		return NewCoverageEngineV1().Analyze(ctx, CoverageAnalysisRequest{CurriculumID: request.Metadata.ID, Goal: request.Input.Goal, Competencies: matrix, Concepts: concepts, Requirements: request.CoverageRequirements, Supports: request.CoverageSupports, EvidenceSets: request.EvidenceSets})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	definitionAudit, pass, err := runCompilerPass(ctx, "definition-before-use", curriculum.DefinitionBeforeUseAuditVersionV1, struct {
		Graph      curriculum.KnowledgeGraphCompilation
		Vocabulary curriculum.VocabularyGraphCompilation
		Topics     []curriculum.TopicSpec
	}{graph, vocabulary, hierarchy.Topics}, func(ctx context.Context) (curriculum.DefinitionBeforeUseAuditResult, error) {
		return NewDefinitionBeforeUseAuditV1().Audit(ctx, DefinitionBeforeUseAuditRequest{Graph: graph, Vocabulary: vocabulary, Topics: hierarchy.Topics})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	zeroAudit, pass, err := runCompilerPass(ctx, "zero-assumption", curriculum.ZeroAssumptionAuditVersionV1, request.AssumptionBaseline, func(ctx context.Context) (curriculum.ZeroAssumptionAuditResult, error) {
		return NewZeroAssumptionAuditV1().Audit(ctx, ZeroAssumptionAuditRequest{DomainProfile: request.DomainProfile, Baseline: request.AssumptionBaseline, Competencies: matrix, Concepts: concepts, Graph: graph, EvidenceSets: request.EvidenceSets})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	lessonInputs := temporalLessonInputs(hierarchy, concepts, request.ContextualConceptIDs)
	temporal, pass, err := runCompilerPass(ctx, "temporal-classification", curriculum.TemporalClassificationVersionV1, lessonInputs, func(ctx context.Context) (curriculum.TemporalClassificationResult, error) {
		return NewTemporalClassificationV1().Classify(ctx, TemporalClassificationRequest{Concepts: concepts, ContextualConceptIDs: request.ContextualConceptIDs, Lessons: lessonInputs, EvidenceSets: request.EvidenceSets})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	guidance, pass, err := runCompilerPass(ctx, "guidance-classification", curriculum.GuidanceClassifierVersionV1, temporal, func(ctx context.Context) (curriculum.GuidanceClassificationResult, error) {
		return NewGuidanceClassifierV1().Classify(ctx, GuidanceClassificationRequest{TemporalResult: temporal, EvidenceSets: request.EvidenceSets})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	gapScan, pass, err := runCompilerPass(ctx, "gap-scan", curriculum.GapScannerVersionV1, struct {
		Coverage      curriculum.CoverageReport
		Prerequisites []curriculum.PrerequisiteExpansionGap
		Guidance      []curriculum.CurrentGuidanceFinding
	}{coverage, expansion.UnresolvedGaps, guidance.CurrentGuidanceFindings}, func(ctx context.Context) (curriculum.GapScanReport, error) {
		return NewGapScannerV1().Scan(ctx, GapScanRequest{GoalID: request.Input.Goal.ID, Coverage: coverage, Requirements: request.CoverageRequirements, PrerequisiteGaps: expansion.UnresolvedGaps, CurrentGuidanceFindings: guidance.CurrentGuidanceFindings})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)

	draft := curriculum.CurriculumDefinition{ID: request.Metadata.ID, Version: request.Metadata.Version, Title: request.Metadata.Title, Description: request.Metadata.Description, Goal: request.Input.Goal, Competencies: matrix, Concepts: concepts, Prerequisites: graph.Prerequisites, Vocabulary: vocabulary.Graph, Phases: hierarchy.Phases, Modules: hierarchy.Modules, Lessons: hierarchy.Lessons, Topics: hierarchy.Topics, CoverageRequirements: request.CoverageRequirements, SourcePolicy: request.Config.SourcePolicy, SourceBundles: request.Input.SourceBundles, CreatedAt: request.Metadata.CreatedAt}
	diagnostics := &curriculum.CompilationDiagnostics{Decomposition: decomposition, Granularity: granularity, Graph: graph, Vocabulary: vocabulary, Hierarchy: hierarchy, Coverage: coverage, GapScan: gapScan, DefinitionBeforeUse: definitionAudit, ZeroAssumption: zeroAudit, Temporal: temporal, Guidance: guidance}
	result.Curriculum = draft
	result.Diagnostics = diagnostics
	for _, dimension := range coverage.Dimensions {
		result.Coverage = append(result.Coverage, dimension.Requirements...)
	}
	result.Gaps = append([]curriculum.Gap(nil), gapScan.Gaps...)
	result.Warnings = compilerWarnings(granularity, expansion, coverage, gapScan, definitionAudit, zeroAudit)
	reviewInput := struct {
		Curriculum  curriculum.CurriculumDefinition
		Diagnostics curriculum.CompilationDiagnostics
		Coverage    []curriculum.CoverageResult
		Gaps        []curriculum.Gap
		Warnings    []string
	}{result.Curriculum, *diagnostics, result.Coverage, result.Gaps, result.Warnings}
	review, pass, err := runCompilerPass(ctx, "final-review", curriculum.CurriculumReviewerVersionV1, reviewInput, func(ctx context.Context) (curriculum.CurriculumReviewResult, error) {
		return NewCurriculumReviewerV1(nil).Review(ctx, CurriculumReviewRequest{Compilation: result, EvidenceSets: request.EvidenceSets})
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)
	diagnostics.Review = &review
	_, pass, err = runCompilerPass(ctx, "compiled-artifact", curriculum.CurriculumCompilerVersionV1, diagnostics, func(context.Context) (string, error) {
		return compilerHash(struct {
			Curriculum  curriculum.CurriculumDefinition
			Diagnostics curriculum.CompilationDiagnostics
		}{draft, *diagnostics}), nil
	})
	if err != nil {
		return fail(pass, err)
	}
	appendPass(pass)
	if err := result.Validate(); err != nil {
		return result, Invalid(operation, err)
	}
	return result, nil
}

func runCompilerPass[T any](ctx context.Context, name, version string, input any, execute func(context.Context) (T, error)) (T, curriculum.CompilationPass, error) {
	started := time.Now()
	inputHash := compilerHash(input)
	output, err := execute(ctx)
	pass := curriculum.CompilationPass{Name: name, Version: version, InputHash: inputHash, Duration: time.Since(started)}
	if err != nil {
		pass.OutputHash = compilerHash(struct{ Error string }{err.Error()})
		pass.Errors = []string{err.Error()}
		return output, pass, err
	}
	pass.OutputHash = compilerHash(output)
	pass.Warnings = compilerPassWarnings(any(output))
	return output, pass, nil
}

func compilerPassWarnings(output any) []string {
	var warnings []string
	switch value := output.(type) {
	case curriculum.GranularityResult:
		for _, warning := range value.Warnings {
			warnings = append(warnings, warning.Code)
		}
	case curriculum.PrerequisiteExpansionResult:
		for _, gap := range value.UnresolvedGaps {
			warnings = append(warnings, string(gap.Code)+": "+gap.ConceptID.String())
		}
	case curriculum.CoverageReport:
		for _, dimension := range value.Dimensions {
			if dimension.Status != curriculum.CoverageCovered {
				warnings = append(warnings, string(dimension.Dimension)+": "+string(dimension.Status))
			}
		}
	case curriculum.DefinitionBeforeUseAuditResult:
		if !value.Passed {
			warnings = append(warnings, "definition-before-use audit failed")
		}
	case curriculum.ZeroAssumptionAuditResult:
		if !value.Passed {
			warnings = append(warnings, "zero-assumption audit failed")
		}
	case curriculum.GuidanceClassificationResult:
		for _, finding := range value.CurrentGuidanceFindings {
			warnings = append(warnings, "missing current guidance: "+finding.TargetID.String())
		}
	case curriculum.GapScanReport:
		for _, gap := range value.Gaps {
			warnings = append(warnings, string(gap.Severity)+": "+gap.ID.String())
		}
	}
	return warnings
}

func compilerHash(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		encoded = []byte(fmt.Sprintf("%T:%s", value, err))
	}
	digest := sha256.Sum256(encoded)
	return fmt.Sprintf("sha256:%x", digest)
}

func validateBuildMetadata(metadata CurriculumBuildMetadata) error {
	if err := metadata.ID.Validate(); err != nil {
		return err
	}
	if err := metadata.Version.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(metadata.Title) == "" || strings.TrimSpace(metadata.Description) == "" {
		return fmt.Errorf("curriculum title and description are required")
	}
	return metadata.CreatedAt.Validate()
}

func validateCompilerEvidence(references []curriculum.SourceBundleRef, sets []curriculum.CurriculumEvidenceSet) error {
	known := make(map[curriculum.ID]curriculum.SourceBundleRef, len(references))
	for _, reference := range references {
		known[reference.ID] = reference
	}
	if len(sets) != len(known) {
		return fmt.Errorf("compiler evidence sets do not match source bundle references")
	}
	for _, set := range sets {
		if err := set.Validate(); err != nil {
			return err
		}
		reference, exists := known[set.Bundle.ID]
		if !exists || reference.ContentHash != set.Bundle.ContentHash || reference.AlgorithmVersion != set.Bundle.AlgorithmVersion || !reference.VerifiedAt.Time().Equal(set.Bundle.VerifiedAt.Time()) {
			return fmt.Errorf("compiler evidence bundle %q does not match frozen input", set.Bundle.ID)
		}
	}
	return nil
}

func temporalLessonInputs(hierarchy curriculum.CurriculumHierarchy, concepts []curriculum.Concept, contextual []curriculum.ConceptID) []curriculum.LessonTemporalInput {
	byID := make(map[curriculum.ConceptID]curriculum.Concept, len(concepts))
	contextualSet := make(map[curriculum.ConceptID]struct{}, len(contextual))
	for _, concept := range concepts {
		byID[concept.ID] = concept
	}
	for _, id := range contextual {
		contextualSet[id] = struct{}{}
	}
	byLesson := make(map[curriculum.ID][]curriculum.ConceptID)
	for _, topic := range hierarchy.Topics {
		byLesson[topic.LessonID] = append(byLesson[topic.LessonID], topic.ConceptIDs...)
	}
	result := make([]curriculum.LessonTemporalInput, 0, len(hierarchy.Lessons))
	for _, lesson := range hierarchy.Lessons {
		ids := byLesson[lesson.ID]
		status := curriculum.ConceptCurrent
		contextOnly := len(ids) > 0
		for _, id := range ids {
			if temporalRank(byID[id].Status) > temporalRank(status) {
				status = byID[id].Status
			}
			if _, exists := contextualSet[id]; !exists {
				contextOnly = false
			}
		}
		result = append(result, curriculum.LessonTemporalInput{Lesson: lesson, ConceptIDs: ids, DeclaredStatus: status, ContextualUse: contextOnly})
	}
	return result
}

func temporalRank(status curriculum.ConceptStatus) int {
	switch status {
	case curriculum.ConceptDeprecated:
		return 6
	case curriculum.ConceptHistorical:
		return 5
	case curriculum.ConceptLegacy:
		return 4
	case curriculum.ConceptExperimental:
		return 3
	case curriculum.ConceptPreview:
		return 2
	default:
		return 1
	}
}

func compilerWarnings(granularity curriculum.GranularityResult, expansion curriculum.PrerequisiteExpansionResult, coverage curriculum.CoverageReport, gaps curriculum.GapScanReport, definition curriculum.DefinitionBeforeUseAuditResult, zero curriculum.ZeroAssumptionAuditResult) []string {
	var warnings []string
	for _, warning := range granularity.Warnings {
		warnings = append(warnings, "granularity: "+warning.Code)
	}
	for _, gap := range expansion.UnresolvedGaps {
		warnings = append(warnings, "prerequisite: "+string(gap.Code)+": "+gap.ConceptID.String())
	}
	for _, dimension := range coverage.Dimensions {
		if dimension.Status != curriculum.CoverageCovered {
			warnings = append(warnings, "coverage: "+string(dimension.Dimension)+": "+string(dimension.Status))
		}
	}
	for _, gap := range gaps.Gaps {
		warnings = append(warnings, "gap: "+gap.ID.String())
	}
	if !definition.Passed {
		warnings = append(warnings, "audit: definition-before-use failed")
	}
	if !zero.Passed {
		warnings = append(warnings, "audit: zero-assumption failed")
	}
	return warnings
}

var _ CurriculumCompilerService = CurriculumCompilerV1{}
