package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestConceptCandidateExtractorV1GroupsDuplicateDefinitionAndBehaviorClaims(t *testing.T) {
	t.Parallel()
	evidence, _ := decompositionEvidence(t)
	evidence.Claims = []curriculum.CurriculumEvidenceClaim{
		candidateClaim(t, evidence, "claim.http.definition", curriculum.EvidenceClaimDefinition, "HTTP   Request", "", curriculum.ConceptCurrent, "An HTTP request is a client message."),
		candidateClaim(t, evidence, "claim.http.behavior", curriculum.EvidenceClaimBehavior, "http request", "", curriculum.ConceptCurrent, "An HTTP request carries a method and target."),
		candidateClaim(t, evidence, "claim.http.duplicate", curriculum.EvidenceClaimDefinition, "http request", "", curriculum.ConceptCurrent, "An HTTP request is a client message."),
	}

	service := NewConceptCandidateExtractorV1()
	result, err := service.Extract(context.Background(), ConceptCandidateExtractionRequest{EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if result.AlgorithmVersion != curriculum.ConceptCandidateExtractorVersionV1 || len(result.Candidates) != 1 {
		t.Fatalf("candidate set = %+v", result)
	}
	if got := result.Candidates[0]; got.Scope != "HTTP   Request" || len(got.ClaimRefs) != 3 {
		t.Fatalf("candidate = %+v", got)
	}

	reversed := evidence
	reversed.Claims = append([]curriculum.CurriculumEvidenceClaim(nil), evidence.Claims...)
	for left, right := 0, len(reversed.Claims)-1; left < right; left, right = left+1, right-1 {
		reversed.Claims[left], reversed.Claims[right] = reversed.Claims[right], reversed.Claims[left]
	}
	repeated, err := service.Extract(context.Background(), ConceptCandidateExtractionRequest{EvidenceSets: []curriculum.CurriculumEvidenceSet{reversed}})
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("reordered extraction differs: %+v / %+v / %v", result, repeated, err)
	}
}

func TestConceptCandidateExtractorV1SeparatesHistoricalAndVersionSpecificClaims(t *testing.T) {
	t.Parallel()
	evidence, _ := decompositionEvidence(t)
	evidence.Claims = []curriculum.CurriculumEvidenceClaim{
		candidateClaim(t, evidence, "claim.api.current", curriculum.EvidenceClaimBehavior, "Client API", "", curriculum.ConceptCurrent, "The current client sends requests."),
		candidateClaim(t, evidence, "claim.api.historical", curriculum.EvidenceClaimHistorical, "Client API", "", curriculum.ConceptHistorical, "The old client opened a connection directly."),
		candidateClaim(t, evidence, "claim.api.v2", curriculum.EvidenceClaimVersionChange, "Client API", "2.0", curriculum.ConceptCurrent, "Version 2 changes request construction."),
	}
	evidence.VersionScopes = []string{"2.0"}

	result, err := NewConceptCandidateExtractorV1().Extract(context.Background(), ConceptCandidateExtractionRequest{EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if len(result.Candidates) != 3 {
		t.Fatalf("candidates = %+v", result.Candidates)
	}
	want := map[string]curriculum.ConceptStatus{"": curriculum.ConceptCurrent, "2.0": curriculum.ConceptCurrent}
	foundHistorical := false
	for _, candidate := range result.Candidates {
		if candidate.Status == curriculum.ConceptHistorical {
			foundHistorical = candidate.VersionScope == ""
			continue
		}
		if status, exists := want[candidate.VersionScope]; !exists || status != candidate.Status {
			t.Fatalf("unexpected candidate = %+v", candidate)
		}
	}
	if !foundHistorical {
		t.Fatalf("historical candidate not preserved: %+v", result.Candidates)
	}
}

func candidateClaim(t *testing.T, evidence curriculum.CurriculumEvidenceSet, id string, kind curriculum.EvidenceClaimKind, scope, version string, status curriculum.ConceptStatus, statement string) curriculum.CurriculumEvidenceClaim {
	t.Helper()
	return curriculum.CurriculumEvidenceClaim{
		ID: curriculumID(t, id), Statement: statement, Kind: kind, Scope: scope,
		VersionScope: version, Status: status, Confidence: .9,
		SourceIDs: []curriculum.ID{evidence.SourceAuthority[0].SourceID},
	}
}
