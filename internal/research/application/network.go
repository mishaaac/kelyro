package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
)

// ResearchMode selects whether a use case may attempt live network access.
// No mode overrides the resolved Foundation privacy policy.
type ResearchMode string

const (
	ResearchModeOffline ResearchMode = "offline"
	ResearchModeOnline  ResearchMode = "online"
	ResearchModeAuto    ResearchMode = "auto"
)

func (mode ResearchMode) Validate() error {
	switch mode {
	case ResearchModeOffline, ResearchModeOnline, ResearchModeAuto:
		return nil
	default:
		return fmt.Errorf("invalid research mode %q", mode)
	}
}

// NetworkOperation is stable, bounded authorization metadata. It never
// contains a URL, workspace path, query text, or source content.
type NetworkOperation string

const (
	NetworkOperationDiscovery  NetworkOperation = "research.discovery"
	NetworkOperationFetch      NetworkOperation = "research.fetch"
	NetworkOperationRelease    NetworkOperation = "research.release_lookup"
	NetworkOperationUpdateScan NetworkOperation = "research.update_scan"
)

// NetworkResearchAccess is constructed from Foundation's resolved privacy
// gate and passed to every application service capable of live research.
type NetworkResearchAccess struct {
	Gate privacy.NetworkGate
}

type networkResearchAccess struct {
	gate privacy.NetworkGate
}

type networkAccessDecision struct {
	live    bool
	blocked error
}

func newNetworkResearchAccess(access NetworkResearchAccess) networkResearchAccess {
	return networkResearchAccess{gate: access.Gate}
}

func (access networkResearchAccess) decide(ctx context.Context, mode ResearchMode, operation NetworkOperation) (networkAccessDecision, error) {
	if err := mode.Validate(); err != nil {
		return networkAccessDecision{}, invalid(string(operation), err)
	}
	if err := ctx.Err(); err != nil {
		return networkAccessDecision{}, Classify(ErrorUnavailable, string(operation), err)
	}
	if mode == ResearchModeOffline {
		return networkAccessDecision{blocked: errOfflineResearchMode}, nil
	}
	if access.gate == nil {
		return networkAccessDecision{}, Classify(ErrorUnavailable, string(operation), errors.New("privacy network gate is not configured"))
	}
	err := access.gate.Authorize(ctx, privacy.Request{
		Operation: string(operation),
		Purpose:   privacy.ExternalResource,
	})
	if err == nil {
		return networkAccessDecision{live: true}, nil
	}
	if errors.Is(err, privacy.ErrNetworkBlocked) {
		if mode == ResearchModeAuto {
			return networkAccessDecision{blocked: err}, nil
		}
		return networkAccessDecision{}, networkResearchBlocked(string(operation), err, nil)
	}
	return networkAccessDecision{}, boundaryError(ErrorUnavailable, string(operation), err)
}

var errOfflineResearchMode = errors.New("live research is disabled in offline mode")

func networkResearchBlocked(operation string, privacyCause, cacheCause error) error {
	cause := privacyCause
	if cause == nil {
		cause = errOfflineResearchMode
	}
	cause = errors.Join(ErrNetworkDisabled, cause)
	if cacheCause != nil {
		cause = errors.Join(cause, fmt.Errorf("offline cache unavailable: %w", cacheCause))
	}
	return Classify(ErrorNetworkResearchBlocked, operation, cause)
}

func offlineFallbackError(operation string, blocked, cacheErr error) error {
	if errors.Is(cacheErr, ErrNotFound) {
		return networkResearchBlocked(operation, blocked, cacheErr)
	}
	return repositoryError(operation, cacheErr)
}

type fetchService struct {
	fetcher SourceFetcher
	cache   SourceFetchCache
	access  networkResearchAccess
	costs   ResearchCostService
	clock   Clock
	runID   research.ID
}

func NewFetchService(fetcher SourceFetcher, cache SourceFetchCache, access NetworkResearchAccess) FetchService {
	return &fetchService{fetcher: fetcher, cache: cache, access: newNetworkResearchAccess(access)}
}

// NewCostControlledFetchService binds fetch authorization and cache savings to
// one durable run. Live work reserves one fetch plus its bounded byte allowance
// before the adapter is called; offline cache reuse records avoided units.
func NewCostControlledFetchService(fetcher SourceFetcher, cache SourceFetchCache, access NetworkResearchAccess, costs ResearchCostService, clock Clock, runID research.ID) (FetchService, error) {
	if err := runID.Validate(); err != nil {
		return nil, fmt.Errorf("cost-controlled fetch run: %w", err)
	}
	if costs == nil {
		return nil, errors.New("cost-controlled fetch research cost service is unavailable")
	}
	if clock == nil {
		return nil, errors.New("cost-controlled fetch clock is unavailable")
	}
	return &fetchService{fetcher: fetcher, cache: cache, access: newNetworkResearchAccess(access), costs: costs, clock: clock, runID: runID}, nil
}

