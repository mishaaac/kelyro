package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

type researchTriggerQueueRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchTriggerQueueRepository) Enqueue(ctx context.Context, item research.ResearchQueueItem) (research.ResearchQueueItem, error) {
	const operation = "enqueue SQLite research trigger"
	if err := item.Validate(); err != nil {
		return research.ResearchQueueItem{}, researchInvalid(operation, err)
	}
	if item.Status != research.ResearchQueueQueued {
		return research.ResearchQueueItem{}, researchInvalid(operation, errors.New("new research queue item is not queued"))
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.ResearchQueueItem{}, err
	}
	defer cancel()
	if existing, getErr := repository.getActiveByDedupe(opCtx, item.DedupeKey); getErr == nil {
		return existing, nil
	} else if !errors.Is(getErr, application.ErrNotFound) {
		return research.ResearchQueueItem{}, getErr
	}
	encoded, err := encodeResearchTriggers(item.Triggers)
	if err != nil {
		return research.ResearchQueueItem{}, researchInvalid(operation, err)
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO research_trigger_queue (id,request_id,subject,domain,technology,purpose,target_version,requested_at,triggers_json,priority,dedupe_key,status,queued_at,status_changed_at,algorithm_version) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		item.ID.String(), item.Request.ID.String(), item.Request.Topic.Subject, item.Request.Topic.Domain, item.Request.Topic.Technology,
		string(item.Request.Purpose), optionalVersionText(item.Request.TargetVersion), timestampText(item.Request.RequestedAt), encoded,
		string(item.Priority), item.DedupeKey, string(item.Status), timestampText(item.QueuedAt), optionalTimestampText(item.StatusChangedAt), item.AlgorithmVersion)
	if err == nil {
		return item, nil
	}
	if strings.Contains(err.Error(), "research_trigger_queue.dedupe_key") {
		return repository.getActiveByDedupe(opCtx, item.DedupeKey)
	}
	if strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return research.ResearchQueueItem{}, researchConflict(operation)
	}
	return research.ResearchQueueItem{}, researchPersistence(operation, err)
}

