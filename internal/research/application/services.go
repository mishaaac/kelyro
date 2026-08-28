package application

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
)

type researchService struct{ runs ResearchRunRepository }

func NewResearchService(runs ResearchRunRepository) ResearchService {
	return &researchService{runs: runs}
}

func (service *researchService) Start(ctx context.Context, request research.ResearchRequest, run research.ResearchRun) error {
	const operation = "start research run"
	if err := request.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := run.Validate(); err != nil {
		return invalid(operation, err)
	}
	if run.RequestID != request.ID {
		return invalid(operation, fmt.Errorf("run request relationship does not match"))
	}
	if err := requireDependency(operation, "research run repository", service.runs); err != nil {
		return err
	}
	return repositoryError(operation, service.runs.Create(ctx, request, run))
}

func (service *researchService) Run(ctx context.Context, id research.ID) (research.ResearchRun, error) {
	const operation = "get research run"
	if err := id.Validate(); err != nil {
		return research.ResearchRun{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "research run repository", service.runs); err != nil {
		return research.ResearchRun{}, err
	}
	run, err := service.runs.GetRun(ctx, id)
	return run, repositoryError(operation, err)
}

func (service *researchService) UpdateRun(ctx context.Context, run research.ResearchRun) error {
	const operation = "update research run"
	if err := run.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireDependency(operation, "research run repository", service.runs); err != nil {
		return err
	}
	return repositoryError(operation, service.runs.UpdateRun(ctx, run))
}

type discoveryService struct {
	provider SearchProvider
	cache    SearchCache
	access   networkResearchAccess
}

func NewDiscoveryService(provider SearchProvider, cache SearchCache, access NetworkResearchAccess) DiscoveryService {
	return &discoveryService{provider: provider, cache: cache, access: newNetworkResearchAccess(access)}
}

func (service *discoveryService) Search(ctx context.Context, mode ResearchMode, query SearchQuery, options SearchOptions) ([]SearchResult, error) {
	const operation = "search source candidates"
	query.Text = normalizeSearchText(query.Text)
	if err := query.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	if err := options.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	options = cloneSearchOptions(options)
	decision, err := service.access.decide(ctx, mode, NetworkOperationDiscovery)
	if err != nil {
		return nil, err
	}
	if !decision.live {
		if service.cache == nil {
			return nil, networkResearchBlocked(operation, decision.blocked, nil)
		}
		results, cacheErr := service.cache.SearchCached(ctx, query, options)
		if cacheErr != nil {
			return nil, offlineFallbackError(operation, decision.blocked, cacheErr)
		}
		return normalizeSearchResults(ctx, operation, results, options.Limit, true)
	}
	if err := requireDependency(operation, "search provider", service.provider); err != nil {
		return nil, err
	}
	results, err := service.provider.Search(ctx, query, options)
	if err != nil {
		return nil, externalError(operation, err)
	}
	return normalizeSearchResults(ctx, operation, results, options.Limit, false)
}

func normalizeSearchResults(ctx context.Context, operation string, results []SearchResult, limit int, cached bool) ([]SearchResult, error) {
	if len(results) > MaximumSearchResults {
		return nil, searchResultError(operation, fmt.Errorf("provider returned more than %d results", MaximumSearchResults), cached)
	}
	normalized := make([]SearchResult, 0, min(len(results), limit))
	seen := make(map[string]struct{}, len(results))
	for index, result := range results {
		if err := ctx.Err(); err != nil {
			return nil, Classify(ErrorUnavailable, operation, err)
		}
		result.Title = normalizeSearchText(result.Title)
		result.Snippet = normalizeSearchText(result.Snippet)
		result.Provider = normalizeSearchText(result.Provider)
		if cached {
			result.CacheHit = true
		} else {
			result.CacheHit = false
			result.CacheStale = false
			result.CacheWarning = ""
		}
		locator, err := normalizeDiscoveryLocator(result.Locator)
		if err != nil {
			return nil, searchResultError(operation, fmt.Errorf("result %d: %w", index, err), cached)
		}
		result.Locator = locator
		if err := result.Validate(); err != nil {
			return nil, searchResultError(operation, fmt.Errorf("result %d: %w", index, err), cached)
		}
		key := result.Locator.String()
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		if len(normalized) < limit {
			normalized = append(normalized, cloneSearchResult(result))
		}
	}
	return normalized, nil
}

func normalizeSearchText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func normalizeDiscoveryLocator(locator research.SourceLocator) (research.SourceLocator, error) {
	parsed, err := url.Parse(locator.String())
	if err != nil {
		return research.SourceLocator{}, fmt.Errorf("parse search result locator: %w", err)
	}
	parsed.Fragment = ""
	normalized, err := research.NewSourceLocator(parsed.String())
	if err != nil {
		return research.SourceLocator{}, fmt.Errorf("search result locator: %w", err)
	}
	return normalized, nil
}

func cloneSearchResult(result SearchResult) SearchResult {
	clone := result
	if result.PublishedHint != nil {
		published := *result.PublishedHint
		clone.PublishedHint = &published
	}
	return clone
}

func cloneSearchOptions(options SearchOptions) SearchOptions {
	clone := options
	if options.DesiredKind != nil {
		kind := *options.DesiredKind
		clone.DesiredKind = &kind
	}
	if options.TargetVersion != nil {
		version := *options.TargetVersion
		clone.TargetVersion = &version
	}
	return clone
}

func searchResultError(operation string, cause error, cached bool) error {
	if cached {
		return repositoryError(operation, cause)
	}
	return externalError(operation, cause)
}

type sourceService struct {
	sources   SourceRepository
	snapshots SnapshotRepository
}

func NewSourceService(sources SourceRepository, snapshots SnapshotRepository) SourceService {
	return &sourceService{sources: sources, snapshots: snapshots}
}

func (service *sourceService) Register(ctx context.Context, source research.Source) error {
	const operation = "register source"
	if err := source.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireDependency(operation, "source repository", service.sources); err != nil {
		return err
	}
	return repositoryError(operation, service.sources.Create(ctx, source))
}

func (service *sourceService) Get(ctx context.Context, id research.SourceID) (research.Source, error) {
	const operation = "get source"
	if err := id.Validate(); err != nil {
		return research.Source{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "source repository", service.sources); err != nil {
		return research.Source{}, err
	}
	source, err := service.sources.Get(ctx, id)
	return source, repositoryError(operation, err)
}

func (service *sourceService) List(ctx context.Context) ([]research.Source, error) {
	const operation = "list sources"
	if err := requireDependency(operation, "source repository", service.sources); err != nil {
		return nil, err
	}
	sources, err := service.sources.List(ctx)
	return sources, repositoryError(operation, err)
}

func (service *sourceService) RecordSnapshot(ctx context.Context, snapshot research.SourceSnapshot) error {
	const operation = "record source snapshot"
	if err := snapshot.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireDependency(operation, "source repository", service.sources); err != nil {
		return err
	}
	if err := requireDependency(operation, "snapshot repository", service.snapshots); err != nil {
		return err
	}
	if _, err := service.sources.Get(ctx, snapshot.SourceID); err != nil {
		return repositoryError(operation, err)
	}
	return repositoryError(operation, service.snapshots.Append(ctx, snapshot))
}

func (service *sourceService) LatestSnapshot(ctx context.Context, sourceID research.SourceID) (research.SourceSnapshot, error) {
	const operation = "get latest source snapshot"
	if err := sourceID.Validate(); err != nil {
		return research.SourceSnapshot{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "snapshot repository", service.snapshots); err != nil {
		return research.SourceSnapshot{}, err
	}
	snapshot, err := service.snapshots.LatestBySource(ctx, sourceID)
	return snapshot, repositoryError(operation, err)
}

func (service *sourceService) ClassifyTemporalScope(ctx context.Context, sourceID research.SourceID, scope research.SourceTemporalScope) error {
	const operation = "classify source temporal scope"
	if err := sourceID.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := scope.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireDependency(operation, "source repository", service.sources); err != nil {
		return err
	}
	return repositoryError(operation, service.sources.SetTemporalScope(ctx, sourceID, scope))
}

type sourceRegistryService struct{ repository SourceRegistryRepository }

func NewSourceRegistryService(repository SourceRegistryRepository) SourceRegistryService {
	return &sourceRegistryService{repository: repository}
}

func (service *sourceRegistryService) Save(ctx context.Context, entry research.SourceRegistryEntry) error {
	const operation = "save source registry entry"
	if err := entry.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireDependency(operation, "source registry repository", service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.Save(ctx, entry))
}

func (service *sourceRegistryService) Get(ctx context.Context, id research.ID) (research.SourceRegistryEntry, error) {
	const operation = "get source registry entry"
	if err := id.Validate(); err != nil {
		return research.SourceRegistryEntry{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "source registry repository", service.repository); err != nil {
		return research.SourceRegistryEntry{}, err
	}
	entry, err := service.repository.Get(ctx, id)
	return entry, repositoryError(operation, err)
}

func (service *sourceRegistryService) List(ctx context.Context) ([]research.SourceRegistryEntry, error) {
	const operation = "list source registry entries"
	if err := requireDependency(operation, "source registry repository", service.repository); err != nil {
		return nil, err
	}
	entries, err := service.repository.List(ctx)
	return entries, repositoryError(operation, err)
}

type provenanceService struct{ repository ProvenanceRepository }

func NewProvenanceService(repository ProvenanceRepository) ProvenanceService {
	return &provenanceService{repository: repository}
}

func (service *provenanceService) Record(ctx context.Context, graph research.ProvenanceGraph) error {
	const operation = "record provenance graph"
	if err := graph.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireDependency(operation, "provenance repository", service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.Append(ctx, graph))
}

func (service *provenanceService) Trace(ctx context.Context, claimID research.ClaimID) (research.ProvenanceGraph, error) {
	const operation = "trace claim provenance"
	if err := claimID.Validate(); err != nil {
		return research.ProvenanceGraph{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "provenance repository", service.repository); err != nil {
		return research.ProvenanceGraph{}, err
	}
	graph, err := service.repository.LatestByClaim(ctx, claimID)
	return graph, repositoryError(operation, err)
}

func (service *provenanceService) Export(ctx context.Context, claimID research.ClaimID) ([]byte, error) {
	const operation = "export claim provenance"
	graph, err := service.Trace(ctx, claimID)
	if err != nil {
		return nil, err
	}
	encoded, err := graph.ExportJSON()
	if err != nil {
		return nil, invalid(operation, err)
	}
	return encoded, nil
}

type freshnessService struct{ repository FreshnessRepository }

func NewFreshnessService(repository FreshnessRepository) FreshnessService {
	return &freshnessService{repository: repository}
}

func (service *freshnessService) Save(ctx context.Context, record FreshnessRecord) error {
	const operation = "save freshness state"
	if err := record.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireDependency(operation, "freshness repository", service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.Save(ctx, record))
}

func (service *freshnessService) Get(ctx context.Context, subjectID research.ID) (FreshnessRecord, error) {
	const operation = "get freshness state"
	if err := subjectID.Validate(); err != nil {
		return FreshnessRecord{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "freshness repository", service.repository); err != nil {
		return FreshnessRecord{}, err
	}
	record, err := service.repository.Get(ctx, subjectID)
	return record, repositoryError(operation, err)
}

func (service *freshnessService) Due(ctx context.Context, asOf research.Timestamp) ([]FreshnessRecord, error) {
	const operation = "list freshness due"
	if err := asOf.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	if err := requireDependency(operation, "freshness repository", service.repository); err != nil {
		return nil, err
	}
	records, err := service.repository.ListDue(ctx, asOf)
	return records, repositoryError(operation, err)
}

type releaseIntelligenceService struct{ repository ReleaseRepository }

func NewReleaseIntelligenceService(repository ReleaseRepository) ReleaseIntelligenceService {
	return &releaseIntelligenceService{repository: repository}
}

func (service *releaseIntelligenceService) Record(ctx context.Context, record research.ReleaseRecord) error {
	const operation = "record technology release"
	if err := record.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireDependency(operation, "release repository", service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.Create(ctx, record))
}

func (service *releaseIntelligenceService) Get(ctx context.Context, id research.ID) (research.ReleaseRecord, error) {
	const operation = "get technology release"
	if err := id.Validate(); err != nil {
		return research.ReleaseRecord{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "release repository", service.repository); err != nil {
		return research.ReleaseRecord{}, err
	}
	record, err := service.repository.Get(ctx, id)
	return record, repositoryError(operation, err)
}

func (service *releaseIntelligenceService) List(ctx context.Context, technologyID research.ID) ([]research.ReleaseRecord, error) {
	const operation = "list technology releases"
	if err := technologyID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	if err := requireDependency(operation, "release repository", service.repository); err != nil {
		return nil, err
	}
	records, err := service.repository.ListByTechnology(ctx, technologyID)
	return records, repositoryError(operation, err)
}

type driftService struct{ repository DriftRepository }

func NewDriftService(repository DriftRepository) DriftService {
	return &driftService{repository: repository}
}

func (service *driftService) Record(ctx context.Context, report research.DriftReport) error {
	const operation = "record drift report"
	if err := report.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireDependency(operation, "drift repository", service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.Append(ctx, report))
}

func (service *driftService) Get(ctx context.Context, id research.ID) (research.DriftReport, error) {
	const operation = "get drift report"
	if err := id.Validate(); err != nil {
		return research.DriftReport{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "drift repository", service.repository); err != nil {
		return research.DriftReport{}, err
	}
	report, err := service.repository.Get(ctx, id)
	return report, repositoryError(operation, err)
}

type impactService struct{ repository ImpactRepository }

func NewImpactService(repository ImpactRepository) ImpactService {
	return &impactService{repository: repository}
}

func (service *impactService) Record(ctx context.Context, report research.ImpactReport) error {
	const operation = "record impact report"
	if err := report.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireDependency(operation, "impact repository", service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.Append(ctx, report))
}

func (service *impactService) Get(ctx context.Context, id research.ID) (research.ImpactReport, error) {
	const operation = "get impact report"
	if err := id.Validate(); err != nil {
		return research.ImpactReport{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "impact repository", service.repository); err != nil {
		return research.ImpactReport{}, err
	}
	report, err := service.repository.Get(ctx, id)
	return report, repositoryError(operation, err)
}
