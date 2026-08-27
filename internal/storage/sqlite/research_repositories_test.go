package sqlite

import (
	"bytes"
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
	if _, err := handle.Exec(`INSERT INTO sources (id,kind,locator,title,created_at) VALUES ('source.legacy','other','https://example.test/legacy','Legacy source',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO sources (id,kind,locator,title,created_at) VALUES ('source.legacy-standard','standard','https://example.test/legacy-standard','Legacy standard',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO source_snapshots (id,source_id,locator,fetched_at,status_code,content_type,content_hash,content_length,fetch_version) VALUES ('snapshot.legacy','source.legacy','https://example.test/legacy',?,200,'text/plain','sha256:legacy',6,'fetch/v1')`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	excerptHash := research.CanonicalEvidenceExcerptHashV1("legacy")
	if _, err := handle.Exec(`INSERT INTO evidence (id,source_id,snapshot_id,location,excerpt,excerpt_hash,extracted_at,extractor_version) VALUES ('evidence.legacy','source.legacy','snapshot.legacy','line 1','legacy',?,?, 'extract/v1')`, excerptHash, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO claims (id,topic_subject,statement,claim_type,confidence,evidence_ids_json,created_at) VALUES ('claim.legacy','Legacy topic','Legacy claim','historical',0.5,'["evidence.legacy"]',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO source_conflicts (id,conflict_type,claim_ids_json,resolution,unresolved,detected_at) VALUES ('conflict.legacy','direct_contradiction','["claim.legacy","claim.other"]','',1,?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO verification_results (id,claim_id,status,source_ids_json,confidence,verified_at) VALUES ('verification.legacy','claim.legacy','verified_with_caveat','["source.legacy"]',0.5,?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO research_topics (request_id,subject,purpose,requested_at) VALUES ('request.legacy','Legacy topic','version_behavior',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO research_runs (id,request_id,status,started_at) VALUES ('run.legacy','request.legacy','running',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO source_bundles (id,run_id,topic_subject,purpose,state,verified_at) VALUES ('bundle.legacy','run.legacy','Legacy topic','version_behavior','ready',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO source_bundle_items (bundle_id,item_type,item_id,position) VALUES ('bundle.legacy','claim','claim.legacy',0),('bundle.legacy','source','source.legacy',0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO citations (id,source_id,snapshot_id,evidence_id,title,locator,deep_link_locator,snapshot_date,last_verified) VALUES ('citation.legacy','source.legacy','snapshot.legacy','evidence.legacy','Legacy source','https://example.test/legacy','https://example.test/legacy#section',?,?)`, fixedTime.Format(timestampFormat), fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO freshness_state (subject_id,state,score,last_verified_at,next_verify_at,algorithm_version) VALUES ('claim.legacy','aging',0.6,?,?,?)`, fixedTime.Format(timestampFormat), fixedTime.Add(time.Hour).Format(timestampFormat), research.FreshnessAlgorithmV1); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO deprecation_records (id,subject,status,source_ids_json,evidence_ids_json,verified_at) VALUES ('deprecation.legacy','Legacy API','deprecated','["source.legacy"]','["evidence.legacy"]',?)`, fixedTime.Format(timestampFormat)); err != nil {
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
	if version, err := database.SchemaVersion(context.Background()); err != nil || version != 36 {
		t.Fatalf("schema=(%d,%v), want 36", version, err)
	}
	legacyID, err := research.NewID("authority.legacy")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := database.Repositories().Research.TrustRegistry.GetProfile(context.Background(), legacyID)
	if err != nil || legacy.MinimumCorroboration != 1 || len(legacy.PreferredDomains) != 0 || len(legacy.AllowedSupplementaryKinds) != 0 || len(legacy.FreshnessTTLHints) != 0 {
		t.Fatalf("legacy authority profile after v29 = (%+v, %v)", legacy, err)
	}
	var contextBefore, contextAfter, scope, statusScope string
	if err := handle.QueryRow(`SELECT context_before,context_after FROM evidence WHERE id='evidence.legacy'`).Scan(&contextBefore, &contextAfter); err != nil {
		t.Fatal(err)
	}
	if err := handle.QueryRow(`SELECT scope,status_scope FROM claims WHERE id='claim.legacy'`).Scan(&scope, &statusScope); err != nil {
		t.Fatal(err)
	}
	if contextBefore != "" || contextAfter != "" || scope != "general" || statusScope != "all" {
		t.Fatalf("v26 defaults = contexts (%q,%q), scope %q, status %q", contextBefore, contextAfter, scope, statusScope)
	}
	var linkStrategy, section, deepLabel, algorithm string
	if err := handle.QueryRow(`SELECT link_strategy,section,deep_link_label,algorithm_version FROM citations WHERE id='citation.legacy'`).Scan(&linkStrategy, &section, &deepLabel, &algorithm); err != nil {
		t.Fatal(err)
	}
	if linkStrategy != string(research.CitationURLAnchor) || section != "unspecified" || deepLabel != section || algorithm != research.CitationAlgorithmV1 {
		t.Fatalf("v28 citation defaults = (%q,%q,%q,%q)", linkStrategy, section, deepLabel, algorithm)
	}
	if _, err := handle.Exec(`UPDATE evidence SET context_before=? WHERE id='evidence.legacy'`, strings.Repeat("x", research.MaximumEvidenceContextBytes+1)); err == nil {
		t.Fatal("v26 accepted oversized evidence context")
	}
	if _, err := handle.Exec(`UPDATE claims SET status_scope='sometimes' WHERE id='claim.legacy'`); err == nil {
		t.Fatal("v26 accepted invalid claim status scope")
	}
	if _, err := handle.Exec(`UPDATE authority_profiles SET freshness_ttl_hints_json='{}' WHERE id='authority.legacy'`); err == nil {
		t.Fatal("v29 accepted non-array freshness TTL hints")
	}
	var verificationReason, verificationPriority, schedulingAlgorithm string
	if err := handle.QueryRow(`SELECT json_extract(scheduling_json,'$.verification_reason'),json_extract(scheduling_json,'$.priority'),json_extract(scheduling_json,'$.algorithm_version') FROM freshness_state WHERE subject_id='claim.legacy'`).Scan(&verificationReason, &verificationPriority, &schedulingAlgorithm); err != nil {
		t.Fatal(err)
	}
	if verificationReason != string(research.VerificationTTLExpired) || verificationPriority != string(research.VerificationPriorityNormal) || schedulingAlgorithm != research.RefreshSchedulingAlgorithmV1 {
		t.Fatalf("v30 schedule defaults = (%q,%q,%q)", verificationReason, verificationPriority, schedulingAlgorithm)
	}
	if _, err := handle.Exec(`UPDATE freshness_state SET scheduling_json='{}' WHERE subject_id='claim.legacy'`); err == nil {
		t.Fatal("v30 accepted incomplete scheduling metadata JSON")
	}
	legacyDeprecationID, _ := research.NewID("deprecation.legacy")
	legacyDeprecation, err := database.Repositories().Research.Deprecations.Get(context.Background(), legacyDeprecationID)
	if err != nil || legacyDeprecation.Determination != research.DeprecationLegacyUnclassified ||
		legacyDeprecation.AlgorithmVersion != research.DeprecationLegacyAlgorithm {
		t.Fatalf("v31 legacy deprecation = (%+v, %v)", legacyDeprecation, err)
	}
	if _, err := handle.Exec(`UPDATE deprecation_records SET determination='absence_from_docs' WHERE id='deprecation.legacy'`); err == nil {
		t.Fatal("v31 accepted invalid deprecation determination")
	}
	if _, err := handle.Exec(`UPDATE deprecation_records SET algorithm_version='deprecation-intelligence-v1' WHERE id='deprecation.legacy'`); err == nil {
		t.Fatal("v31 accepted a v1 algorithm with legacy-unclassified determination")
	}
	if _, err := handle.Exec(`INSERT INTO deprecation_records (id,subject,status,source_ids_json,evidence_ids_json,verified_at,determination,algorithm_version) VALUES ('deprecation.false-multi','API','deprecated','["source.legacy"]','["evidence.legacy"]',?,'multi_source_strong_inference','deprecation-intelligence-v1')`, fixedTime.Format(timestampFormat)); err == nil {
		t.Fatal("v31 accepted single-source multi-source inference")
	}
	legacySourceID, _ := research.NewSourceID("source.legacy")
	legacySource, err := database.Repositories().Research.Sources.Get(context.Background(), legacySourceID)
	if err != nil || legacySource.TemporalScope != research.SourceTemporalCurrent {
		t.Fatalf("v32 legacy source scope = (%+v, %v)", legacySource, err)
	}
	legacyCitationID, _ := research.NewID("citation.legacy")
	legacyCitation, err := database.Repositories().Research.Citations.Get(context.Background(), legacyCitationID)
	if err != nil || legacyCitation.TemporalScope != research.SourceTemporalCurrent || legacyCitation.TemporalWarning != "" ||
		legacyCitation.TemporalAlgorithmVersion != research.SourceTemporalLegacyCurrent {
		t.Fatalf("v32 legacy citation scope = (%+v, %v)", legacyCitation, err)
	}
	var claimItemScope, sourceItemScope sql.NullString
	if err := handle.QueryRow(`SELECT temporal_scope FROM source_bundle_items WHERE bundle_id='bundle.legacy' AND item_type='claim'`).Scan(&claimItemScope); err != nil {
		t.Fatal(err)
	}
	if err := handle.QueryRow(`SELECT temporal_scope FROM source_bundle_items WHERE bundle_id='bundle.legacy' AND item_type='source'`).Scan(&sourceItemScope); err != nil {
		t.Fatal(err)
	}
	if claimItemScope.Valid || !sourceItemScope.Valid || sourceItemScope.String != string(research.SourceTemporalCurrent) {
		t.Fatalf("v32 bundle item scopes = claim %+v, source %+v", claimItemScope, sourceItemScope)
	}
	if _, err := handle.Exec(`UPDATE sources SET temporal_scope='version_bound' WHERE id='source.legacy'`); err == nil {
		t.Fatal("v32 accepted version-bound source without version")
	}
	if _, err := handle.Exec(`UPDATE citations SET temporal_scope='historical' WHERE id='citation.legacy'`); err == nil {
		t.Fatal("v32 accepted historical citation without warning")
	}
	if _, err := handle.Exec(`UPDATE source_bundle_items SET temporal_scope=NULL WHERE bundle_id='bundle.legacy' AND item_type='source'`); err == nil {
		t.Fatal("v32 accepted source bundle item without temporal scope")
	}
	legacyConflictID, _ := research.NewID("conflict.legacy")
	legacyConflict, err := database.Repositories().Research.Conflicts.Get(context.Background(), legacyConflictID)
	if err != nil || legacyConflict.AlgorithmVersion != research.ConflictLegacyAlgorithm ||
		legacyConflict.Reason == "" || !legacyConflict.Unresolved || legacyConflict.WinningClaimID != nil {
		t.Fatalf("v33 legacy conflict = (%+v, %v)", legacyConflict, err)
	}
	if _, err := handle.Exec(`UPDATE source_conflicts SET winning_claim_id='claim.legacy' WHERE id='conflict.legacy'`); err == nil {
		t.Fatal("v33 accepted incomplete winning conflict identity")
	}
	if _, err := handle.Exec(`INSERT INTO source_conflicts (id,conflict_type,claim_ids_json,resolution,unresolved,detected_at,confidence,reason,algorithm_version) VALUES ('conflict.too-many','direct_contradiction','["claim.one","claim.two","claim.three"]','',1,?,0.5,'Needs review.','conflict-resolver-v1')`, fixedTime.Format(timestampFormat)); err == nil {
		t.Fatal("v33 accepted more than two claims for resolver v1")
	}
	legacyVerificationID, _ := research.NewID("verification.legacy")
	legacyVerification, err := database.Repositories().Research.Verification.Get(context.Background(), legacyVerificationID)
	if err != nil || legacyVerification.AlgorithmVersion != research.VerificationLegacyAlgorithm ||
		legacyVerification.Requirement != research.VerificationRequirementLegacy ||
		len(legacyVerification.ReasonCodes) != 1 ||
		legacyVerification.ReasonCodes[0] != research.VerificationReasonLegacyUnclassified ||
		legacyVerification.Metrics != (research.VerificationMetrics{}) {
		t.Fatalf("v34 legacy verification = (%+v, %v)", legacyVerification, err)
	}
	if _, err := handle.Exec(`UPDATE verification_results SET algorithm_version='multi-source-verification-v1' WHERE id='verification.legacy'`); err == nil {
		t.Fatal("v34 accepted v1 algorithm with legacy metrics")
	}
	if _, err := handle.Exec(`INSERT INTO verification_results (id,claim_id,status,source_ids_json,confidence,verified_at,requirement,source_count,independent_organization_count,authority_distribution_json,scope_consistent,reason_codes_json,algorithm_version) VALUES ('verification.bad-metrics','claim.legacy','verified','["source.legacy"]',0.8,?,'general_support',1,1,'{"tier_a":0,"tier_b":0,"tier_c":0,"tier_d":0,"tier_e":0,"unknown":0}',1,'["independent_support"]','multi-source-verification-v1')`, fixedTime.Format(timestampFormat)); err == nil {
		t.Fatal("v34 accepted authority distribution that does not total source count")
	}
	legacyBundleID, _ := research.NewID("bundle.legacy")
	legacyBundle, err := database.Repositories().Research.Bundles.Get(context.Background(), legacyBundleID)
	if err != nil || legacyBundle.AlgorithmVersion != research.SourceBundleLegacyAlgorithm ||
		legacyBundle.ContentHash != "" || legacyBundle.Freshness.State != research.FreshnessUnknown ||
		len(legacyBundle.Sources) != 1 || legacyBundle.Sources[0].Role != research.BundleSourceUnclassified {
		t.Fatalf("v35 legacy source bundle = (%+v, %v)", legacyBundle, err)
	}
	if _, err := handle.Exec(`UPDATE source_bundles SET algorithm_version='source-bundle-v1' WHERE id='bundle.legacy'`); err == nil {
		t.Fatal("v35 accepted v1 source bundle without canonical JSON and hash")
	}
	if _, err := handle.Exec(`UPDATE source_bundle_items SET source_role='historical' WHERE bundle_id='bundle.legacy' AND item_type='source'`); err == nil {
		t.Fatal("v35 accepted historical role for a current source item")
	}
	var specializedKind sql.NullString
	var specializedJSON string
	if err := handle.QueryRow(`SELECT specialized_kind,specialized_metadata_json FROM sources WHERE id='source.legacy'`).Scan(&specializedKind, &specializedJSON); err != nil {
		t.Fatal(err)
	}
	if specializedKind.Valid || specializedJSON != "" || legacySource.Specialization != nil {
		t.Fatalf("v36 invented legacy specialized source metadata: kind=%+v JSON=%q source=%+v", specializedKind, specializedJSON, legacySource)
	}
	legacyStandardID, _ := research.NewSourceID("source.legacy-standard")
	legacyStandard, err := database.Repositories().Research.Sources.Get(context.Background(), legacyStandardID)
	if err != nil || legacyStandard.Kind != research.SourceStandard || legacyStandard.Specialization != nil {
		t.Fatalf("v36 legacy standard = (%+v, %v)", legacyStandard, err)
	}
	if _, err := handle.Exec(`UPDATE sources SET specialized_kind='playground' WHERE id='source.legacy'`); err == nil {
		t.Fatal("v36 accepted specialized kind without bounded metadata")
	}
	if _, err := handle.Exec(`INSERT INTO sources (id,kind,locator,title,created_at) VALUES ('source.raw-playground','playground','https://example.test/playground','Playground',?)`, fixedTime.Format(timestampFormat)); err == nil {
		t.Fatal("v36 bypassed the specialized playground storage projection")
	}
}

func TestSpecializedTechnicalSourcesRoundTripSQLite(t *testing.T) {
	database, _ := openTestDatabase(t)
	repository := database.Repositories().Research.Sources
	ctx := context.Background()
	created := researchTestTimestamp(t, fixedTime)

	playgroundVersion := researchTestVersion(t, "python-3.14")
	playgroundLocator := researchTestLocator(t, "https://play.example.test/python")
	playgroundShare := researchTestLocator(t, "https://play.example.test/python/share/abc")
	playground := research.Source{
		ID: researchTestSourceID(t, "source.playground"), Kind: research.SourcePlayground,
		Locator: playgroundLocator, Version: &playgroundVersion, TemporalScope: research.SourceTemporalVersionBound,
		Metadata: research.SourceMetadata{Title: "Python playground"}, CreatedAt: created,
		Specialization: &research.SourceSpecialization{
			Kind: research.SourcePlayground, AlgorithmVersion: research.SpecializedSourceMetadataV1,
			Playground: &research.PlaygroundDetails{
				Interactive: true, LanguageRuntime: "Python runtime", Version: &playgroundVersion,
				Affiliation: research.SourceAffiliationOfficial, ShareableLocator: playgroundShare,
			},
		},
	}
	packageVersion := researchTestVersion(t, "8.1")
	packageLocator := researchTestLocator(t, "https://packages.example.test/portable-client")
	packageCode := researchTestLocator(t, "https://code.example.test/portable-client")
	packageReference := research.Source{
		ID: researchTestSourceID(t, "source.package"), Kind: research.SourcePackageReference,
		Locator: packageLocator, Version: &packageVersion, TemporalScope: research.SourceTemporalVersionBound,
		Metadata: research.SourceMetadata{Title: "Portable client API"}, CreatedAt: created,
		Specialization: &research.SourceSpecialization{
			Kind: research.SourcePackageReference, AlgorithmVersion: research.SpecializedSourceMetadataV1,
			PackageReference: &research.PackageReferenceDetails{
				PackageModule: "portable-client", Symbol: "Client.Connect", Version: &packageVersion,
				CanonicalDocsLocator: packageLocator, SourceCodeLocator: &packageCode,
			},
		},
	}
	revision := researchTestVersion(t, "2022")
	standardLocator := researchTestLocator(t, "https://standards.example.test/rfc-9110")
	standard := research.Source{
		ID: researchTestSourceID(t, "source.standard"), Kind: research.SourceStandard,
		Locator: standardLocator, Version: &revision, TemporalScope: research.SourceTemporalCurrent,
		Metadata: research.SourceMetadata{Title: "HTTP Semantics"}, CreatedAt: created,
		Specialization: &research.SourceSpecialization{
			Kind: research.SourceStandard, AlgorithmVersion: research.SpecializedSourceMetadataV1,
			Standard: &research.StandardDetails{
				StandardsBody: "IETF", StandardID: "RFC 9110", Revision: &revision,
				Status: research.StandardActive, OfficialLocator: standardLocator,
			},
		},
	}

	for _, source := range []research.Source{playground, packageReference, standard} {
		if err := repository.Create(ctx, source); err != nil {
			t.Fatalf("Create(%s) error = %v", source.ID, err)
		}
		got, err := repository.Get(ctx, source.ID)
		if err != nil || !reflect.DeepEqual(got, source) {
			t.Fatalf("specialized source roundtrip %s = (%+v, %v), want %+v", source.ID, got, err, source)
		}
	}

	var storedKind, specializedKind, encoded string
	if err := database.sql.QueryRowContext(ctx, `SELECT kind,specialized_kind,specialized_metadata_json FROM sources WHERE id=?`, playground.ID.String()).Scan(&storedKind, &specializedKind, &encoded); err != nil {
		t.Fatal(err)
	}
	if storedKind != string(research.SourceOther) || specializedKind != string(research.SourcePlayground) || !strings.Contains(encoded, research.SpecializedSourceMetadataV1) {
		t.Fatalf("playground storage projection = (%q,%q,%q)", storedKind, specializedKind, encoded)
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO sources (id,kind,locator,title,created_at,temporal_scope,specialized_kind,specialized_metadata_json) VALUES ('source.invalid-specialized','other','https://invalid.example.test/playground','Invalid',?,'current','playground','{}')`, timestampText(created)); err != nil {
		t.Fatal(err)
	}
	invalidID, _ := research.NewSourceID("source.invalid-specialized")
	if _, err := repository.Get(ctx, invalidID); !errors.Is(err, application.ErrPersistenceFailure) {
		t.Fatalf("invalid specialized metadata read error = %v, want persistence_failure", err)
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
	source := research.Source{ID: researchTestSourceID(t, "source.docs"), Kind: research.SourceOfficialDocumentation, Locator: researchTestLocator(t, "https://example.com/docs"), Version: &version, TemporalScope: research.SourceTemporalCurrent, Metadata: research.SourceMetadata{Title: "Example docs", Publisher: "Example", Language: "en", PublishedAt: &published, UpdatedAt: &updated}, CreatedAt: created}
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

	evidence := research.Evidence{
		ID: researchTestID(t, "evidence.1"), SourceID: source.ID, SnapshotID: snapshot2.ID,
		Location: "section 2", Excerpt: "A bounded excerpt.",
		ExcerptHash:   research.CanonicalEvidenceExcerptHashV1("A bounded excerpt."),
		ContextBefore: "The preceding bounded context.", ContextAfter: "The following bounded context.",
		ExtractedAt: snapshot2.FetchedAt, ExtractorVersion: "extract/v1",
	}
	if err := repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Evidence.Get(ctx, evidence.ID); err != nil || !reflect.DeepEqual(got, evidence) {
		t.Fatalf("evidence roundtrip=(%+v,%v)", got, err)
	}
	if list, err := repositories.Evidence.ListBySnapshot(ctx, snapshot2.ID); err != nil || len(list) != 1 || list[0].ID != evidence.ID {
		t.Fatalf("evidence list=(%+v,%v)", list, err)
	}
	topic, err := research.NewResearchTopic("Example API", "software", "Example")
	if err != nil {
		t.Fatal(err)
	}
	confidence, err := research.NewClaimConfidence(.8)
	if err != nil {
		t.Fatal(err)
	}
	claim := research.Claim{
		ID: researchTestClaimID(t, "claim.release"), Topic: topic, Statement: "The API changed.",
		Type: research.ClaimVersionChange, Scope: "release notes", VersionScope: &version,
		StatusScope: research.ClaimStatusStable, Confidence: confidence,
		SourceIDs: []research.SourceID{source.ID}, EvidenceIDs: []research.ID{evidence.ID}, CreatedAt: snapshot2.FetchedAt,
	}
	if err := repositories.Claims.Append(ctx, claim); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Claims.Get(ctx, claim.ID); err != nil || !reflect.DeepEqual(got, claim) {
		t.Fatalf("claim roundtrip=(%+v,%v), want %+v", got, err, claim)
	}
	rollbackEvidence := evidence
	rollbackEvidence.ID = researchTestID(t, "evidence.rollback")
	rollbackClaim := claim
	rollbackClaim.ID = researchTestClaimID(t, "claim.rollback")
	rollbackClaim.EvidenceIDs = []research.ID{researchTestID(t, "evidence.missing")}
	if err := repositories.ReleaseIngestion.Commit(ctx, application.ReleaseIngestionBatch{
		Evidence: []research.Evidence{rollbackEvidence}, Claims: []research.Claim{rollbackClaim},
	}); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("release ingestion rollback error=%v, want not found", err)
	}
	if _, err := repositories.Evidence.Get(ctx, rollbackEvidence.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("release ingestion retained partial evidence: %v", err)
	}
	deepLocator := researchTestLocator(t, "https://example.com/docs#section-2")
	citation := research.Citation{
		ID: researchTestID(t, "citation.1"), SourceID: source.ID, SnapshotID: snapshot2.ID,
		EvidenceID: evidence.ID, Title: source.Metadata.Title, Locator: source.Locator,
		DeepLink:     &research.DeepLink{Locator: deepLocator, Label: "Section 2"},
		LinkStrategy: research.CitationURLAnchor, Section: "Guide > Section 2",
		SnapshotDate: snapshot2.FetchedAt, VersionScope: &version,
		TemporalScope: source.TemporalScope, LastVerified: snapshot2.FetchedAt,
		AlgorithmVersion:         research.CitationAlgorithmV1,
		TemporalAlgorithmVersion: research.SourceTemporalPolicyV1,
	}
	if err := repositories.Citations.Append(ctx, citation); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Citations.Get(ctx, citation.ID); err != nil || !reflect.DeepEqual(got, citation) {
		t.Fatalf("citation roundtrip=(%+v,%v), want %+v", got, err, citation)
	}
	if list, err := repositories.Citations.ListByEvidence(ctx, evidence.ID); err != nil || len(list) != 1 || list[0].ID != citation.ID {
		t.Fatalf("citation list=(%+v,%v)", list, err)
	}
	if err := repositories.Citations.Append(ctx, citation); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate citation error=%v", err)
	}
	if err := repositories.Sources.SetTemporalScope(ctx, source.ID, research.SourceTemporalArchived); err != nil {
		t.Fatalf("set archived source scope: %v", err)
	}
	if got, err := repositories.Sources.Get(ctx, source.ID); err != nil || got.TemporalScope != research.SourceTemporalArchived {
		t.Fatalf("archived source=(%+v,%v)", got, err)
	}
	archivedCitation := citation
	archivedCitation.ID = researchTestID(t, "citation.archived")
	archivedCitation.TemporalScope = research.SourceTemporalArchived
	archivedCitation.TemporalWarning, err = research.SourceTemporalArchived.Warning(&version)
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.Citations.Append(ctx, archivedCitation); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Citations.Get(ctx, archivedCitation.ID); err != nil || !reflect.DeepEqual(got, archivedCitation) {
		t.Fatalf("archived citation roundtrip=(%+v,%v), want %+v", got, err, archivedCitation)
	}
	if got, err := repositories.Citations.Get(ctx, citation.ID); err != nil || got.TemporalScope != research.SourceTemporalCurrent {
		t.Fatalf("source reclassification rewrote prior citation=(%+v,%v)", got, err)
	}
	if err := repositories.Sources.SetTemporalScope(ctx, source.ID, research.SourceTemporalVersionBound); err != nil {
		t.Fatalf("set version-bound source scope: %v", err)
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
	target := researchTestVersion(t, "1.24.0")
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

	source := research.Source{ID: researchTestSourceID(t, "source.registry"), Kind: research.SourceSpecification, Locator: researchTestLocator(t, "https://example.com/spec"), TemporalScope: research.SourceTemporalCurrent, Metadata: research.SourceMetadata{Title: "Specification"}, CreatedAt: at}
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	securityClaim := research.ClaimSecurity
	releaseNotesKind := research.SourceReleaseNotes
	profile := research.AuthorityProfile{ID: researchTestID(t, "profile.1"), Version: "profile/v1", Domain: "software", TopicPattern: "HTTP*", PreferredKinds: []research.SourceKind{research.SourceSpecification, research.SourceOfficialDocumentation}, PreferredDomains: []string{"example.com", "*.example.org"}, PreferredOrganizations: []string{"Example Standards"}, MinimumCorroboration: 2, AllowedSupplementaryKinds: []research.SourceKind{research.SourceCommunityArticle}, FreshnessTTLHints: []research.FreshnessTTLHint{{ClaimType: &securityClaim, TTLDays: 14}, {SourceKind: &releaseNotesKind, TTLDays: 21}}, MinimumTier: research.AuthorityTierA, CreatedAt: at}
	if err := repositories.TrustRegistry.SaveProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.TrustRegistry.GetProfile(ctx, profile.ID); err != nil || !reflect.DeepEqual(got, profile) {
		t.Fatalf("profile roundtrip=(%+v,%v)", got, err)
	}
	registryEntry := researchTestRegistryEntry(t, "registry.docs", "Docs.Example.COM.", research.RegistryTrusted, at)
	if err := repositories.SourceRegistry.Save(ctx, registryEntry); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.SourceRegistry.Get(ctx, registryEntry.ID); err != nil || !reflect.DeepEqual(got, registryEntry) {
		t.Fatalf("source registry roundtrip=(%+v,%v), want %+v", got, err, registryEntry)
	}
	registryEntry.Status = research.RegistryHistorical
	if err := repositories.SourceRegistry.Save(ctx, registryEntry); err != nil {
		t.Fatalf("source registry update: %v", err)
	}
	if listed, err := repositories.SourceRegistry.List(ctx); err != nil || len(listed) != 1 || listed[0].Status != research.RegistryHistorical {
		t.Fatalf("source registry list=(%+v,%v)", listed, err)
	}
	duplicateDomain := researchTestRegistryEntry(t, "registry.other", "docs.example.com", research.RegistryConditional, at)
	if err := repositories.SourceRegistry.Save(ctx, duplicateDomain); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate registry domain error=%v, want conflict", err)
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
	} else if got.Version.Scheme() != research.VersionSemantic {
		t.Fatalf("release version scheme=%q, want semantic", got.Version.Scheme())
	}
	release.Status = research.ReleaseSuperseded
	if err := repositories.Releases.Update(ctx, release); err != nil {
		t.Fatalf("update release status: %v", err)
	}
	if got, err := repositories.Releases.Get(ctx, release.ID); err != nil || got.Status != research.ReleaseSuperseded {
		t.Fatalf("updated release=(%+v,%v)", got, err)
	}
	deprecationSnapshot := research.SourceSnapshot{
		ID: researchTestID(t, "snapshot.deprecation"), SourceID: source.ID, Locator: source.Locator,
		FetchedAt: at, Fetch: research.FetchMetadata{StatusCode: 200, ContentType: "text/html", ContentHash: "sha256:deprecation", ContentLength: 64, FetchVersion: "fetch/v1"},
	}
	if err := repositories.Snapshots.Append(ctx, deprecationSnapshot); err != nil {
		t.Fatal(err)
	}
	deprecationExcerpt := "Old API is deprecated."
	deprecationEvidence := research.Evidence{
		ID: researchTestID(t, "evidence.deprecation"), SourceID: source.ID, SnapshotID: deprecationSnapshot.ID,
		Location: "deprecations", Excerpt: deprecationExcerpt,
		ExcerptHash: research.CanonicalEvidenceExcerptHashV1(deprecationExcerpt), ExtractedAt: at, ExtractorVersion: "extract/v1",
	}
	if err := repositories.Evidence.Append(ctx, deprecationEvidence); err != nil {
		t.Fatal(err)
	}
	deprecatedIn := researchTestVersion(t, "2.0.0")
	deprecation := research.DeprecationRecord{
		ID: researchTestID(t, "deprecation.api"), Subject: "Old API", Status: research.DeprecationDeprecated,
		Determination: research.DeprecationExplicitEvidence, DeprecatedIn: &deprecatedIn,
		SourceIDs: []research.SourceID{source.ID}, EvidenceIDs: []research.ID{deprecationEvidence.ID}, VerifiedAt: at,
		AlgorithmVersion: research.DeprecationIntelligenceAlgorithmV1,
	}
	if err := repositories.Deprecations.Append(ctx, deprecation); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Deprecations.Get(ctx, deprecation.ID); err != nil || !reflect.DeepEqual(got, deprecation) {
		t.Fatalf("deprecation roundtrip=(%+v,%v), want %+v", got, err, deprecation)
	}
	if history, err := repositories.Deprecations.ListBySubject(ctx, deprecation.Subject); err != nil || len(history) != 1 || !reflect.DeepEqual(history[0], deprecation) {
		t.Fatalf("deprecation history=(%+v,%v)", history, err)
	}

	next := researchTestTimestamp(t, fixedTime.Add(time.Hour))
	score, _ := research.NewFreshnessScore(.8)
	freshness := application.FreshnessRecord{
		SubjectID: researchTestID(t, source.ID.String()), State: research.FreshnessFresh, Score: score,
		LastVerifiedAt: at, NextVerifyAt: &next, VerificationReason: research.VerificationTTLExpired,
		Priority: research.VerificationPriorityNormal, AlgorithmVersion: research.FreshnessAlgorithmV1,
		SchedulingAlgorithmVersion: research.RefreshSchedulingAlgorithmV1,
	}
	if err := repositories.Freshness.Save(ctx, freshness); err != nil {
		t.Fatal(err)
	}
	critical := freshness
	critical.SubjectID = researchTestID(t, "claim.critical")
	critical.VerificationReason = research.VerificationManualRequest
	critical.Priority = research.VerificationPriorityCritical
	if err := repositories.Freshness.Save(ctx, critical); err != nil {
		t.Fatal(err)
	}
	futureAt := researchTestTimestamp(t, next.Time().Add(time.Hour))
	future := freshness
	future.SubjectID = researchTestID(t, "claim.future")
	future.NextVerifyAt = &futureAt
	if err := repositories.Freshness.Save(ctx, future); err != nil {
		t.Fatal(err)
	}
	if due, err := repositories.Freshness.ListDue(ctx, next); err != nil || len(due) != 2 || !reflect.DeepEqual(due[0], critical) || !reflect.DeepEqual(due[1], freshness) {
		t.Fatalf("freshness due=(%+v,%v)", due, err)
	}

	confidence, _ := research.NewClaimConfidence(.9)
	claimOne := research.Claim{
		ID: researchTestClaimID(t, "claim.conflict.one"), Topic: topic, Statement: "The feature is required.",
		Type: research.ClaimRequirement, Scope: "HTTP caching", StatusScope: research.ClaimStatusStable,
		Confidence: confidence, SourceIDs: []research.SourceID{source.ID},
		EvidenceIDs: []research.ID{deprecationEvidence.ID}, CreatedAt: at,
	}
	claimTwo := claimOne
	claimTwo.ID = researchTestClaimID(t, "claim.conflict.two")
	claimTwo.Statement = "The feature is forbidden."
	for _, claim := range []research.Claim{claimOne, claimTwo} {
		if err := repositories.Claims.Append(ctx, claim); err != nil {
			t.Fatal(err)
		}
	}
	verification := research.VerificationResult{
		ID: researchTestID(t, "verification.1"), ClaimID: claimOne.ID,
		Status: research.VerificationVerified, Requirement: research.VerificationRequirementNormativePrimary,
		SourceIDs: []research.SourceID{source.ID},
		Metrics: research.VerificationMetrics{
			SourceCount: 1, IndependentOrganizationCount: 0, ScopeConsistent: true,
			AuthorityDistribution: research.VerificationAuthorityDistribution{TierA: 1},
		},
		ReasonCodes: []research.ClaimVerificationReason{research.VerificationReasonPrimarySourceSufficient},
		Confidence:  confidence, VerifiedAt: at, AlgorithmVersion: research.MultiSourceVerificationAlgorithmV1,
	}
	if err := repositories.Verification.Append(ctx, verification); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Verification.LatestByClaim(ctx, verification.ClaimID); err != nil || !reflect.DeepEqual(got, verification) {
		t.Fatalf("verification roundtrip=(%+v,%v)", got, err)
	}
	winnerClaim, winnerSource := claimOne.ID, source.ID
	conflict := research.Conflict{
		ID: researchTestID(t, "conflict.1"), Type: research.ConflictAuthorityMismatch,
		ClaimIDs:   []research.ClaimID{claimOne.ID, claimTwo.ID},
		Resolution: "Prefer the normative source within this scope.", Confidence: confidence,
		Reason:         "The winning source is authoritative for this requirement.",
		WinningClaimID: &winnerClaim, WinningSourceID: &winnerSource, WinningScope: claimOne.Scope,
		DetectedAt: at, AlgorithmVersion: research.ConflictResolverAlgorithmV1,
	}
	if err := repositories.Conflicts.Append(ctx, conflict); err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Conflicts.Get(ctx, conflict.ID); err != nil || !reflect.DeepEqual(got, conflict) {
		t.Fatalf("conflict roundtrip=(%+v,%v), want %+v", got, err, conflict)
	}
	if list, err := repositories.Conflicts.ListByClaim(ctx, claimTwo.ID); err != nil || len(list) != 1 || !reflect.DeepEqual(list[0], conflict) {
		t.Fatalf("conflicts by claim=(%+v,%v)", list, err)
	}
	bundleVerifiedAt := researchTestTimestamp(t, fixedTime.Add(2*time.Minute))
	bundle, err := research.SealSourceBundleV1(research.SourceBundle{
		ID: researchTestID(t, "bundle.sqlite.1"), RunID: run.ID, Topic: topic,
		Purpose: research.PurposeCurrentUsage, TargetVersion: &target,
		ClaimIDs: []research.ClaimID{claimOne.ID},
		Sources: []research.SourceBundleSource{{
			SourceID: source.ID, Role: research.BundleSourcePrimary, TemporalScope: research.SourceTemporalCurrent,
		}},
		ConflictIDs: []research.ID{conflict.ID},
		Freshness: research.SourceBundleFreshness{
			State: research.FreshnessFresh, Score: score, LastVerifiedAt: &at,
			SourceAlgorithms: []string{research.FreshnessAlgorithmV1}, AlgorithmVersion: research.SourceBundleFreshnessV1,
		},
		Issues: []research.SourceBundleIssue{research.BundleIssueResolvedConflict}, VerifiedAt: bundleVerifiedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.Bundles.Append(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	wantBundleJSON, err := bundle.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := repositories.Bundles.Get(ctx, bundle.ID); err != nil || !sourceBundleJSONEqual(got, wantBundleJSON) {
		t.Fatalf("source bundle roundtrip=(%+v,%v), want %+v", got, err, bundle)
	}
	if listed, err := repositories.Bundles.ListByRun(ctx, run.ID); err != nil || len(listed) != 1 || !sourceBundleJSONEqual(listed[0], wantBundleJSON) {
		t.Fatalf("source bundles by run=(%+v,%v)", listed, err)
	}
	if err := repositories.Bundles.Append(ctx, bundle); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate source bundle error=%v, want conflict", err)
	}
	if _, err := database.sql.ExecContext(ctx, `UPDATE source_bundles SET summary='changed' WHERE id=?`, bundle.ID.String()); err == nil {
		t.Fatal("v35 allowed mutation of an immutable source bundle")
	}
	if _, err := database.sql.ExecContext(ctx, `UPDATE source_bundle_items SET source_role='supporting' WHERE bundle_id=? AND item_type='source'`, bundle.ID.String()); err == nil {
		t.Fatal("v35 allowed mutation of an immutable source bundle item")
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

func sourceBundleJSONEqual(bundle research.SourceBundle, want []byte) bool {
	got, err := bundle.ExportJSON()
	return err == nil && bytes.Equal(got, want)
}

func researchTestRegistryEntry(t *testing.T, idValue, domainValue string, status research.RegistryStatus, at research.Timestamp) research.SourceRegistryEntry {
	t.Helper()
	domain, err := research.NewCanonicalDomain(domainValue)
	if err != nil {
		t.Fatal(err)
	}
	return research.SourceRegistryEntry{
		ID: researchTestID(t, idValue), Organization: "Example Docs", CanonicalDomains: []research.CanonicalDomain{domain},
		SourceKinds: []research.SourceKind{research.SourceOfficialDocumentation, research.SourceReleaseNotes},
		AuthorityHints: []research.RegistryAuthorityHint{
			{SourceKind: research.SourceOfficialDocumentation, Tier: research.AuthorityTierB, Reason: "Official supporting documentation."},
			{SourceKind: research.SourceReleaseNotes, Tier: research.AuthorityTierA, Reason: "Primary release history."},
		},
		ResearchDomains: []string{"software"}, TopicPatterns: []string{"*"}, Notes: "Registry fixture.",
		Status: status, AddedAt: at, LastReviewedAt: at,
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
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO evidence (id,source_id,snapshot_id,location,excerpt,excerpt_hash,extracted_at,extractor_version) VALUES ('evidence.large','source.raw','snapshot.raw','body',?,'hash',?,'extract/v1')`, strings.Repeat("x", research.MaximumEvidenceExcerptBytes+1), now); err == nil {
		t.Fatal("unbounded excerpt was accepted")
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO evidence (id,source_id,snapshot_id,location,excerpt,excerpt_hash,extracted_at,extractor_version) VALUES ('evidence.raw','source.raw','snapshot.raw','body','bounded','hash',?,'extract/v1')`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO citations (id,source_id,snapshot_id,evidence_id,title,locator,snapshot_date,last_verified,link_strategy,section,algorithm_version) VALUES ('citation.bad','source.raw','snapshot.raw','evidence.raw','Bad','https://example.com/raw',?,?,'canonical_fallback',?,'citation-v1')`, now, now, strings.Repeat("x", research.MaximumCitationSectionBytes+1)); err == nil {
		t.Fatal("unbounded citation section was accepted")
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO citations (id,source_id,snapshot_id,evidence_id,title,locator,snapshot_date,last_verified,link_strategy,section,algorithm_version) VALUES ('citation.bad-link','source.raw','snapshot.raw','evidence.raw','Bad','https://example.com/raw',?,?,'url_anchor','body','citation-v1')`, now, now); err == nil {
		t.Fatal("deep-link strategy without a deep link was accepted")
	}
}

func TestResearchProvenanceRepositoryRoundTripsLatestBoundedGraph(t *testing.T) {
	database, _ := openTestDatabase(t)
	repository := database.Repositories().Research.Provenance
	ctx := context.Background()
	first := researchTestProvenanceGraph(t, "graph.trace.001", fixedTime.Add(time.Minute))
	second := researchTestProvenanceGraph(t, "graph.trace.002", fixedTime.Add(2*time.Minute))
	if err := repository.Append(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := repository.Append(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := repository.Append(ctx, second); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate provenance error = %v", err)
	}
	loaded, err := repository.LatestByClaim(ctx, second.ClaimID)
	loadedJSON, loadedJSONErr := loaded.ExportJSON()
	wantJSON, wantJSONErr := second.ExportJSON()
	if err != nil || loadedJSONErr != nil || wantJSONErr != nil || !reflect.DeepEqual(loadedJSON, wantJSON) {
		t.Fatalf("LatestByClaim() = (%+v, %v), want %+v", loaded, err, second)
	}
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO provenance_graphs (id,claim_id,graph_json,recorded_at,algorithm_version) VALUES ('graph.bad','claim.bad','{}',?,'provenance-graph-v1')`, fixedTime.Format(timestampFormat)); err == nil {
		t.Fatal("provenance schema accepted mismatched graph metadata")
	}
}

func researchTestProvenanceGraph(t *testing.T, graphID string, recorded time.Time) research.ProvenanceGraph {
	t.Helper()
	claimID := researchTestClaimID(t, "claim.trace")
	claimNodeID := researchTestID(t, claimID.String())
	requestID := researchTestID(t, "request.trace")
	runID := researchTestID(t, "run.trace")
	sourceID := researchTestID(t, "source.trace")
	snapshotID := researchTestID(t, "snapshot.trace")
	evidenceID := researchTestID(t, "evidence.trace")
	at := researchTestTimestamp(t, fixedTime)
	return research.ProvenanceGraph{
		ID: researchTestID(t, graphID), ClaimID: claimID,
		RecordedAt: researchTestTimestamp(t, recorded), AlgorithmVersion: research.ProvenanceGraphAlgorithmV1,
		Nodes: []research.ProvenanceNode{
			{ID: requestID, Kind: research.ProvenanceRequest, Label: "trace request", OccurredAt: at},
			{ID: runID, Kind: research.ProvenanceRun, Label: "trace run", OccurredAt: at},
			{ID: sourceID, Kind: research.ProvenanceSource, Label: "manual source", OccurredAt: at},
			{ID: snapshotID, Kind: research.ProvenanceSnapshot, Label: "historical snapshot", OccurredAt: at, ToolVersion: "fetch/v1"},
			{ID: evidenceID, Kind: research.ProvenanceEvidence, Label: "section 1", OccurredAt: at, ToolVersion: "extract/v1"},
			{ID: claimNodeID, Kind: research.ProvenanceClaim, Label: "traceable claim", OccurredAt: at},
		},
		Edges: []research.ProvenanceEdge{
			{From: requestID, To: runID}, {From: runID, To: sourceID},
			{From: sourceID, To: snapshotID}, {From: snapshotID, To: evidenceID},
			{From: evidenceID, To: claimNodeID},
		},
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
