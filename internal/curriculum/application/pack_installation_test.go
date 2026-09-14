package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestPackInstallerV1ValidatesResolvesAndInstallsImmutably(t *testing.T) {
	t.Parallel()
	dependencyPack := installationPack(t, "pack.foundation", "1.0.0")
	rootPack := installationPack(t, "pack.backend", "1.0.0", dependency(t, "pack.foundation", ">=1.0.0 <2.0.0"))
	repository := newInstallationRepositoryFake()
	repository.values[installationKey(dependencyPack)] = installedFixture(t, dependencyPack, "b")
	validator := validationFake{results: map[string]PackValidationResult{"backend.zip": validationFixture(rootPack, "a")}}
	service := NewPackInstallerV1(validator, NewPackDependencyResolverV1(), repository, fixedCurriculumClock(t, 20))

	result, err := service.Install(context.Background(), PackInstallRequest{Source: PackSource{Path: "backend.zip"}})
	if err != nil || !result.Installed || result.Pack.Manifest.ID != rootPack.Manifest.ID {
		t.Fatalf("Install() = %+v, %v", result, err)
	}
	if len(repository.added.PortableArchive) == 0 || repository.added.ContentHash != result.ContentHash {
		t.Fatalf("stored artifact = %+v", repository.added)
	}
	repeated, err := service.Install(context.Background(), PackInstallRequest{Source: PackSource{Path: "backend.zip"}})
	if err != nil || repeated.Installed {
		t.Fatalf("idempotent Install() = %+v, %v", repeated, err)
	}

	validator.results["backend.zip"] = validationFixture(rootPack, "c")
	_, err = service.Install(context.Background(), PackInstallRequest{Source: PackSource{Path: "backend.zip"}})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("changed immutable version error = %v", err)
	}
}

func TestPackInstallerV1RejectsMissingDependencyAndInvalidArtifact(t *testing.T) {
	t.Parallel()
	pack := installationPack(t, "pack.backend.missing", "1.0.0", dependency(t, "pack.missing", "1.0.0"))
	repository := newInstallationRepositoryFake()
	service := NewPackInstallerV1(validationFake{results: map[string]PackValidationResult{"pack.zip": validationFixture(pack, "d")}}, NewPackDependencyResolverV1(), repository, fixedCurriculumClock(t, 20))
	_, err := service.Install(context.Background(), PackInstallRequest{Source: PackSource{Path: "pack.zip"}})
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("missing dependency error = %v", err)
	}

	service.validator = validationFake{results: map[string]PackValidationResult{"bad.zip": {Errors: []PackValidationIssue{{Code: "invalid_pack", Path: "pack", Message: "checksum mismatch"}}}}}
	_, err = service.Install(context.Background(), PackInstallRequest{Source: PackSource{Path: "bad.zip"}})
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("invalid artifact error = %v", err)
	}
}

func TestPackInstallerV1ActivatesPerWorkspaceAndListsDeterministically(t *testing.T) {
	t.Parallel()
	first := installationPack(t, "pack.backend", "1.0.0")
	second := installationPack(t, "pack.backend", "1.1.0")
	repository := newInstallationRepositoryFake()
	repository.values[installationKey(first)] = installedFixture(t, first, "e")
	repository.values[installationKey(second)] = installedFixture(t, second, "f")
	service := NewPackInstallerV1(validationFake{}, NewPackDependencyResolverV1(), repository, fixedCurriculumClock(t, 30))

	listed, err := service.Find(context.Background(), first.Manifest.ID)
	if err != nil || len(listed) != 2 || listed[0].Pack.Manifest.Version.String() != "1.1.0" {
		t.Fatalf("Find() = %+v, %v", listed, err)
	}
	activation, err := service.Activate(context.Background(), PackActivateRequest{WorkspaceRoot: "/workspace/one", PackID: first.Manifest.ID, Version: first.Manifest.Version})
	if err != nil || repository.active["/workspace/one"].PackID != first.Manifest.ID || activation.PackID != first.Manifest.ID {
		t.Fatalf("Activate() = %+v, %v; active=%+v", activation, err, repository.active)
	}
	if _, err := service.Active(context.Background(), "/workspace/two"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other workspace Active() error = %v", err)
	}
}

