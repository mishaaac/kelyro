package citation

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/mishaaac/kelyro/internal/research"
)

// Target carries only location metadata observed outside the generator. An
// empty Anchor selects the canonical fallback. Source code locations are read
// from the persisted Evidence record rather than accepted as duplicate input.
type Target struct {
	Anchor  string
	Section string
	Label   string
}

type Request struct {
	ID           research.ID
	Source       research.Source
	Snapshot     research.SourceSnapshot
	Evidence     research.Evidence
	LastVerified research.Timestamp
	Target       Target
}

// GenerateV1 returns a citation tied to one exact source/snapshot/evidence
// chain. It never fetches a page or guesses a heading slug.
func GenerateV1(request Request) (research.Citation, error) {
	if err := request.ID.Validate(); err != nil {
		return research.Citation{}, fmt.Errorf("citation id: %w", err)
	}
	if err := request.Source.Validate(); err != nil {
		return research.Citation{}, fmt.Errorf("citation source: %w", err)
	}
	if err := request.Snapshot.Validate(); err != nil {
		return research.Citation{}, fmt.Errorf("citation snapshot: %w", err)
	}
	if err := request.Evidence.Validate(); err != nil {
		return research.Citation{}, fmt.Errorf("citation evidence: %w", err)
	}
	if request.Snapshot.SourceID != request.Source.ID ||
		request.Evidence.SourceID != request.Source.ID ||
		request.Evidence.SnapshotID != request.Snapshot.ID {
		return research.Citation{}, fmt.Errorf("citation source, snapshot, and evidence relationships do not match")
	}
	if err := request.LastVerified.Validate(); err != nil {
		return research.Citation{}, fmt.Errorf("citation last verified: %w", err)
	}

	section := request.Target.Section
	if section == "" {
		section = request.Evidence.Location
	}
	strategy, deepLink, err := selectDeepLink(request.Source, request.Evidence, request.Target, section)
	if err != nil {
		return research.Citation{}, err
	}
	var version *research.SourceVersion
	if request.Source.Version != nil {
		copy := *request.Source.Version
		version = &copy
	}
	temporalWarning, err := request.Source.TemporalScope.Warning(request.Source.Version)
	if err != nil {
		return research.Citation{}, err
	}
	citation := research.Citation{
		ID: request.ID, SourceID: request.Source.ID, SnapshotID: request.Snapshot.ID,
		EvidenceID: request.Evidence.ID, Title: request.Source.Metadata.Title,
		Locator: request.Source.Locator, DeepLink: deepLink, LinkStrategy: strategy,
		Section: section, SnapshotDate: request.Snapshot.FetchedAt, VersionScope: version,
		TemporalScope: request.Source.TemporalScope, TemporalWarning: temporalWarning,
		LastVerified: request.LastVerified, AlgorithmVersion: research.CitationAlgorithmV1,
		TemporalAlgorithmVersion: research.SourceTemporalPolicyV1,
	}
	if err := research.ValidateCitationRelationships(citation, request.Source, request.Snapshot, request.Evidence); err != nil {
		return research.Citation{}, err
	}
	return citation, nil
}

func selectDeepLink(source research.Source, evidence research.Evidence, target Target, section string) (research.CitationLinkStrategy, *research.DeepLink, error) {
	if source.Kind == research.SourceCode {
		if target.Anchor != "" {
			return "", nil, fmt.Errorf("source code citations require a commit permalink, not a mutable anchor")
		}
		if err := research.ValidateSourceCodeEvidenceRelationship(source, evidence); err != nil {
			return "", nil, err
		}
		label := target.Label
		if label == "" {
			label = section
		}
		return research.CitationSourcePermalink, &research.DeepLink{Locator: evidence.SourceCode.Permalink, Label: label}, nil
	}
	if target.Anchor == "" {
		return research.CitationCanonicalFallback, nil, nil
	}
	locator, err := locatorWithFragment(source.Locator, target.Anchor)
	if err != nil {
		return "", nil, err
	}
	label := target.Label
	if label == "" {
		label = section
	}
	return strategyFor(source.Kind), &research.DeepLink{Locator: locator, Label: label}, nil
}

func strategyFor(kind research.SourceKind) research.CitationLinkStrategy {
	switch kind {
	case research.SourcePackageReference:
		return research.CitationPackageSymbol
	case research.SourceSpecification, research.SourceStandard:
		return research.CitationSpecification
	case research.SourceReleaseNotes:
		return research.CitationReleaseHeading
	default:
		return research.CitationURLAnchor
	}
}

func locatorWithFragment(canonical research.SourceLocator, fragment string) (research.SourceLocator, error) {
	if strings.TrimSpace(fragment) == "" || fragment != strings.TrimSpace(fragment) || strings.HasPrefix(fragment, "#") {
		return research.SourceLocator{}, fmt.Errorf("citation anchor is invalid")
	}
	if strings.IndexFunc(fragment, unicode.IsControl) >= 0 {
		return research.SourceLocator{}, fmt.Errorf("citation anchor contains control characters")
	}
	parsed, err := url.Parse(canonical.String())
	if err != nil {
		return research.SourceLocator{}, fmt.Errorf("parse canonical locator: %w", err)
	}
	parsed.Fragment = fragment
	locator, err := research.NewSourceLocator(parsed.String())
	if err != nil {
		return research.SourceLocator{}, fmt.Errorf("deep link locator: %w", err)
	}
	return locator, nil
}
