package researchsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

const (
	ProviderID              = "brave"
	EndpointURL             = "https://api.search.brave.com/res/v1/web/search"
	AdapterVersion          = "brave-web-search-v1"
	maximumResultsPerPage   = 20
	maximumPageOffset       = 9
	maximumQueryRunes       = 400
	maximumQueryWords       = 50
	maximumTokenBytes       = 4 * 1024
	maximumResponseBytes    = 1 * 1024 * 1024
	maximumRateHeaderBytes  = 256
	credentialHeader        = "X-Subscription-Token"
	responseDrainLimitBytes = 4 * 1024
)

// HTTPClient is the minimal transport boundary used by the adapter. Step 7
// supplies the hardened production implementation; unit tests inject fixtures.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// RateLimitMetadata mirrors Brave's documented quota headers as bounded,
// opaque metadata. It never influences candidate trust or authority.
type RateLimitMetadata struct {
	Limit     string
	Policy    string
	Remaining string
	Reset     string
}

// PageObservation provides enough non-secret metadata for later audit and cost
// accounting without exposing query text, result URLs, bodies, or credentials.
type PageObservation struct {
	StatusCode     int
	Offset         int
	RequestedCount int
	RateLimit      RateLimitMetadata
}

type PageObserver interface {
	ObserveSearchPage(context.Context, PageObservation)
}

type Option func(*Brave)

func WithPageObserver(observer PageObserver) Option {
	return func(provider *Brave) { provider.observer = observer }
}

// Brave implements the provider-neutral SearchProvider contract with Brave's
// documented Web Search REST/JSON endpoint. It performs discovery only.
type Brave struct {
	client   HTTPClient
	token    string
	endpoint string
	observer PageObserver
}

// String and GoString keep diagnostic formatting from reflecting the
// in-memory credential held by the adapter.
func (provider *Brave) String() string {
	if provider == nil {
		return "<nil brave search provider>"
	}
	return "brave search provider"
}

func (provider *Brave) GoString() string { return provider.String() }

// NewBrave constructs the adapter without resolving credentials or enabling
// network access. Those application boundaries are wired in later steps.
func NewBrave(client HTTPClient, token string, options ...Option) (*Brave, error) {
	if client == nil {
		return nil, errors.New("brave search HTTP client is unavailable")
	}
	if err := validateToken(token); err != nil {
		return nil, err
	}
	provider := &Brave{client: client, token: token, endpoint: EndpointURL}
	for _, option := range options {
		if option != nil {
			option(provider)
		}
	}
	return provider, nil
}

func (provider *Brave) Search(ctx context.Context, query application.SearchQuery, options application.SearchOptions) ([]application.SearchResult, error) {
	return provider.search(ctx, query, options, nil)
}

func (provider *Brave) SearchWithCostControl(
	ctx context.Context,
	query application.SearchQuery,
	options application.SearchOptions,
	authorize application.ProviderCallAuthorizer,
) ([]application.SearchResult, error) {
	if authorize == nil {
		return nil, errors.New("brave search provider call authorizer is unavailable")
	}
	return provider.search(ctx, query, options, authorize)
}

func (provider *Brave) search(
	ctx context.Context,
	query application.SearchQuery,
	options application.SearchOptions,
	authorize application.ProviderCallAuthorizer,
) ([]application.SearchResult, error) {
	if provider == nil || provider.client == nil || provider.endpoint == "" {
		return nil, errors.New("brave search provider is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("validate brave search query: %w", err)
	}
	if err := options.Validate(); err != nil {
		return nil, fmt.Errorf("validate brave search options: %w", err)
	}
	if utf8.RuneCountInString(query.Text) > maximumQueryRunes || len(strings.Fields(query.Text)) > maximumQueryWords {
		return nil, &Error{Kind: ErrorInvalidRequest}
	}

	results := make([]application.SearchResult, 0, options.Limit)
	pageSize := min(maximumResultsPerPage, options.Limit)
	for offset := 0; offset <= maximumPageOffset && len(results) < options.Limit; offset++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page, more, err := provider.searchPage(ctx, query.Text, offset, pageSize, offset*pageSize, authorize)
		if err != nil {
			return nil, err
		}
		if remaining := options.Limit - len(results); len(page) > remaining {
			page = page[:remaining]
		}
		results = append(results, page...)
		if !more || len(page) == 0 {
			break
		}
	}
	return results, nil
}

var _ application.CostControlledSearchProvider = (*Brave)(nil)

