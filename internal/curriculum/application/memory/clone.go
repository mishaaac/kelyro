package memory

import (
	"github.com/mishaaac/kelyro/internal/curriculum"
	"github.com/mishaaac/kelyro/internal/curriculum/application"
)

func cloneDefinition(value curriculum.CurriculumDefinition) curriculum.CurriculumDefinition {
	cloned := value
	cloned.Goal = cloneGoal(value.Goal)
	cloned.Competencies = value.Competencies
	cloned.Competencies.Competencies = make([]curriculum.Competency, len(value.Competencies.Competencies))
	for index, competency := range value.Competencies.Competencies {
		cloned.Competencies.Competencies[index] = competency
		cloned.Competencies.Competencies[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), competency.EvidenceRefs...)
		cloned.Competencies.Competencies[index].ConceptRefs = append([]curriculum.ConceptID(nil), competency.ConceptRefs...)
	}
	cloned.Concepts = make([]curriculum.Concept, len(value.Concepts))
	for index, concept := range value.Concepts {
		cloned.Concepts[index] = concept
		cloned.Concepts[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), concept.EvidenceRefs...)
	}
	cloned.Prerequisites = make([]curriculum.Prerequisite, len(value.Prerequisites))
	for index, prerequisite := range value.Prerequisites {
		cloned.Prerequisites[index] = prerequisite
		cloned.Prerequisites[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), prerequisite.EvidenceRefs...)
	}
	cloned.Vocabulary.Terms = make([]curriculum.VocabularyTerm, len(value.Vocabulary.Terms))
	for index, term := range value.Vocabulary.Terms {
		cloned.Vocabulary.Terms[index] = term
		cloned.Vocabulary.Terms[index].UsedBy = append([]curriculum.ConceptID(nil), term.UsedBy...)
		cloned.Vocabulary.Terms[index].Aliases = append([]string(nil), term.Aliases...)
	}
	cloned.Phases = append([]curriculum.Phase(nil), value.Phases...)
	cloned.Modules = append([]curriculum.Module(nil), value.Modules...)
	cloned.Lessons = append([]curriculum.LessonSpec(nil), value.Lessons...)
	cloned.Topics = make([]curriculum.TopicSpec, len(value.Topics))
	for index, topic := range value.Topics {
		cloned.Topics[index] = topic
		cloned.Topics[index].ConceptIDs = append([]curriculum.ConceptID(nil), topic.ConceptIDs...)
	}
	cloned.CoverageRequirements = make([]curriculum.CoverageRequirement, len(value.CoverageRequirements))
	for index, requirement := range value.CoverageRequirements {
		cloned.CoverageRequirements[index] = requirement
		cloned.CoverageRequirements[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), requirement.EvidenceRefs...)
	}
	cloned.SourceBundles = append([]curriculum.SourceBundleRef(nil), value.SourceBundles...)
	return cloned
}

func cloneGoal(value curriculum.LearningGoalSpec) curriculum.LearningGoalSpec {
	cloned := value
	if value.Role != nil {
		role := *value.Role
		cloned.Role = &role
	}
	cloned.Outcomes = make([]curriculum.GoalOutcome, len(value.Outcomes))
	for index, outcome := range value.Outcomes {
		cloned.Outcomes[index] = outcome
		cloned.Outcomes[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), outcome.EvidenceRefs...)
	}
	cloned.Scope = append([]string(nil), value.Scope...)
	cloned.Exclusions = append([]string(nil), value.Exclusions...)
	return cloned
}

func cloneManifest(value curriculum.PackManifest) curriculum.PackManifest {
	cloned := value
	cloned.Dependencies = append([]curriculum.PackDependency(nil), value.Dependencies...)
	return cloned
}

func cloneManifests(values []curriculum.PackManifest) []curriculum.PackManifest {
	cloned := make([]curriculum.PackManifest, len(values))
	for index, value := range values {
		cloned[index] = cloneManifest(value)
	}
	return cloned
}

