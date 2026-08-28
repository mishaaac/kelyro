package research

import (
	"fmt"
	"strings"
)

type ResearchRequest struct {
	ID            ID
	Topic         ResearchTopic
	Purpose       ResearchPurpose
	TargetVersion *SourceVersion
	RequestedAt   Timestamp
}

func (request ResearchRequest) Validate() error {
	if err := request.ID.Validate(); err != nil {
		return fmt.Errorf("research request: %w", err)
	}
	if err := request.Topic.Validate(); err != nil {
		return err
	}
	if err := request.Purpose.Validate(); err != nil {
		return err
	}
	if request.TargetVersion != nil {
		if err := request.TargetVersion.Validate(); err != nil {
			return err
		}
	}
	return validateTimestamp("research requested at", request.RequestedAt)
}

type ResearchRunStatus string

const (
	ResearchRunPlanned   ResearchRunStatus = "planned"
	ResearchRunRunning   ResearchRunStatus = "running"
	ResearchRunCompleted ResearchRunStatus = "completed"
	ResearchRunFailed    ResearchRunStatus = "failed"
	ResearchRunCancelled ResearchRunStatus = "cancelled"
)

func (status ResearchRunStatus) Validate() error {
	switch status {
	case ResearchRunPlanned, ResearchRunRunning, ResearchRunCompleted,
		ResearchRunFailed, ResearchRunCancelled:
		return nil
	default:
		return fmt.Errorf("invalid research run status %q", status)
	}
}

type ResearchRun struct {
	ID          ID
	RequestID   ID
	Status      ResearchRunStatus
	StartedAt   Timestamp
	CompletedAt *Timestamp
	Cost        *ResearchCostMetadata
}

func (run ResearchRun) Validate() error {
	if err := run.ID.Validate(); err != nil {
		return fmt.Errorf("research run: %w", err)
	}
	if err := run.RequestID.Validate(); err != nil {
		return fmt.Errorf("research run request: %w", err)
	}
	if err := run.Status.Validate(); err != nil {
		return err
	}
	if err := validateTimestamp("research run started at", run.StartedAt); err != nil {
		return err
	}
	if err := validateOptionalTimestamp("research run completed at", run.CompletedAt); err != nil {
		return err
	}
	if run.CompletedAt != nil && run.CompletedAt.Before(run.StartedAt) {
		return fmt.Errorf("research run completion precedes start")
	}
	if run.Cost != nil {
		if err := run.Cost.Validate(); err != nil {
			return fmt.Errorf("research run cost: %w", err)
		}
	}
	switch run.Status {
	case ResearchRunCompleted, ResearchRunFailed, ResearchRunCancelled:
		if run.CompletedAt == nil {
			return fmt.Errorf("terminal research run has no completion timestamp")
		}
	case ResearchRunPlanned, ResearchRunRunning:
		if run.CompletedAt != nil {
			return fmt.Errorf("non-terminal research run has a completion timestamp")
		}
	}
	return nil
}

type AuthorityTier string

const (
	AuthorityTierA AuthorityTier = "A"
	AuthorityTierB AuthorityTier = "B"
	AuthorityTierC AuthorityTier = "C"
	AuthorityTierD AuthorityTier = "D"
	AuthorityTierE AuthorityTier = "E"
)

func (tier AuthorityTier) Validate() error {
	switch tier {
	case AuthorityTierA, AuthorityTierB, AuthorityTierC, AuthorityTierD, AuthorityTierE:
		return nil
	default:
		return fmt.Errorf("invalid authority tier %q", tier)
	}
}

// AuthorityProfile is declarative, topic-aware authority data. It describes
// preferences only; it never turns a source into trusted evidence by itself.
type AuthorityProfile struct {
	ID                        ID
	Version                   string
	Domain                    string
	TopicPattern              string
	PreferredKinds            []SourceKind
	PreferredDomains          []string
	PreferredOrganizations    []string
	MinimumCorroboration      int
	AllowedSupplementaryKinds []SourceKind
	FreshnessTTLHints         []FreshnessTTLHint
	MinimumTier               AuthorityTier
	CreatedAt                 Timestamp
}

