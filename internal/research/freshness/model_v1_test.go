package freshness

import (
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestModelV1UsesInjectableClockAtTTLBoundaries(t *testing.T) {
	t.Parallel()
	now := freshnessTimestamp(t, time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC))
	model := NewModelV1(fixedClock{now})
	tests := []struct {
		name  string
		age   time.Duration
		state research.FreshnessState
		score float64
	}{
		{"newly verified", 0, research.FreshnessFresh, 1},
		{"fresh boundary", 45 * 24 * time.Hour, research.FreshnessFresh, .75},
		{"past fresh boundary", 45*24*time.Hour + time.Nanosecond, research.FreshnessAging, .75},
		{"aging boundary", 90 * 24 * time.Hour, research.FreshnessAging, .5},
		{"past TTL", 90*24*time.Hour + time.Nanosecond, research.FreshnessStale, .5},
		{"twice TTL", 180 * 24 * time.Hour, research.FreshnessStale, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			last := freshnessTimestamp(t, now.Time().Add(-test.age))
			got, err := model.Assess(Input{
				LastVerifiedAt: &last, ClaimType: research.ClaimBehavior,
				SourceKind: research.SourceOfficialDocumentation,
			})
			if err != nil {
				t.Fatalf("Assess() error = %v", err)
			}
			if got.State != test.state || got.EffectiveTTLDays != 90 || !approximately(got.Score.Value(), test.score) {
				t.Fatalf("Assess() = state %q score %.12f TTL %d, want %q %.12f 90", got.State, got.Score.Value(), got.EffectiveTTLDays, test.state, test.score)
			}
			if got.AlgorithmVersion != research.FreshnessAlgorithmV1 || !got.EvaluatedAt.Time().Equal(now.Time()) {
				t.Fatalf("assessment metadata = %+v", got)
			}
		})
	}
}

func TestModelV1AppliesAuthorityHintPrecedenceAndReleaseCadenceCap(t *testing.T) {
	t.Parallel()
	now := freshnessTimestamp(t, time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC))
	last := freshnessTimestamp(t, now.Time().Add(-20*24*time.Hour))
	claim := research.ClaimDefinition
	kind := research.SourceOfficialDocumentation
	otherKind := research.SourceOther
	profile := freshnessProfile(t, []research.FreshnessTTLHint{
		{TTLDays: 200},
		{SourceKind: &kind, TTLDays: 80},
		{ClaimType: &claim, TTLDays: 60},
		{ClaimType: &claim, SourceKind: &kind, TTLDays: 40},
		{ClaimType: &claim, SourceKind: &otherKind, TTLDays: 10},
	})
	got, err := NewModelV1(fixedClock{now}).Assess(Input{
		LastVerifiedAt: &last, ReleaseCadenceDays: 30,
		ClaimType: claim, SourceKind: kind, AuthorityProfile: &profile,
	})
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	if got.EffectiveTTLDays != 30 || got.State != research.FreshnessAging || !approximately(got.Score.Value(), 2.0/3.0) {
		t.Fatalf("Assess() = %+v", got)
	}
	wantReasons := []ReasonCode{ReasonAuthorityTTLHint, ReasonReleaseCadenceCap, ReasonAgeAging}
	if len(got.Reasons) != len(wantReasons) {
		t.Fatalf("reasons = %+v", got.Reasons)
	}
	for index, code := range wantReasons {
		if got.Reasons[index].Code != code {
			t.Fatalf("reason %d = %q, want %q", index, got.Reasons[index].Code, code)
		}
	}
}

func TestModelV1CombinesClaimAndSourceDefaultTTLs(t *testing.T) {
	t.Parallel()
	now := freshnessTimestamp(t, time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC))
	last := now
	tests := []struct {
		name       string
		claimType  research.ClaimType
		sourceKind research.SourceKind
		wantTTL    int
	}{
		{"security claim caps a long-lived source", research.ClaimSecurity, research.SourceBookReference, 14},
		{"release notes cap a definition", research.ClaimDefinition, research.SourceReleaseNotes, 30},
		{"interactive playground is rechecked frequently", research.ClaimDefinition, research.SourcePlayground, 30},
		{"historical book reference", research.ClaimHistorical, research.SourceBookReference, 365},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NewModelV1(fixedClock{now}).Assess(Input{
				LastVerifiedAt: &last, ClaimType: test.claimType, SourceKind: test.sourceKind,
			})
			if err != nil || got.EffectiveTTLDays != test.wantTTL || got.Reasons[0].Code != ReasonDefaultTTL {
				t.Fatalf("Assess() = (%+v, %v), want TTL %d", got, err, test.wantTTL)
			}
		})
	}
}

