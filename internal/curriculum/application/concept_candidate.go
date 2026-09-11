package application

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type ConceptCandidateExtractorV1 struct{}

func NewConceptCandidateExtractorV1() ConceptCandidateExtractorV1 {
	return ConceptCandidateExtractorV1{}
}

type candidateGroupKey struct {
	subject string
	version string
	status  curriculum.ConceptStatus
}

type candidateGroup struct {
	scope string
	refs  []curriculum.EvidenceRef
}

func (ConceptCandidateExtractorV1) Extract(ctx context.Context, request ConceptCandidateExtractionRequest) (curriculum.ConceptCandidateSet, error) {
	const operation = "extract concept candidates"
	if err := ctx.Err(); err != nil {
		return curriculum.ConceptCandidateSet{}, ExternalError(operation, err)
	}
	if len(request.EvidenceSets) == 0 {
		return curriculum.ConceptCandidateSet{}, Invalid(operation, fmt.Errorf("concept candidate extraction requires evidence"))
	}

	groups := make(map[candidateGroupKey]*candidateGroup)
	seenRefs := make(map[evidenceKey]struct{})
	for index, set := range request.EvidenceSets {
		if err := set.Validate(); err != nil {
			return curriculum.ConceptCandidateSet{}, Invalid(operation, fmt.Errorf("evidence set %d: %w", index, err))
		}
		if set.Eligibility == curriculum.EvidenceNotReady {
			return curriculum.ConceptCandidateSet{}, Invalid(operation, fmt.Errorf("evidence set %q is not ready", set.Bundle.ID))
		}
		for _, claim := range set.Claims {
			if err := ctx.Err(); err != nil {
				return curriculum.ConceptCandidateSet{}, ExternalError(operation, err)
			}
			refKey := evidenceKey{bundle: set.Bundle.ID.String(), claim: claim.ID.String()}
			if _, exists := seenRefs[refKey]; exists {
				return curriculum.ConceptCandidateSet{}, Invalid(operation, fmt.Errorf("duplicate evidence claim reference %s/%s", refKey.bundle, refKey.claim))
			}
			seenRefs[refKey] = struct{}{}
			key := candidateGroupKey{subject: semanticSubjectV1(claim.Scope), version: claim.VersionScope, status: claim.Status}
			group := groups[key]
			if group == nil {
				group = &candidateGroup{scope: claim.Scope}
				groups[key] = group
			} else if claim.Scope < group.scope {
				// Equivalent spelling is selected without depending on input order.
				group.scope = claim.Scope
			}
			group.refs = append(group.refs, curriculum.EvidenceRef{BundleID: set.Bundle.ID, ClaimID: claim.ID})
		}
	}

	keys := make([]candidateGroupKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].subject != keys[j].subject {
			return keys[i].subject < keys[j].subject
		}
		if keys[i].version != keys[j].version {
			return keys[i].version < keys[j].version
		}
		return keys[i].status < keys[j].status
	})
	result := curriculum.ConceptCandidateSet{AlgorithmVersion: curriculum.ConceptCandidateExtractorVersionV1}
	for _, key := range keys {
		group := groups[key]
		sort.Slice(group.refs, func(i, j int) bool {
			left := group.refs[i].BundleID.String() + "\x00" + group.refs[i].ClaimID.String()
			right := group.refs[j].BundleID.String() + "\x00" + group.refs[j].ClaimID.String()
			return left < right
		})
		id, err := curriculum.NewID(candidateIDV1(key))
		if err != nil {
			return curriculum.ConceptCandidateSet{}, Invalid(operation, err)
		}
		result.Candidates = append(result.Candidates, curriculum.ConceptCandidate{
			ID: id, Title: group.scope, Scope: group.scope, ClaimRefs: append([]curriculum.EvidenceRef(nil), group.refs...),
			VersionScope: key.version, Status: key.status,
		})
	}
	if err := result.Validate(); err != nil {
		return curriculum.ConceptCandidateSet{}, Invalid(operation, err)
	}
	return result, nil
}

func semanticSubjectV1(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func candidateIDV1(key candidateGroupKey) string {
	payload := curriculum.ConceptCandidateExtractorVersionV1 + "\x00" + key.subject + "\x00" + key.version + "\x00" + string(key.status)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("candidate.%x", sum[:])
}

var _ ConceptCandidateExtractorService = ConceptCandidateExtractorV1{}
