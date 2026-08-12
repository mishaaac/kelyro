package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/doctor"
)

func TestHomeViewGoldenWidths(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		width int
	}{
		{name: "small", width: 32},
		{name: "normal", width: 80},
		{name: "large", width: 120},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			model := readyModel(&fakeService{})
			model.width = test.width
			got := model.View()
			path := filepath.Join("testdata", "home_"+test.name+".golden")
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden %s: %v", path, err)
			}
			if got != string(want) {
				t.Errorf("View() mismatch for width %d\n--- want ---\n%s--- got ---\n%s", test.width, want, got)
			}
		})
	}
}

func TestDoctorViewRendersTypedSectionsAndRequirementReasons(t *testing.T) {
	t.Parallel()

	model := readyModel(&fakeService{})
	model.screen = screenDoctor
	model.snapshot.Diagnostics = doctor.Report{Checks: []doctor.Check{
		{ID: "platform.os", Section: doctor.SectionPlatform, DisplayName: "OS detected", Requirement: doctor.Required, State: doctor.Pass, Detail: "linux"},
		{ID: "tool.docker", Section: doctor.SectionOptional, DisplayName: "Docker", Requirement: doctor.Optional, State: doctor.Miss, Detail: "not found", WhyNeeded: "Run isolated module environments."},
	}}
	view := model.View()
	for _, expected := range []string{"Platform", "✓ OS detected", "Optional", "○ Docker [optional]", "Why: Run isolated module environments."} {
		if !strings.Contains(view, expected) {
			t.Errorf("doctor view missing %q:\n%s", expected, view)
		}
	}
}
