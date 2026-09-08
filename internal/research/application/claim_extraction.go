package application

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/research"
)

const (
	ClaimExtractorV1                    = "claim-extractor-v1"
	ClaimExtractorV2                    = "claim-extractor-v2"
	MaximumClaimCandidateStatementBytes = 2 << 10
	MaximumClaimCandidateMarkerBytes    = 256
	MaximumClaimCandidatesPerEvidence   = 8
	MaximumClaimCandidatesPerSource     = MaximumEvidenceCandidatesPerSource * MaximumClaimCandidatesPerEvidence
	MaximumClaimCandidatesPerRun        = 1000
)

type ClaimFamily string

const (
	ClaimFamilyExplicitDefinition     ClaimFamily = "explicit_definition"
	ClaimFamilyVersionReleaseFact     ClaimFamily = "version_release_fact"
	ClaimFamilyDeprecationStatement   ClaimFamily = "deprecation_statement"
	ClaimFamilyAvailabilitySupport    ClaimFamily = "availability_support_statement"
	ClaimFamilyExplicitRequirement    ClaimFamily = "explicit_requirement"
	ClaimFamilyExplicitRecommendation ClaimFamily = "explicit_recommendation"
)

func (family ClaimFamily) Validate() error {
	switch family {
	case ClaimFamilyExplicitDefinition, ClaimFamilyVersionReleaseFact,
		ClaimFamilyDeprecationStatement, ClaimFamilyAvailabilitySupport,
		ClaimFamilyExplicitRequirement, ClaimFamilyExplicitRecommendation:
		return nil
	default:
		return fmt.Errorf("invalid claim family %q", family)
	}
}

func (family ClaimFamily) ClaimType() research.ClaimType {
	switch family {
	case ClaimFamilyExplicitDefinition:
		return research.ClaimDefinition
	case ClaimFamilyVersionReleaseFact:
		return research.ClaimVersionChange
	case ClaimFamilyDeprecationStatement:
		return research.ClaimDeprecation
	case ClaimFamilyAvailabilitySupport:
		return research.ClaimCompatibility
	case ClaimFamilyExplicitRequirement:
		return research.ClaimRequirement
	case ClaimFamilyExplicitRecommendation:
		return research.ClaimRecommendation
	default:
		return ""
	}
}

func CanonicalClaimCandidateStatementHashV1(statement string) string {
	return research.CanonicalContentHashV1([]byte(statement))
}

// ClaimCandidate is one literal, unambiguous statement selected from exactly
// one persisted Evidence record. It is transient until Step 26 validates,
// aggregates, and persists the corresponding Claim.
type ClaimCandidate struct {
	SourceID         research.SourceID
	SnapshotID       research.ID
	EvidenceID       research.ID
	Family           ClaimFamily
	Statement        string
	StatementHash    string
	Scope            string
	VersionScope     *research.SourceVersion
	StatusScope      research.ClaimStatusScope
	Confidence       research.ClaimConfidence
	ExplicitMarker   string
	SentenceIndex    int
	ExtractorVersion string
}

