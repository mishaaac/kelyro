package researchnormalize

import (
	"context"
	stdhtml "html"
	"strconv"
	"strings"
)

type htmlTokenKind uint8

const (
	htmlTextToken htmlTokenKind = iota
	htmlStartToken
	htmlEndToken
)

type htmlToken struct {
	kind        htmlTokenKind
	data        string
	attributes  map[string]string
	selfClosing bool
}

var htmlDiscardElements = map[string]struct{}{
	"script": {}, "style": {}, "noscript": {}, "template": {},
	"svg": {}, "canvas": {}, "nav": {}, "footer": {}, "form": {}, "aside": {},
}

var htmlBlockElements = map[string]struct{}{
	"address": {}, "article": {}, "blockquote": {}, "dd": {}, "div": {}, "dl": {}, "dt": {},
	"fieldset": {}, "figcaption": {}, "figure": {}, "header": {}, "hr": {}, "li": {}, "main": {},
	"ol": {}, "p": {}, "section": {}, "table": {}, "tbody": {}, "td": {}, "tfoot": {}, "th": {},
	"thead": {}, "tr": {}, "ul": {},
}

type htmlState struct {
	builder      *documentBuilder
	text         strings.Builder
	title        strings.Builder
	inTitle      bool
	heading      strings.Builder
	headingLevel int
	code         strings.Builder
	codeEndTag   string
	codeLanguage string
	anchor       strings.Builder
	anchorHref   string
}

func normalizeHTML(ctx context.Context, builder *documentBuilder, document string) error {
	state := &htmlState{builder: builder}
	err := scanHTML(ctx, document, func(token htmlToken) error { return state.consume(token) })
	if err != nil {
		return err
	}
	state.flushText()
	if state.headingLevel != 0 {
		state.builder.addHeading(state.headingLevel, state.heading.String())
	}
	if state.codeEndTag != "" {
		state.builder.addCode(state.codeLanguage, state.code.String())
	}
	return state.builder.err
}

func (state *htmlState) consume(token htmlToken) error {
	switch token.kind {
	case htmlTextToken:
		value := stdhtml.UnescapeString(token.data)
		switch {
		case state.inTitle:
			state.title.WriteString(value)
		case state.headingLevel != 0:
			state.heading.WriteString(value)
		case state.codeEndTag != "":
			state.code.WriteString(value)
		default:
			state.text.WriteString(value)
			state.text.WriteByte(' ')
		}
		if state.anchorHref != "" {
			state.anchor.WriteString(value)
		}
	case htmlStartToken:
		state.start(token)
	case htmlEndToken:
		state.end(token.data)
	}
	return state.builder.err
}

func (state *htmlState) start(token htmlToken) {
	tag := token.data
	if _, block := htmlBlockElements[tag]; block {
		state.flushText()
	}
	if level := headingLevel(tag); level != 0 {
		state.flushText()
		state.heading.Reset()
		state.headingLevel = level
	}
	switch tag {
	case "html":
		state.builder.setLanguage(token.attributes["lang"])
	case "title":
		state.inTitle = true
		state.title.Reset()
	case "meta":
		state.consumeMeta(token.attributes)
	case "link":
		if containsToken(token.attributes["rel"], "canonical") {
			state.builder.setCanonical(token.attributes["href"])
		}
	case "br":
		state.flushText()
	case "pre":
		state.flushText()
		if state.codeEndTag == "" {
			state.codeEndTag = "pre"
			state.codeLanguage = codeLanguageFromAttributes(token.attributes)
			state.code.Reset()
		}
	case "code":
		if state.codeEndTag == "" {
			state.flushText()
			state.codeEndTag = "code"
			state.code.Reset()
		}
		if language := codeLanguageFromAttributes(token.attributes); language != "" {
			state.codeLanguage = language
		}
	case "a":
		state.anchorHref = token.attributes["href"]
		state.anchor.Reset()
	case "time":
		state.consumeTime(token.attributes)
	}
	if token.selfClosing {
		state.end(tag)
	}
}

