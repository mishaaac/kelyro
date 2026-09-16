package application

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type CurriculumHierarchyBuilderV1 struct{}

func NewCurriculumHierarchyBuilderV1() CurriculumHierarchyBuilderV1 {
	return CurriculumHierarchyBuilderV1{}
}

type CurriculumHierarchyBuilderV2 struct{}

func NewCurriculumHierarchyBuilderV2() CurriculumHierarchyBuilderV2 {
	return CurriculumHierarchyBuilderV2{}
}

type hierarchyPlacement struct {
	concept    curriculum.Concept
	areaID     curriculum.ID
	area       string
	competency curriculum.ID
	context    string
	position   int
}

type hierarchyGroupKey struct {
	difficulty curriculum.Difficulty
	areaID     string
	competency string
	context    string
}

func (CurriculumHierarchyBuilderV1) Build(ctx context.Context, request CurriculumHierarchyBuildRequest) (curriculum.CurriculumHierarchy, error) {
	return buildCurriculumHierarchy(ctx, request, curriculum.CurriculumHierarchyBuilderVersionV1, false)
}

func (CurriculumHierarchyBuilderV2) Build(ctx context.Context, request CurriculumHierarchyBuildRequest) (curriculum.CurriculumHierarchy, error) {
	return buildCurriculumHierarchy(ctx, request, curriculum.CurriculumHierarchyBuilderVersionV2, true)
}

func buildCurriculumHierarchy(ctx context.Context, request CurriculumHierarchyBuildRequest, algorithmVersion string, prerequisiteOrder bool) (curriculum.CurriculumHierarchy, error) {
	const operation = "build curriculum hierarchy"
	if err := ctx.Err(); err != nil {
		return curriculum.CurriculumHierarchy{}, ExternalError(operation, err)
	}
	if err := request.Competencies.Validate(); err != nil {
		return curriculum.CurriculumHierarchy{}, Invalid(operation, err)
	}
	if err := request.Graph.Validate(); err != nil {
		return curriculum.CurriculumHierarchy{}, Invalid(operation, err)
	}

	concepts := make(map[curriculum.ConceptID]curriculum.Concept, len(request.Concepts))
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return curriculum.CurriculumHierarchy{}, Invalid(operation, err)
		}
		if concept.Atomicity != curriculum.AtomicityAtomic {
			return curriculum.CurriculumHierarchy{}, Invalid(operation, fmt.Errorf("hierarchy concept %q is not atomic", concept.ID))
		}
		if _, exists := concepts[concept.ID]; exists {
			return curriculum.CurriculumHierarchy{}, Invalid(operation, fmt.Errorf("duplicate hierarchy concept %q", concept.ID))
		}
		concepts[concept.ID] = cloneConcept(concept)
	}
	if len(concepts) != len(request.Graph.ConceptIDs) {
		return curriculum.CurriculumHierarchy{}, Invalid(operation, fmt.Errorf("hierarchy concepts do not match knowledge graph"))
	}
	position := make(map[curriculum.ConceptID]int, len(request.Graph.TopologicalOrder))
	for index, id := range request.Graph.TopologicalOrder {
		if _, exists := concepts[id]; !exists {
			return curriculum.CurriculumHierarchy{}, Invalid(operation, fmt.Errorf("knowledge graph references missing hierarchy concept %q", id))
		}
		position[id] = index
	}

	contexts := make(map[curriculum.ConceptID]string, len(request.PracticeContext))
	for _, assignment := range request.PracticeContext {
		if err := assignment.Validate(); err != nil {
			return curriculum.CurriculumHierarchy{}, Invalid(operation, err)
		}
		if _, exists := concepts[assignment.ConceptID]; !exists {
			return curriculum.CurriculumHierarchy{}, Invalid(operation, fmt.Errorf("practice context references missing concept %q", assignment.ConceptID))
		}
		if _, exists := contexts[assignment.ConceptID]; exists {
			return curriculum.CurriculumHierarchy{}, Invalid(operation, fmt.Errorf("duplicate practice context for concept %q", assignment.ConceptID))
		}
		contexts[assignment.ConceptID] = strings.Join(strings.Fields(assignment.Context), " ")
	}

	competencies := append([]curriculum.Competency(nil), request.Competencies.Competencies...)
	sort.Slice(competencies, func(i, j int) bool {
		if competencies[i].AreaID != competencies[j].AreaID {
			return competencies[i].AreaID.String() < competencies[j].AreaID.String()
		}
		return competencies[i].ID.String() < competencies[j].ID.String()
	})
	assignment := make(map[curriculum.ConceptID]curriculum.Competency, len(concepts))
	for _, competency := range competencies {
		for _, id := range competency.ConceptRefs {
			if _, exists := concepts[id]; !exists {
				return curriculum.CurriculumHierarchy{}, Invalid(operation, fmt.Errorf("competency %q references missing hierarchy concept %q", competency.ID, id))
			}
			if _, exists := assignment[id]; !exists {
				assignment[id] = competency
			}
		}
	}
	unassignedArea, _ := curriculum.NewID("area.unassigned")
	unassignedCompetency, _ := curriculum.NewID("competency.unassigned")
	placements := make([]hierarchyPlacement, 0, len(concepts))
	for _, id := range request.Graph.TopologicalOrder {
		concept := concepts[id]
		competency, assigned := assignment[id]
		if !assigned {
			competency = curriculum.Competency{ID: unassignedCompetency, AreaID: unassignedArea, Area: "Foundations and supporting concepts"}
		}
		practiceContext := contexts[id]
		if practiceContext == "" {
			practiceContext = "Concepts and mental models"
		}
		placements = append(placements, hierarchyPlacement{concept: concept, areaID: competency.AreaID, area: competency.Area, competency: competency.ID, context: practiceContext, position: position[id]})
	}

	result := buildHierarchyProjection(placements, algorithmVersion, prerequisiteOrder)
	if err := result.Validate(request.Concepts); err != nil {
		return curriculum.CurriculumHierarchy{}, Invalid(operation, err)
	}
	return result, nil
}

