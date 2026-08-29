package application

import (
	"context"
	"strconv"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	driftpolicy "github.com/mishaaac/kelyro/internal/research/drift"
)

func (service *driftService) Detect(ctx context.Context, request DriftDetectionRequest) (DriftDetectionResult, error) {
	const operation = "detect evidence drift"
	if err := ctx.Err(); err != nil {
		return DriftDetectionResult{}, Classify(ErrorUnavailable, operation, err)
	}
	result, err := driftpolicy.DetectV1(driftpolicy.Input{
		OldBundle: request.OldBundle, NewBundle: request.NewBundle,
		OldClaims: request.OldClaims, NewClaims: request.NewClaims,
		SnapshotObservations: request.SnapshotObservations,
		ReleaseObservations:  request.ReleaseObservations, DetectedAt: request.DetectedAt,
	})
	if err != nil {
		return DriftDetectionResult{}, invalid(operation, err)
	}
	var newBundleID *research.ID
	if request.NewBundle != nil {
		id := request.NewBundle.ID
		newBundleID = &id
	}
	reports := make([]research.DriftReport, 0, len(result.Findings))
	for _, finding := range result.Findings {
		identity := []string{
			request.OldBundle.ID.String(), string(finding.Type),
			string(finding.Severity), strconv.FormatFloat(finding.Confidence.Value(), 'g', -1, 64),
			request.DetectedAt.Time().Format(time.RFC3339Nano),
		}
		if newBundleID != nil {
			identity = append(identity, newBundleID.String())
		}
		for _, claimID := range finding.AffectedClaims {
			identity = append(identity, claimID.String())
		}
		for _, evidenceID := range finding.OldEvidence {
			identity = append(identity, "old:"+evidenceID.String())
		}
		for _, evidenceID := range finding.NewEvidence {
			identity = append(identity, "new:"+evidenceID.String())
		}
		report := research.DriftReport{
			ID: stableResearchID("drift", identity...), OldBundleID: request.OldBundle.ID,
			NewBundleID: newBundleID, Type: finding.Type, Severity: finding.Severity,
			AffectedClaims: append([]research.ClaimID(nil), finding.AffectedClaims...),
			OldEvidence:    append([]research.ID(nil), finding.OldEvidence...),
			NewEvidence:    append([]research.ID(nil), finding.NewEvidence...),
			Confidence:     finding.Confidence, DetectedAt: request.DetectedAt,
			AlgorithmVersion: research.DriftAlgorithmV1,
		}
		if err := report.Validate(); err != nil {
			return DriftDetectionResult{}, invalid(operation, err)
		}
		reports = append(reports, report)
	}
	return DriftDetectionResult{
		Reports:          reports,
		UnresolvedClaims: append([]research.ClaimID(nil), result.UnresolvedClaims...),
	}, nil
}
