package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := Run([]string{"--help"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("Run() exit code = %d, want 0", exitCode)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("Run() output = %q, want usage information", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("Run() stderr = %q, want empty", stderr.String())
	}
}

func TestRunVersion(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if exitCode := Run([]string{"--version"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("Run() exit code = %d, want 0", exitCode)
	}
	if got, want := stdout.String(), "kelyro dev (commit unknown, built unknown)\n"; got != want {
		t.Errorf("Run() output = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("Run() stderr = %q, want empty", stderr.String())
	}
}
