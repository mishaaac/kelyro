package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestPackVersioningPolicyV1ClassifiesStablePatchMinorAndMajor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		kind       curriculum.CurriculumChangeKind
		migration  curriculum.MigrationClass
		candidate  string
		impact     curriculum.PackChangeImpact
		transition curriculum.PackVersionTransition
	}{
		{"source refresh", curriculum.ChangeSourceRefresh, curriculum.MigrationSafe, "1.2.4", curriculum.PackImpactPatch, curriculum.PackTransitionPatch},
		{"new concept", curriculum.ChangeConceptAdded, curriculum.MigrationSafe, "1.3.0", curriculum.PackImpactMinor, curriculum.PackTransitionMinor},
		{"removed concept cannot be downgraded", curriculum.ChangeConceptRemoved, curriculum.MigrationRequiresRecompile, "2.0.0", curriculum.PackImpactMajor, curriculum.PackTransitionMajor},
		{"restructured for learner review", curriculum.ChangeHierarchyChanged, curriculum.MigrationRequiresStudentReview, "2.0.0", curriculum.PackImpactMajor, curriculum.PackTransitionMajor},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := NewPackVersioningPolicyV1().Classify(context.Background(), PackVersioningRequest{
				CurrentVersion: packVersion(t, "1.2.3"), CandidateVersion: packVersion(t, test.candidate),
				Changes: []curriculum.CurriculumChange{packChange(t, "change.one", test.kind, test.migration)},
			})
			if err != nil || !result.Allowed || result.ChangeImpact != test.impact || result.RequiredTransition != test.transition || result.ActualTransition != test.transition || result.AlgorithmVersion != curriculum.PackVersioningPolicyVersionV1 {
				t.Fatalf("decision = %+v / %v", result, err)
			}
		})
	}
}

func TestPackVersioningPolicyV1RejectsWrongSkippedAndNonIncreasingVersions(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, candidate string }{
		{"under bump", "1.2.4"},
		{"over bump", "2.0.0"},
		{"skip minor", "1.4.0"},
		{"same published identity", "1.2.3"},
		{"build metadata only", "1.2.3+rebuilt"},
		{"backward", "1.2.2"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := NewPackVersioningPolicyV1().Classify(context.Background(), PackVersioningRequest{
				CurrentVersion: packVersion(t, "1.2.3"), CandidateVersion: packVersion(t, test.candidate),
				Changes: []curriculum.CurriculumChange{packChange(t, "change.add", curriculum.ChangeConceptAdded, curriculum.MigrationSafe)},
			})
			if err != nil || result.Allowed || len(result.Reasons) != 1 {
				t.Fatalf("decision = %+v / %v", result, err)
			}
		})
	}
}

func TestPackVersioningPolicyV1UsesNextMinorForBreakingZeroMajorChange(t *testing.T) {
	t.Parallel()
	result, err := NewPackVersioningPolicyV1().Classify(context.Background(), PackVersioningRequest{
		CurrentVersion: packVersion(t, "0.4.2"), CandidateVersion: packVersion(t, "0.5.0"),
		Changes: []curriculum.CurriculumChange{packChange(t, "change.split", curriculum.ChangeConceptSplit, curriculum.MigrationBreaking)},
	})
	if err != nil || !result.Allowed || result.ChangeImpact != curriculum.PackImpactMajor || result.RequiredTransition != curriculum.PackTransitionMinor || result.ActualTransition != curriculum.PackTransitionMinor {
		t.Fatalf("0.x decision = %+v / %v", result, err)
	}
}

func TestPackVersioningPolicyV1AllowsOnlyNewIncreasingPrereleaseIdentity(t *testing.T) {
	t.Parallel()
	service := NewPackVersioningPolicyV1()
	request := PackVersioningRequest{
		CurrentVersion: packVersion(t, "0.5.0-alpha.1"), CandidateVersion: packVersion(t, "0.5.0-alpha.2"),
		Changes: []curriculum.CurriculumChange{packChange(t, "change.breaking", curriculum.ChangeConceptMerged, curriculum.MigrationBreaking)},
	}
	result, err := service.Classify(context.Background(), request)
	if err != nil || !result.Allowed || result.ActualTransition != curriculum.PackTransitionPrerelease || result.ChangeImpact != curriculum.PackImpactMajor {
		t.Fatalf("prerelease decision = %+v / %v", result, err)
	}
	request.CandidateVersion = request.CurrentVersion
	rejected, err := service.Classify(context.Background(), request)
	if err != nil || rejected.Allowed || rejected.ActualTransition != curriculum.PackTransitionInvalid {
		t.Fatalf("same prerelease decision = %+v / %v", rejected, err)
	}
}

func TestPackVersioningPolicyV1UsesHighestImpactAndStableChangeOrder(t *testing.T) {
	t.Parallel()
	minor := packChange(t, "change.b", curriculum.ChangeEnvironmentChanged, curriculum.MigrationSafe)
	patch := packChange(t, "change.a", curriculum.ChangeMetadataOnly, curriculum.MigrationSafe)
	request := PackVersioningRequest{CurrentVersion: packVersion(t, "1.0.0"), CandidateVersion: packVersion(t, "1.1.0"), Changes: []curriculum.CurriculumChange{minor, patch}}
	first, err := NewPackVersioningPolicyV1().Classify(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.Changes = []curriculum.CurriculumChange{patch, minor}
	second, err := NewPackVersioningPolicyV1().Classify(context.Background(), request)
	if err != nil || !reflect.DeepEqual(first, second) || first.ChangeImpact != curriculum.PackImpactMinor || !first.Allowed {
		t.Fatalf("decisions differ: %+v / %+v / %v", first, second, err)
	}
}

func packVersion(t *testing.T, value string) curriculum.PackVersion {
	t.Helper()
	version, err := curriculum.NewPackVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func packChange(t *testing.T, id string, kind curriculum.CurriculumChangeKind, migration curriculum.MigrationClass) curriculum.CurriculumChange {
	t.Helper()
	from, _ := curriculum.NewCurriculumVersion("curriculum-v1")
	to, _ := curriculum.NewCurriculumVersion("curriculum-v2")
	return curriculum.CurriculumChange{ID: curriculumID(t, id), FromVersion: from, ToVersion: to, Kind: kind, Migration: migration, Rationale: "Version policy fixture."}
}
