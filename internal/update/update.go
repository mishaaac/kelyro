// Package update defines version-aware update checks without coupling the core
// to GitHub, HTTP, a cache format, or an installation mechanism.
package update

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/privacy"
	buildversion "github.com/mishaaac/kelyro/internal/version"
)

const DefaultCacheTTL = 24 * time.Hour

var (
	ErrMalformedVersion    = errors.New("malformed semantic version")
	ErrProviderUnavailable = errors.New("release provider unavailable")
	semanticVersionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
)

// Channel controls whether prerelease versions may be selected.
type Channel string

const (
	Stable     Channel = "stable"
	Prerelease Channel = "prerelease"
)

func (channel Channel) Valid() bool { return channel == Stable || channel == Prerelease }

// Version is a parsed SemVer 2.0 version. Build metadata is retained for
// display but ignored for precedence.
type Version struct {
	major      uint64
	minor      uint64
	patch      uint64
	prerelease []string
	build      string
}

// ParseVersion parses SemVer 2.0 with an optional conventional leading v.
func ParseVersion(value string) (Version, error) {
	matches := semanticVersionPattern.FindStringSubmatch(value)
	if matches == nil {
		return Version{}, fmt.Errorf("%w %q", ErrMalformedVersion, value)
	}
	major, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return Version{}, fmt.Errorf("%w %q", ErrMalformedVersion, value)
	}
	minor, err := strconv.ParseUint(matches[2], 10, 64)
	if err != nil {
		return Version{}, fmt.Errorf("%w %q", ErrMalformedVersion, value)
	}
	patch, err := strconv.ParseUint(matches[3], 10, 64)
	if err != nil {
		return Version{}, fmt.Errorf("%w %q", ErrMalformedVersion, value)
	}

	var prerelease []string
	if matches[4] != "" {
		prerelease = strings.Split(matches[4], ".")
		for _, identifier := range prerelease {
			if numeric(identifier) && len(identifier) > 1 && identifier[0] == '0' {
				return Version{}, fmt.Errorf("%w %q", ErrMalformedVersion, value)
			}
		}
	}
	return Version{major: major, minor: minor, patch: patch, prerelease: prerelease, build: matches[5]}, nil
}

func (version Version) String() string {
	result := fmt.Sprintf("%d.%d.%d", version.major, version.minor, version.patch)
	if len(version.prerelease) > 0 {
		result += "-" + strings.Join(version.prerelease, ".")
	}
	if version.build != "" {
		result += "+" + version.build
	}
	return result
}

func (version Version) IsPrerelease() bool { return len(version.prerelease) > 0 }

