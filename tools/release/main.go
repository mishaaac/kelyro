// Command release builds Kelyro's reproducible cross-platform release archives.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/update"
)

const usage = `Usage:
  go run ./tools/release build --version <vX.Y.Z> --commit <full-sha> --date <RFC3339> [--output <directory>]

The output directory must be empty. The command builds all supported release
targets with CGO disabled, creates deterministic archives, and writes SHA256SUMS.
`

const versionPackage = "github.com/mishaaac/kelyro/internal/version"

type target struct {
	os   string
	arch string
}

var releaseTargets = []target{
	{os: "darwin", arch: "amd64"},
	{os: "darwin", arch: "arm64"},
	{os: "linux", arch: "amd64"},
	{os: "linux", arch: "arm64"},
	{os: "windows", arch: "amd64"},
	{os: "windows", arch: "arm64"},
}

type metadata struct {
	version string
	commit  string
	date    time.Time
}

type buildBinary func(context.Context, string, target, metadata, string) error

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, executeBuild))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, build buildBinary) int {
	if len(args) == 0 || args[0] != "build" {
		fmt.Fprint(stderr, usage)
		return 2
	}

	flags := flag.NewFlagSet("release build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	version := flags.String("version", "", "release version with leading v")
	commit := flags.String("commit", "", "full Git commit SHA")
	date := flags.String("date", "", "reproducible build date in RFC3339 format")
	output := flags.String("output", "dist", "empty output directory")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "release: unexpected arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}

	meta, err := parseMetadata(*version, *commit, *date)
	if err != nil {
		fmt.Fprintf(stderr, "release: %v\n", err)
		return 2
	}
	root, err := moduleRoot()
	if err != nil {
		fmt.Fprintf(stderr, "release: %v\n", err)
		return 1
	}
	outputPath, err := filepath.Abs(*output)
	if err != nil {
		fmt.Fprintf(stderr, "release: resolve output directory: %v\n", err)
		return 1
	}
	if err := buildRelease(ctx, root, outputPath, meta, build, stdout); err != nil {
		fmt.Fprintf(stderr, "release: %v\n", err)
		return 1
	}
	return 0
}

func parseMetadata(version, commit, date string) (metadata, error) {
	if !strings.HasPrefix(version, "v") {
		return metadata{}, errors.New("version must be SemVer with a leading v")
	}
	parsedVersion, err := update.ParseVersion(version)
	if err != nil || "v"+parsedVersion.String() != version {
		return metadata{}, fmt.Errorf("version %q is not canonical SemVer", version)
	}
	if len(commit) != 40 || !isLowerHex(commit) {
		return metadata{}, errors.New("commit must be a full 40-character lowercase hexadecimal SHA")
	}
	parsedDate, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return metadata{}, fmt.Errorf("date must use RFC3339: %w", err)
	}
	return metadata{version: version, commit: commit, date: parsedDate.UTC()}, nil
}

func isLowerHex(value string) bool {
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func buildRelease(ctx context.Context, root, output string, meta metadata, build buildBinary, stdout io.Writer) error {
	if err := requireEmptyDirectory(output); err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "kelyro-release-")
	if err != nil {
		return fmt.Errorf("create temporary build directory: %w", err)
	}
	defer os.RemoveAll(work)

	archivePaths := make([]string, 0, len(releaseTargets))
	for _, target := range releaseTargets {
		executable := "kelyro"
		if target.os == "windows" {
			executable += ".exe"
		}
		binaryPath := filepath.Join(work, target.os+"-"+target.arch, executable)
		if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
			return fmt.Errorf("prepare %s/%s build: %w", target.os, target.arch, err)
		}
		fmt.Fprintf(stdout, "==> build %s/%s\n", target.os, target.arch)
		if err := build(ctx, root, target, meta, binaryPath); err != nil {
			return fmt.Errorf("build %s/%s: %w", target.os, target.arch, err)
		}

		archivePath := filepath.Join(output, archiveName(meta.version, target))
		if target.os == "windows" {
			err = writeZip(archivePath, binaryPath, executable, meta.date)
		} else {
			err = writeTarGzip(archivePath, binaryPath, executable, meta.date)
		}
		if err != nil {
			return fmt.Errorf("archive %s/%s: %w", target.os, target.arch, err)
		}
		archivePaths = append(archivePaths, archivePath)
	}

	if err := writeChecksums(output, archivePaths); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "==> wrote %d archives and SHA256SUMS to %s\n", len(archivePaths), output)
	return nil
}

