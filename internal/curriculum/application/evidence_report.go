package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

const PrimarySourceCoveragePolicyV1 = "primary-source-coverage-v1"

type CurriculumEvidenceReporterV1 struct{}

func NewCurriculumEvidenceReporterV1() CurriculumEvidenceReporterV1 {
	return CurriculumEvidenceReporterV1{}
}

func (CurriculumEvidenceReporterV1) Generate(ctx context.Context, request CurriculumEvidenceReportRequest) (CurriculumEvidenceReportResult, error) {
	const operation = "generate curriculum evidence report"
	if err := ctx.Err(); err != nil {
		return CurriculumEvidenceReportResult{}, ExternalError(operation, err)
	}
	if err := request.Compilation.Validate(); err != nil {
		return CurriculumEvidenceReportResult{}, Invalid(operation, err)
	}
	if request.Compilation.BuildInfo == nil {
		return CurriculumEvidenceReportResult{}, Invalid(operation, fmt.Errorf("compilation has no reproducibility metadata"))
	}
	definition := request.Compilation.Curriculum
	if err := validateCompilerEvidence(definition.SourceBundles, request.EvidenceSets); err != nil {
		return CurriculumEvidenceReportResult{}, Invalid(operation, err)
	}

	report := curriculum.CurriculumEvidenceReport{
		SchemaVersion: curriculum.CurriculumEvidenceReportSchemaVersionV1,
		Goal:          curriculum.EvidenceReportGoal{ID: definition.Goal.ID, Title: definition.Goal.Title, Domain: definition.Goal.Domain},
		ConceptCount:  len(definition.Concepts), SourceBundleCount: len(definition.SourceBundles),
		Bundles:         append([]curriculum.SourceBundleRef(nil), definition.SourceBundles...),
		Gaps:            append([]curriculum.Gap(nil), request.Compilation.Gaps...),
		CompilerVersion: request.Compilation.BuildInfo.CompilerVersion,
		PassVersions:    append([]curriculum.CompilationPassVersion(nil), request.Compilation.BuildInfo.Passes...),
		Citations:       append([]curriculum.EvidenceReportCitation(nil), request.Citations...),
	}
	for _, competency := range definition.Competencies.Competencies {
		report.Competencies = append(report.Competencies, curriculum.EvidenceReportCompetency{
			ID: competency.ID, Area: competency.Area, OutcomeID: competency.OutcomeID,
			ExpectedLevel: competency.ExpectedLevel, ConceptCount: len(competency.ConceptRefs),
		})
	}
	sort.Slice(report.Competencies, func(i, j int) bool { return report.Competencies[i].ID.String() < report.Competencies[j].ID.String() })

	report.Claims = compilationEvidenceRefs(definition)
	report.PrimarySourceCoverage = primaryCoverage(report.Claims, request.EvidenceSets)
	for _, set := range request.EvidenceSets {
		report.Freshness = append(report.Freshness, curriculum.EvidenceReportFreshness{BundleID: set.Bundle.ID, Freshness: set.Freshness})
		for _, conflict := range set.Conflicts {
			report.Conflicts = append(report.Conflicts, curriculum.EvidenceReportConflict{
				BundleID: set.Bundle.ID, ConflictID: conflict.ID, ClaimIDs: append([]curriculum.ID(nil), conflict.ClaimIDs...),
				Unresolved: conflict.Unresolved, Reason: conflict.Reason, AlgorithmVersion: conflict.AlgorithmVersion,
			})
		}
		for _, caveat := range set.Caveats {
			report.Caveats = append(report.Caveats, curriculum.EvidenceReportCaveat{BundleID: set.Bundle.ID, Text: caveat})
		}
	}
	for _, concept := range definition.Concepts {
		content := curriculum.EvidenceReportTemporalContent{ConceptID: concept.ID, Status: concept.Status}
		switch concept.Status {
		case curriculum.ConceptLegacy, curriculum.ConceptHistorical, curriculum.ConceptDeprecated:
			report.HistoricalContent = append(report.HistoricalContent, content)
		case curriculum.ConceptExperimental, curriculum.ConceptPreview:
			report.ExperimentalContent = append(report.ExperimentalContent, content)
		}
	}
	sortEvidenceReport(&report)
	if err := report.Validate(); err != nil {
		return CurriculumEvidenceReportResult{}, Invalid(operation, err)
	}
	return CurriculumEvidenceReportResult{Report: report, Markdown: RenderCurriculumEvidenceMarkdown(report)}, nil
}

