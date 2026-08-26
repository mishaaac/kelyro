package researchrelease

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

var versionInTitle = regexp.MustCompile(`(?i)(?:^|[[:space:]])v?([0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?)(?:$|[[:space:]:-])`)

type provider struct{ id string }

func newProvider(id string) (provider, error) {
	id = strings.TrimSpace(id)
	if id == "" || strings.ContainsAny(id, "\r\n\t") {
		return provider{}, fmt.Errorf("release provider id is invalid")
	}
	return provider{id: id}, nil
}

func (value provider) ID() string { return value.id }

func validateFetched(fetched application.FetchedSource) error {
	if err := fetched.Validate(); err != nil {
		return fmt.Errorf("validate fetched release feed: %w", err)
	}
	if fetched.Metadata.StatusCode < http.StatusOK || fetched.Metadata.StatusCode >= http.StatusMultipleChoices ||
		fetched.Metadata.StatusCode == http.StatusNoContent {
		return fmt.Errorf("release feed has no parseable representation")
	}
	if int64(len(fetched.Body)) != fetched.Metadata.ContentLength ||
		fetched.Metadata.ContentHash != research.CanonicalContentHashV1(fetched.Body) {
		return fmt.Errorf("release feed body integrity is invalid")
	}
	if !utf8.Valid(fetched.Body) {
		return fmt.Errorf("release feed is not valid UTF-8")
	}
	return nil
}

func versionFromTitle(title string) (research.VersionIdentifier, error) {
	matches := versionInTitle.FindStringSubmatch(strings.TrimSpace(title))
	if matches == nil {
		return "", fmt.Errorf("release title %q has no supported version", title)
	}
	return research.NewVersionIdentifier(matches[1])
}

func channelFromText(value string, version research.VersionIdentifier) (research.ReleaseChannel, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "stable", "latest", "release":
		if semantic, ok := version.Semantic(); ok && semantic.IsPrerelease() {
			return channelFromPrerelease(semantic.Prerelease), nil
		}
		return research.ReleaseStable, nil
	case "preview", "prerelease", "pre-release":
		return research.ReleasePreview, nil
	case "beta":
		return research.ReleaseBeta, nil
	case "rc", "release-candidate", "release_candidate":
		return research.ReleaseRC, nil
	case "experimental", "alpha":
		return research.ReleaseExperimental, nil
	case "nightly", "dev", "canary":
		return research.ReleaseNightly, nil
	case "unknown":
		return research.ReleaseChannelUnknown, nil
	default:
		return "", fmt.Errorf("unsupported release channel %q", value)
	}
}

func channelFromPrerelease(value string) research.ReleaseChannel {
	lower := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lower, "rc"):
		return research.ReleaseRC
	case strings.HasPrefix(lower, "beta") || strings.HasPrefix(lower, "b."):
		return research.ReleaseBeta
	case strings.HasPrefix(lower, "nightly") || strings.HasPrefix(lower, "dev") || strings.HasPrefix(lower, "canary"):
		return research.ReleaseNightly
	case strings.HasPrefix(lower, "alpha") || strings.HasPrefix(lower, "experimental"):
		return research.ReleaseExperimental
	default:
		return research.ReleasePreview
	}
}

func changesFromText(location, value string) ([]application.ReleaseChange, error) {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	paragraphs := strings.Split(value, "\n")
	result := make([]application.ReleaseChange, 0, len(paragraphs))
	for index, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(strings.TrimLeft(paragraph, "-*+• \t"))
		paragraph = strings.Join(strings.Fields(paragraph), " ")
		if paragraph == "" {
			continue
		}
		for _, piece := range splitUTF8(paragraph, research.MaximumEvidenceExcerptBytes) {
			if len(result) == application.MaximumReleaseChangesPerCandidate {
				return nil, fmt.Errorf("release notes exceed %d bounded changes", application.MaximumReleaseChangesPerCandidate)
			}
			result = append(result, application.ReleaseChange{
				Location: fmt.Sprintf("%s[%d]", location, index), Statement: piece, Excerpt: piece,
			})
		}
	}
	return result, nil
}

func splitUTF8(value string, maximum int) []string {
	if len(value) <= maximum {
		return []string{value}
	}
	result := make([]string, 0, len(value)/maximum+1)
	for len(value) > maximum {
		cut := maximum
		for cut > 0 && !utf8.RuneStart(value[cut]) {
			cut--
		}
		if space := strings.LastIndexByte(value[:cut], ' '); space > maximum/2 {
			cut = space
		}
		result = append(result, strings.TrimSpace(value[:cut]))
		value = strings.TrimSpace(value[cut:])
	}
	if value != "" {
		result = append(result, value)
	}
	return result
}
