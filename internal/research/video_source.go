package research

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
)

const (
	VideoSupplementMetadataV1     = "video-supplement-metadata-v1"
	MaximumVideoMetadataJSONBytes = 16384
	MaximumVideoDescriptionBytes  = 4096
	MaximumVideoDeepLinks         = 32
	MaximumVideoDurationSeconds   = 7 * 24 * 60 * 60
	maximumVideoMetadataTextBytes = 512
)

type TranscriptAvailability string

const (
	TranscriptAvailable   TranscriptAvailability = "available"
	TranscriptPartial     TranscriptAvailability = "partial"
	TranscriptUnavailable TranscriptAvailability = "unavailable"
	TranscriptUnknown     TranscriptAvailability = "unknown"
)

func (availability TranscriptAvailability) Validate() error {
	switch availability {
	case TranscriptAvailable, TranscriptPartial, TranscriptUnavailable, TranscriptUnknown:
		return nil
	default:
		return fmt.Errorf("invalid transcript availability %q", availability)
	}
}

// VideoDeepLink is adapter-supplied because timestamp URL formats are host
// specific. The domain records an exact locator and offset without constructing
// a provider-specific URL.
type VideoDeepLink struct {
	OffsetSeconds int
	Locator       SourceLocator
}

func (link VideoDeepLink) Validate(durationSeconds int) error {
	if link.OffsetSeconds <= 0 || link.OffsetSeconds >= durationSeconds {
		return fmt.Errorf("video deep-link offset must be within the video duration")
	}
	if err := link.Locator.Validate(); err != nil {
		return fmt.Errorf("video deep-link locator: %w", err)
	}
	return nil
}

// VideoSupplementMetadata contains only video-specific metadata. URL, title,
// publisher, and published_at remain normalized in Source and SourceMetadata.
// Transcript text is deliberately absent from this contract.
type VideoSupplementMetadata struct {
	VideoLocator           SourceLocator
	Channel                string
	DurationSeconds        int
	Description            string
	Affiliation            SourceAffiliation
	TranscriptAvailability TranscriptAvailability
	DeepLinks              []VideoDeepLink
	AlgorithmVersion       string
}

func (metadata VideoSupplementMetadata) Validate() error {
	if metadata.AlgorithmVersion != VideoSupplementMetadataV1 {
		return fmt.Errorf("video metadata algorithm version must be %q", VideoSupplementMetadataV1)
	}
	if err := metadata.VideoLocator.Validate(); err != nil {
		return fmt.Errorf("video locator: %w", err)
	}
	if err := validateVideoText("video channel", metadata.Channel, false, maximumVideoMetadataTextBytes); err != nil {
		return err
	}
	if metadata.DurationSeconds <= 0 || metadata.DurationSeconds > MaximumVideoDurationSeconds {
		return fmt.Errorf("video duration must be between 1 and %d seconds", MaximumVideoDurationSeconds)
	}
	if err := validateVideoText("video description", metadata.Description, true, MaximumVideoDescriptionBytes); err != nil {
		return err
	}
	if err := metadata.Affiliation.Validate(); err != nil {
		return err
	}
	if err := metadata.TranscriptAvailability.Validate(); err != nil {
		return err
	}
	if len(metadata.DeepLinks) > MaximumVideoDeepLinks {
		return fmt.Errorf("video deep links exceed %d", MaximumVideoDeepLinks)
	}
	seenOffsets := make(map[int]struct{}, len(metadata.DeepLinks))
	seenLocators := make(map[string]struct{}, len(metadata.DeepLinks))
	previousOffset := 0
	for index, link := range metadata.DeepLinks {
		if err := link.Validate(metadata.DurationSeconds); err != nil {
			return fmt.Errorf("video deep link %d: %w", index, err)
		}
		if index > 0 && link.OffsetSeconds <= previousOffset {
			return fmt.Errorf("video deep links must be ordered by increasing offset")
		}
		if _, exists := seenOffsets[link.OffsetSeconds]; exists {
			return fmt.Errorf("video deep links contain duplicate offset %d", link.OffsetSeconds)
		}
		if _, exists := seenLocators[link.Locator.String()]; exists {
			return fmt.Errorf("video deep links contain duplicate locator")
		}
		seenOffsets[link.OffsetSeconds] = struct{}{}
		seenLocators[link.Locator.String()] = struct{}{}
		previousOffset = link.OffsetSeconds
	}
	return nil
}

func (metadata VideoSupplementMetadata) Clone() *VideoSupplementMetadata {
	clone := metadata
	clone.DeepLinks = append([]VideoDeepLink(nil), metadata.DeepLinks...)
	return &clone
}

// DeepLinkAt returns only an explicitly stored provider-supplied link. It does
// not invent a URL format when an offset is not known.
func (metadata VideoSupplementMetadata) DeepLinkAt(offsetSeconds int) (SourceLocator, bool, error) {
	if err := metadata.Validate(); err != nil {
		return SourceLocator{}, false, err
	}
	for _, link := range metadata.DeepLinks {
		if link.OffsetSeconds == offsetSeconds {
			return link.Locator, true, nil
		}
	}
	return SourceLocator{}, false, nil
}

