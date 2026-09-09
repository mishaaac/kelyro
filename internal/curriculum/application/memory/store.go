// Package memory provides deterministic in-memory fakes for Curriculum
// Compiler application tests. It is not a production persistence adapter.
package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/mishaaac/kelyro/internal/curriculum"
	"github.com/mishaaac/kelyro/internal/curriculum/application"
)

type Store struct {
	mu           sync.RWMutex
	curricula    map[string]curriculum.CurriculumDefinition
	packs        map[string]curriculum.LearningPack
	activePack   string
	catalog      []curriculum.PackManifest
	environments map[string]curriculum.EnvironmentPack
	compilations map[curriculum.ID]application.CompilationRecord
}

func NewStore() *Store {
	return &Store{
		curricula:    make(map[string]curriculum.CurriculumDefinition),
		packs:        make(map[string]curriculum.LearningPack),
		environments: make(map[string]curriculum.EnvironmentPack),
		compilations: make(map[curriculum.ID]application.CompilationRecord),
	}
}

func (store *Store) Add(ctx context.Context, value curriculum.CurriculumDefinition) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := value.Validate(); err != nil {
		return application.Invalid("add curriculum", err)
	}
	key := versionKey(value.ID.String(), value.Version.String())
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.curricula[key]; exists {
		return application.Classify(application.ErrorConflict, "add curriculum", fmt.Errorf("curriculum version already exists"))
	}
	store.curricula[key] = cloneDefinition(value)
	return nil
}

func (store *Store) Get(ctx context.Context, id curriculum.CurriculumID, version curriculum.CurriculumVersion) (curriculum.CurriculumDefinition, error) {
	if err := checkContext(ctx); err != nil {
		return curriculum.CurriculumDefinition{}, err
	}
	if err := id.Validate(); err != nil {
		return curriculum.CurriculumDefinition{}, application.Invalid("get curriculum", err)
	}
	if err := version.Validate(); err != nil {
		return curriculum.CurriculumDefinition{}, application.Invalid("get curriculum", err)
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	value, exists := store.curricula[versionKey(id.String(), version.String())]
	if !exists {
		return curriculum.CurriculumDefinition{}, application.Classify(application.ErrorNotFound, "get curriculum", nil)
	}
	return cloneDefinition(value), nil
}

func (store *Store) List(ctx context.Context) ([]curriculum.CurriculumDefinition, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	values := make([]curriculum.CurriculumDefinition, 0, len(store.curricula))
	for _, value := range store.curricula {
		values = append(values, cloneDefinition(value))
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].ID.String() != values[j].ID.String() {
			return values[i].ID.String() < values[j].ID.String()
		}
		return values[i].Version.String() < values[j].Version.String()
	})
	return values, nil
}

func (store *Store) AddPack(ctx context.Context, value curriculum.LearningPack) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := value.Validate(); err != nil {
		return application.Invalid("add pack", err)
	}
	key := versionKey(value.Manifest.ID.String(), value.Manifest.Version.String())
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.packs[key]; exists {
		return application.Classify(application.ErrorConflict, "add pack", fmt.Errorf("pack version already exists"))
	}
	store.packs[key] = clonePack(value)
	return nil
}

func (store *Store) GetPack(ctx context.Context, id curriculum.ID, version curriculum.PackVersion) (curriculum.LearningPack, error) {
	if err := checkContext(ctx); err != nil {
		return curriculum.LearningPack{}, err
	}
	if err := id.Validate(); err != nil {
		return curriculum.LearningPack{}, application.Invalid("get pack", err)
	}
	if err := version.Validate(); err != nil {
		return curriculum.LearningPack{}, application.Invalid("get pack", err)
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	value, exists := store.packs[versionKey(id.String(), version.String())]
	if !exists {
		return curriculum.LearningPack{}, application.Classify(application.ErrorNotFound, "get pack", nil)
	}
	return clonePack(value), nil
}

func (store *Store) ListPacks(ctx context.Context) ([]curriculum.LearningPack, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	values := make([]curriculum.LearningPack, 0, len(store.packs))
	for _, value := range store.packs {
		values = append(values, clonePack(value))
	}
	sort.Slice(values, func(i, j int) bool { return packKey(values[i]) < packKey(values[j]) })
	return values, nil
}

func (store *Store) Activate(ctx context.Context, activation application.PackActivation) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := activation.Validate(); err != nil {
		return application.Invalid("activate pack", err)
	}
	key := versionKey(activation.PackID.String(), activation.Version.String())
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.packs[key]; !exists {
		return application.Classify(application.ErrorNotFound, "activate pack", nil)
	}
	store.activePack = key
	return nil
}

