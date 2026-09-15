package learningpack

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
	"go.yaml.in/yaml/v3"
)

func encodePackEntries(request curriculumapp.PackBuildRequest, report curriculumapp.CurriculumEvidenceReportResult) (map[string][]byte, error) {
	entries := make(map[string][]byte, 8+len(request.Assets))
	encoders := []struct {
		name  string
		value any
		yaml  bool
	}{
		{ManifestName, encodeManifestDocument(request.Manifest), true},
		{request.Manifest.CurriculumEntry, encodeCurriculumDocument(request.Compilation.Curriculum), true},
		{request.Manifest.SourceEvidenceEntry, encodeEvidenceReportDocument(report.Report), false},
		{request.Manifest.BuildInfoEntry, encodeBuildInfoDocument(*request.Compilation.BuildInfo), false},
	}
	if request.Environment != nil {
		encoders = append(encoders, struct {
			name  string
			value any
			yaml  bool
		}{request.Manifest.EnvironmentEntry, encodeEnvironmentDocument(*request.Environment), true})
	}
	for _, item := range encoders {
		var encoded []byte
		var err error
		if item.yaml {
			encoded, err = yaml.Marshal(item.value)
		} else {
			encoded, err = json.MarshalIndent(item.value, "", "  ")
			encoded = append(encoded, '\n')
		}
		if err != nil {
			return nil, fmt.Errorf("encode %q: %w", item.name, err)
		}
		if _, exists := entries[item.name]; exists {
			return nil, fmt.Errorf("duplicate generated entry %q", item.name)
		}
		entries[item.name] = encoded
	}
	entries[EvidenceMarkdownName] = []byte(report.Markdown)
	entries["README.md"] = []byte(fmt.Sprintf("# %s\n\n%s\n", request.Manifest.Name, request.Manifest.Description))
	entries["LICENSE"] = []byte("Declared pack license: " + request.Manifest.License + "\n")
	ledger, err := encodeAssetLicenses(request.Assets)
	if err != nil {
		return nil, err
	}
	entries[AssetLicensesName] = append(ledger, '\n')
	for _, asset := range request.Assets {
		if _, exists := entries[asset.Path]; exists {
			return nil, fmt.Errorf("asset path %q conflicts with generated pack entry", asset.Path)
		}
		entries[asset.Path] = append([]byte(nil), asset.Content...)
	}
	return entries, nil
}

func encodeManifestDocument(value curriculum.PackManifest) manifestDocument {
	result := manifestDocument{
		ID: value.ID.String(), Name: value.Name, Description: value.Description, Version: value.Version.String(), SchemaVersion: value.SchemaVersion,
		Domain: value.Domain, Target: value.Target, Authors: append([]string(nil), value.Authors...), Maintainers: append([]string(nil), value.Maintainers...),
		License: value.License, CreatedAt: formatTimestamp(value.CreatedAt), MinimumKelyroVersion: value.MinimumKelyroVersion.String(),
		EnvironmentPack: value.EnvironmentEntry, CurriculumEntry: value.CurriculumEntry, SourceEvidenceEntry: value.SourceEvidenceEntry,
		BuildInfoEntry: value.BuildInfoEntry, Status: string(value.Status), CurriculumID: value.CurriculumID.String(),
	}
	for _, dependency := range value.Dependencies {
		result.Dependencies = append(result.Dependencies, dependencyDocument{ID: dependency.PackID.String(), Constraint: dependency.Constraint})
	}
	return result
}

