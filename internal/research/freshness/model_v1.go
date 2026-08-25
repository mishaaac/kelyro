package freshness

import (
	"fmt"
	"math"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

type Clock interface {
	Now() research.Timestamp
}

type Input struct {
	LastVerifiedAt     *research.Timestamp
	SourceUpdatedAt    *research.Timestamp
	ReleaseCadenceDays int
	ClaimType          research.ClaimType
	SourceKind         research.SourceKind
	KnownNewRelease    bool
	AuthorityProfile   *research.AuthorityProfile
}

type ReasonCode string

const (
	ReasonMissingLastVerified ReasonCode = "missing_last_verified"
	ReasonDefaultTTL          ReasonCode = "default_ttl"
	ReasonAuthorityTTLHint    ReasonCode = "authority_ttl_hint"
	ReasonReleaseCadenceCap   ReasonCode = "release_cadence_cap"
	ReasonKnownNewRelease     ReasonCode = "known_new_release"
	ReasonSourceUpdated       ReasonCode = "source_updated_after_verification"
	ReasonAgeFresh            ReasonCode = "age_fresh"
	ReasonAgeAging            ReasonCode = "age_aging"
	ReasonAgeStale            ReasonCode = "age_stale"
)

func (code ReasonCode) Validate() error {
	switch code {
	case ReasonMissingLastVerified, ReasonDefaultTTL, ReasonAuthorityTTLHint,
		ReasonReleaseCadenceCap, ReasonKnownNewRelease, ReasonSourceUpdated,
		ReasonAgeFresh, ReasonAgeAging, ReasonAgeStale:
		return nil
	default:
		return fmt.Errorf("invalid freshness reason code %q", code)
	}
}

type Reason struct {
	Code   ReasonCode
	Detail string
}

func (reason Reason) Validate() error {
	if err := reason.Code.Validate(); err != nil {
		return err
	}
	if reason.Detail == "" {
		return fmt.Errorf("freshness reason detail is empty")
	}
	return nil
}

type Assessment struct {
	State            research.FreshnessState
	Score            research.FreshnessScore
	EvaluatedAt      research.Timestamp
	LastVerifiedAt   *research.Timestamp
	EffectiveTTLDays int
	AlgorithmVersion string
	Reasons          []Reason
}

func (assessment Assessment) Validate() error {
	if err := assessment.State.Validate(); err != nil {
		return err
	}
	if err := assessment.Score.Validate(); err != nil {
		return err
	}
	if err := assessment.EvaluatedAt.Validate(); err != nil {
		return fmt.Errorf("freshness evaluated at: %w", err)
	}
	if assessment.AlgorithmVersion != research.FreshnessAlgorithmV1 {
		return fmt.Errorf("freshness algorithm version must be %q", research.FreshnessAlgorithmV1)
	}
	if assessment.State == research.FreshnessUnknown {
		if assessment.LastVerifiedAt != nil || assessment.EffectiveTTLDays != 0 || assessment.Score.Value() != 0 {
			return fmt.Errorf("unknown freshness must have no verification and zero TTL and score")
		}
	} else {
		if assessment.LastVerifiedAt == nil {
			return fmt.Errorf("known freshness requires last verification")
		}
		if err := assessment.LastVerifiedAt.Validate(); err != nil {
			return fmt.Errorf("freshness last verified: %w", err)
		}
		if assessment.LastVerifiedAt.After(assessment.EvaluatedAt) {
			return fmt.Errorf("freshness last verification follows assessment")
		}
		if assessment.EffectiveTTLDays < research.MinimumFreshnessTTLDays || assessment.EffectiveTTLDays > research.MaximumFreshnessTTLDays {
			return fmt.Errorf("freshness effective TTL is outside supported bounds")
		}
	}
	if len(assessment.Reasons) == 0 {
		return fmt.Errorf("freshness assessment has no reasons")
	}
	seen := make(map[ReasonCode]struct{}, len(assessment.Reasons))
	for _, reason := range assessment.Reasons {
		if err := reason.Validate(); err != nil {
			return err
		}
		if _, exists := seen[reason.Code]; exists {
			return fmt.Errorf("freshness assessment contains duplicate reason %q", reason.Code)
		}
		seen[reason.Code] = struct{}{}
	}
	return nil
}

type ModelV1 struct {
	clock Clock
}

func NewModelV1(clock Clock) ModelV1 {
	return ModelV1{clock: clock}
}

func (model ModelV1) Assess(input Input) (Assessment, error) {
	if model.clock == nil {
		return Assessment{}, fmt.Errorf("assess freshness-v1: clock is not configured")
	}
	now := model.clock.Now()
	if err := now.Validate(); err != nil {
		return Assessment{}, fmt.Errorf("assess freshness-v1: clock: %w", err)
	}
	if err := validateInput(input, now); err != nil {
		return Assessment{}, fmt.Errorf("assess freshness-v1: %w", err)
	}
	if input.LastVerifiedAt == nil {
		return validatedAssessmentWithScore(Assessment{
			State: research.FreshnessUnknown, EvaluatedAt: now,
			AlgorithmVersion: research.FreshnessAlgorithmV1,
			Reasons:          []Reason{{Code: ReasonMissingLastVerified, Detail: "No last_verified_at is available."}},
		}, 0)
	}

	ttlDays, ttlReason := effectiveTTL(input)
	reasons := []Reason{ttlReason}
	if input.ReleaseCadenceDays > 0 && input.ReleaseCadenceDays < ttlDays {
		ttlDays = input.ReleaseCadenceDays
		reasons = append(reasons, Reason{Code: ReasonReleaseCadenceCap, Detail: fmt.Sprintf("Release cadence caps TTL at %d day(s).", ttlDays)})
	}
	triggered := false
	if input.KnownNewRelease {
		triggered = true
		reasons = append(reasons, Reason{Code: ReasonKnownNewRelease, Detail: "A relevant newer release is known."})
	}
	if input.SourceUpdatedAt != nil && input.SourceUpdatedAt.After(*input.LastVerifiedAt) {
		triggered = true
		reasons = append(reasons, Reason{Code: ReasonSourceUpdated, Detail: "The source was updated after last verification."})
	}
	if triggered {
		return validatedAssessmentWithScore(Assessment{
			State: research.FreshnessStale, EvaluatedAt: now,
			LastVerifiedAt:   cloneTimestamp(input.LastVerifiedAt),
			EffectiveTTLDays: ttlDays, AlgorithmVersion: research.FreshnessAlgorithmV1,
			Reasons: reasons,
		}, 0)
	}

	age := now.Time().Sub(input.LastVerifiedAt.Time())
	ttl := time.Duration(ttlDays) * 24 * time.Hour
	ratio := float64(age) / float64(ttl)
	value := math.Max(0, 1-ratio/2)
	state := research.FreshnessFresh
	ageReason := Reason{Code: ReasonAgeFresh, Detail: "Verification age is within half of the effective TTL."}
	if age > ttl {
		state = research.FreshnessStale
		ageReason = Reason{Code: ReasonAgeStale, Detail: "Verification age exceeds the effective TTL."}
	} else if age > ttl/2 {
		state = research.FreshnessAging
		ageReason = Reason{Code: ReasonAgeAging, Detail: "Verification age is beyond half of the effective TTL."}
	}
	reasons = append(reasons, ageReason)
	return validatedAssessmentWithScore(Assessment{
		State: state, EvaluatedAt: now, LastVerifiedAt: cloneTimestamp(input.LastVerifiedAt), EffectiveTTLDays: ttlDays,
		AlgorithmVersion: research.FreshnessAlgorithmV1, Reasons: reasons,
	}, value)
}

func validateInput(input Input, now research.Timestamp) error {
	if err := input.ClaimType.Validate(); err != nil {
		return err
	}
	if err := input.SourceKind.Validate(); err != nil {
		return err
	}
	if input.ReleaseCadenceDays < 0 || input.ReleaseCadenceDays > research.MaximumFreshnessTTLDays {
		return fmt.Errorf("release cadence days must be zero or between %d and %d", research.MinimumFreshnessTTLDays, research.MaximumFreshnessTTLDays)
	}
	if input.LastVerifiedAt != nil {
		if err := input.LastVerifiedAt.Validate(); err != nil {
			return fmt.Errorf("last verified at: %w", err)
		}
		if input.LastVerifiedAt.After(now) {
			return fmt.Errorf("last verification is in the future")
		}
	}
	if input.SourceUpdatedAt != nil {
		if err := input.SourceUpdatedAt.Validate(); err != nil {
			return fmt.Errorf("source updated at: %w", err)
		}
		if input.SourceUpdatedAt.After(now) {
			return fmt.Errorf("source update is in the future")
		}
	}
	if input.AuthorityProfile != nil {
		if err := input.AuthorityProfile.Validate(); err != nil {
			return fmt.Errorf("authority profile: %w", err)
		}
	}
	return nil
}

func effectiveTTL(input Input) (int, Reason) {
	if input.AuthorityProfile != nil {
		if ttl, found := matchingTTLHint(input.AuthorityProfile.FreshnessTTLHints, input.ClaimType, input.SourceKind); found {
			return ttl, Reason{Code: ReasonAuthorityTTLHint, Detail: fmt.Sprintf("Authority Profile selects a %d day TTL.", ttl)}
		}
	}
	ttl := min(defaultClaimTTLDays(input.ClaimType), defaultSourceTTLDays(input.SourceKind))
	return ttl, Reason{Code: ReasonDefaultTTL, Detail: fmt.Sprintf("Claim/source defaults select a %d day TTL.", ttl)}
}

func matchingTTLHint(hints []research.FreshnessTTLHint, claimType research.ClaimType, sourceKind research.SourceKind) (int, bool) {
	bestRank := -1
	bestTTL := 0
	for _, hint := range hints {
		if hint.ClaimType != nil && *hint.ClaimType != claimType {
			continue
		}
		if hint.SourceKind != nil && *hint.SourceKind != sourceKind {
			continue
		}
		rank := 0
		switch {
		case hint.ClaimType != nil && hint.SourceKind != nil:
			rank = 3
		case hint.ClaimType != nil:
			rank = 2
		case hint.SourceKind != nil:
			rank = 1
		}
		if rank > bestRank {
			bestRank = rank
			bestTTL = hint.TTLDays
		}
	}
	return bestTTL, bestRank >= 0
}

func defaultClaimTTLDays(claimType research.ClaimType) int {
	switch claimType {
	case research.ClaimSecurity:
		return 14
	case research.ClaimVersionChange, research.ClaimDeprecation, research.ClaimCompatibility:
		return 30
	case research.ClaimWarning:
		return 45
	case research.ClaimBehavior, research.ClaimRecommendation:
		return 90
	case research.ClaimExample:
		return 120
	case research.ClaimDefinition, research.ClaimRequirement:
		return 180
	case research.ClaimHistorical:
		return 365
	default:
		return 90
	}
}

func defaultSourceTTLDays(kind research.SourceKind) int {
	switch kind {
	case research.SourceReleaseNotes, research.SourceIssueTracker, research.SourceCommunityForum:
		return 30
	case research.SourceOfficialBlog, research.SourceCommunityArticle, research.SourceVideo:
		return 60
	case research.SourcePackageReference, research.SourceCode, research.SourceOther:
		return 90
	case research.SourceOfficialDocumentation, research.SourceOfficialTutorial:
		return 120
	case research.SourceSpecification, research.SourceStandard, research.SourcePaper:
		return 180
	case research.SourceBookReference:
		return 365
	default:
		return 90
	}
}

func validatedAssessment(assessment Assessment) (Assessment, error) {
	if err := assessment.Validate(); err != nil {
		return Assessment{}, err
	}
	assessment.Reasons = append([]Reason(nil), assessment.Reasons...)
	assessment.LastVerifiedAt = cloneTimestamp(assessment.LastVerifiedAt)
	return assessment, nil
}

func cloneTimestamp(value *research.Timestamp) *research.Timestamp {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func validatedAssessmentWithScore(assessment Assessment, value float64) (Assessment, error) {
	result, err := research.NewFreshnessScore(value)
	if err != nil {
		return Assessment{}, fmt.Errorf("freshness-v1 score: %w", err)
	}
	assessment.Score = result
	return validatedAssessment(assessment)
}
