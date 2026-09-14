package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type CurriculumReviewerV1 struct {
	advisor CurriculumReviewAdvisor
}

func NewCurriculumReviewerV1(advisor CurriculumReviewAdvisor) CurriculumReviewerV1 {
	return CurriculumReviewerV1{advisor: advisor}
}

func (reviewer CurriculumReviewerV1) Review(ctx context.Context, request CurriculumReviewRequest) (curriculum.CurriculumReviewResult, error) {
	const operation = "review curriculum"
	if err := ctx.Err(); err != nil {
		return curriculum.CurriculumReviewResult{}, ExternalError(operation, err)
	}
	if err := request.Compilation.Validate(); err != nil {
		return curriculum.CurriculumReviewResult{}, Invalid(operation, err)
	}
	if request.Compilation.Diagnostics == nil {
		return curriculum.CurriculumReviewResult{}, Invalid(operation, fmt.Errorf("compilation diagnostics are required"))
	}
	if err := validateCompilerEvidence(request.Compilation.Curriculum.SourceBundles, request.EvidenceSets); err != nil {
		return curriculum.CurriculumReviewResult{}, Invalid(operation, err)
	}
	diagnostics := request.Compilation.Diagnostics
	dimensions := map[curriculum.CurriculumReviewDimension][]curriculum.CurriculumReviewFinding{
		curriculum.ReviewCoverage:            reviewGeneralCoverage(diagnostics.Coverage),
		curriculum.ReviewGranularity:         reviewGranularity(diagnostics.Granularity),
		curriculum.ReviewPrerequisites:       reviewPrerequisites(diagnostics.Graph, diagnostics.GapScan),
		curriculum.ReviewDefinitionBeforeUse: reviewDefinitionBeforeUse(diagnostics.DefinitionBeforeUse),
		curriculum.ReviewZeroAssumption:      reviewZeroAssumption(diagnostics.ZeroAssumption),
		curriculum.ReviewSourceReadiness:     reviewSourceReadiness(request.EvidenceSets),
		curriculum.ReviewFreshness:           reviewFreshness(request.EvidenceSets),
		curriculum.ReviewSecurity:            reviewCoverageDimension(diagnostics.Coverage, curriculum.CoverageSecurity),
		curriculum.ReviewProduction:          reviewCoverageDimension(diagnostics.Coverage, curriculum.CoverageProduction),
		curriculum.ReviewToolchain:           reviewCoverageDimension(diagnostics.Coverage, curriculum.CoverageToolchain),
		curriculum.ReviewTemporalStatus:      reviewTemporal(diagnostics.Temporal),
		curriculum.ReviewBeginnerSimulation:  reviewBeginnerSimulation(diagnostics.BeginnerSimulation),
		curriculum.ReviewExpertCoverage:      reviewExpertCoverage(diagnostics.ExpertCoverage),
	}
	result := curriculum.CurriculumReviewResult{AlgorithmVersion: curriculum.CurriculumReviewerVersionV1}
	hasError, hasWarning := false, false
	for _, dimension := range curriculum.AllCurriculumReviewDimensions() {
		findings := dimensions[dimension]
		sort.Slice(findings, func(i, j int) bool {
			if findings[i].Code != findings[j].Code {
				return findings[i].Code < findings[j].Code
			}
			if findings[i].Target != findings[j].Target {
				return findings[i].Target < findings[j].Target
			}
			return findings[i].Reason < findings[j].Reason
		})
		for _, finding := range findings {
			hasError = hasError || finding.Severity == curriculum.ReviewError
			hasWarning = hasWarning || finding.Severity == curriculum.ReviewWarning
		}
		result.Dimensions = append(result.Dimensions, curriculum.CurriculumReviewDimensionResult{Dimension: dimension, Passed: !containsReviewError(findings), Findings: findings})
	}
	result.Decision = curriculum.ReviewApproved
	if hasError {
		result.Decision = curriculum.ReviewRejected
	} else if hasWarning {
		result.Decision = curriculum.ReviewApprovedWithWarnings
	}
	if err := result.Validate(); err != nil {
		return curriculum.CurriculumReviewResult{}, Invalid(operation, err)
	}
	if reviewer.advisor != nil {
		notes, err := reviewer.advisor.Advise(ctx, request, result)
		if err != nil {
			result.AdvisorNotes = []string{"advisor unavailable: " + err.Error()}
		} else {
			for _, note := range notes {
				if normalized := strings.TrimSpace(note); normalized != "" {
					result.AdvisorNotes = append(result.AdvisorNotes, normalized)
				}
			}
			sort.Strings(result.AdvisorNotes)
		}
	}
	if err := result.Validate(); err != nil {
		return curriculum.CurriculumReviewResult{}, Invalid(operation, err)
	}
	return result, nil
}

