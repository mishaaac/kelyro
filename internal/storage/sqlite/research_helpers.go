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

func researchOperationContext(ctx context.Context, timeout time.Duration, operation string) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, application.Classify(application.ErrorInvalidState, operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, application.Classify(application.ErrorUnavailable, operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, timeout)
	return operationContext, cancel, nil
}

func researchInvalid(operation string, err error) error {
	return application.Classify(application.ErrorInvalidState, operation, err)
}

func researchNotFound(operation string) error {
	return application.Classify(application.ErrorNotFound, operation, errors.New("record does not exist"))
}

func researchConflict(operation string) error {
	return application.Classify(application.ErrorConflict, operation, errors.New("record already exists"))
}

func researchPersistence(operation string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return application.Classify(application.ErrorUnavailable, operation, err)
	}
	return application.Classify(application.ErrorPersistenceFailure, operation, err)
}

func validateResearchKey(operation, key string) error {
	if strings.TrimSpace(key) == "" || key != strings.TrimSpace(key) {
		return researchInvalid(operation, errors.New("cache key is invalid"))
	}
	return nil
}

func timestampText(value research.Timestamp) string {
	return value.Time().Format(timestampFormat)
}

func optionalTimestampText(value *research.Timestamp) any {
	if value == nil {
		return nil
	}
	return timestampText(*value)
}

func optionalVersionText(value *research.SourceVersion) any {
	if value == nil {
		return nil
	}
	return value.String()
}

func scanTimestamp(value string) (research.Timestamp, error) {
	parsed, err := parseTimestamp(value)
	if err != nil {
		return research.Timestamp{}, err
	}
	return research.NewTimestamp(parsed)
}

func scanOptionalTimestamp(value sql.NullString) (*research.Timestamp, error) {
	if !value.Valid {
		return nil, nil
	}
	timestamp, err := scanTimestamp(value.String)
	if err != nil {
		return nil, err
	}
	return &timestamp, nil
}

func scanOptionalVersion(value sql.NullString) (*research.SourceVersion, error) {
	if !value.Valid {
		return nil, nil
	}
	version, err := research.NewSourceVersion(value.String)
	if err != nil {
		return nil, err
	}
	return &version, nil
}

func encodeJSON(operation string, value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", researchInvalid(operation, fmt.Errorf("encode repository value: %w", err))
	}
	return string(encoded), nil
}

func decodeJSON(value string, target any) error {
	return json.Unmarshal([]byte(value), target)
}

func sourceIDStrings(ids []research.SourceID) []string {
	result := make([]string, len(ids))
	for index, id := range ids {
		result[index] = id.String()
	}
	return result
}

func idStrings(ids []research.ID) []string {
	result := make([]string, len(ids))
	for index, id := range ids {
		result[index] = id.String()
	}
	return result
}

func claimIDStrings(ids []research.ClaimID) []string {
	result := make([]string, len(ids))
	for index, id := range ids {
		result[index] = id.String()
	}
	return result
}

func parseSourceIDs(values []string) ([]research.SourceID, error) {
	result := make([]research.SourceID, len(values))
	for index, value := range values {
		id, err := research.NewSourceID(value)
		if err != nil {
			return nil, err
		}
		result[index] = id
	}
	return result, nil
}

func parseIDs(values []string) ([]research.ID, error) {
	result := make([]research.ID, len(values))
	for index, value := range values {
		id, err := research.NewID(value)
		if err != nil {
			return nil, err
		}
		result[index] = id
	}
	return result, nil
}

func parseClaimIDs(values []string) ([]research.ClaimID, error) {
	result := make([]research.ClaimID, len(values))
	for index, value := range values {
		id, err := research.NewClaimID(value)
		if err != nil {
			return nil, err
		}
		result[index] = id
	}
	return result, nil
}

func recordExists(ctx context.Context, executor executor, table, column, value string) (bool, error) {
	var exists int
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE %s = ?)", table, column)
	if err := executor.QueryRowContext(ctx, query, value).Scan(&exists); err != nil {
		return false, err
	}
	return exists == 1, nil
}
