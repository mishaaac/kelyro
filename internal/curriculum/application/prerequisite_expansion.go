package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type PrerequisiteExpansionV1 struct{}

func NewPrerequisiteExpansionV1() PrerequisiteExpansionV1 {
	return PrerequisiteExpansionV1{}
}

type prerequisiteEdgeKey struct {
	concept  curriculum.ConceptID
	required curriculum.ConceptID
	kind     curriculum.PrerequisiteKind
}

func (PrerequisiteExpansionV1) Expand(ctx context.Context, request PrerequisiteExpansionRequest) (curriculum.PrerequisiteExpansionResult, error) {
	const operation = "expand concept prerequisites"
	if err := ctx.Err(); err != nil {
		return curriculum.PrerequisiteExpansionResult{}, ExternalError(operation, err)
	}
	knownEvidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, err)
	}

	current, err := indexExpansionConcepts(request.Concepts, nil, knownEvidence, false)
	if err != nil {
		return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, err)
	}
	if len(current) == 0 {
		return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, fmt.Errorf("prerequisite expansion has no current concepts"))
	}
	available, err := indexExpansionConcepts(request.AvailableConcepts, current, knownEvidence, true)
	if err != nil {
		return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, err)
	}
	universe := make(map[curriculum.ConceptID]curriculum.Concept, len(current)+len(available))
	for id, concept := range current {
		universe[id] = concept
	}
	for id, concept := range available {
		universe[id] = concept
	}

	semantics := cloneAndSortPrerequisiteSemantics(request.Semantics)
	semanticsByConcept := make(map[curriculum.ConceptID][]curriculum.ConceptPrerequisiteSemantic)
	seenSemantics := make(map[prerequisiteEdgeKey]struct{}, len(semantics))
	for _, semantic := range semantics {
		if err := semantic.Validate(); err != nil {
			return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, err)
		}
		if _, exists := universe[semantic.ConceptID]; !exists {
			return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, fmt.Errorf("prerequisite semantic references unavailable dependent concept %q", semantic.ConceptID))
		}
		if err := requireKnownEvidence(semantic.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, err)
		}
		key := prerequisiteEdgeKey{concept: semantic.ConceptID, required: semantic.RequiredConceptID, kind: semantic.Kind}
		if _, exists := seenSemantics[key]; exists {
			return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, fmt.Errorf("duplicate prerequisite semantic from %q to %q", semantic.ConceptID, semantic.RequiredConceptID))
		}
		seenSemantics[key] = struct{}{}
		semanticsByConcept[semantic.ConceptID] = append(semanticsByConcept[semantic.ConceptID], semantic)
	}

	result := curriculum.PrerequisiteExpansionResult{AlgorithmVersion: curriculum.PrerequisiteExpansionVersionV1}
	adjacency := make(map[curriculum.ConceptID]map[curriculum.ConceptID]struct{})
	edges := make(map[prerequisiteEdgeKey]curriculum.Prerequisite)
	existing := cloneAndSortPrerequisites(request.Prerequisites)
	for _, prerequisite := range existing {
		if err := prerequisite.Validate(); err != nil {
			return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, err)
		}
		if _, exists := current[prerequisite.ConceptID]; !exists {
			return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, fmt.Errorf("existing prerequisite references missing concept %q", prerequisite.ConceptID))
		}
		if _, exists := current[prerequisite.RequiredConceptID]; !exists {
			return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, fmt.Errorf("existing prerequisite references missing required concept %q", prerequisite.RequiredConceptID))
		}
		if len(prerequisite.EvidenceRefs) == 0 {
			return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, fmt.Errorf("existing prerequisite from %q to %q has no evidence", prerequisite.ConceptID, prerequisite.RequiredConceptID))
		}
		if err := requireKnownEvidence(prerequisite.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, err)
		}
		key := prerequisiteEdgeKey{concept: prerequisite.ConceptID, required: prerequisite.RequiredConceptID, kind: prerequisite.Kind}
		if _, exists := edges[key]; exists {
			return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, fmt.Errorf("duplicate existing prerequisite from %q to %q", prerequisite.ConceptID, prerequisite.RequiredConceptID))
		}
		if pathExists(adjacency, prerequisite.RequiredConceptID, prerequisite.ConceptID) {
			return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, fmt.Errorf("existing prerequisites contain a cycle at %q", prerequisite.ConceptID))
		}
		edges[key] = prerequisite
		addAdjacency(adjacency, prerequisite.ConceptID, prerequisite.RequiredConceptID)
	}

	added := make(map[curriculum.ConceptID]curriculum.Concept)
	visited := make(map[curriculum.ConceptID]bool)
	var visit func(curriculum.ConceptID) error
	visit = func(conceptID curriculum.ConceptID) error {
		if visited[conceptID] {
			return nil
		}
		visited[conceptID] = true
		if err := ctx.Err(); err != nil {
			return err
		}
		semanticsForConcept := semanticsByConcept[conceptID]
		if len(semanticsForConcept) == 0 && len(adjacency[conceptID]) == 0 && !universe[conceptID].Foundational {
			result.UnresolvedGaps = append(result.UnresolvedGaps, curriculum.PrerequisiteExpansionGap{
				ConceptID: conceptID, Code: curriculum.PrerequisiteGapMissingRoot,
				Reason:       "non-foundational concept has no declared prerequisite boundary",
				EvidenceRefs: append([]curriculum.EvidenceRef(nil), universe[conceptID].EvidenceRefs...),
			})
			result.Reasons = append(result.Reasons, "unresolved:missing_root_boundary:"+conceptID.String())
		}
		for _, semantic := range semanticsForConcept {
			key := prerequisiteEdgeKey{concept: semantic.ConceptID, required: semantic.RequiredConceptID, kind: semantic.Kind}
			if _, exists := edges[key]; exists {
				if err := visit(semantic.RequiredConceptID); err != nil {
					return err
				}
				continue
			}
			required, exists := universe[semantic.RequiredConceptID]
			if !exists {
				result.UnresolvedGaps = append(result.UnresolvedGaps, prerequisiteGap(semantic, curriculum.PrerequisiteGapConceptUnavailable, "required concept is not available with verified evidence"))
				result.Reasons = append(result.Reasons, expansionGapReason(curriculum.PrerequisiteGapConceptUnavailable, semantic))
				continue
			}
			if pathExists(adjacency, semantic.RequiredConceptID, semantic.ConceptID) {
				result.UnresolvedGaps = append(result.UnresolvedGaps, prerequisiteGap(semantic, curriculum.PrerequisiteGapCyclePrevented, "prerequisite edge would create a cycle"))
				result.Reasons = append(result.Reasons, expansionGapReason(curriculum.PrerequisiteGapCyclePrevented, semantic))
				continue
			}
			if _, exists := current[required.ID]; !exists {
				if _, exists := added[required.ID]; !exists {
					added[required.ID] = cloneConcept(required)
					result.Reasons = append(result.Reasons, "concept_added:"+required.ID.String()+":required_by:"+semantic.ConceptID.String())
				}
			}
			edge := curriculum.Prerequisite{
				ConceptID: semantic.ConceptID, RequiredConceptID: semantic.RequiredConceptID,
				Kind: semantic.Kind, EvidenceRefs: append([]curriculum.EvidenceRef(nil), semantic.EvidenceRefs...),
			}
			edges[key] = edge
			addAdjacency(adjacency, semantic.ConceptID, semantic.RequiredConceptID)
			result.Reasons = append(result.Reasons, "edge_added:"+semantic.ConceptID.String()+":"+semantic.RequiredConceptID.String()+":"+string(semantic.Kind)+":"+semantic.Reason)
			if err := visit(semantic.RequiredConceptID); err != nil {
				return err
			}
		}
		return nil
	}
	for _, conceptID := range sortedConceptMapIDs(current) {
		if err := visit(conceptID); err != nil {
			return curriculum.PrerequisiteExpansionResult{}, ExternalError(operation, err)
		}
	}

	for _, conceptID := range sortedConceptMapIDs(added) {
		result.AddedConcepts = append(result.AddedConcepts, cloneConcept(added[conceptID]))
	}
	result.ExpandedPrerequisites = sortedPrerequisiteMap(edges)
	sortExpansionGaps(result.UnresolvedGaps)
	sort.Strings(result.Reasons)
	result.Reasons = uniqueSortedStrings(result.Reasons)
	if len(result.Reasons) == 0 {
		result.Reasons = []string{"no_expansion_required"}
	}
	if err := result.Validate(); err != nil {
		return curriculum.PrerequisiteExpansionResult{}, Invalid(operation, err)
	}
	return result, nil
}