func reviewBeginnerSimulation(result curriculum.BeginnerSimulationResult) []curriculum.CurriculumReviewFinding {
	findings := make([]curriculum.CurriculumReviewFinding, 0, len(result.Gaps))
	for _, gap := range result.Gaps {
		findings = append(findings, curriculum.CurriculumReviewFinding{
			Severity: curriculum.ReviewError,
			Code:     string(gap.Kind),
			Target:   gap.ConceptID.String(),
			Reason:   gap.Requirement + ": " + gap.Reason,
		})
	}
	return findings
}

func reviewExpertCoverage(result curriculum.ExpertCoverageReviewResult) []curriculum.CurriculumReviewFinding {
	findings := make([]curriculum.CurriculumReviewFinding, 0, len(result.Findings))
	for _, finding := range result.Findings {
		target := finding.TargetID.String()
		if finding.Dimension != "" {
			target += ":" + string(finding.Dimension)
		}
		findings = append(findings, curriculum.CurriculumReviewFinding{
			Severity: curriculum.ReviewError, Code: string(finding.Kind), Target: target, Reason: finding.Reason,
		})
	}
	return findings
}

func reviewGeneralCoverage(report curriculum.CoverageReport) []curriculum.CurriculumReviewFinding {
	var findings []curriculum.CurriculumReviewFinding
	for _, dimension := range []curriculum.CoverageDimension{curriculum.CoverageCompetency, curriculum.CoverageConcept, curriculum.CoverageEvidence, curriculum.CoverageTheory, curriculum.CoveragePractice} {
		findings = append(findings, reviewCoverageDimension(report, dimension)...)
	}
	return findings
}

func reviewCoverageDimension(report curriculum.CoverageReport, target curriculum.CoverageDimension) []curriculum.CurriculumReviewFinding {
	for _, dimension := range report.Dimensions {
		if dimension.Dimension != target || dimension.Status == curriculum.CoverageCovered {
			continue
		}
		return []curriculum.CurriculumReviewFinding{{Severity: curriculum.ReviewError, Code: "coverage_" + string(dimension.Status), Target: string(target), Reason: strings.Join(dimension.Reasons, "; ")}}
	}
	return nil
}

func reviewGranularity(result curriculum.GranularityResult) []curriculum.CurriculumReviewFinding {
	findings := make([]curriculum.CurriculumReviewFinding, 0, len(result.Warnings)+len(result.ForcedSplit))
	for _, warning := range result.Warnings {
		findings = append(findings, curriculum.CurriculumReviewFinding{Severity: curriculum.ReviewWarning, Code: warning.Code, Target: joinConceptIDs(warning.ConceptIDs), Reason: warning.Message})
	}
	for _, id := range result.ForcedSplit {
		findings = append(findings, curriculum.CurriculumReviewFinding{Severity: curriculum.ReviewError, Code: "concept_requires_split", Target: id.String(), Reason: "concept is not independently atomic"})
	}
	return findings
}

func reviewPrerequisites(graph curriculum.KnowledgeGraphCompilation, gaps curriculum.GapScanReport) []curriculum.CurriculumReviewFinding {
	var findings []curriculum.CurriculumReviewFinding
	for _, id := range graph.UnreachableConceptIDs {
		findings = append(findings, curriculum.CurriculumReviewFinding{Severity: curriculum.ReviewError, Code: "concept_unreachable_from_foundation", Target: id.String(), Reason: "concept is not reachable from an explicit foundational root"})
	}
	for _, gap := range gaps.Gaps {
		if gap.Kind == curriculum.GapMissingPrerequisite {
			findings = append(findings, curriculum.CurriculumReviewFinding{Severity: curriculum.ReviewError, Code: "missing_prerequisite", Target: gap.ID.String(), Reason: gap.Reason})
		}
	}
	return findings
}

func reviewDefinitionBeforeUse(result curriculum.DefinitionBeforeUseAuditResult) []curriculum.CurriculumReviewFinding {
	findings := make([]curriculum.CurriculumReviewFinding, 0, len(result.Violations))
	for _, violation := range result.Violations {
		severity := curriculum.ReviewError
		if violation.Severity == curriculum.CurriculumAuditWarning {
			severity = curriculum.ReviewWarning
		}
		findings = append(findings, curriculum.CurriculumReviewFinding{Severity: severity, Code: string(violation.Code), Target: violation.UsedAt.String(), Reason: violation.Reason})
	}
	return findings
}

