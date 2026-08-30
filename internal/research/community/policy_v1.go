package community

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/authority"
)

const (
	PolicyVersionV1         = "community-resource-policy-v1"
	maximumAttributionRunes = 256
)

type ResourceType string

const (
	ResourceBlog              ResourceType = "blog"
	ResourceForum             ResourceType = "forum"
	ResourceQuestionAnswer    ResourceType = "question_answer"
	ResourceConferenceTalk    ResourceType = "conference_talk"
	ResourceCommunityTutorial ResourceType = "community_tutorial"
	ResourceRepositoryExample ResourceType = "repository_example"
)

func (resourceType ResourceType) Validate() error {
	switch resourceType {
	case ResourceBlog, ResourceForum, ResourceQuestionAnswer,
		ResourceConferenceTalk, ResourceCommunityTutorial, ResourceRepositoryExample:
		return nil
	default:
		return fmt.Errorf("invalid community resource type %q", resourceType)
	}
}

type ContributionKind string

const (
	ContributionResource ContributionKind = "resource"
	ContributionComment  ContributionKind = "comment"
)

func (kind ContributionKind) Validate() error {
	switch kind {
	case ContributionResource, ContributionComment:
		return nil
	default:
		return fmt.Errorf("invalid community contribution kind %q", kind)
	}
}

type Role string

const (
	RoleSupplementary           Role = "supplementary"
	RoleRecognizedSupplementary Role = "recognized_supplementary"
	RoleContextOnly             Role = "context_only"
)

func (role Role) Validate() error {
	switch role {
	case RoleSupplementary, RoleRecognizedSupplementary, RoleContextOnly:
		return nil
	default:
		return fmt.Errorf("invalid community resource role %q", role)
	}
}

// EngagementSignals are discovery/UI metadata only. PolicyV1 deliberately
// never uses their values to derive truth, authority, or evidence strength.
type EngagementSignals struct {
	Present bool
	Votes   int64
	Views   int64
}

func (signals EngagementSignals) Validate() error {
	if signals.Votes < 0 || signals.Views < 0 {
		return fmt.Errorf("community engagement counts cannot be negative")
	}
	if !signals.Present && (signals.Votes != 0 || signals.Views != 0) {
		return fmt.Errorf("community engagement counts require present=true")
	}
	return nil
}

type Attribution struct {
	Title        string
	Author       string
	Organization string
	Publisher    string
	Locator      research.SourceLocator
}

func (attribution Attribution) Validate() error {
	if err := validateRequiredText("community attribution title", attribution.Title); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"author": attribution.Author, "organization": attribution.Organization,
		"publisher": attribution.Publisher,
	} {
		if err := validateOptionalText("community attribution "+name, value); err != nil {
			return err
		}
	}
	return attribution.Locator.Validate()
}

type ReasonCode string

const (
	ReasonResourceType      ReasonCode = "resource.type"
	ReasonDefaultSupplement ReasonCode = "role.supplementary_default"
	ReasonAuthorityElevated ReasonCode = "role.authority_profile_elevated"
	ReasonCommentContext    ReasonCode = "role.comment_context_only"
	ReasonFreshness         ReasonCode = "freshness.reviewed"
	ReasonPopularityIgnored ReasonCode = "popularity.ignored"
	ReasonAttribution       ReasonCode = "attribution.explicit"
)

type Reason struct {
	Code   ReasonCode
	Detail string
}

func (reason Reason) Validate() error {
	switch reason.Code {
	case ReasonResourceType, ReasonDefaultSupplement, ReasonAuthorityElevated,
		ReasonCommentContext, ReasonFreshness, ReasonPopularityIgnored, ReasonAttribution:
	default:
		return fmt.Errorf("invalid community policy reason %q", reason.Code)
	}
	return validateRequiredText("community policy reason detail", reason.Detail)
}

type Input struct {
	Source           research.Source
	Topic            research.ResearchTopic
	ResourceType     ResourceType
	Contribution     ContributionKind
	Author           string
	Organization     string
	Freshness        research.FreshnessState
	AuthorityProfile *research.AuthorityProfile
	Engagement       EngagementSignals
}

