package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

const CurriculumInspectionVersionV1 = "curriculum-inspection-v1"

// CurriculumCoverageSummary is the portable coverage information that can be
// inspected after a compilation. Support records stay in compiler diagnostics,
// so an installed pack reports declared requirements and retained gaps without
// pretending to recompute the original coverage pass.
type CurriculumCoverageSummary struct {
	Dimension        curriculum.CoverageDimension
	RequirementCount int
	GapCount         int
}

type CurriculumInspection struct {
	Pack              curriculum.LearningPack
	Coverage          []CurriculumCoverageSummary
	Gaps              []curriculum.Gap
	Audits            []CurriculumAuditResult
	CompilationPasses []curriculum.CompilationPassVersion
	EvidenceLinks     []curriculum.EvidenceReportCitation
	AlgorithmVersion  string
}

type CurriculumInspectionService interface {
	Inspect(context.Context, curriculum.LearningPack) (CurriculumInspection, error)
}

type CurriculumInspectorV1 struct{}

func NewCurriculumInspectorV1() CurriculumInspectorV1 { return CurriculumInspectorV1{} }

func (CurriculumInspectorV1) Inspect(ctx context.Context, pack curriculum.LearningPack) (CurriculumInspection, error) {
	const operation = "inspect compiled curriculum"
	if err := ctx.Err(); err != nil {
		return CurriculumInspection{}, ExternalError(operation, err)
	}
	if err := pack.Validate(); err != nil {
		return CurriculumInspection{}, Invalid(operation, err)
	}

	inspection := CurriculumInspection{
		Pack:             pack,
		AlgorithmVersion: CurriculumInspectionVersionV1,
	}
	if pack.BuildInfo != nil {
		inspection.CompilationPasses = append([]curriculum.CompilationPassVersion(nil), pack.BuildInfo.Passes...)
	}
	if pack.EvidenceReport != nil {
		inspection.Gaps = append([]curriculum.Gap(nil), pack.EvidenceReport.Gaps...)
		inspection.EvidenceLinks = append([]curriculum.EvidenceReportCitation(nil), pack.EvidenceReport.Citations...)
	}

	gapCounts := make(map[curriculum.CoverageDimension]int)
	for _, gap := range inspection.Gaps {
		if dimension, ok := gapCoverageDimension(gap.Kind); ok {
			gapCounts[dimension]++
		}
	}
	requirementCounts := make(map[curriculum.CoverageDimension]int)
	for _, requirement := range pack.Curriculum.CoverageRequirements {
		requirementCounts[requirement.Dimension]++
	}
	for _, dimension := range curriculum.AllCoverageDimensions() {
		inspection.Coverage = append(inspection.Coverage, CurriculumCoverageSummary{
			Dimension: dimension, RequirementCount: requirementCounts[dimension], GapCount: gapCounts[dimension],
		})
	}

	inspection.Audits = append(inspection.Audits,
		CurriculumAuditResult{Name: "pack-structure", Version: curriculum.LearningPackSchemaVersionV1, Passed: true, Reasons: []string{"manifest, curriculum, evidence, build, and environment cross-references are valid"}},
		buildAudit(pack), evidenceAudit(pack),
	)
	graph, err := NewKnowledgeGraphCompilerV1().Compile(ctx, KnowledgeGraphCompilationRequest{
		Concepts: pack.Curriculum.Concepts, Prerequisites: pack.Curriculum.Prerequisites,
	})
	if err != nil {
		return CurriculumInspection{}, err
	}
	graphPassed := len(graph.UnreachableConceptIDs) == 0
	graphReasons := []string{"prerequisite graph is acyclic"}
	if graphPassed {
		graphReasons = append(graphReasons, "every concept is reachable from an explicit foundational root")
	} else {
		graphReasons = append(graphReasons, fmt.Sprintf("%d concepts are unreachable from an explicit foundational root", len(graph.UnreachableConceptIDs)))
	}
	inspection.Audits = append(inspection.Audits, CurriculumAuditResult{
		Name: "prerequisite-graph", Version: curriculum.KnowledgeGraphCompilerVersionV1, Passed: graphPassed, Reasons: graphReasons,
	})
	sort.Slice(inspection.EvidenceLinks, func(i, j int) bool {
		return inspection.EvidenceLinks[i].SourceID.String() < inspection.EvidenceLinks[j].SourceID.String()
	})
	return inspection, nil
}

func buildAudit(pack curriculum.LearningPack) CurriculumAuditResult {
	if pack.BuildInfo == nil {
		return CurriculumAuditResult{Name: "reproducibility", Version: curriculum.ReproducibilityMetadataSchemaVersionV1, Passed: false, Reasons: []string{"pack has no reproducibility metadata"}}
	}
	return CurriculumAuditResult{Name: "reproducibility", Version: pack.BuildInfo.SchemaVersion, Passed: true, Reasons: []string{"compiler recipe, input hash, output hash, source bundles, and pass versions are retained"}}
}

func evidenceAudit(pack curriculum.LearningPack) CurriculumAuditResult {
	if pack.EvidenceReport == nil {
		return CurriculumAuditResult{Name: "source-evidence", Version: curriculum.CurriculumEvidenceReportSchemaVersionV1, Passed: false, Reasons: []string{"pack has no curriculum evidence report"}}
	}
	reasons := []string{fmt.Sprintf("%d referenced claims across %d verified source bundles", len(pack.EvidenceReport.Claims), len(pack.EvidenceReport.Bundles))}
	if len(pack.EvidenceReport.Citations) == 0 {
		reasons = append(reasons, "no source links are retained")
	}
	return CurriculumAuditResult{Name: "source-evidence", Version: pack.EvidenceReport.SchemaVersion, Passed: len(pack.EvidenceReport.Citations) > 0, Reasons: reasons}
}

func gapCoverageDimension(kind curriculum.GapKind) (curriculum.CoverageDimension, bool) {
	switch kind {
	case curriculum.GapMissingCompetency:
		return curriculum.CoverageCompetency, true
	case curriculum.GapMissingConcept:
		return curriculum.CoverageConcept, true
	case curriculum.GapMissingEvidence:
		return curriculum.CoverageEvidence, true
	case curriculum.GapMissingTheory:
		return curriculum.CoverageTheory, true
	case curriculum.GapMissingPractice:
		return curriculum.CoveragePractice, true
	case curriculum.GapMissingProduction:
		return curriculum.CoverageProduction, true
	case curriculum.GapMissingSecurity:
		return curriculum.CoverageSecurity, true
	case curriculum.GapMissingToolchain:
		return curriculum.CoverageToolchain, true
	default:
		return "", false
	}
}

var _ CurriculumInspectionService = CurriculumInspectorV1{}