func reviewZeroAssumption(result curriculum.ZeroAssumptionAuditResult) []curriculum.CurriculumReviewFinding {
	findings := make([]curriculum.CurriculumReviewFinding, 0, len(result.Violations))
	for _, violation := range result.Violations {
		severity := curriculum.ReviewError
		if violation.Severity == curriculum.CurriculumAuditWarning {
			severity = curriculum.ReviewWarning
		}
		findings = append(findings, curriculum.CurriculumReviewFinding{Severity: severity, Code: string(violation.Code), Target: violation.TargetConceptID.String(), Reason: violation.Reason})
	}
	return findings
}

func reviewSourceReadiness(sets []curriculum.CurriculumEvidenceSet) []curriculum.CurriculumReviewFinding {
	var findings []curriculum.CurriculumReviewFinding
	for _, set := range sets {
		switch set.Eligibility {
		case curriculum.EvidenceNotReady:
			findings = append(findings, curriculum.CurriculumReviewFinding{Severity: curriculum.ReviewError, Code: "source_not_ready", Target: set.Bundle.ID.String(), Reason: "source bundle is not ready for compilation"})
		case curriculum.EvidenceReadyWithCaveats:
			findings = append(findings, curriculum.CurriculumReviewFinding{Severity: curriculum.ReviewWarning, Code: "source_ready_with_caveats", Target: set.Bundle.ID.String(), Reason: strings.Join(set.Caveats, "; ")})
		}
		for _, conflict := range set.Conflicts {
			if conflict.Unresolved {
				findings = append(findings, curriculum.CurriculumReviewFinding{Severity: curriculum.ReviewError, Code: "source_conflict_unresolved", Target: conflict.ID.String(), Reason: conflict.Reason})
			}
		}
	}
	return findings
}

func reviewFreshness(sets []curriculum.CurriculumEvidenceSet) []curriculum.CurriculumReviewFinding {
	var findings []curriculum.CurriculumReviewFinding
	for _, set := range sets {
		switch set.Freshness.State {
		case "aging":
			findings = append(findings, curriculum.CurriculumReviewFinding{Severity: curriculum.ReviewWarning, Code: "source_aging", Target: set.Bundle.ID.String(), Reason: "source bundle is aging and should be reviewed"})
		case "stale", "unknown":
			findings = append(findings, curriculum.CurriculumReviewFinding{Severity: curriculum.ReviewError, Code: "source_" + set.Freshness.State, Target: set.Bundle.ID.String(), Reason: "source bundle freshness is insufficient for publication"})
		}
	}
	return findings
}

func reviewTemporal(result curriculum.TemporalClassificationResult) []curriculum.CurriculumReviewFinding {
	var findings []curriculum.CurriculumReviewFinding
	for _, classification := range result.Classifications {
		target := string(classification.TargetKind) + ":" + classification.TargetID.String()
		switch classification.Status {
		case curriculum.ConceptDeprecated:
			findings = append(findings, curriculum.CurriculumReviewFinding{Severity: curriculum.ReviewError, Code: "deprecated_guidance", Target: target, Reason: "deprecated guidance cannot be published as an active curriculum recommendation"})
		case curriculum.ConceptLegacy, curriculum.ConceptHistorical:
			if !classification.ContextualUse {
				findings = append(findings, curriculum.CurriculumReviewFinding{Severity: curriculum.ReviewError, Code: "historical_without_context", Target: target, Reason: "legacy or historical guidance lacks contextual-use classification"})
			}
		case curriculum.ConceptPreview, curriculum.ConceptExperimental:
			findings = append(findings, curriculum.CurriculumReviewFinding{Severity: curriculum.ReviewWarning, Code: "experimental_guidance", Target: target, Reason: "preview or experimental guidance must remain visibly separated"})
		}
	}
	return findings
}

func containsReviewError(findings []curriculum.CurriculumReviewFinding) bool {
	for _, finding := range findings {
		if finding.Severity == curriculum.ReviewError {
			return true
		}
	}
	return false
}

func joinConceptIDs(ids []curriculum.ConceptID) string {
	values := make([]string, len(ids))
	for index, id := range ids {
		values[index] = id.String()
	}
	sort.Strings(values)
	return strings.Join(values, ",")
}

var _ CurriculumReviewerService = CurriculumReviewerV1{}