func (provider *Brave) searchPage(
	ctx context.Context,
	query string,
	offset, count, rankBase int,
	authorize application.ProviderCallAuthorizer,
) ([]application.SearchResult, bool, error) {
	requestURL, err := url.Parse(provider.endpoint)
	if err != nil {
		return nil, false, &Error{Kind: ErrorInvalidRequest}
	}
	parameters := requestURL.Query()
	parameters.Set("q", query)
	parameters.Set("count", strconv.Itoa(count))
	parameters.Set("offset", strconv.Itoa(offset))
	requestURL.RawQuery = parameters.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, false, &Error{Kind: ErrorInvalidRequest}
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set(credentialHeader, provider.token)
	if authorize != nil {
		if err := authorize(ctx); err != nil {
			return nil, false, err
		}
	}

	response, err := provider.client.Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, false, ctxErr
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, false, err
		}
		if _, classified := KindOf(err); classified {
			return nil, false, err
		}
		return nil, false, &Error{Kind: ErrorTransport}
	}
	if response == nil || response.Body == nil {
		return nil, false, &Error{Kind: ErrorTransport}
	}
	defer response.Body.Close()

	rateLimit := parseRateLimit(response.Header)
	provider.observe(ctx, PageObservation{
		StatusCode: response.StatusCode, Offset: offset, RequestedCount: count, RateLimit: rateLimit,
	})
	if response.StatusCode != http.StatusOK {
		drainResponse(response.Body)
		return nil, false, statusError(response.StatusCode, rateLimit)
	}
	if response.ContentLength > maximumResponseBytes {
		return nil, false, &Error{Kind: ErrorResponse, StatusCode: response.StatusCode}
	}
	if encoding := strings.TrimSpace(response.Header.Get("Content-Encoding")); encoding != "" && !strings.EqualFold(encoding, "identity") {
		return nil, false, &Error{Kind: ErrorResponse, StatusCode: response.StatusCode}
	}
	mediaType, _, mediaErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaErr != nil || !strings.EqualFold(mediaType, "application/json") {
		return nil, false, &Error{Kind: ErrorResponse, StatusCode: response.StatusCode}
	}

	body, err := readBounded(ctx, response.Body, maximumResponseBytes)
	if err != nil {
		return nil, false, err
	}
	var payload braveResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&payload); err != nil {
		return nil, false, &Error{Kind: ErrorResponse, StatusCode: response.StatusCode}
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, false, &Error{Kind: ErrorResponse, StatusCode: response.StatusCode}
	}
	if len(payload.Web.Results) > count || len(payload.Web.Results) > maximumResultsPerPage {
		return nil, false, &Error{Kind: ErrorResponse, StatusCode: response.StatusCode}
	}

	results := make([]application.SearchResult, 0, len(payload.Web.Results))
	for index, item := range payload.Web.Results {
		result, err := normalizeResult(item, rankBase+index)
		if err != nil {
			return nil, false, &Error{Kind: ErrorResponse, StatusCode: response.StatusCode}
		}
		results = append(results, result)
	}
	return results, payload.Query.MoreResultsAvailable, nil
}

func validateToken(token string) error {
	if token == "" || len(token) > maximumTokenBytes || strings.IndexFunc(token, func(character rune) bool {
		return unicode.IsControl(character) || unicode.IsSpace(character)
	}) >= 0 {
		return errors.New("brave search credential is missing or invalid")
	}
	return nil
}

func normalizeResult(item braveResult, rank int) (application.SearchResult, error) {
	parsed, err := url.Parse(strings.TrimSpace(item.URL))
	if err != nil {
		return application.SearchResult{}, err
	}
	parsed.Fragment = ""
	parsed.RawFragment = ""
	locator, err := research.NewSourceLocator(parsed.String())
	if err != nil {
		return application.SearchResult{}, err
	}
	result := application.SearchResult{
		Title: strings.Join(strings.Fields(item.Title), " "), Locator: locator,
		Snippet: strings.Join(strings.Fields(item.Description), " "), Provider: ProviderID, Rank: rank,
	}
	if published, ok := parsePublishedHint(item.Age); ok {
		result.PublishedHint = &published
	}
	if err := result.Validate(); err != nil {
		return application.SearchResult{}, err
	}
	return result, nil
}

func parsePublishedHint(value string) (research.Timestamp, bool) {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return research.Timestamp{}, false
	}
	timestamp, err := research.NewTimestamp(parsed)
	return timestamp, err == nil
}

func parseRateLimit(header http.Header) RateLimitMetadata {
	return RateLimitMetadata{
		Limit:     boundedHeader(header.Get("X-RateLimit-Limit")),
		Policy:    boundedHeader(header.Get("X-RateLimit-Policy")),
		Remaining: boundedHeader(header.Get("X-RateLimit-Remaining")),
		Reset:     boundedHeader(header.Get("X-RateLimit-Reset")),
	}
}

func boundedHeader(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > maximumRateHeaderBytes || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return ""
	}
	return value
}

func statusError(status int, rateLimit RateLimitMetadata) error {
	kind := ErrorResponse
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		kind = ErrorAuthentication
	case status == http.StatusTooManyRequests:
		kind = ErrorRateLimited
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		kind = ErrorInvalidRequest
	case status == http.StatusRequestTimeout || status >= http.StatusInternalServerError:
		kind = ErrorUnavailable
	}
	return &Error{Kind: kind, StatusCode: status, RateLimit: rateLimit}
}

func readBounded(ctx context.Context, reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &Error{Kind: ErrorUnavailable}
		}
		return nil, &Error{Kind: ErrorTransport}
	}
	if int64(len(body)) > limit {
		return nil, &Error{Kind: ErrorResponse, StatusCode: http.StatusOK}
	}
	return body, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("response contains trailing JSON")
	}
	return nil
}

func drainResponse(body io.Reader) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, responseDrainLimitBytes))
}

func (provider *Brave) observe(ctx context.Context, event PageObservation) {
	if provider.observer != nil {
		provider.observer.ObserveSearchPage(ctx, event)
	}
}

type braveResponse struct {
	Query struct {
		MoreResultsAvailable bool `json:"more_results_available"`
	} `json:"query"`
	Web struct {
		Results []braveResult `json:"results"`
	} `json:"web"`
}

type braveResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Age         string `json:"age"`
}

var _ application.SearchProvider = (*Brave)(nil)
