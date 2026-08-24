package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestStudentCoreDatabaseMigratesToResearchWithoutLosingState(t *testing.T) {
	root := newWorkspaceRoot(t)
	path, err := platform.WorkspaceDBPath(root)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := sql.Open("sqlite", databaseURI(path, defaultOperationTimeout))
	if err != nil {
		t.Fatal(err)
	}
	handle.SetMaxOpenConns(1)
	database := &Database{sql: handle, path: path, timeout: defaultOperationTimeout, now: func() time.Time { return fixedTime }, version: "test"}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.migrate(context.Background(), foundationMigrations[:22]); err != nil {
		t.Fatalf("migrate through I-02: %v", err)
	}
	if _, err := handle.Exec(`INSERT INTO app_state (namespace,key,value,updated_at) VALUES ('i02','kept',X'6F6B',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if err := database.migrate(context.Background(), foundationMigrations[:23]); err != nil {
		t.Fatalf("migrate through research schema v23: %v", err)
	}
	if _, err := handle.Exec(`INSERT INTO authority_profiles (id,version,domain,topic_pattern,preferred_kinds_json,minimum_tier,created_at) VALUES ('authority.legacy','legacy/v1','software','*','["specification"]','C',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate I-03: %v", err)
	}
	var value []byte
	if err := handle.QueryRow(`SELECT value FROM app_state WHERE namespace='i02' AND key='kept'`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if string(value) != "ok" {
		t.Fatalf("preserved value=%q", value)
	}
	if version, err := database.SchemaVersion(context.Background()); err != nil || version != 24 {
		t.Fatalf("schema=(%d,%v), want 24", version, err)
	}
	legacyID, err := research.NewID("authority.legacy")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := database.Repositories().Research.TrustRegistry.GetProfile(context.Background(), legacyID)
	if err != nil || legacy.MinimumCorroboration != 1 || len(legacy.PreferredDomains) != 0 || len(legacy.AllowedSupplementaryKinds) != 0 {
		t.Fatalf("legacy authority profile after v24 = (%+v, %v)", legacy, err)
	}
}

func TestResearchSourceSnapshotEvidenceRepositoriesRoundTrip(t *testing.T) {
	database, _ := openTestDatabase(t)
	repositories := database.Repositories().Research
	ctx := context.Background()
	created := researchTestTimestamp(t, fixedTime)
	published := researchTestTimestamp(t, fixedTime.Add(-24*time.Hour))
	updated := researchTestTimestamp(t, fixedTime.Add(-time.Hour))
	version := researchTestVersion(t, "1.2")
	source := research.Source{ID: researchTestSourceID(t, "source.docs"), Kind: research.SourceOfficialDocumentation, Locator: researchTestLocator(t, "https://example.com/docs"), Version: &version, Metadata: research.SourceMetadata{Title: "Example docs", Publisher: "Example", Language: "en", PublishedAt: &published, UpdatedAt: &updated}, CreatedAt: created}
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Sources.Get(ctx, source.ID); err != nil || !reflect.DeepEqual(got, source) {
		t.Fatalf("source roundtrip=(%+v,%v), want %+v", got, err, source)
	}
	if got, err := repositories.Sources.FindByLocator(ctx, source.Locator); err != nil || got.ID != source.ID {
		t.Fatalf("locator lookup=(%+v,%v)", got, err)
	}
	if list, err := repositories.Sources.List(ctx); err != nil || len(list) != 1 || list[0].ID != source.ID {
		t.Fatalf("source list=(%+v,%v)", list, err)
	}

	snapshot1 := research.SourceSnapshot{ID: researchTestID(t, "snapshot.1"), SourceID: source.ID, Locator: source.Locator, FetchedAt: created, Fetch: research.FetchMetadata{StatusCode: 200, ContentType: "text/html", ETag: "etag-1", ContentHash: "sha256:first", ContentLength: 100, FetchVersion: "fetch/v1"}}
	snapshot2 := snapshot1
	snapshot2.ID = researchTestID(t, "snapshot.2")
	snapshot2.FetchedAt = researchTestTimestamp(t, fixedTime.Add(time.Minute))
	snapshot2.Fetch.ContentHash = "sha256:second"
	for _, item := range []research.SourceSnapshot{snapshot1, snapshot2} {
		if err := repositories.Snapshots.Append(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	if latest, err := repositories.Snapshots.LatestBySource(ctx, source.ID); err != nil || latest.ID != snapshot2.ID {
		t.Fatalf("latest snapshot=(%+v,%v)", latest, err)
	}
	if list, err := repositories.Snapshots.ListBySource(ctx, source.ID); err != nil || len(list) != 2 || list[0].ID != snapshot1.ID {
		t.Fatalf("snapshot list=(%+v,%v)", list, err)
	}

	evidence := research.Evidence{ID: researchTestID(t, "evidence.1"), SourceID: source.ID, SnapshotID: snapshot2.ID, Location: "section 2", Excerpt: "A bounded excerpt.", ExcerptHash: "sha256:excerpt", ExtractedAt: snapshot2.FetchedAt, ExtractorVersion: "extract/v1"}
	if err := repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Evidence.Get(ctx, evidence.ID); err != nil || !reflect.DeepEqual(got, evidence) {
		t.Fatalf("evidence roundtrip=(%+v,%v)", got, err)
	}
	if list, err := repositories.Evidence.ListBySnapshot(ctx, snapshot2.ID); err != nil || len(list) != 1 || list[0].ID != evidence.ID {
		t.Fatalf("evidence list=(%+v,%v)", list, err)
	}

	if err := repositories.Sources.Create(ctx, source); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate id error=%v", err)
	}
	duplicateLocator := source
	duplicateLocator.ID = researchTestSourceID(t, "source.other")
	if err := repositories.Sources.Create(ctx, duplicateLocator); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate locator error=%v", err)
	}
	mismatch := evidence
	mismatch.ID = researchTestID(t, "evidence.bad")
	mismatch.SourceID = researchTestSourceID(t, "source.other")
	if err := repositories.Evidence.Append(ctx, mismatch); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("mismatched evidence error=%v", err)
	}
}

func TestResearchRunRegistryAndIntelligenceRepositoriesRoundTrip(t *testing.T) {
	database, _ := openTestDatabase(t)
	repositories := database.Repositories().Research
	ctx := context.Background()
	at := researchTestTimestamp(t, fixedTime)
	topic, err := research.NewResearchTopic("HTTP caching", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	target := researchTestVersion(t, "1.24")
	request := research.ResearchRequest{ID: researchTestID(t, "request.1"), Topic: topic, Purpose: research.PurposeCurrentUsage, TargetVersion: &target, RequestedAt: at}
	run := research.ResearchRun{ID: researchTestID(t, "run.1"), RequestID: request.ID, Status: research.ResearchRunRunning, StartedAt: at}
	if err := repositories.Runs.Create(ctx, request, run); err != nil {
		t.Fatal(err)
	}
	run2 := run
	run2.ID = researchTestID(t, "run.2")
	if err := repositories.Runs.Create(ctx, request, run2); err != nil {
		t.Fatal(err)
	}
	completed := researchTestTimestamp(t, fixedTime.Add(time.Minute))
	run.Status = research.ResearchRunCompleted
	run.CompletedAt = &completed
	if err := repositories.Runs.UpdateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Runs.GetRequest(ctx, request.ID); err != nil || !reflect.DeepEqual(got, request) {
		t.Fatalf("request roundtrip=(%+v,%v)", got, err)
	}
	if got, err := repositories.Runs.GetRun(ctx, run.ID); err != nil || !reflect.DeepEqual(got, run) {
		t.Fatalf("run roundtrip=(%+v,%v)", got, err)
	}
	changed := request
	changed.Topic.Subject = "Different"
	if err := repositories.Runs.Create(ctx, changed, research.ResearchRun{ID: researchTestID(t, "run.3"), RequestID: request.ID, Status: research.ResearchRunPlanned, StartedAt: at}); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("changed request error=%v", err)
	}

	source := research.Source{ID: researchTestSourceID(t, "source.registry"), Kind: research.SourceSpecification, Locator: researchTestLocator(t, "https://example.com/spec"), Metadata: research.SourceMetadata{Title: "Specification"}, CreatedAt: at}
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	profile := research.AuthorityProfile{ID: researchTestID(t, "profile.1"), Version: "profile/v1", Domain: "software", TopicPattern: "HTTP*", PreferredKinds: []research.SourceKind{research.SourceSpecification, research.SourceOfficialDocumentation}, PreferredDomains: []string{"example.com", "*.example.org"}, PreferredOrganizations: []string{"Example Standards"}, MinimumCorroboration: 2, AllowedSupplementaryKinds: []research.SourceKind{research.SourceCommunityArticle}, MinimumTier: research.AuthorityTierA, CreatedAt: at}
	if err := repositories.TrustRegistry.SaveProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.TrustRegistry.GetProfile(ctx, profile.ID); err != nil || !reflect.DeepEqual(got, profile) {
		t.Fatalf("profile roundtrip=(%+v,%v)", got, err)
	}
	decision := research.TrustDecision{SourceID: source.ID, State: research.TrustAccepted, Tier: research.AuthorityTierA, Reasons: []research.TrustReason{{Code: "primary", Detail: "Normative source"}}, Policy: "trust/v1", EvaluatedAt: at}
	if err := repositories.TrustRegistry.SaveDecision(ctx, decision); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.TrustRegistry.LatestDecision(ctx, source.ID); err != nil || !reflect.DeepEqual(got, decision) {
		t.Fatalf("decision roundtrip=(%+v,%v)", got, err)
	}

	release := research.ReleaseRecord{ID: researchTestID(t, "release.1"), TechnologyID: researchTestID(t, "technology.go"), Version: target, Channel: research.ReleaseStable, Status: research.ReleaseCurrent, SourceIDs: []research.SourceID{source.ID}, ReleasedAt: &at, VerifiedAt: at}
	if err := repositories.Releases.Create(ctx, release); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Releases.Get(ctx, release.ID); err != nil || !reflect.DeepEqual(got, release) {
		t.Fatalf("release roundtrip=(%+v,%v)", got, err)
	}

	next := researchTestTimestamp(t, fixedTime.Add(time.Hour))
	score, _ := research.NewFreshnessScore(.8)
	freshness := application.FreshnessRecord{SubjectID: researchTestID(t, source.ID.String()), State: research.FreshnessFresh, Score: score, LastVerifiedAt: at, NextVerifyAt: &next, AlgorithmVersion: "freshness/v1"}
	if err := repositories.Freshness.Save(ctx, freshness); err != nil {
		t.Fatal(err)
	}
	if due, err := repositories.Freshness.ListDue(ctx, next); err != nil || len(due) != 1 || !reflect.DeepEqual(due[0], freshness) {
		t.Fatalf("freshness due=(%+v,%v)", due, err)
	}

	confidence, _ := research.NewClaimConfidence(.9)
	verification := research.VerificationResult{ID: researchTestID(t, "verification.1"), ClaimID: researchTestClaimID(t, "claim.1"), Status: research.VerificationVerified, SourceIDs: []research.SourceID{source.ID}, Confidence: confidence, VerifiedAt: at}
	if err := repositories.Verification.Append(ctx, verification); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Verification.LatestByClaim(ctx, verification.ClaimID); err != nil || !reflect.DeepEqual(got, verification) {
		t.Fatalf("verification roundtrip=(%+v,%v)", got, err)
	}

	drift := research.DriftReport{ID: researchTestID(t, "drift.1"), OldBundleID: researchTestID(t, "bundle.old"), Type: research.DriftSourceChanged, Severity: research.SeverityImportant, AffectedClaims: []research.ClaimID{verification.ClaimID}, OldEvidence: []research.ID{researchTestID(t, "evidence.old")}, NewEvidence: []research.ID{researchTestID(t, "evidence.new")}, DetectedAt: at}
	if err := repositories.Drift.Append(ctx, drift); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Drift.Get(ctx, drift.ID); err != nil || !reflect.DeepEqual(got, drift) {
		t.Fatalf("drift roundtrip=(%+v,%v)", got, err)
	}
	impact := research.ImpactReport{ID: researchTestID(t, "impact.1"), DriftReportID: drift.ID, AffectedBundleIDs: []research.ID{drift.OldBundleID}, AffectedClaimIDs: drift.AffectedClaims, Severity: research.SeverityImportant, RecommendedAction: research.ActionReviewCurriculum, AssessedAt: at}
	if err := repositories.Impact.Append(ctx, impact); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Impact.Get(ctx, impact.ID); err != nil || !reflect.DeepEqual(got, impact) {
		t.Fatalf("impact roundtrip=(%+v,%v)", got, err)
	}

	entry := application.CacheEntry{Key: "provider:query", Payload: []byte("bounded cache"), ContentHash: "sha256:cache", StoredAt: at, ExpiresAt: &next}
	if err := repositories.Cache.Put(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Cache.Get(ctx, entry.Key); err != nil || !reflect.DeepEqual(got, entry) {
		t.Fatalf("cache roundtrip=(%+v,%v)", got, err)
	}
	if err := repositories.Cache.Delete(ctx, entry.Key); err != nil {
		t.Fatal(err)
	}
	if _, err := repositories.Cache.Get(ctx, entry.Key); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("deleted cache error=%v", err)
	}
	entry.Key = "provider:oversized"
	entry.Payload = make([]byte, maximumResearchCachePayloadBytes+1)
	if err := repositories.Cache.Put(ctx, entry); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("oversized cache error=%v", err)
	}
}

