package research

import "fmt"

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

// AuthorityProfile is declarative domain data. Matching and precedence
// algorithms are reserved for their dedicated implementation step.
type AuthorityProfile struct {
	ID             ID
	Version        string
	Domain         string
	TopicPattern   string
	PreferredKinds []SourceKind
	MinimumTier    AuthorityTier
	CreatedAt      Timestamp
}

func (profile AuthorityProfile) Validate() error {
	if err := profile.ID.Validate(); err != nil {
		return fmt.Errorf("authority profile: %w", err)
	}
	if err := requireText("authority profile version", profile.Version); err != nil {
		return err
	}
	if err := requireText("authority profile domain", profile.Domain); err != nil {
		return err
	}
	if err := validateOptionalText("authority profile topic pattern", profile.TopicPattern); err != nil {
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
	if err := profile.MinimumTier.Validate(); err != nil {
		return err
	}
	return validateTimestamp("authority profile created at", profile.CreatedAt)
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