func cloneEnvironment(value curriculum.EnvironmentPack) curriculum.EnvironmentPack {
	cloned := value
	cloned.SupportedPlatforms = append([]string(nil), value.SupportedPlatforms...)
	cloned.Tools = make([]curriculum.ToolRequirement, len(value.Tools))
	for index, tool := range value.Tools {
		cloned.Tools[index] = tool
		if tool.IntroducedAt != nil {
			introducedAt := *tool.IntroducedAt
			cloned.Tools[index].IntroducedAt = &introducedAt
		}
		if tool.WhenNeeded != nil {
			whenNeeded := *tool.WhenNeeded
			cloned.Tools[index].WhenNeeded = &whenNeeded
		}
		cloned.Tools[index].Platforms = append([]string(nil), tool.Platforms...)
		cloned.Tools[index].InstallGuidanceRefs = append([]curriculum.ID(nil), tool.InstallGuidanceRefs...)
		cloned.Tools[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), tool.EvidenceRefs...)
	}
	cloned.InstallGuidance = make([]curriculum.ToolInstallGuidance, len(value.InstallGuidance))
	for index, guidance := range value.InstallGuidance {
		cloned.InstallGuidance[index] = guidance
		cloned.InstallGuidance[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), guidance.EvidenceRefs...)
	}
	return cloned
}

func clonePack(value curriculum.LearningPack) curriculum.LearningPack {
	cloned := value
	cloned.Manifest = cloneManifest(value.Manifest)
	cloned.Curriculum = cloneDefinition(value.Curriculum)
	if value.Environment != nil {
		environment := cloneEnvironment(*value.Environment)
		cloned.Environment = &environment
	}
	if value.BuildInfo != nil {
		buildInfo := *value.BuildInfo
		buildInfo.Passes = append([]curriculum.CompilationPassVersion(nil), value.BuildInfo.Passes...)
		buildInfo.SourceBundles = append([]curriculum.SourceBundleRef(nil), value.BuildInfo.SourceBundles...)
		cloned.BuildInfo = &buildInfo
	}
	if value.EvidenceReport != nil {
		report := *value.EvidenceReport
		report.Competencies = append([]curriculum.EvidenceReportCompetency(nil), value.EvidenceReport.Competencies...)
		report.Bundles = append([]curriculum.SourceBundleRef(nil), value.EvidenceReport.Bundles...)
		report.Claims = append([]curriculum.EvidenceRef(nil), value.EvidenceReport.Claims...)
		report.Citations = append([]curriculum.EvidenceReportCitation(nil), value.EvidenceReport.Citations...)
		report.Freshness = append([]curriculum.EvidenceReportFreshness(nil), value.EvidenceReport.Freshness...)
		for index := range report.Freshness {
			if value.EvidenceReport.Freshness[index].Freshness.LastVerifiedAt != nil {
				verified := *value.EvidenceReport.Freshness[index].Freshness.LastVerifiedAt
				report.Freshness[index].Freshness.LastVerifiedAt = &verified
			}
		}
		report.Conflicts = make([]curriculum.EvidenceReportConflict, len(value.EvidenceReport.Conflicts))
		for index, conflict := range value.EvidenceReport.Conflicts {
			report.Conflicts[index] = conflict
			report.Conflicts[index].ClaimIDs = append([]curriculum.ID(nil), conflict.ClaimIDs...)
		}
		report.Caveats = append([]curriculum.EvidenceReportCaveat(nil), value.EvidenceReport.Caveats...)
		report.HistoricalContent = append([]curriculum.EvidenceReportTemporalContent(nil), value.EvidenceReport.HistoricalContent...)
		report.ExperimentalContent = append([]curriculum.EvidenceReportTemporalContent(nil), value.EvidenceReport.ExperimentalContent...)
		report.Gaps = make([]curriculum.Gap, len(value.EvidenceReport.Gaps))
		for index, gap := range value.EvidenceReport.Gaps {
			report.Gaps[index] = gap
			report.Gaps[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), gap.EvidenceRefs...)
		}
		report.PassVersions = append([]curriculum.CompilationPassVersion(nil), value.EvidenceReport.PassVersions...)
		cloned.EvidenceReport = &report
	}
	return cloned
}

