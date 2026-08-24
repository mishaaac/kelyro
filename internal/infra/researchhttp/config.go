package researchhttp

import (
	"fmt"
	"mime"
	"strings"
	"time"
)

const (
	maximumConfiguredResponse = 32 * 1024 * 1024
	maximumUserAgentLength    = 128
	maximumAttempts           = 5
	maximumRedirects          = 10
)

// Config contains bounded HTTP and retry policy. Start with DefaultConfig and
// override only the fields required by a caller.
type Config struct {
	UserAgent                 string
	RequestTimeout            time.Duration
	DialTimeout               time.Duration
	TLSHandshakeTimeout       time.Duration
	ResponseHeaderTimeout     time.Duration
	IdleConnTimeout           time.Duration
	MaxResponseBytes          int64
	MaxResponseHeaderBytes    int64
	MaxRedirects              int
	MaxAttempts               int
	InitialBackoff            time.Duration
	MaxBackoff                time.Duration
	MaxIdleConnections        int
	MaxIdleConnectionsPerHost int
	AllowedContentTypes       []string
}

// DefaultConfig returns conservative defaults for bounded public research.
func DefaultConfig() Config {
	return Config{
		UserAgent:                 "Kelyro/dev",
		RequestTimeout:            20 * time.Second,
		DialTimeout:               5 * time.Second,
		TLSHandshakeTimeout:       5 * time.Second,
		ResponseHeaderTimeout:     10 * time.Second,
		IdleConnTimeout:           60 * time.Second,
		MaxResponseBytes:          4 * 1024 * 1024,
		MaxResponseHeaderBytes:    1 * 1024 * 1024,
		MaxRedirects:              5,
		MaxAttempts:               3,
		InitialBackoff:            100 * time.Millisecond,
		MaxBackoff:                2 * time.Second,
		MaxIdleConnections:        32,
		MaxIdleConnectionsPerHost: 4,
		AllowedContentTypes: []string{
			"text/*",
			"application/json",
			"application/*+json",
			"application/xml",
			"application/*+xml",
			"application/xhtml+xml",
			"application/pdf",
			"application/rss+xml",
			"application/atom+xml",
		},
	}
}

func (config Config) validate() error {
	if len(config.UserAgent) == 0 || len(config.UserAgent) > maximumUserAgentLength ||
		!strings.HasPrefix(config.UserAgent, "Kelyro/") || strings.ContainsAny(config.UserAgent, "\r\n\x00") {
		return fmt.Errorf("user agent must be a bounded Kelyro identifier")
	}
	for _, setting := range []struct {
		name  string
		value time.Duration
	}{
		{"request timeout", config.RequestTimeout},
		{"dial timeout", config.DialTimeout},
		{"TLS handshake timeout", config.TLSHandshakeTimeout},
		{"response header timeout", config.ResponseHeaderTimeout},
		{"idle connection timeout", config.IdleConnTimeout},
	} {
		if setting.value <= 0 || setting.value > 2*time.Minute {
			return fmt.Errorf("%s must be positive and at most two minutes", setting.name)
		}
	}
	if config.MaxResponseBytes <= 0 || config.MaxResponseBytes > maximumConfiguredResponse {
		return fmt.Errorf("maximum response bytes must be between 1 and %d", maximumConfiguredResponse)
	}
	if config.MaxResponseHeaderBytes <= 0 || config.MaxResponseHeaderBytes > 4*1024*1024 {
		return fmt.Errorf("maximum response header bytes must be between 1 and %d", 4*1024*1024)
	}
	if config.MaxRedirects < 0 || config.MaxRedirects > maximumRedirects {
		return fmt.Errorf("maximum redirects must be between 0 and %d", maximumRedirects)
	}
	if config.MaxAttempts < 1 || config.MaxAttempts > maximumAttempts {
		return fmt.Errorf("maximum attempts must be between 1 and %d", maximumAttempts)
	}
	if config.InitialBackoff <= 0 || config.MaxBackoff < config.InitialBackoff || config.MaxBackoff > 30*time.Second {
		return fmt.Errorf("retry backoff must be positive, ordered, and at most 30 seconds")
	}
	if config.MaxIdleConnections < 1 || config.MaxIdleConnections > 256 ||
		config.MaxIdleConnectionsPerHost < 1 || config.MaxIdleConnectionsPerHost > config.MaxIdleConnections {
		return fmt.Errorf("idle connection limits are invalid")
	}
	if len(config.AllowedContentTypes) == 0 {
		return fmt.Errorf("allowed content types are empty")
	}
	seen := make(map[string]struct{}, len(config.AllowedContentTypes))
	for _, pattern := range config.AllowedContentTypes {
		if err := validateMediaPattern(pattern); err != nil {
			return err
		}
		if _, exists := seen[pattern]; exists {
			return fmt.Errorf("duplicate allowed content type %q", pattern)
		}
		seen[pattern] = struct{}{}
	}
	return nil
}

func validateMediaPattern(pattern string) error {
	if pattern == "" || pattern != strings.ToLower(pattern) || pattern != strings.TrimSpace(pattern) {
		return fmt.Errorf("invalid allowed content type %q", pattern)
	}
	major, minor, found := strings.Cut(pattern, "/")
	if !found || major == "" || minor == "" || major == "*" || strings.ContainsAny(pattern, "; \t\r\n") {
		return fmt.Errorf("invalid allowed content type %q", pattern)
	}
	if minor == "*" || strings.HasPrefix(minor, "*+") && len(minor) > 2 {
		return nil
	}
	if _, _, err := mime.ParseMediaType(pattern); err != nil {
		return fmt.Errorf("invalid allowed content type %q", pattern)
	}
	return nil
}
