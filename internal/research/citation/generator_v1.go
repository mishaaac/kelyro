package citation

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"

	"github.com/mishaaac/kelyro/internal/research"
)

var commitPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)

// Target carries only location metadata observed outside the generator. An
// empty Anchor selects the canonical fallback. Source code requires an
// immutable permalink instead of an anchor on a mutable branch URL.
type Target struct {
	Anchor    string
	Section   string
	Label     string
	Permalink *SourcePermalink
}

// SourcePermalink describes a source-host URL pinned to an immutable commit
// and exact line range. Locator is supplied by an adapter because source hosts
// do not share a universal URL layout; the remaining fields let v1 verify it.
type SourcePermalink struct {
	Locator   research.SourceLocator
	Commit    string
	FilePath  string
	StartLine int
	EndLine   int
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
	strategy, deepLink, err := selectDeepLink(request.Source, request.Target, section)
	if err != nil {
		return research.Citation{}, err
	}
	var version *research.SourceVersion
	if request.Source.Version != nil {
		copy := *request.Source.Version
		version = &copy
	}
	citation := research.Citation{
		ID: request.ID, SourceID: request.Source.ID, SnapshotID: request.Snapshot.ID,
		EvidenceID: request.Evidence.ID, Title: request.Source.Metadata.Title,
		Locator: request.Source.Locator, DeepLink: deepLink, LinkStrategy: strategy,
		Section: section, SnapshotDate: request.Snapshot.FetchedAt, VersionScope: version,
		LastVerified: request.LastVerified, AlgorithmVersion: research.CitationAlgorithmV1,
	}
	if err := research.ValidateCitationRelationships(citation, request.Source, request.Snapshot, request.Evidence); err != nil {
		return research.Citation{}, err
	}
	return citation, nil
}

func selectDeepLink(source research.Source, target Target, section string) (research.CitationLinkStrategy, *research.DeepLink, error) {
	if source.Kind == research.SourceCode {
		if target.Anchor != "" {
			return "", nil, fmt.Errorf("source code citations require a commit permalink, not a mutable anchor")
		}
		if target.Permalink == nil {
			return research.CitationCanonicalFallback, nil, nil
		}
		locator, err := validateSourcePermalink(source.Locator, *target.Permalink)
		if err != nil {
			return "", nil, fmt.Errorf("source permalink: %w", err)
		}
		label := target.Label
		if label == "" {
			label = section
		}
		return research.CitationSourcePermalink, &research.DeepLink{Locator: locator, Label: label}, nil
	}
	if target.Permalink != nil {
		return "", nil, fmt.Errorf("source permalink strategy requires a source_code source")
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

func validateSourcePermalink(canonical research.SourceLocator, permalink SourcePermalink) (research.SourceLocator, error) {
	if err := permalink.Locator.Validate(); err != nil {
		return research.SourceLocator{}, err
	}
	if !commitPattern.MatchString(permalink.Commit) {
		return research.SourceLocator{}, fmt.Errorf("commit is not a 7-64 character hexadecimal revision")
	}
	if permalink.FilePath == "" || strings.Contains(permalink.FilePath, `\`) ||
		path.IsAbs(permalink.FilePath) || path.Clean(permalink.FilePath) != permalink.FilePath ||
		strings.HasPrefix(permalink.FilePath, "../") {
		return research.SourceLocator{}, fmt.Errorf("file path is not a clean relative path")
	}
	if permalink.StartLine < 1 || permalink.EndLine < permalink.StartLine {
		return research.SourceLocator{}, fmt.Errorf("line range is invalid")
	}
	canonicalURL, err := url.Parse(canonical.String())
	if err != nil {
		return research.SourceLocator{}, fmt.Errorf("parse canonical locator: %w", err)
	}
	deepURL, err := url.Parse(permalink.Locator.String())
	if err != nil {
		return research.SourceLocator{}, fmt.Errorf("parse permalink locator: %w", err)
	}
	if !strings.EqualFold(canonicalURL.Host, deepURL.Host) {
		return research.SourceLocator{}, fmt.Errorf("permalink host does not match canonical source")
	}
	segments := strings.Split(strings.Trim(deepURL.Path, "/"), "/")
	foundCommit := false
	for _, segment := range segments {
		if strings.EqualFold(segment, permalink.Commit) {
			foundCommit = true
			break
		}
	}
	if !foundCommit || !strings.HasSuffix(strings.Trim(deepURL.Path, "/"), "/"+permalink.FilePath) {
		return research.SourceLocator{}, fmt.Errorf("permalink path does not contain the commit and file")
	}
	wantFragment := fmt.Sprintf("L%d", permalink.StartLine)
	if permalink.EndLine > permalink.StartLine {
		wantFragment += fmt.Sprintf("-L%d", permalink.EndLine)
	}
	if deepURL.Fragment != wantFragment {
		return research.SourceLocator{}, fmt.Errorf("permalink fragment is %q, want %q", deepURL.Fragment, wantFragment)
	}
	return permalink.Locator, nil
}