func encodeCurriculumDocument(value curriculum.CurriculumDefinition) curriculumDocument {
	result := curriculumDocument{
		ID: value.ID.String(), Version: value.Version.String(), Title: value.Title, Description: value.Description,
		Goal: encodeGoalDocument(value.Goal), CompetencyMatrix: encodeMatrixDocument(value.Competencies),
		SourcePolicy: string(value.SourcePolicy), CreatedAt: formatTimestamp(value.CreatedAt),
	}
	for _, concept := range value.Concepts {
		result.Concepts = append(result.Concepts, conceptDocument{ID: concept.ID.String(), Title: concept.Title, Definition: concept.Definition, Version: concept.Version, Atomicity: string(concept.Atomicity), Difficulty: int(concept.Difficulty), Status: string(concept.Status), Foundational: concept.Foundational, EvidenceRefs: encodeEvidenceRefs(concept.EvidenceRefs)})
	}
	for _, edge := range value.Prerequisites {
		result.Prerequisites = append(result.Prerequisites, prerequisiteDocument{ConceptID: edge.ConceptID.String(), RequiredConceptID: edge.RequiredConceptID.String(), Kind: string(edge.Kind), EvidenceRefs: encodeEvidenceRefs(edge.EvidenceRefs)})
	}
	for _, term := range value.Vocabulary.Terms {
		used := make([]string, len(term.UsedBy))
		for index, id := range term.UsedBy {
			used[index] = id.String()
		}
		result.Vocabulary = append(result.Vocabulary, vocabularyDocument{Term: term.Term, CanonicalConceptID: term.CanonicalConceptID.String(), IntroducedBy: term.IntroducedBy.String(), UsedBy: used, Aliases: append([]string(nil), term.Aliases...), Scope: term.Scope})
	}
	for _, phase := range value.Phases {
		result.Phases = append(result.Phases, phaseDocument{ID: phase.ID.String(), Title: phase.Title, Description: phase.Description, Order: phase.Order})
	}
	for _, module := range value.Modules {
		result.Modules = append(result.Modules, moduleDocument{ID: module.ID.String(), PhaseID: module.PhaseID.String(), Title: module.Title, Description: module.Description, Order: module.Order})
	}
	for _, lesson := range value.Lessons {
		result.Lessons = append(result.Lessons, lessonDocument{ID: lesson.ID.String(), ModuleID: lesson.ModuleID.String(), Title: lesson.Title, Description: lesson.Description, Order: lesson.Order})
	}
	for _, topic := range value.Topics {
		ids := make([]string, len(topic.ConceptIDs))
		for index, id := range topic.ConceptIDs {
			ids[index] = id.String()
		}
		result.Topics = append(result.Topics, topicDocument{ID: topic.ID.String(), LessonID: topic.LessonID.String(), Title: topic.Title, Description: topic.Description, Order: topic.Order, ConceptIDs: ids})
	}
	for _, requirement := range value.CoverageRequirements {
		result.CoverageRequirements = append(result.CoverageRequirements, coverageRequirementDocument{ID: requirement.ID.String(), Dimension: string(requirement.Dimension), TargetKind: string(requirement.TargetKind), TargetID: requirement.TargetID.String(), Description: requirement.Description, EvidenceRefs: encodeEvidenceRefs(requirement.EvidenceRefs)})
	}
	for _, bundle := range value.SourceBundles {
		result.SourceBundles = append(result.SourceBundles, encodeBundleRef(bundle))
	}
	return result
}

func encodeGoalDocument(value curriculum.LearningGoalSpec) goalDocument {
	result := goalDocument{ID: value.ID.String(), Title: value.Title, Description: value.Description, Domain: value.Domain, Scope: append([]string(nil), value.Scope...), Exclusions: append([]string(nil), value.Exclusions...)}
	if value.Role != nil {
		result.Role = &roleDocument{ID: value.Role.ID.String(), Name: value.Role.Name, Description: value.Role.Description}
	}
	for _, outcome := range value.Outcomes {
		result.Outcomes = append(result.Outcomes, outcomeDocument{ID: outcome.ID.String(), Statement: outcome.Statement, Category: string(outcome.Category), Capability: string(outcome.Capability), EvidenceRefs: encodeEvidenceRefs(outcome.EvidenceRefs)})
	}
	return result
}

