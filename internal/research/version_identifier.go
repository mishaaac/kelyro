package research

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type VersionScheme string

const (
	VersionSemantic  VersionScheme = "semantic"
	VersionDateBased VersionScheme = "date_based"
	VersionOpaque    VersionScheme = "opaque"
)

func (scheme VersionScheme) Validate() error {
	switch scheme {
	case VersionSemantic, VersionDateBased, VersionOpaque:
		return nil
	default:
		return fmt.Errorf("invalid version scheme %q", scheme)
	}
}

// SemanticVersion is the parsed SemVer 2.0.0 identity. Build metadata remains
// part of identity even though SemVer precedence ignores it.
type SemanticVersion struct {
	Major         uint64
	Minor         uint64
	Patch         uint64
	Prerelease    string
	BuildMetadata string
}

func (version SemanticVersion) IsPrerelease() bool { return version.Prerelease != "" }

type DateVersionPrecision string

const (
	DatePrecisionMonth DateVersionPrecision = "month"
	DatePrecisionDay   DateVersionPrecision = "day"
)

func (precision DateVersionPrecision) Validate() error {
	switch precision {
	case DatePrecisionMonth, DatePrecisionDay:
		return nil
	default:
		return fmt.Errorf("invalid date version precision %q", precision)
	}
}

// DateVersion is a validated calendar identity. Day is one for month-precision
// versions and Precision distinguishes that from an explicit first-of-month.
type DateVersion struct {
	Year      int
	Month     time.Month
	Day       int
	Precision DateVersionPrecision
}

func (version DateVersion) Validate() error {
	if err := version.Precision.Validate(); err != nil {
		return err
	}
	if version.Year < 1 || version.Month < time.January || version.Month > time.December {
		return fmt.Errorf("invalid date version calendar fields")
	}
	if version.Precision == DatePrecisionMonth {
		if version.Day != 1 {
			return fmt.Errorf("month-precision date version must use day one")
		}
		return nil
	}
	parsed := time.Date(version.Year, version.Month, version.Day, 0, 0, 0, 0, time.UTC)
	if parsed.Year() != version.Year || parsed.Month() != version.Month || parsed.Day() != version.Day {
		return fmt.Errorf("invalid date version calendar fields")
	}
	return nil
}

var semanticVersionPattern = regexp.MustCompile(`^([0-9]+)\.([0-9]+)\.([0-9]+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// Scheme classifies a valid identifier deterministically. Strict SemVer takes
// precedence, followed by supported ISO-like calendar forms; everything else
// remains opaque rather than being rejected or rewritten.
func (version SourceVersion) Scheme() VersionScheme {
	if _, ok := version.Semantic(); ok {
		return VersionSemantic
	}
	if _, ok := version.DateBased(); ok {
		return VersionDateBased
	}
	return VersionOpaque
}

func (version SourceVersion) Semantic() (SemanticVersion, bool) {
	matches := semanticVersionPattern.FindStringSubmatch(version.String())
	if matches == nil || hasLeadingZero(matches[1]) || hasLeadingZero(matches[2]) || hasLeadingZero(matches[3]) || invalidPrerelease(matches[4]) {
		return SemanticVersion{}, false
	}
	major, majorErr := strconv.ParseUint(matches[1], 10, 64)
	minor, minorErr := strconv.ParseUint(matches[2], 10, 64)
	patch, patchErr := strconv.ParseUint(matches[3], 10, 64)
	if majorErr != nil || minorErr != nil || patchErr != nil {
		return SemanticVersion{}, false
	}
	return SemanticVersion{
		Major: major, Minor: minor, Patch: patch,
		Prerelease: matches[4], BuildMetadata: matches[5],
	}, true
}

func (version SourceVersion) DateBased() (DateVersion, bool) {
	formats := []struct {
		layout    string
		precision DateVersionPrecision
	}{
		{"2006-01-02", DatePrecisionDay},
		{"2006.01.02", DatePrecisionDay},
		{"20060102", DatePrecisionDay},
		{"2006-01", DatePrecisionMonth},
		{"2006.01", DatePrecisionMonth},
	}
	for _, format := range formats {
		parsed, err := time.Parse(format.layout, version.String())
		if err != nil {
			continue
		}
		result := DateVersion{
			Year: parsed.Year(), Month: parsed.Month(), Day: parsed.Day(),
			Precision: format.precision,
		}
		if result.Validate() == nil {
			return result, true
		}
	}
	return DateVersion{}, false
}

func NewSemanticVersionIdentifier(value string) (VersionIdentifier, error) {
	version, err := NewVersionIdentifier(value)
	if err != nil {
		return "", err
	}
	if _, ok := version.Semantic(); !ok {
		return "", fmt.Errorf("version %q is not valid SemVer 2.0.0", value)
	}
	return version, nil
}

func NewDateVersionIdentifier(value string) (VersionIdentifier, error) {
	version, err := NewVersionIdentifier(value)
	if err != nil {
		return "", err
	}
	if _, ok := version.DateBased(); !ok {
		return "", fmt.Errorf("version %q is not a supported date-based version", value)
	}
	return version, nil
}

func hasLeadingZero(value string) bool {
	return len(value) > 1 && value[0] == '0'
}

func invalidPrerelease(value string) bool {
	if value == "" {
		return false
	}
	for _, identifier := range strings.Split(value, ".") {
		if identifier != "" && allDigits(identifier) && hasLeadingZero(identifier) {
			return true
		}
	}
	return false
}

func allDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
