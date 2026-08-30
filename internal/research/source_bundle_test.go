package research

import (
	"bytes"
	"testing"
)

func TestSourceBundleSerializationAndHashAreDeterministic(t *testing.T) {
	t.Parallel()
	topic, _ := NewResearchTopic("Deterministic bundles", "software", "Fixture")
	score, _ := NewFreshnessScore(0.8)
	first := SourceBundle{
		ID: mustID(t, "bundle.deterministic"), RunID: mustID(t, "run.deterministic"),
		Topic: topic, Purpose: PurposeConceptDefinition,
		ClaimIDs: []ClaimID{mustClaimID(t, "claim.z"), mustClaimID(t, "claim.a")},
		Sources: []SourceBundleSource{
			{SourceID: mustSourceID(t, "source.z"), Role: BundleSourceSupporting, TemporalScope: SourceTemporalCurrent},
			{SourceID: mustSourceID(t, "source.a"), Role: BundleSourcePrimary, TemporalScope: SourceTemporalCurrent},
		},
		ConflictIDs: []ID{mustID(t, "conflict.z"), mustID(t, "conflict.a")},
		Freshness: SourceBundleFreshness{
			State: FreshnessAging, Score: score, LastVerifiedAt: timestampPointer(mustTimestamp(t, 10)),
			SourceAlgorithms: []string{"freshness-z", "freshness-a"}, AlgorithmVersion: SourceBundleFreshnessV1,
		},
		Issues:     []SourceBundleIssue{BundleIssueResolvedConflict, BundleIssueAgingFreshness},
		VerifiedAt: mustTimestamp(t, 12),
	}
	second := first
	second.ClaimIDs = []ClaimID{first.ClaimIDs[1], first.ClaimIDs[0]}
	second.Sources = []SourceBundleSource{first.Sources[1], first.Sources[0]}
	second.ConflictIDs = []ID{first.ConflictIDs[1], first.ConflictIDs[0]}
	second.Freshness.SourceAlgorithms = []string{"freshness-a", "freshness-z"}
	second.Issues = []SourceBundleIssue{BundleIssueAgingFreshness, BundleIssueResolvedConflict}

	sealedFirst, err := SealSourceBundleV1(first)
	if err != nil {
		t.Fatal(err)
	}
	sealedSecond, err := SealSourceBundleV1(second)
	if err != nil {
		t.Fatal(err)
	}
	if sealedFirst.ContentHash != sealedSecond.ContentHash {
		t.Fatalf("bundle hashes differ: %q != %q", sealedFirst.ContentHash, sealedSecond.ContentHash)
	}
	firstJSON, err := sealedFirst.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := sealedSecond.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("canonical exports differ:\n%s\n%s", firstJSON, secondJSON)
	}
	parsed, err := ParseSourceBundleJSON(firstJSON)
	if err != nil || parsed.ContentHash != sealedFirst.ContentHash {
		t.Fatalf("parsed bundle = (%+v, %v)", parsed, err)
	}
	parsed.Summary += " tampered"
	if err := parsed.Validate(); err == nil {
		t.Fatal("SourceBundle.Validate() accepted a tampered hashed representation")
	}
}
