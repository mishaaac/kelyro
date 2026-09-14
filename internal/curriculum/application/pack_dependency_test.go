package application

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestPackDependencyResolverV1SelectsHighestCompatibleTransitiveGraph(t *testing.T) {
	t.Parallel()
	root := dependencyManifest(t, "pack.backend", "1.0.0", dependency(t, "pack.foundations", ">=1.0.0 <2.0.0"), dependency(t, "pack.tools", "1.0.0"))
	available := []curriculum.PackManifest{
		dependencyManifest(t, "pack.foundations", "2.0.0"),
		dependencyManifest(t, "pack.foundations", "1.0.0"),
		dependencyManifest(t, "pack.foundations", "1.2.0", dependency(t, "pack.basics", ">=1.0.0")),
		dependencyManifest(t, "pack.basics", "1.1.0"),
		dependencyManifest(t, "pack.tools", "1.0.0"),
	}

	result, err := NewPackDependencyResolverV1().Resolve(context.Background(), PackDependencyResolutionRequest{Root: root, Available: available})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if result.Status != curriculum.PackDependenciesResolved || len(result.Selected) != 4 || len(result.Issues) != 0 || result.AlgorithmVersion != curriculum.PackDependencyResolverVersionV1 {
		t.Fatalf("resolution = %+v", result)
	}
	assertPackReferences(t, result.InstallOrder, "pack.basics@1.1.0", "pack.foundations@1.2.0", "pack.tools@1.0.0", "pack.backend@1.0.0")

	reordered := append([]curriculum.PackManifest(nil), available...)
	for left, right := 0, len(reordered)-1; left < right; left, right = left+1, right-1 {
		reordered[left], reordered[right] = reordered[right], reordered[left]
	}
	repeated, err := NewPackDependencyResolverV1().Resolve(context.Background(), PackDependencyResolutionRequest{Root: root, Available: reordered})
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("reordered resolution differs: %v\nfirst=%+v\nsecond=%+v", err, result, repeated)
	}
}

func TestPackDependencyResolverV1BacktracksAcrossSharedConstraints(t *testing.T) {
	t.Parallel()
	root := dependencyManifest(t, "pack.root", "1.0.0", dependency(t, "pack.a", "1.0.0"), dependency(t, "pack.z", "1.0.0"))
	available := []curriculum.PackManifest{
		dependencyManifest(t, "pack.a", "1.0.0", dependency(t, "pack.shared", ">=1.0.0 <3.0.0")),
		dependencyManifest(t, "pack.z", "1.0.0", dependency(t, "pack.shared", "<2.0.0")),
		dependencyManifest(t, "pack.shared", "2.0.0"),
		dependencyManifest(t, "pack.shared", "1.5.0"),
	}
	result, err := NewPackDependencyResolverV1().Resolve(context.Background(), PackDependencyResolutionRequest{Root: root, Available: available})
	if err != nil || result.Status != curriculum.PackDependenciesResolved {
		t.Fatalf("resolution = %+v / %v", result, err)
	}
	found := false
	for _, selected := range result.Selected {
		found = found || selected.ID.String() == "pack.shared" && selected.Version.String() == "1.5.0"
	}
	if !found {
		t.Fatalf("resolver did not backtrack to shared compatible version: %+v", result.Selected)
	}
}

func TestPackDependencyResolverV1ReportsMissingIncompatibleAndCycle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		root      curriculum.PackManifest
		available []curriculum.PackManifest
		kind      curriculum.PackDependencyIssueKind
	}{
		{
			name: "missing", root: dependencyManifest(t, "pack.root.missing", "1.0.0", dependency(t, "pack.absent", ">=1.0.0")),
			kind: curriculum.PackDependencyMissing,
		},
		{
			name: "incompatible", root: dependencyManifest(t, "pack.root.incompatible", "1.0.0", dependency(t, "pack.base", ">=2.0.0")),
			available: []curriculum.PackManifest{dependencyManifest(t, "pack.base", "1.9.0")}, kind: curriculum.PackDependencyIncompatible,
		},
		{
			name: "cycle", root: dependencyManifest(t, "pack.root.cycle", "1.0.0", dependency(t, "pack.middle", "1.0.0")),
			available: []curriculum.PackManifest{dependencyManifest(t, "pack.middle", "1.0.0", dependency(t, "pack.root.cycle", "1.0.0"))}, kind: curriculum.PackDependencyCycle,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := NewPackDependencyResolverV1().Resolve(context.Background(), PackDependencyResolutionRequest{Root: test.root, Available: test.available})
			if err != nil || result.Status != curriculum.PackDependenciesFailed || len(result.Issues) != 1 || result.Issues[0].Kind != test.kind || len(result.Selected) != 0 || len(result.InstallOrder) != 0 {
				t.Fatalf("resolution = %+v / %v", result, err)
			}
		})
	}
}

func dependencyManifest(t *testing.T, id, version string, dependencies ...curriculum.PackDependency) curriculum.PackManifest {
	t.Helper()
	created, _ := curriculum.NewTimestamp(time.Date(2026, 9, 13, 18, 0, 0, 0, time.UTC))
	curriculumIDValue, _ := curriculum.NewCurriculumID("curriculum." + id)
	return curriculum.PackManifest{
		ID: curriculumID(t, id), Name: id, Description: "Dependency resolver fixture.", Version: packVersion(t, version),
		SchemaVersion: "learning-pack/v1", Domain: "testing", Target: "Dependency resolution",
		Authors: []string{"Kelyro"}, Maintainers: []string{"Kelyro"}, License: "CC-BY-4.0", CreatedAt: created,
		MinimumKelyroVersion: packVersion(t, "0.1.0"), Dependencies: dependencies,
		CurriculumEntry: "curriculum/curriculum.yaml", SourceEvidenceEntry: "sources/evidence-report.json",
		Status: curriculum.ConceptPreview, CurriculumID: curriculumIDValue,
	}
}

func dependency(t *testing.T, id, constraint string) curriculum.PackDependency {
	t.Helper()
	return curriculum.PackDependency{PackID: curriculumID(t, id), Constraint: constraint}
}

func assertPackReferences(t *testing.T, values []curriculum.PackReference, expected ...string) {
	t.Helper()
	actual := make([]string, len(values))
	for index, value := range values {
		actual[index] = value.PackID.String() + "@" + value.Version.String()
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("install order = %v, want %v", actual, expected)
	}
}
