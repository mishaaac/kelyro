package researchnormalize

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

const Version = "source-normalization-v1"

type Normalizer struct{}

func New() *Normalizer { return &Normalizer{} }

func (normalizer *Normalizer) Normalize(ctx context.Context, fetched application.FetchedSource) (application.NormalizedSource, error) {
	if err := ctx.Err(); err != nil {
		return application.NormalizedSource{}, err
	}
	if normalizer == nil {
		return application.NormalizedSource{}, invalidDocument("normalizer is unavailable")
	}
	if err := fetched.Validate(); err != nil {
		return application.NormalizedSource{}, classified(ErrorInvalidDocument, fmt.Errorf("validate fetched source: %w", err))
	}
	if fetched.Metadata.StatusCode < http.StatusOK || fetched.Metadata.StatusCode >= http.StatusMultipleChoices ||
		fetched.Metadata.StatusCode == http.StatusNoContent {
		return application.NormalizedSource{}, invalidDocument("response has no normalizable representation")
	}
	if int64(len(fetched.Body)) != fetched.Metadata.ContentLength {
		return application.NormalizedSource{}, invalidDocument("content length does not match body")
	}
	if fetched.Metadata.ContentHash != research.CanonicalContentHashV1(fetched.Body) {
		return application.NormalizedSource{}, invalidDocument("content hash is not canonical")
	}
	if !utf8.Valid(fetched.Body) {
		return application.NormalizedSource{}, invalidDocument("document is not valid UTF-8")
	}

	builder := newDocumentBuilder(fetched)
	var err error
	switch contentType := strings.ToLower(fetched.Metadata.ContentType); {
	case contentType == "text/html" || contentType == "application/xhtml+xml":
		err = normalizeHTML(ctx, builder, string(fetched.Body))
	case contentType == "text/plain":
		err = normalizePlainText(ctx, builder, string(fetched.Body))
	case contentType == "application/json" || strings.HasSuffix(contentType, "+json"):
		err = normalizeJSON(ctx, builder, fetched.Body)
	case contentType == "text/markdown" || contentType == "text/x-markdown":
		err = normalizeMarkdown(ctx, builder, string(fetched.Body))
	default:
		return application.NormalizedSource{}, classified(ErrorUnsupportedContentType, fmt.Errorf("unsupported media type"))
	}
	if err != nil {
		return application.NormalizedSource{}, err
	}
	result, err := builder.finish()
	if err != nil {
		return application.NormalizedSource{}, err
	}
	if err := result.Validate(); err != nil {
		return application.NormalizedSource{}, classified(ErrorInvalidDocument, fmt.Errorf("validate normalized source: %w", err))
	}
	return result, nil
}

var _ application.SourceNormalizer = (*Normalizer)(nil)
