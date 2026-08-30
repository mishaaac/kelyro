package researchrelease

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

// JSONProvider supports common official API shapes, including a top-level
// array and an object containing a releases/items/results array. Field aliases
// cover generic registries and repository release APIs without coupling the
// application port to one vendor.
type JSONProvider struct{ provider }

func NewJSONProvider(id string) (*JSONProvider, error) {
	base, err := newProvider(id)
	if err != nil {
		return nil, err
	}
	return &JSONProvider{provider: base}, nil
}

func (provider *JSONProvider) Discover(ctx context.Context, fetched application.FetchedSource) ([]application.ReleaseCandidate, error) {
	if provider == nil {
		return nil, fmt.Errorf("JSON release provider is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateFetched(fetched); err != nil {
		return nil, err
	}
	if contentType := strings.ToLower(fetched.Metadata.ContentType); contentType != "application/json" && !strings.HasSuffix(contentType, "+json") {
		return nil, fmt.Errorf("JSON release provider received %q", fetched.Metadata.ContentType)
	}
	decoder := json.NewDecoder(bytes.NewReader(fetched.Body))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode release JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("release JSON contains trailing data")
	}
	entries, err := jsonReleaseEntries(document)
	if err != nil {
		return nil, err
	}
	if len(entries) > application.MaximumReleaseCandidates {
		return nil, fmt.Errorf("release JSON exceeds %d entries", application.MaximumReleaseCandidates)
	}
	result := make([]application.ReleaseCandidate, 0, len(entries))
	for index, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		candidate, skip, err := jsonReleaseCandidate(entry, index)
		if err != nil {
			return nil, err
		}
		if !skip {
			result = append(result, candidate)
		}
	}
	return result, nil
}

func jsonReleaseEntries(document any) ([]map[string]any, error) {
	var values []any
	switch root := document.(type) {
	case []any:
		values = root
	case map[string]any:
		for _, key := range []string{"releases", "items", "results"} {
			if found, exists := root[key]; exists {
				var ok bool
				values, ok = found.([]any)
				if !ok {
					return nil, fmt.Errorf("release JSON field %q is not an array", key)
				}
				break
			}
		}
		if values == nil {
			return nil, fmt.Errorf("release JSON has no releases array")
		}
	default:
		return nil, fmt.Errorf("release JSON root is not an array or object")
	}
	result := make([]map[string]any, len(values))
	for index, value := range values {
		entry, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("release JSON entry %d is not an object", index)
		}
		result[index] = entry
	}
	return result, nil
}

func jsonReleaseCandidate(entry map[string]any, index int) (application.ReleaseCandidate, bool, error) {
	if draft, _ := entry["draft"].(bool); draft {
		return application.ReleaseCandidate{}, true, nil
	}
	versionText := firstJSONString(entry, "version", "tag_name", "name")
	if versionText == "" {
		return application.ReleaseCandidate{}, false, fmt.Errorf("release JSON entry %d has no version", index)
	}
	versionText = strings.TrimPrefix(strings.TrimSpace(versionText), "v")
	version, err := research.NewVersionIdentifier(versionText)
	if err != nil {
		return application.ReleaseCandidate{}, false, fmt.Errorf("release JSON entry %d version: %w", index, err)
	}
	channelText := firstJSONString(entry, "channel", "release_channel")
	if channelText == "" {
		if prerelease, _ := entry["prerelease"].(bool); prerelease {
			channelText = "preview"
		}
	}
	channel, err := channelFromText(channelText, version)
	if err != nil {
		return application.ReleaseCandidate{}, false, fmt.Errorf("release JSON entry %d: %w", index, err)
	}
	releasedAt, err := firstJSONTime(entry, "released_at", "published_at", "created_at", "date")
	if err != nil {
		return application.ReleaseCandidate{}, false, fmt.Errorf("release JSON entry %d: %w", index, err)
	}
	notes, err := jsonNotes(entry, index)
	if err != nil {
		return application.ReleaseCandidate{}, false, err
	}
	return application.ReleaseCandidate{Version: version, Channel: channel, ReleasedAt: releasedAt, Changes: notes}, false, nil
}

func jsonNotes(entry map[string]any, index int) ([]application.ReleaseChange, error) {
	for _, key := range []string{"notes", "body", "description", "changelog"} {
		value, exists := entry[key]
		if !exists {
			continue
		}
		switch notes := value.(type) {
		case string:
			return changesFromText(fmt.Sprintf("/releases/%d/%s", index, key), notes)
		case []any:
			result := make([]application.ReleaseChange, 0, len(notes))
			for noteIndex, note := range notes {
				text, ok := note.(string)
				if !ok {
					return nil, fmt.Errorf("release JSON entry %d notes contain a non-string", index)
				}
				changes, err := changesFromText(fmt.Sprintf("/releases/%d/%s/%d", index, key, noteIndex), text)
				if err != nil {
					return nil, fmt.Errorf("release JSON entry %d: %w", index, err)
				}
				result = append(result, changes...)
				if len(result) > application.MaximumReleaseChangesPerCandidate {
					return nil, fmt.Errorf("release JSON entry %d has too many notes", index)
				}
			}
			return result, nil
		default:
			return nil, fmt.Errorf("release JSON entry %d field %q is not text or an array", index, key)
		}
	}
	return nil, nil
}

func firstJSONString(entry map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := entry[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstJSONTime(entry map[string]any, keys ...string) (*research.Timestamp, error) {
	value := firstJSONString(entry, keys...)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("release date %q is not RFC3339", value)
	}
	timestamp, err := research.NewTimestamp(parsed)
	if err != nil {
		return nil, err
	}
	return &timestamp, nil
}

var _ application.ReleaseNotesProvider = (*JSONProvider)(nil)