// Compare returns -1, 0, or 1 according to SemVer precedence.
func (version Version) Compare(other Version) int {
	for _, pair := range [][2]uint64{{version.major, other.major}, {version.minor, other.minor}, {version.patch, other.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if len(version.prerelease) == 0 && len(other.prerelease) == 0 {
		return 0
	}
	if len(version.prerelease) == 0 {
		return 1
	}
	if len(other.prerelease) == 0 {
		return -1
	}
	for index := 0; index < len(version.prerelease) && index < len(other.prerelease); index++ {
		left, right := version.prerelease[index], other.prerelease[index]
		if left == right {
			continue
		}
		leftNumeric, rightNumeric := numeric(left), numeric(right)
		switch {
		case leftNumeric && rightNumeric:
			return compareNumericIdentifier(left, right)
		case leftNumeric:
			return -1
		case rightNumeric:
			return 1
		case left < right:
			return -1
		default:
			return 1
		}
	}
	if len(version.prerelease) < len(other.prerelease) {
		return -1
	}
	if len(version.prerelease) > len(other.prerelease) {
		return 1
	}
	return 0
}

func numeric(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func compareNumericIdentifier(left, right string) int {
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

// Release is provider-neutral published release metadata. URL is shown as
// text only; Foundation never opens or downloads it automatically.
type Release struct {
	Version     string    `json:"version"`
	URL         string    `json:"url,omitempty"`
	PublishedAt time.Time `json:"published_at,omitempty"`
}

// ReleaseProvider retrieves the newest release allowed by a channel.
type ReleaseProvider interface {
	Latest(ctx context.Context, channel Channel) (release Release, found bool, err error)
}

// CachedCheck stores one provider result, including an empty release list.
type CachedCheck struct {
	Channel   Channel   `json:"channel"`
	CheckedAt time.Time `json:"checked_at"`
	Found     bool      `json:"found"`
	Release   Release   `json:"release,omitempty"`
}

// Cache persists disposable update metadata independently of configuration.
type Cache interface {
	Load(ctx context.Context, channel Channel) (CachedCheck, bool, error)
	Save(ctx context.Context, check CachedCheck) error
}

type Status string

const (
	UpdateAvailable Status = "update-available"
	UpToDate        Status = "up-to-date"
	Unavailable     Status = "unavailable"
)

type Source string

const (
	SourceNetwork Source = "network"
	SourceCache   Source = "cache"
	SourceNone    Source = "none"
)

// Result is presentation-neutral and never initiates installation.
type Result struct {
	Status         Status
	Source         Source
	Channel        Channel
	CurrentVersion string
	LatestVersion  string
	ReleaseURL     string
	CheckedAt      time.Time
	Detail         string
}

// Checker is the application-facing update-check contract.
type Checker interface {
	Check(ctx context.Context, channel Channel, gate privacy.NetworkGate) (Result, error)
}

type Service struct {
	current  string
	provider ReleaseProvider
	cache    Cache
	ttl      time.Duration
	now      func() time.Time
}

// New creates a version checker. It checks only when explicitly called and
// never downloads or installs an artifact.
func New(current string, provider ReleaseProvider, cache Cache) *Service {
	return &Service{current: current, provider: provider, cache: cache, ttl: DefaultCacheTTL, now: time.Now}
}

// WithCacheTTL changes cache freshness, primarily for controlled callers.
func (service *Service) WithCacheTTL(ttl time.Duration) *Service {
	service.ttl = ttl
	return service
}

func (service *Service) withClock(now func() time.Time) *Service {
	service.now = now
	return service
}

func (service *Service) Check(ctx context.Context, channel Channel, gate privacy.NetworkGate) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if !channel.Valid() {
		return Result{}, fmt.Errorf("invalid update channel %q", channel)
	}
	if buildversion.IsDevelopment(service.current) {
		return Result{
			Status: Unavailable, Source: SourceNone, Channel: channel,
			CurrentVersion: service.current, Detail: "development build",
		}, nil
	}
	current, err := ParseVersion(service.current)
	if err != nil {
		return Result{}, fmt.Errorf("current version: %w", err)
	}
	if service.now == nil {
		return Result{}, errors.New("update clock is unavailable")
	}
	now := service.now().UTC()

	if service.cache != nil {
		cached, found, loadErr := service.cache.Load(ctx, channel)
		if loadErr == nil && found && service.cacheFresh(cached, now) {
			if result, resultErr := service.result(current, cached, SourceCache); resultErr == nil {
				return result, nil
			}
		}
	}

	if gate == nil {
		return Result{}, errors.New("privacy network gate is unavailable")
	}
	if err := gate.Authorize(ctx, privacy.Request{Operation: "update.check", Purpose: privacy.ExternalResource}); err != nil {
		if errors.Is(err, privacy.ErrNetworkBlocked) {
			return Result{
				Status: Unavailable, Source: SourceNone, Channel: channel,
				CurrentVersion: current.String(), Detail: "network access is disabled by privacy policy",
			}, nil
		}
		return Result{}, err
	}
	if service.provider == nil {
		return Result{}, errors.New("release provider is unavailable")
	}

	release, found, err := service.provider.Latest(ctx, channel)
	if err != nil {
		if errors.Is(err, ErrProviderUnavailable) {
			return Result{
				Status: Unavailable, Source: SourceNone, Channel: channel,
				CurrentVersion: current.String(), Detail: "release metadata is temporarily unavailable",
			}, nil
		}
		return Result{}, err
	}
	check := CachedCheck{Channel: channel, CheckedAt: now, Found: found, Release: release}
	result, err := service.result(current, check, SourceNetwork)
	if err != nil {
		return Result{}, fmt.Errorf("release provider metadata: %w", err)
	}
	if service.cache != nil {
		_ = service.cache.Save(context.WithoutCancel(ctx), check)
	}
	return result, nil
}

func (service *Service) cacheFresh(check CachedCheck, now time.Time) bool {
	return service.ttl > 0 && !check.CheckedAt.After(now) && now.Sub(check.CheckedAt) < service.ttl
}

func (service *Service) result(current Version, check CachedCheck, source Source) (Result, error) {
	result := Result{
		Status: UpToDate, Source: source, Channel: check.Channel,
		CurrentVersion: current.String(), CheckedAt: check.CheckedAt,
	}
	if !check.Channel.Valid() || check.CheckedAt.IsZero() {
		return Result{}, errors.New("channel or check timestamp is invalid")
	}
	if !check.Found {
		result.Detail = "no published releases found"
		return result, nil
	}
	latest, err := ParseVersion(check.Release.Version)
	if err != nil {
		return Result{}, err
	}
	if check.Channel == Stable && latest.IsPrerelease() {
		return Result{}, errors.New("stable channel returned a prerelease version")
	}
	result.LatestVersion = latest.String()
	result.ReleaseURL = check.Release.URL
	if latest.Compare(current) > 0 {
		result.Status = UpdateAvailable
	} else if latest.Compare(current) < 0 {
		result.Detail = "installed version is newer than the published release; no downgrade will be offered"
	}
	return result, nil
}