func (candidate ClaimCandidate) Validate() error {
	if err := candidate.SourceID.Validate(); err != nil {
		return err
	}
	if err := candidate.SnapshotID.Validate(); err != nil {
		return fmt.Errorf("claim candidate snapshot: %w", err)
	}
	if err := candidate.EvidenceID.Validate(); err != nil {
		return fmt.Errorf("claim candidate evidence: %w", err)
	}
	if err := candidate.Family.Validate(); err != nil {
		return err
	}
	if err := validateClaimCandidateText("claim candidate statement", candidate.Statement, MaximumClaimCandidateStatementBytes, true); err != nil {
		return err
	}
	if candidate.StatementHash != CanonicalClaimCandidateStatementHashV1(candidate.Statement) {
		return fmt.Errorf("claim candidate statement hash does not match statement")
	}
	if err := validateClaimCandidateText("claim candidate scope", candidate.Scope, research.MaximumClaimScopeBytes, true); err != nil {
		return err
	}
	if candidate.VersionScope != nil {
		if err := candidate.VersionScope.Validate(); err != nil {
			return err
		}
	}
	if err := validateClaimCandidateStatusScope(candidate.StatusScope); err != nil {
		return err
	}
	if err := candidate.Confidence.Validate(); err != nil {
		return err
	}
	if candidate.Confidence.Value() <= 0 {
		return fmt.Errorf("claim candidate confidence must be greater than zero")
	}
	if err := validateClaimCandidateText("claim candidate explicit marker", candidate.ExplicitMarker, MaximumClaimCandidateMarkerBytes, true); err != nil {
		return err
	}
	if candidate.SentenceIndex < 0 {
		return fmt.Errorf("claim candidate sentence index is negative")
	}
	if !supportedClaimExtractorVersion(candidate.ExtractorVersion) {
		return fmt.Errorf("unsupported claim candidate extractor %q", candidate.ExtractorVersion)
	}
	return nil
}

func validateClaimCandidateText(name, value string, maximum int, required bool) error {
	if required {
		if err := requireText(name, value); err != nil {
			return err
		}
	} else if err := validateOptionalText(name, value); err != nil {
		return err
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s is not valid UTF-8", name)
	}
	if len(value) > maximum {
		return fmt.Errorf("%s exceeds %d bytes", name, maximum)
	}
	return nil
}

func validateClaimCandidateStatusScope(scope research.ClaimStatusScope) error {
	switch scope {
	case research.ClaimStatusAll, research.ClaimStatusStable,
		research.ClaimStatusPreview, research.ClaimStatusExperimental,
		research.ClaimStatusLegacy:
		return nil
	default:
		return fmt.Errorf("invalid claim candidate status scope %q", scope)
	}
}

type ClaimExtractionRequest struct {
	Topic         research.ResearchTopic
	Purpose       research.ResearchPurpose
	TargetVersion *research.SourceVersion
	Evidence      research.Evidence
}

func (request ClaimExtractionRequest) Validate() error {
	if err := request.Topic.Validate(); err != nil {
		return err
	}
	if err := request.Purpose.Validate(); err != nil {
		return err
	}
	if request.TargetVersion != nil {
		if err := request.TargetVersion.Validate(); err != nil {
			return err
		}
	}
	return request.Evidence.Validate()
}

type ClaimExtractionResult struct {
	Candidates       []ClaimCandidate
	AlgorithmVersion string
}

func (result ClaimExtractionResult) Validate() error {
	if len(result.Candidates) > MaximumClaimCandidatesPerEvidence {
		return fmt.Errorf("claim candidate count exceeds %d", MaximumClaimCandidatesPerEvidence)
	}
	seen := make(map[string]struct{}, len(result.Candidates))
	for index, candidate := range result.Candidates {
		if err := candidate.Validate(); err != nil {
			return fmt.Errorf("claim candidate %d: %w", index, err)
		}
		key := string(candidate.Family) + "\x00" + candidate.StatementHash + "\x00" + candidate.Scope + "\x00" + optionalSourceVersionString(candidate.VersionScope) + "\x00" + string(candidate.StatusScope)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("claim extraction repeats a semantic candidate")
		}
		seen[key] = struct{}{}
	}
	if !supportedClaimExtractorVersion(result.AlgorithmVersion) {
		return fmt.Errorf("unsupported claim extraction algorithm %q", result.AlgorithmVersion)
	}
	return nil
}

func supportedClaimExtractorVersion(version string) bool {
	return version == ClaimExtractorV1 || version == ClaimExtractorV2
}

// ClaimExtractor deterministically selects literal, single-family statements
// from one persisted Evidence record. Implementations must not perform I/O.
type ClaimExtractor interface {
	Extract(context.Context, ClaimExtractionRequest) (ClaimExtractionResult, error)
}