func encodeMatrixDocument(value curriculum.CompetencyMatrix) competencyMatrixDocument {
	result := competencyMatrixDocument{Version: value.Version, GoalID: value.GoalID.String()}
	for _, competency := range value.Competencies {
		concepts := make([]string, len(competency.ConceptRefs))
		for index, id := range competency.ConceptRefs {
			concepts[index] = id.String()
		}
		document := competencyDocument{ID: competency.ID.String(), AreaID: competency.AreaID.String(), Area: competency.Area, OutcomeID: competency.OutcomeID.String(), ExpectedLevel: string(competency.ExpectedLevel), EvidenceRefs: encodeEvidenceRefs(competency.EvidenceRefs), ConceptRefs: concepts}
		if competency.ParentID != nil {
			document.ParentID = competency.ParentID.String()
		}
		for _, dimension := range competency.Dimensions {
			document.Dimensions = append(document.Dimensions, dimensionDocument{ID: dimension.ID.String(), ExpectedLevel: string(dimension.ExpectedLevel)})
		}
		result.Competencies = append(result.Competencies, document)
	}
	return result
}

func encodeEnvironmentDocument(value curriculum.EnvironmentPack) environmentDocument {
	result := environmentDocument{SchemaVersion: value.SchemaVersion, ID: value.ID.String(), Version: value.Version.String(), PlatformSupport: append([]string(nil), value.SupportedPlatforms...)}
	for _, tool := range value.Tools {
		document := toolDocument{ID: tool.ID.String(), DisplayName: tool.DisplayName, Purpose: tool.Purpose, MinimumVersion: tool.MinimumVersion, Level: string(tool.Level), Platforms: append([]string(nil), tool.Platforms...), EvidenceRefs: encodeEvidenceRefs(tool.EvidenceRefs)}
		if tool.IntroducedAt != nil {
			document.IntroducedAt = tool.IntroducedAt.String()
		}
		if tool.WhenNeeded != nil {
			document.WhenNeeded = tool.WhenNeeded.String()
		}
		for _, id := range tool.InstallGuidanceRefs {
			document.InstallGuidanceRefs = append(document.InstallGuidanceRefs, id.String())
		}
		result.Tools = append(result.Tools, document)
	}
	for _, guidance := range value.InstallGuidance {
		result.InstallGuidance = append(result.InstallGuidance, installGuidanceDocument{ID: guidance.ID.String(), Platform: guidance.Platform, SourceName: guidance.SourceName, OfficialURL: guidance.OfficialURL, Instructions: guidance.Instructions, EvidenceRefs: encodeEvidenceRefs(guidance.EvidenceRefs)})
	}
	return result
}

