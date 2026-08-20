package learning

import (
	"strings"
	"testing"
)

func TestCurriculumFingerprintProtectsCompleteVersionedDefinition(t *testing.T) {
	t.Parallel()
	curriculum := mustCurriculum(t, validCurriculumNodes(t))
	fingerprint, err := CurriculumFingerprint(curriculum)
	if err != nil || !strings.HasPrefix(fingerprint, "sha256:") || len(fingerprint) != 71 {
		t.Fatalf("CurriculumFingerprint() = (%q, %v)", fingerprint, err)
	}
	reversedNodes := validCurriculumNodes(t)
	for left, right := 0, len(reversedNodes)-1; left < right; left, right = left+1, right-1 {
		reversedNodes[left], reversedNodes[right] = reversedNodes[right], reversedNodes[left]
	}
	equivalent, err := CurriculumFingerprint(mustCurriculum(t, reversedNodes))
	if err != nil || equivalent != fingerprint {
		t.Fatalf("equivalent fingerprint = (%q, %v), want %q", equivalent, err, fingerprint)
	}
	changed := curriculum
	changed.Nodes = cloneCurriculumNodes(curriculum.Nodes)
	changed.Nodes[len(changed.Nodes)-1].Concept.Objectives[0] = "Changed objective"
	changedFingerprint, err := CurriculumFingerprint(changed)
	if err != nil || changedFingerprint == fingerprint {
		t.Fatalf("changed fingerprint = (%q, %v), want a different value", changedFingerprint, err)
	}
}

func TestInstanceConceptStateModelsLazyProgressAndTemporalPolicy(t *testing.T) {
	t.Parallel()
	instance, err := NewCurriculumInstance(
		mustID(t, "instance.ratios"), mustID(t, "student.ada"), mustID(t, "goal.ratios"),
		CurriculumRef{ID: mustID(t, "curriculum.ratios"), Version: "1.0.0"},
		CurriculumSourceFixture, mustTimestamp(t, 10),
	)
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewInstanceConceptState(instance, mustID(t, "concept.ratio"), mustTimestamp(t, 10))
	if err != nil || state.Exposure != ExposureNotSeen || state.Mastery.Value() != 0 || state.FirstSeenAt != nil {
		t.Fatalf("NewInstanceConceptState() = (%+v, %v)", state, err)
	}
	seen := mustTimestamp(t, 11)
	state.Exposure = ExposureLearning
	state.Mastery = mustScore(t, .55)
	state.FirstSeenAt = &seen
	state.LastSeenAt = &seen
	state.UpdatedAt = mustTimestamp(t, 12)
	state.ManualFlags = []string{"flag.needs-example", "flag.needs-review"}
	if err := state.Validate(); err != nil {
		t.Fatalf("valid learning state: %v", err)
	}
	state.ManualFlags = []string{"flag.needs-review", "flag.needs-example"}
	if err := state.Validate(); err == nil {
		t.Fatal("Validate() accepted unstable manual flag order")
	}
	state.ManualFlags = nil
	state.LastSeenAt = nil
	if err := state.Validate(); err == nil {
		t.Fatal("Validate() accepted first_seen without last_seen")
	}
}
