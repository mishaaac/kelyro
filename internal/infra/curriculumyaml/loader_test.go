package curriculumyaml

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/learning"
)

func TestLoadFoundationDemoFixture(t *testing.T) {
	t.Parallel()

	encoded := readFoundationFixture(t)
	first, err := Load(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	second, err := Load(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Load() repeated error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same fixture/version did not load deterministically")
	}
	if first.ContractVersion != learning.CurriculumContractVersion || first.Reference.Version != "1.0.0" {
		t.Fatalf("loaded contract/reference = %+v / %+v", first.ContractVersion, first.Reference)
	}
	if got := len(first.Nodes); got != 6 {
		t.Fatalf("loaded node count = %d, want 6", got)
	}

	conceptID, err := learning.NewID("concept.equivalent-ratios")
	if err != nil {
		t.Fatal(err)
	}
	concept, found := first.Node(conceptID)
	if !found || concept.Concept == nil {
		t.Fatalf("equivalent-ratios concept = %+v, %v", concept, found)
	}
	if got := concept.Concept.Prerequisites[0].Requirement; got != learning.PrerequisiteMastered {
		t.Fatalf("prerequisite requirement = %q", got)
	}
	if !concept.Concept.TheoryRequired || concept.Concept.EstimatedEffortMinutes != 30 {
		t.Fatalf("concept pedagogical metadata = %+v", concept.Concept)
	}
}

func TestLoadRejectsStrictYAMLViolations(t *testing.T) {
	t.Parallel()

	fixture := readFoundationFixture(t)
	tests := []struct {
		name    string
		encoded []byte
		want    string
	}{
		{
			name:    "unknown field",
			encoded: append(append([]byte(nil), fixture...), []byte("\nunknown_field: true\n")...),
			want:    "field unknown_field not found",
		},
		{
			name:    "duplicate mapping key",
			encoded: append([]byte("id: duplicate\n"), fixture...),
			want:    "mapping key \"id\" already defined",
		},
		{
			name:    "multiple documents",
			encoded: append(append([]byte(nil), fixture...), []byte("\n---\nid: another\n")...),
			want:    "multiple documents are not allowed",
		},
		{
			name:    "empty document",
			encoded: nil,
			want:    "document is empty",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			_, err := Load(bytes.NewReader(test.encoded))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestLoadRejectsDanglingFixturePrerequisite(t *testing.T) {
	t.Parallel()

	encoded := strings.Replace(
		string(readFoundationFixture(t)),
		"concept_id: concept.ratio-meaning",
		"concept_id: concept.not-present",
		1,
	)
	_, err := Load(strings.NewReader(encoded))
	if err == nil || !strings.Contains(err.Error(), "unknown prerequisite") {
		t.Fatalf("Load() error = %v, want unknown prerequisite", err)
	}
}

func readFoundationFixture(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "curricula", "foundation-demo", "curriculum.yaml")
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return encoded
}