func cloneCompilation(value application.CompilationRecord) application.CompilationRecord {
	cloned := value
	cloned.Input.Goal = cloneGoal(value.Input.Goal)
	cloned.Input.SourceBundles = append([]curriculum.SourceBundleRef(nil), value.Input.SourceBundles...)
	cloned.Result.Curriculum = cloneDefinition(value.Result.Curriculum)
	cloned.Result.Passes = make([]curriculum.CompilationPass, len(value.Result.Passes))
	for index, pass := range value.Result.Passes {
		cloned.Result.Passes[index] = pass
		cloned.Result.Passes[index].Warnings = append([]string(nil), pass.Warnings...)
		cloned.Result.Passes[index].Errors = append([]string(nil), pass.Errors...)
	}
	cloned.Result.Coverage = make([]curriculum.CoverageResult, len(value.Result.Coverage))
	for index, result := range value.Result.Coverage {
		cloned.Result.Coverage[index] = result
		cloned.Result.Coverage[index].Reasons = append([]string(nil), result.Reasons...)
	}
	cloned.Result.Gaps = make([]curriculum.Gap, len(value.Result.Gaps))
	for index, gap := range value.Result.Gaps {
		cloned.Result.Gaps[index] = gap
		cloned.Result.Gaps[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), gap.EvidenceRefs...)
	}
	cloned.Result.Warnings = append([]string(nil), value.Result.Warnings...)
	if value.Result.BuildInfo != nil {
		buildInfo := *value.Result.BuildInfo
		buildInfo.Passes = append([]curriculum.CompilationPassVersion(nil), value.Result.BuildInfo.Passes...)
		buildInfo.SourceBundles = append([]curriculum.SourceBundleRef(nil), value.Result.BuildInfo.SourceBundles...)
		cloned.Result.BuildInfo = &buildInfo
	}
	if value.Result.Diagnostics != nil {
		diagnostics := cloneCompilationDiagnostics(*value.Result.Diagnostics)
		cloned.Result.Diagnostics = &diagnostics
	}
	return cloned
}

