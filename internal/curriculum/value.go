package curriculum

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var (
	ErrEmptyID            = errors.New("id is empty")
	ErrEmptyTimestamp     = errors.New("timestamp is empty")
	ErrInvalidPackVersion = errors.New("invalid pack version")
	packVersionPattern    = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
	contentHashPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// ID is a stable opaque identity used by curriculum records other than
// curricula and concepts, which have strongly typed identities.
type ID struct{ value string }

func NewID(value string) (ID, error) {
	if err := validateIdentifier("id", value); err != nil {
		return ID{}, err
	}
	return ID{value: value}, nil
}

func (id ID) String() string  { return id.value }
func (id ID) Validate() error { return validateIdentifier("id", id.value) }
func (id ID) MarshalText() ([]byte, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return []byte(id.value), nil
}

// CurriculumID remains stable across immutable CurriculumVersion values.
type CurriculumID struct{ value string }

func NewCurriculumID(value string) (CurriculumID, error) {
	if err := validateIdentifier("curriculum id", value); err != nil {
		return CurriculumID{}, err
	}
	return CurriculumID{value: value}, nil
}

func (id CurriculumID) String() string  { return id.value }
func (id CurriculumID) Validate() error { return validateIdentifier("curriculum id", id.value) }
func (id CurriculumID) MarshalText() ([]byte, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return []byte(id.value), nil
}

// ConceptID identifies one independently trackable knowledge unit. Display
// names, hierarchy placement, and wording changes never define this identity.
type ConceptID struct{ value string }

func NewConceptID(value string) (ConceptID, error) {
	if err := validateIdentifier("concept id", value); err != nil {
		return ConceptID{}, err
	}
	return ConceptID{value: value}, nil
}

func (id ConceptID) String() string  { return id.value }
func (id ConceptID) Validate() error { return validateIdentifier("concept id", id.value) }
func (id ConceptID) MarshalText() ([]byte, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return []byte(id.value), nil
}

func validateIdentifier(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s: %w", name, ErrEmptyID)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("%s %q has surrounding whitespace", name, value)
	}
	if strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return fmt.Errorf("%s %q contains whitespace", name, value)
	}
	return nil
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
func (timestamp Timestamp) MarshalText() ([]byte, error) {
	if err := timestamp.Validate(); err != nil {
		return nil, err
	}
	return []byte(timestamp.value.Format(time.RFC3339Nano)), nil
}

func (timestamp Timestamp) Validate() error {
	if timestamp.value.IsZero() {
		return ErrEmptyTimestamp
	}
	if timestamp.value.Location() != time.UTC {
		return errors.New("timestamp is not UTC")
	}
	return nil
}

// CurriculumVersion is an opaque immutable definition version. Pack versions
// are intentionally separate and follow stricter syntax.
type CurriculumVersion struct{ value string }

func NewCurriculumVersion(value string) (CurriculumVersion, error) {
	if err := requireText("curriculum version", value); err != nil {
		return CurriculumVersion{}, err
	}
	if value != strings.TrimSpace(value) {
		return CurriculumVersion{}, errors.New("curriculum version has surrounding whitespace")
	}
	return CurriculumVersion{value: value}, nil
}

func (version CurriculumVersion) String() string { return version.value }
func (version CurriculumVersion) MarshalText() ([]byte, error) {
	if err := version.Validate(); err != nil {
		return nil, err
	}
	return []byte(version.value), nil
}
func (version CurriculumVersion) Validate() error {
	_, err := NewCurriculumVersion(version.value)
	return err
}

// PackVersion is strict SemVer 2.0 syntax without a leading "v". Step 35 owns
// compatibility classification; this value object only validates identity.
type PackVersion struct{ value string }

func NewPackVersion(value string) (PackVersion, error) {
	match := packVersionPattern.FindStringSubmatch(value)
	if match == nil || invalidNumericPrerelease(match[4]) {
		return PackVersion{}, fmt.Errorf("%w %q", ErrInvalidPackVersion, value)
	}
	return PackVersion{value: value}, nil
}

func (version PackVersion) String() string { return version.value }
func (version PackVersion) MarshalText() ([]byte, error) {
	if err := version.Validate(); err != nil {
		return nil, err
	}
	return []byte(version.value), nil
}
func (version PackVersion) Validate() error {
	_, err := NewPackVersion(version.value)
	return err
}

func invalidNumericPrerelease(value string) bool {
	for _, identifier := range strings.Split(value, ".") {
		if len(identifier) > 1 && identifier[0] == '0' && onlyDigits(identifier) {
			return true
		}
	}
	return false
}

func onlyDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

// SourceBundleRef freezes the complete I-03 Source Bundle identity consumed by
// a compilation. It contains no raw source content and grants no network access.
type SourceBundleRef struct {
	ID               ID
	ContentHash      string
	AlgorithmVersion string
	VerifiedAt       Timestamp
}

func (reference SourceBundleRef) Validate() error {
	if err := reference.ID.Validate(); err != nil {
		return fmt.Errorf("source bundle: %w", err)
	}
	if !contentHashPattern.MatchString(reference.ContentHash) {
		return errors.New("source bundle content hash is not canonical sha256")
	}
	if err := requireText("source bundle algorithm version", reference.AlgorithmVersion); err != nil {
		return err
	}
	if err := reference.VerifiedAt.Validate(); err != nil {
		return fmt.Errorf("source bundle verified at: %w", err)
	}
	return nil
}

// EvidenceRef connects a curriculum fact to a Claim in an exact Source Bundle.
type EvidenceRef struct {
	BundleID ID
	ClaimID  ID
}

func (reference EvidenceRef) Validate() error {
	if err := reference.BundleID.Validate(); err != nil {
		return fmt.Errorf("evidence bundle: %w", err)
	}
	if err := reference.ClaimID.Validate(); err != nil {
		return fmt.Errorf("evidence claim: %w", err)
	}
	return nil
}

func requireText(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is empty", name)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("%s has surrounding whitespace", name)
	}
	return nil
}

func validateTexts(name string, values []string) error {
	for index, value := range values {
		if err := requireText(fmt.Sprintf("%s %d", name, index), value); err != nil {
			return err
		}
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

func validateConceptIDs(name string, ids []ConceptID) error {
	seen := make(map[ConceptID]struct{}, len(ids))
	for _, id := range ids {
		if err := id.Validate(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("%s contains duplicate concept id %q", name, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateEvidenceRefs(name string, references []EvidenceRef) error {
	seen := make(map[EvidenceRef]struct{}, len(references))
	for _, reference := range references {
		if err := reference.Validate(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, exists := seen[reference]; exists {
			return fmt.Errorf("%s contains duplicate evidence reference", name)
		}
		seen[reference] = struct{}{}
	}
	return nil
}
