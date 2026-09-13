package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type SecurityCoverageV1 struct{}

func NewSecurityCoverageV1() SecurityCoverageV1 {
	return SecurityCoverageV1{}
}

func (SecurityCoverageV1) Analyze(ctx context.Context, request SecurityCoverageRequest) (curriculum.SecurityCoverageReport, error) {
	const operation = "analyze security coverage"
	if err := ctx.Err(); err != nil {
		return curriculum.SecurityCoverageReport{}, ExternalError(operation, err)
	}
	knownEvidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.SecurityCoverageReport{}, Invalid(operation, err)
	}
	verifiedEvidence := indexVerifiedSecurityEvidence(request.EvidenceSets)
	indexes, err := indexCoverageRequest(CoverageAnalysisRequest{
		CurriculumID: request.CurriculumID, Goal: request.Goal, Competencies: request.Competencies,
		Concepts: request.Concepts, EvidenceSets: request.EvidenceSets,
	}, knownEvidence)
	if err != nil {
		return curriculum.SecurityCoverageReport{}, Invalid(operation, err)
	}
	requirements := make(map[curriculum.ID]curriculum.SecurityRequirement, len(request.Requirements))
	for _, requirement := range request.Requirements {
		if err := requirement.Validate(); err != nil {
			return curriculum.SecurityCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := requirements[requirement.ID]; exists {
			return curriculum.SecurityCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate security requirement %q", requirement.ID))
		}
		if err := requireKnownEvidence(requirement.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.SecurityCoverageReport{}, Invalid(operation, fmt.Errorf("security requirement %q: %w", requirement.ID, err))
		}
		generic := curriculum.CoverageRequirement{
			ID: requirement.ID, Dimension: curriculum.CoverageSecurity,
			TargetKind: requirement.TargetKind, TargetID: requirement.TargetID,
			Description: requirement.Description, EvidenceRefs: requirement.EvidenceRefs,
		}
		if err := validateCoverageTarget(generic, indexes); err != nil {
			return curriculum.SecurityCoverageReport{}, Invalid(operation, err)
		}
		requirements[requirement.ID] = requirement
	}
	if len(requirements) == 0 {
		return curriculum.SecurityCoverageReport{}, Invalid(operation, fmt.Errorf("security coverage has no domain-declared requirements"))
	}
	supports := make(map[curriculum.ID][]curriculum.SecuritySupport)
	seenSupports := make(map[curriculum.ID]struct{}, len(request.Supports))
	for _, support := range request.Supports {
		if err := support.Validate(); err != nil {
			return curriculum.SecurityCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := seenSupports[support.ID]; exists {
			return curriculum.SecurityCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate security support %q", support.ID))
		}
		seenSupports[support.ID] = struct{}{}
		requirement, exists := requirements[support.RequirementID]
		if !exists {
			return curriculum.SecurityCoverageReport{}, Invalid(operation, fmt.Errorf("security support %q references missing requirement", support.ID))
		}
		for _, conceptID := range support.ConceptIDs {
			if _, exists := indexes.concepts[conceptID]; !exists {
				return curriculum.SecurityCoverageReport{}, Invalid(operation, fmt.Errorf("security support %q references missing concept %q", support.ID, conceptID))
			}
		}
		if !securitySupportMatchesTarget(support, requirement, indexes) {
			return curriculum.SecurityCoverageReport{}, Invalid(operation, fmt.Errorf("security support %q is outside requirement target", support.ID))
		}
		if err := requireKnownEvidence(support.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.SecurityCoverageReport{}, Invalid(operation, fmt.Errorf("security support %q: %w", support.ID, err))
		}
		supports[support.RequirementID] = append(supports[support.RequirementID], support)
	}

	report := curriculum.SecurityCoverageReport{
		EvidencePolicyVersion: curriculum.SecurityEvidencePolicyVersionV1,
		AlgorithmVersion:      curriculum.SecurityCoverageVersionV1,
	}
	ids := make([]curriculum.ID, 0, len(requirements))
	for id := range requirements {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return curriculum.SecurityCoverageReport{}, ExternalError(operation, err)
		}
		requirement := requirements[id]
		generic := curriculum.CoverageRequirement{
			ID: requirement.ID, Dimension: curriculum.CoverageSecurity,
			TargetKind: requirement.TargetKind, TargetID: requirement.TargetID,
			Description: requirement.Description, EvidenceRefs: sortedEvidenceCopy(requirement.EvidenceRefs),
		}
		report.CoverageRequirements = append(report.CoverageRequirements, generic)
		result := curriculum.SecurityCoverageResult{RequirementID: id, Category: requirement.Category, Status: curriculum.CoverageMissing}
		values := append([]curriculum.SecuritySupport(nil), supports[id]...)
		sort.Slice(values, func(i, j int) bool { return values[i].ID.String() < values[j].ID.String() })
		targetExists := coverageTargetExists(generic, indexes)
		requirementVerified := securityEvidenceVerified(requirement.EvidenceRefs, verifiedEvidence)
		for _, support := range values {
			if !targetExists || !requirementVerified || !securityEvidenceVerified(support.EvidenceRefs, verifiedEvidence) {
				result.RejectedSupportIDs = append(result.RejectedSupportIDs, support.ID)
				continue
			}
			result.VerifiedSupportIDs = append(result.VerifiedSupportIDs, support.ID)
			report.CoverageSupports = append(report.CoverageSupports, curriculum.CoverageSupport{
				ID: support.ID, RequirementID: id, ConceptIDs: sortedConceptIDCopy(support.ConceptIDs),
				EvidenceRefs: sortedEvidenceCopy(support.EvidenceRefs), Reason: support.Reason,
			})
		}
		switch {
		case !targetExists:
			result.Reasons = []string{"security_coverage_target_missing"}
		case !requirementVerified:
			result.Reasons = []string{"security_requirement_evidence_unverified"}
		case len(result.VerifiedSupportIDs) > 0:
			result.Status = curriculum.CoverageCovered
			result.Reasons = []string{"verified_security_support"}
		case len(values) == 0:
			result.Reasons = []string{"no_security_support_declared"}
		default:
			result.Reasons = []string{"security_support_evidence_unverified"}
		}
		report.Results = append(report.Results, result)
	}
	sort.Slice(report.CoverageSupports, func(i, j int) bool {
		return report.CoverageSupports[i].ID.String() < report.CoverageSupports[j].ID.String()
	})
	if err := report.Validate(); err != nil {
		return curriculum.SecurityCoverageReport{}, Invalid(operation, err)
	}
	return report, nil
}