func indexExpansionConcepts(values []curriculum.Concept, existing map[curriculum.ConceptID]curriculum.Concept, evidence map[evidenceKey]struct{}, requireEvidence bool) (map[curriculum.ConceptID]curriculum.Concept, error) {
	result := make(map[curriculum.ConceptID]curriculum.Concept, len(values))
	for _, concept := range values {
		if err := concept.Validate(); err != nil {
			return nil, err
		}
		if concept.Atomicity != curriculum.AtomicityAtomic {
			return nil, fmt.Errorf("prerequisite concept %q is not atomic", concept.ID)
		}
		if _, exists := result[concept.ID]; exists {
			return nil, fmt.Errorf("duplicate prerequisite concept %q", concept.ID)
		}
		if _, exists := existing[concept.ID]; exists {
			return nil, fmt.Errorf("available prerequisite concept %q already exists", concept.ID)
		}
		if requireEvidence && len(concept.EvidenceRefs) == 0 {
			return nil, fmt.Errorf("available prerequisite concept %q has no evidence", concept.ID)
		}
		if err := requireKnownEvidence(concept.EvidenceRefs, evidence); err != nil {
			return nil, fmt.Errorf("prerequisite concept %q: %w", concept.ID, err)
		}
		result[concept.ID] = cloneConcept(concept)
	}
	return result, nil
}