func encodeEvidenceReportDocument(value curriculum.CurriculumEvidenceReport) evidenceReportDocument {
	result := evidenceReportDocument{
		SchemaVersion: value.SchemaVersion, Goal: evidenceReportGoalDocument{ID: value.Goal.ID.String(), Title: value.Goal.Title, Domain: value.Goal.Domain},
		ConceptCount: value.ConceptCount, SourceBundleCount: value.SourceBundleCount, Claims: encodeEvidenceRefs(value.Claims),
		PrimarySourceCoverage: evidenceReportCoverageDocument{ReferencedClaims: value.PrimarySourceCoverage.ReferencedClaims, PrimaryClaims: value.PrimarySourceCoverage.PrimaryClaims, Ratio: value.PrimarySourceCoverage.Ratio, PolicyVersion: value.PrimarySourceCoverage.PolicyVersion},
		CompilerVersion:       value.CompilerVersion,
	}
	for _, competency := range value.Competencies {
		result.Competencies = append(result.Competencies, evidenceReportCompetencyDocument{ID: competency.ID.String(), Area: competency.Area, OutcomeID: competency.OutcomeID.String(), ExpectedLevel: string(competency.ExpectedLevel), ConceptCount: competency.ConceptCount})
	}
	for _, bundle := range value.Bundles {
		result.Bundles = append(result.Bundles, encodeBundleRef(bundle))
	}
	for _, citation := range value.Citations {
		result.Citations = append(result.Citations, evidenceReportCitationDocument{SourceID: citation.SourceID.String(), Title: citation.Title, URL: citation.URL, License: citation.License, Excerpt: citation.Excerpt, ExcerptHash: citation.ExcerptHash})
	}
	for _, freshness := range value.Freshness {
		document := evidenceReportFreshnessDocument{BundleID: freshness.BundleID.String(), State: freshness.Freshness.State, Score: freshness.Freshness.Score, Algorithm: freshness.Freshness.Algorithm}
		if freshness.Freshness.LastVerifiedAt != nil {
			document.LastVerifiedAt = formatTimestamp(*freshness.Freshness.LastVerifiedAt)
		}
		result.Freshness = append(result.Freshness, document)
	}
	for _, conflict := range value.Conflicts {
		ids := make([]string, len(conflict.ClaimIDs))
		for index, id := range conflict.ClaimIDs {
			ids[index] = id.String()
		}
		result.Conflicts = append(result.Conflicts, evidenceReportConflictDocument{BundleID: conflict.BundleID.String(), ConflictID: conflict.ConflictID.String(), ClaimIDs: ids, Unresolved: conflict.Unresolved, Reason: conflict.Reason, AlgorithmVersion: conflict.AlgorithmVersion})
	}
	for _, caveat := range value.Caveats {
		result.Caveats = append(result.Caveats, evidenceReportCaveatDocument{BundleID: caveat.BundleID.String(), Text: caveat.Text})
	}
	for _, content := range value.HistoricalContent {
		result.HistoricalContent = append(result.HistoricalContent, temporalContentDocument{ConceptID: content.ConceptID.String(), Status: string(content.Status)})
	}
	for _, content := range value.ExperimentalContent {
		result.ExperimentalContent = append(result.ExperimentalContent, temporalContentDocument{ConceptID: content.ConceptID.String(), Status: string(content.Status)})
	}
	for _, gap := range value.Gaps {
		result.Gaps = append(result.Gaps, gapDocument{ID: gap.ID.String(), Kind: string(gap.Kind), Severity: string(gap.Severity), TargetID: gap.TargetID.String(), Reason: gap.Reason, EvidenceRefs: encodeEvidenceRefs(gap.EvidenceRefs)})
	}
	for _, pass := range value.PassVersions {
		result.PassVersions = append(result.PassVersions, compilationPassDocument{Name: pass.Name, Version: pass.Version})
	}
	return result
}

func encodeBuildInfoDocument(value curriculum.ReproducibilityMetadata) buildInfoDocument {
	result := buildInfoDocument{SchemaVersion: value.SchemaVersion, CompilerVersion: value.CompilerVersion, CompilationConfig: compilationConfigDocument{CompilerVersion: value.CompilationConfig.CompilerVersion, SourcePolicy: string(value.CompilationConfig.SourcePolicy), PackSchemaVersion: value.CompilationConfig.PackSchemaVersion}, PackSchemaVersion: value.PackSchemaVersion, InputHash: value.InputHash, OutputHash: value.OutputHash, BuiltAt: formatTimestamp(value.BuiltAt)}
	for _, pass := range value.Passes {
		result.Passes = append(result.Passes, compilationPassDocument{Name: pass.Name, Version: pass.Version})
	}
	for _, bundle := range value.SourceBundles {
		result.SourceBundles = append(result.SourceBundles, encodeBundleRef(bundle))
	}
	return result
}

func encodeEvidenceRefs(values []curriculum.EvidenceRef) []evidenceRefDocument {
	result := make([]evidenceRefDocument, len(values))
	for index, value := range values {
		result[index] = evidenceRefDocument{BundleID: value.BundleID.String(), ClaimID: value.ClaimID.String()}
	}
	return result
}

func encodeBundleRef(value curriculum.SourceBundleRef) bundleRefDocument {
	return bundleRefDocument{ID: value.ID.String(), ContentHash: value.ContentHash, AlgorithmVersion: value.AlgorithmVersion, VerifiedAt: formatTimestamp(value.VerifiedAt)}
}
func formatTimestamp(value curriculum.Timestamp) string {
	return value.Time().UTC().Format(time.RFC3339Nano)
}
