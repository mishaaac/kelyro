package research

import "fmt"

const (
	SourceTemporalPolicyV1      = "source-temporal-policy-v1"
	SourceTemporalLegacyCurrent = "source-temporal-legacy-current"
)

type SourceKind string

const (
	SourceOfficialDocumentation SourceKind = "official_documentation"
	SourceSpecification         SourceKind = "specification"
	SourceStandard              SourceKind = "standard"
	SourceReleaseNotes          SourceKind = "release_notes"
	SourceOfficialBlog          SourceKind = "official_blog"
	SourcePackageReference      SourceKind = "package_reference"
	SourceOfficialTutorial      SourceKind = "official_tutorial"
	SourceCode                  SourceKind = "source_code"
	SourceIssueTracker          SourceKind = "issue_tracker"
	SourceCommunityArticle      SourceKind = "community_article"
	SourceCommunityForum        SourceKind = "community_forum"
	SourceVideo                 SourceKind = "video"
	SourcePlayground            SourceKind = "playground"
	SourcePaper                 SourceKind = "paper"
	SourceBookReference         SourceKind = "book_reference"
	SourceOther                 SourceKind = "other"
)

func (kind SourceKind) Validate() error {
	switch kind {
	case SourceOfficialDocumentation, SourceSpecification, SourceStandard,
		SourceReleaseNotes, SourceOfficialBlog, SourcePackageReference,
		SourceOfficialTutorial, SourceCode, SourceIssueTracker,
		SourceCommunityArticle, SourceCommunityForum, SourceVideo,
		SourcePlayground, SourcePaper, SourceBookReference, SourceOther:
		return nil
	default:
		return fmt.Errorf("invalid source kind %q", kind)
	}
}

// SourceTemporalScope records how a source may be applied over time. It is
// independent from authority: an archived source can still be primary evidence
// for the exact historical version it documents.
type SourceTemporalScope string

const (
	SourceTemporalCurrent      SourceTemporalScope = "current"
	SourceTemporalHistorical   SourceTemporalScope = "historical"
	SourceTemporalVersionBound SourceTemporalScope = "version_bound"
	SourceTemporalArchived     SourceTemporalScope = "archived"
)

func (scope SourceTemporalScope) Validate() error {
	switch scope {
	case SourceTemporalCurrent, SourceTemporalHistorical,
		SourceTemporalVersionBound, SourceTemporalArchived:
		return nil
	default:
		return fmt.Errorf("invalid source temporal scope %q", scope)
	}
}

// Warning returns the immutable v1 annotation required when a non-current
// source is cited or included in a source bundle.
func (scope SourceTemporalScope) Warning(version *SourceVersion) (string, error) {
	if err := scope.Validate(); err != nil {
		return "", err
	}
	versionText := ""
	if version != nil {
		if err := version.Validate(); err != nil {
			return "", err
		}
		versionText = fmt.Sprintf(" for version %q", version.String())
	}
	switch scope {
	case SourceTemporalCurrent:
		return "", nil
	case SourceTemporalHistorical:
		return "Historical source" + versionText + "; do not treat as current guidance.", nil
	case SourceTemporalVersionBound:
		if version == nil {
			return "", fmt.Errorf("version-bound source requires a version")
		}
		return "Version-bound source" + versionText + "; verify applicability outside this version.", nil
	case SourceTemporalArchived:
		return "Archived source" + versionText + "; use only for historical or version-specific guidance.", nil
	default:
		panic("validated temporal scope is unreachable")
	}
}

// SourceMetadata describes a source independently from a particular fetch.
type SourceMetadata struct {
	Title       string
	Publisher   string
	Language    string
	PublishedAt *Timestamp
	UpdatedAt   *Timestamp
}

func (metadata SourceMetadata) Validate() error {
	if err := requireText("source title", metadata.Title); err != nil {
		return err
	}
	if err := validateOptionalText("source publisher", metadata.Publisher); err != nil {
		return err
	}
	if err := validateOptionalText("source language", metadata.Language); err != nil {
		return err
	}
	if err := validateOptionalTimestamp("source published at", metadata.PublishedAt); err != nil {
		return err
	}
	if err := validateOptionalTimestamp("source updated at", metadata.UpdatedAt); err != nil {
		return err
	}
	if metadata.PublishedAt != nil && metadata.UpdatedAt != nil && metadata.UpdatedAt.Before(*metadata.PublishedAt) {
		return fmt.Errorf("source updated at precedes published at")
	}
	return nil
}

// Source is the stable identity and classification of an external resource.
// Its content and fetch history live in immutable SourceSnapshots.
type Source struct {
	ID             SourceID
	Kind           SourceKind
	Locator        SourceLocator
	Version        *SourceVersion
	TemporalScope  SourceTemporalScope
	Metadata       SourceMetadata
	Specialization *SourceSpecialization
	Video          *VideoSupplementMetadata
	CreatedAt      Timestamp
}

func (source Source) Validate() error {
	if err := source.ID.Validate(); err != nil {
		return err
	}
	if err := source.Kind.Validate(); err != nil {
		return err
	}
	if err := source.Locator.Validate(); err != nil {
		return err
	}
	if source.Version != nil {
		if err := source.Version.Validate(); err != nil {
			return err
		}
	}
	if err := source.TemporalScope.Validate(); err != nil {
		return err
	}
	if source.TemporalScope == SourceTemporalVersionBound && source.Version == nil {
		return fmt.Errorf("version-bound source requires a version")
	}
	if err := source.Metadata.Validate(); err != nil {
		return err
	}
	if err := validateSourceSpecialization(source); err != nil {
		return err
	}
	if err := validateSourceVideoMetadata(source); err != nil {
		return err
	}
	return validateTimestamp("source created at", source.CreatedAt)
}

// FetchMetadata records adapter output without coupling the domain to net/http.
type FetchMetadata struct {
	StatusCode    int
	ContentType   string
	ETag          string
	LastModified  string
	ContentHash   string
	ContentLength int64
	FetchVersion  string
}

func (metadata FetchMetadata) Validate() error {
	if metadata.StatusCode < 100 || metadata.StatusCode > 599 {
		return fmt.Errorf("invalid fetch status code %d", metadata.StatusCode)
	}
	if metadata.StatusCode != 204 && metadata.StatusCode != 304 {
		if err := requireText("fetch content type", metadata.ContentType); err != nil {
			return err
		}
	} else if err := validateOptionalText("fetch content type", metadata.ContentType); err != nil {
		return err
	}
	if metadata.ContentLength < 0 {
		return fmt.Errorf("fetch content length is negative")
	}
	if err := validateOptionalText("fetch etag", metadata.ETag); err != nil {
		return err
	}
	if err := validateOptionalText("fetch last modified", metadata.LastModified); err != nil {
		return err
	}
	if err := validateOptionalText("fetch content hash", metadata.ContentHash); err != nil {
		return err
	}
	return requireText("fetch version", metadata.FetchVersion)
}

// SourceSnapshot is immutable evidence of a fetch at a particular instant.
// Body-retention policy is intentionally outside this record.
type SourceSnapshot struct {
	ID        ID
	SourceID  SourceID
	Locator   SourceLocator
	FetchedAt Timestamp
	Fetch     FetchMetadata
}

func (snapshot SourceSnapshot) Validate() error {
	if err := snapshot.ID.Validate(); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	if err := snapshot.SourceID.Validate(); err != nil {
		return err
	}
	if err := snapshot.Locator.Validate(); err != nil {
		return err
	}
	if err := validateTimestamp("snapshot fetched at", snapshot.FetchedAt); err != nil {
		return err
	}
	return snapshot.Fetch.Validate()
}
