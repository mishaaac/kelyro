package application

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestEnvironmentDoctorPlannerV1ClassifiesToolsForCurrentCurriculumPosition(t *testing.T) {
	t.Parallel()
	concepts, graph, hierarchy, environment := environmentDoctorFixture(t)
	request := EnvironmentDoctorPlanRequest{
		Environment: environment, Concepts: concepts, Graph: graph, Hierarchy: hierarchy,
		CurrentConceptID: concepts[0].ID, Platform: curriculum.EnvironmentPlatformLinux,
	}

	plan, err := NewEnvironmentDoctorPlannerV1().Plan(context.Background(), request)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	wantTiming := map[string]curriculum.EnvironmentToolTiming{
		"docker":     curriculum.EnvironmentToolNotNeededYet,
		"git":        curriculum.EnvironmentToolCurrent,
		"go":         curriculum.EnvironmentToolCurrent,
		"postgresql": curriculum.EnvironmentToolFuture,
	}
	if len(plan.Tools) != len(wantTiming) {
		t.Fatalf("plan tools = %+v", plan.Tools)
	}
	for _, tool := range plan.Tools {
		if tool.Timing != wantTiming[tool.ToolID.String()] {
			t.Errorf("tool %q timing = %q, want %q", tool.ToolID, tool.Timing, wantTiming[tool.ToolID.String()])
		}
		if tool.OfficialSourceName == "" || !strings.HasPrefix(tool.OfficialURL, "https://") || tool.InstallInstructions == "" {
			t.Errorf("tool %q lacks official installation metadata: %+v", tool.ToolID, tool)
		}
	}

	reordered := request
	reordered.Environment.Tools = append([]curriculum.ToolRequirement(nil), environment.Tools...)
	for left, right := 0, len(reordered.Environment.Tools)-1; left < right; left, right = left+1, right-1 {
		reordered.Environment.Tools[left], reordered.Environment.Tools[right] = reordered.Environment.Tools[right], reordered.Environment.Tools[left]
	}
	repeated, err := NewEnvironmentDoctorPlannerV1().Plan(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(plan, repeated) {
		t.Fatalf("reordered plan differs: %v\nfirst=%+v\nsecond=%+v", err, plan, repeated)
	}
}

func TestEnvironmentDoctorPlannerV1MakesLaterToolCurrentAtNeededConcept(t *testing.T) {
	t.Parallel()
	concepts, graph, hierarchy, environment := environmentDoctorFixture(t)
	plan, err := NewEnvironmentDoctorPlannerV1().Plan(context.Background(), EnvironmentDoctorPlanRequest{
		Environment: environment, Concepts: concepts, Graph: graph, Hierarchy: hierarchy,
		CurrentConceptID: concepts[2].ID, Platform: curriculum.EnvironmentPlatformLinux,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range plan.Tools {
		if tool.Timing != curriculum.EnvironmentToolCurrent {
			t.Errorf("tool %q remains %q at final concept", tool.ToolID, tool.Timing)
		}
	}
}

func TestEnvironmentDoctorPlannerV1RejectsToolNeededBeforeItIsIntroduced(t *testing.T) {
	t.Parallel()
	concepts, graph, hierarchy, environment := environmentDoctorFixture(t)
	late := concepts[2].ID
	early := concepts[1].ID
	environment.Tools[0].IntroducedAt = &late
	environment.Tools[0].WhenNeeded = &early
	_, err := NewEnvironmentDoctorPlannerV1().Plan(context.Background(), EnvironmentDoctorPlanRequest{
		Environment: environment, Concepts: concepts, Graph: graph, Hierarchy: hierarchy,
		CurrentConceptID: concepts[0].ID, Platform: curriculum.EnvironmentPlatformLinux,
	})
	if err == nil || !strings.Contains(err.Error(), "needed before its introduction") {
		t.Fatalf("Plan() error = %v", err)
	}
}

func environmentDoctorFixture(t *testing.T) ([]curriculum.Concept, curriculum.KnowledgeGraphCompilation, curriculum.CurriculumHierarchy, curriculum.EnvironmentPack) {
	t.Helper()
	root := graphConcept(t, "concept.foundations", true)
	middle := graphConcept(t, "concept.containers", false)
	leaf := graphConcept(t, "concept.persistence", false)
	concepts := []curriculum.Concept{root, middle, leaf}
	graph := compileAuditGraph(t, concepts, []curriculum.Prerequisite{
		graphPrerequisite(middle, root, curriculum.PrerequisiteHard),
		graphPrerequisite(leaf, middle, curriculum.PrerequisiteHard),
	})
	phaseIDs := []curriculum.ID{curriculumID(t, "phase.foundation"), curriculumID(t, "phase.services"), curriculumID(t, "phase.data")}
	moduleIDs := []curriculum.ID{curriculumID(t, "module.foundation"), curriculumID(t, "module.containers"), curriculumID(t, "module.persistence")}
	lessonIDs := []curriculum.ID{curriculumID(t, "lesson.foundation"), curriculumID(t, "lesson.containers"), curriculumID(t, "lesson.persistence")}
	topicIDs := []curriculum.ID{curriculumID(t, "topic.foundation"), curriculumID(t, "topic.containers"), curriculumID(t, "topic.persistence")}
	hierarchy := curriculum.CurriculumHierarchy{AlgorithmVersion: curriculum.CurriculumHierarchyBuilderVersionV1}
	for index, title := range []string{"Foundations", "Services", "Data"} {
		hierarchy.Phases = append(hierarchy.Phases, curriculum.Phase{ID: phaseIDs[index], Title: title, Description: title + " phase", Order: index})
		hierarchy.Modules = append(hierarchy.Modules, curriculum.Module{ID: moduleIDs[index], PhaseID: phaseIDs[index], Title: []string{"Tooling", "Containers", "Persistence"}[index], Description: title + " module", Order: 0})
		hierarchy.Lessons = append(hierarchy.Lessons, curriculum.LessonSpec{ID: lessonIDs[index], ModuleID: moduleIDs[index], Title: title + " lesson", Description: title + " lesson description", Order: 0})
		hierarchy.Topics = append(hierarchy.Topics, curriculum.TopicSpec{ID: topicIDs[index], LessonID: lessonIDs[index], Title: title + " topic", Description: title + " topic description", Order: 0, ConceptIDs: []curriculum.ConceptID{concepts[index].ID}})
	}

	version, err := curriculum.NewPackVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	evidence := curriculum.EvidenceRef{BundleID: curriculumID(t, "bundle.tools"), ClaimID: curriculumID(t, "claim.tools")}
	toolSpecs := []struct {
		id, name, version  string
		level              curriculum.ToolRequirementLevel
		introduced, needed *curriculum.ConceptID
	}{
		{"docker", "Docker", "28.0.0", curriculum.ToolRequired, &middle.ID, &leaf.ID},
		{"git", "Git", "2.49.0", curriculum.ToolRecommended, &root.ID, &root.ID},
		{"go", "Go", "1.24.0", curriculum.ToolRequired, &root.ID, &root.ID},
		{"postgresql", "PostgreSQL", "17.0.0", curriculum.ToolRequired, &root.ID, &leaf.ID},
	}
	environment := curriculum.EnvironmentPack{ID: curriculumID(t, "environment.backend"), Version: version, SchemaVersion: curriculum.EnvironmentPackSchemaVersionV1, SupportedPlatforms: []string{curriculum.EnvironmentPlatformLinux}}
	for _, spec := range toolSpecs {
		toolID := curriculumID(t, spec.id)
		guidanceID := curriculumID(t, "guidance."+spec.id+".linux")
		environment.Tools = append(environment.Tools, curriculum.ToolRequirement{
			ID: toolID, DisplayName: spec.name, Purpose: "Use " + spec.name + " for the learning environment.", MinimumVersion: spec.version,
			Level: spec.level, IntroducedAt: spec.introduced, WhenNeeded: spec.needed,
			Platforms: []string{curriculum.EnvironmentPlatformLinux}, InstallGuidanceRefs: []curriculum.ID{guidanceID}, EvidenceRefs: []curriculum.EvidenceRef{evidence},
		})
		environment.InstallGuidance = append(environment.InstallGuidance, curriculum.ToolInstallGuidance{
			ID: guidanceID, Platform: curriculum.EnvironmentPlatformLinux, SourceName: spec.name + " project",
			OfficialURL: "https://example.com/" + spec.id, Instructions: "Follow the official Linux instructions.", EvidenceRefs: []curriculum.EvidenceRef{evidence},
		})
	}
	return concepts, graph, hierarchy, environment
}
