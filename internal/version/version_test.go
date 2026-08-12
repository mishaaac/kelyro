package version

import "testing"

func TestCurrentHasValidDefaults(t *testing.T) {
	t.Parallel()

	info := Current()
	if info.Version == "" {
		t.Error("Current().Version is empty")
	}
	if info.Commit == "" {
		t.Error("Current().Commit is empty")
	}
	if info.Date == "" {
		t.Error("Current().Date is empty")
	}
}

func TestInfoString(t *testing.T) {
	t.Parallel()

	info := Info{
		Version: "v1.2.3",
		Commit:  "abc1234",
		Date:    "2026-08-12T12:00:00Z",
	}

	const want = "v1.2.3 (commit abc1234, built 2026-08-12T12:00:00Z)"
	if got := info.String(); got != want {
		t.Errorf("Info.String() = %q, want %q", got, want)
	}
}

func TestIsDevelopmentUsesExactBuildIdentifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  bool
	}{
		{value: "dev", want: true},
		{value: "unknown", want: true},
		{value: "v0.1.0-alpha.1"},
		{value: "0.1.0-alpha.1"},
		{value: "0.1.0"},
		{value: "1.2.3"},
		{value: "not-a-version"},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()
			if got := IsDevelopment(test.value); got != test.want {
				t.Errorf("IsDevelopment(%q) = %t, want %t", test.value, got, test.want)
			}
		})
	}
}
