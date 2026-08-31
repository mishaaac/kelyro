package application

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/research"
)

const (
	EvidenceExtractorV1                  = "evidence-extractor-v1"
	MinimumEvidenceCandidateScore        = 50
	MaximumEvidenceCandidatesPerSource   = 24
	MaximumEvidenceCandidatesPerRun      = 1000
	MaximumEvidenceCandidateSignals      = 16
	MaximumEvidenceCandidateExcerptBytes = 2 << 10
	MaximumEvidenceCandidateContextBytes = 512
)

// EvidenceCandidateKind identifies which normalized representation contains
// the literal observation. It does not classify authority, trust, or a Claim.
type EvidenceCandidateKind string

const (
	EvidenceCandidateHeading  EvidenceCandidateKind = "heading"
	EvidenceCandidatePassage  EvidenceCandidateKind = "passage"
	EvidenceCandidateMetadata EvidenceCandidateKind = "structured_metadata"
)

func (kind EvidenceCandidateKind) Validate() error {
	switch kind {
	case EvidenceCandidateHeading, EvidenceCandidatePassage, EvidenceCandidateMetadata:
		return nil
	default:
		return fmt.Errorf("invalid evidence candidate kind %q", kind)
	}
}

// EvidenceSignal is the closed, versioned vocabulary used by v1 scoring.
// Signals describe why bounded normalized content was selected; they are not
// facts, trust decisions, or Claim types.
type EvidenceSignal string

const (
	EvidenceSignalTopicExact         EvidenceSignal = "topic_exact"
	EvidenceSignalTopicTerms         EvidenceSignal = "topic_terms"
	EvidenceSignalDocumentTopic      EvidenceSignal = "document_topic"
	EvidenceSignalTechnology         EvidenceSignal = "technology"
	EvidenceSignalDomain             EvidenceSignal = "domain"
	EvidenceSignalTargetVersion      EvidenceSignal = "target_version"
	EvidenceSignalHeading            EvidenceSignal = "heading"
	EvidenceSignalStructuredMetadata EvidenceSignal = "structured_metadata"
	EvidenceSignalVersionFact        EvidenceSignal = "version_fact"
	EvidenceSignalReleaseFact        EvidenceSignal = "release_fact"
	EvidenceSignalDeprecationMarker  EvidenceSignal = "deprecation_marker"
	EvidenceSignalPurposeMatch       EvidenceSignal = "purpose_match"
)

func (signal EvidenceSignal) Validate() error {
	switch signal {
	case EvidenceSignalTopicExact, EvidenceSignalTopicTerms,
		EvidenceSignalDocumentTopic, EvidenceSignalTechnology,
		EvidenceSignalDomain, EvidenceSignalTargetVersion,
		EvidenceSignalHeading, EvidenceSignalStructuredMetadata,
		EvidenceSignalVersionFact, EvidenceSignalReleaseFact,
		EvidenceSignalDeprecationMarker, EvidenceSignalPurposeMatch:
		return nil
	default:
		return fmt.Errorf("invalid evidence signal %q", signal)
	}
}

// EvidenceCandidate is a transient, scored projection of one immutable
// snapshot. Only an admitted candidate may later become persisted Evidence.
type EvidenceCandidate struct {
	SourceID         research.SourceID
	SnapshotID       research.ID
	Kind             EvidenceCandidateKind
	Location         string
	Excerpt          string
	ExcerptHash      string
	ContextBefore    string
	ContextAfter     string
	Score            int
	Signals          []EvidenceSignal
	ExtractorVersion string
}

func (candidate EvidenceCandidate) Validate() error {
	if err := candidate.SourceID.Validate(); err != nil {
		return err
	}
	if err := candidate.SnapshotID.Validate(); err != nil {
		return fmt.Errorf("evidence candidate snapshot: %w", err)
	}
	if err := candidate.Kind.Validate(); err != nil {
		return err
	}
	if err := validateEvidenceCandidateText("evidence candidate location", candidate.Location, research.MaximumCitationSectionBytes, true); err != nil {
		return err
	}
	if err := validateEvidenceCandidateText("evidence candidate excerpt", candidate.Excerpt, MaximumEvidenceCandidateExcerptBytes, true); err != nil {
		return err
	}
	if candidate.ExcerptHash != research.CanonicalEvidenceExcerptHashV1(candidate.Excerpt) {
		return fmt.Errorf("evidence candidate excerpt hash does not match excerpt")
	}
	if err := validateEvidenceCandidateText("evidence candidate context before", candidate.ContextBefore, MaximumEvidenceCandidateContextBytes, false); err != nil {
		return err
	}
	if err := validateEvidenceCandidateText("evidence candidate context after", candidate.ContextAfter, MaximumEvidenceCandidateContextBytes, false); err != nil {
		return err
	}
	if candidate.Score < MinimumEvidenceCandidateScore || candidate.Score > 100 {
		return fmt.Errorf("evidence candidate score must be between %d and 100", MinimumEvidenceCandidateScore)
	}
	if len(candidate.Signals) == 0 || len(candidate.Signals) > MaximumEvidenceCandidateSignals {
		return fmt.Errorf("evidence candidate signals must contain between 1 and %d values", MaximumEvidenceCandidateSignals)
	}
	seen := make(map[EvidenceSignal]struct{}, len(candidate.Signals))
	for _, signal := range candidate.Signals {
		if err := signal.Validate(); err != nil {
			return err
		}
		if _, duplicate := seen[signal]; duplicate {
			return fmt.Errorf("evidence candidate repeats signal %q", signal)
		}
		seen[signal] = struct{}{}
	}
	if candidate.ExtractorVersion != EvidenceExtractorV1 {
		return fmt.Errorf("evidence candidate extractor must be %q", EvidenceExtractorV1)
	}
	return nil
}

func validateEvidenceCandidateText(name, value string, maximum int, required bool) error {
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

type EvidenceExtractionRequest struct {
	Topic         research.ResearchTopic
	Purpose       research.ResearchPurpose
	TargetVersion *research.SourceVersion
	Source        NormalizedSource
	Snapshot      research.SourceSnapshot
}

func (request EvidenceExtractionRequest) Validate() error {
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
	if err := request.Source.Validate(); err != nil {
		return fmt.Errorf("evidence normalized source: %w", err)
	}
	if err := request.Snapshot.Validate(); err != nil {
		return fmt.Errorf("evidence source snapshot: %w", err)
	}
	if request.Source.SourceID != request.Snapshot.SourceID || request.Source.Locator != request.Snapshot.Locator {
		return fmt.Errorf("normalized source does not match evidence snapshot")
	}
	return nil
}

type EvidenceExtractionResult struct {
	Candidates       []EvidenceCandidate
	AlgorithmVersion string
}

func (result EvidenceExtractionResult) Validate() error {
	if len(result.Candidates) > MaximumEvidenceCandidatesPerSource {
		return fmt.Errorf("evidence candidate count exceeds %d", MaximumEvidenceCandidatesPerSource)
	}
	for index, candidate := range result.Candidates {
		if err := candidate.Validate(); err != nil {
			return fmt.Errorf("evidence candidate %d: %w", index, err)
		}
	}
	if result.AlgorithmVersion != EvidenceExtractorV1 {
		return fmt.Errorf("evidence extraction algorithm must be %q", EvidenceExtractorV1)
	}
	return nil
}

// EvidenceExtractor deterministically selects bounded literal observations
// from one already-normalized source. Implementations must not perform I/O.
type EvidenceExtractor interface {
	Extract(context.Context, EvidenceExtractionRequest) (EvidenceExtractionResult, error)
}
