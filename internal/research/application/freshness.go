package application

import (
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/freshness"
)

// FreshnessRecordFromAssessment maps a known-verification assessment to the
// existing persistence contract. Unknown assessments deliberately remain
// transient because storing one would require inventing last_verified_at.
func FreshnessRecordFromAssessment(subjectID research.ID, assessment freshness.Assessment) (FreshnessRecord, error) {
	if err := subjectID.Validate(); err != nil {
		return FreshnessRecord{}, fmt.Errorf("freshness subject: %w", err)
	}
	if err := assessment.Validate(); err != nil {
		return FreshnessRecord{}, fmt.Errorf("freshness assessment: %w", err)
	}
	if assessment.State == research.FreshnessUnknown || assessment.LastVerifiedAt == nil {
		return FreshnessRecord{}, fmt.Errorf("unknown freshness has no persistable last verification")
	}
	record := FreshnessRecord{
		SubjectID: subjectID, State: assessment.State, Score: assessment.Score,
		LastVerifiedAt: *assessment.LastVerifiedAt, AlgorithmVersion: assessment.AlgorithmVersion,
	}
	if err := record.Validate(); err != nil {
		return FreshnessRecord{}, err
	}
	return record, nil
}

// FreshnessRecordFromSchedule combines matching, independently versioned
// freshness and scheduling outputs for persistence and due-list queries.
func FreshnessRecordFromSchedule(subjectID research.ID, assessment freshness.Assessment, schedule freshness.Schedule) (FreshnessRecord, error) {
	record, err := FreshnessRecordFromAssessment(subjectID, assessment)
	if err != nil {
		return FreshnessRecord{}, err
	}
	if err := schedule.Validate(); err != nil {
		return FreshnessRecord{}, fmt.Errorf("freshness schedule: %w", err)
	}
	if !schedule.LastVerifiedAt.Time().Equal(record.LastVerifiedAt.Time()) {
		return FreshnessRecord{}, fmt.Errorf("freshness schedule uses a different last verification")
	}
	record.NextVerifyAt = &schedule.NextVerifyAt
	record.VerificationReason = schedule.Reason
	record.Priority = schedule.Priority
	record.SchedulingAlgorithmVersion = schedule.AlgorithmVersion
	if err := record.Validate(); err != nil {
		return FreshnessRecord{}, err
	}
	return record, nil
}