func (input Input) Validate() error {
	if err := input.Source.Validate(); err != nil {
		return fmt.Errorf("community source: %w", err)
	}
	if err := input.Topic.Validate(); err != nil {
		return fmt.Errorf("community topic: %w", err)
	}
	if err := input.ResourceType.Validate(); err != nil {
		return err
	}
	if !kindSupportsType(input.Source.Kind, input.ResourceType) {
		return fmt.Errorf("source kind %q does not support community resource type %q", input.Source.Kind, input.ResourceType)
	}
	if input.ResourceType == ResourceConferenceTalk && input.Source.Video != nil &&
		input.Source.Video.Affiliation != research.SourceAffiliationCommunity {
		return fmt.Errorf("community conference talk requires community video affiliation")
	}
	if err := input.Contribution.Validate(); err != nil {
		return err
	}
	if err := validateOptionalText("community author", input.Author); err != nil {
		return err
	}
	if err := validateOptionalText("community organization", input.Organization); err != nil {
		return err
	}
	if err := input.Freshness.Validate(); err != nil {
		return err
	}
	if err := input.Engagement.Validate(); err != nil {
		return err
	}
	if input.AuthorityProfile != nil {
		catalog, err := authority.NewCatalog([]research.AuthorityProfile{*input.AuthorityProfile})
		if err != nil {
			return fmt.Errorf("community authority profile: %w", err)
		}
		matched, found, err := catalog.Match(input.Topic)
		if err != nil || !found || matched.ID != input.AuthorityProfile.ID {
			return fmt.Errorf("community authority profile does not match topic")
		}
	}
	return nil
}

type Assessment struct {
	SourceID             research.SourceID
	ResourceType         ResourceType
	Contribution         ContributionKind
	Attribution          Attribution
	Freshness            research.FreshnessState
	Role                 Role
	Tier                 research.AuthorityTier
	AuthorityElevated    bool
	RequiresVerification bool
	Reasons              []Reason
	PolicyVersion        string
}

func (assessment Assessment) Validate() error {
	if err := assessment.SourceID.Validate(); err != nil {
		return err
	}
	if err := assessment.ResourceType.Validate(); err != nil {
		return err
	}
	if err := assessment.Contribution.Validate(); err != nil {
		return err
	}
	if err := assessment.Attribution.Validate(); err != nil {
		return err
	}
	if err := assessment.Freshness.Validate(); err != nil {
		return err
	}
	if err := assessment.Role.Validate(); err != nil {
		return err
	}
	if err := assessment.Tier.Validate(); err != nil {
		return err
	}
	if assessment.PolicyVersion != PolicyVersionV1 {
		return fmt.Errorf("community policy version must be %q", PolicyVersionV1)
	}
	if assessment.Contribution == ContributionComment {
		if assessment.Role != RoleContextOnly || assessment.Tier != research.AuthorityTierD || !assessment.RequiresVerification || assessment.AuthorityElevated {
			return fmt.Errorf("community comments must remain unprivileged context requiring verification")
		}
	} else if assessment.AuthorityElevated {
		if assessment.Role != RoleRecognizedSupplementary || assessment.Tier != research.AuthorityTierC {
			return fmt.Errorf("elevated community resource must be a tier C recognized supplement")
		}
	} else if assessment.Role != RoleSupplementary || assessment.Tier != research.AuthorityTierD {
		return fmt.Errorf("default community resource must be a tier D supplement")
	}
	if (assessment.Freshness == research.FreshnessStale || assessment.Freshness == research.FreshnessUnknown) && !assessment.RequiresVerification {
		return fmt.Errorf("stale or unknown community freshness requires verification")
	}
	if len(assessment.Reasons) < 4 || len(assessment.Reasons) > 5 {
		return fmt.Errorf("community assessment contains an invalid reason count")
	}
	seen := make(map[ReasonCode]struct{}, len(assessment.Reasons))
	for _, item := range assessment.Reasons {
		if err := item.Validate(); err != nil {
			return err
		}
		if _, exists := seen[item.Code]; exists {
			return fmt.Errorf("community assessment contains duplicate reason %q", item.Code)
		}
		seen[item.Code] = struct{}{}
	}
	for _, required := range []ReasonCode{ReasonResourceType, ReasonFreshness, ReasonAttribution} {
		if _, exists := seen[required]; !exists {
			return fmt.Errorf("community assessment is missing reason %q", required)
		}
	}
	roleReason := ReasonDefaultSupplement
	if assessment.Contribution == ContributionComment {
		roleReason = ReasonCommentContext
	} else if assessment.AuthorityElevated {
		roleReason = ReasonAuthorityElevated
	}
	if _, exists := seen[roleReason]; !exists {
		return fmt.Errorf("community assessment is missing role reason %q", roleReason)
	}
	return nil
}