func (service *fetchService) Fetch(ctx context.Context, mode ResearchMode, request FetchRequest) (FetchedSource, error) {
	const operation = "fetch source"
	if err := request.Validate(); err != nil {
		return FetchedSource{}, invalid(operation, err)
	}
	decision, err := service.access.decide(ctx, mode, NetworkOperationFetch)
	if err != nil {
		return FetchedSource{}, err
	}
	if !decision.live {
		if service.cache == nil {
			return FetchedSource{}, networkResearchBlocked(operation, decision.blocked, nil)
		}
		fetched, cacheErr := service.cache.FetchCached(ctx, request)
		if cacheErr != nil {
			return FetchedSource{}, offlineFallbackError(operation, decision.blocked, cacheErr)
		}
		validated, validateErr := validateFetchedSource(operation, request, fetched, true)
		if validateErr != nil {
			return FetchedSource{}, validateErr
		}
		if service.costs != nil {
			if err := service.accountFetch(ctx, operation, research.ResearchCostUsage{FetchRequests: 1, Bytes: int64(len(validated.Body))}, true); err != nil {
				return FetchedSource{}, err
			}
		}
		return validated, nil
	}
	if err := requireDependency(operation, "source fetcher", service.fetcher); err != nil {
		return FetchedSource{}, err
	}
	if service.costs != nil {
		if err := service.accountFetch(ctx, operation, research.ResearchCostUsage{FetchRequests: 1, Bytes: request.MaximumBytes}, false); err != nil {
			return FetchedSource{}, err
		}
	}
	fetched, err := service.fetcher.Fetch(ctx, request)
	if err != nil {
		return FetchedSource{}, externalError(operation, err)
	}
	return validateFetchedSource(operation, request, fetched, false)
}

func (service *fetchService) accountFetch(ctx context.Context, operation string, usage research.ResearchCostUsage, cache bool) error {
	at := service.clock.Now()
	if err := at.Validate(); err != nil {
		return invalid(operation, fmt.Errorf("research cost clock: %w", err))
	}
	decision, err := service.costs.Evaluate(ctx, CostControlRequest{
		RunID: service.runID, ProposedUsage: usage, ValidCacheAvailable: cache, At: at,
	})
	if err != nil {
		return boundaryError(ErrorPersistenceFailure, operation, err)
	}
	if cache {
		return nil
	}
	return costControlError(operation, decision)
}

func validateFetchedSource(operation string, request FetchRequest, fetched FetchedSource, cached bool) (FetchedSource, error) {
	if cached {
		fetched.Origin = FetchOriginCache
		fetched.CacheHit = true
	} else {
		fetched.Origin = FetchOriginLive
		fetched.CacheHit = false
		fetched.CacheStale = false
		fetched.CacheWarning = ""
	}
	err := fetched.Validate()
	if err == nil && fetched.SourceID != request.SourceID {
		err = errors.New("fetched source identity does not match request")
	}
	if err == nil && int64(len(fetched.Body)) > request.MaximumBytes {
		err = errors.New("fetched source body exceeds request limit")
	}
	if err == nil {
		return fetched, nil
	}
	if cached {
		return FetchedSource{}, repositoryError(operation, err)
	}
	return FetchedSource{}, externalError(operation, err)
}

type releaseLookupService struct {
	provider ReleaseLookupProvider
	cache    ReleaseLookupCache
	access   networkResearchAccess
}

func NewReleaseLookupService(provider ReleaseLookupProvider, cache ReleaseLookupCache, access NetworkResearchAccess) ReleaseLookupService {
	return &releaseLookupService{provider: provider, cache: cache, access: newNetworkResearchAccess(access)}
}

func (service *releaseLookupService) Lookup(ctx context.Context, mode ResearchMode, query ReleaseLookupQuery) ([]research.ReleaseRecord, error) {
	const operation = "lookup technology releases"
	if err := query.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	decision, err := service.access.decide(ctx, mode, NetworkOperationRelease)
	if err != nil {
		return nil, err
	}
	if !decision.live {
		if service.cache == nil {
			return nil, networkResearchBlocked(operation, decision.blocked, nil)
		}
		records, cacheErr := service.cache.LookupCachedReleases(ctx, query)
		if cacheErr != nil {
			return nil, offlineFallbackError(operation, decision.blocked, cacheErr)
		}
		return validateReleaseLookup(operation, query, records, true)
	}
	if err := requireDependency(operation, "release lookup provider", service.provider); err != nil {
		return nil, err
	}
	records, err := service.provider.LookupReleases(ctx, query)
	if err != nil {
		return nil, externalError(operation, err)
	}
	return validateReleaseLookup(operation, query, records, false)
}

func validateReleaseLookup(operation string, query ReleaseLookupQuery, records []research.ReleaseRecord, cached bool) ([]research.ReleaseRecord, error) {
	for index, record := range records {
		err := record.Validate()
		if err == nil && (record.TechnologyID != query.TechnologyID || record.Channel != query.Channel) {
			err = errors.New("release result does not match lookup query")
		}
		if err == nil {
			continue
		}
		cause := fmt.Errorf("result %d: %w", index, err)
		if cached {
			return nil, repositoryError(operation, cause)
		}
		return nil, externalError(operation, cause)
	}
	return records, nil
}
