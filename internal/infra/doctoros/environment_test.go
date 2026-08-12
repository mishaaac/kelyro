package doctoros

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestWritableRemovesProbeFile(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := New().Writable(directory); err != nil {
		t.Fatalf("Writable() error = %v", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("write probe left entries: %v", entries)
	}
}

func TestVersionRunsInDisposableDirectory(t *testing.T) {
	t.Setenv("KELYRO_DOCTOR_HELPER", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	output, err := New().Version(context.Background(), executable, []string{"-test.run=TestDoctorVersionHelperProcess"})
	if err != nil {
		t.Fatalf("Version() error = %v, output = %q", err, output)
	}
	isolate := strings.TrimPrefix(strings.TrimSpace(output), "tool v1.2.3 cwd=")
	if isolate == current || isolate == "" {
		t.Fatalf("version working directory = %q, current = %q", isolate, current)
	}
	if _, err := os.Stat(isolate); !os.IsNotExist(err) {
		t.Errorf("isolated directory still exists: %v", err)
	}
}

func TestDoctorVersionHelperProcess(t *testing.T) {
	if os.Getenv("KELYRO_DOCTOR_HELPER") != "1" {
		return
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile("version-side-effect", []byte("probe"), 0o600); err != nil {
		os.Exit(3)
	}
	fmt.Printf("tool v1.2.3 cwd=%s", workingDirectory)
	os.Exit(0)
}

func TestLimitedBufferBoundsCapturedVersionOutput(t *testing.T) {
	t.Parallel()

	buffer := &limitedBuffer{limit: 8}
	input := strings.Repeat("x", 64)
	written, err := buffer.Write([]byte(input))
	if err != nil || written != len(input) {
		t.Fatalf("Write() = %d, %v", written, err)
	}
	if got := buffer.String(); got != strings.Repeat("x", 8) {
		t.Errorf("buffer = %q", got)
	}
}

func TestResolveReportsMissingCandidate(t *testing.T) {
	t.Parallel()

	if path, found := New().Resolve([]string{"kelyro-command-that-must-not-exist"}); found || path != "" {
		t.Errorf("Resolve() = %q, %v", path, found)
	}
}