func primaryCoverage(references []curriculum.EvidenceRef, sets []curriculum.CurriculumEvidenceSet) curriculum.EvidenceReportCoverage {
	primary := make(map[curriculum.ID]map[curriculum.ID]bool, len(sets))
	for _, set := range sets {
		roles := make(map[curriculum.ID]string, len(set.SourceAuthority))
		for _, source := range set.SourceAuthority {
			roles[source.SourceID] = source.Role
		}
		claims := make(map[curriculum.ID]bool, len(set.Claims))
		for _, claim := range set.Claims {
			for _, sourceID := range claim.SourceIDs {
				if roles[sourceID] == "primary" {
					claims[claim.ID] = true
					break
				}
			}
		}
		primary[set.Bundle.ID] = claims
	}
	covered := 0
	for _, reference := range references {
		if primary[reference.BundleID][reference.ClaimID] {
			covered++
		}
	}
	ratio := 0.0
	if len(references) > 0 {
		ratio = float64(covered) / float64(len(references))
	}
	return curriculum.EvidenceReportCoverage{ReferencedClaims: len(references), PrimaryClaims: covered, Ratio: ratio, PolicyVersion: PrimarySourceCoveragePolicyV1}
}

func compilationEvidenceRefs(definition curriculum.CurriculumDefinition) []curriculum.EvidenceRef {
	seen := make(map[curriculum.EvidenceRef]struct{})
	var result []curriculum.EvidenceRef
	add := func(values []curriculum.EvidenceRef) {
		for _, value := range values {
			if _, exists := seen[value]; !exists {
				seen[value] = struct{}{}
				result = append(result, value)
			}
		}
	}
	for _, outcome := range definition.Goal.Outcomes {
		add(outcome.EvidenceRefs)
	}
	for _, competency := range definition.Competencies.Competencies {
		add(competency.EvidenceRefs)
	}
	for _, concept := range definition.Concepts {
		add(concept.EvidenceRefs)
	}
	for _, prerequisite := range definition.Prerequisites {
		add(prerequisite.EvidenceRefs)
	}
	for _, requirement := range definition.CoverageRequirements {
		add(requirement.EvidenceRefs)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].BundleID != result[j].BundleID {
			return result[i].BundleID.String() < result[j].BundleID.String()
		}
		return result[i].ClaimID.String() < result[j].ClaimID.String()
	})
	return result
}

func sortEvidenceReport(report *curriculum.CurriculumEvidenceReport) {
	sort.Slice(report.Citations, func(i, j int) bool {
		return report.Citations[i].SourceID.String() < report.Citations[j].SourceID.String()
	})
	sort.Slice(report.Freshness, func(i, j int) bool {
		return report.Freshness[i].BundleID.String() < report.Freshness[j].BundleID.String()
	})
	sort.Slice(report.Conflicts, func(i, j int) bool {
		if report.Conflicts[i].BundleID != report.Conflicts[j].BundleID {
			return report.Conflicts[i].BundleID.String() < report.Conflicts[j].BundleID.String()
		}
		return report.Conflicts[i].ConflictID.String() < report.Conflicts[j].ConflictID.String()
	})
	sort.Slice(report.Caveats, func(i, j int) bool {
		if report.Caveats[i].BundleID != report.Caveats[j].BundleID {
			return report.Caveats[i].BundleID.String() < report.Caveats[j].BundleID.String()
		}
		return report.Caveats[i].Text < report.Caveats[j].Text
	})
	sort.Slice(report.HistoricalContent, func(i, j int) bool {
		return report.HistoricalContent[i].ConceptID.String() < report.HistoricalContent[j].ConceptID.String()
	})
	sort.Slice(report.ExperimentalContent, func(i, j int) bool {
		return report.ExperimentalContent[i].ConceptID.String() < report.ExperimentalContent[j].ConceptID.String()
	})
	sort.Slice(report.Gaps, func(i, j int) bool { return report.Gaps[i].ID.String() < report.Gaps[j].ID.String() })
}