func buildHierarchyProjection(placements []hierarchyPlacement, algorithmVersion string, prerequisiteOrder bool) curriculum.CurriculumHierarchy {
	result := curriculum.CurriculumHierarchy{AlgorithmVersion: algorithmVersion}
	difficulties := make(map[curriculum.Difficulty]struct{})
	areas := make(map[curriculum.Difficulty]map[string]string)
	lessons := make(map[string]map[string]struct{})
	topics := make(map[string]map[string][]hierarchyPlacement)
	areaPositions := make(map[curriculum.Difficulty]map[string]int)
	lessonPositions := make(map[string]map[string]int)
	topicPositions := make(map[string]map[string]int)
	for _, placement := range placements {
		difficulties[placement.concept.Difficulty] = struct{}{}
		if areas[placement.concept.Difficulty] == nil {
			areas[placement.concept.Difficulty] = make(map[string]string)
		}
		areas[placement.concept.Difficulty][placement.areaID.String()] = placement.area
		if areaPositions[placement.concept.Difficulty] == nil {
			areaPositions[placement.concept.Difficulty] = make(map[string]int)
		}
		retainEarliestPosition(areaPositions[placement.concept.Difficulty], placement.areaID.String(), placement.position)
		moduleKey := fmt.Sprintf("%d\x00%s", placement.concept.Difficulty, placement.areaID.String())
		if lessons[moduleKey] == nil {
			lessons[moduleKey] = make(map[string]struct{})
		}
		lessons[moduleKey][placement.competency.String()] = struct{}{}
		if lessonPositions[moduleKey] == nil {
			lessonPositions[moduleKey] = make(map[string]int)
		}
		retainEarliestPosition(lessonPositions[moduleKey], placement.competency.String(), placement.position)
		lessonKey := moduleKey + "\x00" + placement.competency.String()
		if topics[lessonKey] == nil {
			topics[lessonKey] = make(map[string][]hierarchyPlacement)
		}
		topics[lessonKey][placement.context] = append(topics[lessonKey][placement.context], placement)
		if topicPositions[lessonKey] == nil {
			topicPositions[lessonKey] = make(map[string]int)
		}
		retainEarliestPosition(topicPositions[lessonKey], placement.context, placement.position)
	}

	difficultyOrder := make([]int, 0, len(difficulties))
	for difficulty := range difficulties {
		difficultyOrder = append(difficultyOrder, int(difficulty))
	}
	sort.Ints(difficultyOrder)
	for phaseOrder, rawDifficulty := range difficultyOrder {
		difficulty := curriculum.Difficulty(rawDifficulty)
		phaseID := derivedHierarchyID("phase", fmt.Sprintf("%d", rawDifficulty))
		label := difficultyLabel(difficulty)
		result.Phases = append(result.Phases, curriculum.Phase{ID: phaseID, Title: label, Description: label + " concepts grouped for curriculum navigation.", Order: phaseOrder})

		areaIDs := sortedStringKeys(areas[difficulty])
		if prerequisiteOrder {
			sortByPosition(areaIDs, areaPositions[difficulty])
		}
		for moduleOrder, rawAreaID := range areaIDs {
			moduleID := derivedHierarchyID("module", fmt.Sprintf("%d\x00%s", rawDifficulty, rawAreaID))
			areaTitle := areas[difficulty][rawAreaID]
			result.Modules = append(result.Modules, curriculum.Module{ID: moduleID, PhaseID: phaseID, Title: areaTitle, Description: "Competency area: " + areaTitle + ".", Order: moduleOrder})
			moduleKey := fmt.Sprintf("%d\x00%s", rawDifficulty, rawAreaID)
			competencyIDs := sortedStringSet(lessons[moduleKey])
			if prerequisiteOrder {
				sortByPosition(competencyIDs, lessonPositions[moduleKey])
			}
			for lessonOrder, rawCompetencyID := range competencyIDs {
				lessonID := derivedHierarchyID("lesson", moduleKey+"\x00"+rawCompetencyID)
				result.Lessons = append(result.Lessons, curriculum.LessonSpec{ID: lessonID, ModuleID: moduleID, Title: "Competency " + rawCompetencyID, Description: "Concepts supporting competency " + rawCompetencyID + ".", Order: lessonOrder})
				lessonKey := moduleKey + "\x00" + rawCompetencyID
				contextNames := sortedPlacementContexts(topics[lessonKey])
				if prerequisiteOrder {
					sortByPosition(contextNames, topicPositions[lessonKey])
				}
				for topicOrder, contextName := range contextNames {
					values := topics[lessonKey][contextName]
					sort.Slice(values, func(i, j int) bool {
						if values[i].position != values[j].position {
							return values[i].position < values[j].position
						}
						return values[i].concept.ID.String() < values[j].concept.ID.String()
					})
					conceptIDs := make([]curriculum.ConceptID, 0, len(values))
					for _, value := range values {
						conceptIDs = append(conceptIDs, value.concept.ID)
					}
					result.Topics = append(result.Topics, curriculum.TopicSpec{ID: derivedHierarchyID("topic", lessonKey+"\x00"+contextName), LessonID: lessonID, Title: contextName, Description: "Cohesive concepts for " + contextName + ".", Order: topicOrder, ConceptIDs: conceptIDs})
				}
			}
		}
	}
	return result
}