func (repository *researchTriggerQueueRepository) Get(ctx context.Context, id research.ID) (research.ResearchQueueItem, error) {
	const operation = "get SQLite research trigger"
	if err := id.Validate(); err != nil {
		return research.ResearchQueueItem{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.ResearchQueueItem{}, err
	}
	defer cancel()
	return scanResearchQueueItem(repository.executor.QueryRowContext(opCtx, researchTriggerSelect+` WHERE id=?`, id.String()), operation)
}

func (repository *researchTriggerQueueRepository) getActiveByDedupe(ctx context.Context, dedupeKey string) (research.ResearchQueueItem, error) {
	return scanResearchQueueItem(repository.executor.QueryRowContext(ctx, researchTriggerSelect+` WHERE dedupe_key=? AND status='queued'`, dedupeKey), "get SQLite active research trigger")
}

func (repository *researchTriggerQueueRepository) ListQueued(ctx context.Context) ([]research.ResearchQueueItem, error) {
	const operation = "list SQLite research triggers"
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	rows, err := repository.executor.QueryContext(opCtx, researchTriggerSelect+` WHERE status='queued' ORDER BY CASE priority WHEN 'critical' THEN 0 WHEN 'high' THEN 1 ELSE 2 END,queued_at,id`)
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	items := make([]research.ResearchQueueItem, 0)
	for rows.Next() {
		item, scanErr := scanResearchQueueItem(rows, operation)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, researchPersistence(operation, err)
	}
	return items, nil
}

func (repository *researchTriggerQueueRepository) Update(ctx context.Context, item research.ResearchQueueItem) error {
	const operation = "update SQLite research trigger"
	if err := item.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	if item.Status == research.ResearchQueueQueued {
		return researchInvalid(operation, errors.New("research queue update does not transition status"))
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	stored, err := scanResearchQueueItem(repository.executor.QueryRowContext(opCtx, researchTriggerSelect+` WHERE id=?`, item.ID.String()), operation)
	if err != nil {
		return err
	}
	if stored.Status != research.ResearchQueueQueued || !sameSQLiteResearchQueueIdentity(stored, item) {
		return researchInvalid(operation, errors.New("research queue transition is invalid"))
	}
	result, err := repository.executor.ExecContext(opCtx, `UPDATE research_trigger_queue SET status=?,status_changed_at=? WHERE id=? AND status='queued'`, string(item.Status), optionalTimestampText(item.StatusChangedAt), item.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if err := requireAffected(result); errors.Is(err, sql.ErrNoRows) {
		return researchConflict(operation)
	} else if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}

const researchTriggerSelect = `SELECT id,request_id,subject,domain,technology,purpose,target_version,requested_at,triggers_json,priority,dedupe_key,status,queued_at,status_changed_at,algorithm_version FROM research_trigger_queue`

func scanResearchQueueItem(row rowScanner, operation string) (research.ResearchQueueItem, error) {
	var idValue, requestValue, subject, domain, technology, purpose, requested, encoded, priority, dedupe, status, queued, algorithm string
	var targetVersion, statusChanged sql.NullString
	if err := row.Scan(&idValue, &requestValue, &subject, &domain, &technology, &purpose, &targetVersion, &requested, &encoded, &priority, &dedupe, &status, &queued, &statusChanged, &algorithm); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.ResearchQueueItem{}, researchNotFound(operation)
		}
		return research.ResearchQueueItem{}, researchPersistence(operation, err)
	}
	id, err := research.NewID(idValue)
	if err != nil {
		return research.ResearchQueueItem{}, researchPersistence(operation, err)
	}
	requestID, err := research.NewID(requestValue)
	if err != nil {
		return research.ResearchQueueItem{}, researchPersistence(operation, err)
	}
	topic, err := research.NewResearchTopic(subject, domain, technology)
	if err != nil {
		return research.ResearchQueueItem{}, researchPersistence(operation, err)
	}
	version, err := scanOptionalVersion(targetVersion)
	if err != nil {
		return research.ResearchQueueItem{}, researchPersistence(operation, err)
	}
	requestedAt, err := scanTimestamp(requested)
	if err != nil {
		return research.ResearchQueueItem{}, researchPersistence(operation, err)
	}
	queuedAt, err := scanTimestamp(queued)
	if err != nil {
		return research.ResearchQueueItem{}, researchPersistence(operation, err)
	}
	changedAt, err := scanOptionalTimestamp(statusChanged)
	if err != nil {
		return research.ResearchQueueItem{}, researchPersistence(operation, err)
	}
	triggers, err := decodeResearchTriggers(encoded)
	if err != nil {
		return research.ResearchQueueItem{}, researchPersistence(operation, err)
	}
	item := research.ResearchQueueItem{
		ID: id, Request: research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.ResearchPurpose(purpose), TargetVersion: version, RequestedAt: requestedAt},
		Triggers: triggers, Priority: research.VerificationPriority(priority), DedupeKey: dedupe,
		Status: research.ResearchQueueStatus(status), QueuedAt: queuedAt, StatusChangedAt: changedAt, AlgorithmVersion: algorithm,
	}
	if err := item.Validate(); err != nil {
		return research.ResearchQueueItem{}, researchPersistence(operation, err)
	}
	return item, nil
}

func encodeResearchTriggers(triggers []research.ResearchTrigger) (string, error) {
	encoded, err := json.Marshal(triggers)
	if err != nil {
		return "", fmt.Errorf("encode research triggers: %w", err)
	}
	return string(encoded), nil
}

func decodeResearchTriggers(encoded string) ([]research.ResearchTrigger, error) {
	var triggers []research.ResearchTrigger
	if err := json.Unmarshal([]byte(encoded), &triggers); err != nil {
		return nil, fmt.Errorf("decode research triggers: %w", err)
	}
	return triggers, nil
}

func sameSQLiteResearchQueueIdentity(left, right research.ResearchQueueItem) bool {
	if left.ID != right.ID || !sameResearchRequest(left.Request, right.Request) || left.Priority != right.Priority ||
		left.DedupeKey != right.DedupeKey || !left.QueuedAt.Time().Equal(right.QueuedAt.Time()) || left.AlgorithmVersion != right.AlgorithmVersion || len(left.Triggers) != len(right.Triggers) {
		return false
	}
	for index := range left.Triggers {
		if left.Triggers[index] != right.Triggers[index] {
			return false
		}
	}
	return true
}