func TestModelV1ReleaseAndSourceUpdateTriggersForceStale(t *testing.T) {
	t.Parallel()
	now := freshnessTimestamp(t, time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC))
	last := freshnessTimestamp(t, now.Time().Add(-time.Hour))
	updated := freshnessTimestamp(t, last.Time().Add(time.Minute))
	got, err := NewModelV1(fixedClock{now}).Assess(Input{
		LastVerifiedAt: &last, SourceUpdatedAt: &updated,
		ClaimType: research.ClaimDefinition, SourceKind: research.SourceSpecification,
		KnownNewRelease: true,
	})
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	if got.State != research.FreshnessStale || got.Score.Value() != 0 {
		t.Fatalf("triggered assessment = %+v", got)
	}
	if !hasReason(got.Reasons, ReasonKnownNewRelease) || !hasReason(got.Reasons, ReasonSourceUpdated) {
		t.Fatalf("trigger reasons = %+v", got.Reasons)
	}
}

func TestModelV1ReturnsUnknownWithoutInventingLastVerified(t *testing.T) {
	t.Parallel()
	now := freshnessTimestamp(t, time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC))
	got, err := NewModelV1(fixedClock{now}).Assess(Input{
		ClaimType: research.ClaimHistorical, SourceKind: research.SourceBookReference,
		KnownNewRelease: true,
	})
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	if got.State != research.FreshnessUnknown || got.Score.Value() != 0 || got.EffectiveTTLDays != 0 ||
		len(got.Reasons) != 1 || got.Reasons[0].Code != ReasonMissingLastVerified {
		t.Fatalf("unknown assessment = %+v", got)
	}
}

func TestModelV1RejectsInvalidTemporalAndConfigurationInputs(t *testing.T) {
	t.Parallel()
	now := freshnessTimestamp(t, time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC))
	future := freshnessTimestamp(t, now.Time().Add(time.Second))
	valid := Input{ClaimType: research.ClaimBehavior, SourceKind: research.SourceOfficialDocumentation}
	tests := []struct {
		name   string
		model  ModelV1
		input  Input
		needle string
	}{
		{"missing clock", NewModelV1(nil), valid, "clock is not configured"},
		{"future verification", NewModelV1(fixedClock{now}), Input{LastVerifiedAt: &future, ClaimType: valid.ClaimType, SourceKind: valid.SourceKind}, "verification is in the future"},
		{"future update", NewModelV1(fixedClock{now}), Input{SourceUpdatedAt: &future, ClaimType: valid.ClaimType, SourceKind: valid.SourceKind}, "update is in the future"},
		{"negative cadence", NewModelV1(fixedClock{now}), Input{ReleaseCadenceDays: -1, ClaimType: valid.ClaimType, SourceKind: valid.SourceKind}, "release cadence"},
	}
	for _, test := range tests {
		if _, err := test.model.Assess(test.input); err == nil || !strings.Contains(err.Error(), test.needle) {
			t.Fatalf("Assess(%s) error = %v, want %q", test.name, err, test.needle)
		}
	}
}

type fixedClock struct{ now research.Timestamp }

func (clock fixedClock) Now() research.Timestamp { return clock.now }

func freshnessTimestamp(t *testing.T, value time.Time) research.Timestamp {
	t.Helper()
	result, err := research.NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func freshnessProfile(t *testing.T, hints []research.FreshnessTTLHint) research.AuthorityProfile {
	t.Helper()
	id, _ := research.NewID("authority.freshness")
	created := freshnessTimestamp(t, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC))
	return research.AuthorityProfile{
		ID: id, Version: "authority-profile/v1", Domain: "software", TopicPattern: "*",
		PreferredKinds:       []research.SourceKind{research.SourceSpecification},
		MinimumCorroboration: 1, FreshnessTTLHints: hints,
		MinimumTier: research.AuthorityTierC, CreatedAt: created,
	}
}

func approximately(left, right float64) bool { return mathAbs(left-right) < 1e-9 }

func mathAbs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func hasReason(reasons []Reason, code ReasonCode) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}
