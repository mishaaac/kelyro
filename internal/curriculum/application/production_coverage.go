package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type ProductionCoverageV1 struct{}

func NewProductionCoverageV1() ProductionCoverageV1 {
	return ProductionCoverageV1{}
}

func (ProductionCoverageV1) Analyze(ctx context.Context, request ProductionCoverageRequest) (curriculum.ProductionCoverageReport, error) {
	const operation = "analyze production coverage"
	if err := ctx.Err(); err != nil {
		return curriculum.ProductionCoverageReport{}, ExternalError(operation, err)
	}
	knownEvidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.ProductionCoverageReport{}, Invalid(operation, err)
	}
	adequateEvidence := indexAdequateProductionEvidence(request.EvidenceSets)
	indexes, err := indexCoverageRequest(CoverageAnalysisRequest{
		CurriculumID: request.CurriculumID, Goal: request.Goal, Competencies: request.Competencies,
		Concepts: request.Concepts, EvidenceSets: request.EvidenceSets,
	}, knownEvidence)
	if err != nil {
		return curriculum.ProductionCoverageReport{}, Invalid(operation, err)
	}
	requirements := make(map[curriculum.ID]curriculum.ProductionRequirement, len(request.Requirements))
	for _, requirement := range request.Requirements {
		if err := requirement.Validate(); err != nil {
			return curriculum.ProductionCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := requirements[requirement.ID]; exists {
			return curriculum.ProductionCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate production requirement %q", requirement.ID))
		}
		if err := requireKnownEvidence(requirement.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.ProductionCoverageReport{}, Invalid(operation, fmt.Errorf("production requirement %q: %w", requirement.ID, err))
		}
		generic := curriculum.CoverageRequirement{
			ID: requirement.ID, Dimension: curriculum.CoverageProduction,
			TargetKind: requirement.TargetKind, TargetID: requirement.TargetID,
			Description: requirement.Description, EvidenceRefs: requirement.EvidenceRefs,
		}
		if err := validateCoverageTarget(generic, indexes); err != nil {
			return curriculum.ProductionCoverageReport{}, Invalid(operation, err)
		}
		requirements[requirement.ID] = requirement
	}
	if len(requirements) == 0 {
		return curriculum.ProductionCoverageReport{}, Invalid(operation, fmt.Errorf("production coverage has no domain-declared requirements"))
	}
	supports := make(map[curriculum.ID][]curriculum.ProductionSupport)
	seenSupports := make(map[curriculum.ID]struct{}, len(request.Supports))
	for _, support := range request.Supports {
		if err := support.Validate(); err != nil {
			return curriculum.ProductionCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := seenSupports[support.ID]; exists {
			return curriculum.ProductionCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate production support %q", support.ID))
		}
		seenSupports[support.ID] = struct{}{}
		requirement, exists := requirements[support.RequirementID]
		if !exists {
			return curriculum.ProductionCoverageReport{}, Invalid(operation, fmt.Errorf("production support %q references missing requirement", support.ID))
		}
		for _, conceptID := range support.ConceptIDs {
			if _, exists := indexes.concepts[conceptID]; !exists {
				return curriculum.ProductionCoverageReport{}, Invalid(operation, fmt.Errorf("production support %q references missing concept %q", support.ID, conceptID))
			}
		}
		if !productionSupportMatchesTarget(support, requirement, indexes) {
			return curriculum.ProductionCoverageReport{}, Invalid(operation, fmt.Errorf("production support %q is outside requirement target", support.ID))
		}
		if err := requireKnownEvidence(support.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.ProductionCoverageReport{}, Invalid(operation, fmt.Errorf("production support %q: %w", support.ID, err))
		}
		supports[support.RequirementID] = append(supports[support.RequirementID], support)
	}

	report := curriculum.ProductionCoverageReport{
		EvidencePolicyVersion: curriculum.ProductionEvidencePolicyVersionV1,
		AlgorithmVersion:      curriculum.ProductionCoverageVersionV1,
	}
	ids := make([]curriculum.ID, 0, len(requirements))
	for id := range requirements {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return curriculum.ProductionCoverageReport{}, ExternalError(operation, err)
		}
		requirement := requirements[id]
		generic := curriculum.CoverageRequirement{
			ID: requirement.ID, Dimension: curriculum.CoverageProduction,
			TargetKind: requirement.TargetKind, TargetID: requirement.TargetID,
			Description: requirement.Description, EvidenceRefs: sortedEvidenceCopy(requirement.EvidenceRefs),
		}
		report.CoverageRequirements = append(report.CoverageRequirements, generic)
		result := curriculum.ProductionCoverageResult{RequirementID: id, Category: requirement.Category, Status: curriculum.CoverageMissing}
		values := append([]curriculum.ProductionSupport(nil), supports[id]...)
		sort.Slice(values, func(i, j int) bool { return values[i].ID.String() < values[j].ID.String() })
		targetExists := coverageTargetExists(generic, indexes)
		requirementAdequate := productionEvidenceAdequate(requirement.EvidenceRefs, adequateEvidence)
		for _, support := range values {
			if !targetExists || !requirementAdequate || !productionEvidenceAdequate(support.EvidenceRefs, adequateEvidence) {
				result.RejectedSupportIDs = append(result.RejectedSupportIDs, support.ID)
				continue
			}
			result.AdequateSupportIDs = append(result.AdequateSupportIDs, support.ID)
			report.CoverageSupports = append(report.CoverageSupports, curriculum.CoverageSupport{
				ID: support.ID, RequirementID: id, ConceptIDs: sortedConceptIDCopy(support.ConceptIDs),
				EvidenceRefs: sortedEvidenceCopy(support.EvidenceRefs), Reason: support.Reason,
			})
		}
		switch {
		case !targetExists:
			result.Reasons = []string{"production_coverage_target_missing"}
		case !requirementAdequate:
			result.Reasons = []string{"production_requirement_evidence_inadequate"}
		case len(result.AdequateSupportIDs) > 0:
			result.Status = curriculum.CoverageCovered
			result.Reasons = []string{"adequate_production_support"}
		case len(values) == 0:
			result.Reasons = []string{"no_production_support_declared"}
		default:
			result.Reasons = []string{"production_support_evidence_inadequate"}
		}
		report.Results = append(report.Results, result)
	}
	sort.Slice(report.CoverageSupports, func(i, j int) bool {
		return report.CoverageSupports[i].ID.String() < report.CoverageSupports[j].ID.String()
	})
	if err := report.Validate(); err != nil {
		return curriculum.ProductionCoverageReport{}, Invalid(operation, err)
	}
	return report, nil
}

