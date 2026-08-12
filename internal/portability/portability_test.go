package portability

import "testing"

func TestModesAndConflictStrategies(t *testing.T) {
	for _, mode := range []Mode{ModeHuman, ModeFull} {
		if !mode.Valid() {
			t.Errorf("mode %q is invalid", mode)
		}
	}
	if Mode("future").Valid() {
		t.Fatal("unknown mode is valid")
	}
	for _, strategy := range []ConflictStrategy{ConflictFail, ConflictKeep, ConflictOverwrite} {
		if !strategy.Valid() {
			t.Errorf("strategy %q is invalid", strategy)
		}
	}
	if ConflictStrategy("merge").Valid() {
		t.Fatal("unknown conflict strategy is valid")
	}
}
