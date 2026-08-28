package community

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestPolicyV1SupportsAllCommunityResourceTypesAsSupplements(t *testing.T) {
	t.Parallel()
	tests := []struct {
		resourceType ResourceType
		kind         research.SourceKind
	}{
		{ResourceBlog, research.SourceCommunityArticle},
		{ResourceForum, research.SourceCommunityForum},
		{ResourceQuestionAnswer, research.SourceCommunityForum},
		{ResourceConferenceTalk, research.SourceVideo},
		{ResourceCommunityTutorial, research.SourceCommunityArticle},
		{ResourceRepositoryExample, research.SourceCode},
	}
	for _, test := range tests {
		test := test
		t.Run(string(test.resourceType), func(t *testing.T) {
			t.Parallel()
			input := communityInput(t, test.resourceType, test.kind)
			got, err := (PolicyV1{}).Evaluate(input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Role != RoleSupplementary || got.Tier != research.AuthorityTierD || got.PolicyVersion != PolicyVersionV1 {
				t.Fatalf("assessment = %+v", got)
			}
			if got.Attribution.Title == "" || got.Attribution.Locator != input.Source.Locator {
				t.Fatalf("attribution = %+v", got.Attribution)
			}
		})
	}
}

func TestPolicyV1ElevatesOnlyAnExplicitAuthorityProfileMatch(t *testing.T) {
	t.Parallel()
	input := communityInput(t, ResourceBlog, research.SourceCommunityArticle)
	input.Organization = "Recognized Practitioners"
	profile := communityProfile(t, input.Source.Kind)
	profile.PreferredDomains = nil
	input.AuthorityProfile = &profile

	got, err := (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != RoleRecognizedSupplementary || got.Tier != research.AuthorityTierC || !got.AuthorityElevated {
		t.Fatalf("recognized assessment = %+v", got)
	}

	input.Organization = "Unreviewed Author"
	got, err = (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != RoleSupplementary || got.Tier != research.AuthorityTierD || got.AuthorityElevated {
		t.Fatalf("unrecognized assessment = %+v", got)
	}
}

func TestPolicyV1NeverElevatesCommentsAndRequiresFreshnessReview(t *testing.T) {
	t.Parallel()
	input := communityInput(t, ResourceQuestionAnswer, research.SourceCommunityForum)
	input.Contribution = ContributionComment
	input.Freshness = research.FreshnessStale
	profile := communityProfile(t, input.Source.Kind)
	input.AuthorityProfile = &profile
	input.Organization = "Recognized Practitioners"

	got, err := (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != RoleContextOnly || got.Tier != research.AuthorityTierD || !got.RequiresVerification || got.AuthorityElevated {
		t.Fatalf("comment assessment = %+v", got)
	}
	assertCommunityReason(t, got, ReasonCommentContext)
}

func TestPolicyV1PopularityDoesNotAffectAuthorityOrRole(t *testing.T) {
	t.Parallel()
	low := communityInput(t, ResourceForum, research.SourceCommunityForum)
	low.Engagement = EngagementSignals{Present: true}
	high := low
	high.Engagement = EngagementSignals{Present: true, Votes: 9_000_000, Views: 50_000_000}

	lowResult, err := (PolicyV1{}).Evaluate(low)
	if err != nil {
		t.Fatal(err)
	}
	highResult, err := (PolicyV1{}).Evaluate(high)
	if err != nil {
		t.Fatal(err)
	}
	if lowResult.Role != highResult.Role || lowResult.Tier != highResult.Tier || lowResult.RequiresVerification != highResult.RequiresVerification {
		t.Fatalf("popularity changed policy: low=%+v high=%+v", lowResult, highResult)
	}
	if !reflect.DeepEqual(lowResult.Reasons, highResult.Reasons) {
		t.Fatalf("popularity counts leaked into reasons: low=%+v high=%+v", lowResult.Reasons, highResult.Reasons)
	}
	assertCommunityReason(t, highResult, ReasonPopularityIgnored)
}

func TestPolicyV1RejectsMismatchedKindsProfilesAndInvalidOutput(t *testing.T) {
	t.Parallel()
	input := communityInput(t, ResourceRepositoryExample, research.SourceCommunityArticle)
	if _, err := (PolicyV1{}).Evaluate(input); err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Fatalf("kind mismatch error = %v", err)
	}

	input = communityInput(t, ResourceBlog, research.SourceCommunityArticle)
	profile := communityProfile(t, input.Source.Kind)
	profile.Domain = "medicine"
	input.AuthorityProfile = &profile
	if _, err := (PolicyV1{}).Evaluate(input); err == nil || !strings.Contains(err.Error(), "does not match topic") {
		t.Fatalf("profile mismatch error = %v", err)
	}

	valid, err := (PolicyV1{}).Evaluate(communityInput(t, ResourceBlog, research.SourceCommunityArticle))
	if err != nil {
		t.Fatal(err)
	}
	valid.Role = RoleRecognizedSupplementary
	if err := valid.Validate(); err == nil || !strings.Contains(err.Error(), "default community") {
		t.Fatalf("divergent output error = %v", err)
	}
}

func communityInput(t *testing.T, resourceType ResourceType, kind research.SourceKind) Input {
	t.Helper()
	created := communityTimestamp(t)
	id, _ := research.NewSourceID("community.fixture")
	locator, _ := research.NewSourceLocator("https://community.example.org/resources/fixture")
	topic, _ := research.NewResearchTopic("portable request routing", "software", "runtime")
	return Input{
		Source: research.Source{
			ID: id, Kind: kind, Locator: locator, TemporalScope: research.SourceTemporalCurrent,
			Metadata:  research.SourceMetadata{Title: "Practical routing notes", Publisher: "Community Publisher", Language: "en", PublishedAt: &created},
			CreatedAt: created,
		},
		Topic: topic, ResourceType: resourceType, Contribution: ContributionResource,
		Author: "A. Practitioner", Freshness: research.FreshnessFresh,
	}
}

func communityProfile(t *testing.T, kind research.SourceKind) research.AuthorityProfile {
	t.Helper()
	id, _ := research.NewID("authority.community.fixture")
	return research.AuthorityProfile{
		ID: id, Version: "authority-profile/v1", Domain: "software", TopicPattern: "runtime/*",
		PreferredKinds: []research.SourceKind{kind}, PreferredDomains: []string{"community.example.org"},
		PreferredOrganizations: []string{"Recognized Practitioners"}, MinimumCorroboration: 2,
		MinimumTier: research.AuthorityTierC, CreatedAt: communityTimestamp(t),
	}
}

func communityTimestamp(t *testing.T) research.Timestamp {
	t.Helper()
	value, err := research.NewTimestamp(time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func assertCommunityReason(t *testing.T, assessment Assessment, code ReasonCode) {
	t.Helper()
	for _, reason := range assessment.Reasons {
		if reason.Code == code {
			return
		}
	}
	t.Fatalf("assessment reasons do not contain %q: %+v", code, assessment.Reasons)
}
