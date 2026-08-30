package research

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestVideoSupplementMetadataIsHostNeutralBoundedAndCanonical(t *testing.T) {
	t.Parallel()
	metadata := videoMetadataFixture(t, SourceAffiliationOfficial)
	encoded, err := EncodeVideoSupplementMetadata(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(bytes.ToLower(encoded), []byte("youtube")) {
		t.Fatalf("provider-specific dependency leaked into metadata: %s", encoded)
	}
	parsed, err := ParseVideoSupplementMetadata(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Channel != metadata.Channel || len(parsed.DeepLinks) != 2 || parsed.TranscriptAvailability != TranscriptAvailable {
		t.Fatalf("parsed video metadata = %+v", parsed)
	}
	if _, err := ParseVideoSupplementMetadata(append([]byte(" "), encoded...)); err == nil || !strings.Contains(err.Error(), "canonical") {
		t.Fatalf("non-canonical JSON error = %v", err)
	}
	if _, err := ParseVideoSupplementMetadata([]byte(`{"video_locator":"https://media.example.test/watch/portable","channel":"Channel","duration_seconds":600,"affiliation":"official","transcript_availability":"available","deep_links":[],"algorithm_version":"video-supplement-metadata-v1","transcript":"raw"}`)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("transcript payload error = %v", err)
	}
}

func TestVideoSupplementDeepLinksAreExplicitAndDefensivelyCloned(t *testing.T) {
	t.Parallel()
	metadata := videoMetadataFixture(t, SourceAffiliationCommunity)
	locator, found, err := metadata.DeepLinkAt(90)
	if err != nil || !found || locator != metadata.DeepLinks[0].Locator {
		t.Fatalf("DeepLinkAt(90) = (%s,%v,%v)", locator.String(), found, err)
	}
	if _, found, err := metadata.DeepLinkAt(91); err != nil || found {
		t.Fatalf("DeepLinkAt(91) = (%v,%v)", found, err)
	}
	clone := metadata.Clone()
	clone.DeepLinks[0].OffsetSeconds = 120
	if metadata.DeepLinks[0].OffsetSeconds != 90 {
		t.Fatal("video metadata clone leaked mutable deep links")
	}
}

func TestVideoSupplementValidationRejectsContradictionsAndUnboundedData(t *testing.T) {
	t.Parallel()
	valid := videoMetadataFixture(t, SourceAffiliationOfficial)
	tests := []struct {
		name   string
		mutate func(*VideoSupplementMetadata)
		needle string
	}{
		{"algorithm", func(value *VideoSupplementMetadata) { value.AlgorithmVersion = "video-v2" }, "algorithm version"},
		{"duration", func(value *VideoSupplementMetadata) { value.DurationSeconds = 0 }, "duration"},
		{"description", func(value *VideoSupplementMetadata) {
			value.Description = strings.Repeat("x", MaximumVideoDescriptionBytes+1)
		}, "description"},
		{"affiliation", func(value *VideoSupplementMetadata) { value.Affiliation = "partner" }, "affiliation"},
		{"transcript", func(value *VideoSupplementMetadata) { value.TranscriptAvailability = "complete_text" }, "transcript availability"},
		{"offset", func(value *VideoSupplementMetadata) { value.DeepLinks[0].OffsetSeconds = value.DurationSeconds }, "within the video duration"},
		{"ordering", func(value *VideoSupplementMetadata) { value.DeepLinks[1].OffsetSeconds = 30 }, "ordered"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := *valid.Clone()
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil || !strings.Contains(err.Error(), test.needle) {
				t.Fatalf("Validate() error = %v, want %q", err, test.needle)
			}
		})
	}
}

func TestSourceValidatesNormalizedVideoMetadataRelationship(t *testing.T) {
	t.Parallel()
	source := videoSourceFixture(t, SourceAffiliationOfficial)
	if err := source.Validate(); err != nil {
		t.Fatal(err)
	}
	source.Metadata.Publisher = ""
	if err := source.Validate(); err == nil || !strings.Contains(err.Error(), "publisher") {
		t.Fatalf("missing publisher error = %v", err)
	}
	source = videoSourceFixture(t, SourceAffiliationOfficial)
	source.Metadata.PublishedAt = nil
	if err := source.Validate(); err == nil || !strings.Contains(err.Error(), "published_at") {
		t.Fatalf("missing published_at error = %v", err)
	}
	source = videoSourceFixture(t, SourceAffiliationOfficial)
	other, _ := NewSourceLocator("https://media.example.test/watch/other")
	source.Video.VideoLocator = other
	if err := source.Validate(); err == nil || !strings.Contains(err.Error(), "must match") {
		t.Fatalf("locator mismatch error = %v", err)
	}
}

func videoSourceFixture(t *testing.T, affiliation SourceAffiliation) Source {
	t.Helper()
	published, _ := NewTimestamp(time.Date(2026, time.July, 20, 14, 0, 0, 0, time.UTC))
	id, _ := NewSourceID("source.video.fixture")
	metadata := videoMetadataFixture(t, affiliation)
	return Source{
		ID: id, Kind: SourceVideo, Locator: metadata.VideoLocator, TemporalScope: SourceTemporalCurrent,
		Metadata: SourceMetadata{Title: "Portable concurrency explained", Publisher: "Open Systems Conference", Language: "en", PublishedAt: &published},
		Video:    metadata.Clone(), CreatedAt: published,
	}
}

func videoMetadataFixture(t *testing.T, affiliation SourceAffiliation) VideoSupplementMetadata {
	t.Helper()
	video, _ := NewSourceLocator("https://media.example.test/watch/portable")
	link90, _ := NewSourceLocator("https://media.example.test/watch/portable?at=90")
	link300, _ := NewSourceLocator("https://media.example.test/watch/portable?at=300")
	return VideoSupplementMetadata{
		VideoLocator: video, Channel: "Open Systems Sessions", DurationSeconds: 900,
		Description: "A reviewed, provider-neutral conference session.", Affiliation: affiliation,
		TranscriptAvailability: TranscriptAvailable,
		DeepLinks:              []VideoDeepLink{{OffsetSeconds: 90, Locator: link90}, {OffsetSeconds: 300, Locator: link300}},
		AlgorithmVersion:       VideoSupplementMetadataV1,
	}
}