func (store *Store) Active(ctx context.Context) (curriculum.LearningPack, error) {
	if err := checkContext(ctx); err != nil {
		return curriculum.LearningPack{}, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	value, exists := store.packs[store.activePack]
	if !exists {
		return curriculum.LearningPack{}, application.Classify(application.ErrorNotFound, "get active pack", nil)
	}
	return clonePack(value), nil
}

func (store *Store) ReplaceCatalog(ctx context.Context, manifests []curriculum.PackManifest) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	values := make([]curriculum.PackManifest, len(manifests))
	seen := make(map[string]struct{}, len(manifests))
	for index, manifest := range manifests {
		if err := manifest.Validate(); err != nil {
			return application.Invalid("replace pack catalog", err)
		}
		key := versionKey(manifest.ID.String(), manifest.Version.String())
		if _, exists := seen[key]; exists {
			return application.Classify(application.ErrorConflict, "replace pack catalog", fmt.Errorf("duplicate catalog version"))
		}
		seen[key] = struct{}{}
		values[index] = cloneManifest(manifest)
	}
	sort.Slice(values, func(i, j int) bool { return manifestKey(values[i]) < manifestKey(values[j]) })
	store.mu.Lock()
	store.catalog = values
	store.mu.Unlock()
	return nil
}

func (store *Store) ListCatalog(ctx context.Context) ([]curriculum.PackManifest, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	return cloneManifests(store.catalog), nil
}

func (store *Store) FindCatalogByID(ctx context.Context, id curriculum.ID) ([]curriculum.PackManifest, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if err := id.Validate(); err != nil {
		return nil, application.Invalid("find catalog pack", err)
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	values := make([]curriculum.PackManifest, 0)
	for _, manifest := range store.catalog {
		if manifest.ID == id {
			values = append(values, cloneManifest(manifest))
		}
	}
	if len(values) == 0 {
		return nil, application.Classify(application.ErrorNotFound, "find catalog pack", nil)
	}
	return values, nil
}

func (store *Store) AddEnvironment(ctx context.Context, value curriculum.EnvironmentPack) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := value.Validate(); err != nil {
		return application.Invalid("add environment pack", err)
	}
	key := versionKey(value.ID.String(), value.Version.String())
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.environments[key]; exists {
		return application.Classify(application.ErrorConflict, "add environment pack", nil)
	}
	store.environments[key] = cloneEnvironment(value)
	return nil
}