func (state *htmlState) end(tag string) {
	if tag == "title" && state.inTitle {
		state.inTitle = false
		state.builder.setTitle(state.title.String())
	}
	if level := headingLevel(tag); level != 0 && state.headingLevel == level {
		state.builder.addHeading(level, state.heading.String())
		state.heading.Reset()
		state.headingLevel = 0
	}
	if tag == state.codeEndTag {
		state.builder.addCode(state.codeLanguage, state.code.String())
		state.code.Reset()
		state.codeEndTag = ""
		state.codeLanguage = ""
	}
	if tag == "a" && state.anchorHref != "" {
		state.builder.addLink(state.anchor.String(), state.anchorHref)
		state.anchorHref = ""
		state.anchor.Reset()
	}
	if _, block := htmlBlockElements[tag]; block {
		state.flushText()
	}
}

func (state *htmlState) flushText() {
	state.builder.addText(state.text.String())
	state.text.Reset()
}

func (state *htmlState) consumeMeta(attributes map[string]string) {
	key := strings.ToLower(firstNonEmptyValue(attributes["name"], attributes["property"], attributes["itemprop"], attributes["http-equiv"]))
	value := attributes["content"]
	switch key {
	case "title", "og:title", "twitter:title":
		state.builder.setTitle(value)
	case "language", "content-language", "og:locale":
		state.builder.setLanguage(value)
	case "url", "og:url", "canonical", "canonicalurl":
		state.builder.setCanonical(value)
	case "date", "datepublished", "article:published_time", "published", "publication_date":
		state.builder.setPublished(value)
	case "datemodified", "article:modified_time", "updated", "last-modified":
		state.builder.setUpdated(value)
	case "version", "softwareversion", "release", "release-version":
		state.builder.addVersion(value)
	}
}

func (state *htmlState) consumeTime(attributes map[string]string) {
	value := attributes["datetime"]
	key := strings.ToLower(firstNonEmptyValue(attributes["itemprop"], attributes["property"]))
	switch key {
	case "datepublished", "article:published_time":
		state.builder.setPublished(value)
	case "datemodified", "article:modified_time":
		state.builder.setUpdated(value)
	}
}

func scanHTML(ctx context.Context, document string, emit func(htmlToken) error) error {
	lower := strings.ToLower(document)
	for position := 0; position < len(document); {
		if err := ctx.Err(); err != nil {
			return err
		}
		open := strings.IndexByte(document[position:], '<')
		if open < 0 {
			return emit(htmlToken{kind: htmlTextToken, data: document[position:]})
		}
		open += position
		if open > position {
			if err := emit(htmlToken{kind: htmlTextToken, data: document[position:open]}); err != nil {
				return err
			}
		}
		if strings.HasPrefix(document[open:], "<!--") {
			end := strings.Index(document[open+4:], "-->")
			if end < 0 {
				return nil
			}
			position = open + 4 + end + 3
			continue
		}
		end := findHTMLTagEnd(document, open+1)
		if end < 0 {
			return emit(htmlToken{kind: htmlTextToken, data: document[open:]})
		}
		token, ok := parseHTMLTag(document[open+1 : end])
		position = end + 1
		if !ok {
			raw := strings.TrimSpace(document[open+1 : end])
			if !strings.HasPrefix(raw, "!") && !strings.HasPrefix(raw, "?") {
				if err := emit(htmlToken{kind: htmlTextToken, data: document[open : end+1]}); err != nil {
					return err
				}
			}
			continue
		}
		if token.kind == htmlStartToken {
			if _, discard := htmlDiscardElements[token.data]; discard && !token.selfClosing {
				closeStart := findHTMLClosingElement(lower, position, token.data)
				if closeStart < 0 {
					return nil
				}
				closeEnd := findHTMLTagEnd(document, closeStart+2)
				if closeEnd < 0 {
					return nil
				}
				position = closeEnd + 1
				continue
			}
		}
		if err := emit(token); err != nil {
			return err
		}
	}
	return nil
}

