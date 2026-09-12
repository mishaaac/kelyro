package curriculum

import (
	"fmt"
	"sort"
)

const PrerequisiteExpansionVersionV1 = "prerequisite-expansion-v1"

type PrerequisiteExpansionGapCode string

const (
	PrerequisiteGapMissingRoot        PrerequisiteExpansionGapCode = "missing_root_boundary"
	PrerequisiteGapConceptUnavailable PrerequisiteExpansionGapCode = "prerequisite_concept_unavailable"
	PrerequisiteGapCyclePrevented     PrerequisiteExpansionGapCode = "cycle_prevented"
)

func (code PrerequisiteExpansionGapCode) Validate() error {
	switch code {
	case PrerequisiteGapMissingRoot, PrerequisiteGapConceptUnavailable, PrerequisiteGapCyclePrevented:
		return nil
	default:
		return fmt.Errorf("invalid prerequisite expansion gap code %q", code)
	}
}

type PrerequisiteExpansionGap struct {
	ConceptID         ConceptID
	RequiredConceptID *ConceptID
	Kind              *PrerequisiteKind
	Code              PrerequisiteExpansionGapCode
	Reason            string
	EvidenceRefs      []EvidenceRef
}

func (gap PrerequisiteExpansionGap) Validate() error {
	if err := gap.ConceptID.Validate(); err != nil {
		return fmt.Errorf("prerequisite expansion gap concept: %w", err)
	}
	if err := gap.Code.Validate(); err != nil {
		return err
	}
	if err := requireText("prerequisite expansion gap reason", gap.Reason); err != nil {
		return err
	}
	if err := validateEvidenceRefs("prerequisite expansion gap evidence", gap.EvidenceRefs); err != nil {
		return err
	}
	if gap.Code == PrerequisiteGapMissingRoot {
		if gap.RequiredConceptID != nil || gap.Kind != nil {
			return fmt.Errorf("missing-root gap cannot identify a prerequisite edge")
		}
		return nil
	}
	if gap.RequiredConceptID == nil || gap.Kind == nil {
		return fmt.Errorf("prerequisite edge gap requires required concept and kind")
	}
	if err := gap.RequiredConceptID.Validate(); err != nil {
		return fmt.Errorf("prerequisite expansion gap required concept: %w", err)
	}
	if *gap.RequiredConceptID == gap.ConceptID {
		return fmt.Errorf("prerequisite expansion gap cannot be a self-reference")
	}
	return gap.Kind.Validate()
}

type PrerequisiteExpansionResult struct {
	AddedConcepts         []Concept
	ExpandedPrerequisites []Prerequisite
	UnresolvedGaps        []PrerequisiteExpansionGap
	Reasons               []string
	AlgorithmVersion      string
}

func (result PrerequisiteExpansionResult) Validate() error {
	concepts := make(map[ConceptID]struct{}, len(result.AddedConcepts))
	for _, concept := range result.AddedConcepts {
		if err := concept.Validate(); err != nil {
			return err
		}
		if concept.Atomicity != AtomicityAtomic {
			return fmt.Errorf("added prerequisite concept %q is not atomic", concept.ID)
		}
		if _, exists := concepts[concept.ID]; exists {
			return fmt.Errorf("duplicate added prerequisite concept %q", concept.ID)
		}
		concepts[concept.ID] = struct{}{}
	}
	type edgeKey struct {
		concept  ConceptID
		required ConceptID
		kind     PrerequisiteKind
	}
	seenEdges := make(map[edgeKey]struct{}, len(result.ExpandedPrerequisites))
	for _, prerequisite := range result.ExpandedPrerequisites {
		if err := prerequisite.Validate(); err != nil {
			return err
		}
		key := edgeKey{concept: prerequisite.ConceptID, required: prerequisite.RequiredConceptID, kind: prerequisite.Kind}
		if _, exists := seenEdges[key]; exists {
			return fmt.Errorf("duplicate expanded prerequisite edge from %q to %q", prerequisite.ConceptID, prerequisite.RequiredConceptID)
		}
		seenEdges[key] = struct{}{}
	}
	if cycleAt, found := prerequisiteCycle(result.ExpandedPrerequisites); found {
		return fmt.Errorf("expanded prerequisites contain a cycle at %q", cycleAt)
	}
	seenGaps := make(map[string]struct{}, len(result.UnresolvedGaps))
	for _, gap := range result.UnresolvedGaps {
		if err := gap.Validate(); err != nil {
			return err
		}
		key := string(gap.Code) + "\x00" + gap.ConceptID.String()
		if gap.RequiredConceptID != nil {
			key += "\x00" + gap.RequiredConceptID.String() + "\x00" + string(*gap.Kind)
		}
		if _, exists := seenGaps[key]; exists {
			return fmt.Errorf("duplicate prerequisite expansion gap for concept %q", gap.ConceptID)
		}
		seenGaps[key] = struct{}{}
	}
	if err := validateUniqueTexts("prerequisite expansion reasons", result.Reasons); err != nil {
		return err
	}
	if len(result.Reasons) == 0 {
		return fmt.Errorf("prerequisite expansion has no reasons")
	}
	if result.AlgorithmVersion != PrerequisiteExpansionVersionV1 {
		return fmt.Errorf("unsupported prerequisite expansion version %q", result.AlgorithmVersion)
	}
	return nil
}

func prerequisiteCycle(edges []Prerequisite) (ConceptID, bool) {
	adjacency := make(map[ConceptID][]ConceptID)
	nodes := make(map[ConceptID]struct{})
	for _, edge := range edges {
		adjacency[edge.ConceptID] = append(adjacency[edge.ConceptID], edge.RequiredConceptID)
		nodes[edge.ConceptID] = struct{}{}
		nodes[edge.RequiredConceptID] = struct{}{}
	}
	ids := make([]ConceptID, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	state := make(map[ConceptID]uint8, len(nodes))
	var visit func(ConceptID) (ConceptID, bool)
	visit = func(id ConceptID) (ConceptID, bool) {
		if state[id] == 1 {
			return id, true
		}
		if state[id] == 2 {
			return ConceptID{}, false
		}
		state[id] = 1
		next := append([]ConceptID(nil), adjacency[id]...)
		sort.Slice(next, func(i, j int) bool { return next[i].String() < next[j].String() })
		for _, required := range next {
			if cycleAt, found := visit(required); found {
				return cycleAt, true
			}
		}
		state[id] = 2
		return ConceptID{}, false
	}
	for _, id := range ids {
		if cycleAt, found := visit(id); found {
			return cycleAt, true
		}
	}
	return ConceptID{}, false
}