func cloneCompilationDiagnostics(value curriculum.CompilationDiagnostics) curriculum.CompilationDiagnostics {
	cloned := value
	cloned.Decomposition.Outcomes = append([]curriculum.GoalOutcome(nil), value.Decomposition.Outcomes...)
	for index := range cloned.Decomposition.Outcomes {
		cloned.Decomposition.Outcomes[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), value.Decomposition.Outcomes[index].EvidenceRefs...)
	}
	cloned.Decomposition.CompetencyAreas = append([]curriculum.CompetencyArea(nil), value.Decomposition.CompetencyAreas...)
	for index := range cloned.Decomposition.CompetencyAreas {
		cloned.Decomposition.CompetencyAreas[index].OutcomeIDs = append([]curriculum.ID(nil), value.Decomposition.CompetencyAreas[index].OutcomeIDs...)
		cloned.Decomposition.CompetencyAreas[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), value.Decomposition.CompetencyAreas[index].EvidenceRefs...)
	}
	cloned.Decomposition.Scope = append([]string(nil), value.Decomposition.Scope...)
	cloned.Decomposition.Exclusions = append([]string(nil), value.Decomposition.Exclusions...)
	cloned.Granularity.Warnings = append([]curriculum.GranularityWarning(nil), value.Granularity.Warnings...)
	for index := range cloned.Granularity.Warnings {
		cloned.Granularity.Warnings[index].ConceptIDs = append([]curriculum.ConceptID(nil), value.Granularity.Warnings[index].ConceptIDs...)
	}
	cloned.Granularity.ForcedSplit = append([]curriculum.ConceptID(nil), value.Granularity.ForcedSplit...)
	cloned.Granularity.SafeVisualGrouping = append([]curriculum.VisualConceptGroup(nil), value.Granularity.SafeVisualGrouping...)
	for index := range cloned.Granularity.SafeVisualGrouping {
		cloned.Granularity.SafeVisualGrouping[index].ConceptIDs = append([]curriculum.ConceptID(nil), value.Granularity.SafeVisualGrouping[index].ConceptIDs...)
	}
	cloned.Granularity.MergeDecisions = append([]curriculum.ConceptMergeDecision(nil), value.Granularity.MergeDecisions...)
	for index := range cloned.Granularity.MergeDecisions {
		cloned.Granularity.MergeDecisions[index].Reasons = append([]string(nil), value.Granularity.MergeDecisions[index].Reasons...)
	}
	cloned.Graph.ConceptIDs = append([]curriculum.ConceptID(nil), value.Graph.ConceptIDs...)
	cloned.Graph.Prerequisites = append([]curriculum.Prerequisite(nil), value.Graph.Prerequisites...)
	for index := range cloned.Graph.Prerequisites {
		cloned.Graph.Prerequisites[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), value.Graph.Prerequisites[index].EvidenceRefs...)
	}
	cloned.Graph.TopologicalOrder = append([]curriculum.ConceptID(nil), value.Graph.TopologicalOrder...)
	cloned.Graph.RootConceptIDs = append([]curriculum.ConceptID(nil), value.Graph.RootConceptIDs...)
	cloned.Graph.UnreachableConceptIDs = append([]curriculum.ConceptID(nil), value.Graph.UnreachableConceptIDs...)
	cloned.Graph.Components = append([]curriculum.KnowledgeGraphComponent(nil), value.Graph.Components...)
	for index := range cloned.Graph.Components {
		cloned.Graph.Components[index].ConceptIDs = append([]curriculum.ConceptID(nil), value.Graph.Components[index].ConceptIDs...)
		cloned.Graph.Components[index].RootConceptIDs = append([]curriculum.ConceptID(nil), value.Graph.Components[index].RootConceptIDs...)
		cloned.Graph.Components[index].FoundationalRootIDs = append([]curriculum.ConceptID(nil), value.Graph.Components[index].FoundationalRootIDs...)
		cloned.Graph.Components[index].CriticalPath.ConceptIDs = append([]curriculum.ConceptID(nil), value.Graph.Components[index].CriticalPath.ConceptIDs...)
	}
	cloned.Graph.CriticalPath.ConceptIDs = append([]curriculum.ConceptID(nil), value.Graph.CriticalPath.ConceptIDs...)
	cloned.Graph.ConsumptionPrerequisites = append([]curriculum.ConsumptionPrerequisite(nil), value.Graph.ConsumptionPrerequisites...)
	cloned.Vocabulary.Graph.Terms = append([]curriculum.VocabularyTerm(nil), value.Vocabulary.Graph.Terms...)
	for index := range cloned.Vocabulary.Graph.Terms {
		cloned.Vocabulary.Graph.Terms[index].UsedBy = append([]curriculum.ConceptID(nil), value.Vocabulary.Graph.Terms[index].UsedBy...)
		cloned.Vocabulary.Graph.Terms[index].Aliases = append([]string(nil), value.Vocabulary.Graph.Terms[index].Aliases...)
	}
	cloned.Vocabulary.BaselineTerms = append([]curriculum.DomainVocabularyBaselineTerm(nil), value.Vocabulary.BaselineTerms...)
	for index := range cloned.Vocabulary.BaselineTerms {
		cloned.Vocabulary.BaselineTerms[index].Aliases = append([]string(nil), value.Vocabulary.BaselineTerms[index].Aliases...)
	}
	cloned.Vocabulary.ResolvedUses = append([]curriculum.ResolvedVocabularyUse(nil), value.Vocabulary.ResolvedUses...)
	cloned.Hierarchy.Phases = append([]curriculum.Phase(nil), value.Hierarchy.Phases...)
	cloned.Hierarchy.Modules = append([]curriculum.Module(nil), value.Hierarchy.Modules...)
	cloned.Hierarchy.Lessons = append([]curriculum.LessonSpec(nil), value.Hierarchy.Lessons...)
	cloned.Hierarchy.Topics = append([]curriculum.TopicSpec(nil), value.Hierarchy.Topics...)
	for index := range cloned.Hierarchy.Topics {
		cloned.Hierarchy.Topics[index].ConceptIDs = append([]curriculum.ConceptID(nil), value.Hierarchy.Topics[index].ConceptIDs...)
	}
	cloned.Coverage.Dimensions = append([]curriculum.CoverageDimensionReport(nil), value.Coverage.Dimensions...)
	for index := range cloned.Coverage.Dimensions {
		dimension := &cloned.Coverage.Dimensions[index]
		dimension.Requirements = append([]curriculum.CoverageResult(nil), value.Coverage.Dimensions[index].Requirements...)
		for resultIndex := range dimension.Requirements {
			dimension.Requirements[resultIndex].Reasons = append([]string(nil), value.Coverage.Dimensions[index].Requirements[resultIndex].Reasons...)
		}
		dimension.MissingRequirementIDs = append([]curriculum.ID(nil), value.Coverage.Dimensions[index].MissingRequirementIDs...)
		dimension.PartialRequirementIDs = append([]curriculum.ID(nil), value.Coverage.Dimensions[index].PartialRequirementIDs...)
		dimension.CoveredRequirementIDs = append([]curriculum.ID(nil), value.Coverage.Dimensions[index].CoveredRequirementIDs...)
		dimension.Reasons = append([]string(nil), value.Coverage.Dimensions[index].Reasons...)
	}
	cloned.GapScan.Gaps = append([]curriculum.Gap(nil), value.GapScan.Gaps...)
	for index := range cloned.GapScan.Gaps {
		cloned.GapScan.Gaps[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), value.GapScan.Gaps[index].EvidenceRefs...)
	}
	cloned.DefinitionBeforeUse.Violations = append([]curriculum.DefinitionBeforeUseViolation(nil), value.DefinitionBeforeUse.Violations...)
	for index := range cloned.DefinitionBeforeUse.Violations {
		if value.DefinitionBeforeUse.Violations[index].SuggestedPrerequisite != nil {
			id := *value.DefinitionBeforeUse.Violations[index].SuggestedPrerequisite
			cloned.DefinitionBeforeUse.Violations[index].SuggestedPrerequisite = &id
		}
	}
	cloned.ZeroAssumption.Violations = append([]curriculum.ZeroAssumptionViolation(nil), value.ZeroAssumption.Violations...)
	for index := range cloned.ZeroAssumption.Violations {
		cloned.ZeroAssumption.Violations[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), value.ZeroAssumption.Violations[index].EvidenceRefs...)
	}
	cloned.Temporal.Classifications = append([]curriculum.TemporalClassification(nil), value.Temporal.Classifications...)
	for index := range cloned.Temporal.Classifications {
		cloned.Temporal.Classifications[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), value.Temporal.Classifications[index].EvidenceRefs...)
		cloned.Temporal.Classifications[index].Reasons = append([]string(nil), value.Temporal.Classifications[index].Reasons...)
	}
	cloned.Guidance.Classifications = append([]curriculum.GuidanceClassification(nil), value.Guidance.Classifications...)
	for index := range cloned.Guidance.Classifications {
		cloned.Guidance.Classifications[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), value.Guidance.Classifications[index].EvidenceRefs...)
	}
	cloned.BeginnerSimulation.Steps = append([]curriculum.BeginnerSimulationStep(nil), value.BeginnerSimulation.Steps...)
	for index := range cloned.BeginnerSimulation.Steps {
		cloned.BeginnerSimulation.Steps[index].IntroducedVocabulary = append([]string(nil), value.BeginnerSimulation.Steps[index].IntroducedVocabulary...)
		cloned.BeginnerSimulation.Steps[index].IntroducedToolIDs = append([]curriculum.ID(nil), value.BeginnerSimulation.Steps[index].IntroducedToolIDs...)
		cloned.BeginnerSimulation.Steps[index].ResolvedAssumptionIDs = append([]curriculum.ID(nil), value.BeginnerSimulation.Steps[index].ResolvedAssumptionIDs...)
	}
	cloned.BeginnerSimulation.Gaps = append([]curriculum.BeginnerGap(nil), value.BeginnerSimulation.Gaps...)
	cloned.BeginnerSimulation.IntroducedConceptIDs = append([]curriculum.ConceptID(nil), value.BeginnerSimulation.IntroducedConceptIDs...)
	cloned.BeginnerSimulation.IntroducedVocabulary = append([]string(nil), value.BeginnerSimulation.IntroducedVocabulary...)
	cloned.BeginnerSimulation.IntroducedToolIDs = append([]curriculum.ID(nil), value.BeginnerSimulation.IntroducedToolIDs...)
	cloned.BeginnerSimulation.ResolvedAssumptionIDs = append([]curriculum.ID(nil), value.BeginnerSimulation.ResolvedAssumptionIDs...)
	cloned.ExpertCoverage.ReviewedOutcomeIDs = append([]curriculum.ID(nil), value.ExpertCoverage.ReviewedOutcomeIDs...)
	cloned.ExpertCoverage.ReviewedCompetencyIDs = append([]curriculum.ID(nil), value.ExpertCoverage.ReviewedCompetencyIDs...)
	cloned.ExpertCoverage.Findings = append([]curriculum.ExpertCoverageFinding(nil), value.ExpertCoverage.Findings...)
	cloned.ExpertCoverage.AdvisorNotes = append([]string(nil), value.ExpertCoverage.AdvisorNotes...)
	cloned.Guidance.CurrentGuidanceFindings = append([]curriculum.CurrentGuidanceFinding(nil), value.Guidance.CurrentGuidanceFindings...)
	for index := range cloned.Guidance.CurrentGuidanceFindings {
		cloned.Guidance.CurrentGuidanceFindings[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), value.Guidance.CurrentGuidanceFindings[index].EvidenceRefs...)
	}
	if value.Review != nil {
		review := *value.Review
		review.Dimensions = append([]curriculum.CurriculumReviewDimensionResult(nil), value.Review.Dimensions...)
		for index := range review.Dimensions {
			review.Dimensions[index].Findings = append([]curriculum.CurriculumReviewFinding(nil), value.Review.Dimensions[index].Findings...)
		}
		review.AdvisorNotes = append([]string(nil), value.Review.AdvisorNotes...)
		cloned.Review = &review
	}
	return cloned
}
