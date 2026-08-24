package researchnormalize

import (
	"context"
	"regexp"
	"strings"
)

var markdownLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\(([^\s\)]+)(?:\s+["'][^"']*["'])?\)`)

func normalizeMarkdown(ctx context.Context, builder *documentBuilder, document string) error {
	document = strings.ReplaceAll(document, "\r\n", "\n")
	document = strings.ReplaceAll(document, "\r", "\n")
	lines := strings.Split(document, "\n")
	position := consumeFrontMatter(builder, lines)
	var paragraph []string
	var fence string
	var codeLanguage string
	var code strings.Builder
	var suppressedTag string

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		value, err := sanitizeMarkdownProse(ctx, strings.Join(paragraph, " "))
		if err != nil {
			builder.err = err
			return
		}
		for _, match := range markdownLinkPattern.FindAllStringSubmatch(value, -1) {
			builder.addLink(match[1], match[2])
		}
		value = markdownLinkPattern.ReplaceAllString(value, "$1")
		builder.addText(value)
		paragraph = paragraph[:0]
	}

	for ; position < len(lines); position++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := lines[position]
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				builder.addCode(codeLanguage, code.String())
				fence, codeLanguage = "", ""
				code.Reset()
				continue
			}
			if code.Len() > 0 {
				code.WriteByte('\n')
			}
			code.WriteString(line)
			continue
		}
		if marker, language, ok := markdownFence(trimmed); ok {
			flushParagraph()
			if builder.err != nil {
				return builder.err
			}
			fence, codeLanguage = marker, language
			continue
		}
		if suppressedTag != "" {
			if strings.Contains(strings.ToLower(line), "</"+suppressedTag) {
				suppressedTag = ""
			}
			continue
		}
		if tag := markdownSuppressedStart(strings.ToLower(line)); tag != "" {
			if !strings.Contains(strings.ToLower(line), "</"+tag) {
				suppressedTag = tag
			}
			continue
		}
		if level, heading, ok := markdownHeading(trimmed); ok {
			flushParagraph()
			if builder.err != nil {
				return builder.err
			}
			heading, sanitizeErr := sanitizeMarkdownProse(ctx, heading)
			if sanitizeErr != nil {
				return sanitizeErr
			}
			for _, match := range markdownLinkPattern.FindAllStringSubmatch(heading, -1) {
				builder.addLink(match[1], match[2])
			}
			heading = markdownLinkPattern.ReplaceAllString(heading, "$1")
			state := normalizeWhitespace(heading)
			builder.addHeading(level, state)
			continue
		}
		if trimmed == "" {
			flushParagraph()
			if builder.err != nil {
				return builder.err
			}
			continue
		}
		paragraph = append(paragraph, trimmed)
		if builder.err != nil {
			return builder.err
		}
	}
	flushParagraph()
	if fence != "" {
		return invalidDocument("markdown code fence is not closed")
	}
	return builder.err
}

func consumeFrontMatter(builder *documentBuilder, lines []string) int {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return 0
	}
	for index := 1; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if line == "---" {
			return index + 1
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "title":
			builder.setTitle(value)
		case "language", "lang":
			builder.setLanguage(value)
		case "canonical", "canonical_url", "url":
			builder.setCanonical(value)
		case "published", "published_at", "date":
			builder.setPublished(value)
		case "updated", "updated_at", "modified":
			builder.setUpdated(value)
		case "version":
			builder.addVersion(value)
		}
	}
	return 0
}

func markdownFence(line string) (string, string, bool) {
	for _, marker := range []string{"```", "~~~"} {
		if strings.HasPrefix(line, marker) {
			return marker, normalizeCodeLanguage(strings.TrimSpace(strings.TrimPrefix(line, marker))), true
		}
	}
	return "", "", false
}

func markdownHeading(line string) (int, string, bool) {
	level := 0
	for level < len(line) && level < 6 && line[level] == '#' {
		level++
	}
	if level == 0 || level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(strings.TrimRight(strings.TrimSpace(line[level:]), "#")), true
}

func markdownSuppressedStart(line string) string {
	for tag := range htmlDiscardElements {
		if strings.Contains(line, "<"+tag) {
			return tag
		}
	}
	return ""
}

func sanitizeMarkdownProse(ctx context.Context, value string) (string, error) {
	var text strings.Builder
	err := scanHTML(ctx, value, func(token htmlToken) error {
		if token.kind == htmlTextToken {
			text.WriteString(token.data)
			text.WriteByte(' ')
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return normalizeWhitespace(text.String()), nil
}
