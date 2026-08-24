package researchnormalize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

func normalizeJSON(ctx context.Context, builder *documentBuilder, document []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var root any
	if err := decoder.Decode(&root); err != nil {
		return classified(ErrorInvalidDocument, fmt.Errorf("decode JSON: %w", err))
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return invalidDocument("JSON contains trailing values")
	}
	if object, ok := root.(map[string]any); ok {
		consumeJSONMetadata(builder, object)
	}
	if err := walkJSON(ctx, builder, "$", "", root, 0); err != nil {
		return err
	}
	return builder.err
}

func consumeJSONMetadata(builder *documentBuilder, object map[string]any) {
	values := make(map[string]any, len(object))
	for key, value := range object {
		values[strings.ToLower(key)] = value
	}
	if value, ok := jsonString(values, "title", "name"); ok {
		builder.setTitle(value)
	}
	if value, ok := jsonString(values, "language", "lang"); ok {
		builder.setLanguage(value)
	}
	if value, ok := jsonString(values, "canonical_url", "canonical", "url"); ok {
		builder.setCanonical(value)
	}
	if value, ok := jsonString(values, "published_at", "date_published", "published", "date"); ok {
		builder.setPublished(value)
	}
	if value, ok := jsonString(values, "updated_at", "date_modified", "modified", "updated"); ok {
		builder.setUpdated(value)
	}
	if value, ok := jsonScalarString(values, "version", "software_version", "release"); ok {
		builder.addVersion(value)
	}
}

func walkJSON(ctx context.Context, builder *documentBuilder, path, key string, value any, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth > 64 {
		return invalidDocument("JSON nesting exceeds 64 levels")
	}
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for childKey := range typed {
			keys = append(keys, childKey)
		}
		sort.Strings(keys)
		for _, childKey := range keys {
			childPath := path + "/" + escapeJSONPointer(childKey)
			if err := walkJSON(ctx, builder, childPath, childKey, typed[childKey], depth+1); err != nil {
				return err
			}
		}
	case []any:
		for index, child := range typed {
			if err := walkJSON(ctx, builder, path+"/"+strconv.Itoa(index), key, child, depth+1); err != nil {
				return err
			}
		}
	case string:
		lowerKey := strings.ToLower(key)
		if isJSONCodeKey(lowerKey) {
			builder.addCode("", typed)
			return builder.err
		}
		if isJSONLinkKey(lowerKey) {
			builder.addLink(key, typed)
		}
		if strings.Contains(lowerKey, "version") || lowerKey == "release" {
			builder.addVersion(typed)
		}
		builder.addText(path + ": " + typed)
	case json.Number:
		if strings.Contains(strings.ToLower(key), "version") {
			builder.addVersion(typed.String())
		}
		builder.addText(path + ": " + typed.String())
	case bool:
		builder.addText(path + ": " + strconv.FormatBool(typed))
	case nil:
		builder.addText(path + ": null")
	default:
		return invalidDocument("JSON contains an unsupported value")
	}
	return builder.err
}

func jsonString(values map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return value, true
		}
	}
	return "", false
}

func jsonScalarString(values map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		switch value := values[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return value, true
			}
		case json.Number:
			return value.String(), true
		}
	}
	return "", false
}

func isJSONLinkKey(key string) bool {
	return key == "url" || key == "href" || key == "canonical_url" || strings.HasSuffix(key, "_url")
}

func isJSONCodeKey(key string) bool {
	return key == "code" || key == "source" || key == "snippet" || key == "example_code"
}

func escapeJSONPointer(value string) string {
	value = strings.ReplaceAll(value, "~", "~0")
	return strings.ReplaceAll(value, "/", "~1")
}
