package tui

import (
	"os"
	"path/filepath"
	"testing"
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
