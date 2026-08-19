package learning

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
)

var (
	ErrEmptyID          = errors.New("id is empty")
	ErrInvalidScore     = errors.New("mastery score must be between 0 and 1")
	ErrInvalidThreshold = errors.New("mastery threshold must be between 0 and 1")
	ErrEmptyTimestamp   = errors.New("timestamp is empty")
)

// ID is a stable machine identity. Display names and titles must never be used
// in its place.
type ID struct{ value string }

// NewID validates a stable identity. IDs are opaque to the domain, but may not
// be empty, padded, or contain whitespace.
func NewID(value string) (ID, error) {
	if strings.TrimSpace(value) == "" {
		return ID{}, ErrEmptyID
	}
	if value != strings.TrimSpace(value) {
		return ID{}, fmt.Errorf("id %q has surrounding whitespace", value)
	}
	if strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return ID{}, fmt.Errorf("id %q contains whitespace", value)
	}
	return ID{value: value}, nil
}

func (id ID) String() string { return id.value }

func (id ID) Validate() error {
	_, err := NewID(id.value)
	return err
}

// Timestamp is a non-zero instant normalized to UTC at the domain boundary.
type Timestamp struct{ value time.Time }

func NewTimestamp(value time.Time) (Timestamp, error) {
	if value.IsZero() {
		return Timestamp{}, ErrEmptyTimestamp
	}
	return Timestamp{value: value.UTC()}, nil
}

func (timestamp Timestamp) Time() time.Time { return timestamp.value }

func (timestamp Timestamp) Validate() error {
	if timestamp.value.IsZero() {
		return ErrEmptyTimestamp
	}
	if timestamp.value.Location() != time.UTC {
		return errors.New("timestamp is not UTC")
	}
	return nil
}

func (timestamp Timestamp) Before(other Timestamp) bool {
	return timestamp.value.Before(other.value)
}

func (timestamp Timestamp) After(other Timestamp) bool {
	return timestamp.value.After(other.value)
}

// MasteryScore is evidence-derived proficiency in the closed interval [0, 1].
// It is deliberately separate from exposure lifecycle state.
type MasteryScore struct{ value float64 }

func NewMasteryScore(value float64) (MasteryScore, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		return MasteryScore{}, ErrInvalidScore
	}
	return MasteryScore{value: value}, nil
}

func (score MasteryScore) Value() float64 { return score.value }

func (score MasteryScore) Validate() error {
	_, err := NewMasteryScore(score.value)
	return err
}

// MasteryThreshold is the minimum mastery score required by a goal or
// curriculum policy. It is a policy value, not a computed score.
type MasteryThreshold struct{ value float64 }

func NewMasteryThreshold(value float64) (MasteryThreshold, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		return MasteryThreshold{}, ErrInvalidThreshold
	}
	return MasteryThreshold{value: value}, nil
}

func (threshold MasteryThreshold) Value() float64 { return threshold.value }

func (threshold MasteryThreshold) Validate() error {
	_, err := NewMasteryThreshold(threshold.value)
	return err
}

func requireText(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is empty", name)
	}
	return nil
}

func validateIDs(name string, ids []ID) error {
	seen := make(map[ID]struct{}, len(ids))
	for _, id := range ids {
		if err := id.Validate(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("%s contains duplicate id %q", name, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateOptionalTimestamp(name string, timestamp *Timestamp) error {
	if timestamp == nil {
		return nil
	}
	if err := timestamp.Validate(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
