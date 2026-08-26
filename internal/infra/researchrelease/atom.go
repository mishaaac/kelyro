package researchrelease

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

type AtomProvider struct{ provider }

func NewAtomProvider(id string) (*AtomProvider, error) {
	base, err := newProvider(id)
	if err != nil {
		return nil, err
	}
	return &AtomProvider{provider: base}, nil
}

type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}
type atomEntry struct {
	Title      string         `xml:"title"`
	Updated    string         `xml:"updated"`
	Published  string         `xml:"published"`
	Summary    atomText       `xml:"summary"`
	Content    atomText       `xml:"content"`
	Categories []atomCategory `xml:"category"`
}
type atomText struct {
	Inner string `xml:",innerxml"`
}
type atomCategory struct {
	Term string `xml:"term,attr"`
}

func (provider *AtomProvider) Discover(ctx context.Context, fetched application.FetchedSource) ([]application.ReleaseCandidate, error) {
	if provider == nil {
		return nil, fmt.Errorf("Atom release provider is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateFetched(fetched); err != nil {
		return nil, err
	}
	contentType := strings.ToLower(fetched.Metadata.ContentType)
	if contentType != "application/atom+xml" && contentType != "application/xml" && !strings.HasSuffix(contentType, "+xml") {
		return nil, fmt.Errorf("Atom release provider received %q", fetched.Metadata.ContentType)
	}
	decoder := xml.NewDecoder(bytes.NewReader(fetched.Body))
	decoder.Strict = true
	var feed atomFeed
	if err := decoder.Decode(&feed); err != nil {
		return nil, fmt.Errorf("decode Atom release feed: %w", err)
	}
	if len(feed.Entries) > application.MaximumReleaseCandidates {
		return nil, fmt.Errorf("Atom feed exceeds %d entries", application.MaximumReleaseCandidates)
	}
	result := make([]application.ReleaseCandidate, 0, len(feed.Entries))
	for index, entry := range feed.Entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		version, err := versionFromTitle(entry.Title)
		if err != nil {
			return nil, fmt.Errorf("Atom entry %d: %w", index, err)
		}
		category := ""
		if len(entry.Categories) > 0 {
			category = entry.Categories[0].Term
		}
		channel, err := channelFromText(category, version)
		if err != nil {
			return nil, fmt.Errorf("Atom entry %d: %w", index, err)
		}
		releasedAt, err := atomTime(entry.Published, entry.Updated)
		if err != nil {
			return nil, fmt.Errorf("Atom entry %d: %w", index, err)
		}
		notes := entry.Content.Inner
		location := fmt.Sprintf("/feed/entry[%d]/content", index)
		if strings.TrimSpace(notes) == "" {
			notes, location = entry.Summary.Inner, fmt.Sprintf("/feed/entry[%d]/summary", index)
		}
		plain, err := xmlFragmentText(notes)
		if err != nil {
			return nil, fmt.Errorf("Atom entry %d notes: %w", index, err)
		}
		changes, err := changesFromText(location, plain)
		if err != nil {
			return nil, fmt.Errorf("Atom entry %d: %w", index, err)
		}
		result = append(result, application.ReleaseCandidate{
			Version: version, Channel: channel, ReleasedAt: releasedAt, Changes: changes,
		})
	}
	return result, nil
}

func atomTime(values ...string) (*research.Timestamp, error) {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("release date %q is not RFC3339", value)
		}
		timestamp, err := research.NewTimestamp(parsed)
		if err != nil {
			return nil, err
		}
		return &timestamp, nil
	}
	return nil, nil
}

func xmlFragmentText(fragment string) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader("<root>" + fragment + "</root>"))
	var builder strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if text, ok := token.(xml.CharData); ok {
			if builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			builder.Write(text)
		}
	}
	return strings.Join(strings.Fields(builder.String()), " "), nil
}

var _ application.ReleaseNotesProvider = (*AtomProvider)(nil)