func derivedHierarchyID(kind, seed string) curriculum.ID {
	digest := sha256.Sum256([]byte(kind + "\x00" + seed))
	id, _ := curriculum.NewID(fmt.Sprintf("%s.%x", kind, digest[:8]))
	return id
}

func difficultyLabel(difficulty curriculum.Difficulty) string {
	switch difficulty {
	case curriculum.DifficultyIntroductory:
		return "Introductory"
	case curriculum.DifficultyFoundational:
		return "Foundational"
	case curriculum.DifficultyIntermediate:
		return "Intermediate"
	case curriculum.DifficultyAdvanced:
		return "Advanced"
	case curriculum.DifficultyExpert:
		return "Expert"
	default:
		panic("validated difficulty is unsupported")
	}
}

func sortedStringKeys(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sortedStringSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sortedPlacementContexts(values map[string][]hierarchyPlacement) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func retainEarliestPosition(values map[string]int, key string, position int) {
	current, exists := values[key]
	if !exists || position < current {
		values[key] = position
	}
}

func sortByPosition(values []string, positions map[string]int) {
	sort.Slice(values, func(i, j int) bool {
		if positions[values[i]] != positions[values[j]] {
			return positions[values[i]] < positions[values[j]]
		}
		return values[i] < values[j]
	})
}

var _ CurriculumHierarchyBuilderService = CurriculumHierarchyBuilderV1{}
var _ CurriculumHierarchyBuilderService = CurriculumHierarchyBuilderV2{}
