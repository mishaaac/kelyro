package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type GapScannerV1 struct{}

func NewGapScannerV1() GapScannerV1 {
	return GapScannerV1{}
}

func (GapScannerV1) Scan(ctx context.Context, request GapScanRequest) (curriculum.GapScanReport, error) {
	const operation = "scan curriculum gaps"
	if err := ctx.Err(); err != nil {
		return curriculum.GapScanReport{}, ExternalError(operation, err)
	}
	if err := request.GoalID.Validate(); err != nil {
		return curriculum.GapScanReport{}, Invalid(operation, err)
	}
	if err := request.Coverage.Validate(); err != nil {
		return curriculum.GapScanReport{}, Invalid(operation, err)
	}
	requirements := make(map[curriculum.ID]curriculum.CoverageRequirement, len(request.Requirements))
	for _, requirement := range request.Requirements {
		if err := requirement.Validate(); err != nil {
			return curriculum.GapScanReport{}, Invalid(operation, err)
		}
		if _, exists := requirements[requirement.ID]; exists {
			return curriculum.GapScanReport{}, Invalid(operation, fmt.Errorf("duplicate gap-scan coverage requirement %q", requirement.ID))
		}
		requirements[requirement.ID] = requirement
	}
	seenReportRequirements := make(map[curriculum.ID]struct{}, len(requirements))
	gaps := make(map[curriculum.ID]curriculum.Gap)
	for _, dimension := range request.Coverage.Dimensions {
		if err := ctx.Err(); err != nil {
			return curriculum.GapScanReport{}, ExternalError(operation, err)
		}
		if len(dimension.Requirements) == 0 {
			if dimension.Status != curriculum.CoverageCovered {
				gap := coverageDeclarationGap(request.GoalID, dimension.Dimension)
				gaps[gap.ID] = gap
			}
			continue
		}
		for _, result := range dimension.Requirements {
			requirement, exists := requirements[result.RequirementID]
			if !exists {
				return curriculum.GapScanReport{}, Invalid(operation, fmt.Errorf("coverage report references missing requirement %q", result.RequirementID))
			}
			if requirement.Dimension != dimension.Dimension {
				return curriculum.GapScanReport{}, Invalid(operation, fmt.Errorf("coverage requirement %q appears in wrong dimension %q", requirement.ID, dimension.Dimension))
			}
			if _, exists := seenReportRequirements[requirement.ID]; exists {
				return curriculum.GapScanReport{}, Invalid(operation, fmt.Errorf("coverage requirement %q appears multiple times in report", requirement.ID))
			}
			seenReportRequirements[requirement.ID] = struct{}{}
			if result.Status == curriculum.CoverageCovered {
				continue
			}
			gap := coverageResultGap(requirement, result)
			gaps[gap.ID] = gap
		}
	}
	if len(seenReportRequirements) != len(requirements) {
		return curriculum.GapScanReport{}, Invalid(operation, fmt.Errorf("coverage report does not contain every supplied requirement"))
	}
	for _, finding := range request.PrerequisiteGaps {
		if err := finding.Validate(); err != nil {
			return curriculum.GapScanReport{}, Invalid(operation, err)
		}
		gap := prerequisiteFindingGap(finding)
		gaps[gap.ID] = gap
	}
	for _, finding := range request.CurrentGuidanceFindings {
		if err := finding.Validate(); err != nil {
			return curriculum.GapScanReport{}, Invalid(operation, err)
		}
		gap := currentGuidanceGap(finding)
		gaps[gap.ID] = gap
	}

	report := curriculum.GapScanReport{AlgorithmVersion: curriculum.GapScannerVersionV1, Gaps: make([]curriculum.Gap, 0, len(gaps))}
	for _, gap := range gaps {
		gap.EvidenceRefs = append([]curriculum.EvidenceRef(nil), gap.EvidenceRefs...)
		sort.Slice(gap.EvidenceRefs, func(i, j int) bool {
			if gap.EvidenceRefs[i].BundleID != gap.EvidenceRefs[j].BundleID {
				return gap.EvidenceRefs[i].BundleID.String() < gap.EvidenceRefs[j].BundleID.String()
			}
			return gap.EvidenceRefs[i].ClaimID.String() < gap.EvidenceRefs[j].ClaimID.String()
		})
		report.Gaps = append(report.Gaps, gap)
	}
	sort.Slice(report.Gaps, func(i, j int) bool { return gapLess(report.Gaps[i], report.Gaps[j]) })
	if err := report.Validate(); err != nil {
		return curriculum.GapScanReport{}, Invalid(operation, err)
	}
	return report, nil
}

func coverageResultGap(requirement curriculum.CoverageRequirement, result curriculum.CoverageResult) curriculum.Gap {
	kind := gapKindForCoverage(requirement.Dimension)
	severity := gapSeverityForCoverage(requirement.Dimension, result.Status)
	reason := fmt.Sprintf("%s coverage for requirement %q (%s): %s", result.Status, requirement.ID, requirement.Description, strings.Join(result.Reasons, "; "))
	return curriculum.Gap{
		ID:   deterministicGapID(kind, requirement.TargetID, "requirement:"+requirement.ID.String()),
		Kind: kind, Severity: severity, TargetID: requirement.TargetID, Reason: reason,
		EvidenceRefs: append([]curriculum.EvidenceRef(nil), requirement.EvidenceRefs...),
	}
}

