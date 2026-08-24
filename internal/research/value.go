package research

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
	"unicode"
)

var (
	ErrEmptyID        = errors.New("id is empty")
	ErrEmptyTimestamp = errors.New("timestamp is empty")
	ErrInvalidScore   = errors.New("score must be between 0 and 1")
)

// ID is a stable identity for domain records other than sources and claims,
// which use their own strongly typed identities.
type ID struct{ value string }

func NewID(value string) (ID, error) {
	if err := validateIdentifier("id", value); err != nil {
		return ID{}, err
	}
	return ID{value: value}, nil
}

func (id ID) String() string  { return id.value }
func (id ID) Validate() error { return validateIdentifier("id", id.value) }

// SourceID is stable across aliases, locator changes, and snapshots.
type SourceID struct{ value string }

func NewSourceID(value string) (SourceID, error) {
	if err := validateIdentifier("source id", value); err != nil {
		return SourceID{}, err
	}
	return SourceID{value: value}, nil
}

func (id SourceID) String() string  { return id.value }
func (id SourceID) Validate() error { return validateIdentifier("source id", id.value) }

// ClaimID identifies an assertion independently from its wording revisions.
type ClaimID struct{ value string }

func NewClaimID(value string) (ClaimID, error) {
	if err := validateIdentifier("claim id", value); err != nil {
		return ClaimID{}, err
	}
	return ClaimID{value: value}, nil
}

func (id ClaimID) String() string  { return id.value }
func (id ClaimID) Validate() error { return validateIdentifier("claim id", id.value) }

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

// SourceLocator is a validated absolute HTTP(S) locator. It is not source
// identity: a SourceID remains stable when a canonical locator changes.
type SourceLocator struct{ value string }

func NewSourceLocator(value string) (SourceLocator, error) {
	normalized, err := normalizeLocator(value)
	if err != nil {
		return SourceLocator{}, err
	}
	return SourceLocator{value: normalized}, nil
}

func (locator SourceLocator) String() string { return locator.value }

func (locator SourceLocator) Validate() error {
	_, err := normalizeLocator(locator.value)
	return err
}

func normalizeLocator(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", errors.New("source locator is empty")
	}
	if value != strings.TrimSpace(value) || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return "", errors.New("source locator contains whitespace")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse source locator: %w", err)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("source locator scheme %q is unsupported", parsed.Scheme)
	}
	if parsed.Hostname() == "" {
		return "", errors.New("source locator host is empty")
	}
	if parsed.User != nil {
		return "", errors.New("source locator must not contain credentials")
	}
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String(), nil
}

// SourceVersion is an opaque, non-empty version identity. It deliberately does
// not require SemVer because many source ecosystems use different schemes.
type SourceVersion string

func NewSourceVersion(value string) (SourceVersion, error) {
	version := SourceVersion(value)
	if err := version.Validate(); err != nil {
		return "", err
	}
	return version, nil
}

func (version SourceVersion) String() string { return string(version) }

func (version SourceVersion) Validate() error {
	return requireText("source version", string(version))
}

// ResearchTopic describes a subject without assuming that the domain is
// software or that a technology is always present.
type ResearchTopic struct {
	Subject    string
	Domain     string
	Technology string
}

func NewResearchTopic(subject, domain, technology string) (ResearchTopic, error) {
	topic := ResearchTopic{Subject: subject, Domain: domain, Technology: technology}
	if err := topic.Validate(); err != nil {
		return ResearchTopic{}, err
	}
	return topic, nil
}

func (topic ResearchTopic) Validate() error {
	if err := requireText("research topic", topic.Subject); err != nil {
		return err
	}
	if err := validateOptionalText("research domain", topic.Domain); err != nil {
		return err
	}
	return validateOptionalText("research technology", topic.Technology)
}

type ResearchPurpose string

const (
	PurposeConceptDefinition    ResearchPurpose = "concept_definition"
	PurposeCurrentUsage         ResearchPurpose = "current_usage"
	PurposeVersionBehavior      ResearchPurpose = "version_behavior"
	PurposeReleaseStatus        ResearchPurpose = "release_status"
	PurposeDeprecationCheck     ResearchPurpose = "deprecation_check"
	PurposePrerequisiteResearch ResearchPurpose = "prerequisite_research"
	PurposeProductionPractice   ResearchPurpose = "production_practice"
	PurposeSecurityGuidance     ResearchPurpose = "security_guidance"
)

func (purpose ResearchPurpose) Validate() error {
	switch purpose {
	case PurposeConceptDefinition, PurposeCurrentUsage, PurposeVersionBehavior,
		PurposeReleaseStatus, PurposeDeprecationCheck, PurposePrerequisiteResearch,
		PurposeProductionPractice, PurposeSecurityGuidance:
		return nil
	default:
		return fmt.Errorf("invalid research purpose %q", purpose)
	}
}

// ClaimConfidence expresses evidence-backed confidence in [0, 1].
type ClaimConfidence struct{ value float64 }

func NewClaimConfidence(value float64) (ClaimConfidence, error) {
	if err := validateScore(value); err != nil {
		return ClaimConfidence{}, fmt.Errorf("claim confidence: %w", err)
	}
	return ClaimConfidence{value: value}, nil
}

func (confidence ClaimConfidence) Value() float64 { return confidence.value }
func (confidence ClaimConfidence) Validate() error {
	return validateScore(confidence.value)
}

// FreshnessScore expresses computed freshness in [0, 1]. It does not imply a
// particular algorithm; versioned scoring begins in its dedicated step.
type FreshnessScore struct{ value float64 }

func NewFreshnessScore(value float64) (FreshnessScore, error) {
	if err := validateScore(value); err != nil {
		return FreshnessScore{}, fmt.Errorf("freshness score: %w", err)
	}
	return FreshnessScore{value: value}, nil
}

func (score FreshnessScore) Value() float64  { return score.value }
func (score FreshnessScore) Validate() error { return validateScore(score.value) }

func validateScore(value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		return ErrInvalidScore
	}
	return nil
}

type DeepLink struct {
	Locator SourceLocator
	Label   string
}

func (link DeepLink) Validate() error {
	if err := link.Locator.Validate(); err != nil {
		return fmt.Errorf("deep link: %w", err)
	}
	return validateOptionalText("deep link label", link.Label)
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

func validateOptionalText(name, value string) error {
	if value == "" {
		return nil
	}
	return requireText(name, value)
}

func validateTimestamp(name string, timestamp Timestamp) error {
	if err := timestamp.Validate(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func validateOptionalTimestamp(name string, timestamp *Timestamp) error {
	if timestamp == nil {
		return nil
	}
	return validateTimestamp(name, *timestamp)
}

func validateIDs(name string, ids []ID, minimum int) error {
	if len(ids) < minimum {
		return fmt.Errorf("%s requires at least %d id(s)", name, minimum)
	}
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

func validateSourceIDs(name string, ids []SourceID, minimum int) error {
	if len(ids) < minimum {
		return fmt.Errorf("%s requires at least %d source id(s)", name, minimum)
	}
	seen := make(map[SourceID]struct{}, len(ids))
	for _, id := range ids {
		if err := id.Validate(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("%s contains duplicate source id %q", name, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateClaimIDs(name string, ids []ClaimID, minimum int) error {
	if len(ids) < minimum {
		return fmt.Errorf("%s requires at least %d claim id(s)", name, minimum)
	}
	seen := make(map[ClaimID]struct{}, len(ids))
	for _, id := range ids {
		if err := id.Validate(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("%s contains duplicate claim id %q", name, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}
