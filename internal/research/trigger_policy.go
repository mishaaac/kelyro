package research

import (
	"fmt"
	"strings"
)

const ResearchTriggerAlgorithmV1 = "research-trigger-v1"

type ResearchTrigger string

const (
	ResearchTriggerManual            ResearchTrigger = "manual"
	ResearchTriggerMissingEvidence   ResearchTrigger = "missing_evidence"
	ResearchTriggerFreshnessExpired  ResearchTrigger = "freshness_expired"
	ResearchTriggerNewRelease        ResearchTrigger = "new_technology_release"
	ResearchTriggerDeprecation       ResearchTrigger = "deprecation_detected"
	ResearchTriggerConflict          ResearchTrigger = "conflict_unresolved"
	ResearchTriggerCurriculumCompile ResearchTrigger = "curriculum_compile_request"
	ResearchTriggerSecurityRefresh   ResearchTrigger = "security_sensitive_refresh"
)

func (trigger ResearchTrigger) Validate() error {
	switch trigger {
	case ResearchTriggerManual, ResearchTriggerMissingEvidence, ResearchTriggerFreshnessExpired,
		ResearchTriggerNewRelease, ResearchTriggerDeprecation, ResearchTriggerConflict,
		ResearchTriggerCurriculumCompile, ResearchTriggerSecurityRefresh:
		return nil
	default:
		return fmt.Errorf("invalid research trigger %q", trigger)
	}
}

type ResearchQueueStatus string

const (
	ResearchQueueQueued     ResearchQueueStatus = "queued"
	ResearchQueueDispatched ResearchQueueStatus = "dispatched"
	ResearchQueueCancelled  ResearchQueueStatus = "cancelled"
)

func (status ResearchQueueStatus) Validate() error {
	switch status {
	case ResearchQueueQueued, ResearchQueueDispatched, ResearchQueueCancelled:
		return nil
	default:
		return fmt.Errorf("invalid research queue status %q", status)
	}
}

type ResearchQueueItem struct {
	ID               ID
	Request          ResearchRequest
	Triggers         []ResearchTrigger
	Priority         VerificationPriority
	DedupeKey        string
	Status           ResearchQueueStatus
	QueuedAt         Timestamp
	StatusChangedAt  *Timestamp
	AlgorithmVersion string
}

func (item ResearchQueueItem) Validate() error {
	if err := item.ID.Validate(); err != nil {
		return fmt.Errorf("research queue item: %w", err)
	}
	if err := item.Request.Validate(); err != nil {
		return err
	}
	if len(item.Triggers) == 0 || len(item.Triggers) > 8 {
		return fmt.Errorf("research queue triggers must contain between 1 and 8 entries")
	}
	seen := make(map[ResearchTrigger]struct{}, len(item.Triggers))
	for _, trigger := range item.Triggers {
		if err := trigger.Validate(); err != nil {
			return err
		}
		if _, exists := seen[trigger]; exists {
			return fmt.Errorf("research queue contains duplicate trigger %q", trigger)
		}
		seen[trigger] = struct{}{}
	}
	if err := item.Priority.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(item.DedupeKey) == "" || item.DedupeKey != strings.TrimSpace(item.DedupeKey) {
		return fmt.Errorf("research queue dedupe key is invalid")
	}
	if err := item.Status.Validate(); err != nil {
		return err
	}
	if err := item.QueuedAt.Validate(); err != nil {
		return err
	}
	if item.QueuedAt.Before(item.Request.RequestedAt) {
		return fmt.Errorf("research queue time precedes request")
	}
	if item.Status == ResearchQueueQueued {
		if item.StatusChangedAt != nil {
			return fmt.Errorf("queued research item has a terminal status timestamp")
		}
	} else {
		if item.StatusChangedAt == nil {
			return fmt.Errorf("non-queued research item has no status timestamp")
		}
		if err := item.StatusChangedAt.Validate(); err != nil {
			return err
		}
		if item.StatusChangedAt.Before(item.QueuedAt) {
			return fmt.Errorf("research queue status change precedes queue time")
		}
	}
	if item.AlgorithmVersion != ResearchTriggerAlgorithmV1 {
		return fmt.Errorf("research queue algorithm must be %q", ResearchTriggerAlgorithmV1)
	}
	return nil
}

func ResearchTriggerDedupeKeyV1(request ResearchRequest) (string, error) {
	if err := request.Validate(); err != nil {
		return "", err
	}
	version := ""
	if request.TargetVersion != nil {
		version = request.TargetVersion.String()
	}
	canonical := strings.Join([]string{
		strings.ToLower(request.Topic.Subject), strings.ToLower(request.Topic.Domain),
		strings.ToLower(request.Topic.Technology), string(request.Purpose), version,
	}, "\x00")
	return "trigger:" + strings.TrimPrefix(CanonicalContentHashV1([]byte(canonical)), "sha256:"), nil
}
