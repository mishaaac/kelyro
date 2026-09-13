package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type TheoryCoverageV1 struct{}

func NewTheoryCoverageV1() TheoryCoverageV1 {
	return TheoryCoverageV1{}
}

func (TheoryCoverageV1) Analyze(ctx context.Context, request TheoryCoverageRequest) (curriculum.TheoryCoverageReport, error) {
	const operation = "analyze theory coverage"
	if err := ctx.Err(); err != nil {
		return curriculum.TheoryCoverageReport{}, ExternalError(operation, err)
	}
	knownEvidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.TheoryCoverageReport{}, Invalid(operation, err)
	}
	if err := request.Competencies.Validate(); err != nil {
		return curriculum.TheoryCoverageReport{}, Invalid(operation, err)
	}
	competencies := make(map[curriculum.ID]curriculum.Competency, len(request.Competencies.Competencies))
	for _, competency := range request.Competencies.Competencies {
		if err := requireKnownEvidence(competency.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("theory competency %q: %w", competency.ID, err))
		}
		competencies[competency.ID] = competency
	}
	concepts := make(map[curriculum.ConceptID]curriculum.Concept, len(request.Concepts))
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := concepts[concept.ID]; exists {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate theory concept %q", concept.ID))
		}
		if err := requireKnownEvidence(concept.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("theory concept %q: %w", concept.ID, err))
		}
		concepts[concept.ID] = concept
	}
	important := make(map[curriculum.ID]struct{}, len(request.ImportantCompetencyIDs))
	for _, competencyID := range request.ImportantCompetencyIDs {
		if err := competencyID.Validate(); err != nil {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := competencies[competencyID]; !exists {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("important theory competency %q is absent from matrix", competencyID))
		}
		if _, exists := important[competencyID]; exists {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate important theory competency %q", competencyID))
		}
		important[competencyID] = struct{}{}
	}
	if len(important) == 0 {
		return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("theory coverage has no important competencies"))
	}
	contracts := make(map[curriculum.ID]curriculum.TheoryContract, len(request.Contracts))
	contractsByCompetency := make(map[curriculum.ID]curriculum.TheoryContract, len(request.Contracts))
	for _, contract := range request.Contracts {
		if err := contract.Validate(); err != nil {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := important[contract.CompetencyID]; !exists {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("theory contract %q targets a competency not declared important", contract.ID))
		}
		if _, exists := contracts[contract.ID]; exists {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate theory contract %q", contract.ID))
		}
		if _, exists := contractsByCompetency[contract.CompetencyID]; exists {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("multiple theory contracts target competency %q", contract.CompetencyID))
		}
		if err := requireKnownEvidence(contract.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("theory contract %q: %w", contract.ID, err))
		}
		contracts[contract.ID] = contract
		contractsByCompetency[contract.CompetencyID] = contract
	}
	supportsByContractFacet := make(map[curriculum.ID]map[curriculum.TheoryFacet][]curriculum.TheoryFacetSupport)
	seenSupports := make(map[curriculum.ID]struct{}, len(request.Supports))
	for _, support := range request.Supports {
		if err := support.Validate(); err != nil {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := seenSupports[support.ID]; exists {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate theory support %q", support.ID))
		}
		seenSupports[support.ID] = struct{}{}
		contract, exists := contracts[support.ContractID]
		if !exists {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("theory support %q references missing contract %q", support.ID, support.ContractID))
		}
		if !theoryFacetRequired(contract.RequiredFacets, support.Facet) {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("theory support %q targets unrequired facet %q", support.ID, support.Facet))
		}
		competency := competencies[contract.CompetencyID]
		for _, conceptID := range support.ConceptIDs {
			if _, exists := concepts[conceptID]; !exists {
				return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("theory support %q references missing concept %q", support.ID, conceptID))
			}
			if !containsConceptID(competency.ConceptRefs, conceptID) {
				return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("theory support %q references concept outside competency %q", support.ID, competency.ID))
			}
		}
		if err := requireKnownEvidence(support.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.TheoryCoverageReport{}, Invalid(operation, fmt.Errorf("theory support %q: %w", support.ID, err))
		}
		if supportsByContractFacet[support.ContractID] == nil {
			supportsByContractFacet[support.ContractID] = make(map[curriculum.TheoryFacet][]curriculum.TheoryFacetSupport)
		}
		supportsByContractFacet[support.ContractID][support.Facet] = append(supportsByContractFacet[support.ContractID][support.Facet], support)
	}

	report := curriculum.TheoryCoverageReport{AlgorithmVersion: curriculum.TheoryCoverageVersionV1}
	importantIDs := make([]curriculum.ID, 0, len(important))
	for id := range important {
		importantIDs = append(importantIDs, id)
	}
	sort.Slice(importantIDs, func(i, j int) bool { return importantIDs[i].String() < importantIDs[j].String() })
	for _, competencyID := range importantIDs {
		if err := ctx.Err(); err != nil {
			return curriculum.TheoryCoverageReport{}, ExternalError(operation, err)
		}
		competency := competencies[competencyID]
		contract, exists := contractsByCompetency[competencyID]
		if !exists {
			requirementID := theoryCoverageRequirementID(competencyID, "contract")
			report.Competencies = append(report.Competencies, curriculum.TheoryCompetencyCoverage{
				CompetencyID: competencyID, Status: curriculum.CoverageMissing, Reasons: []string{"no_theory_contract_declared"},
			})
			report.CoverageRequirements = append(report.CoverageRequirements, curriculum.CoverageRequirement{
				ID: requirementID, Dimension: curriculum.CoverageTheory, TargetKind: curriculum.CoverageTargetCompetency,
				TargetID: competencyID, Description: "Declare an evidence-backed theory contract for the important competency.",
				EvidenceRefs: append([]curriculum.EvidenceRef(nil), competency.EvidenceRefs...),
			})
			continue
		}
		contractID := contract.ID
		coverage := curriculum.TheoryCompetencyCoverage{CompetencyID: competencyID, ContractID: &contractID}
		coveredCount := 0
		for _, facet := range curriculum.AllTheoryFacets() {
			if !theoryFacetRequired(contract.RequiredFacets, facet) {
				continue
			}
			requirementID := theoryCoverageRequirementID(contract.ID, string(facet))
			report.CoverageRequirements = append(report.CoverageRequirements, curriculum.CoverageRequirement{
				ID: requirementID, Dimension: curriculum.CoverageTheory, TargetKind: curriculum.CoverageTargetCompetency,
				TargetID: competencyID, Description: fmt.Sprintf("Theory facet %s required by contract %s.", facet, contract.ID),
				EvidenceRefs: sortedEvidenceCopy(contract.EvidenceRefs),
			})
			facetCoverage := curriculum.TheoryFacetCoverage{Facet: facet, Status: curriculum.CoverageMissing, Reasons: []string{"no_explicit_theory_support"}}
			facetSupports := append([]curriculum.TheoryFacetSupport(nil), supportsByContractFacet[contract.ID][facet]...)
			sort.Slice(facetSupports, func(i, j int) bool { return facetSupports[i].ID.String() < facetSupports[j].ID.String() })
			if len(facetSupports) > 0 {
				facetCoverage.Status = curriculum.CoverageCovered
				facetCoverage.Reasons = []string{"explicit_evidence_backed_theory_support"}
				coveredCount++
				conceptSet := make(map[curriculum.ConceptID]struct{})
				for _, support := range facetSupports {
					facetCoverage.SupportIDs = append(facetCoverage.SupportIDs, support.ID)
					for _, conceptID := range support.ConceptIDs {
						conceptSet[conceptID] = struct{}{}
					}
					report.CoverageSupports = append(report.CoverageSupports, curriculum.CoverageSupport{
						ID: support.ID, RequirementID: requirementID,
						ConceptIDs:   sortedConceptIDCopy(support.ConceptIDs),
						EvidenceRefs: sortedEvidenceCopy(support.EvidenceRefs), Reason: support.Reason,
					})
				}
				for conceptID := range conceptSet {
					facetCoverage.ConceptIDs = append(facetCoverage.ConceptIDs, conceptID)
				}
				sort.Slice(facetCoverage.ConceptIDs, func(i, j int) bool {
					return facetCoverage.ConceptIDs[i].String() < facetCoverage.ConceptIDs[j].String()
				})
			}
			coverage.Facets = append(coverage.Facets, facetCoverage)
		}
		switch {
		case coveredCount == len(coverage.Facets):
			coverage.Status = curriculum.CoverageCovered
		case coveredCount == 0:
			coverage.Status = curriculum.CoverageMissing
		default:
			coverage.Status = curriculum.CoveragePartial
		}
		coverage.Reasons = []string{fmt.Sprintf("theory_facets:covered=%d:missing=%d", coveredCount, len(coverage.Facets)-coveredCount)}
		report.Competencies = append(report.Competencies, coverage)
	}
	sort.Slice(report.CoverageRequirements, func(i, j int) bool {
		return report.CoverageRequirements[i].ID.String() < report.CoverageRequirements[j].ID.String()
	})
	sort.Slice(report.CoverageSupports, func(i, j int) bool {
		return report.CoverageSupports[i].ID.String() < report.CoverageSupports[j].ID.String()
	})
	if err := report.Validate(); err != nil {
		return curriculum.TheoryCoverageReport{}, Invalid(operation, err)
	}
	return report, nil
}

func theoryFacetRequired(values []curriculum.TheoryFacet, target curriculum.TheoryFacet) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsConceptID(values []curriculum.ConceptID, target curriculum.ConceptID) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func theoryCoverageRequirementID(owner curriculum.ID, facet string) curriculum.ID {
	digest := sha256.Sum256([]byte(owner.String() + "\x00" + facet))
	id, err := curriculum.NewID("theory." + hex.EncodeToString(digest[:16]))
	if err != nil {
		panic(err)
	}
	return id
}

func sortedEvidenceCopy(values []curriculum.EvidenceRef) []curriculum.EvidenceRef {
	result := append([]curriculum.EvidenceRef(nil), values...)
	sortEvidenceRefs(result)
	return result
}

func sortedConceptIDCopy(values []curriculum.ConceptID) []curriculum.ConceptID {
	result := append([]curriculum.ConceptID(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}