func RenderCurriculumEvidenceMarkdown(report curriculum.CurriculumEvidenceReport) string {
	lines := []string{
		"# Curriculum Evidence Report", "",
		"Goal: " + markdownText(report.Goal.Title) + " (`" + report.Goal.ID.String() + "`)",
		"Domain: " + markdownText(report.Goal.Domain), "",
		"## Summary", "",
		fmt.Sprintf("- Competencies: %d", len(report.Competencies)),
		fmt.Sprintf("- Concepts: %d", report.ConceptCount),
		fmt.Sprintf("- Source bundles: %d", report.SourceBundleCount),
		fmt.Sprintf("- Primary-source coverage: %d/%d (%.1f%%)", report.PrimarySourceCoverage.PrimaryClaims, report.PrimarySourceCoverage.ReferencedClaims, report.PrimarySourceCoverage.Ratio*100),
		"- Compiler: `" + report.CompilerVersion + "`", "",
		"## Competencies", "",
	}
	for _, competency := range report.Competencies {
		lines = append(lines, fmt.Sprintf("- `%s` — %s, %s, %d concepts", competency.ID, markdownText(competency.Area), competency.ExpectedLevel, competency.ConceptCount))
	}
	lines = append(lines, "", "## Sources and freshness", "")
	for _, freshness := range report.Freshness {
		lines = append(lines, fmt.Sprintf("- `%s` — %s (%.2f), policy `%s`", freshness.BundleID, freshness.Freshness.State, freshness.Freshness.Score, freshness.Freshness.Algorithm))
	}
	lines = append(lines, "", "## Evidence references", "")
	for _, reference := range report.Claims {
		lines = append(lines, fmt.Sprintf("- `%s#%s`", reference.BundleID, reference.ClaimID))
	}
	lines = append(lines, "", "## Citations", "")
	if len(report.Citations) == 0 {
		lines = append(lines, "No source locators were supplied.")
	}
	for _, citation := range report.Citations {
		line := fmt.Sprintf("- [%s](<%s>) (`%s`)", markdownText(citation.Title), citation.URL, citation.SourceID)
		if citation.License != "" {
			line += " — license: " + markdownText(citation.License)
		}
		lines = append(lines, line)
		if citation.Excerpt != "" {
			lines = append(lines, "  - Minimal excerpt: “"+markdownText(citation.Excerpt)+"”")
		}
	}
	lines = append(lines, "", "## Conflicts and caveats", "")
	if len(report.Conflicts) == 0 && len(report.Caveats) == 0 {
		lines = append(lines, "None.")
	}
	for _, conflict := range report.Conflicts {
		state := "resolved"
		if conflict.Unresolved {
			state = "unresolved"
		}
		lines = append(lines, fmt.Sprintf("- Conflict `%s` in `%s` (%s): %s", conflict.ConflictID, conflict.BundleID, state, markdownText(conflict.Reason)))
	}
	for _, caveat := range report.Caveats {
		lines = append(lines, fmt.Sprintf("- Caveat in `%s`: %s", caveat.BundleID, markdownText(caveat.Text)))
	}
	lines = append(lines, "", "## Historical and experimental content", "")
	if len(report.HistoricalContent) == 0 && len(report.ExperimentalContent) == 0 {
		lines = append(lines, "None.")
	}
	for _, content := range report.HistoricalContent {
		lines = append(lines, fmt.Sprintf("- `%s` — %s", content.ConceptID, content.Status))
	}
	for _, content := range report.ExperimentalContent {
		lines = append(lines, fmt.Sprintf("- `%s` — %s", content.ConceptID, content.Status))
	}
	lines = append(lines, "", "## Gaps", "")
	if len(report.Gaps) == 0 {
		lines = append(lines, "None.")
	}
	for _, gap := range report.Gaps {
		lines = append(lines, fmt.Sprintf("- `%s` [%s] %s", gap.ID, gap.Severity, markdownText(gap.Reason)))
	}
	lines = append(lines, "", "## Compiler passes", "")
	for _, pass := range report.PassVersions {
		lines = append(lines, fmt.Sprintf("- `%s`: `%s`", pass.Name, pass.Version))
	}
	return strings.Join(lines, "\n") + "\n"
}

func markdownText(value string) string {
	value = strings.NewReplacer("\r", " ", "\n", " ", "\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "<", "&lt;", ">", "&gt;").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

var _ CurriculumEvidenceReportService = CurriculumEvidenceReporterV1{}
