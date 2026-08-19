package learning

import (
	"errors"
	"math"
	"testing"
	"time"
)

const domainFixtureVersion = "i02-step01-v1"

func TestIDRequiresStableNonEmptyValue(t *testing.T) {
	t.Parallel()

	valid, err := NewID("knowledge.statistics.mean")
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	if got := valid.String(); got != "knowledge.statistics.mean" {
		t.Fatalf("ID.String() = %q", got)
	}

	for _, value := range []string{"", "   ", " concept", "concept ", "two concepts"} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			if _, err := NewID(value); err == nil {
				t.Fatalf("NewID(%q) accepted invalid value", value)
			}
		})
	}

	if _, err := NewID(""); !errors.Is(err, ErrEmptyID) {
		t.Fatalf("NewID(empty) error = %v, want ErrEmptyID", err)
	}
}

func TestMasteryValuesEnforceClosedUnitInterval(t *testing.T) {
	t.Parallel()

	for _, value := range []float64{0, 0.5, 1} {
		if score, err := NewMasteryScore(value); err != nil || score.Value() != value {
			t.Errorf("NewMasteryScore(%v) = %v, %v", value, score.Value(), err)
		}
		if threshold, err := NewMasteryThreshold(value); err != nil || threshold.Value() != value {
			t.Errorf("NewMasteryThreshold(%v) = %v, %v", value, threshold.Value(), err)
		}
	}

	for _, value := range []float64{-0.01, 1.01, math.NaN(), math.Inf(1)} {
		if _, err := NewMasteryScore(value); !errors.Is(err, ErrInvalidScore) {
			t.Errorf("NewMasteryScore(%v) error = %v", value, err)
		}
		if _, err := NewMasteryThreshold(value); !errors.Is(err, ErrInvalidThreshold) {
			t.Errorf("NewMasteryThreshold(%v) error = %v", value, err)
		}
	}
}

func TestTimestampNormalizesUTCAndRejectsZero(t *testing.T) {
	t.Parallel()

	local := time.Date(2026, time.August, 19, 9, 30, 0, 0, time.FixedZone("PET", -5*60*60))
	timestamp, err := NewTimestamp(local)
	if err != nil {
		t.Fatalf("NewTimestamp() error = %v", err)
	}
	if timestamp.Time().Location() != time.UTC {
		t.Fatalf("timestamp location = %v, want UTC", timestamp.Time().Location())
	}
	if !timestamp.Time().Equal(local) {
		t.Fatalf("timestamp instant = %v, want %v", timestamp.Time(), local)
	}
	if _, err := NewTimestamp(time.Time{}); !errors.Is(err, ErrEmptyTimestamp) {
		t.Fatalf("NewTimestamp(zero) error = %v", err)
	}
}

func mustID(t *testing.T, suffix string) ID {
	t.Helper()
	id, err := NewID(domainFixtureVersion + "." + suffix)
	if err != nil {
		t.Fatalf("NewID() fixture error = %v", err)
	}
	return id
}

func mustTimestamp(t *testing.T, hour int) Timestamp {
	t.Helper()
	timestamp, err := NewTimestamp(time.Date(2026, time.August, 19, hour, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewTimestamp() fixture error = %v", err)
	}
	return timestamp
}

func mustScore(t *testing.T, value float64) MasteryScore {
	t.Helper()
	score, err := NewMasteryScore(value)
	if err != nil {
		t.Fatalf("NewMasteryScore() fixture error = %v", err)
	}
	return score
}

func mustThreshold(t *testing.T, value float64) MasteryThreshold {
	t.Helper()
	threshold, err := NewMasteryThreshold(value)
	if err != nil {
		t.Fatalf("NewMasteryThreshold() fixture error = %v", err)
	}
	return threshold
}
