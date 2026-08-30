package application

import (
	"context"
	"strconv"

	"github.com/mishaaac/kelyro/internal/research"
	impactpolicy "github.com/mishaaac/kelyro/internal/research/impact"
)

func (service *impactService) Assess(ctx context.Context, request ImpactAssessmentRequest) (research.ImpactReport, error) {
	const operation = "assess research impact"
	if err := ctx.Err(); err != nil {
		return research.ImpactReport{}, Classify(ErrorUnavailable, operation, err)
	}
	if err := request.DriftReportID.Validate(); err != nil {
		return research.ImpactReport{}, invalid(operation, err)
	}
	if err := request.AssessedAt.Validate(); err != nil {
		return research.ImpactReport{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "drift repository", service.drift); err != nil {
		return research.ImpactReport{}, err
	}
	drift, err := service.drift.Get(ctx, request.DriftReportID)
	if err != nil {
		return research.ImpactReport{}, repositoryError(operation, err)
	}
	assessment, err := impactpolicy.AnalyzeV1(impactpolicy.Input{
		Drift: drift, References: request.References, AssessedAt: request.AssessedAt,
	})
	if err != nil {
		return research.ImpactReport{}, invalid(operation, err)
	}
	identity := []string{
		drift.ID.String(), string(assessment.Severity), string(assessment.RecommendedAction),
		request.AssessedAt.Time().Format("2006-01-02T15:04:05.999999999Z07:00"),
	}
	for _, id := range assessment.AffectedEvidenceIDs {
		identity = append(identity, "evidence:"+id.String())
	}
	for _, id := range assessment.AffectedBundleIDs {
		identity = append(identity, "bundle:"+id.String())
	}
	for _, id := range assessment.AffectedClaimIDs {
		identity = append(identity, "claim:"+id.String())
	}
	for _, id := range assessment.FutureConceptRefs {
		identity = append(identity, "concept:"+id.String())
	}
	for _, id := range assessment.FutureLessonRefs {
		identity = append(identity, "lesson:"+id.String())
	}
	for index, reference := range assessment.TechnologyVersionRefs {
		identity = append(identity, "technology:"+strconv.Itoa(index)+":"+reference.TechnologyID.String()+"@"+reference.Version.String())
	}
	report := research.ImpactReport{
		ID: stableResearchID("impact", identity...), DriftReportID: drift.ID,
		AffectedEvidenceIDs: assessment.AffectedEvidenceIDs, AffectedBundleIDs: assessment.AffectedBundleIDs,
		AffectedClaimIDs: assessment.AffectedClaimIDs, FutureConceptRefs: assessment.FutureConceptRefs,
		FutureLessonRefs: assessment.FutureLessonRefs, TechnologyVersionRefs: assessment.TechnologyVersionRefs,
		Severity: assessment.Severity, RecommendedAction: assessment.RecommendedAction,
		AssessedAt: request.AssessedAt, AlgorithmVersion: research.ImpactAnalysisAlgorithmV1,
	}
	if err := report.Validate(); err != nil {
		return research.ImpactReport{}, invalid(operation, err)
	}
	return report, nil
}