func validateSourceVideoMetadata(source Source) error {
	if source.Video == nil {
		return nil
	}
	if source.Kind != SourceVideo {
		return fmt.Errorf("only video sources can carry video supplement metadata")
	}
	if err := source.Video.Validate(); err != nil {
		return err
	}
	if source.Locator != source.Video.VideoLocator {
		return fmt.Errorf("source locator must match video locator")
	}
	if strings.TrimSpace(source.Metadata.Publisher) == "" {
		return fmt.Errorf("video supplement requires source publisher metadata")
	}
	if source.Metadata.PublishedAt == nil {
		return fmt.Errorf("video supplement requires source published_at metadata")
	}
	return nil
}

type videoSupplementMetadataJSON struct {
	VideoLocator           string              `json:"video_locator"`
	Channel                string              `json:"channel"`
	DurationSeconds        int                 `json:"duration_seconds"`
	Description            string              `json:"description,omitempty"`
	Affiliation            string              `json:"affiliation"`
	TranscriptAvailability string              `json:"transcript_availability"`
	DeepLinks              []videoDeepLinkJSON `json:"deep_links"`
	AlgorithmVersion       string              `json:"algorithm_version"`
}

type videoDeepLinkJSON struct {
	OffsetSeconds int    `json:"offset_seconds"`
	Locator       string `json:"locator"`
}

func EncodeVideoSupplementMetadata(metadata VideoSupplementMetadata) ([]byte, error) {
	if err := metadata.Validate(); err != nil {
		return nil, err
	}
	payload := videoSupplementMetadataJSON{
		VideoLocator: metadata.VideoLocator.String(), Channel: metadata.Channel,
		DurationSeconds: metadata.DurationSeconds, Description: metadata.Description,
		Affiliation: string(metadata.Affiliation), TranscriptAvailability: string(metadata.TranscriptAvailability),
		DeepLinks: make([]videoDeepLinkJSON, len(metadata.DeepLinks)), AlgorithmVersion: metadata.AlgorithmVersion,
	}
	for index, link := range metadata.DeepLinks {
		payload.DeepLinks[index] = videoDeepLinkJSON{OffsetSeconds: link.OffsetSeconds, Locator: link.Locator.String()}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode video supplement metadata: %w", err)
	}
	if len(encoded) > MaximumVideoMetadataJSONBytes {
		return nil, fmt.Errorf("video supplement metadata exceeds %d bytes", MaximumVideoMetadataJSONBytes)
	}
	return encoded, nil
}

func ParseVideoSupplementMetadata(encoded []byte) (VideoSupplementMetadata, error) {
	if len(encoded) == 0 || len(encoded) > MaximumVideoMetadataJSONBytes {
		return VideoSupplementMetadata{}, fmt.Errorf("video supplement metadata size must be between 1 and %d bytes", MaximumVideoMetadataJSONBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var payload videoSupplementMetadataJSON
	if err := decoder.Decode(&payload); err != nil {
		return VideoSupplementMetadata{}, fmt.Errorf("decode video supplement metadata: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return VideoSupplementMetadata{}, fmt.Errorf("decode video supplement metadata: trailing data")
	}
	videoLocator, err := NewSourceLocator(payload.VideoLocator)
	if err != nil {
		return VideoSupplementMetadata{}, err
	}
	metadata := VideoSupplementMetadata{
		VideoLocator: videoLocator, Channel: payload.Channel, DurationSeconds: payload.DurationSeconds,
		Description: payload.Description, Affiliation: SourceAffiliation(payload.Affiliation),
		TranscriptAvailability: TranscriptAvailability(payload.TranscriptAvailability),
		DeepLinks:              make([]VideoDeepLink, len(payload.DeepLinks)), AlgorithmVersion: payload.AlgorithmVersion,
	}
	for index, link := range payload.DeepLinks {
		locator, locatorErr := NewSourceLocator(link.Locator)
		if locatorErr != nil {
			return VideoSupplementMetadata{}, locatorErr
		}
		metadata.DeepLinks[index] = VideoDeepLink{OffsetSeconds: link.OffsetSeconds, Locator: locator}
	}
	if err := metadata.Validate(); err != nil {
		return VideoSupplementMetadata{}, err
	}
	canonical, err := EncodeVideoSupplementMetadata(metadata)
	if err != nil {
		return VideoSupplementMetadata{}, err
	}
	if !bytes.Equal(encoded, canonical) {
		return VideoSupplementMetadata{}, fmt.Errorf("video supplement metadata is not canonical")
	}
	return metadata, nil
}

func validateVideoText(name, value string, optional bool, maximumBytes int) error {
	if optional && value == "" {
		return nil
	}
	if err := requireText(name, value); err != nil {
		return err
	}
	if len(value) > maximumBytes || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("%s exceeds its bounded text contract or contains control characters", name)
	}
	return nil
}