func cloneConcept(concept curriculum.Concept) curriculum.Concept {
	concept.EvidenceRefs = append([]curriculum.EvidenceRef(nil), concept.EvidenceRefs...)
	return concept
}

func cloneAndSortPrerequisites(values []curriculum.Prerequisite) []curriculum.Prerequisite {
	result := append([]curriculum.Prerequisite(nil), values...)
	for index := range result {
		result[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), result[index].EvidenceRefs...)
		sortEvidenceRefs(result[index].EvidenceRefs)
	}
	sort.Slice(result, func(i, j int) bool { return prerequisiteLess(result[i], result[j]) })
	return result
}

func sortedPrerequisiteMap(values map[prerequisiteEdgeKey]curriculum.Prerequisite) []curriculum.Prerequisite {
	result := make([]curriculum.Prerequisite, 0, len(values))
	for _, edge := range values {
		edge.EvidenceRefs = append([]curriculum.EvidenceRef(nil), edge.EvidenceRefs...)
		sortEvidenceRefs(edge.EvidenceRefs)
		result = append(result, edge)
	}
	sort.Slice(result, func(i, j int) bool { return prerequisiteLess(result[i], result[j]) })
	return result
}

func prerequisiteLess(left, right curriculum.Prerequisite) bool {
	if left.ConceptID != right.ConceptID {
		return left.ConceptID.String() < right.ConceptID.String()
	}
	if left.RequiredConceptID != right.RequiredConceptID {
		return left.RequiredConceptID.String() < right.RequiredConceptID.String()
	}
	return left.Kind < right.Kind
}

func addAdjacency(graph map[curriculum.ConceptID]map[curriculum.ConceptID]struct{}, concept, required curriculum.ConceptID) {
	if graph[concept] == nil {
		graph[concept] = make(map[curriculum.ConceptID]struct{})
	}
	graph[concept][required] = struct{}{}
}

func pathExists(graph map[curriculum.ConceptID]map[curriculum.ConceptID]struct{}, start, target curriculum.ConceptID) bool {
	if start == target {
		return true
	}
	seen := make(map[curriculum.ConceptID]struct{})
	stack := []curriculum.ConceptID{start}
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if _, exists := seen[current]; exists {
			continue
		}
		seen[current] = struct{}{}
		for next := range graph[current] {
			if next == target {
				return true
			}
			stack = append(stack, next)
		}
	}
	return false
}

func prerequisiteGap(semantic curriculum.ConceptPrerequisiteSemantic, code curriculum.PrerequisiteExpansionGapCode, reason string) curriculum.PrerequisiteExpansionGap {
	required := semantic.RequiredConceptID
	kind := semantic.Kind
	return curriculum.PrerequisiteExpansionGap{
		ConceptID: semantic.ConceptID, RequiredConceptID: &required, Kind: &kind,
		Code: code, Reason: reason, EvidenceRefs: append([]curriculum.EvidenceRef(nil), semantic.EvidenceRefs...),
	}
}

func expansionGapReason(code curriculum.PrerequisiteExpansionGapCode, semantic curriculum.ConceptPrerequisiteSemantic) string {
	return "unresolved:" + string(code) + ":" + semantic.ConceptID.String() + ":" + semantic.RequiredConceptID.String() + ":" + string(semantic.Kind)
}

func sortExpansionGaps(values []curriculum.PrerequisiteExpansionGap) {
	sort.Slice(values, func(i, j int) bool {
		left, right := values[i], values[j]
		if left.ConceptID != right.ConceptID {
			return left.ConceptID.String() < right.ConceptID.String()
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		leftRequired, rightRequired := "", ""
		if left.RequiredConceptID != nil {
			leftRequired = left.RequiredConceptID.String()
		}
		if right.RequiredConceptID != nil {
			rightRequired = right.RequiredConceptID.String()
		}
		if leftRequired != rightRequired {
			return leftRequired < rightRequired
		}
		leftKind, rightKind := "", ""
		if left.Kind != nil {
			leftKind = string(*left.Kind)
		}
		if right.Kind != nil {
			rightKind = string(*right.Kind)
		}
		return leftKind < rightKind
	})
}

func uniqueSortedStrings(values []string) []string {
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

var _ PrerequisiteExpansionService = PrerequisiteExpansionV1{}
