package curriculumscale_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
	"github.com/mishaaac/kelyro/internal/infra/curriculumscale"
	"github.com/mishaaac/kelyro/internal/infra/learningpack"
)

func TestLargeCurriculumFixtureCompilesDeterministically(t *testing.T) {
	ctx := context.Background()
	fixtureStarted := time.Now()
	fixture, err := curriculumscale.Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("scale fixture build: %s", time.Since(fixtureStarted))

	compiler := curriculumapp.NewCurriculumCompilerV1()
	compileStarted := time.Now()
	first, err := compiler.Compile(ctx, fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("scale compiler pipeline: %s", time.Since(compileStarted))
	assertScaleCardinalities(t, first)
	logMeasuredPasses(t, first.Passes)

	repeatedStarted := time.Now()
	second, err := compiler.Compile(ctx, fixture.Request)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("scale compiler repeat: %s", time.Since(repeatedStarted))
	if !reflect.DeepEqual(first.Curriculum, second.Curriculum) ||
		!reflect.DeepEqual(first.Diagnostics, second.Diagnostics) ||
		first.BuildInfo.OutputHash != second.BuildInfo.OutputHash {
		t.Fatal("large compiler output is not deterministic")
	}

	buildRequest := fixture.PackBuildRequest(first)
	serializationStarted := time.Now()
	firstPack, err := learningpack.NewBuilder().Build(ctx, buildRequest)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("scale pack serialization: %s (%d archive bytes)", time.Since(serializationStarted), len(firstPack.PortableArchive))
	secondPack, err := learningpack.NewBuilder().Build(ctx, buildRequest)
	if err != nil {
		t.Fatal(err)
	}
	if firstPack.ContentHash != secondPack.ContentHash || !bytes.Equal(firstPack.PortableArchive, secondPack.PortableArchive) {
		t.Fatal("large pack serialization is not deterministic")
	}
	totalBytes, largestName, largestBytes := archiveSizes(t, firstPack.PortableArchive)
	t.Logf("scale pack uncompressed: %d bytes total; largest entry %s=%d bytes", totalBytes, largestName, largestBytes)

	archivePath := filepath.Join(t.TempDir(), "large-learning-pack.zip")
	if err := os.WriteFile(archivePath, firstPack.PortableArchive, 0o600); err != nil {
		t.Fatal(err)
	}
	repository := newPackRepository()
	installer := curriculumapp.NewPackInstallerV1(learningpack.NewValidator(), curriculumapp.NewPackDependencyResolverV1(), repository, scaleClock{at: first.BuildInfo.BuiltAt})
	installationStarted := time.Now()
	installed, err := installer.Install(ctx, curriculumapp.PackInstallRequest{Source: curriculumapp.PackSource{Path: archivePath}})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("scale pack validation and install: %s", time.Since(installationStarted))
	if !installed.Installed || len(installed.Pack.Curriculum.Concepts) != curriculumscale.ConceptCount {
		t.Fatalf("large pack install = %+v", installed)
	}
}

func BenchmarkLargeCurriculumCompiler(b *testing.B) {
	fixture, err := curriculumscale.Build(context.Background())
	if err != nil {
		b.Fatal(err)
	}
	compiler := curriculumapp.NewCurriculumCompilerV1()
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := compiler.Compile(context.Background(), fixture.Request); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLargeCurriculumPackSerialization(b *testing.B) {
	fixture, err := curriculumscale.Build(context.Background())
	if err != nil {
		b.Fatal(err)
	}
	compiled, err := curriculumapp.NewCurriculumCompilerV1().Compile(context.Background(), fixture.Request)
	if err != nil {
		b.Fatal(err)
	}
	request := fixture.PackBuildRequest(compiled)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, err := learningpack.NewBuilder().Build(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}

func assertScaleCardinalities(t *testing.T, result curriculum.CompilationResult) {
	t.Helper()
	if len(result.Curriculum.Concepts) != curriculumscale.ConceptCount ||
		len(result.Curriculum.Prerequisites) != curriculumscale.PrerequisiteCount ||
		len(result.Curriculum.Competencies.Competencies) != curriculumscale.CompetencyCount ||
		len(result.Curriculum.SourceBundles) != curriculumscale.SourceBundleCount {
		t.Fatalf("scale cardinalities: concepts=%d prerequisites=%d competencies=%d bundles=%d",
			len(result.Curriculum.Concepts), len(result.Curriculum.Prerequisites),
			len(result.Curriculum.Competencies.Competencies), len(result.Curriculum.SourceBundles))
	}
	if result.Diagnostics == nil || len(result.Diagnostics.Graph.TopologicalOrder) != curriculumscale.ConceptCount ||
		result.Diagnostics.Graph.CriticalPath.EdgeCount != curriculumscale.ConceptCount-1 {
		t.Fatalf("scale graph diagnostics = %+v", result.Diagnostics)
	}
}

func logMeasuredPasses(t *testing.T, passes []curriculum.CompilationPass) {
	t.Helper()
	measured := map[string]bool{
		"knowledge-graph": true, "coverage": true, "definition-before-use": true,
		"zero-assumption": true, "gap-scan": true, "beginner-simulation": true,
		"expert-coverage-review": true, "final-review": true,
	}
	for _, pass := range passes {
		if measured[pass.Name] {
			t.Logf("scale pass %s: %s", pass.Name, pass.Duration)
		}
	}
}

type scaleClock struct{ at curriculum.Timestamp }

func (clock scaleClock) Now() curriculum.Timestamp { return clock.at }

type packRepository struct {
	values map[string]curriculumapp.InstalledPack
}

func newPackRepository() *packRepository {
	return &packRepository{values: make(map[string]curriculumapp.InstalledPack)}
}

func (repository *packRepository) Add(_ context.Context, artifact curriculumapp.PackInstallationArtifact) error {
	key := packKey(artifact.Pack.Manifest.ID, artifact.Pack.Manifest.Version)
	if _, exists := repository.values[key]; exists {
		return curriculumapp.Classify(curriculumapp.ErrorConflict, "add scale pack", nil)
	}
	repository.values[key] = artifact.InstalledPack
	return nil
}

func (repository *packRepository) Get(_ context.Context, id curriculum.ID, version curriculum.PackVersion) (curriculumapp.InstalledPack, error) {
	value, exists := repository.values[packKey(id, version)]
	if !exists {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorNotFound, "get scale pack", errors.New("not installed"))
	}
	return value, nil
}

func (repository *packRepository) List(context.Context) ([]curriculumapp.InstalledPack, error) {
	result := make([]curriculumapp.InstalledPack, 0, len(repository.values))
	for _, value := range repository.values {
		result = append(result, value)
	}
	return result, nil
}

func (*packRepository) Activate(context.Context, string, curriculumapp.PackActivation) error {
	return nil
}

func (*packRepository) Active(context.Context, string) (curriculumapp.InstalledPack, error) {
	return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorNotFound, "active scale pack", errors.New("not active"))
}

func packKey(id curriculum.ID, version curriculum.PackVersion) string {
	return id.String() + "@" + version.String()
}

func archiveSizes(t *testing.T, data []byte) (uint64, string, uint64) {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var total, largest uint64
	var name string
	for _, entry := range archive.File {
		total += entry.UncompressedSize64
		if entry.UncompressedSize64 > largest {
			name, largest = entry.Name, entry.UncompressedSize64
		}
	}
	return total, name, largest
}