func coverageDeclarationGap(goalID curriculum.ID, dimension curriculum.CoverageDimension) curriculum.Gap {
	kind := gapKindForCoverage(dimension)
	return curriculum.Gap{
		ID:   deterministicGapID(kind, goalID, "undeclared:"+string(dimension)),
		Kind: kind, Severity: gapSeverityForCoverage(dimension, curriculum.CoverageMissing), TargetID: goalID,
		Reason: fmt.Sprintf("%s coverage has no declared requirements; declare and satisfy at least one requirement", dimension),
	}
}

func prerequisiteFindingGap(finding curriculum.PrerequisiteExpansionGap) curriculum.Gap {
	targetID, _ := curriculum.NewID(finding.ConceptID.String())
	source := "prerequisite:" + string(finding.Code) + ":" + finding.Reason
	if finding.RequiredConceptID != nil {
		source += ":" + finding.RequiredConceptID.String()
	}
	if finding.Kind != nil {
		source += ":" + string(*finding.Kind)
	}
	source += ":" + gapEvidenceKey(finding.EvidenceRefs)
	return curriculum.Gap{
		ID:   deterministicGapID(curriculum.GapMissingPrerequisite, targetID, source),
		Kind: curriculum.GapMissingPrerequisite, Severity: curriculum.GapBlocking, TargetID: targetID,
		Reason:       fmt.Sprintf("prerequisite %s: %s", finding.Code, finding.Reason),
		EvidenceRefs: append([]curriculum.EvidenceRef(nil), finding.EvidenceRefs...),
	}
}

func currentGuidanceGap(finding curriculum.CurrentGuidanceFinding) curriculum.Gap {
	return curriculum.Gap{
		ID:   deterministicGapID(curriculum.GapMissingCurrentGuidance, finding.TargetID, "current-guidance:"+finding.Reason+":"+gapEvidenceKey(finding.EvidenceRefs)),
		Kind: curriculum.GapMissingCurrentGuidance, Severity: curriculum.GapImportant, TargetID: finding.TargetID,
		Reason: finding.Reason, EvidenceRefs: append([]curriculum.EvidenceRef(nil), finding.EvidenceRefs...),
	}
}

func gapEvidenceKey(references []curriculum.EvidenceRef) string {
	values := make([]string, len(references))
	for index, reference := range references {
		values[index] = reference.BundleID.String() + "/" + reference.ClaimID.String()
	}
	sort.Strings(values)
	return strings.Join(values, ",")
}

func gapKindForCoverage(dimension curriculum.CoverageDimension) curriculum.GapKind {
	switch dimension {
	case curriculum.CoverageCompetency:
		return curriculum.GapMissingCompetency
	case curriculum.CoverageConcept:
		return curriculum.GapMissingConcept
	case curriculum.CoverageEvidence:
		return curriculum.GapMissingEvidence
	case curriculum.CoverageTheory:
		return curriculum.GapMissingTheory
	case curriculum.CoveragePractice:
		return curriculum.GapMissingPractice
	case curriculum.CoverageProduction:
		return curriculum.GapMissingProduction
	case curriculum.CoverageSecurity:
		return curriculum.GapMissingSecurity
	case curriculum.CoverageToolchain:
		return curriculum.GapMissingToolchain
	default:
		panic("validated coverage dimension is unsupported")
	}
}

func gapSeverityForCoverage(dimension curriculum.CoverageDimension, status curriculum.CoverageStatus) curriculum.GapSeverity {
	base := curriculum.GapImportant
	switch dimension {
	case curriculum.CoverageCompetency, curriculum.CoverageConcept, curriculum.CoverageEvidence, curriculum.CoverageSecurity:
		base = curriculum.GapBlocking
	}
	if status != curriculum.CoveragePartial {
		return base
	}
	if base == curriculum.GapBlocking {
		return curriculum.GapImportant
	}
	return curriculum.GapRecommended
}

func deterministicGapID(kind curriculum.GapKind, target curriculum.ID, source string) curriculum.ID {
	digest := sha256.Sum256([]byte(string(kind) + "\x00" + target.String() + "\x00" + source))
	id, err := curriculum.NewID("gap." + hex.EncodeToString(digest[:16]))
	if err != nil {
		panic(err)
	}
	return id
}

func gapLess(left, right curriculum.Gap) bool {
	leftSeverity, rightSeverity := gapSeverityRank(left.Severity), gapSeverityRank(right.Severity)
	if leftSeverity != rightSeverity {
		return leftSeverity < rightSeverity
	}
	if left.Kind != right.Kind {
		return left.Kind < right.Kind
	}
	if left.TargetID != right.TargetID {
		return left.TargetID.String() < right.TargetID.String()
	}
	return left.ID.String() < right.ID.String()
}

func gapSeverityRank(severity curriculum.GapSeverity) int {
	switch severity {
	case curriculum.GapBlocking:
		return 0
	case curriculum.GapImportant:
		return 1
	case curriculum.GapRecommended:
		return 2
	default:
		return 3
	}
}