func requireEmptyDirectory(path string) error {
	entries, err := os.ReadDir(path)
	switch {
	case err == nil && len(entries) != 0:
		return fmt.Errorf("output directory %s is not empty", path)
	case err == nil:
		return nil
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("inspect output directory: %w", err)
	}
}

func executeBuild(ctx context.Context, root string, target target, meta metadata, output string) error {
	date := meta.date.Format(time.RFC3339)
	ldflags := strings.Join([]string{
		"-s",
		"-w",
		"-buildid=",
		"-X", versionPackage + ".Version=" + meta.version,
		"-X", versionPackage + ".Commit=" + meta.commit,
		"-X", versionPackage + ".Date=" + date,
	}, " ")
	process := exec.CommandContext(ctx, "go", "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", output, "./cmd/kelyro")
	process.Dir = root
	process.Env = releaseEnvironment(os.Environ(), target, meta.date)
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr
	return process.Run()
}

func releaseEnvironment(environment []string, target target, date time.Time) []string {
	filtered := make([]string, 0, len(environment)+4)
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "GOOS", "GOARCH", "CGO_ENABLED", "SOURCE_DATE_EPOCH":
			continue
		default:
			filtered = append(filtered, entry)
		}
	}
	return append(filtered,
		"GOOS="+target.os,
		"GOARCH="+target.arch,
		"CGO_ENABLED=0",
		fmt.Sprintf("SOURCE_DATE_EPOCH=%d", date.Unix()),
	)
}

func archiveName(version string, target target) string {
	extension := ".tar.gz"
	if target.os == "windows" {
		extension = ".zip"
	}
	return fmt.Sprintf("kelyro_%s_%s_%s%s", strings.TrimPrefix(version, "v"), target.os, target.arch, extension)
}

func writeTarGzip(archivePath, binaryPath, executable string, timestamp time.Time) (returnErr error) {
	input, err := os.Open(binaryPath)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	output, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := output.Close(); returnErr == nil && closeErr != nil {
			returnErr = closeErr
		}
	}()
	gzipWriter := gzip.NewWriter(output)
	gzipWriter.Header.ModTime = timestamp.UTC()
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	header := &tar.Header{
		Name: executable, Mode: 0o755, Size: info.Size(),
		ModTime: timestamp.UTC(), Format: tar.FormatPAX,
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}
	if _, err := io.Copy(tarWriter, input); err != nil {
		return err
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	return gzipWriter.Close()
}

func writeZip(archivePath, binaryPath, executable string, timestamp time.Time) (returnErr error) {
	input, err := os.Open(binaryPath)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := output.Close(); returnErr == nil && closeErr != nil {
			returnErr = closeErr
		}
	}()
	zipWriter := zip.NewWriter(output)
	header := &zip.FileHeader{Name: executable, Method: zip.Deflate}
	header.SetMode(0o755)
	header.SetModTime(timestamp.UTC())
	entry, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	if _, err := io.Copy(entry, input); err != nil {
		return err
	}
	return zipWriter.Close()
}

func writeChecksums(output string, archivePaths []string) (returnErr error) {
	sort.Slice(archivePaths, func(left, right int) bool {
		return filepath.Base(archivePaths[left]) < filepath.Base(archivePaths[right])
	})
	checksums, err := os.Create(filepath.Join(output, "SHA256SUMS"))
	if err != nil {
		return fmt.Errorf("create SHA256SUMS: %w", err)
	}
	defer func() {
		if closeErr := checksums.Close(); returnErr == nil && closeErr != nil {
			returnErr = closeErr
		}
	}()
	for _, archivePath := range archivePaths {
		archive, err := os.Open(archivePath)
		if err != nil {
			return fmt.Errorf("open archive for checksum: %w", err)
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, archive)
		closeErr := archive.Close()
		if copyErr != nil {
			return fmt.Errorf("hash archive: %w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close archive after checksum: %w", closeErr)
		}
		if _, err := fmt.Fprintf(checksums, "%x  %s\n", hash.Sum(nil), filepath.Base(archivePath)); err != nil {
			return fmt.Errorf("write SHA256SUMS: %w", err)
		}
	}
	return nil
}

func moduleRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		_, err := os.Stat(filepath.Join(directory, "go.mod"))
		if err == nil {
			return directory, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect go.mod: %w", err)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("go.mod not found in current directory or its parents")
		}
		directory = parent
	}
}
