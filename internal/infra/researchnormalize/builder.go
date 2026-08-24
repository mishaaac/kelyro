package researchnormalize

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

var versionPattern = regexp.MustCompile(`(?i)\b(?:version\s+|v)([0-9]+(?:\.[0-9]+){1,3}(?:[-+][0-9a-z.-]+)?)\b`)

type documentBuilder struct {
	result       application.NormalizedSource
	headingPath  [6]string
	seenLinks    map[string]struct{}
	seenVersions map[string]struct{}
	err          error
}

func newDocumentBuilder(fetched application.FetchedSource) *documentBuilder {
	canonical := fetched.Locator
	return &documentBuilder{
		result: application.NormalizedSource{
			SourceID: fetched.SourceID, Locator: fetched.Locator, ContentType: fetched.Metadata.ContentType,
			CanonicalLocator: &canonical, NormalizationVersion: Version,
		},
		seenLinks: make(map[string]struct{}), seenVersions: make(map[string]struct{}),
	}
}

func (builder *documentBuilder) setTitle(value string) {
	value = normalizeWhitespace(value)
	if builder.result.Title == "" && value != "" {
		builder.result.Title = truncateUTF8(value, application.MaximumNormalizedTextBytes)
		builder.extractVersions(value)
	}
}

func (builder *documentBuilder) setLanguage(value string) {
	if builder.result.Language != "" {
		return
	}
	value = strings.ToLower(strings.ReplaceAll(normalizeWhitespace(value), "_", "-"))
	if value != "" && len(value) <= 64 && !strings.ContainsAny(value, "<>/\\") {
		builder.result.Language = value
	}
}

func (builder *documentBuilder) setCanonical(raw string) {
	locator, ok := resolveLocator(builder.result.Locator, raw, true)
	if ok {
		builder.result.CanonicalLocator = &locator
	}
}

func (builder *documentBuilder) addHeading(level int, value string) {
	if builder.err != nil {
		return
	}
	value = normalizeWhitespace(value)
	if value == "" {
		return
	}
	if len(builder.result.Headings) >= application.MaximumNormalizedHeadings {
		builder.err = classified(ErrorOutputLimit, fmt.Errorf("heading count exceeds limit"))
		return
	}
	value = truncateUTF8(value, application.MaximumNormalizedTextBytes)
	builder.headingPath[level-1] = value
	for index := level; index < len(builder.headingPath); index++ {
		builder.headingPath[index] = ""
	}
	path := make([]string, 0, level)
	for index := 0; index < level; index++ {
		if builder.headingPath[index] != "" {
			path = append(path, builder.headingPath[index])
		}
	}
	builder.result.Headings = append(builder.result.Headings, application.NormalizedHeading{Level: level, Text: value, Path: path})
	builder.extractVersions(value)
	if builder.result.Title == "" && level == 1 {
		builder.setTitle(value)
	}
}

func (builder *documentBuilder) addText(value string) {
	if builder.err != nil {
		return
	}
	value = normalizeWhitespace(value)
	if value == "" {
		return
	}
	for _, segment := range splitBoundedText(value, application.MaximumNormalizedTextBytes) {
		if len(builder.result.TextSegments) >= application.MaximumNormalizedSegments {
			builder.err = classified(ErrorOutputLimit, fmt.Errorf("text segment count exceeds limit"))
			return
		}
		builder.result.TextSegments = append(builder.result.TextSegments, segment)
		builder.extractVersions(segment)
	}
}

func (builder *documentBuilder) addCode(language, content string) {
	if builder.err != nil {
		return
	}
	content = normalizeCode(content)
	if strings.TrimSpace(content) == "" {
		return
	}
	if len(builder.result.CodeBlocks) >= application.MaximumNormalizedCodeBlocks || len(content) > application.MaximumNormalizedCodeBytes {
		builder.err = classified(ErrorOutputLimit, fmt.Errorf("code block exceeds limit"))
		return
	}
	language = normalizeCodeLanguage(language)
	builder.result.CodeBlocks = append(builder.result.CodeBlocks, application.NormalizedCodeBlock{Language: language, Content: content})
}

func (builder *documentBuilder) addLink(label, raw string) {
	if builder.err != nil {
		return
	}
	locator, ok := resolveLocator(builder.result.Locator, raw, false)
	if !ok {
		return
	}
	key := locator.String()
	if _, exists := builder.seenLinks[key]; exists {
		return
	}
	if len(builder.result.Links) >= application.MaximumNormalizedLinks {
		builder.err = classified(ErrorOutputLimit, fmt.Errorf("link count exceeds limit"))
		return
	}
	builder.seenLinks[key] = struct{}{}
	builder.result.Links = append(builder.result.Links, application.NormalizedLink{
		Label: truncateUTF8(normalizeWhitespace(label), application.MaximumNormalizedTextBytes), Locator: locator,
	})
}

func (builder *documentBuilder) addVersion(value string) {
	value = normalizeWhitespace(value)
	if value == "" {
		return
	}
	value = truncateUTF8(value, 128)
	key := strings.ToLower(value)
	if _, exists := builder.seenVersions[key]; exists {
		return
	}
	if len(builder.result.VersionHints) >= application.MaximumNormalizedVersionHints {
		builder.err = classified(ErrorOutputLimit, fmt.Errorf("version hint count exceeds limit"))
		return
	}
	builder.seenVersions[key] = struct{}{}
	builder.result.VersionHints = append(builder.result.VersionHints, value)
}

func (builder *documentBuilder) extractVersions(value string) {
	for _, match := range versionPattern.FindAllStringSubmatch(value, -1) {
		builder.addVersion(match[1])
	}
}

func (builder *documentBuilder) finish() (application.NormalizedSource, error) {
	if builder.err != nil {
		return application.NormalizedSource{}, builder.err
	}
	if builder.result.Title == "" && len(builder.result.Headings) > 0 {
		builder.result.Title = builder.result.Headings[0].Text
	}
	return builder.result, nil
}

func resolveLocator(base research.SourceLocator, raw string, canonical bool) (research.SourceLocator, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return research.SourceLocator{}, false
	}
	reference, err := url.Parse(raw)
	if err != nil {
		return research.SourceLocator{}, false
	}
	baseURL, err := url.Parse(base.String())
	if err != nil {
		return research.SourceLocator{}, false
	}
	resolved := baseURL.ResolveReference(reference)
	if canonical {
		resolved.Fragment = ""
	}
	locator, err := research.NewSourceLocator(resolved.String())
	return locator, err == nil
}

func normalizeWhitespace(value string) string { return strings.Join(strings.Fields(value), " ") }

func normalizeCode(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.Trim(value, "\n")
}

func normalizeCodeLanguage(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "language-")
	value = strings.TrimPrefix(value, "lang-")
	if len(value) > 64 || strings.ContainsAny(value, " \t\r\n<>") {
		return ""
	}
	return value
}

func truncateUTF8(value string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	value = value[:maximum]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return strings.TrimSpace(value)
}

func splitBoundedText(value string, maximum int) []string {
	if len(value) <= maximum {
		return []string{value}
	}
	result := make([]string, 0, len(value)/maximum+1)
	remaining := value
	for len(remaining) > maximum {
		cut := maximum
		for cut > 0 && !utf8.RuneStart(remaining[cut]) {
			cut--
		}
		if space := strings.LastIndexByte(remaining[:cut], ' '); space > maximum/2 {
			cut = space
		}
		result = append(result, strings.TrimSpace(remaining[:cut]))
		remaining = strings.TrimSpace(remaining[cut:])
	}
	if remaining != "" {
		result = append(result, remaining)
	}
	return result
}