func (profile AuthorityProfile) Validate() error {
	if err := profile.ID.Validate(); err != nil {
		return fmt.Errorf("authority profile: %w", err)
	}
	if err := requireText("authority profile version", profile.Version); err != nil {
		return err
	}
	if err := validateAuthorityDomain(profile.Domain); err != nil {
		return err
	}
	if err := validateTopicPattern(profile.TopicPattern); err != nil {
		return err
	}
	if len(profile.PreferredKinds) == 0 {
		return fmt.Errorf("authority profile preferred kinds are empty")
	}
	seen := make(map[SourceKind]struct{}, len(profile.PreferredKinds))
	for _, kind := range profile.PreferredKinds {
		if err := kind.Validate(); err != nil {
			return err
		}
		if _, exists := seen[kind]; exists {
			return fmt.Errorf("authority profile contains duplicate source kind %q", kind)
		}
		seen[kind] = struct{}{}
	}
	if err := validatePreferredDomains(profile.PreferredDomains); err != nil {
		return err
	}
	if err := validateUniqueText("authority profile preferred organization", profile.PreferredOrganizations); err != nil {
		return err
	}
	if profile.MinimumCorroboration < 1 {
		return fmt.Errorf("authority profile minimum corroboration must be at least 1")
	}
	supplementary := make(map[SourceKind]struct{}, len(profile.AllowedSupplementaryKinds))
	for _, kind := range profile.AllowedSupplementaryKinds {
		if err := kind.Validate(); err != nil {
			return err
		}
		if _, exists := supplementary[kind]; exists {
			return fmt.Errorf("authority profile contains duplicate supplementary source kind %q", kind)
		}
		if _, preferred := seen[kind]; preferred {
			return fmt.Errorf("authority profile source kind %q is both preferred and supplementary", kind)
		}
		supplementary[kind] = struct{}{}
	}
	if len(profile.FreshnessTTLHints) > MaximumFreshnessTTLHints {
		return fmt.Errorf("authority profile freshness TTL hints exceed %d", MaximumFreshnessTTLHints)
	}
	seenTTLHints := make(map[string]struct{}, len(profile.FreshnessTTLHints))
	for index, hint := range profile.FreshnessTTLHints {
		if err := hint.Validate(); err != nil {
			return fmt.Errorf("authority profile freshness TTL hint %d: %w", index, err)
		}
		key := freshnessTTLHintKey(hint)
		if _, exists := seenTTLHints[key]; exists {
			return fmt.Errorf("authority profile contains duplicate freshness TTL hint %q", key)
		}
		seenTTLHints[key] = struct{}{}
	}
	if err := profile.MinimumTier.Validate(); err != nil {
		return err
	}
	return validateTimestamp("authority profile created at", profile.CreatedAt)
}

func freshnessTTLHintKey(hint FreshnessTTLHint) string {
	claimType := "*"
	if hint.ClaimType != nil {
		claimType = string(*hint.ClaimType)
	}
	sourceKind := "*"
	if hint.SourceKind != nil {
		sourceKind = string(*hint.SourceKind)
	}
	return claimType + "/" + sourceKind
}

func validateAuthorityDomain(value string) error {
	if err := requireText("authority profile domain", value); err != nil {
		return err
	}
	if value == "*" {
		return nil
	}
	if value != strings.ToLower(value) || strings.TrimSpace(value) != value {
		return fmt.Errorf("authority profile domain %q must be a lowercase domain label or *", value)
	}
	for index, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			continue
		}
		if (character == '-' || character == '_' || character == '.') && index > 0 && index < len(value)-1 {
			continue
		}
		return fmt.Errorf("invalid authority profile domain %q", value)
	}
	return nil
}

func validateTopicPattern(value string) error {
	if value == "" { // Legacy persisted profiles use empty as the fallback.
		return nil
	}
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("authority profile topic pattern has surrounding whitespace")
	}
	if strings.ContainsAny(value, "?[]\\") || strings.Contains(value, "**") {
		return fmt.Errorf("invalid authority profile topic pattern %q: only * wildcards are supported", value)
	}
	return requireText("authority profile topic pattern", value)
}

