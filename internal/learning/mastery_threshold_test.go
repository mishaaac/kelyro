package learning

import (
	"math"
	"testing"
	"time"
)

func TestMasteryRequirementPresetsAndCustomRange(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		value float64
		mode  MasteryThresholdMode
	}{
		{.70, MasteryModeRelaxed}, {.80, MasteryModeStandard}, {.85, MasteryModeStrict},
		{.90, MasteryModeMastery}, {.50, MasteryModeCustom}, {.77, MasteryModeCustom}, {.99, MasteryModeCustom},
	} {
		requirement, err := NewMasteryRequirement(test.value)
		if err != nil || requirement.Mode != test.mode || requirement.Threshold.Value() != test.value {
			t.Errorf("NewMasteryRequirement(%v) = (%+v, %v)", test.value, requirement, err)
		}
	}
	for _, value := range []float64{.49, 1, -1, math.NaN()} {
		if _, err := NewMasteryRequirement(value); err == nil {
			t.Errorf("NewMasteryRequirement(%v) accepted invalid value", value)
		}
	}
}

func TestMasteryRequirementUsesInclusiveCalculatedMasteryBoundary(t *testing.T) {
	t.Parallel()
	requirement, _ := NewMasteryRequirement(.85)
	below, _ := NewMasteryScore(.849)
	equal, _ := NewMasteryScore(.85)
	above, _ := NewMasteryScore(.90)
	if requirement.SatisfiedBy(below) || !requirement.SatisfiedBy(equal) || !requirement.SatisfiedBy(above) {
		t.Fatal("threshold-v1 must use calculated mastery >= required threshold")
	}
}

func TestResolveMasteryThresholdPrecedenceAndPackLimits(t *testing.T) {
	t.Parallel()
	studentID, _ := NewID("student.mastery")
	settings, err := NewMasteryThresholdSettings(studentID, masteryTime(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveMasteryThreshold(settings, nil)
	if err != nil || resolved.Source != MasterySourceStudentDefault || resolved.Requirement.Mode != MasteryModeStandard {
		t.Fatalf("student default = (%+v, %v)", resolved, err)
	}
	strict, _ := NewMasteryThreshold(.85)
	settings, _ = settings.SetWorkspaceOverride(strict, masteryTime(t, 1))
	resolved, _ = ResolveMasteryThreshold(settings, nil)
	if resolved.Source != MasterySourceWorkspaceOverride || resolved.Requirement.Mode != MasteryModeStrict {
		t.Fatalf("workspace override = %+v", resolved)
	}
	pack, err := NewPackMasteryOverride(.90, .80, .95)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err = ResolveMasteryThreshold(settings, &pack)
	if err != nil || resolved.Source != MasterySourcePackOverride || resolved.Requirement.Mode != MasteryModeMastery || resolved.PolicyVersion != MasteryThresholdPolicyVersion {
		t.Fatalf("pack override = (%+v, %v)", resolved, err)
	}
	if _, err := NewPackMasteryOverride(.75, .80, .90); err == nil {
		t.Fatal("pack override outside declared bounds was accepted")
	}
	if _, err := NewPackMasteryOverride(.85, .90, .80); err == nil {
		t.Fatal("reversed pack bounds were accepted")
	}
}

func TestMasteryThresholdSettingsTransitions(t *testing.T) {
	t.Parallel()
	studentID, _ := NewID("student.settings")
	settings, _ := NewMasteryThresholdSettings(studentID, masteryTime(t, 1))
	relaxed, _ := NewMasteryThreshold(.70)
	settings, err := settings.SetStudentDefault(relaxed, masteryTime(t, 2))
	if err != nil || settings.StudentDefault != relaxed {
		t.Fatalf("SetStudentDefault() = (%+v, %v)", settings, err)
	}
	custom, _ := NewMasteryThreshold(.77)
	settings, err = settings.SetWorkspaceOverride(custom, masteryTime(t, 3))
	if err != nil || settings.WorkspaceOverride == nil || *settings.WorkspaceOverride != custom {
		t.Fatalf("SetWorkspaceOverride() = (%+v, %v)", settings, err)
	}
	settings, err = settings.ClearWorkspaceOverride(masteryTime(t, 4))
	if err != nil || settings.WorkspaceOverride != nil {
		t.Fatalf("ClearWorkspaceOverride() = (%+v, %v)", settings, err)
	}
	if _, err := settings.SetStudentDefault(relaxed, masteryTime(t, 0)); err == nil {
		t.Fatal("backward transition time accepted")
	}
}

func masteryTime(t *testing.T, minute int) Timestamp {
	t.Helper()
	value, err := NewTimestamp(time.Date(2026, time.August, 19, 12, minute, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return value
}
