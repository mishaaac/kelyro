package research

import (
	"testing"
	"time"
)

func TestVersionIdentifierRecognizesStrictSemanticVersions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value      string
		major      uint64
		minor      uint64
		patch      uint64
		prerelease string
		build      string
	}{
		{"1.2.3", 1, 2, 3, "", ""},
		{"0.0.0-rc.1+build.42", 0, 0, 0, "rc.1", "build.42"},
		{"18446744073709551615.1.0", ^uint64(0), 1, 0, "", ""},
	}
	for _, test := range tests {
		version, err := NewVersionIdentifier(test.value)
		if err != nil {
			t.Fatalf("NewVersionIdentifier(%q) error = %v", test.value, err)
		}
		semantic, ok := version.Semantic()
		if !ok || version.Scheme() != VersionSemantic {
			t.Fatalf("version %q classification = %q, semantic=%+v/%t", test.value, version.Scheme(), semantic, ok)
		}
		if semantic.Major != test.major || semantic.Minor != test.minor || semantic.Patch != test.patch ||
			semantic.Prerelease != test.prerelease || semantic.BuildMetadata != test.build {
			t.Fatalf("Semantic(%q) = %+v", test.value, semantic)
		}
		if semantic.IsPrerelease() != (test.prerelease != "") {
			t.Fatalf("IsPrerelease(%q) = %t", test.value, semantic.IsPrerelease())
		}
	}
}

func TestVersionIdentifierRecognizesDateBasedVersions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value     string
		year      int
		month     time.Month
		day       int
		precision DateVersionPrecision
	}{
		{"2024-02-29", 2024, time.February, 29, DatePrecisionDay},
		{"2026.08.26", 2026, time.August, 26, DatePrecisionDay},
		{"20260826", 2026, time.August, 26, DatePrecisionDay},
		{"2026-08", 2026, time.August, 1, DatePrecisionMonth},
		{"2026.08", 2026, time.August, 1, DatePrecisionMonth},
	}
	for _, test := range tests {
		version, err := NewDateVersionIdentifier(test.value)
		if err != nil {
			t.Fatalf("NewDateVersionIdentifier(%q) error = %v", test.value, err)
		}
		date, ok := version.DateBased()
		if !ok || version.Scheme() != VersionDateBased {
			t.Fatalf("version %q classification = %q, date=%+v/%t", test.value, version.Scheme(), date, ok)
		}
		if date.Year != test.year || date.Month != test.month || date.Day != test.day || date.Precision != test.precision {
			t.Fatalf("DateBased(%q) = %+v", test.value, date)
		}
	}
}

func TestVersionIdentifierPreservesNonSemverAsOpaque(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"go1.25", "R2026a", "Bookworm", "edition-7", "01.2.3", "2023-02-29"} {
		version, err := NewVersionIdentifier(value)
		if err != nil {
			t.Fatalf("NewVersionIdentifier(%q) error = %v", value, err)
		}
		if version.String() != value || version.Scheme() != VersionOpaque {
			t.Fatalf("version %q = %q/%q", value, version.String(), version.Scheme())
		}
	}
	if _, err := NewSemanticVersionIdentifier("1.2"); err == nil {
		t.Fatal("NewSemanticVersionIdentifier accepted incomplete SemVer")
	}
	if _, err := NewSemanticVersionIdentifier("1.2.3-01"); err == nil {
		t.Fatal("NewSemanticVersionIdentifier accepted leading-zero prerelease")
	}
	if _, err := NewDateVersionIdentifier("2023-02-29"); err == nil {
		t.Fatal("NewDateVersionIdentifier accepted invalid calendar date")
	}
}

func TestVersionSchemeAndDatePrecisionRejectUnknownValues(t *testing.T) {
	t.Parallel()
	if err := VersionScheme("debian").Validate(); err == nil {
		t.Fatal("VersionScheme.Validate accepted unknown value")
	}
	if err := DateVersionPrecision("quarter").Validate(); err == nil {
		t.Fatal("DateVersionPrecision.Validate accepted unknown value")
	}
}
