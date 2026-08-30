package research

import (
	"bytes"
	"testing"
	"time"
)

func TestResearchRunAuditV1CanonicalRoundTripAndValidation(t *testing.T) {
	started, _ := NewTimestamp(time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC))
	completed, _ := NewTimestamp(started.Time().Add(time.Minute))
	recorded, _ := NewTimestamp(completed.Time().Add(time.Second))
	id, _ := NewID("audit.run-one.completed")
	runID, _ := NewID("run.one")
	sourceID, _ := NewSourceID("source.docs")
	snapshotID, _ := NewID("snapshot.docs")
	locator, _ := NewSourceLocator("https://docs.example.test/guide")
	version, _ := NewSourceVersion("1.2")
	input := ResearchRunAudit{
		ID: id, RunID: runID, RecordedAt: recorded, StartedAt: started, CompletedAt: &completed,
		Outcome: ResearchRunCompleted, QueryPlannerVersion: "query-planner-v1", TrustPolicyVersion: "trust-policy-v1",
		FreshnessVersion: FreshnessAlgorithmV1, ConflictResolverVersion: ConflictResolverAlgorithmV1,
		ProvidersUsed: []string{"provider-z", "provider-a"}, NetworkMode: ResearchAuditNetworkAuto, NetworkAllowed: true,
		CacheHits: 2, BytesFetched: 2048, Queries: []string{"example guide official documentation"},
		Sources:          []ResearchAuditSource{{SourceID: sourceID, Locator: locator, SnapshotID: snapshotID, SnapshotHash: CanonicalContentHashV1([]byte("snapshot"))}},
		TargetTechnology: "Example", TargetVersion: &version,
		AdditionalAlgorithms: []ResearchAuditAlgorithm{{Stage: "verification", Version: MultiSourceVerificationAlgorithmV1}, {Stage: "normalization", Version: "source-normalization-v1"}},
	}
	audit, err := SealResearchRunAuditV1(input)
	if err != nil {
		t.Fatal(err)
	}
	if audit.SourceCount != 1 || audit.ProvidersUsed[0] != "provider-a" || audit.ContentHash == "" {
		t.Fatalf("sealed audit = %+v", audit)
	}
	encoded, err := audit.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseResearchRunAuditJSON(encoded)
	if err != nil || parsed.ContentHash != audit.ContentHash || parsed.Sources[0].SnapshotHash != audit.Sources[0].SnapshotHash {
		t.Fatalf("parsed audit = (%+v, %v)", parsed, err)
	}

	reordered := input
	reordered.ProvidersUsed = []string{"provider-a", "provider-z"}
	reordered.AdditionalAlgorithms = []ResearchAuditAlgorithm{{Stage: "normalization", Version: "source-normalization-v1"}, {Stage: "verification", Version: MultiSourceVerificationAlgorithmV1}}
	second, err := SealResearchRunAuditV1(reordered)
	if err != nil || second.ContentHash != audit.ContentHash {
		t.Fatalf("canonical hash = (%q, %v), want %q", second.ContentHash, err, audit.ContentHash)
	}

	tampered := append([]byte(nil), encoded...)
	tampered = bytes.Replace(tampered, []byte(`"cache_hits":2`), []byte(`"cache_hits":3`), 1)
	if _, err := ParseResearchRunAuditJSON(tampered); err == nil {
		t.Fatal("tampered audit JSON was accepted")
	}
}

func TestResearchRunAuditV1RejectsUnsafeOrInconsistentMetadata(t *testing.T) {
	started, _ := NewTimestamp(time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC))
	id, _ := NewID("audit.invalid")
	runID, _ := NewID("run.invalid")
	base := ResearchRunAudit{
		ID: id, RunID: runID, RecordedAt: started, StartedAt: started, Outcome: ResearchRunPlanned,
		QueryPlannerVersion: "query-planner-v1", TrustPolicyVersion: "trust-policy-v1",
		FreshnessVersion: FreshnessAlgorithmV1, ConflictResolverVersion: ConflictResolverAlgorithmV1,
		NetworkMode: ResearchAuditNetworkAuto, Queries: []string{"valid query"},
	}
	for name, mutate := range map[string]func(*ResearchRunAudit){
		"negative cache hits": func(value *ResearchRunAudit) { value.CacheHits = -1 },
		"offline allowed": func(value *ResearchRunAudit) {
			value.NetworkMode = ResearchAuditNetworkOffline
			value.NetworkAllowed = true
		},
		"missing query":               func(value *ResearchRunAudit) { value.Queries = nil },
		"terminal without completion": func(value *ResearchRunAudit) { value.Outcome = ResearchRunCompleted },
		"target whitespace":           func(value *ResearchRunAudit) { value.TargetTechnology = " Go " },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate)
			if _, err := SealResearchRunAuditV1(candidate); err == nil {
				t.Fatalf("SealResearchRunAuditV1(%s) accepted invalid metadata", name)
			}
		})
	}
}

func TestParseResearchRunAuditJSONRejectsUnknownTrailingAndOversizePayloads(t *testing.T) {
	started, _ := NewTimestamp(time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC))
	id, _ := NewID("audit.strict")
	runID, _ := NewID("run.strict")
	audit, err := SealResearchRunAuditV1(ResearchRunAudit{
		ID: id, RunID: runID, RecordedAt: started, StartedAt: started, Outcome: ResearchRunPlanned,
		QueryPlannerVersion: "query-planner-v1", TrustPolicyVersion: "trust-policy-v1",
		FreshnessVersion: FreshnessAlgorithmV1, ConflictResolverVersion: ConflictResolverAlgorithmV1,
		NetworkMode: ResearchAuditNetworkAuto, Queries: []string{"strict query"},
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := audit.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	unknown := append([]byte(nil), encoded[:len(encoded)-1]...)
	unknown = append(unknown, []byte(`,"unknown":true}`)...)
	for name, payload := range map[string][]byte{
		"unknown":  unknown,
		"trailing": append(append([]byte(nil), encoded...), []byte(` {}`)...),
		"oversize": bytes.Repeat([]byte("x"), MaximumResearchAuditJSONBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseResearchRunAuditJSON(payload); err == nil {
				t.Fatalf("ParseResearchRunAuditJSON(%s) accepted invalid payload", name)
			}
		})
	}
}