func indexVerifiedSecurityEvidence(sets []curriculum.CurriculumEvidenceSet) map[evidenceKey]struct{} {
	result := make(map[evidenceKey]struct{})
	for _, set := range sets {
		if set.Eligibility != curriculum.EvidenceReadyForCompile || set.Freshness.State != "fresh" {
			continue
		}
		conflicted := make(map[curriculum.ID]struct{})
		for _, conflict := range set.Conflicts {
			if conflict.Unresolved {
				for _, claimID := range conflict.ClaimIDs {
					conflicted[claimID] = struct{}{}
				}
			}
		}
		authority := make(map[curriculum.ID]curriculum.EvidenceSourceAuthority, len(set.SourceAuthority))
		for _, source := range set.SourceAuthority {
			authority[source.SourceID] = source
		}
		for _, claim := range set.Claims {
			if claim.Kind != curriculum.EvidenceClaimSecurity || claim.Status != curriculum.ConceptCurrent || claim.Confidence < .8 {
				continue
			}
			if _, exists := conflicted[claim.ID]; exists {
				continue
			}
			adequateSources, primary := 0, false
			for _, sourceID := range claim.SourceIDs {
				source := authority[sourceID]
				if (source.Role == "primary" || source.Role == "supporting") && (source.TemporalScope == "current" || source.TemporalScope == "version_bound") {
					adequateSources++
					primary = primary || source.Role == "primary"
				}
			}
			if adequateSources >= 2 && primary {
				result[evidenceKey{bundle: set.Bundle.ID.String(), claim: claim.ID.String()}] = struct{}{}
			}
		}
	}
	return result
}

func securityEvidenceVerified(references []curriculum.EvidenceRef, verified map[evidenceKey]struct{}) bool {
	if len(references) == 0 {
		return false
	}
	for _, reference := range references {
		if _, exists := verified[evidenceKey{bundle: reference.BundleID.String(), claim: reference.ClaimID.String()}]; !exists {
			return false
		}
	}
	return true
}

func securitySupportMatchesTarget(support curriculum.SecuritySupport, requirement curriculum.SecurityRequirement, indexes coverageIndexes) bool {
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
