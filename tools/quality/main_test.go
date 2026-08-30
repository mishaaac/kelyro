package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPlanIndividualGates(t *testing.T) {
	binary := filepath.Join("build", "kelyro")
	tests := []struct {
		name string
		gate string
		want []command
	}{
		{name: "test", gate: "test", want: []command{{name: "go", args: []string{"test", "./..."}}}},
		{name: "e2e", gate: "e2e", want: []command{{name: "go", args: []string{"test", "-tags=e2e", "./tests/e2e"}}}},
		{name: "vet", gate: "vet", want: []command{{name: "go", args: []string{"vet", "./..."}}}},
		{name: "race", gate: "race", want: []command{{name: "go", args: []string{"test", "-race", "-timeout=20m", "./..."}}}},
		{name: "build and smoke", gate: "build-smoke", want: []command{
			{name: "go", args: []string{"build", "-o", binary, "./cmd/kelyro"}},
			{name: binary, args: []string{"--version"}},
			{name: binary, args: []string{"--help"}},
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := plan(test.gate, binary)
			if err != nil {
				t.Fatalf("plan(%q) error = %v", test.gate, err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("plan(%q) = %#v, want %#v", test.gate, got, test.want)
			}
		})
	}
}

func TestPlanAllPreservesGateOrder(t *testing.T) {
	binary := filepath.Join("build", "kelyro")
	got, err := plan("all", binary)
	if err != nil {
		t.Fatalf("plan(all) error = %v", err)
	}
	want := []command{
		{name: "go", args: []string{"test", "./..."}},
		{name: "go", args: []string{"test", "-tags=e2e", "./tests/e2e"}},
		{name: "go", args: []string{"vet", "./..."}},
		{name: "go", args: []string{"test", "-race", "-timeout=20m", "./..."}},
		{name: "go", args: []string{"build", "-o", binary, "./cmd/kelyro"}},
		{name: binary, args: []string{"--version"}},
		{name: binary, args: []string{"--help"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan(all) = %#v, want %#v", got, want)
	}
}

func TestPlanRejectsUnknownGate(t *testing.T) {
	if _, err := plan("lint", "kelyro"); err == nil {
		t.Fatal("plan(lint) error = nil, want an error")
	}
}