func findHTMLTagEnd(document string, start int) int {
	var quote byte
	for index := start; index < len(document); index++ {
		current := document[index]
		if quote != 0 {
			if current == quote {
				quote = 0
			}
			continue
		}
		if current == '\'' || current == '"' {
			quote = current
			continue
		}
		if current == '>' {
			return index
		}
	}
	return -1
}

func findHTMLClosingElement(document string, start int, tag string) int {
	wanted := "</" + tag
	for start < len(document) {
		index := strings.Index(document[start:], wanted)
		if index < 0 {
			return -1
		}
		index += start
		after := index + len(wanted)
		if after == len(document) || document[after] == '>' || document[after] == '/' || isHTMLSpace(document[after]) {
			return index
		}
		start = after
	}
	return -1
}

func parseHTMLTag(raw string) (htmlToken, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "!") || strings.HasPrefix(raw, "?") {
		return htmlToken{}, false
	}
	if strings.HasPrefix(raw, "/") {
		name := readHTMLName(strings.TrimSpace(strings.TrimPrefix(raw, "/")))
		return htmlToken{kind: htmlEndToken, data: name}, name != ""
	}
	selfClosing := strings.HasSuffix(raw, "/")
	if selfClosing {
		raw = strings.TrimSpace(strings.TrimSuffix(raw, "/"))
	}
	name := readHTMLName(raw)
	if name == "" {
		return htmlToken{}, false
	}
	return htmlToken{
		kind: htmlStartToken, data: name, selfClosing: selfClosing,
		attributes: parseHTMLAttributes(raw[len(name):]),
	}, true
}

func readHTMLName(value string) string {
	if value == "" || !isASCIIAlpha(value[0]) {
		return ""
	}
	end := 0
	for end < len(value) {
		current := value[end]
		if !(isASCIIAlpha(current) || (current >= '0' && current <= '9') || current == ':' || current == '-') {
			break
		}
		end++
	}
	return strings.ToLower(value[:end])
}

func isASCIIAlpha(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func parseHTMLAttributes(raw string) map[string]string {
	attributes := make(map[string]string)
	for position := 0; position < len(raw); {
		for position < len(raw) && isHTMLSpace(raw[position]) {
			position++
		}
		start := position
		for position < len(raw) && !isHTMLSpace(raw[position]) && raw[position] != '=' {
			position++
		}
		if start == position {
			position++
			continue
		}
		key := strings.ToLower(raw[start:position])
		for position < len(raw) && isHTMLSpace(raw[position]) {
			position++
		}
		value := ""
		if position < len(raw) && raw[position] == '=' {
			position++
			for position < len(raw) && isHTMLSpace(raw[position]) {
				position++
			}
			if position < len(raw) && (raw[position] == '\'' || raw[position] == '"') {
				quote := raw[position]
				position++
				start = position
				for position < len(raw) && raw[position] != quote {
					position++
				}
				value = raw[start:position]
				if position < len(raw) {
					position++
				}
			} else {
				start = position
				for position < len(raw) && !isHTMLSpace(raw[position]) {
					position++
				}
				value = raw[start:position]
			}
		}
		if _, exists := attributes[key]; !exists {
			attributes[key] = stdhtml.UnescapeString(value)
		}
	}
	return attributes
}

func isHTMLSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n' || value == '\f'
}

func headingLevel(tag string) int {
	if len(tag) == 2 && tag[0] == 'h' {
		level, err := strconv.Atoi(tag[1:])
		if err == nil && level >= 1 && level <= 6 {
			return level
		}
	}
	return 0
}

func containsToken(value, wanted string) bool {
	for _, token := range strings.Fields(strings.ToLower(value)) {
		if token == wanted {
			return true
		}
	}
	return false
}

func codeLanguageFromAttributes(attributes map[string]string) string {
	if value := firstNonEmptyValue(attributes["data-language"], attributes["lang"]); value != "" {
		return normalizeCodeLanguage(value)
	}
	for _, class := range strings.Fields(attributes["class"]) {
		if strings.HasPrefix(class, "language-") || strings.HasPrefix(class, "lang-") {
			return normalizeCodeLanguage(class)
		}
	}
	return ""
}

func firstNonEmptyValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
