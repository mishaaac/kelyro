package application

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
)

const SourceClassifierV1 = "source-classifier-v1"

// SourceClassification is a deterministic classification made only after a
// successful fetch and normalization. Provider rank, title, and snippet are
// deliberately absent from the input and cannot influence the result.
type SourceClassification struct {
	SourceID         research.SourceID
	Kind             research.SourceKind
	AlgorithmVersion string
}

func (classification SourceClassification) Validate() error {
	if err := classification.SourceID.Validate(); err != nil {
		return err
	}
	if err := classification.Kind.Validate(); err != nil {
		return err
	}
	if classification.AlgorithmVersion != SourceClassifierV1 {
		return fmt.Errorf("source classification algorithm must be %q", SourceClassifierV1)
	}
	return nil
}

type SourceClassificationRequest struct {
	Source     research.Source
	Normalized NormalizedSource
}

func (request SourceClassificationRequest) Validate() error {
	if err := request.Source.Validate(); err != nil {
		return err
	}
	if err := request.Normalized.Validate(); err != nil {
		return err
	}
	if request.Source.ID != request.Normalized.SourceID || request.Source.Locator != request.Normalized.Locator {
		return errors.New("normalized source does not match source classification identity")
	}
	return nil
}

// SourceClassifier classifies fetched, normalized content without I/O.
type SourceClassifier interface {
	Classify(SourceClassificationRequest) (SourceClassification, error)
}

type deterministicSourceClassifierV1 struct{}

func NewDeterministicSourceClassifierV1() SourceClassifier {
	return deterministicSourceClassifierV1{}
}

func (deterministicSourceClassifierV1) Classify(request SourceClassificationRequest) (SourceClassification, error) {
	if err := request.Validate(); err != nil {
		return SourceClassification{}, err
	}
	kind := request.Source.Kind
	if kind == research.SourceOther {
		kind = classifyNormalizedLocatorV1(request.Normalized)
	}
	result := SourceClassification{SourceID: request.Source.ID, Kind: kind, AlgorithmVersion: SourceClassifierV1}
	if err := result.Validate(); err != nil {
		return SourceClassification{}, err
	}
	return result, nil
}

func classifyNormalizedLocatorV1(source NormalizedSource) research.SourceKind {
	locator := source.Locator
	if source.CanonicalLocator != nil {
		locator = *source.CanonicalLocator
	}
	parsed, err := url.Parse(locator.String())
	if err != nil {
		return research.SourceOther
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	path := strings.ToLower(parsed.EscapedPath())

	switch host {
	case "go.dev", "golang.org", "tip.golang.org":
		switch {
		case path == "/ref/spec", strings.HasPrefix(path, "/ref/spec/"):
			return research.SourceSpecification
		case strings.HasPrefix(path, "/doc/devel/release"), isGoReleaseNotesPathV1(path):
			return research.SourceReleaseNotes
		case strings.HasPrefix(path, "/blog/"):
			return research.SourceOfficialBlog
		case strings.HasPrefix(path, "/tour/"):
			return research.SourceOfficialTutorial
		case strings.HasPrefix(path, "/src/"):
			return research.SourceCode
		default:
			return research.SourceOfficialDocumentation
		}
	case "pkg.go.dev":
		return research.SourcePackageReference
	case "stackoverflow.com", "reddit.com", "discuss.python.org", "forum.golangbridge.org":
		return research.SourceCommunityForum
	case "youtube.com", "youtu.be", "vimeo.com":
		return research.SourceVideo
	case "arxiv.org":
		return research.SourcePaper
	case "github.com", "gitlab.com", "bitbucket.org":
		switch {
		case pathContainsSegmentV1(path, "issues"), pathContainsSegmentV1(path, "pull"):
			return research.SourceIssueTracker
		case pathContainsSegmentV1(path, "blob"), pathContainsSegmentV1(path, "tree"):
			return research.SourceCode
		}
	}

	contentType := strings.ToLower(strings.TrimSpace(strings.SplitN(source.ContentType, ";", 2)[0]))
	if contentType == "text/html" || contentType == "application/xhtml+xml" || contentType == "text/plain" || contentType == "text/markdown" {
		return research.SourceCommunityArticle
	}
	return research.SourceOther
}

func isGoReleaseNotesPathV1(path string) bool {
	if !strings.HasPrefix(path, "/doc/go1.") {
		return false
	}
	remainder := strings.TrimPrefix(path, "/doc/go1.")
	if remainder == "" {
		return false
	}
	for _, item := range remainder {
		if item == '/' {
			break
		}
		if item < '0' || item > '9' {
			return false
		}
	}
	return true
}

func pathContainsSegmentV1(path, want string) bool {
	for _, segment := range strings.Split(strings.Trim(path, "/"), "/") {
		if segment == want {
			return true
		}
	}
	return false
}

var _ SourceClassifier = deterministicSourceClassifierV1{}