func (store *Store) GetEnvironment(ctx context.Context, id curriculum.ID, version curriculum.PackVersion) (curriculum.EnvironmentPack, error) {
	if err := checkContext(ctx); err != nil {
		return curriculum.EnvironmentPack{}, err
	}
	if err := id.Validate(); err != nil {
		return curriculum.EnvironmentPack{}, application.Invalid("get environment pack", err)
	}
	if err := version.Validate(); err != nil {
		return curriculum.EnvironmentPack{}, application.Invalid("get environment pack", err)
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	value, exists := store.environments[versionKey(id.String(), version.String())]
	if !exists {
		return curriculum.EnvironmentPack{}, application.Classify(application.ErrorNotFound, "get environment pack", nil)
	}
	return cloneEnvironment(value), nil
}

func (store *Store) ListEnvironments(ctx context.Context) ([]curriculum.EnvironmentPack, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	values := make([]curriculum.EnvironmentPack, 0, len(store.environments))
	for _, value := range store.environments {
		values = append(values, cloneEnvironment(value))
	}
	sort.Slice(values, func(i, j int) bool {
		return versionKey(values[i].ID.String(), values[i].Version.String()) < versionKey(values[j].ID.String(), values[j].Version.String())
	})
	return values, nil
}

func (store *Store) AppendCompilation(ctx context.Context, record application.CompilationRecord) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := record.Validate(); err != nil {
		return application.Invalid("append compilation", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.compilations[record.ID]; exists {
		return application.Classify(application.ErrorConflict, "append compilation", nil)
	}
	store.compilations[record.ID] = cloneCompilation(record)
	return nil
}

func (store *Store) GetCompilation(ctx context.Context, id curriculum.ID) (application.CompilationRecord, error) {
	if err := checkContext(ctx); err != nil {
		return application.CompilationRecord{}, err
	}
	if err := id.Validate(); err != nil {
		return application.CompilationRecord{}, application.Invalid("get compilation", err)
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	value, exists := store.compilations[id]
	if !exists {
		return application.CompilationRecord{}, application.Classify(application.ErrorNotFound, "get compilation", nil)
	}
	return cloneCompilation(value), nil
}

func (store *Store) ListCompilationsByCurriculum(ctx context.Context, id curriculum.CurriculumID) ([]application.CompilationRecord, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if err := id.Validate(); err != nil {
		return nil, application.Invalid("list curriculum compilations", err)
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	values := make([]application.CompilationRecord, 0)
	for _, record := range store.compilations {
		if record.Result.Curriculum.ID == id {
			values = append(values, cloneCompilation(record))
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ID.String() < values[j].ID.String() })
	return values, nil
}

// Narrow wrappers expose one Store through the five repository interfaces
// without relying on Go's inability to overload Add/Get/List method names.
type CurriculumRepository struct{ Store *Store }

func (repository CurriculumRepository) Add(ctx context.Context, value curriculum.CurriculumDefinition) error {
	return repository.Store.Add(ctx, value)
}
func (repository CurriculumRepository) Get(ctx context.Context, id curriculum.CurriculumID, version curriculum.CurriculumVersion) (curriculum.CurriculumDefinition, error) {
	return repository.Store.Get(ctx, id, version)
}
func (repository CurriculumRepository) List(ctx context.Context) ([]curriculum.CurriculumDefinition, error) {
	return repository.Store.List(ctx)
}

type PackRepository struct{ Store *Store }

func (repository PackRepository) Add(ctx context.Context, value curriculum.LearningPack) error {
	return repository.Store.AddPack(ctx, value)
}
func (repository PackRepository) Get(ctx context.Context, id curriculum.ID, version curriculum.PackVersion) (curriculum.LearningPack, error) {
	return repository.Store.GetPack(ctx, id, version)
}
func (repository PackRepository) List(ctx context.Context) ([]curriculum.LearningPack, error) {
	return repository.Store.ListPacks(ctx)
}
func (repository PackRepository) Activate(ctx context.Context, value application.PackActivation) error {
	return repository.Store.Activate(ctx, value)
}
func (repository PackRepository) Active(ctx context.Context) (curriculum.LearningPack, error) {
	return repository.Store.Active(ctx)
}

type PackCatalogRepository struct{ Store *Store }

func (repository PackCatalogRepository) Replace(ctx context.Context, values []curriculum.PackManifest) error {
	return repository.Store.ReplaceCatalog(ctx, values)
}
func (repository PackCatalogRepository) List(ctx context.Context) ([]curriculum.PackManifest, error) {
	return repository.Store.ListCatalog(ctx)
}
func (repository PackCatalogRepository) FindByID(ctx context.Context, id curriculum.ID) ([]curriculum.PackManifest, error) {
	return repository.Store.FindCatalogByID(ctx, id)
}

type EnvironmentPackRepository struct{ Store *Store }

func (repository EnvironmentPackRepository) Add(ctx context.Context, value curriculum.EnvironmentPack) error {
	return repository.Store.AddEnvironment(ctx, value)
}
func (repository EnvironmentPackRepository) Get(ctx context.Context, id curriculum.ID, version curriculum.PackVersion) (curriculum.EnvironmentPack, error) {
	return repository.Store.GetEnvironment(ctx, id, version)
}
func (repository EnvironmentPackRepository) List(ctx context.Context) ([]curriculum.EnvironmentPack, error) {
	return repository.Store.ListEnvironments(ctx)
}

type CompilationRepository struct{ Store *Store }

func (repository CompilationRepository) Append(ctx context.Context, value application.CompilationRecord) error {
	return repository.Store.AppendCompilation(ctx, value)
}
func (repository CompilationRepository) Get(ctx context.Context, id curriculum.ID) (application.CompilationRecord, error) {
	return repository.Store.GetCompilation(ctx, id)
}
func (repository CompilationRepository) ListByCurriculum(ctx context.Context, id curriculum.CurriculumID) ([]application.CompilationRecord, error) {
	return repository.Store.ListCompilationsByCurriculum(ctx, id)
}

func checkContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return application.Classify(application.ErrorUnavailable, "memory repository", err)
	}
	return nil
}

func versionKey(id, version string) string         { return id + "\x00" + version }
func packKey(value curriculum.LearningPack) string { return manifestKey(value.Manifest) }
func manifestKey(value curriculum.PackManifest) string {
	return versionKey(value.ID.String(), value.Version.String())
}

var (
	_ application.CurriculumRepository      = CurriculumRepository{}
	_ application.PackRepository            = PackRepository{}
	_ application.PackCatalogRepository     = PackCatalogRepository{}
	_ application.EnvironmentPackRepository = EnvironmentPackRepository{}
	_ application.CompilationRepository     = CompilationRepository{}
)