func validatePreferredDomains(patterns []string) error {
	seen := make(map[string]struct{}, len(patterns))
	for _, pattern := range patterns {
		if pattern == "" || pattern != strings.TrimSpace(pattern) || pattern != strings.ToLower(pattern) {
			return fmt.Errorf("invalid preferred domain pattern %q", pattern)
		}
		host := strings.TrimPrefix(pattern, "*.")
		if strings.Contains(host, "*") || strings.ContainsAny(host, "/:@") {
			return fmt.Errorf("invalid preferred domain pattern %q", pattern)
		}
		labels := strings.Split(host, ".")
		if len(labels) < 2 {
			return fmt.Errorf("invalid preferred domain pattern %q", pattern)
		}
		for _, label := range labels {
			if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return fmt.Errorf("invalid preferred domain pattern %q", pattern)
			}
			for _, character := range label {
				if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' {
					continue
				}
				return fmt.Errorf("invalid preferred domain pattern %q", pattern)
			}
		}
		if _, exists := seen[pattern]; exists {
			return fmt.Errorf("authority profile contains duplicate preferred domain %q", pattern)
		}
		seen[pattern] = struct{}{}
	}
	return nil
}

func validateUniqueText(name string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if err := requireText(name, value); err != nil {
			return err
		}
		if strings.TrimSpace(value) != value {
			return fmt.Errorf("%s %q has surrounding whitespace", name, value)
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate %s %q", name, value)
		}
		seen[key] = struct{}{}
	}
	return nil
}

type TrustDecisionState string

const (
	TrustAccepted             TrustDecisionState = "accepted"
	TrustAcceptedSupplement   TrustDecisionState = "accepted_as_supplement"
	TrustRequiresVerification TrustDecisionState = "requires_verification"
	TrustRejected             TrustDecisionState = "rejected"
)

func (state TrustDecisionState) Validate() error {
	switch state {
	case TrustAccepted, TrustAcceptedSupplement, TrustRequiresVerification, TrustRejected:
		return nil
	default:
		return fmt.Errorf("invalid trust decision %q", state)
	}
}

type TrustReason struct {
	Code   string
	Detail string
}

func (reason TrustReason) Validate() error {
	if err := requireText("trust reason code", reason.Code); err != nil {
		return err
	}
	return validateOptionalText("trust reason detail", reason.Detail)
}

// TrustDecision records policy output without implementing trust-policy-v1.
type TrustDecision struct {
	SourceID    SourceID
	State       TrustDecisionState
	Tier        AuthorityTier
	Reasons     []TrustReason
	Policy      string
	EvaluatedAt Timestamp
}

func (decision TrustDecision) Validate() error {
	if err := decision.SourceID.Validate(); err != nil {
		return err
	}
	if err := decision.State.Validate(); err != nil {
		return err
	}
	if err := decision.Tier.Validate(); err != nil {
		return err
	}
	if len(decision.Reasons) == 0 {
		return fmt.Errorf("trust decision reasons are empty")
	}
	for _, reason := range decision.Reasons {
		if err := reason.Validate(); err != nil {
			return err
		}
	}
	if err := requireText("trust policy version", decision.Policy); err != nil {
		return err
	}
	return validateTimestamp("trust evaluated at", decision.EvaluatedAt)
}

// DiscoveredSource is an unverified candidate. It must never be treated as
// evidence until it has been classified, fetched, and snapshotted.
type DiscoveredSource struct {
	ID           ID
	RequestID    ID
	Locator      SourceLocator
	Title        string
	Provider     string
	Rank         int
	DiscoveredAt Timestamp
}

func (source DiscoveredSource) Validate() error {
	if err := source.ID.Validate(); err != nil {
		return fmt.Errorf("discovered source: %w", err)
	}
	if err := source.RequestID.Validate(); err != nil {
		return fmt.Errorf("discovered source request: %w", err)
	}
	if err := source.Locator.Validate(); err != nil {
		return err
	}
	if err := requireText("discovered source title", source.Title); err != nil {
		return err
	}
	if err := requireText("discovery provider", source.Provider); err != nil {
		return err
	}
	if source.Rank < 0 {
		return fmt.Errorf("discovery rank is negative")
	}
	return validateTimestamp("source discovered at", source.DiscoveredAt)
}