func TestResearchSchemaEnforcesRelationshipsAndBoundedExcerpts(t *testing.T) {
	database, _ := openTestDatabase(t)
	ctx := context.Background()
	now := fixedTime.Format(timestampFormat)
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO source_snapshots (id,source_id,locator,fetched_at,status_code,content_type,content_length,fetch_version) VALUES ('snapshot.orphan','source.missing','https://example.com',?,200,'text/html',1,'fetch/v1')`, now); err == nil {
		t.Fatal("orphan snapshot was accepted")
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO sources (id,kind,locator,title,created_at) VALUES ('source.raw','other','https://example.com/raw','Raw',?)`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO source_snapshots (id,source_id,locator,fetched_at,status_code,content_type,content_length,fetch_version) VALUES ('snapshot.raw','source.raw','https://example.com/raw',?,200,'text/html',9000,'fetch/v1')`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO evidence (id,source_id,snapshot_id,location,excerpt,excerpt_hash,extracted_at,extractor_version) VALUES ('evidence.large','source.raw','snapshot.raw','body',?,'hash',?,'extract/v1')`, strings.Repeat("x", 8193), now); err == nil {
		t.Fatal("unbounded excerpt was accepted")
	}
}

func researchTestID(t *testing.T, value string) research.ID {
	t.Helper()
	id, err := research.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func researchTestSourceID(t *testing.T, value string) research.SourceID {
	t.Helper()
	id, err := research.NewSourceID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func researchTestClaimID(t *testing.T, value string) research.ClaimID {
	t.Helper()
	id, err := research.NewClaimID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func researchTestTimestamp(t *testing.T, value time.Time) research.Timestamp {
	t.Helper()
	timestamp, err := research.NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}
func researchTestLocator(t *testing.T, value string) research.SourceLocator {
	t.Helper()
	locator, err := research.NewSourceLocator(value)
	if err != nil {
		t.Fatal(err)
	}
	return locator
}
func researchTestVersion(t *testing.T, value string) research.SourceVersion {
	t.Helper()
	version, err := research.NewSourceVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}
