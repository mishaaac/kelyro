// Package researchdb binds Research application services to a workspace-local
// SQLite database without exposing storage details to presentation packages.
package researchdb

import (
	"context"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
)

type Factory struct {
	appVersion string
	backup     sqlite.BackupFunc
}

func NewFactory(appVersion string) *Factory { return &Factory{appVersion: appVersion} }

func (factory *Factory) WithMigrationBackup(create sqlite.BackupFunc) *Factory {
	factory.backup = create
	return factory
}

func (factory *Factory) Open(ctx context.Context, workspaceRoot string) (application.SourceRegistryStore, error) {
	database, err := sqlite.Open(ctx, workspaceRoot, sqlite.WithAppVersion(factory.appVersion), sqlite.WithDestructiveMigrationBackup(factory.backup))
	if err != nil {
		return nil, err
	}
	registry := application.NewSourceRegistryService(database.Repositories().Research.SourceRegistry)
	trust := application.NewTrustDecisionService(database.Repositories().Research.TrustRegistry)
	trustRepository := database.Repositories().Research.TrustRegistry
	sources := application.NewSourceService(database.Repositories().Research.Sources, database.Repositories().Research.Snapshots)
	snapshots := application.NewSnapshotCaptureService(database.Repositories().Research.Sources, database.Repositories().Research.Snapshots, nil)
	evidence := database.Repositories().Research.Evidence
	claims := database.Repositories().Research.Claims
	citations := database.Repositories().Research.Citations
	releases := database.Repositories().Research.Releases
	deprecations := database.Repositories().Research.Deprecations
	provenance := application.NewProvenanceService(database.Repositories().Research.Provenance)
	freshness := application.NewFreshnessService(database.Repositories().Research.Freshness)
	researchService := application.NewResearchService(database.Repositories().Research.Runs)
	verifications := application.NewVerificationService(
		database.Repositories().Research.Verification,
		database.Repositories().Research.Claims,
		database.Repositories().Research.Sources,
		database.Repositories().Research.TrustRegistry,
		database.Repositories().Research.SourceRegistry,
		database.Repositories().Research.Conflicts,
		systemClock{},
	)
	diversityService := application.NewSourceDiversityService(
		database.Repositories().Research.Claims,
		database.Repositories().Research.Sources,
		database.Repositories().Research.TrustRegistry,
		database.Repositories().Research.SourceRegistry,
	)
	costs := application.NewResearchCostService(database.Repositories().Research.Costs)
	triggers := application.NewResearchTriggerService(database.Repositories().Research.TriggerQueue)
	updateScan := application.NewUpdateScanService(
		database.Repositories().Research.Sources,
		database.Repositories().Research.Snapshots,
		database.Repositories().Research.Releases,
		database.Repositories().Research.Deprecations,
		database.Repositories().Research.Freshness,
		database.Repositories().Research.Conflicts,
		nil,
	)
	bundles := application.NewSourceBundleService(
		database.Repositories().Research.Bundles,
		database.Repositories().Research.Runs,
		database.Repositories().Research.Claims,
		database.Repositories().Research.Sources,
		database.Repositories().Research.Evidence,
		database.Repositories().Research.TrustRegistry,
		database.Repositories().Research.Verification,
		database.Repositories().Research.Conflicts,
		database.Repositories().Research.Freshness,
		systemClock{},
	)
	conflicts := application.NewConflictResolutionService(database.Repositories().Research.Conflicts, nil, nil, nil, nil)
	return &store{database: database, sources: sources, snapshots: snapshots, evidence: evidence, claims: claims, citations: citations, releases: releases, deprecations: deprecations, registry: registry, trust: trust, trustRepository: trustRepository, provenance: provenance, freshness: freshness, research: researchService, verifications: verifications, diversity: diversityService, bundles: bundles, conflicts: conflicts, costs: costs, triggers: triggers, updateScan: updateScan}, nil
}

type store struct {
	database        *sqlite.Database
	sources         application.SourceService
	snapshots       application.SnapshotCaptureService
	evidence        application.EvidenceRepository
	claims          application.ClaimRepository
	citations       application.CitationRepository
	releases        application.ReleaseRepository
	deprecations    application.DeprecationRepository
	registry        application.SourceRegistryService
	trust           application.TrustDecisionService
	trustRepository application.TrustRegistryRepository
	provenance      application.ProvenanceService
	freshness       application.FreshnessService
	research        application.ResearchService
	verifications   application.VerificationService
	diversity       application.SourceDiversityService
	bundles         application.SourceBundleService
	conflicts       application.ConflictResolutionService
	costs           application.ResearchCostService
	triggers        application.ResearchTriggerService
	updateScan      application.UpdateScanService
}

func (store *store) Sources() application.SourceService               { return store.sources }
func (store *store) Snapshots() application.SnapshotCaptureService    { return store.snapshots }
func (store *store) Evidence() application.EvidenceRepository         { return store.evidence }
func (store *store) Claims() application.ClaimRepository              { return store.claims }
func (store *store) Citations() application.CitationRepository        { return store.citations }
func (store *store) Releases() application.ReleaseRepository          { return store.releases }
func (store *store) Deprecations() application.DeprecationRepository  { return store.deprecations }
func (store *store) Registry() application.SourceRegistryService      { return store.registry }
func (store *store) TrustDecisions() application.TrustDecisionService { return store.trust }
func (store *store) TrustRepository() application.TrustRegistryRepository {
	return store.trustRepository
}
func (store *store) Provenance() application.ProvenanceService        { return store.provenance }
func (store *store) Freshness() application.FreshnessService          { return store.freshness }
func (store *store) Research() application.ResearchService            { return store.research }
func (store *store) Verifications() application.VerificationService   { return store.verifications }
func (store *store) Diversity() application.SourceDiversityService    { return store.diversity }
func (store *store) Bundles() application.SourceBundleService         { return store.bundles }
func (store *store) Conflicts() application.ConflictResolutionService { return store.conflicts }
func (store *store) Costs() application.ResearchCostService           { return store.costs }
func (store *store) Triggers() application.ResearchTriggerService     { return store.triggers }
func (store *store) UpdateScan() application.UpdateScanService        { return store.updateScan }

func (store *store) Close() error {
	if err := store.database.Close(); err != nil {
		return fmt.Errorf("close research database: %w", err)
	}
	return nil
}

var _ application.SourceRegistryStoreFactory = (*Factory)(nil)
var _ application.SourceRegistryStore = (*store)(nil)

type systemClock struct{}

func (systemClock) Now() research.Timestamp {
	timestamp, err := research.NewTimestamp(time.Now().UTC())
	if err != nil {
		return research.Timestamp{}
	}
	return timestamp
}
