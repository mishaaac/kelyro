package curriculum

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStableIdentityValueObjects(t *testing.T) {
	t.Parallel()

	for name, construct := range map[string]func(string) error{
		"generic":    func(value string) error { _, err := NewID(value); return err },
		"curriculum": func(value string) error { _, err := NewCurriculumID(value); return err },
		"concept":    func(value string) error { _, err := NewConceptID(value); return err },
	} {
		construct := construct
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := construct("domain.stable-id"); err != nil {
				t.Fatalf("valid identity rejected: %v", err)
			}
			for _, invalid := range []string{"", " ", "padded ", "two words", "line\nbreak", "terminal\x1b[2J"} {
				if err := construct(invalid); err == nil {
					t.Fatalf("identity %q accepted", invalid)
				}
			}
		})
	}
}

func TestTimestampNormalizesUTCAndRejectsInvalidRawValue(t *testing.T) {
	t.Parallel()

	local := time.Date(2026, time.September, 9, 12, 0, 0, 0, time.FixedZone("local", -5*60*60))
	timestamp, err := NewTimestamp(local)
	if err != nil {
		t.Fatalf("NewTimestamp() error = %v", err)
	}
	if timestamp.Time().Location() != time.UTC {
		t.Fatalf("timestamp location = %v, want UTC", timestamp.Time().Location())
	}
	if err := (Timestamp{value: local}).Validate(); err == nil {
		t.Fatal("raw non-UTC timestamp accepted")
	}
	if _, err := NewTimestamp(time.Time{}); !errors.Is(err, ErrEmptyTimestamp) {
		t.Fatalf("zero timestamp error = %v, want ErrEmptyTimestamp", err)
	}
}

func TestPackVersionRequiresStrictSemVerIdentity(t *testing.T) {
	t.Parallel()

	for _, valid := range []string{"0.1.0", "1.2.3", "1.2.3-alpha.1", "1.2.3-alpha+build.7"} {
		version, err := NewPackVersion(valid)
		if err != nil {
			t.Fatalf("NewPackVersion(%q) error = %v", valid, err)
		}
		if version.String() != valid {
			t.Fatalf("version string = %q, want %q", version, valid)
		}
	}
	for _, invalid := range []string{"v1.2.3", "1.2", "01.2.3", "1.2.3-01", "1.2.3+", " 1.2.3"} {
		if _, err := NewPackVersion(invalid); !errors.Is(err, ErrInvalidPackVersion) {
			t.Fatalf("NewPackVersion(%q) error = %v, want ErrInvalidPackVersion", invalid, err)
		}
	}
}

func TestSourceBundleRefRequiresCompleteImmutableIdentity(t *testing.T) {
	t.Parallel()

	reference := testSourceBundleRef(t)
	if err := reference.Validate(); err != nil {
		t.Fatalf("valid source bundle reference rejected: %v", err)
	}
	reference.ContentHash = strings.Repeat("a", 64)
	if err := reference.Validate(); err == nil {
		t.Fatal("source bundle reference accepted hash without algorithm prefix")
	}
}
