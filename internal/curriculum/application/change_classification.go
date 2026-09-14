package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
	"github.com/mishaaac/kelyro/internal/research"
)

type CurriculumChangeClassifierV1 struct{}

func NewCurriculumChangeClassifierV1() CurriculumChangeClassifierV1 {
	return CurriculumChangeClassifierV1{}
}

type accumulatedChange struct {
	migration curriculum.MigrationClass
	concepts  map[curriculum.ConceptID]struct{}
	rationale string
}

func (CurriculumChangeClassifierV1) Classify(ctx context.Context, request CurriculumChangeClassificationRequest) (curriculum.CurriculumChangeClassification, error) {
	const operation = "classify curriculum changes"
	if err := ctx.Err(); err != nil {
		return curriculum.CurriculumChangeClassification{}, ExternalError(operation, err)
	}
	if err := request.Old.Validate(); err != nil {
		return curriculum.CurriculumChangeClassification{}, Invalid(operation, fmt.Errorf("old curriculum: %w", err))
	}
	if err := request.New.Validate(); err != nil {
		return curriculum.CurriculumChangeClassification{}, Invalid(operation, fmt.Errorf("new curriculum: %w", err))
	}
	if request.Old.ID != request.New.ID {
		return curriculum.CurriculumChangeClassification{}, Invalid(operation, fmt.Errorf("curriculum IDs differ"))
	}
	if request.Old.Version == request.New.Version {
		return curriculum.CurriculumChangeClassification{}, Invalid(operation, fmt.Errorf("curriculum versions are identical"))
	}
	for name, environment := range map[string]*curriculum.EnvironmentPack{"old": request.OldEnvironment, "new": request.NewEnvironment} {
		if environment != nil {
			if err := environment.Validate(); err != nil {
				return curriculum.CurriculumChangeClassification{}, Invalid(operation, fmt.Errorf("%s environment: %w", name, err))
			}
		}
	}

	oldConcepts, newConcepts := conceptsByID(request.Old.Concepts), conceptsByID(request.New.Concepts)
	changes := make(map[curriculum.CurriculumChangeKind]*accumulatedChange)
	add := func(kind curriculum.CurriculumChangeKind, migration curriculum.MigrationClass, rationale string, concepts ...curriculum.ConceptID) {
		current, exists := changes[kind]
		if !exists {
			current = &accumulatedChange{migration: migration, rationale: rationale, concepts: make(map[curriculum.ConceptID]struct{})}
			changes[kind] = current
		}
		if migrationRank(migration) > migrationRank(current.migration) {
			current.migration = migration
		}
		for _, concept := range concepts {
			current.concepts[concept] = struct{}{}
		}
	}

	mappedOld, mappedNew := make(map[curriculum.ConceptID]struct{}), make(map[curriculum.ConceptID]struct{})
	mappingRationales := make(map[curriculum.CurriculumChangeKind][]string)
	for _, mapping := range request.IdentityMappings {
		if err := mapping.Validate(); err != nil {
			return curriculum.CurriculumChangeClassification{}, Invalid(operation, err)
		}
		for _, id := range mapping.OldConceptIDs {
			if _, exists := oldConcepts[id]; !exists {
				return curriculum.CurriculumChangeClassification{}, Invalid(operation, fmt.Errorf("identity mapping references missing old concept %q", id))
			}
			if _, stable := newConcepts[id]; stable {
				return curriculum.CurriculumChangeClassification{}, Invalid(operation, fmt.Errorf("identity mapping old concept %q was not removed", id))
			}
			if _, duplicate := mappedOld[id]; duplicate {
				return curriculum.CurriculumChangeClassification{}, Invalid(operation, fmt.Errorf("old concept %q appears in multiple identity mappings", id))
			}
			mappedOld[id] = struct{}{}
		}
		for _, id := range mapping.NewConceptIDs {
			if _, exists := newConcepts[id]; !exists {
				return curriculum.CurriculumChangeClassification{}, Invalid(operation, fmt.Errorf("identity mapping references missing new concept %q", id))
			}
			if _, stable := oldConcepts[id]; stable {
				return curriculum.CurriculumChangeClassification{}, Invalid(operation, fmt.Errorf("identity mapping new concept %q was not added", id))
			}
			if _, duplicate := mappedNew[id]; duplicate {
				return curriculum.CurriculumChangeClassification{}, Invalid(operation, fmt.Errorf("new concept %q appears in multiple identity mappings", id))
			}
			mappedNew[id] = struct{}{}
		}
		affected := append(append([]curriculum.ConceptID(nil), mapping.OldConceptIDs...), mapping.NewConceptIDs...)
		if len(mapping.OldConceptIDs) == 1 {
			add(curriculum.ChangeConceptSplit, curriculum.MigrationBreaking, "Explicit identity mappings split removed Concepts into multiple new Concept IDs.", affected...)
			mappingRationales[curriculum.ChangeConceptSplit] = append(mappingRationales[curriculum.ChangeConceptSplit], mapping.Rationale)
		} else {
			add(curriculum.ChangeConceptMerged, curriculum.MigrationBreaking, "Explicit identity mappings merge removed Concepts into new Concept IDs.", affected...)
			mappingRationales[curriculum.ChangeConceptMerged] = append(mappingRationales[curriculum.ChangeConceptMerged], mapping.Rationale)
		}
	}
	for kind, rationales := range mappingRationales {
		sort.Strings(rationales)
		changes[kind].rationale += " Reasons: " + strings.Join(rationales, "; ")
	}

	for id := range newConcepts {
		if _, existed := oldConcepts[id]; !existed {
			if _, mapped := mappedNew[id]; !mapped {
				add(curriculum.ChangeConceptAdded, curriculum.MigrationSafe, "New independently trackable Concept IDs were added.", id)
			}
		}
	}
	for id := range oldConcepts {
		if _, remains := newConcepts[id]; !remains {
			if _, mapped := mappedOld[id]; !mapped {
				add(curriculum.ChangeConceptRemoved, curriculum.MigrationRequiresStudentReview, "Concept IDs were removed without asserting migrated mastery.", id)
			}
		}
	}
	for id, oldConcept := range oldConcepts {
		if newConcept, stable := newConcepts[id]; stable && oldConcept.Status != newConcept.Status {
			add(curriculum.ChangeStatusChanged, curriculum.MigrationRequiresStudentReview, "Temporal guidance status changed and requires learner-visible review.", id)
		}
	}

	if affected := changedPrerequisiteConcepts(request.Old.Prerequisites, request.New.Prerequisites); len(affected) > 0 {
		add(curriculum.ChangePrerequisiteChanged, curriculum.MigrationRequiresRecompile, "Prerequisite graph edges or kinds changed.", affected...)
	}
	if !reflect.DeepEqual(request.Old.Phases, request.New.Phases) || !reflect.DeepEqual(request.Old.Modules, request.New.Modules) ||
		!reflect.DeepEqual(request.Old.Lessons, request.New.Lessons) || !reflect.DeepEqual(request.Old.Topics, request.New.Topics) {
		add(curriculum.ChangeHierarchyChanged, curriculum.MigrationRequiresRecompile, "Visible curriculum hierarchy changed independently from mastery.", commonConceptIDs(oldConcepts, newConcepts)...)
	}
	if !reflect.DeepEqual(request.OldEnvironment, request.NewEnvironment) {
		add(curriculum.ChangeEnvironmentChanged, curriculum.MigrationRequiresRecompile, "Environment Pack requirements or guidance changed.", environmentConceptIDs(request.OldEnvironment, request.NewEnvironment)...)
	}

	sourceAffected, sourceMigration, sourceChanged, err := classifySourceSignals(request, oldConcepts, newConcepts)
	if err != nil {
		return curriculum.CurriculumChangeClassification{}, Invalid(operation, err)
	}
	if sourceChanged || evidenceChanged(request.Old, request.New) {
		add(curriculum.ChangeSourceRefresh, sourceMigration, "Verified source identities, evidence references, or I-03 drift/impact signals changed.", sourceAffected...)
	}

	metadataChanged, metadataMigration, metadataConcepts := classifyMetadataChanges(request.Old, request.New, oldConcepts, newConcepts)
	if metadataChanged {
		add(curriculum.ChangeMetadataOnly, metadataMigration, "Curriculum metadata or same-identity concept specification changed.", metadataConcepts...)
	}

	result := curriculum.CurriculumChangeClassification{
		FromVersion: request.Old.Version, ToVersion: request.New.Version,
		AlgorithmVersion: curriculum.CurriculumChangeClassifierVersionV1,
	}
	kinds := make([]curriculum.CurriculumChangeKind, 0, len(changes))
	for kind := range changes {
		kinds = append(kinds, kind)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	for _, kind := range kinds {
		change := changes[kind]
		affected := sortedConceptSet(change.concepts)
		id := deterministicChangeID(request.Old.Version, request.New.Version, kind, change.migration, affected)
		result.Changes = append(result.Changes, curriculum.CurriculumChange{
			ID: id, FromVersion: request.Old.Version, ToVersion: request.New.Version, Kind: kind,
			Migration: change.migration, AffectedConcepts: affected, Rationale: change.rationale,
		})
	}
	if err := result.Validate(); err != nil {
		return curriculum.CurriculumChangeClassification{}, Invalid(operation, err)
	}
	return result, nil
}

func conceptsByID(values []curriculum.Concept) map[curriculum.ConceptID]curriculum.Concept {
	result := make(map[curriculum.ConceptID]curriculum.Concept, len(values))
	for _, concept := range values {
		result[concept.ID] = concept
	}
	return result
}

func changedPrerequisiteConcepts(oldValues, newValues []curriculum.Prerequisite) []curriculum.ConceptID {
	type edge struct {
		concept, required curriculum.ConceptID
		kind              curriculum.PrerequisiteKind
	}
	oldSet, newSet := make(map[edge]struct{}), make(map[edge]struct{})
	for _, value := range oldValues {
		oldSet[edge{value.ConceptID, value.RequiredConceptID, value.Kind}] = struct{}{}
	}
	for _, value := range newValues {
		newSet[edge{value.ConceptID, value.RequiredConceptID, value.Kind}] = struct{}{}
	}
	affected := make(map[curriculum.ConceptID]struct{})
	for value := range oldSet {
		if _, exists := newSet[value]; !exists {
			affected[value.concept] = struct{}{}
			affected[value.required] = struct{}{}
		}
	}
	for value := range newSet {
		if _, exists := oldSet[value]; !exists {
			affected[value.concept] = struct{}{}
			affected[value.required] = struct{}{}
		}
	}
	return sortedConceptSet(affected)
}

func commonConceptIDs(oldValues, newValues map[curriculum.ConceptID]curriculum.Concept) []curriculum.ConceptID {
	result := make(map[curriculum.ConceptID]struct{})
	for id := range oldValues {
		if _, exists := newValues[id]; exists {
			result[id] = struct{}{}
		}
	}
	return sortedConceptSet(result)
}

func environmentConceptIDs(values ...*curriculum.EnvironmentPack) []curriculum.ConceptID {
	result := make(map[curriculum.ConceptID]struct{})
	for _, environment := range values {
		if environment == nil {
			continue
		}
		for _, tool := range environment.Tools {
			if tool.IntroducedAt != nil {
				result[*tool.IntroducedAt] = struct{}{}
			}
			if tool.WhenNeeded != nil {
				result[*tool.WhenNeeded] = struct{}{}
			}
		}
	}
	return sortedConceptSet(result)
}

func classifySourceSignals(request CurriculumChangeClassificationRequest, oldConcepts, newConcepts map[curriculum.ConceptID]curriculum.Concept) ([]curriculum.ConceptID, curriculum.MigrationClass, bool, error) {
	migration := curriculum.MigrationSafe
	affected := make(map[curriculum.ConceptID]struct{})
	drifts := make(map[research.ID]research.DriftReport, len(request.DriftReports))
	oldBundles, newBundles := make(map[string]struct{}), make(map[string]struct{})
	for _, bundle := range request.Old.SourceBundles {
		oldBundles[bundle.ID.String()] = struct{}{}
	}
	for _, bundle := range request.New.SourceBundles {
		newBundles[bundle.ID.String()] = struct{}{}
	}
	for _, report := range request.DriftReports {
		if err := report.Validate(); err != nil {
			return nil, "", false, err
		}
		if _, duplicate := drifts[report.ID]; duplicate {
			return nil, "", false, fmt.Errorf("duplicate drift report %q", report.ID)
		}
		if _, exists := oldBundles[report.OldBundleID.String()]; !exists {
			return nil, "", false, fmt.Errorf("drift report %q references an old bundle outside the curriculum", report.ID)
		}
		if report.NewBundleID != nil {
			if _, exists := newBundles[report.NewBundleID.String()]; !exists {
				return nil, "", false, fmt.Errorf("drift report %q references a new bundle outside the curriculum", report.ID)
			}
		}
		drifts[report.ID] = report
		if report.Severity == research.SeverityCritical {
			migration = maxMigration(migration, curriculum.MigrationRequiresStudentReview)
		} else if report.Severity == research.SeverityImportant {
			migration = maxMigration(migration, curriculum.MigrationRequiresRecompile)
		}
	}
	seenImpact := make(map[research.ID]struct{}, len(request.ImpactReports))
	for _, report := range request.ImpactReports {
		if err := report.Validate(); err != nil {
			return nil, "", false, err
		}
		if _, duplicate := seenImpact[report.ID]; duplicate {
			return nil, "", false, fmt.Errorf("duplicate impact report %q", report.ID)
		}
		seenImpact[report.ID] = struct{}{}
		if _, exists := drifts[report.DriftReportID]; !exists {
			return nil, "", false, fmt.Errorf("impact report %q has no matching drift report", report.ID)
		}
		for _, reference := range report.FutureConceptRefs {
			id, err := curriculum.NewConceptID(reference.String())
			if err != nil {
				return nil, "", false, err
			}
			if _, old := oldConcepts[id]; old {
				affected[id] = struct{}{}
			}
			if _, current := newConcepts[id]; current {
				affected[id] = struct{}{}
			}
		}
		switch report.RecommendedAction {
		case research.ActionManualReview, research.ActionReviewCurriculum:
			migration = maxMigration(migration, curriculum.MigrationRequiresStudentReview)
		case research.ActionRecompileFuture:
			migration = maxMigration(migration, curriculum.MigrationRequiresRecompile)
		}
	}
	return sortedConceptSet(affected), migration, len(request.DriftReports) > 0 || len(request.ImpactReports) > 0, nil
}

func evidenceChanged(oldDefinition, newDefinition curriculum.CurriculumDefinition) bool {
	collect := func(definition curriculum.CurriculumDefinition) []string {
		var values []string
		for _, bundle := range definition.SourceBundles {
			values = append(values, "bundle:"+bundle.ID.String()+":"+bundle.ContentHash+":"+bundle.AlgorithmVersion+":"+bundle.VerifiedAt.Time().String())
		}
		appendRefs := func(owner string, refs []curriculum.EvidenceRef) {
			for _, ref := range refs {
				values = append(values, owner+":"+ref.BundleID.String()+":"+ref.ClaimID.String())
			}
		}
		for _, outcome := range definition.Goal.Outcomes {
			appendRefs("outcome:"+outcome.ID.String(), outcome.EvidenceRefs)
		}
		for _, competency := range definition.Competencies.Competencies {
			appendRefs("competency:"+competency.ID.String(), competency.EvidenceRefs)
		}
		for _, concept := range definition.Concepts {
			appendRefs("concept:"+concept.ID.String(), concept.EvidenceRefs)
		}
		for _, edge := range definition.Prerequisites {
			appendRefs("edge:"+edge.ConceptID.String()+":"+edge.RequiredConceptID.String()+":"+string(edge.Kind), edge.EvidenceRefs)
		}
		for _, requirement := range definition.CoverageRequirements {
			appendRefs("coverage:"+requirement.ID.String(), requirement.EvidenceRefs)
		}
		sort.Strings(values)
		return values
	}
	return !reflect.DeepEqual(collect(oldDefinition), collect(newDefinition))
}

func classifyMetadataChanges(oldDefinition, newDefinition curriculum.CurriculumDefinition, oldConcepts, newConcepts map[curriculum.ConceptID]curriculum.Concept) (bool, curriculum.MigrationClass, []curriculum.ConceptID) {
	changed := oldDefinition.Title != newDefinition.Title || oldDefinition.Description != newDefinition.Description
	migration := curriculum.MigrationSafe
	affected := make(map[curriculum.ConceptID]struct{})
	stripGoalEvidence := func(value curriculum.LearningGoalSpec) curriculum.LearningGoalSpec {
		value.Outcomes = append([]curriculum.GoalOutcome(nil), value.Outcomes...)
		for index := range value.Outcomes {
			value.Outcomes[index].EvidenceRefs = nil
		}
		return value
	}
	if !reflect.DeepEqual(stripGoalEvidence(oldDefinition.Goal), stripGoalEvidence(newDefinition.Goal)) {
		changed = true
		migration = curriculum.MigrationRequiresRecompile
	}
	stripCompetencies := func(value curriculum.CompetencyMatrix) curriculum.CompetencyMatrix {
		value.Competencies = append([]curriculum.Competency(nil), value.Competencies...)
		for index := range value.Competencies {
			value.Competencies[index].EvidenceRefs = nil
			value.Competencies[index].ConceptRefs = nil
		}
		return value
	}
	if !reflect.DeepEqual(stripCompetencies(oldDefinition.Competencies), stripCompetencies(newDefinition.Competencies)) {
		changed = true
		migration = curriculum.MigrationRequiresRecompile
	}
	for id, oldConcept := range oldConcepts {
		newConcept, stable := newConcepts[id]
		if !stable {
			continue
		}
		oldConcept.Status, newConcept.Status = "", ""
		oldConcept.EvidenceRefs, newConcept.EvidenceRefs = nil, nil
		if !reflect.DeepEqual(oldConcept, newConcept) {
			changed = true
			migration = curriculum.MigrationRequiresRecompile
			affected[id] = struct{}{}
		}
	}
	if !reflect.DeepEqual(oldDefinition.Vocabulary, newDefinition.Vocabulary) {
		changed = true
		migration = curriculum.MigrationRequiresRecompile
	}
	stripCoverage := func(values []curriculum.CoverageRequirement) []curriculum.CoverageRequirement {
		result := append([]curriculum.CoverageRequirement(nil), values...)
		for index := range result {
			result[index].EvidenceRefs = nil
		}
		return result
	}
	if !reflect.DeepEqual(stripCoverage(oldDefinition.CoverageRequirements), stripCoverage(newDefinition.CoverageRequirements)) {
		changed = true
		migration = curriculum.MigrationRequiresRecompile
	}
	return changed, migration, sortedConceptSet(affected)
}

func sortedConceptSet(values map[curriculum.ConceptID]struct{}) []curriculum.ConceptID {
	result := make([]curriculum.ConceptID, 0, len(values))
	for id := range values {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func deterministicChangeID(from, to curriculum.CurriculumVersion, kind curriculum.CurriculumChangeKind, migration curriculum.MigrationClass, concepts []curriculum.ConceptID) curriculum.ID {
	parts := []string{from.String(), to.String(), string(kind), string(migration)}
	for _, concept := range concepts {
		parts = append(parts, concept.String())
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	id, _ := curriculum.NewID("change." + string(kind) + "." + hex.EncodeToString(digest[:8]))
	return id
}

func migrationRank(value curriculum.MigrationClass) int {
	switch value {
	case curriculum.MigrationBreaking:
		return 4
	case curriculum.MigrationRequiresStudentReview:
		return 3
	case curriculum.MigrationRequiresRecompile:
		return 2
	default:
		return 1
	}
}

func maxMigration(left, right curriculum.MigrationClass) curriculum.MigrationClass {
	if migrationRank(right) > migrationRank(left) {
		return right
	}
	return left
}

var _ CurriculumChangeClassificationService = CurriculumChangeClassifierV1{}
