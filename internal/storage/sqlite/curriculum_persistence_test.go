package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/platform"
)

func TestResearchSchemaMigratesToCurriculumPersistenceWithoutLosingState(t *testing.T) {
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

	if err := database.migrate(context.Background(), foundationMigrations[:47]); err != nil {
		t.Fatalf("migrate through I-03 schema: %v", err)
	}
	if _, err := handle.Exec(`INSERT INTO app_state (namespace,key,value,updated_at) VALUES ('i03','kept',X'6F6B',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO research_topics (request_id,subject,purpose,requested_at) VALUES ('request.i04','I-04 baseline','concept_definition',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO research_runs (id,request_id,status,started_at) VALUES ('run.i04','request.i04','running',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	bundleHash := "sha256:" + strings.Repeat("a", 64)
	if _, err := handle.Exec(`INSERT INTO source_bundles (id,run_id,topic_subject,purpose,state,verified_at,bundle_json,content_hash,algorithm_version) VALUES ('bundle.i04','run.i04','I-04 baseline','concept_definition','ready',?,'{}',?,'source-bundle-v1')`, fixedTime.Format(timestampFormat), bundleHash); err != nil {
		t.Fatal(err)
	}

	if err := database.migrate(context.Background(), foundationMigrations); err != nil {
		t.Fatalf("migrate I-03 to I-04: %v", err)
	}
	if version, err := database.SchemaVersion(context.Background()); err != nil || version != 48 {
		t.Fatalf("schema = (%d,%v), want 48", version, err)
	}
	var marker string
	if err := handle.QueryRow(`SELECT CAST(value AS TEXT) FROM app_state WHERE namespace='i03' AND key='kept'`).Scan(&marker); err != nil || marker != "ok" {
		t.Fatalf("I-03 marker = (%q,%v)", marker, err)
	}
	var bundleCount int
	if err := handle.QueryRow(`SELECT COUNT(*) FROM source_bundles WHERE id='bundle.i04'`).Scan(&bundleCount); err != nil || bundleCount != 1 {
		t.Fatalf("source bundle count = (%d,%v)", bundleCount, err)
	}

	if _, err := handle.Exec(`INSERT INTO curriculum_instances (id,version) VALUES ('curriculum.i04','1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO curriculum_definitions (id,title,description,created_at) VALUES ('curriculum.i04','Curriculum','Source backed',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Exec(`INSERT INTO curriculum_versions (curriculum_id,version,source_policy,definition_json,created_at) VALUES ('curriculum.i04','1','required','{}',?)`, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	hash := bundleHash
	if _, err := handle.Exec(`INSERT INTO curriculum_sources (curriculum_id,curriculum_version,bundle_id,content_hash,algorithm_version,verified_at,position) VALUES ('curriculum.i04','1','bundle.i04',?,'source-bundle-v1',?,0)`, hash, fixedTime.Format(timestampFormat)); err != nil {
		t.Fatal(err)
	}
	wrongHash := "sha256:" + strings.Repeat("b", 64)
	if _, err := handle.Exec(`INSERT INTO curriculum_sources (curriculum_id,curriculum_version,bundle_id,content_hash,algorithm_version,verified_at,position) VALUES ('curriculum.i04','1','bundle.i04',?,'source-bundle-v1',?,1)`, wrongHash, fixedTime.Format(timestampFormat)); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("mismatched Source Bundle identity error = %v", err)
	}
	if _, err := handle.Exec(`UPDATE curriculum_versions SET definition_json='{"changed":true}' WHERE curriculum_id='curriculum.i04' AND version='1'`); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("immutable curriculum version update error = %v", err)
	}
	if _, err := handle.Exec(`INSERT INTO curriculum_sources (curriculum_id,curriculum_version,bundle_id,content_hash,algorithm_version,verified_at,position) VALUES ('curriculum.i04','1','bundle.missing',?,'source-bundle-v1',?,1)`, hash, fixedTime.Format(timestampFormat)); err == nil {
		t.Fatal("curriculum source accepted missing I-03 bundle")
	}
}