func productionSupportMatchesTarget(support curriculum.ProductionSupport, requirement curriculum.ProductionRequirement, indexes coverageIndexes) bool {
	switch requirement.TargetKind {
	case curriculum.CoverageTargetCompetency:
		competency, exists := indexes.competencies[requirement.TargetID]
		if !exists {
			return true
		}
		for _, conceptID := range support.ConceptIDs {
			if !containsConceptID(competency.ConceptRefs, conceptID) {
				return false
			}
		}
		return true
	case curriculum.CoverageTargetConcept:
		target, err := curriculum.NewConceptID(requirement.TargetID.String())
		if err != nil {
			return true
		}
		if _, exists := indexes.concepts[target]; !exists {
			return true
		}
		return containsConceptID(support.ConceptIDs, target)
	default:
		return true
	}
}

func indexAdequateProductionEvidence(sets []curriculum.CurriculumEvidenceSet) map[evidenceKey]struct{} {
	result := make(map[evidenceKey]struct{})
	for _, set := range sets {
		authority := make(map[curriculum.ID]bool, len(set.SourceAuthority))
		for _, source := range set.SourceAuthority {
			authority[source.SourceID] = (source.Role == "primary" || source.Role == "supporting") && (source.TemporalScope == "current" || source.TemporalScope == "version_bound")
		}
		for _, claim := range set.Claims {
			if !productionClaimKindAdequate(claim.Kind) || !productionClaimStatusAdequate(claim.Status) {
				continue
			}
			adequateSource := false
			for _, sourceID := range claim.SourceIDs {
				adequateSource = adequateSource || authority[sourceID]
			}
			if adequateSource {
				result[evidenceKey{bundle: set.Bundle.ID.String(), claim: claim.ID.String()}] = struct{}{}
			}
		}
	}
	return result
}

func productionClaimKindAdequate(kind curriculum.EvidenceClaimKind) bool {
	switch kind {
	case curriculum.EvidenceClaimRequirement, curriculum.EvidenceClaimBehavior,
		curriculum.EvidenceClaimVersionChange, curriculum.EvidenceClaimDeprecation,
		curriculum.EvidenceClaimRecommendation, curriculum.EvidenceClaimWarning,
		curriculum.EvidenceClaimCompatibility, curriculum.EvidenceClaimSecurity:
		return true
	default:
		return false
	}
}

func productionClaimStatusAdequate(status curriculum.ConceptStatus) bool {
	return status == curriculum.ConceptCurrent || status == curriculum.ConceptExperimental || status == curriculum.ConceptPreview
}

func productionEvidenceAdequate(references []curriculum.EvidenceRef, adequate map[evidenceKey]struct{}) bool {
	if len(references) == 0 {
		return false
	}
	for _, reference := range references {
		if _, exists := adequate[evidenceKey{bundle: reference.BundleID.String(), claim: reference.ClaimID.String()}]; !exists {
			return false
		}
	}
	return true
}
