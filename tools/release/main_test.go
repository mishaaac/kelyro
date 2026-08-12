package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testCommit = "0123456789abcdef0123456789abcdef01234567"

func TestParseMetadata(t *testing.T) {
	meta, err := parseMetadata("v0.2.0-alpha.1", testCommit, "2026-08-12T08:00:00-05:00")
	if err != nil {
		t.Fatalf("parseMetadata() error = %v", err)
	}
	if got, want := meta.date.Format(time.RFC3339), "2026-08-12T13:00:00Z"; got != want {
		t.Fatalf("metadata date = %q, want %q", got, want)
	}
}

func TestParseMetadataRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		version string
		commit  string
		date    string
	}{
		{name: "missing v", version: "0.2.0", commit: testCommit, date: "2026-08-12T13:00:00Z"},
		{name: "invalid semver", version: "v0.2", commit: testCommit, date: "2026-08-12T13:00:00Z"},
		{name: "short commit", version: "v0.2.0", commit: "0123456", date: "2026-08-12T13:00:00Z"},
		{name: "uppercase commit", version: "v0.2.0", commit: strings.ToUpper(testCommit), date: "2026-08-12T13:00:00Z"},
		{name: "invalid date", version: "v0.2.0", commit: testCommit, date: "today"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseMetadata(test.version, test.commit, test.date); err == nil {
				t.Fatal("parseMetadata() error = nil, want an error")
			}
		})
	}
}

func TestArchiveNamesCoverReleaseTargets(t *testing.T) {
	want := []string{
		"kelyro_0.2.0_darwin_amd64.tar.gz",
		"kelyro_0.2.0_darwin_arm64.tar.gz",
		"kelyro_0.2.0_linux_amd64.tar.gz",
		"kelyro_0.2.0_linux_arm64.tar.gz",
		"kelyro_0.2.0_windows_amd64.zip",
		"kelyro_0.2.0_windows_arm64.zip",
	}
	for index, target := range releaseTargets {
		if got := archiveName("v0.2.0", target); got != want[index] {
			t.Errorf("archiveName(%+v) = %q, want %q", target, got, want[index])
		}
	}
}

func TestArchivesAreDeterministicAndExecutable(t *testing.T) {
	directory := t.TempDir()
	binary := filepath.Join(directory, "source")
	if err := os.WriteFile(binary, []byte("binary contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	timestamp := time.Date(2026, 8, 12, 13, 0, 0, 0, time.UTC)

	for _, test := range []struct {
		name       string
		extension  string
		write      func(string, string, string, time.Time) error
		readMember func(*testing.T, string) (string, os.FileMode, string)
	}{
		{name: "tar gzip", extension: ".tar.gz", write: writeTarGzip, readMember: readTarMember},
		{name: "zip", extension: ".zip", write: writeZip, readMember: readZipMember},
	} {
		t.Run(test.name, func(t *testing.T) {
			first := filepath.Join(directory, "first"+test.extension)
			second := filepath.Join(directory, "second"+test.extension)
			if err := test.write(first, binary, "kelyro", timestamp); err != nil {
				t.Fatal(err)
			}
			if err := test.write(second, binary, "kelyro", timestamp); err != nil {
				t.Fatal(err)
			}
			firstBytes, _ := os.ReadFile(first)
			secondBytes, _ := os.ReadFile(second)
			if !bytes.Equal(firstBytes, secondBytes) {
				t.Fatal("archives built from identical inputs differ")
			}
			name, mode, contents := test.readMember(t, first)
			if name != "kelyro" || mode.Perm() != 0o755 || contents != "binary contents" {
				t.Fatalf("archive member = (%q, %o, %q)", name, mode.Perm(), contents)
			}
		})
	}
}

func TestBuildReleaseCreatesArchivesAndChecksums(t *testing.T) {
	output := filepath.Join(t.TempDir(), "dist")
	meta, err := parseMetadata("v0.2.0", testCommit, "2026-08-12T13:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	fakeBuild := func(_ context.Context, _ string, target target, _ metadata, output string) error {
		return os.WriteFile(output, []byte(target.os+"/"+target.arch), 0o755)
	}
	if err := buildRelease(context.Background(), t.TempDir(), output, meta, fakeBuild, io.Discard); err != nil {
		t.Fatalf("buildRelease() error = %v", err)
	}
	entries, err := os.ReadDir(output)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(entries), len(releaseTargets)+1; got != want {
		t.Fatalf("output entries = %d, want %d", got, want)
	}
	checksums, err := os.ReadFile(filepath.Join(output, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(checksums)), "\n")
	if got, want := len(lines), len(releaseTargets); got != want {
		t.Fatalf("checksum lines = %d, want %d", got, want)
	}
	for _, line := range lines {
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed checksum line %q", line)
		}
		contents, err := os.ReadFile(filepath.Join(output, parts[1]))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := parts[0], fmt.Sprintf("%x", sha256.Sum256(contents)); got != want {
			t.Errorf("checksum for %s = %q, want %q", parts[1], got, want)
		}
	}
	if err := buildRelease(context.Background(), t.TempDir(), output, meta, fakeBuild, io.Discard); err == nil {
		t.Fatal("buildRelease() into non-empty directory error = nil")
	}
}

func TestReleaseEnvironmentOverridesBuildInputs(t *testing.T) {
	timestamp := time.Unix(1234567890, 0).UTC()
	environment := releaseEnvironment([]string{"PATH=/bin", "GOOS=old", "CGO_ENABLED=1"}, target{os: "linux", arch: "arm64"}, timestamp)
	joined := strings.Join(environment, "\n")
	for _, want := range []string{"PATH=/bin", "GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0", "SOURCE_DATE_EPOCH=1234567890"} {
		if !strings.Contains(joined, want) {
			t.Errorf("environment missing %q: %v", want, environment)
		}
	}
	if strings.Contains(joined, "GOOS=old") || strings.Contains(joined, "CGO_ENABLED=1") {
		t.Fatalf("environment retains overridden values: %v", environment)
	}
}

func readTarMember(t *testing.T, path string) (string, os.FileMode, string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	header, err := tarReader.Next()
	if err != nil {
		t.Fatal(err)
	}
	contents, err := io.ReadAll(tarReader)
	if err != nil {
		t.Fatal(err)
	}
	return header.Name, header.FileInfo().Mode(), string(contents)
}

func readZipMember(t *testing.T, path string) (string, os.FileMode, string) {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if len(reader.File) != 1 {
		t.Fatalf("zip member count = %d, want 1", len(reader.File))
	}
	member := reader.File[0]
	file, err := member.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	contents, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	return member.Name, member.Mode(), string(contents)
}