type validationFake struct {
	results map[string]PackValidationResult
}

func (fake validationFake) Validate(_ context.Context, source PackSource) (PackValidationResult, error) {
	return fake.results[source.Path], nil
}

type fixedClock struct{ timestamp curriculum.Timestamp }

func fixedCurriculumClock(t *testing.T, minute int) fixedClock {
	t.Helper()
	timestamp, _ := curriculum.NewTimestamp(time.Date(2026, 9, 14, 12, minute, 0, 0, time.UTC))
	return fixedClock{timestamp: timestamp}
}
func (clock fixedClock) Now() curriculum.Timestamp { return clock.timestamp }

type installationRepositoryFake struct {
	values map[string]InstalledPack
	active map[string]PackActivation
	added  PackInstallationArtifact
}

func newInstallationRepositoryFake() *installationRepositoryFake {
	return &installationRepositoryFake{values: map[string]InstalledPack{}, active: map[string]PackActivation{}}
}
func (fake *installationRepositoryFake) Add(_ context.Context, artifact PackInstallationArtifact) error {
	key := installationKey(artifact.Pack)
	if _, exists := fake.values[key]; exists {
		return Classify(ErrorConflict, "fake add", nil)
	}
	fake.added = artifact
	fake.values[key] = artifact.InstalledPack
	return nil
}
func (fake *installationRepositoryFake) Get(_ context.Context, id curriculum.ID, version curriculum.PackVersion) (InstalledPack, error) {
	value, exists := fake.values[id.String()+"@"+version.String()]
	if !exists {
		return InstalledPack{}, Classify(ErrorNotFound, "fake get", nil)
	}
	return value, nil
}
func (fake *installationRepositoryFake) List(context.Context) ([]InstalledPack, error) {
	values := make([]InstalledPack, 0, len(fake.values))
	for _, value := range fake.values {
		values = append(values, value)
	}
	return values, nil
}
func (fake *installationRepositoryFake) Activate(_ context.Context, workspace string, activation PackActivation) error {
	fake.active[workspace] = activation
	return nil
}
func (fake *installationRepositoryFake) Active(_ context.Context, workspace string) (InstalledPack, error) {
	activation, exists := fake.active[workspace]
	if !exists {
		return InstalledPack{}, Classify(ErrorNotFound, "fake active", nil)
	}
	return fake.Get(context.Background(), activation.PackID, activation.Version)
}

func installationPack(t *testing.T, id, version string, dependencies ...curriculum.PackDependency) curriculum.LearningPack {
	t.Helper()
	request := compilerFixture(t)
	compiled, err := NewCurriculumCompilerV1().Compile(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	manifest := dependencyManifest(t, id, version, dependencies...)
	manifest.CurriculumID = compiled.Curriculum.ID
	return curriculum.LearningPack{Manifest: manifest, Curriculum: compiled.Curriculum}
}

func validationFixture(pack curriculum.LearningPack, marker string) PackValidationResult {
	return PackValidationResult{Pack: &pack, ContentHash: "sha256:" + strings.Repeat(marker, 64), PortableArchive: []byte("archive-" + marker)}
}

func installedFixture(t *testing.T, pack curriculum.LearningPack, marker string) InstalledPack {
	t.Helper()
	return InstalledPack{Pack: pack, ContentHash: "sha256:" + strings.Repeat(marker, 64), InstalledAt: fixedCurriculumClock(t, 10).Now()}
}

func installationKey(pack curriculum.LearningPack) string {
	return pack.Manifest.ID.String() + "@" + pack.Manifest.Version.String()
}