type PolicyV1 struct{}

func (PolicyV1) Evaluate(input Input) (Assessment, error) {
	if err := input.Validate(); err != nil {
		return Assessment{}, fmt.Errorf("evaluate %s: %w", PolicyVersionV1, err)
	}
	attribution := Attribution{
		Title: input.Source.Metadata.Title, Author: input.Author,
		Organization: input.Organization, Publisher: input.Source.Metadata.Publisher,
		Locator: input.Source.Locator,
	}
	elevated := input.Contribution != ContributionComment && authorityRecognizes(input)
	role, tier, roleReason := RoleSupplementary, research.AuthorityTierD,
		Reason{Code: ReasonDefaultSupplement, Detail: "Community resources are supplementary by default and do not equal normative documentation."}
	if elevated {
		role, tier = RoleRecognizedSupplementary, research.AuthorityTierC
		roleReason = Reason{Code: ReasonAuthorityElevated, Detail: "A matching Authority Profile explicitly recognizes this source kind and organization or domain; it remains supplementary."}
	}
	requiresVerification := input.Freshness == research.FreshnessStale || input.Freshness == research.FreshnessUnknown
	if input.Contribution == ContributionComment {
		role, tier, elevated, requiresVerification = RoleContextOnly, research.AuthorityTierD, false, true
		roleReason = Reason{Code: ReasonCommentContext, Detail: "A discussion comment is contextual material, never strong evidence on its own."}
	}
	assessment := Assessment{
		SourceID: input.Source.ID, ResourceType: input.ResourceType,
		Contribution: input.Contribution, Attribution: attribution, Freshness: input.Freshness,
		Role: role, Tier: tier, AuthorityElevated: elevated,
		RequiresVerification: requiresVerification, PolicyVersion: PolicyVersionV1,
		Reasons: []Reason{
			{Code: ReasonResourceType, Detail: "The reviewed community resource type is " + string(input.ResourceType) + "."},
			roleReason,
			{Code: ReasonFreshness, Detail: "Community freshness was independently reviewed as " + string(input.Freshness) + "."},
			{Code: ReasonAttribution, Detail: "Title, publisher, optional author or organization, and canonical locator remain explicit."},
		},
	}
	if input.Engagement.Present {
		assessment.Reasons = append(assessment.Reasons, Reason{Code: ReasonPopularityIgnored, Detail: "Votes and views are retained as engagement signals but do not affect truth, authority, or role."})
	}
	if err := assessment.Validate(); err != nil {
		return Assessment{}, fmt.Errorf("evaluate %s output: %w", PolicyVersionV1, err)
	}
	assessment.Reasons = append([]Reason(nil), assessment.Reasons...)
	return assessment, nil
}

func kindSupportsType(kind research.SourceKind, resourceType ResourceType) bool {
	switch resourceType {
	case ResourceBlog, ResourceCommunityTutorial:
		return kind == research.SourceCommunityArticle
	case ResourceForum, ResourceQuestionAnswer:
		return kind == research.SourceCommunityForum
	case ResourceConferenceTalk:
		return kind == research.SourceVideo
	case ResourceRepositoryExample:
		return kind == research.SourceCode
	default:
		return false
	}
}

func authorityRecognizes(input Input) bool {
	if input.AuthorityProfile == nil || !containsKind(input.AuthorityProfile.PreferredKinds, input.Source.Kind) {
		return false
	}
	organization := input.Organization
	if organization == "" {
		organization = input.Source.Metadata.Publisher
	}
	if containsFold(input.AuthorityProfile.PreferredOrganizations, organization) {
		return true
	}
	parsed, err := url.Parse(input.Source.Locator.String())
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	for _, pattern := range input.AuthorityProfile.PreferredDomains {
		if host == pattern || (strings.HasPrefix(pattern, "*.") && strings.HasSuffix(host, pattern[1:]) && host != pattern[2:]) {
			return true
		}
	}
	return false
}

func containsKind(values []research.SourceKind, target research.SourceKind) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func validateRequiredText(name, value string) error {
	if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) || utf8.RuneCountInString(value) > maximumAttributionRunes {
		return fmt.Errorf("%s is invalid", name)
	}
	return nil
}

func validateOptionalText(name, value string) error {
	if value == "" {
		return nil
	}
	return validateRequiredText(name, value)
}
