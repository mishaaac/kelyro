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
	cloned.Tools = make([]curriculum.ToolRequirement, len(value.Tools))
	for index, tool := range value.Tools {
		cloned.Tools[index] = tool
		if tool.IntroducedAt != nil {
			introducedAt := *tool.IntroducedAt
			cloned.Tools[index].IntroducedAt = &introducedAt
		}
		cloned.Tools[index].Platforms = append([]string(nil), tool.Platforms...)
		cloned.Tools[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), tool.EvidenceRefs...)
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
	return cloned
}
